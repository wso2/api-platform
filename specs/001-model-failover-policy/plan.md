# Implementation Plan: Model Failover Policy

**Branch**: `001-model-failover-policy` | **Date**: 2026-10-22 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-model-failover-policy/spec.md`

## Summary

> **Agent Router comparison** (source-verified): see [`agent-router-comparison.md`](agent-router-comparison.md) — three adopted changes to filter placement, response phase and baseline state, and why their core failover mechanism is not adopted.

> **Reconciliation with the original design document** (composite clusters, probe recovery, per-provider upstream policies): see [`design-doc-reconciliation.md`](design-doc-reconciliation.md). It records what was carried in, what this spec supersedes, and three open conflicts (C1–C3) that still need a decision.

Add same-request failover for AI proxy APIs: when a target model fails, the gateway re-attempts the
client's request against the next target and returns that target's successful response, so consuming
applications never write their own retry logic. Failover ships as its own policy (`model-failover`)
owning target health, suspension and probe recovery, and composing with the existing routing
policies through an ordered target preference list.

**Mechanism** (decided 2026-10-23, see [research.md](./research.md) §10): Envoy's **`priority_group`
cluster specifier plugin**. The downstream policy chain runs **exactly once** and writes the resolved,
ranked targets into request dynamic metadata as an ordered list of priority groups, each naming
**clusters**. Envoy selects the group by attempt number — attempt 1 takes group 1, the first retry
takes group 2, and so on, staying on the last group once attempts exceed the list. Per-attempt work —
provider credentials, request translation — runs in the **upstream** filter chain, the only chain
Envoy re-invokes on a retry.

This satisfies all three constraints — arbitrary per-request ranking obeyed, downstream chain runs
once, no per-ordering configuration generated — and because groups name clusters rather than
endpoints, the per-provider clusters are used directly, with their own TLS, credentials and health
handling intact.

**Validated on CI before adoption**: the required Envoy 1.40.0 bump passed the full gateway
integration suite, with a 1.39.0 baseline run confirming the suite is healthy on the same fork. Both
green.

## Technical Context

**Language/Version**: Go 1.x (gateway-controller, policy-engine, policy modules); policy modules are
separate Go modules in `wso2/gateway-controllers`

**Primary Dependencies** — two pins move, both validated on CI:

| Pin | From | To | Why |
|---|---|---|---|
| Envoy image (`gateway/gateway-runtime/Dockerfile:24`) | v1.39.0 | **v1.40.0** | `priority_group` ships in 1.40.0 |
| `go-control-plane/envoy` (`gateway-controller/go.mod:7`) | v1.37.0 | **v1.40.0** | carries the plugin proto and `RetryPolicy.refresh_cluster_on_retry`; neither exists in v1.37.0 |

Root `go-control-plane` stays at **v0.14.0** (already latest). `go-control-plane/envoy@v1.40.0`
requires **Go ≥ 1.26.0**; the controller declares 1.26.5, so CI images must be confirmed at ≥ 1.26.0.

**Storage**: None persistent. Target health is in-memory per policy-engine instance (FR-020), behind
a replaceable boundary (FR-020a)

**Testing**: Go unit tests per module; godog/BDD suites under `gateway/it/features/` following
`model-round-robin.feature` conventions; `gateway-controller` xDS translator tests

**Target Platform**: Linux containers (gateway-runtime: Envoy + policy-engine; gateway-controller)

**Project Type**: Gateway policy plus control-plane configuration generation

**Performance Goals**: No measurable added latency when no failover occurs — the downstream pass only
writes a metadata list. On the failover path, added latency bounded by the per-attempt time limit
plus the serving target's response time (SC-003); a suspended target contributes zero

**Constraints**: Downstream chain runs once; per-attempt adaptation confined to upstream filters;
`refresh_cluster_on_retry: true` is mandatory and the route cache must be cleared after the policy
writes its metadata (both proven necessary by POC probes B3 and B2); no failover after response
headers or streamed content reach the client (FR-013)

**Scale/Scope**: Per-operation configuration; target counts in the low single digits; health state
per (model, provider) pair per instance

## Constitution Check

*GATE: evaluated before Phase 0 research; re-evaluated after Phase 1 design.*

| Principle | Gate | Pre-Phase 0 | Post-Phase 1 |
|---|---|---|---|
| I. Component Independence & Minimal Footprint | No new cross-component dependency; no dependency bloat | **PASS** | **PASS** — zero new libraries; policy is an independently versioned module |
| II. Policy-First Gateway Extension | Behavior ships as a policy, not core proxy logic | **CONDITIONAL** | **CONDITIONAL** — see Complexity Tracking. Reduced in scope versus earlier drafts: the controller emits a cluster LB policy and a retry policy; *all* selection and ordering logic stays in policies |
| III. Security Rules Are Non-Negotiable Gates | Fail-closed; no silent lapse of a security control | **PASS** | **PASS** — FR-012a/b enforced; metadata carrier is gateway-internal and rejected on ingress |
| IV. Test-First with Runtime Verification | Unit + real-runtime tests in the same PR | **PASS** | **PASS** — FR-012c mandates the masking-plus-failover test; BDD per story |
| V. Backward-Compatible, GitOps-Safe Evolution | Additive; no breaking change to shipped config | **PASS** | **PASS** — FR-026c and SC-012 require unchanged behavior for routing policies deployed without failover |

**Gate result: PASS with one justified deviation** (Principle II), in Complexity Tracking.

Constitution-driven obligations carried into design:

- `go-network-service-hardening.md` — explicit per-attempt timeout (`per_try_timeout`) and an attempt
  bound equal to chain length; the outer bound must be tighter than a generic multiple of the inner
- `error-handling.md` — sterile exhaustion response (FR-014); telemetry carries no prompt content
- `go-control-plane-xds-security.md` — generated HCMs keep `NormalizePath`, `MergeSlashes`,
  `PathWithEscapedSlashesAction` (directive 6); per-endpoint TLS must come from SDS-backed
  `transport_socket_matches`, never inline key material (directive 3)
- `feature-docs.md` — update any `kb/**/*.md` whose `sources:` lists a touched path, same PR

## Project Structure

### Documentation (this feature)

```text
specs/001-model-failover-policy/
├── plan.md              # This file
├── research.md          # Phase 0 — eight mechanisms evaluated, priority_group adopted
├── data-model.md        # Phase 1 — entities, state machine, validation
├── quickstart.md        # Phase 1 — runnable validation guide
├── contracts/
│   ├── model-failover-policy.yaml      # policy definition
│   ├── target-preference-list.md       # routing-policy ↔ failover contract
│   └── generated-envoy-config.md       # cluster/LB/retry generation contract
├── checklists/requirements.md
└── tasks.md             # STALE — regenerate with /speckit-tasks
```

### Source Code (repository root)

```text
# Policy modules — external repo wso2/gateway-controllers, pinned here
gateway/build-manifest.yaml                       # add model-failover; bump the two routing policies

# Version pins
gateway/gateway-runtime/Dockerfile                        # ARG ENVOY_VERSION -> v1.40.0
gateway/gateway-controller/go.mod                         # go-control-plane/envoy -> v1.40.0

# Control plane — generated configuration (the Principle II deviation)
gateway/gateway-controller/pkg/transform/failover.go      # policy params -> resolved target list
gateway/gateway-controller/pkg/transform/restapi.go       # populates Route.Failover per operation;
                                                          # LlmProxy reaches it via LLMTransformer,
                                                          # which converts to a RestAPI first
gateway/gateway-controller/pkg/xds/translator.go          # route plugin wiring + retry policy
                                                          # incl. refresh_cluster_on_retry: true
gateway/gateway-controller/pkg/xds/failover_specifier.go  # priority_group specifier generation
gateway/gateway-controller/pkg/models/runtime_deploy_config.go
gateway/gateway-controller/pkg/config/failover_validator.go   # schema-level validation only (FR-006)

# Data plane
gateway/gateway-runtime/policy-engine/internal/kernel/    # priority-group metadata write + route-cache clear
gateway/gateway-runtime/router/                           # upstream-position filter wiring

# Tests
gateway/it/features/failover-core.feature                 # core platform, no policy module
gateway/it/features/model-failover.feature
gateway/it/features/model-failover-routing-layered.feature
gateway/it/features/model-failover-load.feature           # sustained load + latency (SC-008, SC-003)
gateway/gateway-controller/pkg/xds/translator_test.go
gateway/gateway-controller/pkg/xds/failover_specifier_test.go
gateway/gateway-runtime/policy-engine/internal/kernel/failover_metadata_test.go
```

**Structure Decision**: Delivered primarily as a new gateway policy. Repository changes are confined
to **configuration generation** (one cluster LB policy, one retry policy per failover-enabled
operation) and **per-attempt plumbing**. No model-selection or ordering logic enters core gateway
code — the order is data the policy writes per request, not configuration the controller enumerates.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|---|---|---|
| **Principle II** — emitting cluster LB policy and route retry configuration from `gateway-controller/pkg/xds` | Same-request re-dispatch does not exist in this gateway (`RetryPolicy`/`NumRetries`/`RetryOn`: zero occurrences in `pkg/xds`). A policy cannot cause the proxy to re-send a request. Without this, FR-008 is unimplementable | **Policy-only**: the capability provably does not exist for a policy to invoke. **Policy engine dispatches attempts itself**: bypasses Envoy pooling, TLS, circuit breaking and observability, and duplicates provider auth. Note this deviation is now *smaller* than in earlier drafts — no synthetic redirects, no per-ordering cluster enumeration, just a standard LB policy plus a standard retry policy |
| **Bumping the Envoy data plane and control-plane library** (1.39.0 → 1.40.0) | `priority_group` ships in Envoy 1.40.0; the plugin proto and `refresh_cluster_on_retry` are absent from the pinned `go-control-plane/envoy v1.37.0`. Without the bump the only alternative is Override Host, which POC probe A3 showed will route to a **deliberately excluded (suspended) target** when attempts overrun the list | **Stay on 1.39.0 with Override Host**: rejected — it carries the A3 safety gap, relies on an undocumented internal-address form (probe A4), and needs internal listeners plus `internal_upstream` plumbing that `priority_group` makes unnecessary. The bump was validated on CI with a baseline control run; both passed |

## Phase 0 → Phase 1 Gate

Phase 0 is complete ([research.md](./research.md)); no `NEEDS CLARIFICATION` markers remain.
FR-021 is satisfiable as written — **no spec amendment is required**, unlike the two earlier drafts.

One risk is carried into design and **must be retired before implementation**:

> **R1 — endpoint addressing: RESOLVED by the mechanism change.** It only existed because Override
> Host selects among *endpoints of one cluster*. `priority_group` names **clusters**, so per-provider
> clusters are used directly — no internal listeners, no `internal_upstream`, no reliance on the
> undocumented `envoy://` address form. The remaining must-prove items are narrower and were
> demonstrated in the POC: `refresh_cluster_on_retry: true` is mandatory (probe B3) and the route
> cache must be cleared after the metadata write (probe B2).

**`tasks.md` is stale** — generated against a withdrawn design and predating five clarification
sessions (windowed error rate, aggregate latency, escalating backoff, kind applicability and
isolation, silent no-op). Regenerate with `/speckit-tasks`.
