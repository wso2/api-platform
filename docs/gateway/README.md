# API Platform Gateway

A complete API gateway system for managing, securing, and routing API traffic to your backend services.

## Quick Start

For step-by-step instructions on setting up and running the gateway using Docker Compose, see the [Quick Start Guide](quick-start-guide.md).

## Components

| Component | Purpose |
|-----------|---------|
| **Gateway-Controller** | Control plane that manages API configurations and dynamically configures the Router |
| **Router** | Data plane (Envoy Proxy) that routes HTTP/HTTPS traffic to backend services |
| **Policy Engine** | Processes requests/responses through configurable policies (authentication, rate limiting, etc.) |
| **Policy Builder** | Build-time tooling for compiling custom policy implementations |

### CLI Tool (ap)

The `ap` CLI provides a command-line interface for managing gateways, APIs, and MCP proxies. Key capabilities include:

- Gateway management (add, list, remove, health check)
- API lifecycle management (apply, list, get, delete)
- MCP proxy management (generate, list, get, delete)

For the complete list of CLI commands and usage examples, see the [CLI Reference](../cli/reference.md).

## Default Ports

| Port | Service | Description |
|------|---------|-------------|
| 8080 | Router | HTTP traffic |
| 8443 | Router | HTTPS traffic |
| 9090 | Gateway-Controller | REST API |

## Architecture

```
User → Gateway-Controller (REST API)
         ↓ (validates & persists config)
         ↓
       Router (Envoy Proxy) → Backend Services
```

**How it works:**

1. User submits API configuration (YAML/JSON) to Gateway-Controller
2. Gateway-Controller validates and persists the configuration
3. Router receives the updated configuration and starts routing traffic

## Documentation

| Section | Description |
|---------|-------------|
| [policies/](policies/) | Authentication policies (JWT, API Key) |
| [mcp/](mcp/) | MCP proxy setup and policies |
| [observability/](observability/) | Logging and tracing configuration |
| [analytics/](analytics/) | Analytics integrations (Moesif) |
| [gateway-rest-api/](gateway-rest-api/) | REST API authentication and usage |
