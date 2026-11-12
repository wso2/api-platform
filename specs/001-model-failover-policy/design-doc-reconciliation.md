# Reconciliation with the original design document

Source: *Copy of Model Failover Policy* (28 pp., design reviews 2026-09-04 → 2026-10-09).
Read into this spec on 2026-10-26.

The document predates this spec's clarification sessions and the A–E probe results, so the two
disagree in places. This file records every divergence found, which one governs, and why — so a
reader of either artifact can tell deliberate supersession from drift.

## Carried in wholesale

The upstream/downstream policy separation (§ "Per-provider upstream policies", § "Upstream
ext_proc and existing policies") is now **FR-035–FR-044** in `spec.md` and tasks **T024a–T024j**
in `tasks.md`. The 2026-10-09 design review minute — *"We decided to go with the upstream ext
proc. Under `spec.upstream` or `spec.providers` we will add a section called `policies`"* — is the
governing decision, and Probe E validates the mechanism.

Three details from the document are load-bearing and were not in this spec before:

1. **Controller-fixed chain order**: author policies → transformer → **auth last**, so no
   author-written policy ever observes the credential.
2. **`RestApi` gets upstream policies too** (`upstream.main.policies`, `upstream.sandbox.policies`).
   This is not an LLM-only feature; the spec had been treating per-attempt work as LLM-specific.
3. **"Each attempt starts from the original request."** This answers the re-entrancy question the
   probes raised, and it is the right answer — but the phrase needs reading in its context. The
   baseline is the **post-downstream** request, not the raw client request: the document places
   this under "Upstream ext_proc and existing policies", describing what the upstream chain sees
   after the downstream chain has run once. Taken literally it would strip the downstream
   masking and guardrail rewrites that must apply to every attempt. Spelt out as FR-038/FR-038a
   after the ambiguity was caught on 2026-10-27.

## Where this spec supersedes the document

| Topic | Document | This spec | Why |
|---|---|---|---|
| Cluster mechanism | `envoy.clusters.composite`, one composite per chain plus one suffix composite per fallback position | `priority_group` cluster specifier | POC probes A/B (`poc-results.md`) chose `priority_group` on live evidence. The document's own "Alternatives considered" rules out *aggregate cluster + previous_priorities*, not `priority_group`; it reaches composite clusters without evaluating it. Composite also drew the explicit user direction *"creating all possible composite clusters is not the solution"* |
| Recovery from suspension | Probe-based: `recoverAfterSuccesses` successes in a row, probes sent one at a time, any failed probe re-suspends | **Both, as of 2026-10-28.** The probe phase is restored as the document specifies; the suspension *window* it returns from is now an escalating backoff | The two were briefly treated as alternatives and the probe phase dropped. That was wrong: backoff sets how long a target stays out, probing sets how it comes back. Restoring at full share exposes a burst to a still-broken target (FR-019, FR-019e, FR-019f) |
| Attribution of a rescued request | Not addressed | `x-envoy-attempt-count` indexes the chain so attempts 1..N-1 are recorded as failures (FR-030f–FR-030j) | Found during this spec's review: grading only the first target records a 200 for a target that actually failed, so a consistently failing target is never suspended |
| Per-attempt target identity | A controller-inserted per-attempt `model-failover` copy sets `selected_provider`, which gates each provider's list | ext_proc `xds.cluster_name` attribute | Probe E: Envoy supplies correct per-attempt identity natively. Removes a whole mechanism, and with it the document's exception on p.9 where transformer and auth stay downstream because *"nothing tells the upstream phase which provider an attempt is going to"* |

## Open conflicts — these need a decision, not a merge

### C1 — CLOSED 2026-10-26: the document's removal was not a meeting decision

The "Removed" line in the document's alternatives table does not carry the weight of an approved
decision, so it does not override the 2026-10-22 clarification. `errorRatePercent`, `windowSeconds`,
`minimumRequests` and `p95LatencyMs` are reinstated in `policy-definition.yaml`.

The document's second objection is still a real caveat and is now documented on the parameter
itself: for LLM traffic, latency correlates with prompt and completion size as much as with
provider health, so `p95LatencyMs` can make a member serving legitimately large requests look
unhealthy. It defaults to 0 (disabled) for that reason. The first objection ("needs 20+
attempts/min per replica") is answered by `minimumRequests`, default 20.

The original conflict, for the record:

**Scope check (asked 2026-10-26): is the "Alternatives considered" table composite-cluster specific?**
Mostly yes — five of its six rows are about routing mechanism (aggregate clusters, outlier
detection, bypassing to a leaf, composite-per-subset, a custom extension) and die with the composite
design. The sixth does not. "Rolling failure-rate and p95 latency rules" is about *what makes a
target count as unhealthy*, which is independent of how traffic is routed, and both stated reasons
still apply to this spec unchanged:

- *"Rate rule needs 20+ attempts/min per replica"* — health state is per-replica in memory here too
  (FR-020), so the same thin-traffic problem exists. **This spec already answers it**: evaluation is
  gated on `minimumRequests` (default 20), so a quiet replica never computes a rate.
- *"latency tracks prompt size, not health"* — unanswered. A long prompt legitimately takes longer,
  so a p95 latency rule can suspend a perfectly healthy provider for serving large requests. The
  only mitigation today is that `p95LatencyMs` defaults to 0 (disabled).

**Status**: the rate rule can be kept on its merits. The latency rule needs either a defence or
removal. Pending that, `errorRatePercent`, `windowSeconds`, `minimumRequests` and `p95LatencyMs` are
**parked** — they are not in the reviewed schema adopted under C2, and were not carried into the
rewritten `policy-definition.yaml`. Reinstating them is a small change once decided.

The original conflict, for the record:

The document lists under "Alternatives considered":

> **Rolling failure-rate and p95 latency rules** — Rate rule needs 20+ attempts/min per replica;
> latency tracks prompt size, not health. **Removed.**

This spec's FR-018 has three criteria — consecutive failures, `errorRatePercent` over a window,
and `p95LatencyMs` — added via the 2026-10-22 clarification session. Two of the three are the ones
the document rejected, and its stated reasons are specific and empirical (traffic volume per
replica; latency correlating with prompt size rather than provider health).

Either the clarification superseded a considered rejection, or the rejection was not surfaced when
the clarification was asked. **Unresolved.** `minimumRequests` (default 20) partly answers the
first objection; nothing in this spec answers the second.

### C2 — RESOLVED 2026-10-26: adopt the document's shape

Decision: the reviewed schema governs. `policy-definition.yaml` has been rewritten (T024k); the
parsing, chain resolution, controller transform, validator and tests written against the flat shape
are now invalid and are tracked as T024l–T024r. Two deliberate departures: `recoverAfterSuccesses`
is NOT reinstated (probe recovery was superseded by escalating backoff in the 2026-10-22
clarification), and `backoffResetSeconds` is kept because escalating backoff needs it.

The original divergence, for the record:

| | Document | This spec / `policy-definition.yaml` |
|---|---|---|
| Shape | `targets[]` where each entry is a *chain* keyed by the client-requested model, each with its own `fallbacks[]` | Flat `targets[]`, one ordered list |
| Match on requested model | Yes — an entry's `model` is the client-requested model it matches, with `"*"` as catch-all and exact entries winning | **Absent.** The chain applies to the operation regardless of which model was requested |
| `"*"` member | Passes the client's own model through unchanged | Not supported |
| Suspension params | `suspendDuration`, `suspendAfterFailures`, `maxSuspendMultiplier`, `recoverAfterSuccesses` | `suspension.{consecutiveFailures, durationSeconds, maxDurationSeconds, backoffResetSeconds, errorRatePercent, windowSeconds, p95LatencyMs, minimumRequests}` |
| Failure codes | `statusCodes` (replaces the 5xx default entirely) | `failureConditions.{statusCodes, onTransportFailure}` |
| Per-attempt timeout | `perTryTimeout` (seconds) | `perAttemptTimeoutMs` (milliseconds) |

The structural gap matters most: the document's design lets one policy attachment serve **different
chains for different requested models** (`gpt-4o` has one chain, `gpt-4.1` another, `"*"` catches
the rest). This spec cannot express that at all — its chain is per operation. The parameter renames
are cosmetic by comparison, but the shipped schema should match whichever is approved, and the
document's is the one that went through design review.

### C3 — CLOSED 2026-10-26: the document's limit is accepted as-is

Decision: accept *"connection failures retry but never suspend"* as a documented architecture
limit rather than engineer around it. This spec had claimed otherwise, and that claim is now
corrected (FR-009a, FR-018h) rather than left as aspirational text backed by unreachable code.

What was done:

- `classify.go`'s `isTransportFailure` is **removed**. It could never fire — Envoy does not invoke
  ext_proc for its own local replies — and a predicate that looks like it handles a case is worse
  than its visible absence. The reason is documented in its place.
- `onTransportFailure` is removed as a configurable condition from the policy schema, the runtime
  model and the controller transform. The generated `retry_on` lists `connect-failure`, `reset`
  and `refused-stream` unconditionally, so the flag controlled nothing.
- The limit is stated in the policy's own user-facing description, so an operator reading the
  parameter reference sees it.

Why the limit is acceptable: the attempt-count inference already records a transport-dead member
as failed whenever a later member returns a real response, so the common case still drives
suspension. Only an all-transport-dead chain goes unrecorded, and suspension there has nowhere
healthier to route. Probe F separately ruled out closing even that with outlier detection.

The earlier recommendation, superseded by this decision:

**The common case is already handled, by accident of a different design decision.** The
attempt-count attribution added for FR-030f–FR-030j infers failures: attempts 1..N-1 were retried,
and Envoy only retries what failed. So when a transport-dead member is attempt 1 and a healthy one
answers on attempt 2, the response phase runs on that real 200 and records attempt 1 as failed.
The dead member does reach suspension. The gap is narrower than the document implies.

What remains uncovered is only the case where **every** member fails at transport level: no real
response exists, the policy is never invoked, and nothing is recorded. Suspension there would save
dial time but has nowhere healthier to send traffic.

Recommended order of work:

1. **Formalise and test the inference** (cheapest, already works). Make FR-009/FR-018 state what
   actually happens rather than implying the policy observes transport errors directly.
2. **Close the remainder over the existing access-log pipeline.** The router already streams access
   logs over gRPC to the collector (`createAccessLogConfig`, ALS cluster in `translator.go`) and
   already emits `%RESPONSE_FLAGS%` and `%UPSTREAM_TRANSPORT_FAILURE_REASON%`. The signal is on the
   wire today. Adding the policy engine as a second ALS sink puts those records in the same process
   as the in-memory health store, so no new component, protocol or cross-process state is needed —
   which is what made this look expensive before the pipeline was found.
3. **Probe whether outlier detection now works.** The document rejected it as *"Disproven live: the
   composite still sends attempt 1 to an ejected leaf → 503 no healthy upstream, not retryable"* —
   but that was the composite cluster. Under `priority_group` the IT run showed a zero-host cluster
   produce a 503 that WAS retried, with the chain traversing to group 2. If Envoy will eject dead
   hosts itself and the chain still traverses, most of this problem disappears natively. One probe
   settles it, and it is cheap enough to do before building step 2.
4. **Fix the dead code now, regardless of the above.** `classify.go`'s `isTransportFailure` cannot
   fire as written.

The original conflict, for the record:

### C3 — Transport failures and suspension

The document states plainly under "Surprises": *"ext_proc is not called for Envoy's local replies,
so connection failures retry but never suspend."* It records this as a known architecture limit.

This spec's FR-009 treats transport failures as failover-worthy *and* `classify.go`'s
`isTransportFailure` records them against target health — which cannot work, for exactly the reason
the document gives. Tracked as **T024i**; the fix is either another outcome source or an honest
documented limit.


---

## C4 — "Why priority_group if its groups are also deploy-time?" (raised 2026-10-27)

A fair challenge, and it exposed a real design error. Recorded in full because the answer is partly
"the choice was right" and partly "the implementation gave away why".

### What composite was actually rejected for

Orderings, not members. From `research.md` § M3: *"A per-request arbitrary ranking would need one
composite cluster per permutation (N! in the limit)"*, and the design document's own alternatives
table rejects *"Composite per suspended subset — Exponential cluster count."*

| | Composite cluster | `priority_group` |
|---|---|---|
| Deploy-time artifact per | **ordering** | **member** |
| Artifacts for N members | N! for an arbitrary ranking | N |
| Express a new order at runtime | needs a new cluster | free — publish names in any order |

That advantage is real and unchanged. It is why "obey the ordering decided by other policies" ruled
composite out: composite cannot express a per-request reordering at all without new config.

### Where the advantage was given away

The `PriorityGroup` proto carries `{name, clusters: [{cluster_name, weight}]}`. A published group
may carry **its own cluster list** — Probe B demonstrated exactly this, since the b1 Lua stand-in
published inline clusters and the probe passed.

`buildPriorityGroups` publishes **name only**, deliberately, to keep cluster naming out of the
policy. That re-imported a deploy-time constraint the mechanism does not impose: a published name
can only resolve if the controller pre-configured a group with that exact name.

### The concrete bug this produces

The reviewed design's wildcard member (*"A `"*"` member keeps the client's model"*) cannot work
under a name-only contract:

- controller configures `t:azure:*` — `flattenChainMembers` keeps the model literal
- policy publishes `t:azure:gpt-4-turbo` — `resolveMember` substitutes the requested model

The names never match, so **every wildcard member is an unresolvable group**, and by Probe G that
silently falls back to the configured group at that position rather than being skipped.

### Fix

The cluster is a property of the **provider**, not the model — a wildcard member changes the model
and keeps its provider. So:

1. the controller passes a provider → cluster map to the policy (deploy time, `systemParameters`);
2. the policy publishes complete groups, `{name, clusters:[{cluster_name}]}`.

Any `(provider, model)` reachable at runtime then resolves — wildcards, routing-policy rankings,
and models nobody enumerated — provided the *provider* is deployed, which it must be to have a
cluster at all. This also makes T024v1 (pre-enumerating the routing policy's `models[]`)
unnecessary.

Tracked as T024w. T024v1 is superseded; T024v2 remains valid as defence in depth.
