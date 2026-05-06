# API Platform Gateway

A complete API gateway system for managing, securing, and routing API traffic to your backend services.

## Quick Start

For step-by-step instructions on setting up and running the gateway using Docker Compose, including verifying the Gateway Controller admin health endpoint and deploying a sample REST API via `POST /api/management/v0.9/rest-apis`, see the [Quick Start Guide](quick-start-guide.md).

## Components

| Component | Purpose |
|-----------|---------|
| **Gateway-Controller** | Control plane that manages API configurations and dynamically configures the Router |
| **Gateway-Runtime** | Data plane (Envoy Proxy) that routes HTTP/HTTPS traffic to backend services and Processes requests/responses through configurable policies (authentication, rate limiting, etc.)|
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
| 9094 | Gateway-Controller Admin | Health and admin endpoints |

## Architecture

```
User → Gateway-Controller (REST API)
         ↓
       Gateway-Controller Admin (/health)
         ↓ (validates & persists config)
         ↓
       Router (Envoy Proxy) → Backend Services
```

**How it works:**

1. User verifies the Gateway-Controller admin health endpoint
2. User submits API configuration (YAML/JSON) to the Gateway-Controller REST API
3. Gateway-Controller validates and persists the configuration
4. Router receives the updated configuration and starts routing traffic

## Policies

Policies allow you to intercept and transform API traffic at the Gateway-Runtime (Envoy Proxy). They can be applied to request/response flows to handle concerns like authentication, rate limiting, header manipulation, and more.

The complete and up-to-date policy catalogue — with configuration references and examples — is maintained in the gateway-controllers repository: https://github.com/wso2/gateway-controllers/blob/main/docs/README.md

You can extend the gateway with your own policies or include specific policies from the catalogue by building a custom gateway image using the `ap` CLI. See [Customizing the Gateway by Adding and Removing Policies](../cli/customizing-gateway-policies.md).

## Documentation

| Section | Description |
|---------|-------------|
| [Kubernetes](kubernetes/) | Kubernetes Gateway Operator deployment |
| [MCP](mcp/) | MCP proxy setup and policies |
| [Observability](observability/) | Logging, metrics, and tracing configuration |
| [Resiliency](resiliency/) | Gateway resiliency features (timeouts, failure handling) |
| [Analytics](analytics/) | Analytics integrations (Moesif) |
| [REST APIs](../rest-apis/gateway/) | REST API authentication and usage |
| [Policies and Guardrails](https://github.com/wso2/gateway-controllers/blob/main/docs/README.md) | Gateway policies and guardrails for API traffic control |
| [Policy Languages and Runtimes](policy-languages-and-runtimes.md) | Dual-language policy development guide (Go and Python) |
| [Immutable Gateway](immutable-gateway.md) | File-based, GitOps-native gateway configuration |
