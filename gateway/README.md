# API Platform Gateway

A complete API gateway system consisting of Gateway-Controller (xDS control plane), Router (Envoy Proxy data plane), Policy Engine (request/response processing), and Policy Builder (policy compilation tooling).

For end-user documentation, see [docs/gateway/](../docs/gateway/).

## Components

| Component | Technology | Ports |
|-----------|------------|-------|
| **Gateway-Controller** | Go, Gin, oapi-codegen, bbolt, go-control-plane | 9090 (REST), 18000 (xDS) |
| **Router** | Envoy Proxy 1.35.3 | 8080 (HTTP), 8443 (HTTPS), 9901 (Admin) |
| **Policy Engine** | Go, gRPC, ext_proc, xDS, CEL | 9002 (Admin) |
| **Policy Builder** | Go, Docker | Build-time only |

## Prerequisites

- Docker + Docker Compose
- Go 1.25.1+
- Make

## Development

### Build

```bash
# Build all components (recommended for local development)
make build-local

# Build individual components
make build-local-controller
make build-local-policy-engine
make build-local-router
make build-local-gateway-builder

# Multi-architecture builds using buildx
make build
```

### Run

```bash
docker compose up -d
curl http://localhost:9090/health
```

### Test

```bash
# Unit tests
make test

# Integration tests (builds coverage-instrumented images + runs tests)
make test-integration-all

# Run integration tests only (images must be pre-built)
make test-integration

# Build coverage-instrumented images only
make build-coverage
```

For integration test details, see [it/README.md](it/README.md).

### Clean

```bash
make clean
```

## Configuration

### Gateway-Controller & Policy-Engine

Environment variables use `APIP_GW_` prefix:

| Variable | Description |
|----------|-------------|
| `APIP_GW_GATEWAY__CONTROLLER_STORAGE_TYPE` | `sqlite` or `memory` |
| `APIP_GW_GATEWAY__CONTROLLER_STORAGE_SQLITE_PATH` | Path to SQLite database |
| `APIP_GW_GATEWAY__CONTROLLER_LOGGING_LEVEL` | `debug`, `info`, `warn`, `error` |
| `APIP_GW_POLICY__ENGINE_METRICS_PORT` | Policy engine metrics port |

See [gateway-controller/README.md](gateway-controller/README.md) for full configuration options.

### Gateway Runtime

| Variable | Description |
|----------|-------------|
| `GATEWAY_CONTROLLER_HOST` | Gateway-Controller hostname (default: `gateway-controller`). The well-known xDS ports (18000 for Router, 18001 for Policy Engine) are derived automatically. |

## Component Documentation

- [gateway-controller/](gateway-controller/) - Control plane
- [router/](router/) - Envoy data plane
- [policy-engine/](policy-engine/) - Request/response processing
- [gateway-builder/](gateway-builder/) - Policy compilation tooling

## Examples

See [examples/](examples/) for sample API configurations.
