package registries_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/registry/internal/validators/registries"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetAPIBaseURL_AdditionalCoverage tests uncovered paths in getAPIBaseURL
func TestGetAPIBaseURL_AdditionalCoverage(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name            string
		registryBaseURL string
		shouldError     bool
		expectedError   string
	}{
		{
			name:            "GAR regional endpoint us-west1",
			registryBaseURL: "https://us-west1-docker.pkg.dev",
			shouldError:     false,
		},
		{
			name:            "GAR regional endpoint asia-southeast1",
			registryBaseURL: "https://asia-southeast1-docker.pkg.dev",
			shouldError:     false,
		},
		{
			name:            "GCR regional endpoint eu.gcr.io",
			registryBaseURL: "https://eu.gcr.io",
			shouldError:     false,
		},
		{
			name:            "ECR regional endpoint us-east-1",
			registryBaseURL: "https://123456789012.dkr.ecr.us-east-1.amazonaws.com",
			shouldError:     false,
		},
		{
			name:            "ACR custom instance",
			registryBaseURL: "https://mycompany.azurecr.io",
			shouldError:     false,
		},
		{
			name:            "Localhost with port 5000",
			registryBaseURL: "http://localhost:5000",
			shouldError:     false,
		},
		{
			name:            "127.0.0.1 with port 8080",
			registryBaseURL: "http://127.0.0.1:8080",
			shouldError:     false,
		},
		{
			name:            "Unsupported registry with special characters",
			registryBaseURL: "https://not-supported-registry!@#$.com",
			shouldError:     true,
			expectedError:   "unsupported OCI registry",
		},
		{
			name:            "Empty registry URL should default to Docker",
			registryBaseURL: "",
			shouldError:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pkg := model.Package{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: tc.registryBaseURL,
				Identifier:      "test/image",
				Version:         "latest",
			}

			err := registries.ValidateOCI(ctx, pkg, "com.example/test")
			
			if tc.shouldError {
				require.Error(t, err)
				if tc.expectedError != "" {
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			} else {
				// These will fail to connect to the registry, but that's expected
				// We're just testing that the URL is accepted
				require.Error(t, err)
				// Should not be an "unsupported registry" error
				assert.NotContains(t, err.Error(), "unsupported OCI registry")
			}
		})
	}
}

// TestValidateOCI_EdgeCases tests additional edge cases
func TestValidateOCI_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("Very long namespace and repo names", func(t *testing.T) {
		pkg := model.Package{
			RegistryType:    model.RegistryTypeOCI,
			RegistryBaseURL: model.RegistryURLDocker,
			Identifier:      "verylongnamespacethatexceedsnormallengthlimits/verylongrepothatexceedsnormallengthlimitsaswell",
			Version:         "latest",
		}

		err := registries.ValidateOCI(ctx, pkg, "com.example/test")
		require.Error(t, err)
		// Should fail to connect, not because of parsing
		assert.NotContains(t, err.Error(), "invalid image reference")
	})

	t.Run("Special characters in image tag", func(t *testing.T) {
		pkg := model.Package{
			RegistryType:    model.RegistryTypeOCI,
			RegistryBaseURL: model.RegistryURLDocker,
			Identifier:      "test/repo",
			Version:         "v1.0.0-alpha+build.123",
		}

		err := registries.ValidateOCI(ctx, pkg, "com.example/test")
		require.Error(t, err)
		// Should fail to connect, not because of parsing
		assert.NotContains(t, err.Error(), "invalid")
	})

	t.Run("Registry URL with path", func(t *testing.T) {
		pkg := model.Package{
			RegistryType:    model.RegistryTypeOCI,
			RegistryBaseURL: "https://myregistry.com/v2",
			Identifier:      "test/repo",
			Version:         "latest",
		}

		err := registries.ValidateOCI(ctx, pkg, "com.example/test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported OCI registry")
	})

	t.Run("Registry URL with subdomain", func(t *testing.T) {
		pkg := model.Package{
			RegistryType:    model.RegistryTypeOCI,
			RegistryBaseURL: "https://docker.mycompany.com",
			Identifier:      "test/repo",
			Version:         "latest",
		}

		err := registries.ValidateOCI(ctx, pkg, "com.example/test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported OCI registry")
	})
}

// TestValidateOCI_AllRegistryConstants tests all registry constants are handled
func TestValidateOCI_AllRegistryConstants(t *testing.T) {
	ctx := context.Background()

	// Test all registry constants to ensure they're properly handled
	registryConstants := []string{
		model.RegistryURLDocker,
		model.RegistryURLGHCR,
		model.RegistryURLGAR,
		model.RegistryURLGCR,
		model.RegistryURLECR,
		model.RegistryURLACR,
		model.RegistryURLQuay,
		model.RegistryURLGitLabCR,
		model.RegistryURLDockerHub,
		model.RegistryURLJFrogCR,
		model.RegistryURLHarborCR,
		model.RegistryURLAlibabaACR,
		model.RegistryURLIBMCR,
		model.RegistryURLOracleCR,
		model.RegistryURLDigitalOceanCR,
	}

	for _, registryURL := range registryConstants {
		t.Run(registryURL, func(t *testing.T) {
			pkg := model.Package{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: registryURL,
				Identifier:      "test/repo",
				Version:         "latest",
			}

			err := registries.ValidateOCI(ctx, pkg, "com.example/test")
			require.Error(t, err)
			// Should not be an "unsupported registry" error
			assert.NotContains(t, err.Error(), "unsupported OCI registry")
		})
	}
}