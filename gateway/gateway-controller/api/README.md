# API Specifications

This directory contains OpenAPI specifications for the Gateway Controller API.

## Files

- **`openapi.yaml`** - Original REST API for managing API configurations (HTTP/REST only)
- **`asyncapi-openapi.yaml`** - Enhanced REST API that supports both HTTP/REST and Async API configurations (WebSub, WebSocket, SSE)

## Generating Code

### Generate from original OpenAPI spec (REST only)
```bash
make generate
```

### Generate from enhanced spec (REST + Async support)
```bash
make generate-websub
```

This generates:
- Type definitions for API configurations
- Gin server interface
- Router registration code
- Embedded OpenAPI spec

Output: `pkg/api/generated/async_generated.go`

## API Configuration Examples

### REST API Configuration
```yaml
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: weather-api
  version: v1.0
  context: /weather
  upstream:
    - url: https://api.weather.com/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: POST
      path: /{country_code}/{city}
```

### WebSub Async API Configuration
```yaml
version: api-platform.wso2.com/v1
kind: http/websub
data:
  name: github-webhooks
  version: v1.0
  context: /github-events
  upstream:
    - url: http://localhost:9098
  operations:
    - method: SUBSCRIBE
      path: /issues
      operation_type: receive
      description: Subscribe to GitHub issue events
    - method: SUBSCRIBE
      path: /pull-requests
      operation_type: receive
      description: Subscribe to GitHub PR events
  asyncapi:
    protocol: websub
    channels:
      /issues:
        description: GitHub issue events
        subscribe:
          summary: Receive issue events
          message:
            name: IssueEvent
            content_type: application/json
            payload:
              type: object
              properties:
                action:
                  type: string
                issue:
                  type: object
      /pull-requests:
        description: GitHub pull request events
        subscribe:
          summary: Receive PR events
          message:
            name: PullRequestEvent
            content_type: application/json
            payload:
              type: object
              properties:
                action:
                  type: string
                pull_request:
                  type: object
    subscriptions:
      callback_url: https://myapp.example.com/webhooks
      lease_seconds: 86400
      secret: my-webhook-secret
```

## Supported API Kinds

| Kind | Description | Protocol |
|------|-------------|----------|
| `http/rest` | Traditional REST APIs | HTTP/HTTPS |
| `http/websub` | WebSub (PubSubHubbub) event APIs | HTTP + WebSub |
| `async/websocket` | WebSocket streaming APIs | WebSocket |
| `async/sse` | Server-Sent Events APIs | HTTP + SSE |

## Operation Methods

### REST APIs
- `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `HEAD`, `OPTIONS`

### Async APIs
- `SUBSCRIBE` - Subscribe to a topic/channel
- `PUBLISH` - Publish to a topic/channel

## Async Operation Types

- `send` - One-way send (fire-and-forget)
- `receive` - One-way receive (subscribe to events)
- `request` - Request-reply pattern (send and wait for response)
- `reply` - Reply pattern (respond to requests)

## Async Protocols

- `websub` - WebSub (W3C standard for webhooks)
- `websocket` - WebSocket bidirectional streaming
- `sse` - Server-Sent Events (one-way server push)
- `mqtt` - MQTT message broker
- `kafka` - Apache Kafka streaming

## Development Workflow

1. **Edit the OpenAPI spec** (`asyncapi-openapi.yaml`)
2. **Regenerate Go code**: `make generate-websub`
3. **Implement handlers** in `pkg/api/handlers.go`
4. **Build**: `make build`
5. **Test**: `make test`

## Notes

- The generated code in `pkg/api/generated/` should **never be edited manually**
- All changes must be made to the OpenAPI spec and regenerated
- The spec is embedded in the binary for runtime API documentation
- Use `GET /openapi.json` or similar endpoints to retrieve the embedded spec at runtime
