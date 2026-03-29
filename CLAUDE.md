# CLAUDE.md - AI Agent Guide for WSO2 API Platform

## Project Overview

WSO2 API Platform is a cloud-native API management platform with a policy-first gateway built on Envoy Proxy. The codebase is a Go-heavy monorepo with Node.js portals, managed via a Go workspace (`go.work`).

**License:** Apache 2.0
**Go Version:** 1.26.1
**Docker Registry:** `ghcr.io/wso2/api-platform`

## Repository Structure

```
api-platform/
├── gateway/                 # Core API Gateway (Envoy + Go control plane)
│   ├── gateway-controller/  # xDS control plane, REST API (Go, Gin)
│   ├── gateway-runtime/     # Envoy router + policy engine
│   │   └── policy-engine/   # gRPC ext_proc service (Go)
│   ├── gateway-builder/     # Policy compilation tool (Go)
│   ├── system-policies/     # Built-in policies (analytics, etc.)
│   ├── sample-policies/     # Example policies
│   ├── dev-policies/        # Development policy templates
│   ├── it/                  # Integration tests (Godog/BDD)
│   ├── configs/             # Configuration examples
│   └── examples/            # API configuration samples
├── platform-api/            # Backend API service (Go, Gin, PostgreSQL/SQLite)
├── cli/                     # CLI tool (Go)
├── common/                  # Shared Go utilities and models
├── sdk/
│   ├── core/                # Core SDK (Go)
│   └── ai/                  # AI SDK (Go)
├── portals/
│   ├── developer-portal/    # Developer portal (Node.js)
│   └── management-portal/   # Management portal (Node.js)
├── kubernetes/
│   ├── gateway-operator/    # Kubernetes operator (Go)
│   └── helm/                # Helm charts
├── sts/                     # Security Token Service (OAuth2/OIDC)
├── distribution/            # Docker Compose deployments
│   └── all-in-one/          # Full platform stack
├── samples/                 # Sample applications
├── tests/                   # Mock servers for integration testing
├── docs/                    # Documentation
├── concepts/                # API specification docs
├── guidelines/              # Code and documentation guidelines
├── scripts/                 # Build and utility scripts
├── tools/                   # Development tools
├── go.work                  # Go workspace definition
├── Makefile                 # Root build orchestration
└── VERSION                  # Platform version (0.0.1-SNAPSHOT)
```

## Gateway Architecture

```
                    ┌──────────────────────────────────┐
                    │  Gateway-Controller (Go, Gin)     │
                    │  REST :9090 | xDS :18000          │
                    │  Storage: SQLite / PostgreSQL     │
                    └──────────────┬───────────────────┘
                                   │ xDS protocol
                    ┌──────────────┴───────────────────┐
                    ▼                                   ▼
          ┌──────────────────┐              ┌───────────────────┐
          │ Router (Envoy)   │──ext_proc──▶│ Policy Engine     │
          │ HTTP :8080       │   gRPC      │ Admin :9002       │
          │ HTTPS :8443      │             │ CEL evaluation    │
          │ Admin :9901      │             │ Go policy plugins │
          └──────────────────┘              └───────────────────┘
```

## Build & Run Commands

### Prerequisites

- Docker + Docker Compose
- Go 1.26.1+
- Make

### Gateway

```bash
# Build all gateway images
cd gateway && make build

# Run the full gateway stack
cd gateway && docker compose up -d

# Verify health
curl http://localhost:9090/health

# Debug mode (dlv on :2345 controller, :2346 runtime)
make build-debug
docker compose -f docker-compose.debug.yaml up

# Individual builds
make build-controller
make build-gateway-runtime
make build-gateway-builder

# Multi-arch build and push
make build-and-push-multiarch
```

### Platform API

```bash
cd platform-api
make build          # Docker image
make test           # Run tests
make generate       # OpenAPI code generation (oapi-codegen)
make build-and-push-multiarch
```

### CLI

```bash
cd cli/src
make build          # Platform-specific binary
make build-all      # Cross-platform builds
make test
```

### Full Platform (All-in-One)

```bash
cd distribution/all-in-one
docker compose up
```

### Root-Level Targets

```bash
make version                  # Show all component versions
make build-gateway            # Build all gateway images
make test-gateway             # Run gateway tests
make test-platform-api        # Run platform-api tests
make test-cli                 # Run CLI tests
make push-gateway             # Push gateway images to registry
make validate-versions        # Validate version consistency
make clean-gateway            # Clean gateway build artifacts
```

## Testing

### Unit Tests

```bash
# Gateway components
cd gateway && make test-controller
cd gateway && make test-gateway-builder
cd gateway && make test-policy-engine

# All gateway tests
cd gateway && make test
```

### Integration Tests (BDD with Godog)

Integration tests are in `gateway/it/` using Gherkin feature files.

```bash
cd gateway
make test-integration          # Run integration tests (requires running images)
make test-integration-all      # Build coverage images + run tests (30m timeout)
make test-postgres             # Run with PostgreSQL backend
```

**Test artifacts:**
- `reports/integration-test-results.json`
- `coverage/integration-test-coverage.txt`
- `coverage/integration-test-coverage.html`

### Test Infrastructure

Mock servers in `/tests/mock-servers/`:
- JWKS server (JWT key validation)
- Embedding providers
- Analytics collectors
- Azure Content Safety mock
- AWS Bedrock Guardrails mock
- Mock platform API

## Key Technologies & Dependencies

| Component | Framework/Library | Purpose |
|---|---|---|
| Gateway Controller | Gin, go-control-plane, bbolt | REST API, xDS, embedded store |
| Policy Engine | gRPC, ext_proc, CEL | Policy evaluation |
| Router | Envoy Proxy 1.35.3 | Data plane routing |
| Platform API | Gin, oapi-codegen, pgx | Backend service |
| CLI | cobra (likely) | CLI framework |
| Kubernetes | controller-runtime | Operator |
| Portals | Node.js | Web UIs |

**Key Go libraries:** testify, gorilla/websocket, prometheus/client_golang, jackc/pgx, mattn/go-sqlite3

## Configuration

Environment variables use the `APIP_GW_` prefix:

```bash
APIP_GW_CONTROLLER_STORAGE_TYPE=sqlite
APIP_GW_CONTROLLER_STORAGE_SQLITE_PATH=./data/gateway.db
APIP_GW_CONTROLLER_LOGGING_LEVEL=info
APIP_GW_DEVELOPMENT_MODE=true
APIP_GW_POLICY_ENGINE_METRICS_PORT=9002
GATEWAY_CONTROLLER_HOST=gateway-controller   # Used by runtime to find controller
```

Config files: `gateway/configs/config.toml`

## Go Workspace Modules

All Go modules are linked via `go.work`:

- `cli/it`, `cli/src`
- `common`
- `gateway/gateway-builder`, `gateway/gateway-controller`, `gateway/gateway-runtime/policy-engine`
- `gateway/it`
- `gateway/sample-policies/count-letters`, `gateway/sample-policies/uppercase-body`
- `gateway/system-policies/analytics`
- `platform-api/src`
- `samples/sample-service`
- `sdk/core`, `sdk/ai`
- `tests/mock-servers/mock-platform-api`

## Version Files

| Component | File | Current Version |
|---|---|---|
| Platform | `/VERSION` | 0.0.1-SNAPSHOT |
| Gateway | `/gateway/VERSION` | 1.1.0-SNAPSHOT |
| Platform API | `/platform-api/VERSION` | 0.9.0-SNAPSHOT |
| CLI | `/cli/VERSION` | 0.6.0-SNAPSHOT |

## Code Patterns

- **API Design:** OpenAPI specs + oapi-codegen for Go server/client generation
- **Error Handling:** Custom error types in `common/errors/` with field-level validation
- **Storage:** Interface-based storage layer supporting SQLite (dev) and PostgreSQL (prod)
- **Testing:** Table-driven unit tests, BDD integration tests (Godog/Gherkin)
- **Config:** koanf + mapstructure for config management
- **Logging:** Leveled logging (debug, info, warn, error)
- **Policies:** Go plugins compiled via gateway-builder, evaluated with CEL expressions
- **Communication:** xDS (controller-to-router), gRPC ext_proc (router-to-policy-engine), REST + WebSocket (platform-api-to-controller)

## Linting

Go linting configured via `.golangci.yml` (in `kubernetes/gateway-operator/`):
- gofmt, goimports, govet, staticcheck, errcheck, gosimple, unused, misspell, gocyclo, dupl, lll

## CI/CD (GitHub Actions)

- **PR checks:** `gateway-integration-test.yml`, `platform-api-pr-check.yml`
- **Releases:** `gateway-release.yml`, `platform-api-release.yml`, `cli-release.yml`
- **Security:** `go-scan.yaml`, `trivy-scan.yaml`, `jfrog-scan.yaml`
- **Coverage:** `codecov.yml`
- **Performance:** `perf-gateway.yml`

## Important Ports Summary

| Service | Port | Protocol |
|---|---|---|
| Gateway Controller REST API | 9090 | HTTP |
| Gateway Controller xDS | 18000 | gRPC |
| Gateway Controller Metrics | 9091 | HTTP |
| Router HTTP | 8080 | HTTP |
| Router HTTPS | 8443 | HTTPS |
| Router Admin | 9901 | HTTP |
| Policy Engine Admin | 9002 | HTTP |
| Policy Engine Metrics | 9003 | HTTP |
| Debug: Controller (dlv) | 2345 | TCP |
| Debug: Runtime (dlv) | 2346 | TCP |

## Gateway Controller REST API Usage

The gateway controller REST API is served on port **9090** with **no base path prefix** — routes are registered at the root (e.g., `/llm-providers`, `/rest-apis`, not `/api/v1/...`).

### Authentication

Dev mode uses basic auth configured in `gateway/configs/config.toml`:

```bash
# Default dev credentials
curl -u admin:admin http://localhost:9090/llm-providers
```

There is no `/health` endpoint on the management API. A request without credentials returning `{"error":"no valid authentication credentials provided"}` confirms the server is running.

### Key API Endpoints

```
GET    /llm-providers                  # List LLM providers
POST   /llm-providers                  # Create LLM provider
GET    /llm-providers/:id              # Get LLM provider by handle
PUT    /llm-providers/:id              # Update LLM provider
DELETE /llm-providers/:id              # Delete LLM provider

GET    /llm-proxies                    # List LLM proxies
POST   /llm-proxies                    # Create LLM proxy
GET    /llm-proxies/:id               # Get LLM proxy by handle
PUT    /llm-proxies/:id               # Update LLM proxy
DELETE /llm-proxies/:id               # Delete LLM proxy

GET    /llm-provider-templates         # List provider templates
POST   /llm-provider-templates         # Create template
GET    /rest-apis                      # List REST APIs
POST   /rest-apis                      # Create REST API
```

### Content Types

The API accepts both YAML (`Content-Type: application/yaml`) and JSON (`Content-Type: application/json`).

### Example: Deploy an LLM Provider

```bash
curl -u admin:admin -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -d '
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: my-openai-provider
spec:
  displayName: My OpenAI Provider
  version: v1.0
  template: openai
  vhost: api.openai.local
  upstream:
    url: "https://api.openai.com/v1"
  accessControl:
    mode: allow_all
'
```

Note: `spec.accessControl.mode` (`allow_all` or `deny_all`) is a **required field** for LLM providers.

### Stale Database on Startup

If the controller fails on startup with errors like `no such column: m.gateway_id`, the SQLite database schema is outdated. Fix by removing the Docker volume:

```bash
cd gateway && docker compose down -v && docker compose up -d
```
