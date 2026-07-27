---
name: gateway-integration-tests
description: Run or debug WSO2 API Platform gateway integration tests. Use when the user asks to run gateway Integration Tests (IT), BDD tests, godog tests, or a specific `.feature` file; verify a feature file passes; or debug why a gateway integration test is failing.
compatibility: Requires docker (with compose plugin), make, Go 1.23+. Optional: delve (`dlv`) for step-level debugging.
allowed-tools: Bash Read Edit Grep Glob
---

# Gateway Integration Tests

Runs the WSO2 API Platform gateway BDD integration suite (`gateway/it/`, Godog +
Docker Compose) for a chosen subset of feature files, diagnoses failures from
container logs, and cleans up afterward.

## When to use

- "Run the gateway integration tests" / "run `<x>.feature`"
- "Verify my new feature file passes"
- "Why is this Integration Test failing?" — triage from container logs
- After editing a `.feature` file, a policy, or gateway code, to confirm behavior

## Workflow

Always run these steps in order:

1. **Build the gateway images** — `cd gateway && make build-coverage` (see Step 1).
   Do this first, every time, so the test stack runs the current code.
2. **Run the tests** for the chosen feature files via `IT_FEATURE_PATHS` (see Step 2).
3. **Troubleshoot** from container logs if anything fails (see Step 3).
4. **Clean up** the stack with `make clean` (see Step 4).

## Layout

- Suite dir: `gateway/it/` — run all `make` commands from here.
- Feature files: `gateway/it/features/*.feature`. Default suite list (and how
  `IT_FEATURE_PATHS` overrides it) is in `gateway/it/suite_test.go` → `getFeaturePaths()`.
- Compose stack: `gateway/it/docker-compose.test.yaml`. Containers are named
  `it-<service>` (`it-gateway-controller`, `it-gateway-runtime`, `it-sample-backend`,
  `it-echo-backend`, `it-mock-platform-api`, `it-mock-jwks`, …).
- Failure logs: on any scenario failure the suite dumps all container logs to
  `gateway/it/logs/suite_failure_<timestamp>.log`.

## Step 1: Build the gateway images (do this first, always)

The test stack runs from `*-coverage` images. Build them **before every test run**
so the stack reflects the current code — any change to gateway-controller,
gateway-runtime, policy-engine, or SDK code is otherwise invisible to the tests:

```bash
cd gateway && make build-coverage      # builds gateway-{controller,runtime}-coverage:<VERSION>
```

This is slow (~5-15 min). `make test` later retags `:<VERSION>` → `:test` via its
`ensure-test-tags` step. Confirm the images exist afterward:

```bash
docker images --format '{{.Repository}}:{{.Tag}} {{.CreatedSince}}' | grep -E 'gateway-(runtime|controller)-coverage'
```

If a non-gateway test dependency is stale (e.g. `mock-platform-api`), rebuild that
image too — see the Common failure signatures table.

## Step 2: Run the chosen feature files

Run only the features you need with the `IT_FEATURE_PATHS` env var
(comma-separated, paths relative to `gateway/it/`):

```bash
cd gateway/it
IT_FEATURE_PATHS=features/dynamic-endpoint.feature make test
```

Multiple files:

```bash
IT_FEATURE_PATHS=features/health.feature,features/dynamic-endpoint.feature make test
```

`make test` first runs `ensure-test-tags` (retags `*-coverage:<VERSION>` images to
`:test`) then `go test`. Total wall time is dominated by Docker Compose startup
(~60-90s) plus the scenarios.

### Cold-start caveat — IMPORTANT

When a feature runs **first** against a freshly-started gateway, the first requests
can return `503 UH no_healthy_upstream` for several seconds while Envoy clusters
warm and xDS syncs. The per-feature readiness wait (`I wait for the endpoint ... to
be ready`) is only ~9s, which can be too short at cold start.

To verify a single feature reliably, prepend warm-up features so the gateway is hot
by the time yours runs:

```bash
IT_FEATURE_PATHS=features/health.feature,features/api_deploy.feature,features/<target>.feature make test
```

This mirrors how the feature runs inside the full default suite (where it executes
after ~40 other features).

## Step 3: Troubleshoot a failure

When scenarios fail, find the **root cause** — do not just rerun. Work through:

### 1. Read the suite output
The summary table lists which scenario failed and the failing step. A failure at
`Given the gateway services are running` or at a readiness wait means the stack
itself is unhealthy — go to step 2. A failed assertion means the gateway did
something wrong — go to step 4.

### 2. Check container health and exit codes

```bash
docker ps -a --filter 'name=it-' --format '{{.Names}}\t{{.Status}}'
```

If a container `Exited (1)`, read its logs:

```bash
docker logs it-gateway-controller 2>&1 | tail -50
docker logs it-gateway-runtime    2>&1 | tail -50
```

The gateway-runtime log interleaves three sources — filter by prefix:
- `[rtr]` — Envoy router (access logs, routing errors)
- `[pol]` — policy engine (ext_proc, policy execution)
- unprefixed — the container entrypoint / process manager

### 3. Use the dumped failure log
The suite writes every container's logs to one file on failure:

```bash
ls -t gateway/it/logs/suite_failure_*.log | head -1
```

Grep it for the API/policy under test, then narrow to errors:

```bash
latest=$(ls -t gateway/it/logs/suite_failure_*.log | head -1)
grep -iE '<api-or-policy-name>' "$latest" | grep -iE 'error|warn|fail|panic|reject|503|no_healthy' | head -40
```

### 4. Reproduce manually for assertion failures
Bring up only the services you need, deploy the artifact by hand, and inspect.
The controller exposes two REST APIs (basic auth `admin:admin` — the plaintext credential hardcoded in
the IT fixture `gateway/it/test-config.toml`, distinct from the shipped `config.toml`, which provisions
its admin credential via `setup.sh`); the router is `http://localhost:8080`; Envoy admin is
`http://localhost:9901`.

- Management API — `http://localhost:9090/api/management/v0.9` — data-plane
  artifacts: REST APIs, LLM providers, LLM proxies, MCP servers, …
- Admin API — `http://localhost:9090/api/admin/v0.9` — admin resources: secrets,
  API keys, …

**Do not hard-code resource paths.** Resolve the correct path, method, and
payload shape from the OpenAPI specs in the repo before issuing the request —
artifact types and field names drift, and the spec is the source of truth:

- `gateway/gateway-controller/api/management-openapi.yaml`
- `gateway/gateway-controller/api/admin-openapi.yaml`

Read the spec, pick the operation matching the artifact under test (e.g.
`POST /rest-apis`, `POST /llm-providers`, `POST /llm-proxies`, `POST /mcp-servers`,
`POST /secrets`, `POST /api-keys`), then issue it:

```bash
cd gateway/it
docker compose -f docker-compose.test.yaml up -d gateway-controller gateway-runtime sample-backend mock-platform-api mock-openapi
# Generic shape — substitute <base> and <resource> per the OpenAPI operation:
curl -s -u admin:admin -X POST '<base>/<resource>' \
  -H 'Content-Type: application/yaml' --data-binary @artifact.yaml
curl -s http://localhost:8080/<context>/<path>     # invoke via the router
# inspect each component's config — see step 5 for the config_dump endpoints
```

Then trace the request: policy execution in `[pol]` logs, the upstream chosen in
the `[rtr]` access log (`upstream_cluster`, `response_code`, `response_code_details`).

### 5. Compare config dumps across components

Each component exposes a `config_dump` showing the configuration it actually holds.
Comparing them pinpoints **which** component a deployment went wrong in — config
flows controller → (xDS) → policy engine and Envoy, so the first dump that looks
wrong is the culprit.

```bash
# Gateway Controller — the deployment snapshot it built (APIs, routes, clusters,
# policy chains). Also /xds_sync_status to confirm xDS push state.
curl -s http://localhost:9092/api/admin/v0.9/config_dump | jq .
curl -s http://localhost:9092/api/admin/v0.9/xds_sync_status | jq .

# Policy Engine — what it received via xDS (routes, policy chains, route metadata
# such as upstream definition paths). Admin port 9002.
curl -s http://localhost:9002/config_dump | jq .

# Envoy router — live listeners, routes, and clusters. Admin port 9901.
curl -s http://localhost:9901/config_dump | jq .
curl -s http://localhost:9901/clusters            # endpoint health per cluster
```

Typical use: a request is routed/rewritten wrong → check the controller dump for
the expected route + cluster; if correct there, check the policy engine dump for
the route metadata and policy chain; if correct there, check Envoy's dump. The
component where the expected config is missing or wrong is where the bug lives.

### 6. Pause a live scenario with Delve (last resort)

`make test` is the normal path — reach for Delve only when steps 1-5 above have
not pinpointed the bug and you need to freeze the actual failing scenario.

Run the suite under the Delve debugger (`dlv`). The Compose stack is
daemon-managed — started in `BeforeSuite`, torn down in `AfterSuite` — so it runs
independently of the test process. **While the test is paused at a breakpoint
between those two points the whole stack stays live**, with the scenario's exact
deployed APIs present, and you can `curl` config dumps / invoke APIs from another
terminal. Halting also pauses Go's `-test.timeout` clock, so inspection time is
unlimited.

`dlv test` bypasses the Makefile, so build/tag the images first:

```bash
cd gateway/it
make ensure-test-tags        # or: cd ../ && make build-coverage
IT_FEATURE_PATHS=features/<target>.feature COMPOSE_FILE=docker-compose.test.yaml \
  dlv test . -- -test.run TestFeatures -test.v -test.timeout 0
```

Both `break <file>:<line>` and `break <function>` work. Function-name
breakpoints survive edits to the file, so they're the more durable choice when
you're not sure how many iterations you'll go through — use either as it suits.
Godog step definitions are ordinary Go functions, so you can break inside any
step, mid-scenario — the most useful mode, since you pause *before* things go
wrong with the scenario's APIs already deployed. Map a Gherkin phrase to its
handler by grepping the step registrations:

```bash
grep -rn 'ctx.Step(' gateway/it/      # each step regex → its handler function
```

Then at the `(dlv)` prompt break on that function by name (`funcs <regexp>`
discovers exact names), and `continue` — BeforeSuite starts the stack (~60-90s),
then execution stops at the breakpoint:

```
(dlv) funcs steps.*AssertSteps             # list candidate function names
(dlv) break steps.(*AssertSteps).<method>  # pause on an assertion, response in hand
(dlv) break steps.(*HTTPSteps).<method>    # pause on a request (the deploy step calls SendPOSTToService)
(dlv) continue
```

When stopped inside a step you can `next` / `step` through its code and `print`
locals (`print testState`) — full Go debugging. Meanwhile, from a **second
terminal**, the live stack is reachable: hit any component's `config_dump` as in
step 5, or invoke the scenario's deployed API on the router (`:8080`).

Break at step boundaries, not mid-HTTP-call. Caveat: if you `quit` Delve before
the suite finishes, `AfterSuite` never runs and containers leak — run
`make clean` afterward.

### 7. Switch to source-level gateway debug (when step 6 isn't enough)

Step 6 pauses the **test process**. When the bug is in the **gateway code
itself** (controller deploy handler, xDS push, policy execution) you need to
pause the gateway process — hand off to the [[gateway-debug]] skill, which
runs gateway-controller and policy-engine as host-side processes under `dlv`.
The failing scenario's deploy YAML and request become the manual reproducer.
Map the scenario to one of [[gateway-debug]]'s two paths:

- **Scenario invokes the API on `:8080` through Envoy** (most `.feature` files
  — anything with `When I send a … request to "http://…8080…"`): take
  gateway-debug's **Path B (request-runtime)** and pick the **IT handoff**
  compose target at Step 2 so the IT mocks (`mock-jwks`,
  `mock-platform-api`, …) the scenario depends on stay live.
- **Scenario only hits the controller REST API** (api-keys, secrets, REST
  API CRUD, MCP / LLM-resource CRUD — never touches `:8080`): take
  gateway-debug's **Path A (deployment-only)** — no docker, just Step 3a.

### Common failure signatures

| Symptom | Likely cause | Action |
|---|---|---|
| `it-gateway-controller` exits, log shows `manifest ... 404` | stale `mock-platform-api` image missing an endpoint | `docker compose -f docker-compose.test.yaml build mock-platform-api` |
| `503 UH no_healthy_upstream` only at start | cold-start cluster warmup | add warm-up features (see Cold-start caveat) |
| Assertion mismatch on routed path/host | policy or controller xDS bug | reproduce manually, compare `[pol]` metadata vs `[rtr]` access log |
| `No image found for ...` | coverage images missing | `cd gateway && make build-coverage` |

## Step 4: Clean up — always run when finished

After tests pass (or after troubleshooting), tear the stack down:

```bash
cd gateway/it && make clean
```

`make clean` force-removes `it-*` containers, runs `docker compose down -v
--remove-orphans` for every compose file, and deletes `coverage/` and `reports/`
artifacts. If you manually started a partial stack, `make clean` also covers it.
