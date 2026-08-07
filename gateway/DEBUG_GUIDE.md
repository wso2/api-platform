# Gateway Debug Guide

Three debug options are available:

| Option | What runs locally | Best for |
|--------|------------------|----------|
| **[Option 1 — Remote Debug](#option-1-recommended-remote-debug--all-components-in-docker)** *(recommended)* | Nothing — everything runs in Docker, VS Code attaches via dlv | Production-like debugging, Go policies only |
| **[Option 2A — Local Process (Go only)](#option-2a-go-only)** | Controller + Policy Engine | Go policy development and iteration |
| **[Option 2B — Local Process (Go + Python)](#option-2b-go-and-python)** | Controller + Policy Engine + Python Executor | Python policy development and debugging |

> [!TIP]
> **Choose Option 2B** if you are developing or debugging a Python policy and need breakpoints, print statements, or rapid iteration without rebuilding Docker images. It extends Option 2A with the Python Executor running on the host.

---

## Option 1 (Recommended): Remote Debug — All Components in Docker

Gateway Controller and Policy Engine run inside Docker containers with dlv in server mode. VS Code attaches remotely.

### Step 1: Build Debug Images

```bash
cd gateway
make build-debug
```

This builds both `gateway-controller-debug:latest` and `gateway-runtime-debug:latest`.

### Step 2: Start the Full Stack

```bash
cd gateway
docker compose -f docker-compose.debug.yaml up
```

Wait until you see both containers are ready. The policy engine waits up to 1 minute for dlv startup before the socket becomes available.

### Step 3: Set Breakpoints

Open the relevant source files in VS Code and set breakpoints:

- **Gateway Controller**: files under `gateway/gateway-controller/`
- **Policy Engine**: files under `gateway/gateway-runtime/policy-engine/`

### Step 4: Attach VS Code Debugger

In the VS Code **Run & Debug** panel, launch:

- **"Gateway Controller (Remote)"** — attaches to `localhost:2345`
- **"Policy Engine (Remote)"** — attaches to `localhost:2346`

Both can be attached simultaneously. Source path substitution is configured automatically in `.vscode/launch.json`:

| Component | Local path | Container path |
|---|---|---|
| Gateway Controller | `gateway/gateway-controller` | `/build` |
| All others (policy-engine, common, system-policies) | `${workspaceFolder}` | `/api-platform` |
| SDK | `sdk` | `/go/pkg/mod/github.com/wso2/api-platform/sdk@v0.3.9` |

The repo root maps to `/api-platform` in the container, so `policy-engine`, `common`, and `system-policies` are all covered by a single substitutePath entry.

### Debugging SDK / Common / Policy Source Code

#### `common` and system policies

No extra steps required. Both are covered by the root `substitutePath` entry in `.vscode/launch.json`.

#### `sdk`

No extra steps required. Covered by the `sdk` substitutePath entry.

> **Note**: The `sdk` entry includes its version (`@v0.3.9`). If you update the sdk version in `policy-engine/go.mod`, update the matching entry in `.vscode/launch.json` accordingly.

#### Gateway-controller policy source

By default `build.yaml` uses `gomodule:` entries — policies compile from the Go module cache at a path like `/go/pkg/mod/...@vX.Y.Z/` inside the container. Add a `substitutePath` entry in `.vscode/launch.json` to map your local policy checkout to that path — no `build.yaml` changes or image rebuild needed.

1. **Find the exact version compiled into the image:** look up the policy in `gateway/build-manifest.yaml`. The `version` field is the resolved version and the `gomodule` field gives the module path:
   ```yaml
   - name: api-key-auth
     version: v1.8.0
     gomodule: github.com/wso2/gateway-controllers/policies/api-key-auth@v0
   ```

2. **Add an entry to the `substitutePath` array** in the `"Policy Engine (Remote)"` config — construct the `to` path from the `gomodule` module path and the `version`:
   ```json
   {
       "from": "/path/to/your/local/gateway-controllers/policies/api-key-auth",
       "to": "/go/pkg/mod/github.com/wso2/gateway-controllers/policies/api-key-auth@v0.8.0"
   }
   ```
   Repeat for each policy you want to step into.

3. Set breakpoints in your local policy source files and attach the debugger.

---

### Step 5: Deploy an API and Trigger Breakpoints

```bash
# Deploy a test API
curl -X POST http://localhost:9090/api/management/v0.9/rest-apis \
  -H "Content-Type: application/json" \
  --data-binary @examples/reading-list-v1.json

# Send a request through the router
# (-v prints the response headers, incl. the X-Served-By header the API's set-headers policy adds)
curl -v http://localhost:8080/reading-list/books
```

### Notes

- dlv runs with `--accept-multiclient` — you can detach and re-attach without restarting containers.
- Containers run as root (required by dlv for ptrace); resource limits are removed for debug headroom.
- Policy Engine socket wait timeout is 60s (vs 10s in production) to account for dlv startup overhead.
- All ports remain accessible: `9090` (Controller REST), `8080`/`8443` (Router), `9002` (PE admin), `18000`/`18001` (xDS).

---

## Option 2: Local Process Debug

Two variants run the Gateway Controller and Policy Engine as local VS Code processes — only the Envoy Router stays in Docker Compose. **Option 2A** covers Go policy work; **Option 2B** extends it by also running the Python Executor on the host for Python policy work. Both variants share the config overlay and one-time file provisioning described first, so read those two sections before jumping to 2A or 2B.

### Shared setup: config overlay

Both local-process options (2A and 2B) need a handful of settings that differ from a production container — chiefly the policy-engine ext_proc, ALS, and Python-executor connections switching from **UDS to TCP**, because there is no shared Unix socket between the Docker router and the host-run processes.

These values live in **`gateway/configs/config-debug.toml`**, a small overlay that is layered on top of the shipped `config.toml`:

- `-config` is **repeatable** on both the gateway-controller and policy-engine binaries. The loaders merge the files in the order given, **last-wins per key** (a key set in a later file overrides the same key from an earlier one), deep-merging sections and leaving every unlisted key untouched.
- The **Gateway Controller**, **Policy Engine - xDS**, and **Policy Engine - File** launch configs already pass both files:

  ```jsonc
  "args": [
      "-config", "${workspaceFolder}/gateway/configs/config.toml",
      "-config", "${workspaceFolder}/gateway/configs/config-debug.toml",
  ]
  ```

Why an overlay instead of editing `config.toml` or setting env vars:

- **`config.toml` ships in the distribution** — it must stay production-clean. The overlay keeps debug-only values out of it, and is **never mounted into the container** (the container reads `config.toml` directly, in UDS mode), so there is nothing to remember to revert.
- **Env vars only work where `config.toml` references them** via a `{{ env "NAME" }}` token — there is no prefix-based override. The overlay sets concrete values directly, with no dependency on token names matching.

Workspace-relative **file paths** (policy definitions, DB) stay as `launch.json` env vars — TOML can't expand VS Code's `${workspaceFolder}` — and the Controller's `cwd` is set to `gateway/gateway-controller` so `config.toml`'s relative defaults (certs, lua, LLM templates) resolve to the checked-in dirs there. The **one file the overlay does carry** is the AES-GCM encryption key (`[[controller.encryption.providers]]`): the shipped `config.toml` has no encryption section, so without it the host-run controller falls back to a code-default key path that doesn't exist under the debug `cwd` and fails to start. Its value is a stable, repo-relative path (not per-developer), so it lives in the overlay rather than an env var.

### Shared setup: one-time file provisioning

Two files that both local-process options depend on are **gitignored and not committed** (see `gateway/.gitignore`), so they don't exist on a fresh checkout. Generate them once from the `gateway/` directory — the listener TLS cert/key are already committed, so these are the only files you need to create.

**1. AES-256 at-rest encryption key** — a real secret the config overlay points at:

```bash
cd gateway
mkdir -p gateway-controller/aesgcm-keys
( umask 177; openssl rand 32 > gateway-controller/aesgcm-keys/default-aesgcm256-v1.bin )
```

This writes the 32-byte key to exactly the path the debug controller's `cwd` reads. Without it the Gateway Controller **exits at startup** while loading encryption providers.

**2. Environment file** — `docker-compose.yaml` declares `api-platform.env` as a `required` `env_file` for the containerized services. In the debug flow it carries no values you need (every key `config.toml` reads has a default), so an **empty file** is enough — the file just has to exist:

```bash
cd gateway
touch api-platform.env
```

Without it, `docker compose up gateway-runtime` **fails to start** with `env file .../api-platform.env not found`. Because the file is gitignored, creating it leaves nothing to revert.

---

### Option 2A: Go only

Gateway Controller and Policy Engine run as local VS Code processes. Only the Envoy Router runs in Docker Compose.

> [!WARNING]
> Processes run directly on the host, so Go resolves modules via `go.work`. Local versions of `sdk` and other workspace modules are used instead of the published Go module versions — including any uncommitted or untagged changes. Behavior may differ from a production build.

#### Architecture

```mermaid
graph TB
    subgraph "VS Code Debugger (Local)"
        GC[Gateway Controller<br/>REST API: :9090<br/>Router xDS: :18000<br/>Policy xDS: :18001]
        PE[Policy Engine<br/>ext_proc: :9001 TCP<br/>ALS: :18090 TCP<br/>Admin: :9002]
    end

    subgraph "Docker Compose"
        Router[Gateway Runtime<br/>Envoy Router<br/>HTTP: :8080<br/>HTTPS: :8443<br/>Admin: :9901]
    end

    Router -->|host.docker.internal:9001| PE
    Router -->|host.docker.internal:18090| PE
    Router -->|host.docker.internal:18000| GC
    GC -->|localhost:18001| PE
```

#### Prerequisites

- VS Code with Go extension installed
- Docker and Docker Compose
- Control plane host and registration token (optional, for gateway registration)
- **One-time file provisioning** — generate the gitignored AES-256 encryption key and the empty `api-platform.env` (see [Shared setup: one-time file provisioning](#shared-setup-one-time-file-provisioning)). The controller **won't start**, and `docker compose up` **fails**, without them.

#### Step 1: Run Gateway Builder

Run the **Gateway Builder** debug configuration from VS Code. This compiles all policies and generates the policy-engine binary into `gateway/gateway-builder/target/output/`. It also writes the policy **definition** files into `gateway/gateway-builder/target/output/gateway-controller/policies/` — the directory the controller reads via `APIP_GW_CONTROLLER_POLICIES_DEFINITIONS_PATH`.

> **Note:** This is the slowest step, so start it first — it can compile in the background while you do Steps 2–3. It **must** finish before you start the Gateway Controller (Step 4): the controller loads these policy definitions once at startup (before it hydrates stored configs and builds the first xDS snapshot). If the directory is still empty because the builder hasn't finished, the controller starts with **zero** policy definitions — with no hard error — and you must restart it after the builder completes.

#### Step 2: Configure Control Plane Connection

Update `.vscode/launch.json` in the **Gateway Controller** configuration with your control plane details:

```json
{
    "name": "Gateway Controller",
    "env": {
        "APIP_GW_CONTROLLER_CONTROLPLANE_HOST": "<your-control-plane-host>",
        "APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN": "<your-registration-token>",
        // ... other env vars
    }
}
```

> **Note:** Environment variables no longer override config keys by prefix. These values take
> effect because the config.toml the controller loads references them with `{{ env }}` tokens
> (`configs/config.toml` already has `host = '{{ env "APIP_GW_CONTROLLER_CONTROLPLANE_HOST" "" }}'`
> and the matching token for the registration token). Leave them empty (`""`) to run in standalone
> mode without a control plane connection.

#### Step 3: Update Docker Compose Configuration

In `gateway/docker-compose.yaml`, make two changes to the `gateway-runtime` service:

1. Set `GATEWAY_CONTROLLER_HOST` to `host.docker.internal` so the runtime reaches the locally-running controller:

```yaml
services:
  gateway-runtime:
    environment:
      - GATEWAY_CONTROLLER_HOST=host.docker.internal
```

2. Comment out the **Policy Engine** port block:

```yaml
services:
  gateway-runtime:
    ports:
      # Router (Envoy) - keep these
      - "8080:8080"   # HTTP ingress
      - "8443:8443"   # HTTPS ingress
      - "8081:8081"   # xDS-managed API listener
      - "8082:8082"   # WebSub Hub dynamic forward proxy
      - "8083:8083"   # WebSub Hub internal listener
      - "9901:9901"   # Envoy admin
      # Policy Engine - comment these out
      # - "9002:9002"   # Admin API
      # - "9003:9003"   # Metrics
```

#### Step 4: Start Gateway Controller

Run the **Gateway Controller** debug configuration from VS Code.

#### Step 5: Start Policy Engine

Run the **Policy Engine - xDS** debug configuration from VS Code.

#### Step 6: Start Gateway Runtime (Router)

Run the router in Docker Compose:

```bash
cd gateway
docker compose up gateway-runtime sample-backend -d
docker compose logs -ft gateway-runtime sample-backend
```

#### Step 7: Deploy an API and Test

Deploy a test API via the Gateway Controller REST API. The `-u "admin:admin"` below is the **local-only** debug login set by the `Gateway Controller` launch config (`config-debug.toml`); it is never a shipped or deployable credential. Against a real control plane, pass your own instead, e.g. `-u "$APIP_ADMIN_USER:$APIP_ADMIN_PASS"`.

```bash
curl -X POST http://localhost:9090/api/management/v1/rest-apis \
  -H "Content-Type: application/json" \
  -u "admin:admin" \
  --data-binary @examples/reading-list-v1.json
```

Send a request to the deployed API. The `-v` flag prints the response headers, so you can confirm the `X-Served-By` header injected by the API's `set-headers` policy:

```bash
curl -v http://localhost:8080/reading-list/books
```

In the verbose output you should see the gateway-added response header:

```text
< X-Served-By: wso2 api platform gateway
```

---

### Option 2B: Go and Python

This extends **Option 2A** by also running the Python Executor on the host, giving you full debugger access to the Python policy runtime.

> [!NOTE]
> **When to use this:** You are developing or debugging a Python policy and need to set breakpoints, add print statements, or iterate rapidly without rebuilding Docker images.

> [!WARNING]
> Processes run directly on the host, so Go resolves modules via `go.work`. Local versions of `sdk` and other workspace modules are used instead of the published Go module versions — including any uncommitted or untagged changes. Behavior may differ from a production build.

#### Architecture

```mermaid
graph TB
    subgraph "VS Code Debugger (Local)"
        GC["Gateway Controller<br/>REST API: :9090<br/>xDS: :18000 / :18001"]
        PE["Policy Engine<br/>ext_proc: :9001 (TCP)<br/>Admin: :9002"]
        PYE["Python Executor<br/>gRPC: localhost:9010 (TCP)"]
    end

    subgraph "Docker Compose"
        Router["Gateway Runtime<br/>Envoy Router<br/>HTTP: :8080<br/>HTTPS: :8443<br/>Admin: :9901"]
    end

    Router -->|"host.docker.internal:9001"| PE
    Router -->|"host.docker.internal:18000"| GC
    GC -->|"localhost:18001"| PE
    PE -->|"localhost:9010"| PYE
```

#### Prerequisites

- Python 3.10+ with `venv`
- VS Code with Go and Python extensions installed
- Docker and Docker Compose
- Control plane host and registration token (optional, for gateway registration)
- **One-time file provisioning** — generate the gitignored AES-256 encryption key and the empty `api-platform.env` (see [Shared setup: one-time file provisioning](#shared-setup-one-time-file-provisioning)). The controller **won't start**, and `docker compose up` **fails**, without them.

#### Step 1: TCP Mode (no action needed)

The Policy Engine reaches the host-run Python Executor over TCP at `localhost:9010`. This is **already configured** in the local debug overlay `configs/config-debug.toml`, which the **Policy Engine - xDS** launch config loads as a second `-config` on top of `config.toml`:

```toml
# configs/config-debug.toml (already committed)
[policy_engine.python_executor.server]
mode = "tcp"
port = 9010
host = "localhost"
```

No `config.toml` edit is required, and there is nothing to revert afterward. Because the overlay is passed only on the launch config's command line — never mounted into the container — the shipped `config.toml` stays in UDS mode, so the containerized Policy Engine is unaffected.

> **How it works:** `-config` is repeatable; the loaders merge the files in order with last-wins precedence, so the overlay's `tcp` values override the `uds` defaults from `config.toml` for the host-run process only. See [Shared setup: config overlay](#shared-setup-config-overlay).

#### Step 2: Run Gateway Builder

Run the **Gateway Builder** debug configuration from VS Code. This compiles all policies (Go + Python) and generates:

- The Policy Engine binary (compiled with the Python bridge code)
- `python_policy_registry.py` (maps policy names to Python modules)
- Merged `requirements.txt` (all Python policy dependencies)

> **Note:** Wait for the builder to complete successfully before starting the other components.

#### Step 3: Prepare the Python Environment

```bash
cd gateway

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

#### Step 4: Update Docker Compose Configuration

In `gateway/docker-compose.yaml`, make two changes to the `gateway-runtime` service:

1. Set `GATEWAY_CONTROLLER_HOST` to `host.docker.internal` so the runtime reaches the locally-running controller:

```yaml
services:
  gateway-runtime:
    environment:
      - GATEWAY_CONTROLLER_HOST=host.docker.internal
```

2. Comment out the **Policy Engine** port block:

```yaml
services:
  gateway-runtime:
    ports:
      # Router (Envoy) - keep these
      - "8080:8080"   # HTTP ingress
      - "8443:8443"   # HTTPS ingress
      - "8081:8081"   # xDS-managed API listener
      - "8082:8082"   # WebSub Hub dynamic forward proxy
      - "8083:8083"   # WebSub Hub internal listener
      - "9901:9901"   # Envoy admin
      # Policy Engine - comment these out
      # - "9002:9002"   # Admin API
      # - "9003:9003"   # Metrics
```


#### Step 5: Start Gateway Controller

Run the **Gateway Controller** debug configuration from VS Code.

> **Note:** Leave `APIP_GW_CONTROLLER_CONTROLPLANE_HOST` and `APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN` empty (`""`) in `.vscode/launch.json` if you want to run in standalone mode without control plane connection.

#### Step 6: Start the Python Executor

Run the **Python Executor** configuration from VS Code (see [Python debugging tips](#python-debugging-tips) below for breakpoint locations).

Alternatively, start it from the terminal:

```bash
gateway-runtime/python-executor/.venv/bin/python3 \
  gateway-runtime/python-executor/main.py \
  --listen localhost:9010 \
  --log-level debug
```

You should see:

```text
Python Executor starting (listen=localhost:9010, workers=4, ...)
Starting Python Executor on localhost:9010 (mode=tcp)
Loaded policy registry with 1 entries
Loaded policy factory: prompt-compressor:v0 from prompt_compressor_v0.policy
Python Executor ready on localhost:9010
```

#### Step 7: Start the Policy Engine

Run the **Policy Engine - xDS** debug configuration from VS Code.

The Policy Engine will connect to the Python Executor over TCP when the first Python policy is triggered. You should see:

```text
Python executor bridge initialized  address=localhost:9010  mode=tcp  timeout=30s
```

#### Step 8: Start the Gateway Runtime (Router)

Run the router in Docker Compose:

```bash
cd gateway
docker compose up gateway-runtime sample-backend -d
docker compose logs -ft gateway-runtime sample-backend
```

#### Step 9: Deploy and Test

```bash
# Deploy an API with a Python policy (e.g., prompt-compressor)
curl -X POST http://localhost:9090/api/management/v0.9/rest-apis \
  -u "<USERNAME>:<PASSWORD>" \
  -H "Content-Type: application/yaml" \
  --data-binary @path/to/api.yaml

# Send a request that triggers the policy
curl -X POST http://localhost:8080/your-api/chat \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Your test prompt here"}]}'
```

#### Step 10: Clean Up

When you are done debugging, **revert the Docker Compose changes** from Step 4 (restore `GATEWAY_CONTROLLER_HOST` and uncomment the Policy Engine ports) so `docker compose up` runs the full containerized stack again.

There is nothing to undo in `config.toml` — all TCP/debug settings live in `configs/config-debug.toml`, which is only ever passed to the host-run processes via `launch.json`, never mounted into the container.

---

## Python Debugging Tips

### Setting Breakpoints in VS Code

When using the **Python Executor** launch config, set breakpoints in:

- `executor/server.py` — gRPC servicer logic (`InitPolicy`, `ExecuteStream`)
- `executor/translator.py` — protobuf ↔ SDK type translation
- Any installed policy module (e.g., `.venv/lib/python3.*/site-packages/prompt_compressor_v0/policy.py`)

### Debugging with pdb

For quick terminal-based debugging, add breakpoints directly in policy code:

```python
# In your policy's on_request_body():
import pdb; pdb.set_trace()
```

---

## Quick Reference

### Port Map

| Port | Component | Protocol |
|------|-----------|----------|
| 9090 | Gateway Controller REST API | HTTP |
| 18000 | Gateway Controller xDS (Router) | gRPC |
| 18001 | Gateway Controller xDS (Policy Engine) | gRPC |
| 9001 | Policy Engine ext_proc | gRPC (TCP) |
| 9010 | Python Executor | gRPC (TCP) |
| 8080 | Router HTTP ingress | HTTP |
| 8443 | Router HTTPS ingress | HTTPS |
| 9901 | Router (Envoy) Admin | HTTP |
| 15000 | Sample Backend | HTTP |

### Python Executor Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PYTHON_EXECUTOR_LISTEN` | UDS socket path | Listen address — UDS path or `host:port` for TCP |
| `PYTHON_POLICY_WORKERS` | 4 | gRPC worker thread count |
| `PYTHON_POLICY_MAX_CONCURRENT` | 100 | Max concurrent policy executions |
| `PYTHON_POLICY_TIMEOUT` | 30 | Execution timeout in seconds |
| `LOG_LEVEL` | info | Log level (debug, info, warn, error) |

### VS Code Debug Configurations

All launch configurations live in `.vscode/launch.json`:

| Configuration | Type | Component |
|---|---|---|
| Gateway Controller | Go (launch) | Controller process |
| Gateway Builder | Go (launch) | Build-time compilation |
| Policy Engine - xDS | Go (launch) | Policy Engine with xDS discovery |
| Policy Engine - File | Go (launch) | Policy Engine with file-based policy chains |
| Python Executor | Python (debugpy) | Python policy runtime |
| Gateway Controller (Remote) | Go (attach) | Remote attach — Option 1 |
| Policy Engine (Remote) | Go (attach) | Remote attach — Option 1 |

---

## Common Issues

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
3. Is the **Policy Engine - xDS** launch config loading `configs/config-debug.toml` as a second `-config` (it carries `[policy_engine.python_executor.server] mode = "tcp"`)?

**"bind: address already in use" on port 9010**
→ Kill stale Python Executor processes: `pkill -f "python.*main.py"`
