#!/bin/bash

echo "📊 Generating detailed coverage report..."

docker run --rm \
    -v "$PWD":/workspace \
    -w /workspace \
    golang:1.25-alpine \
    sh -c '
        apk add --no-cache git make gcc musl-dev
        
        # Run tests with coverage
        CGO_ENABLED=1 go test -coverprofile=coverage.out \
            -covermode=atomic ./internal/validators/registries/...
        
        # Show uncovered lines for oci.go
        echo "🔍 Uncovered lines in oci.go:"
        go tool cover -func=coverage.out | grep -A50 "oci.go" | grep -E "0.0%|[0-9]+.[0-9]+%" | head -20
        
        # Generate text coverage report
        go tool cover -func=coverage.out > coverage.txt
        
        echo -e "\n📋 Coverage saved to coverage.txt"
    '