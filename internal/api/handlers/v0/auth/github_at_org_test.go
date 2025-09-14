package auth_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	v0auth "github.com/modelcontextprotocol/registry/internal/api/handlers/v0/auth"
	"github.com/modelcontextprotocol/registry/internal/auth"
	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userEndpoint     = "/user"
	userOrgsEndpoint = "/user/orgs"
)

func TestGitHubHandler_UsesUserOrgsEndpoint(t *testing.T) {
	// This test verifies that we use /user/orgs instead of /users/{username}/orgs
	// to ensure we get ALL organizations (including private ones)

	testSeed := make([]byte, ed25519.SeedSize)
	_, err := rand.Read(testSeed)
	require.NoError(t, err)

	cfg := &config.Config{
		JWTPrivateKey: hex.EncodeToString(testSeed),
	}

	// Track which endpoints were called
	var calledEndpoints []string

	// Create mock GitHub API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledEndpoints = append(calledEndpoints, r.URL.Path)

		switch r.URL.Path {
		case userEndpoint:
			user := v0auth.GitHubUserOrOrg{
				Login: "testuser",
				ID:    12345,
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(user); err != nil {
				t.Logf("Failed to encode user response: %v", err)
			}
		case userOrgsEndpoint:
			// NEW endpoint returns ALL orgs (public + private)
			orgs := []v0auth.GitHubUserOrOrg{
				{Login: "public-org", ID: 1},
				{Login: "private-org", ID: 2}, // This would NOT be returned by /users/{username}/orgs
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(orgs); err != nil {
				t.Logf("Failed to encode orgs response: %v", err)
			}
		case "/users/testuser/orgs":
			// OLD endpoint would only return public orgs
			t.Error("Should not call /users/{username}/orgs endpoint")
			orgs := []v0auth.GitHubUserOrOrg{
				{Login: "public-org", ID: 1},
				// private-org would NOT be included here
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(orgs); err != nil {
				t.Logf("Failed to encode orgs response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Create handler and set mock server URL
	handler := v0auth.NewGitHubHandler(cfg)
	handler.SetBaseURL(mockServer.URL)

	// Test token exchange
	ctx := context.Background()
	response, err := handler.ExchangeToken(ctx, "test-token")
	require.NoError(t, err)
	require.NotNil(t, response)

	// Verify the correct endpoints were called
	assert.Contains(t, calledEndpoints, userEndpoint, "Should call /user endpoint")
	assert.Contains(t, calledEndpoints, userOrgsEndpoint, "Should call /user/orgs endpoint")
	assert.NotContains(t, calledEndpoints, "/users/testuser/orgs", "Should NOT call /users/{username}/orgs")

	// Validate the JWT token includes permissions for BOTH orgs
	jwtManager := auth.NewJWTManager(cfg)
	claims, err := jwtManager.ValidateToken(ctx, response.RegistryToken)
	require.NoError(t, err)

	// Should have 3 permissions: user + 2 orgs (including private)
	assert.Len(t, claims.Permissions, 3, "Should have permissions for user and both orgs")

	expectedPatterns := []string{
		"io.github.testuser/*",
		"io.github.public-org/*",
		"io.github.private-org/*", // This is the key - private org is included!
	}

	for i, perm := range claims.Permissions {
		assert.Equal(t, auth.PermissionActionPublish, perm.Action)
		assert.Equal(t, expectedPatterns[i], perm.ResourcePattern)
	}
}
