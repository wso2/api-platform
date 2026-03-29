# CLAUDE.md - AI Agent Guide for WSO2 API Platform

## Project Overview

WSO2 API Platform is a cloud-native API management platform with a policy-first gateway built on Envoy Proxy. The codebase is a Go-heavy monorepo with Node.js portals, managed via a Go workspace (`go.work`).

**License:** Apache 2.0
**Go Version:** 1.26.1
**Docker Registry:** `ghcr.io/wso2/api-platform`

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

## Code Patterns

- **API Design:** OpenAPI specs + oapi-codegen for Go server/client generation
- **Error Handling:** Custom error types in `common/errors/` with field-level validation
- **Storage:** Interface-based storage layer supporting SQLite (dev) and PostgreSQL (prod)
- **Testing:** Table-driven unit tests, BDD integration tests (Godog/Gherkin)
- **Config:** koanf + mapstructure for config management
- **Logging:** Leveled logging (debug, info, warn, error)
- **Policies:** Go plugins compiled via gateway-builder, evaluated with CEL expressions
- **Communication:** xDS (controller-to-router), gRPC ext_proc (router-to-policy-engine), REST + WebSocket (platform-api-to-controller)

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
