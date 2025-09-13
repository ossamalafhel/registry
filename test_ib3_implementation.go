package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/registry/internal/validators"
	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
	"github.com/modelcontextprotocol/registry/pkg/model"
)

func main() {
	fmt.Println("Testing IB-3: Multi-slash validation with HTTP 400 error")
	fmt.Println("=" + strings.Repeat("=", 70))

	testCases := []struct {
		name        string
		serverName  string
		expectError bool
	}{
		// Valid cases - should not return error
		{
			name:        "Valid single slash",
			serverName:  "com.example/server",
			expectError: false,
		},
		{
			name:        "Valid complex namespace",
			serverName:  "com.company.dept.team/project",
			expectError: false,
		},

		// Invalid cases - should return HTTP 400 with clear error
		{
			name:        "Two slashes",
			serverName:  "com.example/server/extra",
			expectError: true,
		},
		{
			name:        "Three slashes",
			serverName:  "com.example/server/path/deep",
			expectError: true,
		},
		{
			name:        "Consecutive slashes",
			serverName:  "com.example//server",
			expectError: true,
		},
		{
			name:        "Trailing slash",
			serverName:  "com.example/server/",
			expectError: true,
		},
		{
			name:        "Version-like path",
			serverName:  "com.example/server/v1",
			expectError: true,
		},
		{
			name:        "GitHub URL-like",
			serverName:  "github.com/user/repo/releases",
			expectError: true,
		},
	}

	passCount := 0
	failCount := 0

	for _, tc := range testCases {
		fmt.Printf("\nTest: %s\n", tc.name)
		fmt.Printf("  Server name: %s\n", tc.serverName)

		serverJSON := apiv0.ServerJSON{
			Name:        tc.serverName,
			Description: "Test server",
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
			if err != nil {
				// Check that error contains the expected message
				errStr := err.Error()
				if strings.Contains(errStr, "server name cannot contain multiple slashes") ||
					strings.Contains(errStr, validators.ErrMultipleSlashesInServerName.Error()) {
					fmt.Printf("  ✓ Correctly returned error: %s\n", err)
					
					// Check that the error includes the problematic server name
					if strings.Contains(errStr, tc.serverName) {
						fmt.Printf("  ✓ Error message includes server name\n")
					} else {
						fmt.Printf("  ⚠ Error message doesn't include server name\n")
					}
					
					passCount++
				} else {
					fmt.Printf("  ✗ Wrong error message: %s\n", err)
					failCount++
				}
			} else {
				fmt.Printf("  ✗ Expected error but got none\n")
				failCount++
			}
		} else {
			if err == nil {
				fmt.Printf("  ✓ No error as expected\n")
				passCount++
			} else {
				fmt.Printf("  ✗ Unexpected error: %s\n", err)
				failCount++
			}
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 71))
	fmt.Printf("Results: %d passed, %d failed\n", passCount, failCount)

	// Test that this would result in HTTP 400 in the API
	fmt.Println("\n" + strings.Repeat("=", 71))
	fmt.Println("Simulating API behavior:")
	
	// Create a server with multiple slashes
	invalidServer := apiv0.ServerJSON{
		Name:        "com.example/server/extra/path",
		Description: "Invalid multi-slash server",
		Version:     "1.0.0",
	}

	err := validators.ValidateServerJSON(&invalidServer)
	if err != nil {
		fmt.Printf("✓ Validation error: %s\n", err)
		fmt.Println("✓ In the API, this would return HTTP 400 with message:")
		fmt.Printf("  'Failed to publish server: %s'\n", err)
	} else {
		fmt.Println("✗ Validation should have failed!")
		os.Exit(1)
	}

	if failCount > 0 {
		os.Exit(1)
	}

	fmt.Println("\n✓ All tests passed! IB-3 implementation is working correctly.")
}