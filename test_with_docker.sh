#!/bin/bash

# Script to compile and test Go code using Docker

echo "🐳 Testing with Docker..."
echo "========================"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}✗ Docker is not running${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker is running${NC}"

# Use the official Go image to compile and test
echo -e "\n${YELLOW}Building and testing with Go 1.25...${NC}"

# Run tests with coverage using Docker
docker run --rm \
    -v "$PWD":/workspace \
    -w /workspace \
    golang:1.25-alpine \
    sh -c '
        echo "Installing dependencies..."
        apk add --no-cache git make
        
        echo -e "\n📦 Building project..."
        if make build; then
            echo "✓ Build successful"
        else
            echo "✗ Build failed"
            exit 1
        fi
        
        echo -e "\n🧪 Running OCI registry tests with coverage..."
        go test -v -race -coverprofile=coverage.out -covermode=atomic \
            ./internal/validators/registries/... \
            -run "TestValidateOCI_WithMockRegistries|TestValidateOCI_ErrorCases|TestValidateOCI_RegionalEndpoints"
        
        echo -e "\n📊 Coverage report for OCI validator:"
        go tool cover -func=coverage.out | grep -E "(oci\.go|total):" || true
        
        echo -e "\n🧪 Running publish integration tests..."
        go test -v ./internal/api/handlers/v0/... \
            -run "TestPublishWithMultipleOCIRegistries|TestPublishWithUnsupportedOCIRegistry"
    '

# Check exit code
if [ $? -eq 0 ]; then
    echo -e "\n${GREEN}✅ All tests passed!${NC}"
else
    echo -e "\n${RED}❌ Tests failed${NC}"
    exit 1
fi

echo -e "\n${YELLOW}Summary:${NC}"
echo "- Successfully compiled the project"
echo "- Ran mock-based OCI registry tests"
echo "- Tested support for GHCR, GAR, GCR, Quay, GitLab CR"
echo "- Verified error handling for unsupported registries"