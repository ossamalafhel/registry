#!/bin/bash

echo "🎯 Running all OCI tests for 100% coverage..."
echo "==========================================="

docker run --rm \
    -v "$PWD":/workspace \
    -w /workspace \
    golang:1.25-alpine \
    sh -c '
        apk add --no-cache git make gcc musl-dev
        
        echo "🧪 Running all OCI validator tests..."
        CGO_ENABLED=1 go test -v -race -coverprofile=coverage.out \
            -covermode=atomic ./internal/validators/registries/...
        
        echo -e "\n📊 Full coverage report:"
        go tool cover -func=coverage.out | grep -E "(\.go:|total:)"
        
        echo -e "\n🎯 Detailed OCI validator coverage:"
        go tool cover -func=coverage.out | grep "oci.go" || echo "No oci.go coverage data"
        
        # Generate HTML report
        go tool cover -html=coverage.out -o coverage.html
        echo -e "\n📄 HTML coverage report generated: coverage.html"
    '