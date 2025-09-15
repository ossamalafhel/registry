package registries //nolint:testpackage // Testing internal functions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetAuthHeader_NonDockerRegistry tests setAuthHeader for non-Docker registries
func TestSetAuthHeader_NonDockerRegistry(t *testing.T) {
	ctx := context.Background()
	req := httptest.NewRequest(http.MethodGet, "https://ghcr.io/v2/test/manifests/latest", nil)
	client := &http.Client{}

	// Test with non-Docker registry (should not add auth header)
	err := setAuthHeader(ctx, req, client, "https://ghcr.io", "", "namespace", "repo")
	assert.NoError(t, err)
	assert.Empty(t, req.Header.Get("Authorization"))
}

// TestGetSpecificManifest_DockerHubAuthError tests auth error in getSpecificManifest
func TestGetSpecificManifest_DockerHubAuthError(t *testing.T) {
	// Mock auth server that returns an error
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer authServer.Close()

	// Override httpClientFactory to redirect to our mock
	oldFactory := httpClientFactory
	httpClientFactory = func() *http.Client {
		return &http.Client{
			Transport: &testRoundTripper{authURL: authServer.URL},
		}
	}
	defer func() {
		httpClientFactory = oldFactory
	}()

	ctx := context.Background()
	client := httpClientFactory()

	// This should fail with auth error
	_, err := getSpecificManifest(ctx, client, dockerIoAPIBaseURL, "library", "test", "sha256:abc123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to authenticate with Docker registry")
}

// TestGetImageConfig_DockerHubAuthError tests auth error in getImageConfig
func TestGetImageConfig_DockerHubAuthError(t *testing.T) {
	// Mock auth server that returns an error
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer authServer.Close()

	// Override httpClientFactory to redirect to our mock
	oldFactory := httpClientFactory
	httpClientFactory = func() *http.Client {
		return &http.Client{
			Transport: &testRoundTripper{authURL: authServer.URL},
		}
	}
	defer func() {
		httpClientFactory = oldFactory
	}()

	ctx := context.Background()
	client := httpClientFactory()

	// This should fail with auth error
	_, err := getImageConfig(ctx, client, dockerIoAPIBaseURL, "library", "test", "sha256:config123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to authenticate with Docker registry")
}

// testRoundTripper redirects auth.docker.io to our test server
type testRoundTripper struct {
	authURL string
}

func (t *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "auth.docker.io" {
		req.URL.Scheme = "http"
		req.URL.Host = t.authURL[7:] // Remove http://
		if len(req.URL.Host) > 0 && req.URL.Host[0] == '/' {
			// Handle case where URL parsing failed
			req.URL.Host = "localhost:12345"
		}
	}
	return http.DefaultTransport.RoundTrip(req)
}