# Quickstart: Validating Model Failover

**Feature**: 001-model-failover-policy | **Date**: 2026-10-22

How to prove the feature works end to end. Details of configuration fields live in
[contracts/model-failover-policy.yaml](./contracts/model-failover-policy.yaml); entity and state
rules live in [data-model.md](./data-model.md).

---

## Prerequisites

- Docker available (`make check-docker` from `gateway/it`)
- Gateway images built with **`make build-coverage`**, not `make build`: the IT stack runs the
  `*-coverage:test` images, and `make test` only retags what `build-coverage` produced
- **`model-failover` is NOT yet pinned in `gateway/build-manifest.yaml`** and is absent from the
  committed `gateway/build.yaml`. Until T040 publishes the module, it is built from a working copy
  through an uncommitted `filePath: ./dev-policies/model-failover` entry, populated by
  `gateway/sync-dev-policies.sh`. A clean checkout therefore builds a gateway with no such policy —
  which is also why nothing here guards anything in CI yet (T029b). The manifest already records
  `model-round-robin version: v1.2.0`, a tag that does not exist either; that is the merge gate
  described in T040, not a prerequisite anyone can satisfy today.
- A backend that can fail on demand. **Not** the IT sample backend: `sample-service` is a pure echo
  that reports `?statusCode=` back in its `query` field and still answers `200`, so it cannot force
  a status (T029c). Use `echo-backend-multi-arch` (`mccutchen/go-httpbin`), already in the compose
  file, which answers `/status/NNN` exactly, `/status/200:2,503:1` as a weighted random draw, and
  `/delay/N` after N seconds. It matches the path exactly, so the status or delay goes in the
  OPERATION path with `basePath` omitted, and chain members are differentiated by HOST.

## Running

```bash
cd gateway/it
make test                       # default suite (load scenarios excluded)
make test-verbose               # verbose
make test-load                  # ONLY the load scenarios
make test-faultflow             # failover features, with the fault flow handling failures

# This feature only. The warm-up prefix is not optional: at cold start Envoy
# answers the first requests with 503 no_healthy_upstream for several seconds
# while clusters warm and xDS syncs, and the per-feature readiness wait is
# shorter than that. Running health + api_deploy first reproduces the warm
# gateway this feature sees inside the full suite.
IT_FEATURE_PATHS=features/health.feature,features/api_deploy.feature,features/model-failover.feature make test
```

Tag selection is `IT_TAGS` (default `~@load && ~@faultflow`), NOT godog's `-godog.tags` flag — the suite builds
`godog.Options` directly and never calls `BindCommandLineFlags`, so that flag is undefined and
`go test ... -godog.tags=@model-failover` exits 2 before any test runs. It is also worth knowing
that `IT_TAGS` selects only WITHIN the features already loaded by `Paths`: setting a tag without
also setting `IT_FEATURE_PATHS` to a file containing it selects zero scenarios and the suite
reports `0 scenarios ... PASS` with exit 0 — a false green, not a pass.

New feature files:

- `gateway/it/features/model-failover.feature` — `@model-failover`
- `gateway/it/features/model-failover-routing-layered.feature` — `@model-failover @routing-layered`

---

## V0 — Mechanism preconditions (verify before building)

The mechanism itself was proven in the Stage 0 POC — see
[poc-results.md](./poc-results.md). Two settings from that work are load-bearing and must be
locked in by tests rather than care:

**V0.1 — `refresh_cluster_on_retry` is set.** Deploy with it `false` and confirm the failure mode:
Envoy retries the **same** failing provider for every attempt and returns its error, while the
healthy next target receives nothing. Then set it `true` and confirm traversal works. Assert the
generated config carries it.

**V0.2 — route cache cleared after the metadata write.** Publish an order that differs from the
configured group order and confirm **attempt 1** honours the published order. Without the cache
clear, attempt 1 silently uses the configured order and only retries honour the override.

**V0.3 — upstream chain identifies its provider.** Confirm the upstream-position filter can tell
which provider cluster the current attempt selected, so credentials and translation apply per
attempt.

**V0.4 — mutated body reaches attempt 2.** Mutate the request body downstream, fail attempt 1, and
assert the **mutated** bytes reach the second provider. Guards a security control; a failure here is
blocking (FR-012b).

## V1 — Fallback to a healthy target (User Story 1, FR-008)

Deploy an operation with failover over a forced-to-fail primary and a healthy fallback; send one
request.

- **Expect**: `200`, body served by the fallback target
- **Expect**: no retry logic in the client, and no `5xx` surfaced
- **Contrast with today**: the same scenario on `model-round-robin` alone returns `500`
  (`gateway/it/features/model-round-robin.feature:849`) — that contrast is the regression guard

Also assert: a non-failover-worthy failure (e.g. `400`) is returned unchanged and triggers no
fallback (FR-005 scenario).

## V2 — Same model, two providers (User Story 2, FR-017)

Two targets naming the same model under different provider configurations.

- **Expect**: the second attempt uses the second provider's credentials and endpoint
- **Expect**: suspending the first leaves the second's health untouched

## V3 — Cross-provider with adaptation and last-resort tail (User Story 3, FR-001b)

Primary on vendor X, fallback on vendor Y configured **only** on the failover policy while a routing
policy spreads load across in-house models.

- **Expect**: while in-house models are healthy, vendor Y receives **zero** traffic
- **Expect**: when all in-house models fail or are suspended, vendor Y serves the request
- **Expect**: usage and cost attribution name vendor Y (FR-016)

## V4 — Suspension and escalating backoff (User Story 4)

- Fail a target `consecutiveFailures` times ⇒ suspended; subsequent requests skip it with no attempt
- Before `durationSeconds` elapses ⇒ no normal traffic reaches it
- After it elapses ⇒ target enters a **half-open probe phase**, admitted one request at a time until
  `recoverAfterSuccesses` consecutive probes succeed; any failed probe re-suspends it at once,
  without waiting for `suspendAfterFailures`
  <br>*(Corrected 2026-11-05 while executing T093. This bullet read "returns to full rotation
  immediately, no probe phase", which is the behaviour FR-019 was revised AWAY from on 2026-10-28 —
  the spec records that revision and why. The implementation half-opens; the quickstart and T060's
  task text were never updated with it.)*
- Fails again ⇒ suspended for **double** the previous duration
- Keeps failing ⇒ duration stops growing at `maxDurationSeconds`
- Stays healthy for `backoffResetSeconds` ⇒ next suspension starts from the base duration
- Also assert the windowed rules: a target failing ~1 in 3 (never consecutively) suspends once
  `minimumRequests` is met; a target whose p95 crosses the threshold suspends even though no single
  request exceeded `perAttemptTimeoutMs`
- A success before the threshold ⇒ counter resets, never suspended
- All targets suspended ⇒ the exhaustion error, immediately, with no per-attempt timeouts burned

## V5 — Ranking traversal (User Story 6, FR-021/FR-022)

Routing policy publishing `[B, C, A]`, failover attached, B forced to fail.

- **Expect**: C attempted next, then A — the published order exactly, with no fallback to a
  configuration-defined order
- **Expect**: the written target list matches the published ranking, with suspended targets removed
- **Expect**: the downstream chain executes **once** — assert a single policy-chain invocation and no
  second classification/scoring call
- **Expect**: once B is suspended, published rankings exclude it and load spreads over A and C
- **Expect**: a non-adopting routing policy (single selection) still works via the one-element-list
  path, continuing into the configured chain (FR-023)

## V6 — Body-mutation preservation (FR-012a/b/c) — **security-critical**

Attach a masking policy (e.g. `pii-masking-regex`) **and** failover; force the primary to fail.

- **Expect**: the fallback target receives the **masked** content
- **Expect**: if mutations cannot be preserved, the attempt **fails and is recorded** — the fallback
  never receives pre-masking content, and the condition is never silent

This is the test FR-012c mandates. A comparable product shipped this exact defect
([agent-router#2671](https://github.com/theagentrouter/agent-router/issues/2671)) with no signal
emitted when the mutation was discarded. Assert on the bytes the upstream received, not on the
policy's own logs.

## V7 — Once-per-client-request side effects (FR-012d, SC-011)

Attach rate limiting and token/cost accounting; drive a request that fails over twice.

- **Expect**: one unit of quota consumed, one rate-limit count, tokens billed for the serving attempt
  only, one analytics record
- **Expect**: a policy with **no declared execution class** behaves as once-per-request (FR-012f)

## V8 — Exhaustion, streaming safety, and sterile errors (FR-013, FR-014)

- Every target failing ⇒ one consistent exhaustion error; no upstream body, provider hostname, or
  credential in the response
- A streaming request that fails **after** headers/first bytes ⇒ **no** failover; the client is told
  the response failed rather than receiving two concatenated answers
- Client disconnects mid-chain ⇒ traversal stops

## V9 — No regression for existing deployments (FR-026c, SC-012)

Deploy a routing policy **without** failover, exactly as today.

- **Expect**: identical behavior, including its own `suspendDuration`
- **Expect**: zero existing configurations require editing or are rejected at deployment

## V10 — Deployment-time behaviour (FR-006, FR-006a/b, FR-030b/e)

| Input | Expect |
|---|---|
| Numeric setting outside its range | **Rejected**, actionable message — schema error |
| Target naming an unknown model/provider | **Accepted**; that target skipped, chain built from the rest |
| No `targets` **and** no ranking-publishing routing policy | **Accepted**; policy inert |
| Policy attached to a kind with no model concept | **Accepted**; policy inert |
| Any inert case above | Behaves identically to the policy not being attached: same routing, same retries, same responses, **no telemetry about being inert** |
| `perAttemptTimeoutMs` ≥ operation request timeout | **Not implemented.** No such constraint exists: the schema bounds `perTryTimeout` only by `minimum: 0`, and nothing compares it to `RouteTimeoutMs` (60s, terminal when it fires). A value at or above it is accepted and silently inert — T030p, open. |

*(Row removed 2026-11-05 while executing T093: "Routing policy's own suspension setting superseded ⇒
reported at deployment (FR-026b)". FR-026b was **withdrawn** on 2026-10-29 — with no override to
apply there is nothing to report — and its task T015 was withdrawn with it. The row required
behaviour the spec no longer asks for.)*

---

## Observability checks (FR-029 – FR-034)

For a request that failed over: telemetry names each target attempted, each failure reason, the
ranking source, and the serving target; suspension and recovery emit records; counters exist per
target for failovers, suspensions, recoveries, and exhaustions. **Assert the absence** of
credentials, authorization headers, and prompt/completion content.

## Local-only checks (not run by CI)

Two scenarios carry the `@load` tag and are excluded from the default suite, because a
10,000-request soak would add meaningful time to every CI run. Run them locally when touching
traversal, at-most-once, or latency behaviour:

```bash
cd gateway/it
make test-load          # runs @load scenarios only
```

- **Sustained load (SC-008)** — ≥10,000 requests with one target failing: no target attempted more
  than once per request, chain never loops.
- **Failover latency (SC-003)** — added latency during a primary outage stays within the per-attempt
  limit plus the fallback's response time, and drops to ~zero once the primary is suspended.

Because CI does not run these, a change that breaks at-most-once under load will pass CI. Run them
before merging anything that touches chain traversal.

### Fault-flow posture (FR-030k/l/m)

```bash
cd gateway/it
make test-faultflow     # failover features, with the fault flow handling upstream failures
```

A separate target rather than a tag, because `handle_upstream_faults` is a process-level setting
read from the policy engine's config — no env override, no per-API form — so selecting it means
starting a differently-configured stack. The target layers a one-stanza config fragment and a
small compose override onto the base; the engine accepts `-config` more than once and merges
last-wins, so nothing duplicates the base config and nothing can drift from it.

Worth its own run because with the flag on the gateway runs a failing response through the fault
chain *instead of* the response flow, which for model-failover is exactly the set of responses it
grades. A regression there leaves the policy not degraded but inert — nothing recorded, nothing
suspended, the upstream's own error reaching the client — while every scenario in the default run
still passes. The failure mode is silence.

For the same reason the run validates its own posture rather than trusting the Makefile: the
`@faultflow` scenario asserts the engine actually diverted the response before it asserts anything
about failover. Confirmed in both directions — it passes under `make test-faultflow`, and fails at
exactly that step when run against the ordinary stack. `@faultflow` is excluded from every other
target, because in an ordinary run those scenarios would not merely be irrelevant, they would fail.

## Docs to update in the same PR

Per `.claude/rules/feature-docs.md`, run
`grep -rl "resource: /gateway" kb` and update any feature doc whose `sources:` lists a touched path.
