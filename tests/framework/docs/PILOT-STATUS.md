# Pilot status

**Where it stands:** the pilot is COMPLETE. All **21 features / 327 scenarios** are migrated
and run against a real gateway across five blocks, with zero docker residue.

**325 of 327 scenarios pass.** The two that do not are left red deliberately — both look like
product defects, and both are written up with evidence in [FINDINGS.md](FINDINGS.md). Neither
assertion has been relaxed to make the suite green.

## What is proven

YAML → engine → topology boot → compose gateway → generated storage and credentials →
management API → data plane → assertions → cleanup.

Two blocks (`gateway-core/sqlite`, `gateway-core/postgres`) boot concurrently from one matrix
declaration, in ~5s and ~11s, each with its own network, database, credentials and gateway.

## How the data-plane EOF was resolved

The last failure was a bare `EOF` on every invocation, retried 252 times over three minutes —
long enough to rule out propagation timing. It took **two** distinct fixes, and the first
diagnosis was superseded by evidence rather than confirmed by it.

The findings below are based on direct runtime observations and the integration suite.

### Round 1 — xDS pointed at the host

```
listeners after deploy: 200 ""          (empty)
ready after deploy:     503 PRE_INITIALIZING
clusters:               xds_cluster -> 192.168.64.2:18000
```

Block networks use `172.x`, so `192.168.64.2` was not a container at all — it was the Colima
VM host. Cause: the compose stack was creating its **own** network instead of joining the
block's, so `gateway-controller` did not resolve and the address fell back to the host.

Fixed by interpolating the external network name at **staging** time. `WithEnv` is applied
after construction, which is too late for compose to parse — that is why the first attempt at
this fix silently did nothing.

### Round 2 — the controller could not build a listener

Re-probing after Round 1 gave a different picture, and it disproved the remaining half of the
original theory:

```
xds_cluster -> 172.19.0.2:18000     (correct: the real controller, on the block network)
cx_total 1, cx_connect_fail 0, rq_total 1, rq_success 0, rq_error 0, rq_timeout 0
listeners: ""   ready: 503 PRE_INITIALIZING
```

Envoy now reached the controller and opened a stream — which then sat **silent**. One request,
no response, and no error of any kind.

Cause: the framework staged `listener-certs/` next to the compose file but never mounted it.
`[router.downstream_tls]` reads `./listener-certs/default-listener.{crt,key}` relative to the
controller's `WORKDIR` of `/app`, so the controller could not build the listener it is
supposed to push. Legacy `gateway/it` mounts exactly this and is why it never saw the bug.

One added volume produced:

```
listeners after deploy: listener_http_8080::0.0.0.0:8080, listener_https_8443::0.0.0.0:8443
ready after deploy:     200 "LIVE"
```

and the invocation scenario went from a 182s timeout to a 3.04s pass.

### The reusable lesson

An `EOF` at the data plane names nothing about its cause, and the same symptom had **two**
unrelated causes in sequence. Both were found by probing state directly — resolved addresses,
cluster stats, listener sets — and in each case the probe contradicted a plausible theory that
had already been written down as fact. Re-probe after every fix; a partial fix leaves the
symptom identical.

## Concurrency measurements

Both axes measured CROSSED (blocks x runners), each cell twice, 18 cells, 0 failures — full
matrix in **[EXPERIMENTS.md](EXPERIMENTS.md)**. Headlines:

- **The axes compose.** Each keeps paying regardless of the other. End to end, 1 block/1 runner
  to 3 blocks/4 runners is a **4.3x throughput gain** (0.35 -> 1.51 scenarios/sec).
- **Set runner concurrency FIRST.** At 4 runners, adding blocks is nearly free in wall-clock
  (96 -> 99 -> 97s) while scenarios executed more than double. At 1 runner the same blocks cost
  185 -> 250 -> 220s: block concurrency without runner concurrency mostly buys contention.
- **Runners are free, blocks are not.** 1->4 runners: 1.92x for +0 containers. 1->3 blocks:
  2.28x for +5. Per-runner durations are unchanged under 4-way concurrency.
- **The ceiling is the slowest unit**, so balance runners by DURATION, not scenario count
  (this workload spans 2.7s to 79s per runner).
- **R=2 is the worst setting**: every partial-concurrency cell was the noisiest in its row
  (15-45% spread) because wall-clock depends on which pair gets scheduled together.

## Phase 3 has started: the real control plane

`gateway-crossplane` runs the gateway against **platform-api built from source**, not the
596-line mock the legacy suite used. Green: the control plane boots, the gateway registers
with it, and an API deploys and serves while attached.

Three framework capabilities were needed and are now in place:

- **`Definition.Provisions`** — a post-start hook returning environment for DEPENDENTS. The
  gateway's registration token is a hashed row in the control plane's database, minted by
  calling a RUNNING platform-api, so it cannot be declared in YAML or generated in isolation
  the way a password is. Seeding the row directly was rejected: it would duplicate the
  product's hashing scheme and leave the registration endpoint untested.
- **Per-block `dependsOn` in the suite YAML** — the gateway depends on the control plane only
  in blocks that run one. Four blocks run a gateway with no control plane at all, and a
  definition-level dependency would (correctly) fail their validation.
- **Go-generated crypto** (`catalog/pki.go`) — a self-signed TLS pair, an RS256 signing pair
  and an encryption key, replacing two openssl init containers and their shared volumes.

Three assumptions were wrong and were corrected against evidence: platform-api's config key is
`user` not `username`; it self-migrates for SQLITE ONLY, so the framework applies
`schema.postgres.sql` as an operator would; and its API is `v0.9` with a FORM-encoded login on
a different base — a wrong version answers 401 "authorization header missing", not 404,
because auth middleware runs before routing.

The mock audit that scoped this is in [migration-policy.md](migration-policy.md).

## Known framework issue: concurrent runners corrupt each other's output

**Open. Ours, not the product's.** Every runner in a block writes godog output to the same
stdout with no serialisation, so with concurrent runners their lines interleave and shred each
other. Observed repeatedly:

```
14 scenarios (14# < passed)
324 steps (32e API "rrr-invalid-json4 p"assed)
```

The damage is not cosmetic. Scenario totals parsed from a run UNDER-REPORT, because some
summary lines are destroyed: three cells of the concurrency matrix reported 56, 106 and 128
scenarios where the true counts were 64, 147 and 147. It scales with runner concurrency, so
the more parallel the run, the more wrong the number — and a suite that appears to have run
fewer scenarios than it did is the same failure class this framework exists to prevent.

It cost real time during the migration: an early sweep looked like 14 scenarios had silently
vanished, and ruling that out took a separate investigation before the corruption was
identified as the cause.

Mitigation today is to trust only the process exit code and the `--- FAIL` count, both of which
are per-test and unaffected. That is a workaround, not a fix — the reporting should be
per-runner and serialised at the point of write, or godog's output captured per runner and
emitted as a block once the runner finishes.

## Product findings

Two migrated scenarios are RED on purpose, and both look like product defects rather than
migration artifacts. They are written up with evidence, reproduction commands and the checks
to run before filing in **[FINDINGS.md](FINDINGS.md)**:

1. A DELETED api is still served by the data plane, and still present in the policy engine's
   config dump 30s later — while the controller correctly forgets it. This also exposed a
   coverage gap: nothing in the suite, migrated or legacy, ever asserted that deleting an API
   stops it being served.
2. A 503 arrives ~130ms before a configured 6000ms connect timeout, consistently across 12
   runs.

Neither assertion has been relaxed to make the suite green.

## Resolved: shared-container port drift

**Fixed. Not an open issue.** Docker reallocates an ephemerally-published host port every time
a container on a non-default network is connected to or disconnected from another network.
Shared components are the only ones re-homed after start, so their captured address went stale
and jwt-auth failed about half of all three-block runs.

Shared components now publish a DECLARED host port, which docker re-publishes unchanged.
The reasoning is in `framework/core/runtime/container.go` (`Options.StableHostPorts`) and the invariant is
pinned by `framework/core/runtime/sharedport_docker_test.go`, which reproduces the drift in ~1.3s.

Worth keeping in mind only as precedent: product-apim never hit this because it puts every
container on one shared network and never re-homes anything. Per-block networks are the
stronger isolation and worth the cost, but the cost is ours to own.

## Environment limits: none known

There is no known scenario ceiling on this setup. Measured directly:

```
one gateway (gateway-core/sqlite), 232 scenarios, 0 failures, 257s
connection reset by peer: 0    connection refused: 0    EOF: 0
```

An earlier revision of this document asserted a ~100-scenario/gateway transport ceiling caused
by Colima's userspace network stack, complete with a table of ruled-out hypotheses. It does not
reproduce and has been removed rather than left as a caveat. The likeliest explanation is that
it was measured while the docker VM was resource-starved — the same period later found 24.3 GB
of build cache and 18.2 GB of images, and 32 GB was reclaimed.

The lesson worth keeping: every run afterwards was chunked to stay under that "ceiling", which
guaranteed nothing would ever retest it. Re-measure a limit before designing around it.

## Migrated features

All 21, 327 scenarios. Ordered by STEP COST rather than scenario count during migration,
which mattered far more than expected: several large features needed ZERO new step
definitions, while small ones needed several. By the end, most features needed none — which is
what the canonical step vocabulary was for.

| Feature | Scenarios |
|---|---|
| `api_deploy` | 7 |
| `api_keys` | 15 |
| `api_management` | 35 |
| `aws_bedrock_guardrail` | 13 |
| `cel_conditions` | 6 |
| `certificates` | 13 |
| `config_dump` | 10 |
| `content_length_guardrail` | 14 |
| `dynamic_endpoint` | 8 |
| `json_schema_guardrail` | 28 |
| `jwt_auth` | 19 |
| `lazy_resources_xds` | 9 |
| `metrics` | 10 |
| `model_round_robin` | 16 |
| `policy_engine_admin` | 11 |
| `prompt_template` | 14 |
| `regex_guardrail` | 23 |
| `request_rewrite` | 11 |
| `respond` | 29 |
| `sandbox_routing` | 23 |
| `word_count_guardrail` | 13 |

Blocks: `gateway-core` (matrix sqlite/postgres), `gateway-observability`, `gateway-jwt`,
`gateway-bedrock`, `gateway-configdump`. A block exists only where a feature needs a
NON-DEFAULT product configuration; everything else shares `gateway-core` so the suite keeps
testing shipped defaults.

## Migration rules learned so far

Each of these cost a debugging cycle; they are the reusable part.

1. **Aliases must match compose SERVICE names, not `container_name`.** Features point API
   upstreams at `http://testbench:3000` and docker resolves by service name. Getting this
   wrong produces an `EOF` from the data plane that mentions nothing about DNS.

2. **A compose component must join the block's network, not create its own.** Declare it
   external and inject the name. Otherwise the gateway cannot resolve the framework-provisioned
   database, and the error is a DNS failure inside the container.

3. **Escape `$` as `$$` in compose env files.** Compose interpolates short-form `env_file`
   values SELECTIVELY: given `$2a$04$abcDEF.ghi` the container receives `$2a$04.ghi`, because a
   digit cannot start a variable name but `$abcDEF` can. A bcrypt hash is exactly that shape,
   so it is silently truncated and every request then fails authentication.

4. **Assertions about a body must not require a 2xx.** Negative scenarios assert on ERROR
   bodies — "valid JSON", "status is error" — so gating those on success fails every negative
   test for an unrelated reason.

5. **Port deliberate special cases, not just the happy path.** The original's
   `jsonFieldShouldBe` accepts a k8s-style status OBJECT when the expected value is "success".
   Dropping that made a passing deploy look like a failure.

6. **Replace fixed sleeps with bounded polling.** `I wait for 2 seconds` is retained only as a
   migration artifact. Deployment and invocation are asynchronous with each other; the
   management API confirming an API exists says the control plane accepted it, not that the
   data plane routes it yet.

7. **Staging a file is not mounting it.** The framework materialised `listener-certs/` next to
   the compose file, the compose header comment said so, and no volume actually mounted it —
   for weeks. A generated artifact needs a staging step AND a mount, and nothing fails loudly
   when only one is present. When porting a service from an existing compose file, diff its
   `volumes:` list entry by entry rather than reading for intent.

## What changes in a migrated feature

Assertions are untouched. Only addressing changes:

| Before | After |
|---|---|
| `"http://localhost:8080/weather/v1.0/us/seattle"` | `"/weather/v1.0/us/seattle"` |
| hardcoded resource names | `${UNIQUE:base}` expansion |
| implicit cleanup | registered on create, swept by hook |
