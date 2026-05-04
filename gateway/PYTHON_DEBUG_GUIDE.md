# Python Executor Debug Guide

Debug and develop Python policies by running the Python Executor as a standalone process on your host machine while the rest of the gateway runs normally.

> **When to use this:** You are developing or debugging a Python policy (like `prompt-compressor`) and need to set breakpoints, add print statements, or iterate rapidly without rebuilding Docker images.

---

## Architecture

```mermaid
graph TB
    subgraph "Docker Compose"
        Router["Envoy Router<br/>HTTP: :8080<br/>HTTPS: :8443"]
    end

    subgraph "Host Process (VS Code / Terminal)"
        GC["Gateway Controller<br/>REST API: :9090<br/>xDS: :18000 / :18001"]
        PE["Policy Engine<br/>ext_proc: :9001 (TCP)"]
        PYE["Python Executor<br/>gRPC: localhost:9010 (TCP)"]
    end

    Router -->|"$HOST_IP:9001"| PE
    Router -->|"$HOST_IP:18000"| GC
    GC -->|"localhost:18001"| PE
    PE -->|"localhost:9010"| PYE
```

> [!NOTE]
> Only the Envoy Router runs in Docker. All other components — Gateway Controller, Policy Engine, and **Python Executor** — run as host processes. This gives you full debugger access to the Python runtime.

---

## Prerequisites

- Python 3.10+ with `venv`
- Go 1.22+
- Docker / Docker Compose (for the Router)
- A Python IDE or debugger (VS Code with Python extension recommended)

---

## Step-by-Step Setup

### Step 1: Enable TCP Mode in config.toml

Add the following block to `configs/config.toml`:

```toml
[python_executor.server]
mode = "tcp"
port = 9010
host = "localhost"
```

This tells the Policy Engine to connect to the Python Executor over TCP instead of the default Unix domain socket.

> [!WARNING]
> **Remove this block when you are done debugging.** The default `config.toml` is also mounted into the Docker container (`docker-compose.yaml`), where the Python Executor runs in UDS mode. If this TCP block is left in, the containerized Policy Engine will try to dial `localhost:9010` while the embedded Python Executor is listening on a UDS socket — causing silent connection failures.

### Step 2: Build the Gateway (one-time)

```bash
cd gateway

go run ./gateway-builder/cmd/builder \
  -build-file ./build.yaml \
  -system-build-lock ./system-policies/system-build-lock.yaml \
  -policy-engine-src ./gateway-runtime/policy-engine \
  -out-dir ./gateway-builder/target/output \
  -log-level debug
```

This generates:
- The Policy Engine binary (compiled with all Go + Python bridge code)
- `python_policy_registry.py` (maps policy names to Python modules)
- Merged `requirements.txt` (all Python policy dependencies)

### Step 3: Prepare the Python Environment

```bash
# Create or activate the venv
python3 -m venv gateway-runtime/python-executor/.venv
source gateway-runtime/python-executor/.venv/bin/activate

# Install dependencies (includes policy packages from the build)
pip install -r gateway-builder/target/output/python-executor/requirements.txt

# Copy the generated registry into the executor source
cp gateway-builder/target/output/python-executor/python_policy_registry.py \
   gateway-runtime/python-executor/python_policy_registry.py
```

> [!IMPORTANT]
> Re-run the `pip install` and `cp` steps after every builder run if policies change.

### Step 4: Start the Gateway Controller

```bash
HOST_IP=$(ifconfig en0 | grep "inet " | awk '{print $2}')

APIP_GW_CONTROLLER_STORAGE_TYPE=sqlite \
APIP_GW_CONTROLLER_STORAGE_SQLITE_PATH=./gateway-controller/data/gateway.db \
APIP_GW_CONTROLLER_LOGGING_LEVEL=debug \
APIP_GW_DEVELOPMENT_MODE=true \
APIP_GW_CONTROLPLANE_HOST="" \
APIP_GW_GATEWAY_REGISTRATION_TOKEN="" \
APIP_GW_CONTROLLER_POLICIES_DEFINITIONS__PATH=./gateway-controller/default-policies \
APIP_GW_CONTROLLER_LLM_TEMPLATE__DEFINITIONS__PATH=./gateway-controller/default-llm-provider-templates \
APIP_GW_ROUTER_DOWNSTREAM__TLS_CERT__PATH=./gateway-controller/listener-certs/default-listener.crt \
APIP_GW_ROUTER_DOWNSTREAM__TLS_KEY__PATH=./gateway-controller/listener-certs/default-listener.key \
APIP_GW_ROUTER_LUA_REQUEST__TRANSFORMATION_SCRIPT__PATH=./gateway-controller/lua/request_transformation.lua \
APIP_GW_ROUTER_POLICY__ENGINE_MODE=tcp \
APIP_GW_ROUTER_POLICY__ENGINE_HOST=$HOST_IP \
APIP_GW_ANALYTICS_GRPC__EVENT__SERVER_MODE=tcp \
  go run ./gateway-controller/cmd/controller \
    -config ./configs/config.toml
```

### Step 5: Start the Python Executor

```bash
gateway-runtime/python-executor/.venv/bin/python3 \
  gateway-runtime/python-executor/main.py \
  --config ./configs/config.toml \
  --listen localhost:9010 \
  --log-level debug
```

The executor reads `config.toml` for timeout/worker settings and binds to `localhost:9010` over TCP.

You should see:

```
Python Executor starting (listen=localhost:9010, workers=4, ...)
Starting Python Executor on localhost:9010 (mode=tcp)
Loaded policy registry with 1 entries
Loaded policy factory: prompt-compressor:v0 from prompt_compressor_v0.policy
Python Executor ready on localhost:9010
```

#### Debugging with VS Code

To use the VS Code Python debugger instead of the terminal, create a launch configuration:

```json
{
    "name": "Python Executor (Debug)",
    "type": "debugpy",
    "request": "launch",
    "module": "main",
    "cwd": "${workspaceFolder}/gateway/gateway-runtime/python-executor",
    "python": "${workspaceFolder}/gateway/gateway-runtime/python-executor/.venv/bin/python3",
    "args": [
        "--config", "${workspaceFolder}/gateway/configs/config.toml",
        "--listen", "localhost:9010",
        "--log-level", "debug"
    ],
    "env": {
        "PYTHONPATH": "${workspaceFolder}/gateway/gateway-runtime/python-executor"
    },
    "justMyCode": false
}
```

Set breakpoints in:
- `executor/server.py` — gRPC servicer logic (InitPolicy, ExecuteStream)
- `executor/translator.py` — protobuf ↔ SDK type translation
- Any installed policy module (e.g., `.venv/lib/python3.*/site-packages/prompt_compressor_v0/policy.py`)

#### Debugging with pdb

For quick terminal-based debugging, add breakpoints directly in policy code:

```python
# In your policy's on_request_body():
import pdb; pdb.set_trace()
```

### Step 6: Start the Policy Engine

```bash
APIP_GW_POLICY__ENGINE_SERVER_MODE=tcp \
APIP_GW_ANALYTICS_ACCESS__LOGS__SERVICE_MODE=tcp \
  go run ./gateway-runtime/policy-engine/cmd/policy-engine \
    -config ./configs/config.toml \
    -xds-server localhost:18001
```

The Policy Engine will connect to the Python Executor over TCP when the first Python policy is triggered. You should see:

```
Python executor bridge initialized  address=localhost:9010  mode=tcp  timeout=30s
```

### Step 7: Start the Router

```bash
HOST_IP=$(ifconfig en0 | grep "inet " | awk '{print $2}')
GATEWAY_CONTROLLER_HOST=$HOST_IP docker compose up gateway-runtime sample-backend -d
```

> [!WARNING]
> **Rancher Desktop (Lima) users:** `host.docker.internal` may not work. Use your actual `en0` IP address.

### Step 8: Deploy and Test

```bash
# Deploy an API with a Python policy (e.g., prompt-compressor)
curl -X POST http://localhost:9090/api/management/v0.9/rest-apis \
  -u admin:admin \
  -H "Content-Type: application/yaml" \
  --data-binary @examples/prompt-compressor-api.yaml

# Send a request that triggers the policy
curl -X POST http://localhost:8080/your-api/chat \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Your test prompt here"}]}'
```

### Step 9: Clean Up

When you are done debugging, **remove the TCP block** from `configs/config.toml`:

```diff
-[python_executor.server]
-mode = "tcp"
-port = 9010
-host = "localhost"
```

This ensures `docker compose up` continues to work correctly with UDS mode.

---

## Quick Reference

### Ports

| Port | Component | Protocol |
|------|-----------|----------|
| 9090 | Gateway Controller REST API | HTTP |
| 18000 | Gateway Controller xDS (Router) | gRPC |
| 18001 | Gateway Controller xDS (Policy Engine) | gRPC |
| 9001 | Policy Engine ext_proc | gRPC (TCP) |
| 9010 | Python Executor | gRPC (TCP) |
| 8080 | Router HTTP ingress | HTTP |
| 15000 | Sample Backend | HTTP |

### Environment Variables for the Python Executor

| Variable | Default | Description |
|----------|---------|-------------|
| `PYTHON_EXECUTOR_LISTEN` | UDS socket path | Override listen address (e.g., `localhost:9010`) |
| `PYTHON_EXECUTOR_CONFIG` | none | Path to TOML config file |
| `PYTHON_POLICY_WORKERS` | 4 | gRPC worker thread count |
| `PYTHON_POLICY_MAX_CONCURRENT` | 100 | Max concurrent policy executions |
| `PYTHON_POLICY_TIMEOUT` | 30 | Execution timeout in seconds |
| `LOG_LEVEL` | info | Log level (debug, info, warn, error) |

### Common Issues

**"Policy factory not found: prompt-compressor:v0.9.0"**
→ The Python Executor uses **major-version keys** (e.g., `prompt-compressor:v0`). This error means the `python_policy_registry.py` file was not regenerated after the builder ran. Re-copy it:
```bash
cp gateway-builder/target/output/python-executor/python_policy_registry.py \
   gateway-runtime/python-executor/python_policy_registry.py
```

**"context deadline exceeded" when calling a Python policy**
→ The Policy Engine is trying to connect to the Python Executor but failing. Check:
1. Is the Python Executor actually running? (`ps aux | grep main.py`)
2. Is it listening on the right address? (should show `localhost:9010`)
3. Does your `config.toml` have the `[python_executor.server]` block with `mode = "tcp"`?

**"bind: address already in use" on port 9010**
→ Kill stale Python Executor processes: `pkill -f "python.*main.py"`

**Container mode broken after debugging**
→ You likely left `[python_executor.server] mode = "tcp"` in `configs/config.toml`. Remove it — see [Step 9](#step-9-clean-up).
