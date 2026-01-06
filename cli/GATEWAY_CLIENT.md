# Gateway Client Usage

The `internal/gateway/client.go` module provides an HTTP client that automatically handles:
- **Authentication** based on gateway's auth type (none, basic, or bearer)
- **Common headers** (Content-Type, Accept)
- **Environment-based credentials** (reads from WSO2AP_GW_* environment variables)

## Authentication Types

The client supports three authentication types configured per gateway:

- **none**: No authentication (for unsecured gateways)
- **basic**: HTTP Basic Auth using `WSO2AP_GW_USERNAME` and `WSO2AP_GW_PASSWORD`
- **bearer**: Bearer token using `WSO2AP_GW_TOKEN`

## How to Use in Future Commands

### Example 1: Using the Active Gateway

```go
package gateway

import (
    "fmt"
    "github.com/wso2/api-platform/cli/internal/gateway"
)

func runSomeCommand() error {
    // Create client for active gateway
    client, err := gateway.NewClientForActive()
    if err != nil {
        return fmt.Errorf("failed to create gateway client: %w", err)
    }

    // Make a GET request
    resp, err := client.Get("/api/v1/apis")
    if err != nil {
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    // Process response...
    return nil
}
```

### Example 2: Using a Specific Gateway by Name

```go
// Create client for a specific gateway
client, err := gateway.NewClientByName("dev")
if err != nil {
    return err
}

// Make requests
resp, err := client.Get("/api/v1/status")
```

### Example 3: POST with JSON Body

```go
import (
    "bytes"
    "encoding/json"
)

client, err := gateway.NewClientForActive()
if err != nil {
    return err
}

// Prepare request body
data := map[string]interface{}{
    "name": "my-api",
    "version": "1.0.0",
}
body, _ := json.Marshal(data)

// Make POST request
resp, err := client.Post("/api/v1/apis", bytes.NewBuffer(body))
if err != nil {
    return err
}
defer resp.Body.Close()

// Check response
if resp.StatusCode != http.StatusCreated {
    return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}
```

### Example 4: Custom Request with Additional Headers

```go
import "net/http"

client, err := gateway.NewClientForActive()
if err != nil {
    return err
}

// Create custom request
req, err := http.NewRequest("PATCH", client.GetBaseURL()+"/api/v1/apis/123", body)
if err != nil {
    return err
}

// Add custom headers
req.Header.Set("X-Custom-Header", "value")

// Use Do() to execute with gateway's auth and settings
resp, err := client.Do(req)
```

## Key Features

### Automatic Authentication
The client automatically applies authentication based on the gateway's `auth` type:

**None (auth: none)**
- No Authorization header added
- Use for unsecured/development gateways

**Basic Auth (auth: basic)**
- Reads `WSO2AP_GW_USERNAME` and `WSO2AP_GW_PASSWORD` from environment
- Adds `Authorization: Basic <credentials>` header
- Fails if environment variables are not set

**Bearer Token (auth: bearer)**
- Reads `WSO2AP_GW_TOKEN` from environment
- Adds `Authorization: Bearer <token>` header
- Fails if environment variable is not set

### Available Methods
- `Get(path string)` - GET request
- `Post(path, body)` - POST request
- `Put(path, body)` - PUT request
- `Delete(path)` - DELETE request
- `Do(req *http.Request)` - Custom request with auth applied

### Helper Functions
- `NewClientForActive()` - Uses active gateway from config
- `NewClientByName(name)` - Uses specific gateway by name
- `NewClient(gateway)` - Uses provided gateway config

## Config File Reference

The client reads gateway settings from `~/.wso2ap/config.yaml`:

```yaml
gateways:
  - name: dev
    server: http://localhost:9090
    auth: none
  - name: staging
    server: https://staging.example.com
    auth: basic
  - name: prod
    server: https://api.example.com
    auth: bearer
activeGateway: dev
```

When you call `NewClientForActive()`, it will use the gateway marked as `activeGateway`.

### Environment Variables

Set the appropriate environment variables based on the gateway's auth type:

```shell
# For basic auth
export WSO2AP_GW_USERNAME=admin
export WSO2AP_GW_PASSWORD=admin

# For bearer auth
export WSO2AP_GW_TOKEN=your_token_here
```
