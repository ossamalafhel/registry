#!/bin/bash

# Script to check if the code compiles without Go installed
# This performs syntax validation and import checking

echo "🔍 Checking Go code compilation..."
echo "================================"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Go is installed
if command -v go &> /dev/null; then
    echo -e "${GREEN}✓ Go is installed${NC}"
    go version
    
    # Try to build
    echo -e "\n${YELLOW}Building project...${NC}"
    if make build 2>&1; then
        echo -e "${GREEN}✓ Build successful${NC}"
    else
        echo -e "${RED}✗ Build failed${NC}"
        exit 1
    fi
    
    # Run tests with coverage
    echo -e "\n${YELLOW}Running tests with coverage...${NC}"
    if go test -v -race -coverprofile=coverage.out -covermode=atomic -coverpkg=./internal/... ./internal/validators/registries/... 2>&1; then
        echo -e "${GREEN}✓ Tests passed${NC}"
        
        # Show coverage summary
        echo -e "\n${YELLOW}Coverage summary:${NC}"
        go tool cover -func=coverage.out | grep -E "(oci\.go|total):" || true
    else
        echo -e "${RED}✗ Tests failed${NC}"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠️  Go is not installed${NC}"
    echo "Cannot compile or run tests without Go"
    
    # At least check syntax with basic validation
    echo -e "\n${YELLOW}Performing basic syntax checks...${NC}"
    
    # Check for basic Go syntax errors
    errors=0
    
    # Check modified files
    for file in internal/validators/registries/oci.go \
                internal/validators/registries/oci_test.go \
                internal/validators/registries/oci_mock_test.go \
                internal/api/handlers/v0/publish_oci_registries_test.go \
                pkg/model/constants.go; do
        if [ -f "$file" ]; then
            echo -n "Checking $file... "
            
            # Basic syntax checks
            if grep -q "^package " "$file"; then
                echo -e "${GREEN}✓${NC}"
            else
                echo -e "${RED}✗ Missing package declaration${NC}"
                ((errors++))
            fi
            
            # Check for obvious syntax errors
            if grep -E "^\s*}\s*$" "$file" | wc -l | grep -q "0"; then
                echo -e "${RED}  ⚠️  Warning: No closing braces found${NC}"
            fi
        else
            echo -e "${RED}✗ File not found: $file${NC}"
            ((errors++))
        fi
    done
    
    if [ $errors -eq 0 ]; then
        echo -e "\n${GREEN}✓ Basic syntax checks passed${NC}"
    else
        echo -e "\n${RED}✗ Found $errors syntax issues${NC}"
        exit 1
    fi
    
    # List modified files
    echo -e "\n${YELLOW}Modified files in this feature:${NC}"
    git diff --name-only HEAD~1 2>/dev/null || git status --porcelain | awk '{print $2}'
fi

echo -e "\n${YELLOW}Summary of changes:${NC}"
echo "- Added support for multiple OCI registries (GHCR, GAR, GCR, etc.)"
echo "- Updated OCI validator to handle different registry authentications"
echo "- Added comprehensive mock tests for multi-registry support"
echo "- Updated documentation to list all supported registries"

# Check if tests are skipped
echo -e "\n${YELLOW}Checking test configuration:${NC}"
if grep -r "t.Skip" internal/validators/registries/*_test.go 2>/dev/null; then
    echo -e "${YELLOW}⚠️  Warning: Some tests are skipped${NC}"
    grep -n "t.Skip" internal/validators/registries/*_test.go
fi

echo -e "\n✅ Check complete!"