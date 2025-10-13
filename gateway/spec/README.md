# API Gateway - Project Specification

## Overview

The API Gateway is a two-component system consisting of **Gateway-Controller** (Go-based xDS server) and **Router** (Envoy Proxy). Users submit REST API configurations in YAML/JSON format to the Gateway-Controller, which validates, persists, and dynamically configures the Router via xDS protocol. The Router forwards HTTP traffic to backend services according to these configurations.

## Key Components

1. **Gateway Controller** - Go-based xDS server that manages API configurations and dynamically configures the Router
2. **Router** - Envoy Proxy-based component that handles traffic routing and request forwarding
3. **Policy Engine** - Executes policy decisions and enforcement (Standard tier)
4. **Rate Limiter** - Provides advanced rate limiting capabilities (Standard tier only)

## Quick Links

- [Product Requirements](./prd.md)
- [Architecture](./architecture/architecture.md)
- [Design Specifications](./design/design.md)
- [Implementation Guide](./impl.md)
- [Use Cases](./use-cases/use_cases.md)

## Technology Stack

### Gateway Controller
- **Language**: Go 1.25.1
- **Framework**: Gin (REST API)
- **xDS Implementation**: go-control-plane v0.13.0+
- **Storage**: bbolt (embedded key-value store)
- **Logging**: Zap (structured logging)
- **Code Generation**: oapi-codegen v2 (OpenAPI to Go server)

### Router
- **Base**: Envoy Proxy 1.35.3
- **Protocol**: xDS v3 (State-of-the-World)
- **Configuration**: Bootstrap YAML with xds_cluster
- **Logging**: Structured JSON access logs

### Persistence Layer
- **bbolt**: Configuration persistence in Gateway-Controller
- **Redis**: Distributed cache for rate limiting (Standard tier)
- **SQLite**: Gateway artifacts storage (Standard tier, can be switched to external DB)

## Gateway Tiers

### Basic Gateway
- Router + Policy Engine + Gateway Controller
- In-memory configuration (no persistence)
- Basic rate limiting (built into Router)
- Best for: Development, testing, free tier (14-day trial)

### Standard Gateway
- All Basic components + Rate Limiter + Redis + SQLite
- Advanced distributed rate limiting
- Persistent storage for APIs, applications, subscriptions
- External DB support (PostgreSQL, Oracle, MySQL, MSSQL)
- Best for: Production, enterprise deployments, paid tier

## Project Goals

- **Dynamic Configuration**: Zero-downtime API configuration updates via xDS protocol
- **Policy-First Architecture**: Everything beyond basic routing is implemented as a policy
- **Developer Experience**: Simple YAML/JSON API definitions, no manual Envoy configuration
- **Production Ready**: Graceful updates, structured logging, comprehensive validation
- **Lightweight**: Minimal footprint (360-720 MB per gateway instance)
- **Multi-Tenancy**: Support for sub-organizations and multiple environments

## Core Features

### Functional Requirements
- Accept API configurations in YAML/JSON format
- Validate and persist API configurations with structured error reporting
- Act as xDS server for dynamic Router configuration
- Support full CRUD lifecycle for API configurations
- Push configuration updates to Router within 5 seconds
- Zero-downtime updates without dropping in-flight requests
- Configurable log levels (debug, info, warn, error)

### Architecture Highlights
- **Composite Key Identity**: APIs identified by `{name}/{version}` (e.g., "PetStore/v1")
- **SotW (State-of-the-World) xDS**: Complete configuration state pushed to Router on each update
- **In-Memory + Persistent**: In-memory maps for fast access, bbolt for durability
- **Structured Validation**: Field-level error messages with JSON paths
- **Access Logging**: JSON-formatted logs to stdout for observability
- **Startup Resilience**: Router waits indefinitely with exponential backoff if xDS unavailable

## Current Status

### Implemented (Feature 001-gateway-has-two)
- Gateway-Controller with REST API (OpenAPI-based)
- xDS v3 server with SotW protocol
- API configuration validation and persistence (bbolt)
- In-memory configuration maps
- Router with Envoy bootstrap configuration
- Structured logging with Zap
- JSON access logs for traffic observability
- Docker Compose deployment

### In Progress
- Integration tests for full API lifecycle
- Enhanced error handling and validation
- Documentation and quickstart guides

### Planned
- Policy Engine integration
- Advanced rate limiting (Standard tier)
- Redis integration for distributed state
- SQLite persistence for gateway artifacts
- Multi-environment support
- Kubernetes deployment manifests

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.25.1+ (for local development)
- Make (build tool)

### Run Gateway Stack
```bash
# Start Gateway-Controller and Router
cd gateway
docker-compose up -d

# Deploy an API configuration
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d @examples/petstore-api.yaml

# Test the API through Router
curl http://localhost:8081/petstore/v1/pets
```

### Development
```bash
# Build Gateway-Controller
cd gateway/gateway-controller
make build

# Generate API server code from OpenAPI spec
make generate

# Run tests
make test

# Run locally
make run
```

## Documentation Structure

- `spec.md` - This overview document
- `prd.md` - Product requirements and feature breakdown
- `architecture/architecture.md` - Detailed architecture and components
- `design/design.md` - Design decisions and patterns
- `use-cases/use_cases.md` - Practical deployment scenarios
- `impl.md` - Implementation guide and workflow
- `impls/` - Detailed implementation documentation
- `prds/` - Individual product requirement documents

## Related Specifications

- **Feature 001**: Gateway with Controller and Router - `specs/001-gateway-has-two/`
  - Complete specification, research, and planning documents
  - Data model and quickstart guide
  - Implementation tasks and checklists

## Contributing

See the main repository README for contribution guidelines. Gateway development follows:
- Standard Go project layout (`cmd/`, `pkg/`, `tests/`)
- Make-based build system
- OpenAPI-first API design with code generation
- Test-driven development with unit and integration tests

---

**Document Version**: 1.0
**Last Updated**: 2025-10-13
**Status**: Active Development
