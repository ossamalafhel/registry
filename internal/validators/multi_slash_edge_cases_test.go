package validators_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/modelcontextprotocol/registry/internal/validators"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiSlashValidation_EdgeCases tests edge cases and boundary conditions
func TestMultiSlashValidation_EdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		serverName  string
		expectError bool
		errorPart   string
		description string
	}{
		// Minimum valid cases
		{
			name:        "minimal_valid",
			serverName:  "a/b",
			expectError: false,
			description: "Minimal valid format with single slash",
		},
		{
			name:        "single_char_namespace",
			serverName:  "x/server",
			expectError: false,
			description: "Single character namespace",
		},
		{
			name:        "single_char_name",
			serverName:  "com.example/y",
			expectError: false,
			description: "Single character server name",
		},

		// Maximum length cases
		{
			name:        "max_length_namespace",
			serverName:  strings.Repeat("a", 253) + "/server",
			expectError: false,
			description: "Maximum length namespace",
		},
		{
			name:        "max_length_name",
			serverName:  "com.example/" + strings.Repeat("b", 253),
			expectError: false,
			description: "Maximum length server name",
		},
		{
			name:        "very_long_with_multi_slash",
			serverName:  strings.Repeat("a", 100) + "/" + strings.Repeat("b", 100) + "/extra",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Very long name with multiple slashes",
		},

		// Slash position variations
		{
			name:        "slash_at_position_1",
			serverName:  "a/bcdefghijk",
			expectError: false,
			description: "Slash at position 1",
		},
		{
			name:        "slash_at_end_minus_1",
			serverName:  "abcdefghij/k",
			expectError: false,
			description: "Slash near the end",
		},
		{
			name:        "consecutive_double_slash",
			serverName:  "com.example//server",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Consecutive double slashes",
		},
		{
			name:        "consecutive_triple_slash",
			serverName:  "com.example///server",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Consecutive triple slashes",
		},
		{
			name:        "spaced_slashes",
			serverName:  "com.example/ /server",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Slashes with space between",
		},

		// Special character combinations
		{
			name:        "dots_and_single_slash",
			serverName:  "com.example.sub.domain/server",
			expectError: false,
			description: "Multiple dots with single slash",
		},
		{
			name:        "dots_and_multi_slash",
			serverName:  "com.example.sub/server/path",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Dots and multiple slashes",
		},
		{
			name:        "hyphen_underscore_single_slash",
			serverName:  "com-example_test/server-name_123",
			expectError: false,
			description: "Hyphens and underscores with single slash",
		},
		{
			name:        "hyphen_underscore_multi_slash",
			serverName:  "com-example/server-name/path_123",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Hyphens and underscores with multiple slashes",
		},

		// Number combinations
		{
			name:        "numeric_namespace",
			serverName:  "123456/server",
			expectError: false,
			description: "Numeric namespace",
		},
		{
			name:        "numeric_name",
			serverName:  "com.example/789012",
			expectError: false,
			description: "Numeric server name",
		},
		{
			name:        "all_numeric_single_slash",
			serverName:  "123/456",
			expectError: false,
			description: "All numeric with single slash",
		},
		{
			name:        "all_numeric_multi_slash",
			serverName:  "123/456/789",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "All numeric with multiple slashes",
		},

		// Mixed case variations
		{
			name:        "mixed_case_single_slash",
			serverName:  "Com.Example/ServerName",
			expectError: false,
			description: "Mixed case with single slash",
		},
		{
			name:        "mixed_case_multi_slash",
			serverName:  "Com.Example/Server/Name",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Mixed case with multiple slashes",
		},
		{
			name:        "all_caps_single_slash",
			serverName:  "COM.EXAMPLE/SERVER",
			expectError: false,
			description: "All caps with single slash",
		},
		{
			name:        "all_caps_multi_slash",
			serverName:  "COM.EXAMPLE/SERVER/PATH",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "All caps with multiple slashes",
		},

		// Empty and whitespace cases
		{
			name:        "empty_string",
			serverName:  "",
			expectError: true,
			errorPart:   "server name is required",
			description: "Empty server name",
		},
		{
			name:        "only_slash",
			serverName:  "/",
			expectError: true,
			errorPart:   "non-empty namespace and name parts",
			description: "Only slash character",
		},
		{
			name:        "only_slashes",
			serverName:  "///",
			expectError: true,
			errorPart:   "non-empty namespace and name parts",
			description: "Only slash characters",
		},
		{
			name:        "whitespace_namespace",
			serverName:  " /server",
			expectError: true,
			errorPart:   "non-empty namespace and name parts",
			description: "Whitespace in namespace",
		},
		{
			name:        "whitespace_name",
			serverName:  "com.example/ ",
			expectError: true,
			errorPart:   "non-empty namespace and name parts",
			description: "Whitespace in name",
		},

		// Path-like patterns
		{
			name:        "absolute_path_like",
			serverName:  "/com/example/server",
			expectError: true,
			errorPart:   "non-empty namespace and name parts",
			description: "Absolute path-like pattern",
		},
		{
			name:        "relative_path_like",
			serverName:  "./com/example",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Relative path-like pattern",
		},
		{
			name:        "parent_path_like",
			serverName:  "../com/example",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Parent path-like pattern",
		},
		{
			name:        "windows_path_like",
			serverName:  "C:/Users/example",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Windows path-like pattern",
		},

		// URL-like patterns
		{
			name:        "http_like",
			serverName:  "http://example.com",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "HTTP URL-like pattern",
		},
		{
			name:        "https_like",
			serverName:  "https://example.com/path",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "HTTPS URL-like pattern",
		},
		{
			name:        "file_protocol_like",
			serverName:  "file:///path/to/file",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "File protocol-like pattern",
		},

		// Package manager patterns
		{
			name:        "npm_scope_valid",
			serverName:  "@scope/package",
			expectError: false,
			description: "NPM scope-like pattern with single slash",
		},
		{
			name:        "npm_scope_invalid",
			serverName:  "@scope/package/version",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "NPM scope with version",
		},
		{
			name:        "maven_like",
			serverName:  "com.example/artifact/version",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Maven-like pattern",
		},
		{
			name:        "go_module_like",
			serverName:  "github.com/user/repo",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Go module-like pattern",
		},

		// Version patterns
		{
			name:        "version_v1",
			serverName:  "com.example/server/v1",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Version v1 pattern",
		},
		{
			name:        "version_v2_0",
			serverName:  "com.example/server/v2.0",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Version v2.0 pattern",
		},
		{
			name:        "semantic_version",
			serverName:  "com.example/server/1.2.3",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Semantic version pattern",
		},
		{
			name:        "date_version",
			serverName:  "com.example/server/2024-01-01",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "Date version pattern",
		},

		// API endpoint patterns
		{
			name:        "api_v1",
			serverName:  "com.example/api/v1",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "API v1 endpoint pattern",
		},
		{
			name:        "rest_endpoint",
			serverName:  "com.example/users/123",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "REST endpoint pattern",
		},
		{
			name:        "graphql_endpoint",
			serverName:  "com.example/graphql/query",
			expectError: true,
			errorPart:   "server name cannot contain multiple slashes",
			description: "GraphQL endpoint pattern",
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
				require.Error(t, err, "Expected error for case: %s", tc.description)
				if tc.errorPart != "" {
					assert.Contains(t, err.Error(), tc.errorPart,
						"Error should contain '%s' for case: %s", tc.errorPart, tc.description)
				}
			} else {
				assert.NoError(t, err, "Expected no error for case: %s", tc.description)
			}
		})
	}
}

// TestMultiSlashValidation_UnicodeEdgeCases tests Unicode-specific edge cases
func TestMultiSlashValidation_UnicodeEdgeCases(t *testing.T) {
	testCases := []struct {
		name        string
		serverName  string
		expectError bool
		description string
	}{
		// Valid Unicode cases
		{
			name:        "emoji_namespace",
			serverName:  "🌍.🏢/server",
			expectError: false,
			description: "Emoji in namespace",
		},
		{
			name:        "emoji_name",
			serverName:  "com.example/🚀",
			expectError: false,
			description: "Emoji as server name",
		},
		{
			name:        "mixed_scripts_valid",
			serverName:  "com.例え混合/サーバーserver",
			expectError: false,
			description: "Mixed scripts with single slash",
		},
		{
			name:        "rtl_ltr_mixed",
			serverName:  "com.مثال/serverخادم",
			expectError: false,
			description: "RTL and LTR text mixed",
		},
		{
			name:        "zero_width_chars",
			serverName:  "com.example​/server", // Contains zero-width space
			expectError: false,
			description: "Zero-width characters",
		},

		// Invalid Unicode cases
		{
			name:        "emoji_multi_slash",
			serverName:  "🌍.🏢/🚀/🌟",
			expectError: true,
			description: "Emoji with multiple slashes",
		},
		{
			name:        "mixed_scripts_invalid",
			serverName:  "com.例え/サーバー/server/path",
			expectError: true,
			description: "Mixed scripts with multiple slashes",
		},
		{
			name:        "combining_chars_multi_slash",
			serverName:  "com.éxample/sérver/páth",
			expectError: true,
			description: "Combining characters with multiple slashes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify the string is valid UTF-8
			assert.True(t, utf8.ValidString(tc.serverName), "Test case should use valid UTF-8")

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
				assert.Contains(t, err.Error(), "slash", "Error should mention slash")
			} else {
				assert.NoError(t, err, "Expected no error for: %s", tc.description)
			}
		})
	}
}

// TestMultiSlashValidation_SlashCountVariations tests various slash count scenarios
func TestMultiSlashValidation_SlashCountVariations(t *testing.T) {
	// Generate test cases for different slash counts
	for slashCount := 0; slashCount <= 10; slashCount++ {
		t.Run(fmt.Sprintf("slash_count_%d", slashCount), func(t *testing.T) {
			// Build server name with exact number of slashes
			parts := make([]string, slashCount+1)
			for i := range parts {
				parts[i] = fmt.Sprintf("part%d", i)
			}
			serverName := strings.Join(parts, "/")

			// For 0 slashes, create a name without slashes
			if slashCount == 0 {
				serverName = "com.example.server"
			}

			serverJSON := apiv0.ServerJSON{
				Name:    serverName,
				Version: "1.0.0",
			}

			// Only single slash is valid
			expectError := slashCount != 1

			if !expectError {
				serverJSON.Repository = model.Repository{
					URL:    "https://github.com/example/repo",
					Source: "github",
				}
			}

			err := validators.ValidateServerJSON(&serverJSON)

			if expectError {
				require.Error(t, err, "Expected error for %d slashes", slashCount)
				if slashCount > 1 {
					assert.Contains(t, err.Error(), "multiple slashes",
						"Error should mention multiple slashes for count %d", slashCount)
				} else if slashCount == 0 {
					assert.Contains(t, err.Error(), "format",
						"Error should mention format for no slashes")
				}
			} else {
				assert.NoError(t, err, "Expected no error for single slash")
			}

			// Verify actual slash count matches expected
			actualCount := strings.Count(serverName, "/")
			assert.Equal(t, slashCount, actualCount, "Slash count mismatch")
		})
	}
}

// TestMultiSlashValidation_CompleteServerJSON tests with fully populated ServerJSON
func TestMultiSlashValidation_CompleteServerJSON(t *testing.T) {
	// Base valid server JSON
	baseServer := apiv0.ServerJSON{
		Description: "Complete server for edge case testing",
		Version:     "1.0.0",
		Repository: model.Repository{
			URL:       "https://github.com/example/repo",
			Source:    "github",
			ID:        "example/repo",
			Subfolder: "src",
		},
		WebsiteURL: "https://example.com/docs",
		Packages: []model.Package{
			{
				Identifier:   "test-package",
				RegistryType: model.RegistryTypeNPM,
				Version:      "1.0.0",
				Transport: model.Transport{
					Type: model.TransportTypeStdio,
				},
				EnvironmentVariables: []model.EnvironmentVariable{
					{
						Name:     "TEST_VAR",
						Value:    "test_value",
						Required: true,
					},
				},
				RuntimeArguments: []model.Argument{
					{
						Type:  model.ArgumentTypeNamed,
						Name:  "--debug",
						Value: "true",
					},
				},
			},
		},
		Remotes: []model.Transport{
			{
				Type: model.TransportTypeStreamableHTTP,
				URL:  "https://example.com/api",
			},
		},
		Meta: &apiv0.ServerMeta{
			PublisherProvided: map[string]interface{}{
				"test": "data",
			},
		},
	}

	testCases := []struct {
		name        string
		serverName  string
		expectError bool
		description string
	}{
		{
			name:        "complete_valid",
			serverName:  "com.example/complete-server",
			expectError: false,
			description: "Complete valid server",
		},
		{
			name:        "complete_invalid_multi_slash",
			serverName:  "com.example/complete/server/path",
			expectError: true,
			description: "Complete server with multi-slash",
		},
		{
			name:        "complete_edge_case_minimal_name",
			serverName:  "a/b",
			expectError: false,
			description: "Complete server with minimal name",
		},
		{
			name:        "complete_edge_case_long_name",
			serverName:  strings.Repeat("x", 200) + "/" + strings.Repeat("y", 200),
			expectError: false,
			description: "Complete server with very long name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := baseServer
			server.Name = tc.serverName

			err := validators.ValidateServerJSON(&server)

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

// TestMultiSlashValidation_ErrorPriority tests that multi-slash error takes priority
func TestMultiSlashValidation_ErrorPriority(t *testing.T) {
	// Server with multiple validation errors
	serverJSON := apiv0.ServerJSON{
		Name:        "com.example/server/extra/path", // Multi-slash error
		Description: "Server with multiple validation issues",
		Version:     "latest", // Also invalid (reserved version)
		Repository: model.Repository{
			URL:    "not-a-valid-url", // Also invalid
			Source: "unknown-source",  // Also invalid
		},
		Packages: []model.Package{
			{
				Identifier:   "package with spaces", // Also invalid
				RegistryType: "invalid-type",        // Also invalid
				Version:      "^1.0.0",              // Also invalid (version range)
				Transport: model.Transport{
					Type: "invalid-transport", // Also invalid
				},
			},
		},
	}

	err := validators.ValidateServerJSON(&serverJSON)
	require.Error(t, err, "Should have validation error")

	// The multi-slash error should be caught first
	assert.Contains(t, err.Error(), "server name cannot contain multiple slashes",
		"Multi-slash validation should be caught first before other errors")

	// Should include the problematic server name
	assert.Contains(t, err.Error(), "com.example/server/extra/path",
		"Error should include the problematic server name")
}

// TestMultiSlashValidation_PerformanceStress stress tests the validation
func TestMultiSlashValidation_PerformanceStress(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	testCases := []struct {
		name        string
		serverName  string
		expectError bool
	}{
		{
			name:        "extreme_length_valid",
			serverName:  strings.Repeat("a", 10000) + "/" + strings.Repeat("b", 10000),
			expectError: false,
		},
		{
			name:        "extreme_length_invalid",
			serverName:  strings.Repeat("a", 10000) + "/" + strings.Repeat("b", 10000) + "/c",
			expectError: true,
		},
		{
			name:        "many_slashes_stress",
			serverName:  strings.Join(make([]string, 1000), "/"),
			expectError: true,
		},
		{
			name:        "deep_nesting",
			serverName:  "com/" + strings.Repeat("sub/", 500) + "server",
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

			// Should handle even extreme inputs gracefully
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