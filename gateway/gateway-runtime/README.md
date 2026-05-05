# Gateway Runtime

The Gateway Runtime is responsible for handling all API traffic. It consists of two cooperating processes: the **Router** (Envoy Proxy) and the **Policy Engine** (Envoy ext_proc). Both are configured dynamically by the Gateway Controller via xDS without requiring restarts.

## Architecture

### Go Policy Execution

Go policies are compiled directly into the Policy Engine binary at build time via the Gateway Builder. No additional process is involved at runtime.

```text
API Consumer
     ↓  HTTP/HTTPS
  Router (Envoy Proxy)          ← xDS config from Gateway Controller (:18000)
     ↓  ext_proc (gRPC)
  Policy Engine                 ← xDS config from Gateway Controller (:18001)
     │  (Go policies run in-process — no extra hop)
     ↓
  Upstream Service
```

### Python Policy Execution

Python policies run in a separate executor process. The Policy Engine delegates to it over a local gRPC socket, keeping the Go runtime isolated from the Python interpreter.

```text
API Consumer
     ↓  HTTP/HTTPS
  Router (Envoy Proxy)          ← xDS config from Gateway Controller (:18000)
     ↓  ext_proc (gRPC)
  Policy Engine                 ← xDS config from Gateway Controller (:18001)
     ↓  gRPC (Unix socket)
  Python Executor               ← Python-based policy execution
     ↓
  Upstream Service
```

**Request flow:**
1. Router receives the incoming HTTP/HTTPS request
2. Router delegates to Policy Engine via the Envoy `ext_proc` filter
3. Policy Engine runs the configured policy chain (auth, rate limiting, header manipulation, etc.)
   - **Go policies** execute in-process within the Policy Engine
   - **Python policies** are forwarded to the Python Executor over a local gRPC socket
4. Router forwards the request to the upstream service and returns the response

## Components

### Router (Envoy Proxy)
Based on `envoyproxy/envoy:v1.37.1`. Handles all inbound traffic, performs dynamic routing, and delegates request/response processing to the Policy Engine via the `ext_proc` filter.

Connects to the Gateway Controller on port `18000` to receive listener, route, and cluster configuration via xDS (ADS protocol).

### Policy Engine
A Go service that implements Envoy's `ext_proc` gRPC protocol. It receives every request and response from the Router and executes a chain of policies in order.

**Key characteristics:**
- Ships with **zero built-in policies** — all policies are compiled in at build time via the Gateway Builder
- Policies are configured per-route via xDS from the Gateway Controller (port `18001`)
- Supports conditional execution using CEL expressions
- Supports both Go and Python policy implementations
- Zero-downtime policy updates via xDS

### Python Executor
An optional gRPC sidecar that the Policy Engine delegates to when a Python-based policy is configured. It manages a pool of Python workers and handles isolation between policy executions.

### Gateway Builder
A **build-time tool** (not a runtime component) that discovers, validates, and compiles custom policies into the Policy Engine binary. Policies must be compiled via the Builder before they are available at runtime.

## Building

### Prerequisites

- Go 1.26.1+
- Docker with Buildx
- Make

### Build Docker Image

```bash
# Build and run tests
make build

# Build debug image (with dlv debugger on port 2346)
make build-debug

# Build and push multi-arch (amd64 + arm64)
make build-and-push-multiarch
```

### Run Tests

```bash
# Go policy engine unit tests
make test-go

# Python executor unit tests
make test-python

# All tests
make test
```

### Regenerate Protobuf Bindings

```bash
make proto
```

## Running

See the [Gateway README](../README.md) for how to run the full gateway stack including the Gateway Controller and Gateway Runtime together.

## Configuration

Configuration is via environment variables. The `docker-entrypoint.sh` script applies them before starting each process.

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `GATEWAY_CONTROLLER_HOST` | `gateway-controller` | Hostname of the Gateway Controller for xDS |
| `ROUTER_XDS_PORT` | `18000` | xDS port on the Gateway Controller for the Router |
| `POLICY_ENGINE_XDS_PORT` | `18001` | xDS port on the Gateway Controller for the Policy Engine |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `ROUTER_CONCURRENCY` | `0` (auto) | Envoy worker threads (0 = one per CPU core) |
| `GOMAXPROCS` | `2` | Max Go CPU cores for the Policy Engine |
| `APIP_GW_POLICY_ENGINE_METRICS_ENABLED` | `true` | Enable Policy Engine metrics |
| `PYTHON_POLICY_WORKERS` | `4` | Python Executor gRPC worker pool size |
| `PYTHON_POLICY_MAX_CONCURRENT` | `100` | Max concurrent Python policy executions |
| `PYTHON_POLICY_TIMEOUT` | `30` | Python policy execution timeout (seconds) |

### Per-Process CLI Arguments

Arguments can be passed to individual processes using a prefix:

```bash
# Pass concurrency flag to Router (Envoy)
docker run gateway-runtime --rtr.concurrency 4

# Pass log format flag to Policy Engine
docker run gateway-runtime --pol.log-format text

# Pass arguments to Python Executor
docker run gateway-runtime --py.workers 8
```

### Ports

| Port | Description |
|---|---|
| `8080` | HTTP (API traffic) |
| `8443` | HTTPS (API traffic) |
| `9901` | Envoy admin interface |
| `9002` | Policy Engine metrics |

## Development

### Project Structure

```text
gateway-runtime/
├── Dockerfile                      # Multi-stage build
├── Makefile
├── docker-entrypoint.sh            # Process orchestration
├── docker-entrypoint-debug.sh      # Debug variant (dlv on port 2346)
├── health-check.sh                 # Liveness/readiness probe
├── api/
│   └── proto/
│       └── python_executor.proto   # gRPC contract for Python execution
├── policy-engine/                  # Go ext_proc service
├── python-executor/                # Python policy runtime
└── router/
    └── config/
        ├── envoy-bootstrap.yaml    # Envoy bootstrap config
        └── config-override.yaml   # xDS cluster definition
```

### Debug Mode

See the [Gateway Debug Guide](https://github.com/wso2/api-platform/blob/main/gateway/DEBUG_GUIDE.md) for full instructions on remote and local debugging options.

### Writing a Custom Policy

All policies must be compiled into the runtime via the Gateway Builder. See the [Policy Engine README](policy-engine/README.md) for full details on writing and registering custom policies.

## Logging

Both the Router and Policy Engine emit structured logs. Set `LOG_LEVEL=debug` to see full request details, policy chain execution, and xDS snapshot updates.
