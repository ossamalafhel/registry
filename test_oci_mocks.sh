#!/bin/bash

echo "🐳 Running OCI mock tests with Docker..."
echo "========================================"

# Run the mock tests specifically
docker run --rm \
    -v "$PWD":/workspace \
    -w /workspace \
    golang:1.25-alpine \
    sh -c '
        apk add --no-cache git make gcc musl-dev
        
        echo "🧪 Running OCI mock tests..."
        CGO_ENABLED=1 go test -v -race -coverprofile=coverage.out \
            ./internal/validators/registries/... \
            -run "TestValidateOCI_WithMockRegistries|TestValidateOCI_ErrorCases|TestValidateOCI_RegionalEndpoints"
        
        echo -e "\n📊 Coverage report:"
        go tool cover -func=coverage.out | grep -E "(oci\.go|total):" || echo "No coverage data"
    '