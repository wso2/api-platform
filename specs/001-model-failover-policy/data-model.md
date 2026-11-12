# Phase 1 Data Model: Model Failover Policy

**Feature**: 001-model-failover-policy | **Date**: 2026-10-22

No persistent storage is introduced. All entities below are configuration (declarative, GitOps-held)
or in-memory runtime state scoped to a policy-engine instance.

---

## 1. FailoverConfiguration (configuration)

Publisher-authored, attached to an AI proxy API operation. Source of truth is the API's declarative
configuration; see [contracts/model-failover-policy.yaml](./contracts/model-failover-policy.yaml).

| Field | Type | Required | Default | Validation |
|---|---|---|---|---|
| `targets` | list of `Target` | Conditional (FR-001a) | — | Required unless an adopting routing policy is attached to the same operation (FR-006a). Each entry resolvable (FR-006) |
| `failureConditions.statusCodes` | list of int | No | `[429, 500, 502, 503, 504]` | Each in 400–599. Transport failures always trigger regardless (FR-009) |
| `failureConditions.onTransportFailure` | bool | No | `true` | Connection failure, reset, unavailable upstream, per-attempt timeout |
| `suspension.consecutiveFailures` | int | No | `3` | `>= 0`; `0` disables suspension |
| `suspension.durationSeconds` | int | No | `30` | `>= 0`. **Base** duration; doubles per re-suspension up to the cap |
| `suspension.maxDurationSeconds` | int | No | `600` | `>= durationSeconds`. Ceiling for the doubled duration |
| `suspension.backoffResetSeconds` | int | No | `600` | `>= 1`. Healthy period that resets the duration to base |
| `perAttemptTimeoutMs` | int | No | `30000` | `> 0`; must be less than the operation's overall request timeout (hardening rule, directive 5) |

**Derived, not configured**: the attempt bound equals the resolved chain length and is emitted into
generated configuration (FR-027, edge case "attempt budget exhausted").

## 2. Target (configuration + identity)

| Field | Type | Required | Notes |
|---|---|---|---|
| `model` | string | Yes | `minLength: 1` |
| `provider` | string | No | Matches an `additionalProviders[].as` alias, or the provider id when `as` is absent. Omitted ⇒ the proxy's primary provider. Same shape as `model-round-robin.models[]` |

**Identity is the pair `(model, provider)`**, never `model` alone (FR-017). The same model under two
provider configurations is two distinct targets with independent health. The resolved `provider`
also names the upstream cluster, which is what makes a target addressable.

**Roles a target can hold** (FR-001b):

- *Chain member* — appears in `targets`; attempted in configured order
- *Last-resort tail* — appears in `targets` but not in any published ranking; reachable only after
  the published ranking is exhausted
- *Ranked* — appears in a routing policy's published ranking for this request

## 3. TargetPreferenceList (per-request, transient)

Published by a routing policy; see
[contracts/target-preference-list.md](./contracts/target-preference-list.md).

| Field | Type | Notes |
|---|---|---|
| `targets` | ordered list of `(model, provider)` | The routing policy's ranking, best first |
| `source` | string | Publishing policy name/version, for telemetry (FR-029) |

Rules: computed **once per client request** (FR-022) and carried across attempts; ranks over the
healthy set published by failover (FR-026); a policy that publishes only a single selection yields a
one-element list (FR-023); empty list falls back to the configured chain (edge case).

## 4. ResolvedChain (per-request, derived)

The ordered sequence actually traversed, built once per client request:

```
ResolvedChain = dedup( TargetPreferenceList.targets ++ FailoverConfiguration.targets )
                filtered by TargetHealth.state != suspended
```

Order is preference-list-first, configured-targets-after (FR-024). De-duplication keeps a target's
**first** position (FR-010). Length bounds the attempt budget.

## 5. TargetHealth (runtime state, in-memory, per instance)

Keyed by `(operation, model, provider)`. Held behind a replaceable boundary so a shared
cross-instance implementation can be substituted without changing behavior (FR-020a).

| Field | Type | Notes |
|---|---|---|
| `consecutiveFailures` | int | Reset to `0` on any success (FR-018) |
| `state` | enum | `healthy` \| `suspended` — there is no probing state (FR-019) |
| `suspendedUntil` | timestamp | Set on entering `suspended` |
| `currentDurationSeconds` | int | The duration applied to the current/next suspension; doubles per re-suspension, capped at `maxDurationSeconds` (FR-019a/b) |
| `healthySince` | timestamp | When the target last returned to healthy; `backoffResetSeconds` of healthy operation resets `currentDurationSeconds` to base (FR-019c) |
| `latencySamples` | rolling samples | Response times within `windowSeconds`, for the percentile rule (FR-018e). Bounded reservoir, not an unbounded list |
| `windowCounters` | rolling counts | Requests and failures within `windowSeconds`, used by the rate rule (FR-018a). Must expire by time, not by request count, or a quiet target keeps stale failures forever |
| `lastSuspensionReason` | enum + value | `consecutive` with its count, or `rate` with the observed rate and sample size — surfaced in the suspension record (FR-018d) |

### State transitions

| From | Event | To | Side effect |
|---|---|---|---|
| `healthy` | failure, `consecutiveFailures + 1 < threshold` | `healthy` | increment |
| `healthy` | failure, `consecutiveFailures + 1 >= threshold` | `suspended` | set `suspendedUntil`; emit suspension record naming the consecutive criterion |
| `healthy` | failure, window rate >= `errorRatePercent` **and** window requests >= `minimumRequests` | `suspended` | set `suspendedUntil`; emit suspension record naming the rate criterion and observed rate (FR-018a/b/d) |
| `healthy` | window p95 >= `p95LatencyMs` **and** window requests >= `minimumRequests` | `suspended` | set `suspendedUntil`; emit suspension record naming the latency criterion and observed percentile (FR-018e) |
| `healthy` | success | `healthy` | reset counter |
| `suspended` | `now >= suspendedUntil` | `healthy` | return to **full** rotation; set `healthySince`; emit recovery record (FR-019) |
| `healthy` | any suspension criterion re-trips | `suspended` | `currentDurationSeconds = min(currentDurationSeconds * 2, maxDurationSeconds)`; emit record naming the escalation step (FR-019a/b/d) |
| `healthy` | healthy for `backoffResetSeconds` | `healthy` | `currentDurationSeconds = durationSeconds` (base) (FR-019c) |
| any | suspension disabled (all criteria off) | `healthy` | never leaves `healthy` |

Concurrency: transitions must be atomic per key so simultaneous requests cannot skip the threshold
or double the suspension twice for the same failure (edge case "concurrent requests during
suspension transitions").

## 6. Published target list (per-request, written once)

The `ResolvedChain` rendered as proxy-consumable data. Written by the downstream pass before the
first attempt; the proxy walks it across retries. See
[contracts/generated-envoy-config.md](./contracts/generated-envoy-config.md) §3.

| Field | Value |
|---|---|
| Carrier | Request dynamic metadata, namespace `envoy.lb`, key `wso2.failover.targets` |
| Format | Ordered `IP:Port,IP:Port,...` |
| Contents | Resolved chain with suspended targets already removed |

Consequences of the ordering living in request data: the ranking is never recomputed (FR-022) because
it is written once; at-most-once (FR-010) holds positionally because each address appears once; and
suspended targets are skipped twice over — omitted by the policy, and skipped by the proxy if absent
from the endpoint set (FR-025).

**Security**: gateway-internal. MUST NOT be derivable from client input, and an inbound header or
metadata of the same name MUST be rejected at ingress — an accepted client value would let a caller
steer traffic to an arbitrary endpoint of the failover cluster.

## 7. PolicyExecutionClass (policy metadata)

Declared per policy, never inferred (FR-012f).

| Value | Meaning | Filter position | Examples |
|---|---|---|---|
| `once-per-client-request` | Executes once, on the downstream pass | downstream chain | rate limiting, token/cost accounting, analytics, guardrail screening |
| `per-attempt` | Executes on every attempt | upstream chain | credential injection, request/response translation |

Because the downstream chain runs exactly once, once-per-client-request behavior is structural rather
than enforced by bookkeeping (FR-012d). The declaration determines **which filter position** a policy
occupies. **Default when undeclared: `once-per-client-request`** — a policy later found to have side
effects fails safe rather than double-charging (FR-012f).

## 8. FailoverAttemptRecord (telemetry, transient)

| Field | Notes |
|---|---|
| `attempts[]` | Each with target, outcome, failure reason, duration |
| `servingTarget` | The target that produced the returned response (FR-016, FR-029) |
| `rankingSource` | Which routing policy published the ranking, if any |
| `skipped[]` | Targets skipped as suspended — skips are not attempts (FR-025) |
| `exhausted` | Whether every target failed or was skipped |

Must contain no credentials, authorization headers, or prompt/completion content (FR-034), and the
client-facing error derived from it must not leak upstream bodies or internal addresses (FR-014).
