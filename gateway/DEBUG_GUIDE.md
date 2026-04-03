# Gateway Debug Guide

Two debug options are available. **Option 1 (Remote Debug)** is the recommended approach — everything runs in Docker and VS Code attaches via dlv. **Option 2 (Local Process)** runs the controller and policy engine as local VS Code processes with only the router in Docker.

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
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @path/to/api.yaml

# Send a request through the router
curl http://localhost:8080/petstore/v1/pets
```

### Notes

- dlv runs with `--accept-multiclient` — you can detach and re-attach without restarting containers.
- Containers run as root (required by dlv for ptrace); resource limits are removed for debug headroom.
- Policy Engine socket wait timeout is 60s (vs 10s in production) to account for dlv startup overhead.
- All ports remain accessible: `9090` (Controller REST), `8080`/`8443` (Router), `9002` (PE admin), `18000`/`18001` (xDS).

---

## Option 2 (Alternative): Local Process Debug — Controller + Policy Engine in VS Code

Gateway Controller and Policy Engine run as local VS Code processes. Only the Envoy Router runs in Docker Compose.

> **Warning:** Processes run directly on the host, so Go resolves modules via `go.work`. Local versions of `sdk` and other workspace modules are used instead of the published Go module versions — including any uncommitted or untagged changes. Behavior may differ from a production build.

### Architecture

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

### Prerequisites

- VS Code with Go extension installed
- Docker and Docker Compose
- Control plane host and registration token (optional, for gateway registration)

### Step 1: Configure Control Plane Connection

Update `.vscode/launch.json` in the **Gateway Controller** configuration with your control plane details:

```json
{
    "name": "Gateway Controller",
    "env": {
        "APIP_GW_CONTROLPLANE_HOST": "<your-control-plane-host>",
        "APIP_GW_GATEWAY_REGISTRATION_TOKEN": "<your-registration-token>",
        // ... other env vars
    }
}
```

> **Note:** Leave these empty (`""`) if you want to run in standalone mode without control plane connection.

### Step 2: Update Docker Compose Configuration

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

### Step 3: Start Gateway Controller

Run the **Gateway Controller** debug configuration from VS Code.

### Step 4: Run Gateway Builder

Run the **Gateway Builder** debug configuration from VS Code. This compiles all policies and generates the policy-engine binary into `gateway/gateway-builder/target/output/`.

> **Note:** Wait for the builder to complete successfully before starting the Policy Engine.

### Step 5: Start Policy Engine

Run the **Policy Engine - xDS** debug configuration from VS Code.

### Step 6: Start Gateway Runtime (Router)

Run the router in Docker Compose:

```bash
cd gateway
docker compose up gateway-runtime sample-backend -d
docker compose logs -ft gateway-runtime sample-backend
```

### Step 7: Deploy an API and Test

Deploy a test API via the Gateway Controller REST API:

```bash
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @path/to/api.yaml
```

Send a request to the deployed API:

```bash
curl http://localhost:8080/petstore/v1/pets
```
