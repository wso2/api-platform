# Phase 0 Research: Model Failover Policy

**Feature**: 001-model-failover-policy | **Date**: 2026-10-22
**Scope**: How a single client request is re-attempted against successive model targets, and which
mechanism is the right long-term architectural bet for this gateway.

---

## 1. The question this research had to answer

FR-008 requires re-attempting the *same* client request against the next target. FR-021 requires
that, when a routing policy is attached, traversal follows **that policy's per-request ranking** —
an arbitrary order that is only known at request time. FR-012a requires every attempt to carry the
request-body mutations applied by preceding policies (masking, guardrails, prompt decoration).

Those three together are the whole problem. Any mechanism that cannot satisfy all three either
breaks a requirement or silently degrades a security control.

### Baseline: what the gateway can and cannot do today

| Capability | Status | Evidence |
|---|---|---|
| Steer one attempt to a specific provider's upstream | **Exists** | Policy sets an upstream directive → `x-target-upstream` header → route `cluster_header` specifier, with `ClearRouteCache: true` (`gateway-runtime/policy-engine/internal/kernel/translator.go:380-392`, `gateway-controller/pkg/xds/translator.go:410-412`); the shipping `dynamic-endpoint` policy uses it |
| Re-send a request after an upstream failure | **Absent** | Zero occurrences of `RetryPolicy` / `NumRetries` / `RetryOn` in `gateway-controller/pkg/xds` |
| Routing policies publish their selection | **Partially exists** | `metadata["selected_provider"]`, consumed by translators (`gateway/examples/openai-multi-provider-proxy.yaml:18-25`); `providerId` doubles as the upstream cluster name |
| Per-request *ranking* published by routing policies | **Does not exist** | Current policies publish a single selection (`llm-header-router`, `typesafe-jev-model-routing`) |
| "Failover" in `model-round-robin` today | **Not same-request** | `gateway/it/features/model-round-robin.feature:849` asserts the failing request returns **500 to the client**; only the next request moves |

Environment facts verified locally:

- Data plane: `envoyproxy/envoy:v1.39.0` (`gateway/gateway-runtime/Dockerfile:24`)
- Control plane library: `github.com/envoyproxy/go-control-plane/envoy v1.37.0` (`gateway/gateway-controller/go.mod:7`)
- Policies are independently versioned Go modules in `wso2/gateway-controllers`, pinned in `gateway/build-manifest.yaml`, compiled into the policy-engine binary — an added policy is an in-process chain step, not a network hop

---

## 2. Mechanisms evaluated

### M1 — Route-level retry policy (plain Envoy retries)

**Verdict: rejected — cannot change provider.**

Envoy retries stay inside the cluster the route already selected. A maintainer on
[envoy#35877](https://github.com/envoyproxy/envoy/issues/35877): *"one of the clusters is chosen,
and then all retries will happen within that cluster."* Because `providerId` is the cluster name
here, a plain retry re-hits the same provider. Also fails FR-012 — the downstream chain does not
re-run, so credentials and request format stay those of the first provider.

### M2 — Aggregate cluster (`envoy.clusters.aggregate`)

**Verdict: rejected — selects by health, not by attempt.**

Aggregate clusters group clusters in priority order and fail over on health. The retry interaction
is poor: `PriorityLoad` retry plugins do not work under an aggregate cluster because its load
balancer controls distribution at a higher level, and only `max_retries` is meaningful at the
aggregate level. It expresses "use the healthy one", not "attempt 1 here, attempt 2 there".

### M3 — Composite cluster (`envoy.clusters.composite`)

**Verdict: viable, natively supported, but static ordering only.**

Purpose-built for this use case — the architecture overview names AI gateway failover explicitly.
Semantics from the generated proto
(`go-control-plane/envoy@v1.37.0/extensions/clusters/composite/v3/cluster.pb.go`): *"the composite
cluster uses the retry attempt count to deterministically select which sub-cluster to route to"* —
attempt 1 → `clusters[0]`, attempt 2 → `clusters[1]`. If a member has no healthy hosts it moves to
the next member **within the same attempt** (free suspension-skipping). Exceeding the member count
fails with "no host available".

Availability is excellent: added in **Envoy 1.37.0**, the data plane runs **1.39.0**, and the
`composite/v3` package is already present in the pinned `go-control-plane/envoy v1.37.0`. **No
dependency change is required to adopt it.** Members are referenced by name and defined elsewhere,
so the one-cluster-per-provider model — with its per-provider TLS, endpoints, and credentials —
survives intact.

The disqualifier for FR-021 alone: `ClusterConfig_ClusterEntry` has exactly one field, `Name`. The
order is fixed in cluster configuration. A per-request arbitrary ranking would need one composite
cluster per permutation (N! in the limit). A bounded variant — one composite cluster per *starting*
target (N of them), header-selected — honors the routing policy's **first** pick with a static tail,
but not a full ranking.

### M4 — Priority levels within one cluster + `retry_priority`

**Verdict: rejected — same static-ordering limit, worse isolation.**

This is the Agent Router / Envoy Gateway shape: prioritized `backendRefs` compiled into priority
levels, `numAttemptsPerPriority: 1` so a failure drops straight to the next priority, `retryOn:
connect-failure, retriable-status-codes`. Note the configuration trap: `httpStatusCodes` is ignored
unless `retriable-status-codes` appears in the trigger list.

Priorities are assigned at configuration time, so ordering is static exactly as in M3, while also
collapsing providers into one cluster — losing the clean per-provider cluster boundary this gateway
already has. Built-in `previous_priorities` only *excludes* tried priorities; it does not accept a
per-request order.

### M5 — Chain re-entry via internal redirect

**Verdict: rejected (directive, 2026-10-22).**

Technically viable: an internal redirect re-runs the whole downstream chain, which would honor an
arbitrary ranking and re-apply body mutations. Rejected because it re-runs the downstream chain,
which the design directive forbids, and because converting a 5xx into a synthetic 307 is not a
documented Envoy failover pattern.

### M6 — Policy engine dispatches fallback attempts itself

**Verdict: rejected.**

Full control over ordering, but it bypasses Envoy connection pooling, TLS configuration, circuit
breaking, outlier detection and upstream observability, and duplicates provider authentication in a
second code path.

### M7 — Pre-generated permutation composite clusters

**Verdict: rejected (directive, 2026-10-22).**

Honors a ranking by selecting among N! pre-generated composite clusters. Rejected as configuration
explosion; also degrades past a target-count threshold, which would have forced an FR-021 amendment.

### M8 — Override Host load balancing policy — **ADOPTED**

`envoy.extensions.load_balancing_policies.override_host`. Purpose-built for endpoint picking done by
a downstream filter, with retries following the picker's order — the Gateway API Inference Extension
uses it for exactly this shape.

Semantics, quoted from the proto shipped in the pinned library
(`go-control-plane/envoy@v1.37.0/extensions/load_balancing_policies/override_host/v3/override_host.pb.go`):

> *"A list of sources to get host addresses from. The host sources are searched in the order
> specified. The request is forwarded to the first address and subsequent addresses are used for
> request retries or hedging. Note that if an overridden host address is not present in the current
> endpoint set, it is skipped and the next found address is used. If there are not enough overridden
> addresses to satisfy all retry attempts the fallback load balancing policy is used to pick a host."*

A single source carries an **ordered list**: *"at least one host in `IP:Port` format or multiple
hosts in `IP:Port,IP:Port,...` format."* A source is either a request header or a dynamic-metadata
key, and `FallbackPolicy` names the child LB policy used when neither is present.

**Why it is the right answer here**, mapped to the three binding constraints:

| Constraint | How M8 satisfies it |
|---|---|
| Obey the ordering decided by other policies | The ranked list is **request data**, written once by the downstream pass. Any permutation is expressible; nothing is enumerated in configuration |
| Downstream chain runs exactly once | The downstream pass only writes metadata. Retries are pure Envoy plus the upstream filter chain, which is the only chain Envoy re-invokes on a retry |
| No configuration explosion | **One** cluster and **one** LB policy per failover-enabled operation, regardless of target count or ordering |

Two properties come for free: ordering is positional, so at-most-once (FR-010) needs no extra
bookkeeping, and addresses **absent from the endpoint set** are skipped. Note the limits established
in §7 — host *health* is not consulted, and list exhaustion falls through to the fallback policy
rather than failing.

**Availability** — verified, not assumed: added in Envoy **1.35.0**; the data plane runs **1.39.0**
(`gateway/gateway-runtime/Dockerfile:24`); the Go package is present in the already-pinned
`go-control-plane/envoy v1.37.0` (`gateway/gateway-controller/go.mod:7`). **No dependency change.**

**The one hard problem it creates.** Override Host selects among **endpoints of a single cluster**,
matching `IP:Port` against the current endpoint set, while this gateway models one cluster per
provider. Targets must therefore be endpoints of one failover cluster, and a DNS-resolved provider's
address is not knowable to the policy writing the list. Carried as **R1**, with the preferred
resolution being an **internal listener** endpoint per provider so each provider's own cluster — and
with it its TLS context and credentials — stays intact behind a stable address.

---

## 3. Decision

> **SUPERSEDED on 2026-10-23 — see §10.** This recorded the interim decision to adopt M8 (Override
> Host), taken while `priority_group` was believed unavailable. §6 corrected that status, the POC in
> [poc-results.md](./poc-results.md) compared both at runtime, and §10 records the final adoption of
> **`priority_group`**. Kept because three of its four criteria carried over intact — they are what
> the final choice was judged against.

**Interim decision (superseded): adopt M8 (Override Host).** One failover cluster per enabled
operation, its LB policy set to `override_host` with a dynamic-metadata source and a round-robin
fallback; the ranked list written once by the downstream pass; per-attempt credentials and
translation applied by the upstream filter chain.

**The criteria, which still hold for `priority_group`:**

1. **It is the primitive Envoy built for this problem.** `priority_group` is built for exactly
   "retry into a different group" — closer still than Override Host's endpoint picking.
2. **The ordering lives in data, not configuration.** Unchanged: groups are written per request.
3. **The Principle II deviation shrinks to its minimum** — one cluster-specifier plugin and one
   retry policy. No synthetic redirects, no cluster enumeration, no custom Envoy extension.
4. **It degrades safely.** `priority_group` falls back to the *configured* group order when the
   policy writes nothing, rather than to an unranked round-robin pick — strictly better than what
   this section claimed, as POC probes A3 and B4 demonstrated.

**Mechanism seam retained.** The attempt-sequencer interface stays: the policy expresses *"attempt
these targets in this order"*, and generated configuration is one implementation behind it. That is
what made the Override Host → `priority_group` swap a contained change rather than a rewrite.

## 4. Resolved unknowns

| Unknown | Resolution |
|---|---|
| Can a second attempt reach a different target? | Yes — Override Host walks the ranked address list across retries within one cluster |
| Does the downstream chain re-run on a retry? | **No** — only the upstream filter chain is re-invoked. All per-attempt work lives there |
| Can an arbitrary per-request ranking be obeyed? | **Yes** — the ranking is request data (a comma-separated address list), not generated configuration |
| Is the mechanism available? | Yes — Envoy 1.35.0+, data plane 1.39.0, package present in pinned `go-control-plane/envoy v1.37.0`. No dependency change |
| How are suspended targets skipped? | Two layers: the policy omits them from the list it writes, and Envoy independently skips addresses absent from the endpoint set |
| How is at-most-once (FR-010) guaranteed? | Positionally — each address appears once in the list; `num_retries` equals chain length − 1 |
| How are body mutations preserved (FR-012a)? | The downstream chain runs once, so mutations are applied once to the buffered request. The upstream filter must read the **current buffered body**, never a router-phase snapshot — the defect class in agent-router#2671 |
| Cost of an extra policy in the chain | Negligible — in-process step; the downstream pass only writes metadata |

## 5. Open risks carried into design

**R1 (resolved in principle; verify in spike) — endpoint addressing.** Override Host matches `IP:Port` against the cluster's
current endpoint set, but this gateway models one cluster per provider and provider addresses are
often DNS-resolved and unstable. Resolutions in preference order:

- **(a) Internal listener per provider** as endpoints of the failover cluster, wrapped in the
  `internal_upstream` transport socket so `{model, provider}` passes through to the internal
  connection, with each provider's own cluster — TLS context, credentials, timeouts — intact behind a
  stable internal address. §6 confirms this extension is present in the pinned library. **Preferred**;
  preserves the current model rather than flattening it.
- **(b) Controller-published EDS endpoints** with stable addresses, where provider addressing allows.
- **(c) Restrict launch scope** to stably addressed providers and document the limit.

Spike before any generation code is written; record the outcome here.

**R2 — per-attempt provider adaptation.** The upstream filter must identify which endpoint the
current attempt selected in order to apply that provider's credentials and request translation.
Verify the selected host is observable at the upstream filter position.

**R3 — mutated-body fidelity.** Confirm the upstream filter sees the body as mutated by the
downstream chain (masking, guardrails), not an earlier snapshot. This guards a security control;
treat a failure as blocking, per FR-012b.

**R4 — timeout composition.** `per_try_timeout` must bound each attempt and the route timeout must
bound the whole chain, per `go-network-service-hardening.md` directive 5.

**R5 (resolved) — telemetry de-duplication.** Envoy emits an upstream access log per retry attempt
and exposes `upstream_request_attempt_count`, so per-attempt records and a single downstream-level
record per client request are separable natively (§6). Implementation detail, no longer a risk.

## 6. Envoy extension review (2026-10-22)

A sweep of the Envoy v3 extension index for anything else bearing on this feature. Availability was
checked against the pinned `go-control-plane/envoy v1.37.0`, not assumed from documentation.

### Adopted — additions to the design

| Extension | Use | Status |
|---|---|---|
| `envoy.retry_host_predicates.previous_hosts` | Defense in depth for FR-010: a host already attempted cannot be selected again, even if the published list contained a duplicate or the fallback LB policy engaged | Present (`extensions/retry/host/previous_hosts`) |
| `envoy.transport_sockets.internal_upstream` | **Resolves R1.** Wraps an internal-listener endpoint and *"allows passing additional structured state across the user space socket"*, inserting configured `passthrough_metadata` into the internal connection. Each provider becomes a stable internal address endpoint of the failover cluster while its real cluster — TLS, credentials, timeouts — stays intact behind it, with `{model, provider}` carried through | Present (`extensions/transport_sockets/internal_upstream`) |
| Access log `upstream_request_attempt_count` + per-attempt upstream logs | **Resolves R5.** Envoy emits an upstream access log per retry attempt and exposes the attempt count, so attempt-level records and one downstream-level record per client request are separable without bespoke de-duplication | Core access-log field |
| `envoy.http.early_header_mutation` | Optional declarative way to strip a client-supplied carrier header before route selection, complementing the ingress rejection in the policy engine | Present (`extensions/http/early_header_mutation/header_mutation`) |

### Rejected — with reasons worth recording

| Extension | Why it looked relevant | Why rejected |
|---|---|---|
| `envoy.extensions.http.custom_response.local_response_policy` | Would produce the sterile exhaustion response (FR-014) declaratively, no policy code | Marked **work in progress, not suitable for production, unknown security posture**. FR-014 stays in the policy |
| `envoy.retry_host_predicates.omit_host_metadata` | "Reject hosts by metadata match" reads like per-request suspension | `metadata_match` is **static configuration** under `envoy.lb`, not per-request. Cannot express a suspension that changes at runtime |
| `envoy.router.cluster_specifier_plugin.lua` | Per-request cluster choice | Resolved once at route selection, not attempt-aware. Does not help a retry reach a different cluster |
| `envoy.extensions.http.stateful_session.*` (cookie/header/envelope) | Same override-host family | A different *writer* of the same mechanism. Our policy writes the metadata directly; no benefit |
| Internal redirect predicates (`previous_routes`, `filter_state`, …) | Central to the withdrawn chain-re-entry mechanism | Mechanism withdrawn by directive |

### Watch item — likely future migration

`envoy.router.cluster_specifier_plugin.priority_group` selects a priority group **by request attempt
count**, then a cluster within it, *"with optional per-request overrides via dynamic metadata."* That
is attempt-aware selection at **cluster** level rather than endpoint level — which would dissolve R1
entirely, since per-provider clusters would need no flattening at all.

**Status corrected 2026-10-23, then adopted — see §10.** This was recorded as "in-flight upstream work" because
the pinned control-plane library lacks it. Reading the Envoy source tree showed it is **merged and
released in Envoy 1.40.0** (2026-10-13). It is a live option, not a future one, and §10 evaluates it
properly.

## 7. Source-level verification of Override Host (2026-10-23)

§2 M8 and §3 were written from proto comments and documentation. Reading the implementation
(`source/extensions/load_balancing_policies/override_host/load_balancer.cc` and
`override_host_filter_state.h`, `envoyproxy/envoy` main) confirms the core mechanism and corrects
three assumptions. **These corrections are load-bearing.**

### Confirmed

- **Ordering across attempts is real and positional.** The parsed list is stored once in request
  filter state (`OverrideHostFilterState`) with a monotonically advancing index; each host selection
  calls `consumeNextHost()`, returning `host_list_[host_index_++]`. Attempt N gets entry N. This is
  the behavior the whole design rests on, implemented exactly as the proto comment describes.
- **A single value is split on commas** (`absl::StrSplit(hosts.value(), ',', absl::SkipWhitespace())`),
  whitespace stripped — the comma-separated list form is real.
- **Matching is a string lookup** against the cluster's cross-priority host map, keyed by the host's
  `address()->asString()`.

### Correction 1 — host health is **not** checked (highest impact)

`findHost()` carries an explicit upstream TODO:

> `// TODO(yanavlasov): Validate that host health status did not change.`

The lookup returns any host present in the map, **regardless of health or outlier ejection**. The
earlier claim that unavailable targets are "skipped for free" holds only for addresses **absent from
the endpoint set** — an unhealthy or ejected host is still selected.

**Consequence**: FR-025 (never attempt a suspended target) is enforced **solely** by the policy
omitting that target from the published list. There is no proxy-side second layer for health.

### Correction 2 — exhaustion falls back to round robin, it does not fail

When `getEndpoint()` walks past the end of the list it returns `nullptr`, and `chooseHostInternal()`
calls `fallback_picker_lb_->chooseHost(context)`. A retry beyond the list length does **not** error —
it picks a host by round robin, possibly one already attempted.

**Consequence**: aligning `num_retries` to exactly `len(chain) - 1` is **safety-critical, not
tidiness**. The `previous_hosts` predicate is a required guard but *not* a guarantee — see §8,
correction 4.
Publishing fewer addresses than `num_retries` allows is a live footgun — surplus attempts are served
by round robin. Validation must enforce the relationship and a test must assert it.

### Correction 3 — `selected_host_key` is not in the pinned version

Envoy main has a `selected_host_key` field that writes the chosen endpoint back into dynamic
metadata — the natural way for an upstream filter to learn which target an attempt selected. The
pinned `go-control-plane/envoy v1.37.0` `OverrideHost` message has only `override_host_sources` and
`fallback_policy`.

**Consequence**: the upstream filter cannot learn the target from that field. It must derive identity
another way — endpoint metadata carried via `internal_upstream` `passthrough_metadata`, or the
upstream host's own address. Spike T006 must confirm whichever is chosen.

### Note on internal addresses (R1)

Documentation says `IP:Port`, but the implementation performs a plain string lookup against
`address()->asString()`. An internal address renders in its own string form, so an internal-listener
endpoint **may** match when the published list carries that exact string. That is **undocumented
behavior that happens to work**, not a contract. If spike T005 relies on it, record the reliance
explicitly and re-verify on every Envoy bump — or prefer an addressing scheme the documentation
actually promises.

## 8. Documentation sweep beyond extensions (2026-10-23)

Coverage and limits are stated in §9. This section records what the sweep changed.

### Confirmed — the design premise, stated by Envoy itself

*Life of a Request* settles the question the whole mechanism rests on, in Envoy's own words:

> *"as the upstream HTTP filter chain is upstream of the router filter, it is run per each retry
> attempt allowing header manipulation per retry and including information about the upstream stream
> and connection."*

and, for the router, *"the route selection is finalized and a cluster is picked"* once at
`decodeHeaders()`. Downstream filters run once per request; upstream filters run per attempt. That is
exactly the division this design is built on, now sourced from the canonical lifecycle document
rather than inferred.

The transient-failures FAQ independently recommends the Previous Host retry predicate *"to select
different upstreams for retries,"* noting that *"typically failures on that same host are likely to
continue for some time"* — the same conclusion reached in §7, correction 2.

### Correction 4 — `previous_hosts` is not an absolute guarantee

`host_selection_retry_max_attempts` is documented as:

> *"The maximum number of times host selection will be reattempted before giving up, **at which point
> the host that was last selected will be routed to**. If unspecified, this will default to retrying
> once."*

So when a host predicate rejects a host, selection is retried only a bounded number of times, and
then the **rejected host is used anyway**. `previous_hosts` reduces the chance of re-attempting a
target; it does not forbid it.

**Consequence**: at-most-once (FR-010) rests on `num_retries == len(chain) - 1`, not on the
predicate. Raise `host_selection_retry_max_attempts` above the chain length as hardening, but do not
treat it as the guarantee. §7 correction 2 is amended accordingly.

### Correction 5 — rate-limited retry back-off would wreck failover latency

`RateLimitedRetryBackOff` parses `Retry-After` / `X-RateLimit-Reset` from the upstream response and
backs off for that interval, with `max_interval` defaulting to **300 seconds**. LLM providers
routinely send `Retry-After` on 429.

This is a **trap, not a feature, for this design**. Failover moves to a *different* target, so
waiting out the primary's rate-limit window is precisely wrong — a 429 from provider A must not delay
the attempt against provider B.

**Decision**: do **not** configure `rate_limited_retry_back_off`. Keep `retry_back_off` at a small
base interval so successive attempts are effectively immediate, and record this as a deliberate
choice so nobody "improves" it later by honoring `Retry-After`.

### Adopted — `per_try_idle_timeout`

> *"Specifies an upstream idle timeout per retry attempt (including the initial attempt)."*

Distinct from `per_try_timeout`: it bounds a stalled stream rather than total attempt duration. For
streaming LLM responses this is the right instrument for a provider that accepts the request then
stops producing tokens. Add alongside `per_try_timeout` (FR-005).

### Considered and rejected — hedging

`HedgePolicy.hedge_on_per_try_timeout` issues a second attempt when the per-try timeout fires
*without cancelling the first*, leaving multiple upstream requests in flight and returning the first
to complete. It would genuinely cut tail latency on a slow provider.

**Rejected**: for LLM traffic a hedged attempt is a **second billed inference call**, and both may
complete. That contradicts SC-011 (one client request, one unit of cost) and FR-016 (attribution to
the serving target). Note `initial_requests` and `additional_request_chance` are marked
`not-implemented-hide` upstream, so only the per-try-timeout form exists anyway.

### Considered — `retriable_headers`

Retries can be triggered by response *headers* as well as status codes. Some providers signal
overload or model unavailability in headers rather than status. Not required for launch; recorded as
a cheap future extension of `failureConditions`.

### Checked — outlier detection does not help here

Ejection marks a host unhealthy so it is skipped *during load balancing*. But §7 correction 1
established that Override Host looks hosts up directly in the cross-priority host map and does not
consult health. An ejected host therefore remains selectable through the override list, so outlier
detection cannot serve as the suspension mechanism. It remains useful only as independent protection
on the fallback LB path.

### Checked — `retry_options_predicates` has no built-in extensions

The field comment reads: *"add [#extension-category: envoy.retry_options_predicates] when there are
built-in extensions"* — i.e. none exist. Customizing request behavior between retries through this
hook would require a C++ extension and a custom Envoy build. Confirms the stock-image constraint.

## 9. Sweep coverage and limits

**Read**: Life of a Request; HTTP connection management (incl. internal redirects); router filter;
upstream `http_filters` / HTTP protocol options; load balancing policies incl. Override Host (docs,
proto, **and C++ implementation**); composite and aggregate cluster; retry predicates index and
`omit_host_metadata` proto; `RetryPolicy` and `HedgePolicy` protos in full; outlier detection;
transient-failures FAQ; ext_proc proto; the extension index end to end; internal-redirect predicates;
`internal_upstream` transport socket; stateful session; cluster specifier plugins; custom response
policies; Envoy version histories for availability.

**Not read**: the API reference exhaustively (it runs to thousands of pages); listener/network/UDP
filters, tracing, access-log sinks, QUIC, WASM, dynamic modules, rate-limit service, RBAC, and other
areas with no bearing on attempt sequencing; Envoy's issue tracker for known `override_host` defects;
Envoy's unit tests for which behaviors are pinned upstream.

**Not done**: nothing has been executed. Every claim here is from documentation, proto definitions,
or source reading. The Phase 2 spikes remain the only real verification.

## 10. Reassessment: `priority_group` cluster specifier (2026-10-23)

Prompted by a review of the Envoy source tree, which showed the capability dismissed in §6 as
unavailable is in fact released.

### What it does

`envoy.router.cluster_specifier_plugin.priority_group`, from the 1.40.0 changelog:

> *"splits the candidate clusters of a route into a list of named groups. The group is selected based
> on the attempt count of the request (the initial attempt uses the first group, the first retry uses
> the second group, and so on, **staying on the last group** once the attempts go past the end of the
> list) and the target cluster is then selected from the group based on the cluster weights. **The
> groups can be overridden per request by an optional dynamic metadata namespace.** This must be used
> together with `refresh_cluster_on_retry` to ensure the target cluster is re-selected on every
> attempt."*

Confirmed in source (`priority_group_cluster_specifier.cc`): `groupOverrideForAttempt()` reads
`stream_info.dynamicMetadata()` in the configured namespace — typed `PriorityGroupsOverride` preferred,
untyped struct of the same shape accepted — and indexes it with
`groupIndexForAttempt(attempt, size) = min(attempt, size - 1)`. Within a group, clusters are chosen by
weight.

### Why it is a better fit than Override Host

| Dimension | Override Host (§2 M8) | priority_group |
|---|---|---|
| Selection granularity | **Endpoints** of one cluster | **Clusters** |
| R1 — provider addressing | Needs internal-listener endpoints and relies on undocumented `address()->asString()` matching | **Dissolved.** Names existing per-provider clusters directly |
| Per-provider TLS / credentials | Requires `internal_upstream` + `transport_socket_matches` plumbing | Native — each cluster is untouched |
| Past the end of the list | Falls through to the fallback LB (round robin over *all* hosts, possibly already-attempted) | **Stays on the last group** — no arbitrary host |
| Health | Ignored at selection (§7 correction 1) | Normal cluster LB health handling inside the selected group |
| Mixing within one rank | Not expressible | Weighted clusters inside a group |
| Ordering obeyed per request | Yes | Yes |
| Downstream chain runs once | Yes | Yes |
| Configuration growth | None | None |

It removes the single largest open risk in the current design and two of the three corrections in §7,
while keeping everything the design depends on.

### What it costs

- **Envoy data plane bump 1.39.0 → ≥ 1.40.0.** 1.40.0 was released 2026-10-13 — ten days before this
  assessment. The current pin is `gateway/gateway-runtime/Dockerfile:24`.
- **Control-plane library bump.** `go-control-plane/envoy v1.37.0` has neither the plugin proto
  (`extensions/router/cluster_specifiers/` holds only `lua` and `matcher`) nor
  `RetryPolicy.refresh_cluster_on_retry` (zero occurrences).
- **`refresh_cluster_on_retry` is mandatory**, per the changelog — without it the cluster is not
  re-selected per attempt and the mechanism silently does nothing. Any adoption must set it and test
  for it.
- Dependency diligence per `.claude/rules/dependency-management.md`: changelog review, `govulncheck`,
  full IT suite against the new data plane.

### Decision: `priority_group` adopted (2026-10-23)

**Resolved.** The Envoy 1.40.0 bump was validated against the full gateway integration suite on CI,
with a control run to make the result readable:

| Run | Branch | Envoy | Result |
|---|---|---|---|
| Trial | `chore/envoy-1.40-trial` | 1.40.0 | **success** |
| Baseline | `chore/envoy-1.39-baseline` | 1.39.0 | **success** |

Both green. The baseline matters as much as the trial: it establishes the suite is healthy on this
fork, so the trial's pass is evidence rather than a vacuous green. Each run executed
`make test-integration`, `test-vhosts-single` and `test-vhosts-multi` on a clean `ubuntu-24.04`
runner — broader coverage than the local attempts, with the fresh-stack guarantee those lacked.

**`priority_group` is therefore the adopted mechanism**, superseding Override Host (§2 M8, §3). What
the change buys, beyond what §6 already listed:

- **R1 disappears entirely.** Groups name **clusters**, so the per-provider clusters are used
  directly. No internal listeners, no `internal_upstream` plumbing, no reliance on the undocumented
  `envoy://` address form that probe A4 had to discover from trace logs.
- **The POC's decisive finding is designed out.** Probe A3 showed Override Host sending traffic to an
  *excluded* target on overrun — a suspended provider receiving live traffic. Probe B4 showed
  `priority_group` repeating the last group instead, never reaching the excluded one.
- **Health works normally.** §7 correction 1 (Override Host ignoring host health, per its source TODO)
  no longer applies: once a group selects a cluster, that cluster's own LB handles health.

**Carried forward from the POC as binding implementation constraints:**

- `refresh_cluster_on_retry: true` is **mandatory**. Probe B3 showed that without it Envoy retried the
  *same failing provider three times* and returned 500 while the healthy target got nothing — a
  silent no-op plus a 3× load amplifier on a dying backend.
- The route cache **must be cleared** after the policy writes its metadata. Probe B2 showed the first
  attempt otherwise uses the *configured* group order, honouring the override only from the retry.
  The policy-engine kernel already sets `ClearRouteCache: true`, so this is satisfied — but it needs
  a test, not an assumption.

**Dependency moves required:** `go-control-plane/envoy` v1.37.0 → **v1.40.0** (carries the plugin
proto and `RetryPolicy.refresh_cluster_on_retry`, neither present in v1.37.0; requires Go ≥ 1.26.0,
and the controller is on 1.26.5), and the Envoy image v1.39.0 → **v1.40.0**. The root
`go-control-plane` module stays at v0.14.0, already the latest.

## 11. Cluster specifier plugins — the complete set (2026-10-28)

§2 and §6 evaluated Lua and adopted `priority_group`, but never wrote up the rest of the family.
This section closes that: every cluster specifier plugin, assessed against what this feature
actually needs. Facts below come from the protos in
`go-control-plane/envoy@v1.40.0/extensions/router/cluster_specifiers/`, read directly.

**What the tree actually ships at v1.40.0:** `dynamic_modules`, `lua`, `matcher`, `priority_group`.
There is no `golang` and no `http` plugin anywhere in the module — "HTTP cluster specifier" is the
docs *heading* the five plugins sit under, not a plugin.

### The criterion that decides it

Failover needs the cluster to differ **per retry attempt of the same request**. Most specifiers
compute a cluster from the request, and the request is the same on attempt 2 as on attempt 1 —
same input, same answer, no failover. Two further requirements follow from the directives on this
feature: the ordering must be a **request-time** decision (no deploy-time enumeration of
orderings), and it must not require shipping native code in the data plane.

| Plugin | Re-invoked per attempt | Per-request ordering | Cost in the data plane | Verdict |
|---|---|---|---|---|
| `priority_group` | **Yes** — selects the group *by attempt count*; requires `refresh_cluster_on_retry` | **Yes** — per-request override via dynamic metadata | None; built in | **Adopted** |
| `dynamic_modules` | **Yes** — *"invoked again whenever a filter refreshes the route cluster, or a retry re-selects it when `refresh_cluster_on_retry` is set"* | Yes — decides from request headers + stream info, so dynamic metadata is reachable | A `.so` loaded via `dlopen`, built against Envoy's ABI | **Capable; rejected on cost** |
| `matcher` | Not established | Only via static match rules, or by encoding the target in a header | None; built in | Rejected — see below |
| `lua` | **No** — resolved once at route selection (§6) | n/a | Inline script | Rejected |
| `golang` | n/a | n/a | Envoy **contrib** image + `.so` | Not available |

### `dynamic_modules` — the real alternative, and why not

This is the one genuine competitor, and the earlier record missed it. Its proto states plainly that
it is re-invoked when a retry re-selects the cluster under `refresh_cluster_on_retry`, and that the
module *"must be able to produce a decision from the request headers and the stream info alone"* —
which is exactly the information our ranking travels in. A module could therefore read the
published `wso2.failover` ranking and pick the attempt's target itself.

Rejected on cost, not capability:

- It is a shared object `dlopen`-ed into the proxy process. It must be compiled against Envoy's ABI
  and re-pinned on every Envoy bump, and it ships inside the data-plane image.
- It runs in-process with no sandbox: a fault in the module takes the proxy down, not the request.
- It would reimplement, in native code we maintain, what `priority_group` already does
  declaratively — attempt-indexed selection with a per-request metadata override.

Worth keeping on file: if a future requirement needs selection logic that attempt-indexing cannot
express (a decision depending on the *response* of the previous attempt, say), this is the
extension point that could carry it, and the published-ranking metadata would transfer unchanged.

### `matcher` — why it cannot give request-time ordering

Even granting it were re-invoked per attempt, its decision is a **static match tree** built at
configuration load. The only per-attempt input that varies is `x-envoy-attempt-count`, so a match
tree could map attempt 1 → cluster A, attempt 2 → cluster B — but that mapping is fixed at deploy
time. Making the ordering per-request would mean either enumerating every ordering in the match
tree (the composite-cluster explosion this feature was directed away from) or putting the target
name in a header for the tree to match on — which is `cluster_header` with extra steps, and carries
`cluster_header`'s own problems (§T024u2).

### `golang` — not available to us

Not present in `go-control-plane/envoy@v1.40.0` at all. It is an Envoy **contrib** extension, so
adopting it means switching the data plane from `envoyproxy/envoy` to the contrib image
(`gateway/gateway-runtime/Dockerfile`), on top of the same `.so`/ABI costs as `dynamic_modules`.
Ruled out on availability and operational surface before capability is even reached.

### Route specifiers — a different extension point, not an alternative (2026-10-28)

`extensions/router/route_specifiers/` is a separate family from `cluster_specifiers/`, and the two
**compose** rather than compete. A route specifier participates in route *resolution* — per its
docs, for each request the module *"keeps the route that route matching resolved, refines it,
replaces it with one of the route templates it declares, drops it, or lets route matching carry on
with the next route"* — and can also change path rewrites, metadata and header mutations. A cluster
specifier then picks the upstream for whatever route that produced. The proto says so directly:
when the module selects a template, *"the action is resolved the usual way, including
weighted_clusters and cluster specifier plugins."* `priority_group` keeps working underneath one.

Only `dynamic_modules` exists in this family today, and it **is** present in our pinned
`go-control-plane/envoy@v1.40.0` (the docs heading reads 1.41.0-dev, which is misleading).

It does **not** escape deploy-time enumeration for what this feature needs: templates are *"built
and validated once, when the specifier is configured"*, so the module selects among pre-declared
routes rather than synthesising one per request — the same limitation that rules out `matcher` for
ordering. It also carries the identical `.so`/ABI/in-process cost as the dynamic-modules cluster
specifier.

Where it could matter later: it can rewrite the path and set metadata **at route resolution**,
which is part of what the upstream ext_proc position does for us today (the per-attempt provider
re-point, FR-038). Before anyone builds on that, note the open question — invocation is *"while the
route is being resolved, and again whenever the route is recomputed"*, and unlike the dynamic-modules
*cluster* specifier the proto does **not** state whether a `refresh_cluster_on_retry` retry counts
as a recomputation. That needs a probe, not an assumption.

### Correction to the earlier record

§6 and the summary framing implied `priority_group` was the *only* attempt-aware specifier. That is
wrong: `dynamic_modules` is attempt-aware too. `priority_group` was the right choice, but for a
narrower reason than "nothing else can do it" — it is the only one that does it **without shipping
native code into the data plane**.


## Sources

- [Envoy Override Host load balancing policy](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/override_host) · [override_host proto](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/load_balancing_policies/override_host/v3/override_host.proto) · [Envoy 1.35.0 release](https://github.com/envoyproxy/envoy/releases/tag/v1.35.0) · [Gateway API Inference Extension — endpoint picker protocol](https://gateway-api-inference-extension.sigs.k8s.io/guides/implementers/)
- [Envoy composite cluster](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/composite_cluster) · [aggregate cluster](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/aggregate_cluster) · [Envoy 1.37.0 release notes](https://www.envoyproxy.io/docs/envoy/latest/version_history/v1.37/v1.37.0)
- [HTTP connection management — internal redirects](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/http/http_connection_management) · [router filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/router_filter) · [Life of a Request](https://www.envoyproxy.io/docs/envoy/latest/intro/life_of_a_request)
- [upstream `http_filters`](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/upstreams/http/v3/http_protocol_options.proto) · [ext_proc filter](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/ext_proc/v3/ext_proc.proto) · [Lua filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/lua_filter) · [header mutation filter](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/header_mutation_filter)
- [envoy#35877 — retries do not cross clusters](https://github.com/envoyproxy/envoy/issues/35877) · [data sharing between filters](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/advanced/data_sharing_between_filters)
- [Agent Router — data plane architecture](https://theagentrouter.ai/docs/concepts/architecture/data-plane/) · [provider fallback](https://theagentrouter.ai/docs/0.3/capabilities/traffic/provider-fallback/) · [agent-router#2671 — discarded body mutations](https://github.com/theagentrouter/agent-router/issues/2671) · [project transition](https://theagentrouter.ai/blog/envoy-ai-gateway-is-now-agent-router/)
- [kgateway v2.1 — Envoy-based AI gateway deprecation](https://kgateway.dev/blog/kgateway-v2.1-release-blog/)
- Local verification: `go-control-plane/envoy@v1.37.0/extensions/load_balancing_policies/override_host/v3/override_host.pb.go`; `.../extensions/clusters/composite/v3/cluster.pb.go`; `gateway/gateway-runtime/Dockerfile:24`; `gateway/gateway-controller/go.mod:7`
