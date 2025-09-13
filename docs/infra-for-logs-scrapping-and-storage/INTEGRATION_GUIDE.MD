# 🚀 Server Name Validation - Integration Guide

## 🎯 What's New

The MCP Registry now has **enhanced server name validation** that ensures consistent, clear naming conventions across all published servers! This update brings stricter validation rules that prevent confusing multi-slash formats while maintaining backward compatibility with existing valid server names.

### ✨ Key Benefits
- **Crystal Clear Naming**: No more confusion with path-like server names
- **Instant Feedback**: Get helpful error messages immediately when validation fails
- **Consistent Format**: All servers follow the standard `namespace/name` convention
- **Better Organization**: Cleaner registry with predictable naming patterns

## ⚡ Quick Start

### Valid Server Name Format
```
namespace/server-name
```

### Examples of Valid Names
```json
{
  "name": "com.example/my-server",
  "name": "io.github.username/awesome-mcp",
  "name": "org.company.team/data-processor"
}
```

### Examples of Invalid Names (Will Be Rejected)
```json
{
  "name": "com.example/server/v1",      // ❌ Multiple slashes
  "name": "com.example//server",        // ❌ Consecutive slashes
  "name": "github.com/user/repo/path",  // ❌ Path-like structure
}
```

## 🎨 For End Users

### Step 1: Check Your Server Name

Before publishing, ensure your server name follows the correct format:

✅ **Correct Format**
- One namespace part (reverse DNS style)
- Exactly one forward slash (`/`)
- One server name part

❌ **Incorrect Formats**
- Multiple slashes anywhere in the name
- Trailing slashes
- Empty segments between slashes

### Step 2: Understanding Error Messages

If your server name has multiple slashes, you'll receive a clear error:

```json
{
  "status": 400,
  "title": "Bad Request",
  "detail": "Failed to publish server: server name cannot contain multiple slashes: com.example/server/extra"
}
```

### Step 3: Fix Your Server Name

Simply remove extra path segments:

**Before:** `com.example/api/v1/server`  
**After:** `com.example/api-v1-server`

**Before:** `io.github.user/project/module`  
**After:** `io.github.user/project-module`

## 🚀 For Developers

### Publishing a Server Programmatically

```go
package main

import (
    "github.com/modelcontextprotocol/registry/pkg/api/v0"
    "github.com/modelcontextprotocol/registry/pkg/model"
)

func publishServer() error {
    server := apiv0.ServerJSON{
        // ✅ Valid: Single slash format
        Name:        "com.example/my-awesome-server",
        Description: "An amazing MCP server",
        Version:     "1.0.0",
        Repository: model.Repository{
            URL:    "https://github.com/example/my-awesome-server",
            Source: "github",
        },
    }
    
    // Publish to registry
    return registry.Publish(server)
}
```

### Validation in Your Code

```go
import "github.com/modelcontextprotocol/registry/internal/validators"

func validateServerName(name string) error {
    server := apiv0.ServerJSON{
        Name: name,
        // ... other required fields
    }
    
    if err := validators.ValidateServerJSON(&server); err != nil {
        // Handle validation error
        return fmt.Errorf("invalid server name: %w", err)
    }
    
    return nil
}
```

### Using the CLI Publisher

```bash
# ✅ Valid
./bin/mcp-publisher publish \
  --name "com.example/weather-service" \
  --version "2.1.0"

# ❌ Invalid - will return error
./bin/mcp-publisher publish \
  --name "com.example/api/v2/weather" \
  --version "2.1.0"
```

## ⚙️ Configuration

No configuration changes needed! The validation is automatically applied to all publish requests.

Dependencies have been added to the project configuration.

## 🧪 Test Drive

### Quick Validation Test

Try these curl commands to see the validation in action:

```bash
# Valid server name - should succeed
curl -X POST https://registry.modelcontextprotocol.io/v0/publish \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "com.example/test-server",
    "description": "Test server",
    "version": "1.0.0"
  }'

# Invalid server name - will return HTTP 400
curl -X POST https://registry.modelcontextprotocol.io/v0/publish \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "com.example/test/server/v1",
    "description": "Test server",
    "version": "1.0.0"
  }'
```

### Unit Test Example

```go
func TestServerNameValidation(t *testing.T) {
    testCases := []struct {
        name        string
        serverName  string
        shouldError bool
    }{
        {"valid single slash", "com.example/server", false},
        {"invalid multiple slashes", "com.example/a/b", true},
        {"invalid trailing slash", "com.example/server/", true},
        {"invalid consecutive slashes", "com.example//server", true},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            server := apiv0.ServerJSON{Name: tc.serverName}
            err := validators.ValidateServerJSON(&server)
            
            if tc.shouldError && err == nil {
                t.Errorf("expected error for %s", tc.serverName)
            }
            if !tc.shouldError && err != nil {
                t.Errorf("unexpected error for %s: %v", tc.serverName, err)
            }
        })
    }
}
```

## 💡 Pro Tips

### 1. **Namespace Best Practices**
   - Use reverse DNS format: `com.company.team`
   - Keep it hierarchical and meaningful
   - Avoid overly long namespaces

### 2. **Server Name Best Practices**
   - Use kebab-case: `my-awesome-server`
   - Be descriptive but concise
   - Include version info in the version field, not the name

### 3. **Migration Strategy**
   If you have existing servers with multi-slash names:
   - Create a new entry with the corrected name
   - Update clients to use the new name
   - Mark the old entry as deprecated

### 4. **Validation Performance**
   The validation is lightning fast - it runs in microseconds and won't impact your CI/CD pipeline.

### 5. **Error Handling**
   Always check for validation errors before attempting to publish:
   ```go
   if err := validators.ValidateServerJSON(&server); err != nil {
       // Log the specific validation error
       log.Printf("Validation failed: %v", err)
       // Return user-friendly message
       return fmt.Errorf("please check your server configuration")
   }
   ```

## 🆘 Need Help?

### Common Issues and Solutions

#### Issue: "server name cannot contain multiple slashes"
**Solution:** Remove extra path segments from your server name. Use only one slash between namespace and name.

#### Issue: "server name must be in format 'dns-namespace/name'"
**Solution:** Ensure you have exactly one slash separating the namespace from the server name.

#### Issue: Migration from old naming convention
**Solution:** 
1. Publish with new name format
2. Update all references in your documentation
3. Notify users of the name change

### Error Message Reference

| Error | Meaning | Fix |
|-------|---------|-----|
| `cannot contain multiple slashes` | Server name has 2+ slashes | Use format: `namespace/name` |
| `non-empty namespace and name parts` | Missing namespace or name | Provide both parts: `namespace/name` |
| `must be in format 'dns-namespace/name'` | Invalid format | Follow: `com.example/server` pattern |

### Getting Support

- 📖 Check the [full documentation](../README.md)
- 💬 Join the [Discord community](https://modelcontextprotocol.io/community/communication)
- 🐛 Report issues on [GitHub](https://github.com/modelcontextprotocol/registry/issues)
- 💡 Discuss ideas in [GitHub Discussions](https://github.com/modelcontextprotocol/registry/discussions)

---
*Powered by [Kanpredict](https://kanpredict.com) - Premium AI Development Platform*