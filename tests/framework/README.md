# integration-v2 framework

One integration/E2E framework for every api-platform suite: declarative multi-component
topologies, safe parallelism, framework-provisioned infrastructure (containers, databases,
schema), dynamic port mapping, scoped state, resource cleanup, and a tagged coverage taxonomy.

See [`docs/implementation-plan.md`](docs/implementation-plan.md) for the phased plan and the
locked design decisions, and [`docs/capability-map.yml`](docs/capability-map.yml) for the
`@cap`/`@feat` vocabulary.

## Quick start

Everything runs from **this directory** (`tests/framework`) — the suite path below is
relative to it, and the coverage output root defaults under it.

```sh
cd tests/framework

# VM-backed docker (Colima, Rancher Desktop) needs the environment described in
# "Docker environment" below. On Colima with the grpc port forwarder disabled
# (--port-forwarder=none), the THIRD variable is also required — mapped ports answer
# on the VM's address, not 127.0.0.1:
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="/var/run/docker.sock"
export TESTCONTAINERS_HOST_OVERRIDE="$(colima ls -j | jq -r '.address')"

# One block variant (the usual focused run while working on a feature):
go test ./suites/it -count=1 -timeout=30m -blocks=gateway-core/sqlite

# The full matrix (all database engines), bounded concurrency:
go test ./suites/it -count=1 -timeout=45m -blocks=gateway-core -block-parallel=3
#   On Apple silicon, cap coverage runs at -block-parallel=2: the arm64 SQL Server
#   substitute crashes probabilistically when three instrumented stacks cold-boot at once.

# Server-side coverage (needs `make build-coverage` in gateway/ first):
go test ./suites/it -count=1 -timeout=45m -blocks=gateway-core -block-parallel=2 -coverage
make coverage-report      # merges suites/it/coverage-out, prints percents, renders HTML

# The UI suite (browser-driven AI Workspace journeys against the real gateway):
go test ./suites/ui -count=1 -timeout=25m
```

Prerequisite images: `make build` in `gateway/` (product images, pinned by `gateway/VERSION`),
`make testbench` here, and `make build-coverage` in `gateway/` for `-coverage` runs. See
[`docs/coverage-architecture.md`](docs/coverage-architecture.md) for what coverage collects
and how.

The UI suite additionally needs the `ai-workspace` and `platform-api` images (built by their
components' `make build`) and pulls `mcr.microsoft.com/playwright` for the in-network browser;
the playwright driver installs itself on first run. Failing scenarios leave a full-page
screenshot and the DOM under `suites/ui/artifacts/`. See
[`docs/ui-testing-architecture.md`](docs/ui-testing-architecture.md) for the architecture
and the execution findings.

## Layout

```
framework/
  core/components/  component contracts and definitions
  core/catalog/     component packages and registry
  core/topology/    YAML schema, loader, validation, block selection
  core/runtime/     networks, databases, containers, Compose, readiness
  core/cleanup/     owner-tagged resource registry
  core/util/        HTTP, retry, runner context, and unique-name helpers
  core/taxonomy/    capability-map lint and coverage-tree renderer
suites/
  it/  ui/  event-gateway/  e2e/  cli/  portals/
```

Each package's `doc.go` carries its charter and the invariants it exists to protect. Read those
before changing one — most encode a failure mode that presents as a passing test.

## The division of labour

**Go defines components. YAML composes them.**

A component definition (image, ports, alias, wait strategy, config injection, typed wiring,
accessor set, DB contract) lives in Go. The suite YAML references components by name and
declares topology: which blocks exist, which components each needs, what DB, what overlay,
which features run where, and how much runs in parallel.

If a topology need cannot be expressed by composing existing components, the answer is a new
component or a new option in Go — not a new construct in the YAML schema. That line is what
keeps the YAML from becoming a second programming language.

## Invariants

These are not style preferences. Each one is here because its absence produces a **green build
that proves nothing**:

- **Boot failure fails the block, never skips it.** A skipped block leaves the build green
  though its containers never came up.
- **Requests clear the stored response before issuing the call.** Otherwise a step that throws
  leaves a stale response for the next assertion to pass against.
- **Registration records the creating actor.** Teardown deletes as that principal, or it 404s
  cross-tenant and the resource leaks while the run stays green.
- **A new resource type needs all four cleanup wiring points.** Registering to a list nothing
  sweeps leaks silently.
- **Every deadline is floored at the shared propagation ceiling.** A call site that drifts below
  it fails as a timeout and hides the real cause.
- **Assert the exact expected value.** A widened assertion (`401 || 403`) passes today and
  swallows a future regression, including a security bypass returning a different 4xx.
- **No package-level mutable state.** It escapes both scopes and is what made the previous
  suite unparallelisable.

## Running

The suite is a normal Go test. From `suites/it`:

```
go test -timeout 30m                                    # everything
go test -blocks gateway-jwt                             # one block
go test -feature-tags "@request-rewrite"                # one feature, any block
go test -blocks gateway-core -feature-tags "@metrics,@certificates"   # ',' is OR
```

Two flags are deliberately NOT named after their `go test` counterparts, because the go tool
consumes any flag it recognises and forwards only the rest:

| use this | never this | what go test would do with it |
|---|---|---|
| `-feature-tags` | `-tags` | build-constraint flag: the filter never arrives and the suite silently runs EVERYTHING |
| `-block-parallel` | `-parallel` | caps `t.Parallel`: the engine nests parallel subtests, so a low value DEADLOCKS the suite after boot |

`-blocks`, `-skip-blocks` and `-runner-parallel` have no builtin of that name and are forwarded
to the binary as normal.

### Docker environment

Nothing here configures docker. The container library reads the environment; on CI
(`ubuntu-latest`) the defaults resolve and no setup is needed.

**On VM-backed docker — Colima, Rancher Desktop — export both of these:**

```sh
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="/var/run/docker.sock"
```

They answer different questions, and **the second is not optional**:

- `DOCKER_HOST` — where *this process* dials the daemon. The host path.
- `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE` — the path the library *bind-mounts into Ryuk*,
  which must be the path the **daemon** sees. Set only the first, and Ryuk is handed a path
  that is useless inside a container.

A third variable applies when Colima runs with its grpc port forwarder disabled
(`--port-forwarder=none`, the stable configuration on this project — see FINDINGS.md on the
forwarder killing every published port):

```sh
export TESTCONTAINERS_HOST_OVERRIDE="$(colima ls -j | jq -r '.address')"
```

`TESTCONTAINERS_HOST_OVERRIDE` — the address mapped ports are *reachable at*. Without the
forwarder, published ports answer on the VM's address rather than 127.0.0.1; unset, every
wait strategy dials localhost and times out against healthy containers.

Ryuk — the reaper — is **enabled**, and is the crash-safety net: when a test process is
killed outright `t.Cleanup` never runs, and Ryuk removes the orphans.

If you skip the second export, Ryuk fails in a way that names nothing useful. Under Colima
the host socket appears inside the VM as mode 0600 owned by the macOS user; Ryuk runs as a
non-root user, cannot open it, and exits immediately. Its container is created with
`AutoRemove`, so it is gone before its logs can be read, and the failure surfaces as
`"Started" matched 0 times` — a readiness message mentioning neither the socket nor the
permission error behind it. This was once recorded here as an upstream Ryuk defect. It is not
one.

The framework previously set both variables itself, by shelling out to `docker context
inspect`. That was deleted: it is developer environment setup, not something a test framework
should own, and it was the only place the code touched docker outside the container library.

Ryuk is the only reaper. The framework had its own `launcher.Reap` sweep, written when Ryuk
was believed broken here; once that turned out to be our own bug it was a workaround for a
problem that did not exist, never called from anywhere, and it has been deleted. Objects are
still labelled `io.wso2.apip.it` with their block, which is useful for attributing a leak
while debugging.
