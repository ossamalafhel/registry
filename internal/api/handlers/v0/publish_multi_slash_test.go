package v0_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/registry/internal/auth"
	"github.com/modelcontextprotocol/registry/internal/config"
	"github.com/modelcontextprotocol/registry/internal/database"
	"github.com/modelcontextprotocol/registry/internal/service"
	"github.com/modelcontextprotocol/registry/internal/validators"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublishService_MultiSlashValidation tests multi-slash validation at the service layer
func TestPublishService_MultiSlashValidation(t *testing.T) {
	// Setup test service
	cfg := &config.Config{
		EnableRegistryValidation: false,
		DatabaseType:             "in-memory",
	}
	db := database.NewInMemoryDatabase()
	registryService := service.NewRegistryService(db, cfg)

	testCases := []struct {
		name          string
		serverName    string
		expectError   bool
		errorContains string
		description   string
	}{
		// Valid cases
		{
			name:        "valid_single_slash",
			serverName:  "com.example/server",
			expectError: false,
			description: "Valid single slash format",
		},
		{
			name:        "valid_complex_namespace",
			serverName:  "com.company.dept.team/project",
			expectError: false,
			description: "Complex namespace with single slash",
		},
		{
			name:        "valid_hyphenated",
			serverName:  "org.non-profit/my-cool-server",
			expectError: false,
			description: "Hyphenated names with single slash",
		},
		{
			name:        "valid_underscore",
			serverName:  "com.example/server_name",
			expectError: false,
			description: "Underscore in name with single slash",
		},

		// Invalid multi-slash cases
		{
			name:          "two_slashes",
			serverName:    "com.example/server/extra",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "Two slashes should fail",
		},
		{
			name:          "three_slashes",
			serverName:    "com.example/server/path/deep",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "Three slashes should fail",
		},
		{
			name:          "four_slashes",
			serverName:    "com.example/a/b/c/d",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "Four slashes should fail",
		},
		{
			name:          "consecutive_slashes",
			serverName:    "com.example//server",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "Consecutive slashes should fail",
		},
		{
			name:          "trailing_slash",
			serverName:    "com.example/server/",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "Trailing slash should fail",
		},
		{
			name:          "leading_and_middle",
			serverName:    "/com.example/server",
			expectError:   true,
			errorContains: "non-empty namespace and name parts",
			description:   "Leading slash creates empty namespace",
		},
		{
			name:          "version_path",
			serverName:    "com.example/server/v1",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "Version-like path should fail",
		},
		{
			name:          "api_endpoint",
			serverName:    "com.example/api/v2/server",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "API endpoint-like path should fail",
		},
		{
			name:          "github_url",
			serverName:    "github.com/user/repo/releases",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "GitHub URL-like path should fail",
		},
		{
			name:          "npm_scope",
			serverName:    "@scope/package/server",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "NPM scope-like format should fail",
		},
		{
			name:          "file_path",
			serverName:    "com.example/path/to/server",
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
			description:   "File path-like structure should fail",
		},

		// Other invalid formats
		{
			name:          "no_slash",
			serverName:    "com.example.server",
			expectError:   true,
			errorContains: "server name must be in format",
			description:   "No slash should fail",
		},
		{
			name:          "empty_string",
			serverName:    "",
			expectError:   true,
			errorContains: "server name is required",
			description:   "Empty name should fail",
		},
		{
			name:          "only_slash",
			serverName:    "/",
			expectError:   true,
			errorContains: "non-empty namespace and name parts",
			description:   "Only slash should fail",
		},
		{
			name:          "empty_namespace",
			serverName:    "/server",
			expectError:   true,
			errorContains: "non-empty namespace and name parts",
			description:   "Empty namespace should fail",
		},
		{
			name:          "empty_name",
			serverName:    "com.example/",
			expectError:   true,
			errorContains: "non-empty namespace and name parts",
			description:   "Empty name should fail",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:        tc.serverName,
				Description: tc.description,
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				},
			}

			// Attempt to publish
			_, err := registryService.Publish(serverJSON)

			if tc.expectError {
				require.Error(t, err, "Expected error for: %s", tc.description)
				assert.Contains(t, err.Error(), tc.errorContains,
					"Error should contain '%s' for: %s", tc.errorContains, tc.description)
			} else {
				assert.NoError(t, err, "Expected no error for: %s", tc.description)
			}
		})
	}
}

// TestValidateServerJSON_MultiSlash tests the ValidateServerJSON function directly
func TestValidateServerJSON_MultiSlash(t *testing.T) {
	testCases := []struct {
		name          string
		serverJSON    apiv0.ServerJSON
		expectError   bool
		errorContains string
	}{
		{
			name: "valid_complete_server",
			serverJSON: apiv0.ServerJSON{
				Name:        "com.example/server",
				Description: "Valid server",
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/server",
					Source: "github",
				},
				WebsiteURL: "https://example.com",
				Packages: []model.Package{
					{
						Identifier:   "example-package",
						RegistryType: model.RegistryTypeNPM,
						Version:      "1.0.0",
						Transport:    model.Transport{Type: model.TransportTypeStdio},
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid_multi_slash_in_name",
			serverJSON: apiv0.ServerJSON{
				Name:        "com.example/server/path",
				Description: "Invalid multi-slash server",
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/server",
					Source: "github",
				},
			},
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes",
		},
		{
			name: "invalid_multiple_validation_errors",
			serverJSON: apiv0.ServerJSON{
				Name:        "com.example/server/path",
				Description: "Multiple validation errors",
				Version:     "latest", // Also invalid
				Repository: model.Repository{
					URL:    "not-a-url", // Also invalid
					Source: "github",
				},
			},
			expectError:   true,
			errorContains: "server name cannot contain multiple slashes", // Should catch multi-slash first
		},
		{
			name: "valid_name_invalid_package",
			serverJSON: apiv0.ServerJSON{
				Name:        "com.example/server",
				Description: "Valid name, invalid package",
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/server",
					Source: "github",
				},
				Packages: []model.Package{
					{
						Identifier:   "package with spaces", // Invalid
						RegistryType: model.RegistryTypeNPM,
						Version:      "1.0.0",
						Transport:    model.Transport{Type: model.TransportTypeStdio},
					},
				},
			},
			expectError:   true,
			errorContains: "package name cannot contain spaces",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validators.ValidateServerJSON(&tc.serverJSON)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMultiSlashValidation_SlashCounting verifies the slash counting logic
func TestMultiSlashValidation_SlashCounting(t *testing.T) {
	testCases := []struct {
		serverName   string
		slashCount   int
		expectError  bool
	}{
		{"noSlash", 0, true},
		{"one/slash", 1, false},
		{"two/slash/es", 2, true},
		{"three/slash/es/here", 3, true},
		{"four/slash/es/in/path", 4, true},
		{"////", 4, true},
		{"/leading/slash", 2, true},
		{"trailing/slash/", 2, true},
		{"double//slash", 2, true},
		{"a/b/c/d/e/f/g/h/i/j/k", 10, true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_slashes", tc.slashCount), func(t *testing.T) {
			// Verify test case has expected slash count
			actualCount := strings.Count(tc.serverName, "/")
			assert.Equal(t, tc.slashCount, actualCount, "Test case slash count mismatch")

			serverJSON := apiv0.ServerJSON{
				Name:    tc.serverName,
				Version: "1.0.0",
			}

			if !tc.expectError {
				serverJSON.Repository = model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				}
			}

			err := validators.ValidateServerJSON(&serverJSON)

			if tc.expectError {
				require.Error(t, err, "Expected error for %d slashes", tc.slashCount)
				if tc.slashCount > 1 {
					assert.Contains(t, err.Error(), "slash",
						"Error should mention slash for multi-slash case")
				}
			} else {
				assert.NoError(t, err, "Expected no error for single slash")
			}
		})
	}
}

// TestMultiSlashValidation_ErrorMessage verifies error message clarity
func TestMultiSlashValidation_ErrorMessage(t *testing.T) {
	testCases := []struct {
		name                string
		serverName          string
		expectedInMessage   []string
		unexpectedInMessage []string
	}{
		{
			name:       "multi_slash_error",
			serverName: "com.example/server/path",
			expectedInMessage: []string{
				"server name cannot contain multiple slashes",
				"com.example/server/path",
			},
			unexpectedInMessage: []string{
				"panic",
				"internal error",
			},
		},
		{
			name:       "no_slash_error",
			serverName: "com.example.server",
			expectedInMessage: []string{
				"server name must be in format",
				"dns-namespace/name",
			},
			unexpectedInMessage: []string{
				"multiple slashes",
			},
		},
		{
			name:       "empty_parts_error",
			serverName: "/server",
			expectedInMessage: []string{
				"non-empty namespace and name parts",
			},
			unexpectedInMessage: []string{
				"multiple slashes",
			},
		},
		{
			name:       "consecutive_slashes",
			serverName: "com.example//server",
			expectedInMessage: []string{
				"server name cannot contain multiple slashes",
			},
			unexpectedInMessage: []string{
				"format",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:    tc.serverName,
				Version: "1.0.0",
			}

			err := validators.ValidateServerJSON(&serverJSON)
			require.Error(t, err, "Expected error for: %s", tc.serverName)

			errorMsg := err.Error()

			// Check expected substrings
			for _, expected := range tc.expectedInMessage {
				assert.Contains(t, errorMsg, expected,
					"Error should contain '%s'", expected)
			}

			// Check unexpected substrings
			for _, unexpected := range tc.unexpectedInMessage {
				assert.NotContains(t, errorMsg, unexpected,
					"Error should not contain '%s'", unexpected)
			}
		})
	}
}

// TestMultiSlashValidation_Performance tests validation performance with large inputs
func TestMultiSlashValidation_Performance(t *testing.T) {
	// Create very long server names
	longNamespace := strings.Repeat("com.example.subdomain.", 100)
	longName := strings.Repeat("server", 100)

	testCases := []struct {
		name        string
		serverName  string
		expectError bool
	}{
		{
			name:        "very_long_valid",
			serverName:  longNamespace + "/" + longName,
			expectError: false,
		},
		{
			name:        "very_long_invalid",
			serverName:  longNamespace + "/" + longName + "/extra/path",
			expectError: true,
		},
		{
			name:        "many_slashes",
			serverName:  "com/" + strings.Repeat("path/", 1000) + "end",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:    tc.serverName,
				Version: "1.0.0",
			}

			if !tc.expectError {
				serverJSON.Repository = model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				}
			}

			// Measure validation time
			start := time.Now()
			err := validators.ValidateServerJSON(&serverJSON)
			duration := time.Since(start)

			// Validation should be fast even with large inputs
			assert.Less(t, duration, 100*time.Millisecond,
				"Validation should complete quickly")

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "slash")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMultiSlashValidation_Unicode tests validation with Unicode characters
func TestMultiSlashValidation_Unicode(t *testing.T) {
	testCases := []struct {
		name        string
		serverName  string
		expectError bool
		description string
	}{
		{
			name:        "unicode_japanese_valid",
			serverName:  "com.例え/サーバー",
			expectError: false,
			description: "Japanese characters with single slash",
		},
		{
			name:        "unicode_japanese_invalid",
			serverName:  "com.例え/サーバー/パス",
			expectError: true,
			description: "Japanese characters with multiple slashes",
		},
		{
			name:        "unicode_chinese_valid",
			serverName:  "com.示例/服务器",
			expectError: false,
			description: "Chinese characters with single slash",
		},
		{
			name:        "unicode_chinese_invalid",
			serverName:  "com.示例/服务器/路径",
			expectError: true,
			description: "Chinese characters with multiple slashes",
		},
		{
			name:        "unicode_arabic_valid",
			serverName:  "com.مثال/خادم",
			expectError: false,
			description: "Arabic characters with single slash",
		},
		{
			name:        "unicode_arabic_invalid",
			serverName:  "com.مثال/خادم/مسار",
			expectError: true,
			description: "Arabic characters with multiple slashes",
		},
		{
			name:        "unicode_emoji_valid",
			serverName:  "com.example/server🚀",
			expectError: false,
			description: "Emoji with single slash",
		},
		{
			name:        "unicode_emoji_invalid",
			serverName:  "com.example/server🚀/path✨",
			expectError: true,
			description: "Emoji with multiple slashes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:        tc.serverName,
				Description: tc.description,
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				},
			}

			err := validators.ValidateServerJSON(&serverJSON)

			if tc.expectError {
				require.Error(t, err, "Expected error for: %s", tc.description)
				assert.Contains(t, err.Error(), "slash",
					"Error should mention slash for: %s", tc.description)
			} else {
				assert.NoError(t, err, "Expected no error for: %s", tc.description)
			}
		})
	}
}

// TestMultiSlashValidation_RegressionCases ensures backward compatibility
func TestMultiSlashValidation_RegressionCases(t *testing.T) {
	// These cases should continue to work as before
	validServerNames := []string{
		"com.example/server",
		"org.nonprofit/project",
		"io.github.username/repository",
		"net.company.product.service/component",
		"com.company/my-awesome-server",
		"edu.university.department/research-tool",
		"gov.agency.department/public-service",
		"mil.branch.unit/system",
		"com.example.subdomain/server-name",
		"uk.co.company/product",
		"jp.co.company/サーバー",
		"de.company/produkt",
		"fr.entreprise/serveur",
		"com.123company/456server",
		"com.company/server_with_underscore",
		"com.company/server-with-hyphens",
		"com.company/ServerWithCaps",
		"com.company/s", // Single character name
		"c/s",           // Minimal valid format
	}

	for _, serverName := range validServerNames {
		t.Run(serverName, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:        serverName,
				Description: "Regression test",
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				},
			}

			err := validators.ValidateServerJSON(&serverJSON)
			assert.NoError(t, err, "Valid server name '%s' should not produce error", serverName)
		})
	}
}

// TestMultiSlashValidation_WithAuth tests that multi-slash validation occurs before auth checks
func TestMultiSlashValidation_WithAuth(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:                "test-secret",
		EnableRegistryValidation: false,
		DatabaseType:             "in-memory",
	}

	db := database.NewInMemoryDatabase()
	registryService := service.NewRegistryService(db, cfg)
	jwtManager := auth.NewJWTManager(cfg)

	// Create a token with limited permissions
	token, err := jwtManager.CreateToken(context.Background(), []auth.Permission{
		{
			Action:          auth.PermissionActionPublish,
			ResourcePattern: "com.allowed/*",
		},
	}, 1*time.Hour)
	require.NoError(t, err)

	// Validate the token
	claims, err := jwtManager.ValidateToken(context.Background(), token)
	require.NoError(t, err)

	// Test case with multi-slash that would also fail auth
	serverJSON := apiv0.ServerJSON{
		Name:        "com.notallowed/server/extra", // Multi-slash AND not in allowed pattern
		Description: "Should fail on multi-slash validation first",
		Version:     "1.0.0",
		Repository: model.Repository{
			URL:    "https://github.com/example/repo",
			Source: "github",
		},
	}

	// The multi-slash validation should happen in ValidateServerJSON
	err = validators.ValidateServerJSON(&serverJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server name cannot contain multiple slashes",
		"Should fail on multi-slash validation before auth check")

	// Verify auth would also fail (but we don't get there)
	hasPermission := jwtManager.HasPermission(serverJSON.Name, auth.PermissionActionPublish, claims.Permissions)
	assert.False(t, hasPermission, "Auth should also fail, but multi-slash validation comes first")
}

// MockRegistryService is a mock implementation for testing
type MockRegistryService struct {
	PublishFunc func(serverJSON apiv0.ServerJSON) (*apiv0.ServerJSON, error)
}

func (m *MockRegistryService) Publish(serverJSON apiv0.ServerJSON) (*apiv0.ServerJSON, error) {
	if m.PublishFunc != nil {
		return m.PublishFunc(serverJSON)
	}
	// Default behavior: validate and return
	if err := validators.ValidateServerJSON(&serverJSON); err != nil {
		return nil, err
	}
	return &serverJSON, nil
}

func (m *MockRegistryService) Search(query string, limit int) ([]apiv0.ServerJSON, error) {
	return nil, errors.New("not implemented")
}

func (m *MockRegistryService) GetByName(name string) (*apiv0.ServerJSON, error) {
	return nil, errors.New("not implemented")
}

// TestMultiSlashValidation_ServiceLayer tests validation at the service layer
func TestMultiSlashValidation_ServiceLayer(t *testing.T) {
	mockService := &MockRegistryService{}

	testCases := []struct {
		name        string
		serverName  string
		expectError bool
	}{
		{
			name:        "service_valid_single_slash",
			serverName:  "com.example/server",
			expectError: false,
		},
		{
			name:        "service_invalid_multi_slash",
			serverName:  "com.example/server/extra",
			expectError: true,
		},
		{
			name:        "service_invalid_three_slashes",
			serverName:  "com.example/a/b/c",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:        tc.serverName,
				Description: "Service layer test",
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				},
			}

			_, err := mockService.Publish(serverJSON)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "slash")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}