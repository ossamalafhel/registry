package registries

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
)

// mockRoundTripper allows us to intercept HTTP requests
type mockRoundTripper struct {
	authServer     *httptest.Server
	registryServer *httptest.Server
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Intercept auth.docker.io requests
	if req.URL.Host == "auth.docker.io" {
		req.URL.Scheme = "http"
		req.URL.Host = m.authServer.URL[7:] // Remove http://
		return http.DefaultTransport.RoundTrip(req)
	}
	
	// Intercept registry-1.docker.io requests
	if req.URL.Host == "registry-1.docker.io" {
		req.URL.Scheme = "http"
		req.URL.Host = m.registryServer.URL[7:] // Remove http://
		return http.DefaultTransport.RoundTrip(req)
	}
	
	return http.DefaultTransport.RoundTrip(req)
}

// TestValidateOCI_DockerHubFullFlow tests the complete Docker Hub flow
func TestValidateOCI_DockerHubFullFlow(t *testing.T) {
	// Create auth server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		assert.Equal(t, "registry.docker.io", r.URL.Query().Get("service"))
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OCIAuthResponse{Token: "test-token"})
	}))
	defer authServer.Close()

	// Create registry server
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		
		switch r.URL.Path {
		case "/v2/library/test-image/manifests/v1.0.0":
			w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
			manifest := OCIManifest{
				Config: struct {
					Digest string `json:"digest"`
				}{
					Digest: "sha256:config123",
				},
			}
			json.NewEncoder(w).Encode(manifest)
			
		case "/v2/library/test-image/blobs/sha256:config123":
			config := OCIImageConfig{
				Config: struct {
					Labels map[string]string `json:"Labels"`
				}{
					Labels: map[string]string{
						"io.modelcontextprotocol.server.name": "com.example/test-server",
					},
				},
			}
			json.NewEncoder(w).Encode(config)
			
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer registryServer.Close()

	// Create custom HTTP client with our mock transport
	oldClient := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: &mockRoundTripper{
			authServer:     authServer,
			registryServer: registryServer,
		},
	}
	defer func() {
		http.DefaultClient = oldClient
	}()

	ctx := context.Background()
	pkg := model.Package{
		RegistryType:    model.RegistryTypeOCI,
		RegistryBaseURL: model.RegistryURLDocker,
		Identifier:      "test-image",
		Version:         "v1.0.0",
	}

	err := ValidateOCI(ctx, pkg, "com.example/test-server")
	assert.NoError(t, err)
}

// TestGetDockerIoAuthToken_Errors tests error cases in auth token retrieval
func TestGetDockerIoAuthToken_Errors(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{}

	t.Run("Request creation error", func(t *testing.T) {
		// Use nil context to trigger error
		_, err := getDockerIoAuthToken(nil, client, "namespace", "repo")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create auth request")
	})

	t.Run("Network error", func(t *testing.T) {
		// Use invalid URL
		client := &http.Client{
			Transport: &mockRoundTripper{
				authServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Server will be closed before request
				})),
			},
		}
		
		// Close the server to simulate network error
		transport := client.Transport.(*mockRoundTripper)
		transport.authServer.Close()
		
		_, err := getDockerIoAuthToken(ctx, client, "namespace", "repo")
		assert.Error(t, err)
	})

	t.Run("Non-200 status", func(t *testing.T) {
		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer authServer.Close()

		client := &http.Client{
			Transport: &mockRoundTripper{
				authServer: authServer,
			},
		}
		
		_, err := getDockerIoAuthToken(ctx, client, "namespace", "repo")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "auth request failed with status 401")
	})

	t.Run("Invalid JSON response", func(t *testing.T) {
		authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("invalid json"))
		}))
		defer authServer.Close()

		client := &http.Client{
			Transport: &mockRoundTripper{
				authServer: authServer,
			},
		}
		
		_, err := getDockerIoAuthToken(ctx, client, "namespace", "repo")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse auth response")
	})
}

// TestGetSpecificManifest_Errors tests error cases in specific manifest retrieval
func TestGetSpecificManifest_Errors(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{}

	t.Run("Request creation error", func(t *testing.T) {
		// Use nil context to trigger error
		_, err := getSpecificManifest(nil, client, "http://test", "namespace", "repo", "digest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create specific manifest request")
	})

	t.Run("Network error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		serverURL := server.URL
		server.Close()

		_, err := getSpecificManifest(ctx, client, serverURL, "namespace", "repo", "digest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch specific manifest")
	})
}

// TestGetImageConfig_Errors tests error cases in image config retrieval
func TestGetImageConfig_Errors(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{}

	t.Run("Request creation error", func(t *testing.T) {
		// Use nil context to trigger error
		_, err := getImageConfig(nil, client, "http://test", "namespace", "repo", "digest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create config request")
	})

	t.Run("Network error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		serverURL := server.URL
		server.Close()

		_, err := getImageConfig(ctx, client, serverURL, "namespace", "repo", "digest")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch image config")
	})
}

// TestParseImageReference_AllCases tests all cases of parseImageReference
func TestParseImageReference_AllCases(t *testing.T) {
	tests := []struct {
		name          string
		identifier    string
		wantNamespace string
		wantRepo      string
		wantError     bool
	}{
		{
			name:          "namespace/repo format",
			identifier:    "myorg/myrepo",
			wantNamespace: "myorg",
			wantRepo:      "myrepo",
			wantError:     false,
		},
		{
			name:          "single name (library namespace)",
			identifier:    "nginx",
			wantNamespace: "library",
			wantRepo:      "nginx",
			wantError:     false,
		},
		{
			name:          "too many slashes",
			identifier:    "too/many/slashes",
			wantNamespace: "",
			wantRepo:      "",
			wantError:     true,
		},
		{
			name:          "empty identifier",
			identifier:    "",
			wantNamespace: "library",
			wantRepo:      "",
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace, repo, err := parseImageReference(tt.identifier)
			
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantNamespace, namespace)
				assert.Equal(t, tt.wantRepo, repo)
			}
		})
	}
}