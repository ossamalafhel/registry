package v0_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	v0 "github.com/modelcontextprotocol/registry/internal/api/handlers/v0"
	"github.com/modelcontextprotocol/registry/internal/auth"
	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/service"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishWithMultipleOCIRegistries(t *testing.T) {
	// Create test config with validation disabled for testing
	testSeed := make([]byte, ed25519.SeedSize)
	_, err := rand.Read(testSeed)
	require.NoError(t, err)
	testConfig := &config.Config{
		JWTPrivateKey:            hex.EncodeToString(testSeed),
		EnableRegistryValidation: false, // Disable validation for integration tests
	}

	// Setup fake service
	registryService := service.NewRegistryService(database.NewMemoryDB(), testConfig)

	// Create a new ServeMux and Huma API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))

	// Register the endpoint
	v0.RegisterPublishEndpoint(api, registryService, testConfig)

	// Generate valid JWT token with wildcard permission
	jwtManager := auth.NewJWTManager(testConfig)
	claims := auth.JWTClaims{
		AuthMethod: auth.MethodNone,
		Permissions: []auth.Permission{
			{Action: auth.PermissionActionPublish, ResourcePattern: "*"},
		},
	}
	tokenResponse, err := jwtManager.GenerateTokenResponse(context.Background(), claims)
	require.NoError(t, err)
	token := tokenResponse.RegistryToken

	testCases := []struct {
		name            string
		registryBaseURL string
		identifier      string
		description     string
	}{
		{
			name:            "GitHub Container Registry (GHCR)",
			registryBaseURL: model.RegistryURLGHCR,
			identifier:      "octocat/hello-world-mcp",
			description:     "MCP server published to GitHub Container Registry",
		},
		{
			name:            "Google Artifact Registry (GAR)",
			registryBaseURL: model.RegistryURLGAR,
			identifier:      "my-project/my-repo/mcp-server",
			description:     "MCP server published to Google Artifact Registry",
		},
		{
			name:            "Google Container Registry (GCR)",
			registryBaseURL: model.RegistryURLGCR,
			identifier:      "my-project/mcp-server",
			description:     "MCP server published to Google Container Registry",
		},
		{
			name:            "Quay.io",
			registryBaseURL: model.RegistryURLQuay,
			identifier:      "myorg/mcp-server",
			description:     "MCP server published to Quay.io",
		},
		{
			name:            "GitLab Container Registry",
			registryBaseURL: model.RegistryURLGitLabCR,
			identifier:      "mygroup/myproject/mcp-server",
			description:     "MCP server published to GitLab Container Registry",
		},
		{
			name:            "Amazon ECR Public",
			registryBaseURL: model.RegistryURLECR,
			identifier:      "myregistry/mcp-server",
			description:     "MCP server published to Amazon ECR Public",
		},
		{
			name:            "Regional GAR endpoint",
			registryBaseURL: "https://us-central1-docker.pkg.dev",
			identifier:      "my-project/my-repo/mcp-server",
			description:     "MCP server published to regional GAR endpoint",
		},
		{
			name:            "Regional GCR endpoint",
			registryBaseURL: "https://us.gcr.io",
			identifier:      "my-project/mcp-server",
			description:     "MCP server published to regional GCR endpoint",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use unique server name for each test to avoid duplicate version errors
			// Remove slashes from identifier to create a valid server name part
			safeIdentifier := strings.ReplaceAll(tc.identifier, "/", "-")
			serverName := fmt.Sprintf("com.example/oci-test-%s-%d", safeIdentifier, time.Now().UnixNano())
			publishReq := apiv0.ServerJSON{
				Name:        serverName,
				Description: tc.description,
				Version:     "1.0.0",
				Status:      model.StatusActive,
				Packages: []model.Package{
					{
						RegistryType:    model.RegistryTypeOCI,
						RegistryBaseURL: tc.registryBaseURL,
						Identifier:      tc.identifier,
						Version:         "v1.0.0",
						Transport: model.Transport{
							Type: model.TransportTypeStdio,
						},
					},
				},
			}

			body, err := json.Marshal(publishReq)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code, "Response body: %s", rr.Body.String())

			var response apiv0.ServerJSON
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, publishReq.Name, response.Name)
			assert.Equal(t, publishReq.Version, response.Version)
			assert.Len(t, response.Packages, 1)
			assert.Equal(t, tc.registryBaseURL, response.Packages[0].RegistryBaseURL)
			assert.Equal(t, tc.identifier, response.Packages[0].Identifier)
		})
	}
}

func TestPublishWithUnsupportedOCIRegistry(t *testing.T) {
	// Create test config with validation ENABLED
	testSeed := make([]byte, ed25519.SeedSize)
	_, err := rand.Read(testSeed)
	require.NoError(t, err)
	testConfig := &config.Config{
		JWTPrivateKey:            hex.EncodeToString(testSeed),
		EnableRegistryValidation: true, // Enable validation
	}

	// Setup fake service
	registryService := service.NewRegistryService(database.NewMemoryDB(), testConfig)

	// Create a new ServeMux and Huma API
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Test API", "1.0.0"))

	// Register the endpoint
	v0.RegisterPublishEndpoint(api, registryService, testConfig)

	// Generate valid JWT token
	jwtManager := auth.NewJWTManager(testConfig)
	claims := auth.JWTClaims{
		AuthMethod: auth.MethodNone,
		Permissions: []auth.Permission{
			{Action: auth.PermissionActionPublish, ResourcePattern: "*"},
		},
	}
	tokenResponse, err := jwtManager.GenerateTokenResponse(context.Background(), claims)
	require.NoError(t, err)
	token := tokenResponse.RegistryToken

	publishReq := apiv0.ServerJSON{
		Name:        "com.example/unsupported-registry-test",
		Description: "Test with unsupported registry",
		Version:     "1.0.0",
		Packages: []model.Package{
			{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: "https://unsupported.registry.com",
				Identifier:      "test/image",
				Version:         "v1.0.0",
				Transport: model.Transport{
					Type: model.TransportTypeStdio,
				},
			},
		},
	}

	body, err := json.Marshal(publishReq)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Should fail with bad request when validation is enabled
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "unsupported OCI registry")
}