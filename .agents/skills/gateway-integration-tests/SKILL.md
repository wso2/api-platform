---
name: gateway-integration-tests
description: Run or debug WSO2 API Platform gateway integration tests. Use when the user asks to run gateway Integration Tests (IT), BDD tests, godog tests, or a specific `.feature` file; verify a feature file passes; or debug why a gateway integration test is failing.
compatibility: "Requires docker, make, Go (the version in tests/framework/go.mod). Optional: delve (`dlv`) for gateway-level debugging via [[gateway-debug]]."
allowed-tools: Bash Read Edit Grep Glob
---

# Gateway Integration Tests

Runs the gateway integration tests of the integration framework (`tests/framework`, Godog +
Testcontainers) for a chosen block, runner or feature, diagnoses failures, and confirms nothing
is left running afterwards. The framework starts every container a block needs itself — the
gateway, a database and the testbench mock backends — and removes them when the run ends.

`tests/framework/README.md` documents every flag; `tests/framework/CLAUDE.md` holds the
conventions for writing features and steps.

## When to use

- "Run the gateway integration tests" / "run `<x>.feature`"
- "Verify my new feature file passes"
- "Why is this integration test failing?"
- After editing a `.feature` file, a step, or gateway code, to confirm behaviour

## Layout

- Framework root: `tests/framework/` — run `go test` from here.
- Suite: `suites/it/it-suite.yaml`. It declares **blocks** (a set of components — gateway,
  database, testbench, platform-api, … — started once) and, inside each block, **runners**
  (lists of feature files that run against that block). Blocks with a database matrix expand
  to one variant per engine: `<block>/sqlite`, `<block>/postgres`, `<block>/sqlserver`.
- Features: `suites/it/features/<area>/…/*.feature`. Each feature file starts with a tag
  (e.g. `@config-dump`).
- Steps: `suites/it/steps/` (`base.go` for generic request/assertion steps,
  `platformgateway/` for gateway resources, `platformapi/` for the control plane).
- Mock backends: the testbench (`testbench/services/*`), reached in features as
  `http://testbench:<port>`.
- Gateway config for a block: for a source build, `gateway/configs/config.toml` overlaid with
  `core/catalog/overlays/gateway-controller-storage.toml` (storage and the test users), the
  analytics overlay, and the block's own `overlay:` (`core/catalog/overlays/*.toml`); a released
  gateway (`-gateway-version`) uses `core/catalog/platformgateway/resources/<version>/config.toml`
  and that version's `gateway-controller-storage.toml` instead of the first two.

## Step 1: Prepare the Docker environment

VM-backed Docker (Rancher Desktop, Colima) needs the environment in the README's "Docker
environment" section — at least `DOCKER_HOST` pointing at the VM's socket and the socket override
that lets Testcontainers' Ryuk reaper start:

```bash
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock
unset TESTCONTAINERS_RYUK_DISABLED   # keep Ryuk: it removes what a killed run leaves behind
```

Build the testbench image once (and again after changing `testbench/**`), and the `ap` CLI
binary that the CLI runners (e.g. `ap-cli-gateway` in `gateway-core`) drive (again after
changing `cli/src/**`):

```bash
make -C tests/framework testbench
make -C tests/framework ap-cli
```

## Step 2: Run the tests

Find the block and runner that run your feature:

```bash
grep -n "<feature-file>.feature" tests/framework/suites/it/it-suite.yaml   # then read up to the block's `- name:`
```

Run one block variant, optionally narrowed to one feature by its tag:

```bash
cd tests/framework
go test ./suites/it -count=1 -v -timeout=60m -blocks gateway-core/sqlite
go test ./suites/it -count=1 -v -timeout=60m -blocks gateway-core/sqlite -feature-tags "@config-dump"
```

- With no `-gateway-version`, the framework **builds the gateway images from source**, so the
  run tests the current code (the first build takes several minutes). `-gateway-version 1.2.0`
  tests a released gateway instead.
- `make -C gateway test-integration` runs the `gateway-core` block on SQLite;
  `make -C gateway test-integration-all` runs the whole suite.
- Only blocks with a database matrix have `/sqlite`, `/postgres` and `/sqlserver` variants;
  single-engine blocks (e.g. `gateway-mtls-trust-any`) are selected by their bare name.
- Use `-feature-tags`, never `-tags`, and `-block-parallel`, never `-parallel` (see the README).
- A source build rewrites `gateway/build-manifest.yaml` with the newest policy versions; restore
  it (`git restore gateway/build-manifest.yaml`) before committing.
- A selection that runs no scenario fails the suite, so `ok` means scenarios ran. With `-v`,
  each runner prints `N scenarios (N passed)`.

## Step 3: Troubleshoot a failure

Find the **root cause** — do not just rerun.

1. **Read the Godog output.** It names the failed scenario and step, with the request and the
   response body. A failure at boot (`failed to boot`) or at `the gateway services are running`
   means the stack is unhealthy; a failed assertion means the gateway behaved differently.
2. **Capture container logs.** Rerun with `-logs`; each block's combined container output is
   written under `IT_LOG_OUT` (default `suites/it/logs-out/`):

   ```bash
   IT_LOG_OUT=$PWD/suites/it/logs-out go test ./suites/it -count=1 -v -blocks <block>/sqlite -logs
   ls suites/it/logs-out/.run-*/blocks/
   ```

   The gateway-runtime output interleaves `[rtr]` (Envoy router: access logs, routing errors) and
   `[pol]` (policy engine: ext_proc, policy execution).
3. **Inspect a running stack.** While a run is in progress the gateway's containers are named
   `<block>[-<db>]-platform-gateway-<id>-gateway-controller-1` (and `…-gateway-runtime-1`); ports
   are mapped dynamically:

   ```bash
   docker ps --filter name=platform-gateway --format '{{.Names}}\t{{.Status}}'
   docker port <controller-container> 9090    # management API
   docker port <controller-container> 9092    # admin API: config_dump, xds_sync_status
   ```

   The management API is `/api/management/v1` (port 9090) and the admin API `/api/admin/v1`
   (port 9092), with basic auth `admin:admin` (set by the gateway-controller storage overlay, for
   source builds and released versions alike).
   The stack is torn down when the run ends, so for longer hands-on reproduction use the
   standalone stack in [[gateway-debug]].
4. **Resolve request shapes from the OpenAPI specs**, not from memory:
   `gateway/gateway-controller/api/management-openapi.yaml` and
   `gateway/gateway-controller/api/admin-openapi.yaml`.
5. **Compare config dumps** to find which component holds the wrong configuration — config flows
   controller → (xDS) → policy engine and Envoy, so the first dump that looks wrong is where the
   bug is: the controller's `/api/admin/v1/config_dump` (and `/xds_sync_status`), the policy
   engine's `/config_dump` (port 9002) and Envoy's `/config_dump` and `/clusters` (port 9901).
6. **Debug the gateway itself** when the bug is in gateway code (controller deploy, xDS push,
   policy execution): hand off to [[gateway-debug]], which runs gateway-controller and
   policy-engine under `dlv` against the standalone stack. The failing scenario's resource values
   and request become the manual reproducer.

### Common failure signatures

| Symptom | Likely cause | Action |
|---|---|---|
| `failed to boot … all predefined address pools have been fully subnetted` | Networks leaked by earlier runs with Ryuk disabled | Remove them: `docker network rm $(docker network ls -q --filter label=io.wso2.apip.it=integration-v2)` |
| `reaper: … mkdir /Users/…/docker.sock: operation not supported` | Ryuk mounting the host socket path on a VM-backed Docker | Set `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock` |
| `"Started" matched 0 times` (Colima) | Docker environment incomplete | Set the variables in the README's "Docker environment" section |
| Boot fails starting the testbench (image not found) | Testbench image not built | `make -C tests/framework testbench` |
| `ap binary not found at …/cli/src/build/ap` | `ap` CLI not built | `make -C tests/framework ap-cli` |
| Behaviour differs from current code | Run used released or stale images | Run without `-gateway-version` so the gateway is built from source |
| `topology: no block matches <name> (available: …)` | Unknown block or variant name | Pick a name from the list the error prints |
| `the suite executed NO scenarios` | The blocks matched, but no runner or feature did (e.g. a `-feature-tags` tag no selected block runs) | Check the feature's tag and that its runner is in the selected block |

## Step 4: Confirm cleanup

A run removes its containers and networks when it ends; if the process was killed, Ryuk removes
them shortly afterwards (allow up to a minute). Check nothing is left:

```bash
docker ps -a --filter label=org.testcontainers=true
docker network ls --filter label=io.wso2.apip.it=integration-v2
```

If a run was killed with Ryuk disabled, remove the leftovers: the networks by the label above, the
gateway stacks with `docker compose -p <project> down -v` (the project is the container name
before `-gateway-controller-1`), and other containers by `label=org.testcontainers=true`.
