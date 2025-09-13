package validators_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/registry/internal/validators"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiSlashValidation_ComprehensiveCases tests comprehensive cases for multi-slash validation
func TestMultiSlashValidation_ComprehensiveCases(t *testing.T) {
	testCases := []struct {
		name         string
		serverName   string
		expectError  bool
		errorMessage string
		description  string
	}{
		// Valid cases - single slash
		{
			name:        "standard_valid_format",
			serverName:  "com.example/server",
			expectError: false,
			description: "Standard reverse-DNS namespace with single slash",
		},
		{
			name:        "complex_namespace_valid",
			serverName:  "com.company.department.team/project",
			expectError: false,
			description: "Complex multi-level namespace with single slash",
		},
		{
			name:        "short_namespace_valid",
			serverName:  "io.github/myserver",
			expectError: false,
			description: "Short namespace with single slash",
		},
		{
			name:        "hyphenated_name_valid",
			serverName:  "org.nonprofit/my-cool-server",
			expectError: false,
			description: "Hyphenated server name with single slash",
		},
		{
			name:        "underscore_name_valid",
			serverName:  "com.example/my_server_name",
			expectError: false,
			description: "Underscore in server name with single slash",
		},
		{
			name:        "numeric_in_name_valid",
			serverName:  "com.example/server123",
			expectError: false,
			description: "Numeric characters in server name",
		},

		// Invalid cases - multiple slashes
		{
			name:         "two_slashes_basic",
			serverName:   "com.example/server/extra",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Basic two-slash case",
		},
		{
			name:         "three_slashes",
			serverName:   "com.example/server/path/deep",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Three slashes in server name",
		},
		{
			name:         "four_slashes",
			serverName:   "com.example/a/b/c/d",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Four slashes in server name",
		},
		{
			name:         "many_slashes",
			serverName:   "com.example/a/b/c/d/e/f/g/h",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Many slashes in server name",
		},
		{
			name:         "double_slash_consecutive",
			serverName:   "com.example//server",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Consecutive double slashes",
		},
		{
			name:         "triple_slash_consecutive",
			serverName:   "com.example///server",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Consecutive triple slashes",
		},
		{
			name:         "trailing_slash",
			serverName:   "com.example/server/",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Trailing slash counts as second slash",
		},
		{
			name:         "leading_and_middle_slash",
			serverName:   "/com.example/server",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Leading slash plus middle slash",
		},
		{
			name:         "version_like_path",
			serverName:   "com.example/server/v1",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Version-like path segment",
		},
		{
			name:         "api_endpoint_like",
			serverName:   "com.example/api/v2/server",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "API endpoint-like path",
		},
		{
			name:         "github_url_like",
			serverName:   "github.com/user/repo/releases",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "GitHub URL-like path",
		},
		{
			name:         "npm_scope_like",
			serverName:   "@scope/package/server",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "NPM scoped package-like format",
		},
		{
			name:         "file_path_like",
			serverName:   "com.example/path/to/server",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "File path-like structure",
		},
		{
			name:         "mixed_separators",
			serverName:   "com.example.sub/server/path/deep",
			expectError:  true,
			errorMessage: validators.ErrMultipleSlashesInServerName.Error(),
			description:  "Mixed dots and slashes",
		},

		// Other invalid formats (not multi-slash specific)
		{
			name:         "no_slash",
			serverName:   "com.example.server",
			expectError:  true,
			errorMessage: "server name must be in format",
			description:  "No slash separator",
		},
		{
			name:         "empty_string",
			serverName:   "",
			expectError:  true,
			errorMessage: "server name is required",
			description:  "Empty server name",
		},
		{
			name:         "only_slash",
			serverName:   "/",
			expectError:  true,
			errorMessage: "non-empty namespace and name parts",
			description:  "Only slash character",
		},
		{
			name:         "empty_namespace",
			serverName:   "/server",
			expectError:  true,
			errorMessage: "non-empty namespace and name parts",
			description:  "Empty namespace part",
		},
		{
			name:         "empty_name",
			serverName:   "com.example/",
			expectError:  true,
			errorMessage: "non-empty namespace and name parts",
			description:  "Empty name part",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:        tc.serverName,
				Description: tc.description,
				Version:     "1.0.0",
			}

			// Add required fields for valid cases
			if !tc.expectError {
				serverJSON.Repository = model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				}
			}

			err := validators.ValidateServerJSON(&serverJSON)

			if tc.expectError {
				require.Error(t, err, "Expected error for case: %s", tc.description)
				assert.Contains(t, err.Error(), tc.errorMessage,
					"Error message should contain '%s' for case: %s", tc.errorMessage, tc.description)
			} else {
				assert.NoError(t, err, "Expected no error for case: %s", tc.description)
			}
		})
	}
}

// TestMultiSlashValidation_BoundaryConditions tests boundary conditions
func TestMultiSlashValidation_BoundaryConditions(t *testing.T) {
	testCases := []struct {
		name        string
		serverName  string
		expectError bool
		description string
	}{
		// Boundary: exactly one slash (valid)
		{
			name:        "exactly_one_slash",
			serverName:  "a/b",
			expectError: false,
			description: "Minimal valid format with one slash",
		},
		// Boundary: exactly two slashes (invalid)
		{
			name:        "exactly_two_slashes",
			serverName:  "a/b/c",
			expectError: true,
			description: "Minimal invalid format with two slashes",
		},
		// Very long namespace with single slash (valid)
		{
			name:        "very_long_namespace",
			serverName:  strings.Repeat("com.", 50) + "example/server",
			expectError: false,
			description: "Very long namespace but single slash",
		},
		// Very long name with single slash (valid)
		{
			name:        "very_long_name",
			serverName:  "com.example/" + strings.Repeat("server", 50),
			expectError: false,
			description: "Very long server name but single slash",
		},
		// Very long with multiple slashes (invalid)
		{
			name:        "very_long_multi_slash",
			serverName:  "com.example/" + strings.Join(make([]string, 50), "/"),
			expectError: true,
			description: "Very long with multiple slashes",
		},
		// Unicode characters with single slash (valid)
		{
			name:        "unicode_single_slash",
			serverName:  "com.例え/サーバー",
			expectError: false,
			description: "Unicode characters with single slash",
		},
		// Unicode with multiple slashes (invalid)
		{
			name:        "unicode_multi_slash",
			serverName:  "com.例え/サーバー/パス",
			expectError: true,
			description: "Unicode with multiple slashes",
		},
		// Special characters in namespace (valid if single slash)
		{
			name:        "special_chars_namespace",
			serverName:  "com.example-test_123/server",
			expectError: false,
			description: "Special characters in namespace with single slash",
		},
		// Special characters with multi-slash (invalid)
		{
			name:        "special_chars_multi_slash",
			serverName:  "com.example-test/server/path_123",
			expectError: true,
			description: "Special characters with multiple slashes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:        tc.serverName,
				Description: tc.description,
				Version:     "1.0.0",
			}

			if !tc.expectError {
				serverJSON.Repository = model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				}
			}

			err := validators.ValidateServerJSON(&serverJSON)

			if tc.expectError {
				require.Error(t, err, "Expected error for: %s", tc.description)
				assert.Contains(t, err.Error(), "slash", "Error should mention slash for: %s", tc.description)
			} else {
				assert.NoError(t, err, "Expected no error for: %s", tc.description)
			}
		})
	}
}

// TestMultiSlashValidation_SlashCounting tests the slash counting logic
func TestMultiSlashValidation_SlashCounting(t *testing.T) {
	testCases := []struct {
		serverName  string
		slashCount  int
		expectError bool
	}{
		{"no_slash", 0, true},
		{"one/slash", 1, false},
		{"two/slash/es", 2, true},
		{"three/slash/es/here", 3, true},
		{"////", 4, true},
		{"/leading/slash", 2, true},
		{"trailing/slash/", 2, true},
		{"double//slash", 2, true},
		{"a/b/c/d/e/f/g/h/i/j", 9, true},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("slash_count_%d", tc.slashCount), func(t *testing.T) {
			// Verify our test case has the expected number of slashes
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
				require.Error(t, err, "Expected error for %d slashes in '%s'", tc.slashCount, tc.serverName)
			} else {
				assert.NoError(t, err, "Expected no error for %d slash in '%s'", tc.slashCount, tc.serverName)
			}
		})
	}
}

// TestMultiSlashValidation_ErrorMessages tests that error messages are clear and helpful
func TestMultiSlashValidation_ErrorMessages(t *testing.T) {
	testCases := []struct {
		name               string
		serverName         string
		expectedInMessage  []string
		unexpectedInMessage []string
	}{
		{
			name:       "multi_slash_error_message",
			serverName: "com.example/server/path",
			expectedInMessage: []string{
				"server name cannot contain multiple slashes",
				"com.example/server/path",
			},
			unexpectedInMessage: []string{},
		},
		{
			name:       "no_slash_error_message",
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
			unexpectedInMessage: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverJSON := apiv0.ServerJSON{
				Name:    tc.serverName,
				Version: "1.0.0",
			}

			err := validators.ValidateServerJSON(&serverJSON)
			require.Error(t, err, "Expected error for server name: %s", tc.serverName)

			errorMsg := err.Error()
			
			// Check expected substrings
			for _, expected := range tc.expectedInMessage {
				assert.Contains(t, errorMsg, expected,
					"Error message should contain '%s' for server name '%s'", expected, tc.serverName)
			}

			// Check unexpected substrings
			for _, unexpected := range tc.unexpectedInMessage {
				assert.NotContains(t, errorMsg, unexpected,
					"Error message should not contain '%s' for server name '%s'", unexpected, tc.serverName)
			}
		})
	}
}

// TestMultiSlashValidation_WithCompleteServerJSON tests validation with complete ServerJSON objects
func TestMultiSlashValidation_WithCompleteServerJSON(t *testing.T) {
	testCases := []struct {
		name        string
		serverJSON  apiv0.ServerJSON
		expectError bool
		errorPart   string
	}{
		{
			name: "complete_valid_server",
			serverJSON: apiv0.ServerJSON{
				Name:        "com.example/my-server",
				Description: "A fully configured server",
				Version:     "1.2.3",
				Repository: model.Repository{
					URL:    "https://github.com/example/my-server",
					Source: "github",
					ID:     "example/my-server",
				},
				WebsiteURL: "https://example.com/docs",
				Packages: []model.Package{
					{
						Identifier:   "my-package",
						RegistryType: model.RegistryTypeNPM,
						Version:      "1.2.3",
						Transport:    model.Transport{Type: model.TransportTypeStdio},
					},
				},
				Remotes: []model.Transport{
					{
						Type: model.TransportTypeStreamableHTTP,
						URL:  "https://example.com/api",
					},
				},
			},
			expectError: false,
		},
		{
			name: "complete_invalid_multi_slash",
			serverJSON: apiv0.ServerJSON{
				Name:        "com.example/my/server/path",
				Description: "Server with invalid multi-slash name",
				Version:     "1.2.3",
				Repository: model.Repository{
					URL:    "https://github.com/example/my-server",
					Source: "github",
					ID:     "example/my-server",
				},
				Packages: []model.Package{
					{
						Identifier:   "my-package",
						RegistryType: model.RegistryTypeNPM,
						Version:      "1.2.3",
						Transport:    model.Transport{Type: model.TransportTypeStdio},
					},
				},
			},
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
		},
		{
			name: "valid_name_invalid_other_field",
			serverJSON: apiv0.ServerJSON{
				Name:        "com.example/valid-server",
				Description: "Valid name but invalid package",
				Version:     "1.0.0",
				Repository: model.Repository{
					URL:    "https://github.com/example/repo",
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
			expectError: true,
			errorPart:   "package name cannot contain spaces",
		},
		{
			name: "multi_slash_detected_before_other_validations",
			serverJSON: apiv0.ServerJSON{
				Name:        "com.example/server/extra/path",
				Description: "Multi-slash should be caught first",
				Version:     "latest", // Also invalid
				Packages: []model.Package{
					{
						Identifier:   "package with spaces", // Also invalid
						RegistryType: model.RegistryTypeNPM,
						Transport:    model.Transport{Type: model.TransportTypeStdio},
					},
				},
			},
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes", // Should catch this first
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validators.ValidateServerJSON(&tc.serverJSON)

			if tc.expectError {
				require.Error(t, err, "Expected validation error for case: %s", tc.name)
				assert.Contains(t, err.Error(), tc.errorPart,
					"Error should contain '%s' for case: %s", tc.errorPart, tc.name)
			} else {
				assert.NoError(t, err, "Expected no validation error for case: %s", tc.name)
			}
		})
	}
}

// TestMultiSlashValidation_PerformanceWithLargeInputs tests performance with large inputs
func TestMultiSlashValidation_PerformanceWithLargeInputs(t *testing.T) {
	// Create a very long server name with multiple slashes
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
			name:        "many_slashes_performance",
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

			// Validation should complete quickly even with large inputs
			err := validators.ValidateServerJSON(&serverJSON)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "slash")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMultiSlashValidation_RegressionTests ensures old behavior still works
func TestMultiSlashValidation_RegressionTests(t *testing.T) {
	// These test cases ensure that the multi-slash validation doesn't break
	// existing valid server name formats
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
				Description: "Regression test server",
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