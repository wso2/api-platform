# Gateway Client Usage

The `internal/gateway/client.go` module provides an HTTP client that automatically handles:
- **Authentication tokens** (Bearer token added to Authorization header)
- **TLS verification** (skipped if gateway was added with `--insecure`)
- **Common headers** (Content-Type, Accept)

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

### Automatic Token Injection
If the gateway was added with a `--token` flag, the client automatically adds it to all requests:
```
Authorization: Bearer <token>
```

### TLS Configuration
If the gateway was added with `--insecure`, the client skips TLS certificate verification. This is useful for:
- Development environments with self-signed certificates
- Local testing
- Internal networks

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

The client reads gateway settings from `~/.ap/config.yaml`:

```yaml
gateways:
  - name: dev
    url: http://localhost:9090
  - name: prod
    url: https://api.example.com
    token: SECRET_TOKEN
    insecure: true
activeGateway: dev
configVersion: 1.0.0
```

When you call `NewClientForActive()`, it will use the gateway marked as `activeGateway`.
