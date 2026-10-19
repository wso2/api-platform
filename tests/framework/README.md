# integration-v2 framework

One integration/E2E framework for every api-platform suite: declarative multi-component
topologies, safe parallelism, framework-provisioned infrastructure (containers, databases,
schema), dynamic port mapping, scoped state, resource cleanup, and a tagged coverage taxonomy.

See [`docs/migration-tasks.md`](docs/migration-tasks.md) for the active legacy-feature migration
tasks, and [`docs/capability-map.yml`](docs/capability-map.yml) for the
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

# The ap CLI's ai-workspace commands against platform-api (needs the ap binary, see below):
make ap-cli
go test ./suites/it -count=1 -timeout=30m -blocks=ai-workspace-cli

#   On Apple silicon, cap coverage runs at -block-parallel=2: the arm64 SQL Server
#   substitute crashes probabilistically when three instrumented stacks cold-boot at once.

# Coverage-enabled server and browser run (images are built from the checkout):
go test ./suites/it -count=1 -timeout=45m -blocks=gateway-core -block-parallel=2 -coverage
# The test prints the generated coverage run directory. COVERAGE_OUT accepts either that
# directory or the parent coverage-out, which reports on the most recent run inside it:
#   COVERAGE_OUT=/path/to/suites/it/coverage-out/.run-... make coverage-report
# Reports are written under whichever directory you pass; open its index.html to navigate
# to each component report. A run's counters are never merged with another run's.

# The UI suite (browser-driven AI Workspace journeys against the real gateway):
go test ./suites/ui -count=1 -timeout=25m

# The cloud suite (external APIP cloud; requires a workflow-provided access token):
CLOUD_CONSOLE_ACCESS_TOKEN=<access-token> \
go test ./suites/cloud -count=1 -timeout=15m \
  -args -blocks=apip-cloud -cloud-env=development
```

Prerequisite images: `make testbench` and, for blocks that list `a2a-trip-planner`,
`make a2a-trip-planner` here. The `ai-workspace-cli` block also needs the host `ap` binary:
`make ap-cli` builds it to `cli/src/build/ap` (override the location with `AP_CLI_BINARY`); a
missing binary fails the block rather than skipping it. Coverage runs build source images through the
framework and use the product `VERSION` files for their tags; they do not require a separate
product coverage-image target. See
[`docs/coverage-architecture.md`](docs/coverage-architecture.md) for what coverage collects
and how.

The UI suite builds the `ai-workspace` and `platform-api` images through the framework when
coverage is selected and pulls `mcr.microsoft.com/playwright` for the in-network browser;
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
  core/assets/      resolves repository-relative files: workspace first, embedded copy second
  core/suiterun/    the exported suite entry (Main and Run) any project's test binary calls
  snapshots/        empty in git (.gitkeep); the release generates the embedded gateway schemas, current config and pricing file
suites/
  it/  cloud/  ui/
```

The cloud suite targets externally hosted APIP services. It does not compile product code,
pull product images, create Docker networks, or provision databases. The environment-to-domain
mapping is maintained in the local, ignored `core/catalog/cloudconsole/environments.toml`;
copy `core/catalog/cloudconsole/environments.example.toml` there and add or update an
environment without changing Go code. The CI workflow is responsible for obtaining the cloud
access token; the suite consumes it from `CLOUD_CONSOLE_ACCESS_TOKEN` and never calls the token
provider itself. Its suite defaults match the integration suite: a 5-minute boot deadline
and a 60-second propagation deadline. Cloud data-plane availability and eventual reads use the
propagation deadline; the longer deployment and environment-finalizer waits remain explicit in
their step contracts.

Each package's `doc.go` carries its charter and the invariants it exists to protect. Read those
before changing one — most encode a failure mode that presents as a passing test.

## Using the framework from another repository

The framework is a Go module, `github.com/wso2/api-platform/tests/framework`, released with the
tag `tests/framework/vX.Y.Z` by the `Tests Framework Release` workflow. A project that owns its
features, step definitions and resource templates imports it and runs without an api-platform
checkout:

```go
func TestMain(m *testing.M) { suiterun.Main(m, options()) }
func TestIntegrationSuite(t *testing.T) { suiterun.Run(t, options()) }
```

`options()` returns a `suiterun.Options` carrying the project's step definitions and cleanup
handlers. `Main` registers the suite flags (`-blocks`, `-policies`, `-gateway-version`, `-host`,
`-coverage`, `-logs`), and `Run` loads the suite file beside the test, builds images, applies the
selection and runs the runners.

A repository that keeps each policy's tests next to the policy declares one block with
`runnersFrom: policies` and `addPoliciesFrom: <policy tree>`. Each policy lists its runners in
`<policy>/it/it.yaml`; `-policies=a,b` limits a run to the policies a pull request changed. A
selection that leaves nothing to run, for example because every selected policy needs a newer
gateway, skips the test instead of failing it. Both are checked by `cmd/standards`, which
accepts several feature directories and, with `-approved-imports`, the project's own step
packages.

Feature files name templates as `resources/templates/<kind>.yaml`. The suite directory is
searched first and the framework's canonical templates (the ones api-platform's own suite uses)
are the fallback, so a project keeps only the templates that differ. `Options.CleanupDeleters`
takes `platformgateway.RegisterDeleters`, which removes what a scenario created through the
gateway controller's management API.

Paths in a suite resolve against the *workspace root*, the nearest ancestor of the suite that
holds `go.work` or `.git`, or `Options.WorkspaceRoot`. A policy tree named by `addPoliciesFrom`
is relative to it. The files the framework itself reads from the repository — compose files,
gateway version profiles, configuration overlays, the gateway database schemas, the current gateway configuration and the LLM
pricing file — are embedded: `core/assets` uses the workspace copy when one exists, so a run
inside the api-platform checkout reads live source, and the embedded copy otherwise. The
gateway's key material (listener certificate and key, AES key) is generated per run, never
stored. Only the gateway is embedded today; a block that needs another product's files still
needs the api-platform checkout.

The default testbench image follows the framework version in the importing binary: release
`vX.Y.Z` runs against `ghcr.io/wso2/api-platform/testbench:X.Y.Z`. A framework built from a
checkout, a `replace` or an untagged revision uses the locally built `:test` image, and
`APIP_IT_IMAGE_TESTBENCH` overrides either.

A block can use a testbench hosted outside the run instead of the container. `TESTBENCH_URL` is
the base URL used for every testbench endpoint, `TESTBENCH_URL_<ENDPOINT>` (upper-case, `-` as
`_`, e.g. `TESTBENCH_URL_CONTENT_SAFETY`) overrides one endpoint, and `TESTBENCH_ACCESS_TOKEN` is
sent as a bearer token on the test process's own calls to a testbench service. The URL is used both
by the test process and for upstreams handed to the components under test, so it must be reachable
from both. With none of them set the container is used, exactly as before. The resolver is
`core/catalog/testbench` (`HostURL`, `UpstreamURL`, `AuthHeaders`); steps resolve testbench
addresses through it rather than from the topology.

The embedded copies of files that live outside the module (the gateway database schemas, the
current gateway configuration and the pricing file, listed in `assets.SnapshotSources`) are not
committed: `snapshots/` holds only a `.gitkeep`. The release workflow generates them from the
repository and commits them on the release tag only, so a released module carries them and `main`
does not. Inside an api-platform checkout nothing needs generating, because the engine reads the
repository files. To use the module without a release, through a `replace` or an untagged
revision, generate the copies yourself:

```bash
make snapshots
```

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
go test -blocks gateway-restart                         # one block
go test -feature-tags "@request-rewrite"                # one feature, any block
go test -blocks gateway-core -feature-tags "@metrics,@cors"          # ',' is OR
go test -gateway-version 1.2.0 -blocks gateway-core/sqlite # one Gateway release
go test -gateway-version 1.1.0 -host registry.example/test-gateway -blocks gateway-core/sqlite # released images from another repository
```

`-host` overrides the `ghcr.io/wso2/api-platform` image repository prefix for the
Platform Gateway controller and runtime, and for the `gateway-builder` that builds a policy
image on them, so the repository must publish all three. It must be used together with
`-gateway-version`; the version remains the Gateway release used for configuration,
management paths, and compatibility selection. For example, the command above pulls
`registry.example/test-gateway/gateway-controller:1.1.0`,
`registry.example/test-gateway/gateway-runtime:1.1.0` and
`registry.example/test-gateway/gateway-builder:1.1.0`.

`-gateway-version latest` pulls the images that the `gateway-images` job of `daily-builds.yml`
publishes every day at 03:00 UTC from `main`, and whenever it is started by hand with the
`gateway` component (`gateway-controller`, `gateway-runtime` and `gateway-builder`, tagged `latest`
and `main-<sha>`, where `<sha>` is the newest commit that changed the gateway), so a project that
has no gateway source can test against the current gateway. A merge reaches `latest` at the next
run, so it can be up to a day behind.
It is not a release: it uses the current configuration and contract, runners are matched as they
are for a source build (`gateway-version>=1.2.0` runs, `gateway-version<1.2.0` is skipped), and
policy images built on it are re-pulled each run because the tag moves.

```bash
go test -gateway-version latest -blocks gateway-core/sqlite
```

A runner `features` entry is a path relative to the suite directory, and may point at a feature
file owned by another suite (for example `../it/features/...`). Append `:<line>` to run only the
scenario that starts on that line, e.g. `features/api.feature:34`; the line must be the `Scenario`
or `Scenario Outline` line. An entry without a line runs the whole file.

Two flags are deliberately NOT named after their `go test` counterparts, because the go tool
consumes any flag it recognises and forwards only the rest:

| use this | never this | what go test would do with it |
|---|---|---|
| `-feature-tags` | `-tags` | build-constraint flag: the filter never arrives and the suite silently runs EVERYTHING |
| `-block-parallel` | `-parallel` | caps `t.Parallel`: the engine nests parallel subtests, so a low value DEADLOCKS the suite after boot |

`-blocks`, `-skip-blocks` and `-runner-parallel` have no builtin of that name and are forwarded
to the binary as normal.

Runner `tags` may begin with a framework Gateway-release selector, followed by `;` and an
ordinary Godog expression: `gateway-version>1.2.0;~@known-issue`. The selector accepts only
`>`, `>=`, `<`, `<=`, `=`, or `==` with a release `major.minor.patch` version. It is evaluated
before Godog and is removed from the expression Godog receives; incompatible runners are logged
as skipped. Use this only for a genuine Gateway release compatibility boundary. Keep versioned
configuration in a dedicated block rather than changing a shared block's overlay.

Database support boundaries belong in the suite default for `platform-gateway`, rather than in
each matrix block. For example, Gateway 1.1.0 cannot run SQL Server while 1.2.0 and source builds
can:

```yaml
defaults:
  components:
    platform-gateway:
      db: sqlite
      dbCompatibility:
        sqlserver: "gateway-version>=1.2.0"
```

The selector uses the same strict Gateway-version syntax as runner tags. Incompatible matrix
variants are reported as skipped before their topology boots. Explicitly selecting an incompatible
variant, such as `-blocks gateway-core/sqlserver -gateway-version 1.1.0`, is a configuration error.

### Policy error compatibility

The `policy-compat` blocks prove that changing a policy in gateway-controllers leaves the error
response a client receives unchanged — status, contract headers and body — on every supported
Gateway version. Each scenario drives one policy into its main rejection and checks the response
with `the response should match the expected error response "<id>"`, against
`suites/it/resources/expected-error-responses/<id>.json`. One file serves every Gateway version.
If a version's response genuinely differs, record that version's own file as
`expected-error-responses/<gateway version>/<id>.json`; it wins for that version. The source build's
version is the one in `gateway/VERSION`.

The policies under test come from `../gateway-controllers/policies`, a gateway-controllers
checkout next to this repository: in gateway-controllers' CI that is the pull request. The
expected responses were recorded from the policies as they were before the change being checked.

On a released Gateway, only the policies that release ships are built (`releasedPoliciesOnly`
on the block's `platform-gateway`): those are the ones its users would upgrade. The rest of the
checkout can need what the release's gateway-builder lacks — a newer Go, a newer shared module.

| Runner | Gateway versions | Covers |
|---|---|---|
| `policy-errors-from-1-1-0` | 1.1.0 and later | policies every supported release ships |
| `policy-errors-from-1-2-0` | 1.2.0 and later | policies, and a streamed rejection, first shipped in 1.2.0 |
| `policy-errors-latest` | the current source build | unreleased policies, the Python guardrails, Agent APIs |
| `policy-errors-semantic-cache` | 1.1.0 and later | semantic-cache, in its own block with Redis |

Run them alone, for one Gateway version (`-gateway-version` omitted means the source build):

```bash
cd tests/framework
B=policy-compat/sqlite,policy-compat-semantic-ai/sqlite
go test ./suites/it -count=1 -timeout=60m -v -args -blocks=$B -gateway-version=1.2.0
```

`-blocks` selects only these blocks; the `/sqlite` suffix picks one variant of each block's
database matrix (drop it to run every database). To run one runner, add
`-run 'TestIntegrationSuite/policy-compat/sqlite/policy-errors-from-1-2-0'` before `-args`.

**Recording.** Check out the policies from before the change as `../gateway-controllers`, then
run with `IT_RECORD_EXPECTED_ERRORS=1` on the source build, which runs every scenario. Then run
the other Gateway versions without it: a difference there is either a real incompatibility or a
version that genuinely answers differently. For the latter, copy the shared file into
`<gateway version>/` and record again on that version, which then writes the override. A missing
expected response fails rather than recording itself. Responses are stored with UUIDs,
timestamps, hex IDs and the runner's generated names masked, and JSON bodies are compared
structurally.

**An intended difference** is recorded rather than left failing: run with
`IT_ACCEPT_ERROR_CHANGES=1` and each differing response is written beside its expected response
as `<id>.accepted.json`. A response then passes if it matches either file, and the accepted file
is reviewed like any other change.

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
(`--port-forwarder=none`):

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
