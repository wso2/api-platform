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
# Build all components
make build

# Build individual components
make build-controller
make build-gateway-runtime
make build-gateway-builder
```

### Run

Run the one-time setup (generates `api-platform.env`, the router listener TLS certificate, the AES-256
encryption key, and the gateway-controller admin credentials), then start the stack:

```bash
./scripts/setup.sh
docker compose up -d
curl http://localhost:9092/api/admin/v0.9/health
```

`setup.sh` prints the admin password **once** — copy it. The username defaults to `admin`; set
`ADMIN_USERNAME`/`ADMIN_PASSWORD` up front for non-interactive runs. Only the bcrypt hash is stored (in
`api-platform.env`), and — because basic auth is enabled in the shipped config — the controller refuses
to start if the credential is missing.

`setup.sh` is idempotent (rerun with `--force` to rotate certs, the encryption key, and the admin
password, and rewrite `api-platform.env`).

For the full setup reference — flags (`--force`, `--certs-only`, `--with-encryption`), control-plane
connection, at-rest encryption, and how configuration is delivered — see
[docs/gateway/quick-start-guide.md](../docs/gateway/quick-start-guide.md).

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

Configuration is read from a TOML config file (mounted at `/etc/gateway-controller/config.toml`
and `/etc/policy-engine/config.toml`) layered over built-in defaults. **Environment variables do
not override config keys directly.** An environment value reaches a setting only through an explicit
`{{ env "NAME" "default" }}` interpolation token in the config file, resolved at load time; a field
with no token always takes its literal TOML value or the built-in default. Secrets can also be read
from a mounted file with `{{ file "PATH" }}`.

The sample composes deliver these values from `api-platform.env` via docker-compose `env_file:`
(generate it with `./scripts/setup.sh`). The shipped
`configs/config.toml` already carries `{{ env "APIP_GW_..." }}` tokens for the common settings —
`APIP_GW_` is a naming convention on the token argument, not a prefix that auto-maps to config keys:

| Environment variable (token argument) | Config key | Description |
|----------|----------|-------------|
| `APIP_GW_CONTROLLER_STORAGE_TYPE` | `controller.storage.type` | `sqlite`, `postgres`, `sqlserver`, or `memory` |
| `APIP_GW_CONTROLLER_STORAGE_SQLITE_PATH` | `controller.storage.sqlite.path` | Path to SQLite database |
| `APIP_GW_CONTROLLER_STORAGE_DATABASE_DSN` | `controller.storage.database.dsn` | SQL Server DSN (when storage type is `sqlserver`) |
| `APIP_GW_CONTROLLER_CONTROLPLANE_HOST` | `controller.controlplane.host` | Control plane endpoint |
| `APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN` | `controller.controlplane.token` | Control plane registration token |
| `APIP_GW_CONTROLLER_LOGGING_LEVEL` | `controller.logging.level` | `debug`, `info`, `warn`, `error` |

See [gateway-controller/README.md](gateway-controller/README.md) and `configs/config-template.toml`
for the full set of tokens and configuration options.

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
