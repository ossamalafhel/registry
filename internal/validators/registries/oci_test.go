package registries_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/registry/internal/validators/registries"
	"github.com/modelcontextprotocol/registry/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestValidateOCI_RealPackages(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		packageName  string
		version      string
		serverName   string
		expectError  bool
		errorMessage string
	}{
		{
			name:         "non-existent image should fail",
			packageName:  generateRandomImageName(),
			version:      "latest",
			serverName:   "com.example/test",
			expectError:  true,
			errorMessage: "not found",
		},
		{
			name:         "real image without MCP annotation should fail",
			packageName:  "nginx", // Popular image without MCP annotation
			version:      "latest",
			serverName:   "com.example/test",
			expectError:  true,
			errorMessage: "missing required annotation",
		},
		{
			name:         "real image with specific tag without MCP annotation should fail",
			packageName:  "redis",
			version:      "7-alpine", // Specific tag
			serverName:   "com.example/test",
			expectError:  true,
			errorMessage: "missing required annotation",
		},
		{
			name:         "namespaced image without MCP annotation should fail",
			packageName:  "hello-world", // Simple image for testing
			version:      "latest",
			serverName:   "com.example/test",
			expectError:  true,
			errorMessage: "missing required annotation",
		},
		{
			name:        "real image with correct MCP annotation should pass",
			packageName: "domdomegg/airtable-mcp-server",
			version:     "1.7.2",
			serverName:  "io.github.domdomegg/airtable-mcp-server", // This should match the annotation
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping OCI registry tests because we keep hitting DockerHub rate limits")

			pkg := model.Package{
				RegistryType: model.RegistryTypeOCI,
				Identifier:   tt.packageName,
				Version:      tt.version,
			}

			err := registries.ValidateOCI(ctx, pkg, tt.serverName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateOCI_MultipleRegistries(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		pkg             model.Package
		serverName      string
		expectError     bool
		errorContains   string
	}{
		{
			name: "GHCR public image without MCP annotation should fail",
			pkg: model.Package{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: model.RegistryURLGHCR,
				Identifier:      "actions/runner",
				Version:         "latest",
			},
			serverName:    "com.example/test",
			expectError:   true,
			errorContains: "missing required annotation",
		},
		{
			name: "GAR with regional endpoint should be supported",
			pkg: model.Package{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: "https://us-central1-docker.pkg.dev",
				Identifier:      "google-samples/containers/gke/hello-app",
				Version:         "1.0",
			},
			serverName:    "com.example/test",
			expectError:   true,
			errorContains: "missing required annotation",
		},
		{
			name: "Quay.io public image should be supported",
			pkg: model.Package{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: model.RegistryURLQuay,
				Identifier:      "coreos/etcd",
				Version:         "latest",
			},
			serverName:    "com.example/test",
			expectError:   true,
			errorContains: "missing required annotation",
		},
		{
			name: "GCR with regional endpoint should be supported",
			pkg: model.Package{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: "https://us.gcr.io",
				Identifier:      "google-containers/pause",
				Version:         "3.1",
			},
			serverName:    "com.example/test",
			expectError:   true,
			errorContains: "missing required annotation",
		},
		{
			name: "GitLab Container Registry should be supported",
			pkg: model.Package{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: model.RegistryURLGitLabCR,
				Identifier:      "gitlab-org/cluster-integration/auto-build-image",
				Version:         "latest",
			},
			serverName:    "com.example/test",
			expectError:   true,
			errorContains: "missing required annotation",
		},
		{
			name: "Unsupported registry should fail with clear error",
			pkg: model.Package{
				RegistryType:    model.RegistryTypeOCI,
				RegistryBaseURL: "https://unsupported.registry.com",
				Identifier:      "test/image",
				Version:         "latest",
			},
			serverName:    "com.example/test",
			expectError:   true,
			errorContains: "unsupported OCI registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Skipping multi-registry OCI tests to avoid rate limits")

			err := registries.ValidateOCI(ctx, tt.pkg, tt.serverName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

