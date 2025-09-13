package v0_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/modelcontextprotocol/registry/internal/api/handlers/v0"
	"github.com/modelcontextprotocol/registry/internal/auth"
	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/service"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublishMultiSlashValidation_Integration tests the multi-slash validation in the publish endpoint
func TestPublishMultiSlashValidation_Integration(t *testing.T) {
	// Setup test environment
	cfg := &config.Config{
		JWTSecret:                "test-secret-key-for-testing-only",
		EnableRegistryValidation: false,
		DatabaseType:             "in-memory",
	}

	db := database.NewInMemoryDatabase()
	registryService := service.NewRegistryService(db, cfg)
	jwtManager := auth.NewJWTManager(cfg)

	// Create test HTTP server
	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterPublishEndpoint(api, registryService, cfg)

	// Generate a valid JWT token for testing
	token, err := jwtManager.CreateToken(context.Background(), []auth.Permission{
		{
			Action:          auth.PermissionActionPublish,
			ResourcePattern: "*",
		},
	}, 1*time.Hour)
	require.NoError(t, err, "Failed to create JWT token")

	testCases := []struct {
		name           string
		serverName     string
		expectStatus   int
		expectError    bool
		errorContains  []string
		description    string
	}{
		// Valid single-slash cases
		{
			name:         "valid_single_slash",
			serverName:   "com.example/server",
			expectStatus: http.StatusOK,
			expectError:  false,
			description:  "Standard valid format with single slash",
		},
		{
			name:         "valid_complex_namespace",
			serverName:   "com.company.dept.team/project",
			expectStatus: http.StatusOK,
			expectError:  false,
			description:  "Complex namespace with single slash",
		},

		// Invalid multi-slash cases - should return HTTP 400
		{
			name:         "two_slashes_basic",
			serverName:   "com.example/server/extra",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
				"com.example/server/extra",
			},
			description: "Two slashes should return HTTP 400",
		},
		{
			name:         "three_slashes",
			serverName:   "com.example/server/path/deep",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
				"com.example/server/path/deep",
			},
			description: "Three slashes should return HTTP 400",
		},
		{
			name:         "consecutive_slashes",
			serverName:   "com.example//server",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
			},
			description: "Consecutive slashes should return HTTP 400",
		},
		{
			name:         "trailing_slash",
			serverName:   "com.example/server/",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
			},
			description: "Trailing slash should return HTTP 400",
		},
		{
			name:         "version_like_path",
			serverName:   "com.example/server/v1",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
				"com.example/server/v1",
			},
			description: "Version-like path should return HTTP 400",
		},
		{
			name:         "github_url_like",
			serverName:   "github.com/user/repo/releases",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
			},
			description: "GitHub URL-like path should return HTTP 400",
		},
		{
			name:         "api_endpoint_like",
			serverName:   "com.example/api/v2/server",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
			},
			description: "API endpoint-like path should return HTTP 400",
		},
		{
			name:         "file_path_like",
			serverName:   "com.example/path/to/server",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
			},
			description: "File path-like structure should return HTTP 400",
		},
		{
			name:         "many_slashes",
			serverName:   "com.example/a/b/c/d/e/f/g",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
			},
			description: "Many slashes should return HTTP 400",
		},

		// Other invalid formats
		{
			name:         "no_slash",
			serverName:   "com.example.server",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"server name must be in format",
			},
			description: "No slash should return HTTP 400",
		},
		{
			name:         "empty_namespace",
			serverName:   "/server",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"non-empty namespace and name parts",
			},
			description: "Empty namespace should return HTTP 400",
		},
		{
			name:         "empty_name",
			serverName:   "com.example/",
			expectStatus: http.StatusBadRequest,
			expectError:  true,
			errorContains: []string{
				"Failed to publish server",
				"non-empty namespace and name parts",
			},
			description: "Empty name should return HTTP 400",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create server JSON payload
			serverJSON := apiv0.ServerJSON{
				Name:        tc.serverName,
				Description: tc.description,
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				},
			}

			// Marshal to JSON
			body, err := json.Marshal(serverJSON)
			require.NoError(t, err, "Failed to marshal server JSON")

			// Create HTTP request
			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			// Create response recorder
			rec := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(rec, req)

			// Check response status
			assert.Equal(t, tc.expectStatus, rec.Code,
				"Expected status %d for server name '%s', got %d. Response: %s",
				tc.expectStatus, tc.serverName, rec.Code, rec.Body.String())

			if tc.expectError {
				// Parse error response
				var errorResp map[string]interface{}
				err := json.Unmarshal(rec.Body.Bytes(), &errorResp)
				require.NoError(t, err, "Failed to parse error response")

				// Check error message contains expected strings
				errorMsg, ok := errorResp["detail"].(string)
				if !ok {
					if errors, ok := errorResp["errors"].([]interface{}); ok && len(errors) > 0 {
						if firstError, ok := errors[0].(map[string]interface{}); ok {
							errorMsg, _ = firstError["detail"].(string)
						}
					}
				}

				for _, expected := range tc.errorContains {
					assert.Contains(t, errorMsg, expected,
						"Error message should contain '%s' for case: %s", expected, tc.description)
				}
			} else {
				// Parse success response
				var response apiv0.ServerJSON
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err, "Failed to parse success response")
				assert.Equal(t, tc.serverName, response.Name,
					"Published server name should match input")
			}
		})
	}
}

// TestPublishMultiSlashValidation_ErrorMessageClarity tests that error messages are clear and actionable
func TestPublishMultiSlashValidation_ErrorMessageClarity(t *testing.T) {
	// Setup test environment
	cfg := &config.Config{
		JWTSecret:                "test-secret-key-for-testing-only",
		EnableRegistryValidation: false,
		DatabaseType:             "in-memory",
	}

	db := database.NewInMemoryDatabase()
	registryService := service.NewRegistryService(db, cfg)
	jwtManager := auth.NewJWTManager(cfg)

	// Create test HTTP server
	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterPublishEndpoint(api, registryService, cfg)

	// Generate a valid JWT token
	token, err := jwtManager.CreateToken(context.Background(), []auth.Permission{
		{
			Action:          auth.PermissionActionPublish,
			ResourcePattern: "*",
		},
	}, 1*time.Hour)
	require.NoError(t, err)

	// Test specific error message formats
	testCases := []struct {
		serverName            string
		expectedMessageParts  []string
		unexpectedMessageParts []string
		description          string
	}{
		{
			serverName: "com.example/server/extra",
			expectedMessageParts: []string{
				"Failed to publish server",
				"server name cannot contain multiple slashes",
				"com.example/server/extra", // Should include the problematic name
			},
			unexpectedMessageParts: []string{
				"panic",
				"internal error",
				"unknown error",
			},
			description: "Error should be clear about multi-slash issue",
		},
		{
			serverName: "com.example/a/b/c/d",
			expectedMessageParts: []string{
				"server name cannot contain multiple slashes",
				"com.example/a/b/c/d",
			},
			description: "Error should show the full problematic path",
		},
		{
			serverName: "github.com/user/repo/releases/latest",
			expectedMessageParts: []string{
				"server name cannot contain multiple slashes",
			},
			description: "Error should be clear for URL-like patterns",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.serverName, func(t *testing.T) {
			// Create server JSON payload
			serverJSON := apiv0.ServerJSON{
				Name:        tc.serverName,
				Description: tc.description,
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				},
			}

			// Marshal to JSON
			body, err := json.Marshal(serverJSON)
			require.NoError(t, err)

			// Create HTTP request
			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			// Create response recorder
			rec := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(rec, req)

			// Should return 400 Bad Request
			assert.Equal(t, http.StatusBadRequest, rec.Code)

			// Parse error response
			var errorResp map[string]interface{}
			err = json.Unmarshal(rec.Body.Bytes(), &errorResp)
			require.NoError(t, err)

			// Extract error message
			errorMsg, ok := errorResp["detail"].(string)
			if !ok {
				t.Fatalf("Error response missing 'detail' field: %v", errorResp)
			}

			// Check expected message parts
			for _, expected := range tc.expectedMessageParts {
				assert.Contains(t, errorMsg, expected,
					"Error message should contain '%s'", expected)
			}

			// Check unexpected message parts
			for _, unexpected := range tc.unexpectedMessageParts {
				assert.NotContains(t, errorMsg, unexpected,
					"Error message should not contain '%s'", unexpected)
			}

			// Verify the error message is user-friendly
			assert.True(t, strings.Contains(errorMsg, "Failed to publish server"),
				"Error should start with a clear failure message")
			assert.True(t, strings.Contains(errorMsg, "server name cannot contain multiple slashes"),
				"Error should clearly state the validation rule")
		})
	}
}

// TestPublishMultiSlashValidation_BoundaryConditions tests edge cases and boundary conditions
func TestPublishMultiSlashValidation_BoundaryConditions(t *testing.T) {
	// Setup test environment
	cfg := &config.Config{
		JWTSecret:                "test-secret-key-for-testing-only",
		EnableRegistryValidation: false,
		DatabaseType:             "in-memory",
	}

	db := database.NewInMemoryDatabase()
	registryService := service.NewRegistryService(db, cfg)
	jwtManager := auth.NewJWTManager(cfg)

	// Create test HTTP server
	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterPublishEndpoint(api, registryService, cfg)

	// Generate a valid JWT token
	token, err := jwtManager.CreateToken(context.Background(), []auth.Permission{
		{
			Action:          auth.PermissionActionPublish,
			ResourcePattern: "*",
		},
	}, 1*time.Hour)
	require.NoError(t, err)

	// Helper function to make publish request
	makePublishRequest := func(serverName string) (*httptest.ResponseRecorder, error) {
		serverJSON := apiv0.ServerJSON{
			Name:        serverName,
			Description: "Test server",
			Version:     "1.0.0",
			Repository: model.Repository{
				URL:    "https://github.com/example/repo",
				Source: "github",
			},
		}

		body, err := json.Marshal(serverJSON)
		if err != nil {
			return nil, err
		}

		req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		return rec, nil
	}

	t.Run("exactly_one_slash", func(t *testing.T) {
		rec, err := makePublishRequest("a/b")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code, "Minimal valid format should succeed")
	})

	t.Run("exactly_two_slashes", func(t *testing.T) {
		rec, err := makePublishRequest("a/b/c")
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "Two slashes should fail")
	})

	t.Run("very_long_namespace_single_slash", func(t *testing.T) {
		longNamespace := strings.Repeat("com.", 50) + "example"
		rec, err := makePublishRequest(longNamespace + "/server")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code, "Long namespace with single slash should succeed")
	})

	t.Run("very_long_name_single_slash", func(t *testing.T) {
		longName := strings.Repeat("server", 50)
		rec, err := makePublishRequest("com.example/" + longName)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code, "Long server name with single slash should succeed")
	})

	t.Run("very_long_multi_slash", func(t *testing.T) {
		// Create a path with 100 slashes
		parts := make([]string, 101)
		for i := range parts {
			parts[i] = fmt.Sprintf("part%d", i)
		}
		serverName := strings.Join(parts, "/")
		
		rec, err := makePublishRequest(serverName)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "Many slashes should fail")
		
		var errorResp map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &errorResp)
		errorMsg, _ := errorResp["detail"].(string)
		assert.Contains(t, errorMsg, "server name cannot contain multiple slashes")
	})

	t.Run("unicode_single_slash", func(t *testing.T) {
		rec, err := makePublishRequest("com.例え/サーバー")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code, "Unicode with single slash should succeed")
	})

	t.Run("unicode_multi_slash", func(t *testing.T) {
		rec, err := makePublishRequest("com.例え/サーバー/パス")
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "Unicode with multiple slashes should fail")
	})

	t.Run("special_chars_single_slash", func(t *testing.T) {
		rec, err := makePublishRequest("com.example-test_123/server-name_456")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code, "Special characters with single slash should succeed")
	})

	t.Run("special_chars_multi_slash", func(t *testing.T) {
		rec, err := makePublishRequest("com.example-test/server/path_123")
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "Special characters with multiple slashes should fail")
	})
}

// TestPublishMultiSlashValidation_Concurrency tests the validation under concurrent requests
func TestPublishMultiSlashValidation_Concurrency(t *testing.T) {
	// Setup test environment
	cfg := &config.Config{
		JWTSecret:                "test-secret-key-for-testing-only",
		EnableRegistryValidation: false,
		DatabaseType:             "in-memory",
	}

	db := database.NewInMemoryDatabase()
	registryService := service.NewRegistryService(db, cfg)
	jwtManager := auth.NewJWTManager(cfg)

	// Create test HTTP server
	router := http.NewServeMux()
	api := humago.New(router, huma.DefaultConfig("Test API", "1.0.0"))
	v0.RegisterPublishEndpoint(api, registryService, cfg)

	// Generate a valid JWT token
	token, err := jwtManager.CreateToken(context.Background(), []auth.Permission{
		{
			Action:          auth.PermissionActionPublish,
			ResourcePattern: "*",
		},
	}, 1*time.Hour)
	require.NoError(t, err)

	// Test cases for concurrent execution
	serverNames := []string{
		"com.example/server",          // Valid
		"com.example/server/extra",    // Invalid - 2 slashes
		"com.test/project",            // Valid
		"com.test/a/b/c",             // Invalid - 3 slashes
		"org.company/service",         // Valid
		"org.company/service/v1/api",  // Invalid - 3 slashes
		"io.github/repo",              // Valid
		"io.github/user/repo/branch",  // Invalid - 3 slashes
	}

	// Run concurrent requests
	type result struct {
		serverName string
		statusCode int
		hasError   bool
	}

	results := make(chan result, len(serverNames))

	for _, name := range serverNames {
		go func(serverName string) {
			serverJSON := apiv0.ServerJSON{
				Name:        serverName,
				Description: "Concurrent test",
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				},
			}

			body, _ := json.Marshal(serverJSON)
			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			// Check if response contains error
			var resp map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &resp)
			_, hasError := resp["detail"]

			results <- result{
				serverName: serverName,
				statusCode: rec.Code,
				hasError:   hasError,
			}
		}(name)
	}

	// Collect results
	for i := 0; i < len(serverNames); i++ {
		res := <-results
		
		// Verify each result
		slashCount := strings.Count(res.serverName, "/")
		if slashCount == 1 {
			assert.Equal(t, http.StatusOK, res.statusCode,
				"Server name '%s' with 1 slash should succeed", res.serverName)
			assert.False(t, res.hasError,
				"Server name '%s' should not have error", res.serverName)
		} else {
			assert.Equal(t, http.StatusBadRequest, res.statusCode,
				"Server name '%s' with %d slashes should fail", res.serverName, slashCount)
			assert.True(t, res.hasError,
				"Server name '%s' should have error", res.serverName)
		}
	}
}