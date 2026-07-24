---
name: gateway-debug
description: Debug or fix an issue in the WSO2 API Platform gateway. Use when the user asks to debug the gateway, debug the gateway-controller or policy-engine, step through gateway source code, set a breakpoint in the controller or policy-engine, or investigate why a deployed REST API or routed request misbehaves at the source level; or to fix a gateway bug end-to-end.
allowed-tools: Bash, Read, Edit, Grep, Glob
---

# Gateway Debug

Local-process debugger workflow for the gateway, based on `gateway/DEBUG_GUIDE.md`
**Option 2A** (controller + policy-engine locally, Envoy in Docker) and
**Option 2B** (also runs Python Executor locally — only when a Python policy is
under test).

Envoy itself is **never** the debug target — we always run it in Docker. We use
`dlv` against the gateway-controller and policy-engine Go binaries directly.

> **Path convention.** All paths in this skill are written as
> `<REPO_ROOT>/...` — substitute your api-platform checkout (e.g.
> `~/git/api-platform`) for `<REPO_ROOT>`. To paste shell blocks as-is, each
> block first sets a matching variable:
> ```bash
> REPO_ROOT="$(git rev-parse --show-toplevel)"
> ```
> and then references `$REPO_ROOT/gateway/...`. Run the commands from anywhere
> inside the checkout — no hard-coded paths.

## When to use

- "Debug why the controller / policy engine is doing X"
- "Step through controller code when I POST to /rest-apis"
- "Find and fix a bug in gateway-controller / policy-engine"
- A specific request returns the wrong response and source-level inspection is
  needed (when [[gateway-integration-tests]] step 3 — logs / config dumps — wasn't enough)
- An Integration Test (IT) reproducer exists but you need to step through the gateway side,
  not the test side

## Workflow

Work through phases 1-6 + 8 every time. Phase 7 (the fix loop) is **optional** —
only run it when the user explicitly asked you to fix the issue, not just to
investigate / explain it.

1. **Decide which component to debug** (Step 1) — usually controller XOR policy-engine;
   pick wrong and you'll watch the wrong process while the bug fires elsewhere.
2. **Prepare the env** — picking **Path A** (deployment-only) or **Path B** (request-runtime through Envoy) drives which substeps you run: edit a compose file, run the builder, start Envoy (Step 2).
3. **Launch the chosen component under `dlv`** in headless mode (Step 3).
4. **Reproduce the issue** with `curl` against the management REST API + the example YAMLs in `gateway/examples/` (Step 4).
5. **Triage from logs + config dumps** before touching the debugger (Step 5).
6. **Attach `dlv`, set function-name breakpoints, find root cause** (Step 6).
7. **(Optional) Apply a fix and re-verify through the same loop** (Step 7) — skip if the request was investigate-only.
8. **Clean up**: revert the compose / config.toml edits, stop processes, `docker compose down` (Step 8).

## Layout

- Gateway tree: `<REPO_ROOT>/gateway/`. Most commands `cd` into it.
- Controller entry: `<REPO_ROOT>/gateway/gateway-controller/cmd/controller`
- Policy-engine entry: `<REPO_ROOT>/gateway/gateway-runtime/policy-engine/cmd/policy-engine`
- Builder entry: `<REPO_ROOT>/gateway/gateway-builder/cmd/builder`
- Python executor: `<REPO_ROOT>/gateway/gateway-runtime/python-executor/main.py`
- Builder output (required by PE): `<REPO_ROOT>/gateway/gateway-builder/target/output/`
- VS Code launch envs (use these for local processes too): `<REPO_ROOT>/.vscode/launch.json`
- Example API/secret/key YAMLs: `<REPO_ROOT>/gateway/examples/` — including `sample-echo-api.yaml` (debug-friendly echo upstream wired to the in-compose `sample-backend`)
- Management REST API spec: `<REPO_ROOT>/gateway/gateway-controller/api/management-openapi.yaml`
- Admin REST API spec (config_dump etc.): `<REPO_ROOT>/gateway/gateway-controller/api/admin-openapi.yaml`
- Controller config: `<REPO_ROOT>/gateway/configs/config.toml`

## Step 1: Decide which component to debug

Before touching anything, work out where the bug lives. Wrong pick = you stare at
the wrong process. Use config dumps to localise — config flows
**controller → (xDS) → policy-engine + Envoy**, so the first dump that looks
wrong is the culprit. Bring the stack up briefly in normal Docker mode if needed:

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT/gateway"
docker compose up -d gateway-controller gateway-runtime sample-backend
```

Then:

```bash
# Controller — what it decided to deploy (APIs, routes, clusters, policy chains)
curl -s http://localhost:9094/api/admin/v0.9/config_dump        | jq .   # 9094 in docker-compose; 9092 when running locally
curl -s http://localhost:9094/api/admin/v0.9/xds_sync_status    | jq .

# Policy-engine — what it received from controller via xDS
curl -s http://localhost:9002/config_dump                       | jq .

# Envoy — live listeners/routes/clusters + per-endpoint health
curl -s http://localhost:9901/config_dump                       | jq .
curl -s http://localhost:9901/clusters
```

> **Admin port** for the controller is `9092` everywhere except the *dev*
> docker-compose (`<REPO_ROOT>/gateway/docker-compose.yaml`), which remaps it
> to `9094`. The IT compose (`<REPO_ROOT>/gateway/it/docker-compose.test.yaml`)
> and the local dlv-driven controller (Step 3a) both bind the native `9092`.
> Pick the port that matches the mode you're in.

Rule of thumb:

| Symptom | Debug this |
|---|---|
| `POST /rest-apis` returns wrong status / wrong response body | **controller** |
| Controller `config_dump` is missing the API or has the wrong route/cluster/policy chain | **controller** |
| Controller dump looks correct but PE `config_dump` doesn't have it | **controller** (xDS push) — but break in PE's xDS handler to confirm |
| PE dump has it but request behaviour at the router is wrong (response transform, auth, rate limit, header munging…) | **policy-engine** |
| Routing/upstream selection looks wrong and PE metadata is correct | **policy-engine** (`upstream` mapping) — Envoy itself we don't debug |
| Python policy misbehaves | **python-executor** (Option 2B) |
| Anything else | start with the **controller** — it owns the deployment record |

Stop the temporary Docker stack before launching local processes:
```bash
cd "$REPO_ROOT/gateway" && docker compose down
```

## Step 2: Prepare the environment

> **One-time provisioning (required before the first `docker compose up`).** Run setup once from
> `<REPO_ROOT>/gateway`. It generates `api-platform.env` (required runtime defaults), the router's HTTPS
> listener certificate, the AES-256 at-rest encryption key, and the gateway-controller **admin
> credentials**. Startup fails if basic auth is enabled with no credential, so run it non-interactively
> with fixed credentials to keep the `-u admin:admin` examples in this skill valid:
>
> ```bash
> ADMIN_USERNAME=admin ADMIN_PASSWORD=admin ./scripts/setup.sh
> ```
>
> The gateway never auto-generates keys/certs and has no demo mode: the compose `env_file:` is now
> `required: true` (a missing `api-platform.env` fails `docker compose up`), and the controller exits at
> startup if the encryption key is missing. Full reference:
> [Gateway Quick Start](../../../docs/gateway/quick-start-guide.md).

> **Compose target — pick one before editing anything.** Step 2 (and Step 8
> cleanup) operates on **one** compose file. Subsequent substeps reference
> "the compose file" generically:
>
> | Mode | Compose file (`<COMPOSE>`) | Run `docker compose` from (`<COMPOSE_CWD>`) | Use when |
> |---|---|---|---|
> | **Standalone** *(default)* | `<REPO_ROOT>/gateway/docker-compose.yaml` | `<REPO_ROOT>/gateway` (default filename, no `-f` needed) | Investigating from scratch; no IT scenario in play |
> | **IT handoff** | `<REPO_ROOT>/gateway/it/docker-compose.test.yaml` | `<REPO_ROOT>/gateway/it` (use `-f docker-compose.test.yaml` — non-default filename) | Reproducing a failing integration test (handoff from [[gateway-integration-tests]]) — keeps the IT mocks (`mock-jwks`, `mock-platform-api`, …) that the scenario depends on |

> **Pick a path before doing any 2x substep.** Two flows in the gateway
> need different setups; choose based on what the bug actually involves.
> Once you've picked, the rest of Step 2 (and Step 8 cleanup) applies only
> to the substeps your path lists.
>
> | Path | What the bug involves | Setup to run | Then go to |
> |---|---|---|---|
> | **A. Deployment path** | The controller's REST API + its internal state. Examples: deploy / update / delete a REST API, secret CRUD, api-key generate / list / regenerate / revoke, MCP / LLM-resource CRUD. **No traffic flows through Envoy or the policy engine** in the failing flow. | **2b only** (and only if you'll also launch the policy-engine, which Path A normally doesn't) | **3a** — launch controller under dlv |
> | **B. Request-runtime path** | An actual HTTP request through the gateway misbehaves: wrong upstream, wrong status, header munging, auth/rate-limit verdict, response transform, etc. Envoy receives the request and ext_proc-calls the policy engine. | **2a + 2b + 2c** (compose edit + builder + Envoy in Docker) | **3a + 3b** — launch both controller and policy-engine under dlv |
>
> Path B is the broader one — anything that exercises the runtime data
> plane. Path A is faster and dockerless: the controller alone, in Step 3a,
> is self-sufficient for the management-API flows.

### 2a. Edit the compose file (so the in-Docker Envoy talks to the host process)

Make **two** changes to the `gateway-runtime` service block:

1. Point the runtime at the host. Replace whatever the file currently has
   for `GATEWAY_CONTROLLER_HOST` (in the dev compose it's `gateway-controller`;
   in the IT compose it's `it-gateway-controller`) with `host.docker.internal`:
   ```yaml
   gateway-runtime:
     environment:
       - GATEWAY_CONTROLLER_HOST=host.docker.internal
   ```
2. Comment out the **Policy Engine** port mappings — those ports belong to the
   local process now, leaving them mapped collides on bind:
   ```yaml
       ports:
         - "8080:8080"   # HTTP ingress — keep
         - "8443:8443"   # HTTPS ingress — keep
         - "8081:8081"   # xDS-managed listener — keep
         - "9901:9901"   # Envoy admin — keep
         # - "9002:9002"   # PE Admin   — COMMENTED OUT
         # - "9003:9003"   # PE Metrics — COMMENTED OUT
   ```

**Remember to revert these in Step 8** (`git checkout -- <COMPOSE>`).
A leftover `host.docker.internal` will silently break plain `docker compose up`
(or `make test`, for the IT compose) for other people on the repo.

### 2b. Run the gateway-builder (required before launching policy-engine)

The policy-engine source includes generated files (`plugin_registry.go`,
`build_info.go`) that the builder emits — without them, `dlv debug` against
the PE entry point won't compile. Run the builder once before the first PE
debug session, and again whenever `build.yaml` or any compiled policy changes:

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT/gateway/gateway-builder"
go run ./cmd/builder \
  -build-file        ../build.yaml \
  -system-build-lock ../system-policies/system-build-lock.yaml \
  -policy-engine-src ../gateway-runtime/policy-engine \
  -out-dir           ./target/output \
  -log-level         info
```

Confirm:
- `ls "$REPO_ROOT/gateway/gateway-builder/target/output/gateway-controller/policies/" | wc -l` — should be 40+ policy YAMLs.
- `ls "$REPO_ROOT/gateway/gateway-runtime/policy-engine/cmd/policy-engine/" | grep -E 'plugin_registry|build_info'` — both files should exist (gitignored).

> Builder output is also where the controller reads policy definitions from
> (`controller.policies.definitions_path`, wired to `APIP_GW_CONTROLLER_POLICIES_DEFINITIONS_PATH`
> via a `{{ env }}` token in the config — the prefix override no longer applies on its own).
> If the controller starts and complains it can't load policy definitions,
> you skipped the builder.

### 2c. Start the Envoy router (and any backends) in Docker

Standalone mode:
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT/gateway"
docker compose up -d gateway-runtime sample-backend
docker compose logs -ft gateway-runtime          # tail [rtr] / [pol] streams
```

IT-handoff mode (bring up only the services your scenario needs — minimally
`gateway-runtime` + the backends/mocks it touches, e.g. `sample-backend`,
`mock-jwks`, `mock-platform-api`):
```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT/gateway/it"
docker compose -f docker-compose.test.yaml up -d gateway-runtime sample-backend mock-platform-api  # add others as the scenario requires
docker compose -f docker-compose.test.yaml logs -ft gateway-runtime
```

Envoy will sit waiting for xDS from `host.docker.internal:18000` (controller)
and ext_proc on `host.docker.internal:9001` (policy engine). Until you launch
those in Step 3, `curl http://localhost:9901/ready` returns 503 — that's
expected and clears once Step 3 brings both up.

## Step 3: Launch the chosen component under `dlv`

`dlv debug` builds from source and starts the binary. Use `--headless
--listen=127.0.0.1:<port> --accept-multiclient --continue` so the process runs
immediately and you can attach / detach freely.

**Pass the same env vars that `.vscode/launch.json` does for that configuration.**
The blocks below extract those env vars and run `dlv` from a single Bash
invocation.

> ⚠️ **Config change — the `APIP_GW_` prefix override was removed.** The gateway loaders now read
> **only** the `-config` file layered over defaults; an environment value reaches a setting solely
> through a `{{ env "NAME" }}` interpolation token in that file. The `APIP_GW_*` env vars below
> therefore take effect **only** for keys whose config value is a matching token. `configs/config.toml`
> currently tokenizes storage, control-plane, logging, metrics and the policies path — but **not** the
> machine-specific dev paths (LLM-template dir, downstream TLS cert/key, lua script) or the local
> tcp policy-engine/analytics split used here. For from-source `dlv` runs, point `-config` at a
> **local, git-ignored** `config.toml` (copied from `configs/config-template.toml`) with those values
> filled in — or add `{{ env }}` tokens for them to that local config so the variables below apply.
> _Follow-up: ship a ready-made dev `config.toml` template for this recipe._

### 3a. Gateway Controller (dlv on `127.0.0.1:2345`)

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT/gateway/gateway-controller"

# Env vars mirror .vscode/launch.json → "Gateway Controller"
APIP_GW_CONTROLPLANE_HOST="" \
APIP_GW_GATEWAY_REGISTRATION_TOKEN="" \
APIP_GW_CONTROLLER_STORAGE_TYPE=sqlite \
APIP_GW_CONTROLLER_STORAGE_SQLITE_PATH="$REPO_ROOT/gateway/gateway-controller/data/gateway.db" \
APIP_GW_CONTROLLER_LLM_TEMPLATE__DEFINITIONS__PATH="$REPO_ROOT/gateway/gateway-controller/default-llm-provider-templates" \
APIP_GW_CONTROLLER_ROUTER_DOWNSTREAM__TLS_CERT__PATH="$REPO_ROOT/gateway/gateway-controller/listener-certs/default-listener.crt" \
APIP_GW_CONTROLLER_ROUTER_DOWNSTREAM__TLS_KEY__PATH="$REPO_ROOT/gateway/gateway-controller/listener-certs/default-listener.key" \
APIP_GW_ROUTER_DOWNSTREAM__TLS_CERT__PATH="$REPO_ROOT/gateway/gateway-controller/listener-certs/default-listener.crt" \
APIP_GW_ROUTER_DOWNSTREAM__TLS_KEY__PATH="$REPO_ROOT/gateway/gateway-controller/listener-certs/default-listener.key" \
APIP_GW_ROUTER_LUA_REQUEST__TRANSFORMATION_SCRIPT__PATH="$REPO_ROOT/gateway/gateway-controller/lua/request_transformation.lua" \
APIP_GW_CONTROLLER_POLICIES_DEFINITIONS__PATH="$REPO_ROOT/gateway/gateway-builder/target/output/gateway-controller/policies" \
APIP_GW_ROUTER_POLICY__ENGINE_MODE=tcp \
APIP_GW_ROUTER_POLICY__ENGINE_HOST=host.docker.internal \
APIP_GW_ANALYTICS_GRPC__EVENT__SERVER_MODE=tcp \
APIP_GW_IMMUTABLE__GATEWAY_ENABLED=false \
APIP_GW_IMMUTABLE__GATEWAY_ARTIFACTS__DIR="$REPO_ROOT/gateway/examples" \
dlv debug ./cmd/controller \
  --headless --listen=127.0.0.1:2345 --api-version=2 \
  --accept-multiclient --continue \
  -- -config "$REPO_ROOT/gateway/configs/config.toml" \
  > /tmp/dlv_controller.log 2>&1 &
```

### 3b. Policy Engine (dlv on `127.0.0.1:2346`)

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT/gateway/gateway-runtime/policy-engine"

APIP_GW_POLICY__ENGINE_SERVER_MODE=tcp \
APIP_GW_ANALYTICS_ACCESS__LOGS__SERVICE_MODE=tcp \
dlv debug ./cmd/policy-engine \
  --headless --listen=127.0.0.1:2346 --api-version=2 \
  --accept-multiclient --continue \
  -- -config "$REPO_ROOT/gateway/configs/config.toml" \
     -xds-server localhost:18001 \
  > /tmp/dlv_policy_engine.log 2>&1 &
```

### 3c. Wait for the chosen process(es) to be ready, then verify

`dlv debug` compiles before listening — expect ~20-30 s cold, faster on rebuild.
This block is path-aware: Path A waits only for the controller listener and
curls only the controller; Path B waits for both listeners and curls all three
endpoints. Set `MODE` to match the path you picked at Step 2.

```bash
MODE=B   # B = request-runtime (3a + 3b); A = deployment-only (3a only)

echo "waiting for dlv listener(s)..."
for i in $(seq 1 60); do
  l1=$(lsof -nP -iTCP:2345 -sTCP:LISTEN -t 2>/dev/null | head -1)
  if [ "$MODE" = "B" ]; then
    l2=$(lsof -nP -iTCP:2346 -sTCP:LISTEN -t 2>/dev/null | head -1)
    [ -n "$l1" ] && [ -n "$l2" ] && { echo "controller + policy-engine up after ${i}s"; break; }
  else
    [ -n "$l1" ] && { echo "controller up after ${i}s"; break; }
  fi
  sleep 1
done

# Confirm: controller REST 200 always; PE admin + Envoy /ready only on Path B
curl -s -o /dev/null -w "controller mgmt:  HTTP %{http_code}\n" -u admin:admin \
  http://localhost:9090/api/management/v0.9/rest-apis
if [ "$MODE" = "B" ]; then
  curl -s -o /dev/null -w "policy-engine:    HTTP %{http_code}\n" \
    http://localhost:9002/config_dump
  curl -s -o /dev/null -w "envoy /ready:     HTTP %{http_code}\n" \
    http://localhost:9901/ready
fi
```

Each invoked endpoint should return `HTTP 200`. The locally-bound process
appears in `lsof` as `__debug_b<inode>` (that's the dlv-compiled binary name)
— not `controller`.

If you need **both** processes running (e.g. tracing a config flow), launch
both — they listen on different dlv ports (2345, 2346) and you attach to
whichever you're inspecting at the moment.

### 3d. Python Executor (Option 2B — rarely needed)

Only needed if the bug is in a Python policy. Adds two extra steps:

1. Add to `<REPO_ROOT>/gateway/configs/config.toml`:
   ```toml
   [policy_engine.python_executor.server]
   mode = "tcp"
   port = 9010
   host = "localhost"
   ```
   **Remember to remove this block in Step 8** — leaving it in silently breaks
   `docker compose up` (PE in container will dial localhost:9010 against a UDS
   listener).

2. Prepare and run:
   ```bash
   REPO_ROOT="$(git rev-parse --show-toplevel)"
   cd "$REPO_ROOT/gateway"
   python3 -m venv gateway-runtime/python-executor/.venv
   source gateway-runtime/python-executor/.venv/bin/activate
   pip install -r gateway-builder/target/output/python-executor/requirements.txt
   cp gateway-builder/target/output/python-executor/python_policy_registry.py \
      gateway-runtime/python-executor/python_policy_registry.py
   python3 gateway-runtime/python-executor/main.py \
     --listen localhost:9010 --log-level debug \
     > /tmp/python_executor.log 2>&1 &
   ```

For Python breakpoints use `pdb`/`debugpy` in the policy file directly — `dlv`
is Go-only. This skill's `dlv` flow does not cover Python; see the
"Python Debugging Tips" section of `gateway/DEBUG_GUIDE.md` if needed.

## Step 4: Reproduce the issue against the management REST API

The management API runs on `http://localhost:9090/api/management/v0.9`
(basic auth — `admin:admin` if you provisioned it as recommended in Step 2). Full endpoint list:
`<REPO_ROOT>/gateway/gateway-controller/api/management-openapi.yaml`. Most common:

| Resource | Method + path | Example YAML / response |
|---|---|---|
| Deploy / replace a REST API | `POST /rest-apis` (`Content-Type: application/yaml`) | `<REPO_ROOT>/gateway/examples/petstore-api.yaml` · `<REPO_ROOT>/gateway/examples/sample-echo-api.yaml` |
| Get / delete a REST API | `GET /rest-apis/{id}` · `DELETE /rest-apis/{id}` | — |
| Generate API key for a REST API | `POST /rest-apis/{id}/api-keys` (JSON `{"name":"..."}`) | response: `{"status":"success","apiKey":{"name":"...","apiKey":"apip_<hex>..."}}` — note the `apip_` prefix |
| List / regenerate / revoke API key | `GET … /api-keys` · `POST … /api-keys/{name}/regenerate` · `DELETE … /api-keys/{name}` | — |
| Managed secret | `POST /secrets` · `GET /secrets/{id}` · `DELETE /secrets/{id}` | `<REPO_ROOT>/gateway/examples/managed-secret.yaml` |
| MCP proxy | `POST /mcp-proxies` · `DELETE /mcp-proxies/{id}` | `<REPO_ROOT>/gateway/examples/mcp-proxy.yaml` |
| LLM provider / proxy | `POST /llm-providers` · `POST /llm-proxies` · `…/api-keys` | `<REPO_ROOT>/gateway/examples/llm-*.yaml` |

### Recommended reproducer: `sample-echo-api.yaml`

The `sample-backend` service in `<REPO_ROOT>/gateway/docker-compose.yaml`
(`ghcr.io/wso2/api-platform/sample-service`) echoes the full request back as
JSON: `method`, `path`, `query`, `headers`, `body`. That means you can confirm
exactly what the gateway forwarded — header injections, path stripping, body
transforms — without leaving the local docker network. Sample-service also
supports a `?statusCode=<n>` query param so you can drive any upstream status
on demand. The shipped `<REPO_ROOT>/gateway/examples/sample-echo-api.yaml`
combines `api-key-auth` + `set-headers` + the `/anything` operations to make
this a one-liner. Use it as the default reproducer unless the bug requires a
specific upstream.

> The shipped YAML targets `http://sample-backend:5000` — the port from the
> standalone dev compose. For the **IT handoff** compose target the IT
> sample-backend listens on `:9080`; either edit the `upstream.main.url` to
> `http://sample-backend:9080` or use the scenario's own deploy YAML.

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"

# 1) Deploy
curl -s -u admin:admin -X POST 'http://localhost:9090/api/management/v0.9/rest-apis' \
  -H 'Content-Type: application/yaml' \
  --data-binary @"$REPO_ROOT/gateway/examples/sample-echo-api.yaml" \
  | jq '{deployed: .status.state, id: .status.id}'

# 2) Generate an API key (keys returned as "apip_<hex>")
KEY=$(curl -s -u admin:admin -X POST \
  'http://localhost:9090/api/management/v0.9/rest-apis/sample-echo-v1/api-keys' \
  -H 'Content-Type: application/json' -d '{"name":"repro-key"}' \
  | jq -r '.apiKey.apiKey')
echo "key: ${KEY:0:24}..."

# 3a) Without the key → 401 (proves api-key-auth fires)
curl -s -o /dev/null -w "no-key:  HTTP %{http_code}\n" http://localhost:8080/echo/anything

# 3b) With the key → 200 + sample-backend echo
curl -s -H "X-API-Key: $KEY" http://localhost:8080/echo/anything \
  | jq '{method, path, gatewayMarker: .headers["X-Gateway-Marker"]}'

# 4) Tear down when done with this reproducer
curl -s -u admin:admin -X DELETE \
  'http://localhost:9090/api/management/v0.9/rest-apis/sample-echo-v1' | jq .
```

A clean run shows:
```json
{ "method": "GET",
  "path": "/anything",
  "gatewayMarker": ["ap-gateway"] }
```
If any of those is wrong, that's your bug. The shape of sample-service's echo
(`.method`, `.path`, `.query`, `.headers`, `.body`) gives you a precise lens on
what each policy did to the request. To drive an arbitrary upstream status add
`?statusCode=503` (or any code) to the request URL — useful for triaging how
the gateway handles non-2xx upstream responses.

For non-echo bugs (subscription, LLM, MCP, certificate management…) swap
`sample-echo-api.yaml` for the matching example under `<REPO_ROOT>/gateway/examples/`.

## Step 5: Triage from logs + config dumps (before touching the debugger)

Most bugs surface in logs or config dumps without needing to step through code.

### 5a. Logs

| Source | Where |
|---|---|
| Controller (local, under dlv) | `/tmp/dlv_controller.log` — application log interleaved with dlv output |
| Policy engine (local, under dlv) | `/tmp/dlv_policy_engine.log` |
| Envoy router (in Docker) — `[rtr]` lines | `cd <REPO_ROOT>/gateway && docker compose logs --no-log-prefix gateway-runtime 2>&1 \| grep '^\[rtr\]'` |
| Python executor (Option 2B) | `/tmp/python_executor.log` |

> Why the `grep`: the `gateway-runtime` container stamps every log line with
> one of three prefixes — `[rtr]` (Envoy router), `[pol]` (in-container PE,
> still receives xDS pushes even in debug mode), unprefixed (the entrypoint).
> When debugging traffic you only want `[rtr]` — Envoy's access log is where
> each request's status, upstream, and policy verdict actually surface.
> `--no-log-prefix` drops Docker's `gateway-runtime-1  |` per-line prefix so
> the `[rtr]` anchor is at column 0.

**Controller log lines** carry `correlation_id=<uuid>` — grep on it to follow
one request end-to-end across handler → service → xDS push:

```
msg="HTTP request" correlation_id=2ab42e… method=POST path=/api/.../api-keys status=201 latency=1.2ms
msg="Storing API key with state-of-the-world update" api_name="Sample Echo" api_key_name=log-eval-key correlation_id=2ab42e…
msg="Successfully stored API key and updated policy engine state" correlation_id=2ab42e…
```

**Policy-engine log lines** trace xDS receipt + policy chain compilation:

```
msg="Parsed StoredPolicyConfig" api_name="Sample Echo" routes=1
msg="Policy chain update completed successfully" version=2 total_routes=4
msg="Successfully processed and ACKed discovery response" type_url=...PolicyChainConfig version=2 num_resources=4
```

**Envoy `[rtr]` access log** — one line per request, columns
`"METHOD path PROTO" rewritten-path - STATUS RESPONSE_FLAGS RESPONSE_CODE_DETAILS … "upstream-host" "upstream-address"`.
Real examples from the same reproducer:

```
[rtr] [...Z] "GET /echo/anything?statusCode=503 HTTP/1.1" /anything?statusCode=503 HTTP/1.1 503 - via_upstream 0 162 1085 12 1 0 11 "-" "curl/8.7.1" "<x-req-id>" "sample-backend:5000" "172.18.0.4:5000"
[rtr] [...Z] "GET /echo/anything HTTP/1.1" /echo/anything - 401 -  - 0 59 1 - - 0 - "-" "curl/8.7.1" "<x-req-id>" "localhost:8080" "-"
[rtr] [...Z] "GET /this/does/not/exist HTTP/1.1" /this/does/not/exist - 404 - direct_response 0 21 0 - - 0 - "-" "curl/8.7.1" "<x-req-id>" "localhost:8080" "-"
```

Read those three at a glance:
- `503` + `via_upstream` + concrete upstream address → request reached upstream, **upstream returned 503** (here driven by `?statusCode=503` on the sample backend; in real traffic, an actual upstream 5xx). Bug is upstream-side or upstream-config; not the gateway logic.
- `401` + **no `response_code_details`** + `"-"` upstream → request was **rejected by a policy** (here `api-key-auth`) before any upstream selection. Bug is in policy config / api-key store.
- `404` + `direct_response` → **route not matched** by Envoy at all. Bug is route config from controller (missing/wrong `context`/`operations`).

These three patterns cover most "wrong HTTP status" debug sessions before you
even attach `dlv`.

### 5b. Config dumps — pinpoint **which** component holds wrong state

```bash
curl -s http://localhost:9092/api/admin/v0.9/config_dump      | jq .   # controller (local mode)
curl -s http://localhost:9092/api/admin/v0.9/xds_sync_status  | jq .   # has the push gone out?
curl -s http://localhost:9002/config_dump                     | jq .   # policy engine
curl -s http://localhost:9901/config_dump                     | jq .   # envoy router
curl -s http://localhost:9901/clusters                                  # per-endpoint health
```

`xds_sync_status` returns a tiny `{component, policy_chain_version, timestamp}`
object — its `policy_chain_version` should match what PE's `/config_dump` reports;
if PE lags, that's an xDS push lag.

The first dump where the deployment is missing or wrong is where the bug is.

## Step 6: Attach `dlv`, set breakpoints, find the root cause

Attach to whichever process is suspect:

```bash
dlv connect 127.0.0.1:2345        # controller
# or
dlv connect 127.0.0.1:2346        # policy engine
```

### Discover function names — never use `file:line` (drifts on edit)

```
(dlv) funcs <regexp>
```

Common targets (validated patterns that resolve in the current tree):

| Bug area | Useful `funcs` patterns |
|---|---|
| `POST /rest-apis/{id}/api-keys` (generate) | `funcs handlers.*APIServer.*CreateAPIKey` → `(*APIServer).CreateAPIKey` |
| Regenerate / revoke / list api key | `funcs handlers.*APIServer.*APIKey` |
| Generic REST handler entry | `funcs handlers.*APIServer.*Handler` |
| Controller xDS push side | `funcs gateway-controller.*xds.*Push` · `funcs xds.*Snapshot` |
| Controller api-key snapshot/xDS | `funcs apikeyxds.*StoreAPIKey` · `funcs apikeyxds.*UpdateSnapshot` |
| PE per-request execution | `funcs executor.*ChainExecutor.*Execute` (Header / Request / Response / Streaming variants) |
| PE xDS receive side | `funcs kernel/xds` · `funcs registry.*` |
| Route metadata / upstream resolution (PE) | `funcs kernel.*Mapper` · `funcs translator.*Translate` |

Then break by fully qualified name:

```
(dlv) break github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers.(*APIServer).CreateAPIKey
(dlv) breakpoints
(dlv) continue
```

The process is already running (`--continue`), so the breakpoint fires the next
time that code path executes — re-trigger it by repeating the `curl` from Step 4
in another terminal. When it hits, the prompt also prints the function signature
(`func (s *APIServer) CreateAPIKey(c *gin.Context, id string)`) — use **those
names** for `print`, not your guess:

```
(dlv) args                       # list arg names + values (the canonical source)
(dlv) print id                   # exact name from the signature
(dlv) bt 6                       # confirm the call path
(dlv) next / step                # walk through the function
(dlv) on <bp-id> print <expr>    # one-shot eval on every hit
(dlv) clear <bp-id>              # disable when done
```

To leave the process running for another reproducer (don't kill the server):

```
(dlv) continue          # IMPORTANT — resume the program first (see warning below)
(dlv) exit
Would you like to kill the headless instance? [Y/n] n
```

`--accept-multiclient` keeps the dlv server alive; reconnect any time.

> **Always `continue` before `exit` if you've run an inspection command.**
> Read-state commands — `goroutines`, `funcs`, `threads`, `regs`, `stack`
> when issued against a *running* program — auto-halt it to compute their
> answer and leave it halted afterwards. `exit` then disconnects the
> client but the program stays paused, and external clients (your `curl`
> reproducer in another terminal) hang or fail with connection-refused
> until the next dlv client resumes it. Symptoms: `curl ... -w "%{http_code}\n"`
> returns `000`, ports still show LISTEN in `lsof`, no errors in the dlv log.
> Issue a final `continue` and the program runs again. `print`, `args`,
> `locals`, `bt`, `breakpoints` are safe — they only apply when already
> stopped at a breakpoint, so they don't change the running/paused state.

### 6a. Driving dlv non-interactively (scripted repro)

For a hands-off repro: trigger the request in the background, then pipe
`dlv connect` a script that breaks, `continue`s (waits for the hit),
inspects, then `continue`s again to release the request:

```bash
# Trigger 4 s after dlv connects, in the background
( sleep 4
  curl -s -u admin:admin -X POST \
    'http://localhost:9090/api/management/v0.9/rest-apis/sample-echo-v1/api-keys' \
    -H 'Content-Type: application/json' -d '{"name":"bp-key"}'
) &

printf '%s\n' \
  'break github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/handlers.(*APIServer).CreateAPIKey' \
  'continue' \
  'args' \
  'bt 4' \
  'continue' \
  'exit' 'n' \
  | timeout 30 dlv connect 127.0.0.1:2345
```

The `exit` + `n` keeps the dlv server alive (so the controller stays running)
while letting your script return.

## Step 7: (Optional) Apply a fix → restart → re-verify

**Skip this step unless the user asked you to fix the issue, not just find
it.** When the request was "debug X" / "investigate X" / "step through X",
stop at Step 6 — report the root cause and let the user decide what to do
next.

When fixing is in scope: `dlv debug` does **not** hot-reload — code changes
require restarting the debugged process. Loop:

1. Edit the source.
2. Stop the dlv-controlled process:
   ```bash
   # Controller
   lsof -nP -iTCP:2345 -sTCP:LISTEN -t | xargs -r kill
   # Policy engine
   lsof -nP -iTCP:2346 -sTCP:LISTEN -t | xargs -r kill
   ```
   This kills the `dlv` parent, which signals the debugged binary.
3. Re-launch from the same Step 3 block (dlv recompiles with your fix).
4. Re-run the Step 4 reproducer and confirm behaviour now matches expectations.
   With `sample-echo-api.yaml` the assertion is straightforward — the echoed JSON
   either matches or it doesn't.
5. If wrong again → reattach, refine breakpoints, iterate.

For a finalised fix, also run the matching Integration Test feature so the regression is
covered (Step 8 cleanup must run first — the IT stack uses the same ports):

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT/gateway"
make build-coverage                                # rebuild containerised images with the fix
cd it && IT_FEATURE_PATHS=features/<area>.feature make test
```

See [[gateway-integration-tests]] for the full IT workflow.

## Step 8: Clean up — always run when finished

Order matters: revert config first so a stray `docker compose up` doesn't pick
up the debug-only settings.

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"

# 1) Revert the compose file you edited (and config.toml if Option 2B was used).
#    Standalone mode:
cd "$REPO_ROOT/gateway"        && git checkout -- docker-compose.yaml
#    IT-handoff mode (use this one instead, not both):
# cd "$REPO_ROOT/gateway/it"   && git checkout -- docker-compose.test.yaml
cd "$REPO_ROOT/gateway"        && git checkout -- configs/config.toml 2>/dev/null || true

# 2) Stop debugged processes
lsof -nP -iTCP:2345 -sTCP:LISTEN -t 2>/dev/null | xargs -r kill   # controller dlv
lsof -nP -iTCP:2346 -sTCP:LISTEN -t 2>/dev/null | xargs -r kill   # policy-engine dlv
pkill -f gateway-runtime/python-executor/main.py 2>/dev/null      # python executor

# 3) Tear the Docker stack down (from the compose-cwd you chose at Step 2)
#    Standalone:
cd "$REPO_ROOT/gateway"      && docker compose down -v --remove-orphans
#    IT-handoff (use this one instead, not both):
# cd "$REPO_ROOT/gateway/it" && docker compose -f docker-compose.test.yaml down -v --remove-orphans

# 4) (Optional) wipe controller dev DB for a clean next session
rm -f gateway-controller/data/gateway.db*

# 5) Verify ports are free
for p in 9090 9092 8080 9001 9002 2345 2346 18000 18001; do
  c=$(lsof -nP -iTCP:$p -sTCP:LISTEN 2>/dev/null | tail -n +2 | head -1)
  [ -n "$c" ] && echo "  $p: STILL UP — $c" || echo "  $p: free"
done
```

`git checkout -- <COMPOSE>` reliably restores both the
`GATEWAY_CONTROLLER_HOST` env and the PE port mappings to the committed state,
even if Step 2a was edited multiple times. Confirm with
`git diff --stat <COMPOSE>` (should be empty).

## Common failure signatures

| Symptom | Likely cause | Action |
|---|---|---|
| `dlv debug` fails: `plugin_registry.go: no such file` | Builder never ran | Step 2b — run gateway-builder |
| Controller starts but logs `policy definitions path not found` | Builder output missing or wrong path | Step 2b; verify `gateway/gateway-builder/target/output/gateway-controller/policies/` exists |
| Controller starts then exits with `bind: address already in use` on 9090 | Another controller (in Docker, or stale dlv) still holds the port | `docker compose down`; `lsof -nP -iTCP:9090 -sTCP:LISTEN -t \| xargs kill` |
| Router (Envoy) logs `no_healthy_upstream` for the local PE | PE not running, or the compose file still has `GATEWAY_CONTROLLER_HOST=gateway-controller` | Confirm Step 2a edit on the compose file you chose; confirm Step 3b PE listening on `127.0.0.1:9001` (`lsof -nP -iTCP:9001`) |
| `docker compose up` (or `make test` for IT) fails after debugging with PE port collision | Step 2a wasn't reverted | Step 8 `git checkout -- <COMPOSE>` |
| Python policy silently times out after debug session | Step 3d TOML block left in `config.toml` | Step 8 `git checkout -- configs/config.toml` |
| Controller dump shows API but PE dump doesn't | xDS push problem on controller side | Attach to controller (2345), break in `xds.*Push` / `xds.*Snapshot` functions |
| PE dump correct but request mishandled at router | Policy execution bug | Attach to PE (2346), break in `executor.*ChainExecutor.*Execute` or the policy's `OnRequest*` |
| `dlv` `print <var>` fails: `could not find symbol value for <name>` | Local var name guessed wrong | Use `args` / read the function signature dlv prints at the hit — those are the canonical names |
