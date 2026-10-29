---

description: "Task list for Model Failover Policy"
---

# Tasks: Model Failover Policy

**Input**: Design documents from `/specs/001-model-failover-policy/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [poc-results.md](./poc-results.md)

**Tests**: Test tasks ARE included — FR-012c and FR-030d mandate specific tests, and Constitution
Principle IV requires unit plus real-runtime integration tests in the same PR.

**Organization**: Grouped by user story. **Core-first sequencing**: all control-plane, runtime and
Envoy configuration work completes before any policy-module code (first `gc:` file is T030).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on incomplete work)
- **[Story]**: US1–US6 from spec.md
- Paths prefixed `gc:` are in the external policy repo `wso2/gateway-controllers`

## Path Conventions

- `gateway/gateway-controller/` — control plane, generates Envoy configuration
- `gateway/gateway-runtime/` — data plane (Envoy + policy-engine)
- `gc:policies/<name>/` — policy modules, pinned via `gateway/build-manifest.yaml`
- `gateway/it/` — godog BDD integration suites

---

## Mechanism (binding for every task)

Envoy's **`priority_group` cluster specifier plugin** (research.md §10). The downstream chain runs
**once**, writing a ranked list of priority groups — each naming **clusters** — into dynamic metadata
namespace `wso2.failover`. Envoy selects the group by attempt number, staying on the last group when
attempts overrun. Per-attempt work runs in the **upstream** filter chain.

Two settings are load-bearing and were proven necessary by POC probes, not inferred:

- **`refresh_cluster_on_retry: true`** — without it, probe B3 showed Envoy retrying the *same failing
  provider three times* and returning 500 while the healthy target got nothing.
- **Route cache cleared after the metadata write** — without it, probe B2 showed attempt 1 using the
  *configured* order, honouring the per-request override only from the retry onward.

---


> **Agent Router comparison** (source-verified, 2026-10-27): see [`agent-router-comparison.md`](agent-router-comparison.md). Three of their choices are adopted — per-cluster filter placement, response handled downstream, server-side baseline — and the core mechanism choice is explicitly not, with the reason recorded.
>
> **Reconciliation with the original design document** (composite clusters, probe recovery, per-provider upstream policies): see [`design-doc-reconciliation.md`](design-doc-reconciliation.md). It records what was carried in, what this spec supersedes, and three open conflicts (C1–C3) that still need a decision.

## Phase 1: Setup — version pins

- [X] T001 Bump `ARG ENVOY_VERSION` to `v1.40.0` in `gateway/gateway-runtime/Dockerfile` (already validated on CI: trial and 1.39.0 baseline both green)
- [X] T002 Bump `github.com/envoyproxy/go-control-plane/envoy` from `v1.37.0` to `v1.40.0` in `gateway/gateway-controller/go.mod`, run `go mod tidy`, and confirm the root `go-control-plane` stays at `v0.14.0`
- [X] T003 Confirm every CI image and build environment uses Go `>= 1.26.0` as required by `go-control-plane/envoy@v1.40.0`, recording the check in `gateway/gateway-controller/README.md`
- [X] T004 [P] Run `govulncheck ./...` and `go-licenses check ./...` after the bump and record results in the PR description per `.claude/rules/dependency-management.md`
- [ ] T005 [P] Read the Envoy 1.38/1.39/1.40 changelogs for anything else affecting generated config and note findings in `specs/001-model-failover-policy/research.md` §10

---

## Phase 2: Foundational — Core Platform (BLOCKING, no policy-module code)

**Goal**: The gateway can walk a ranked list of clusters across retry attempts, driven by request
metadata, verified without any policy module existing.

### 2a. Control plane — generated configuration

- [X] T006 Add failover fields to `gateway/gateway-controller/pkg/models/runtime_deploy_config.go`: resolved target list, `failureConditions`, `suspension` (incl. `maxDurationSeconds`, `backoffResetSeconds`, `errorRatePercent`, `windowSeconds`, `minimumRequests`, `p95LatencyMs`), `perAttemptTimeoutMs`, resolved chain length
- [X] T007 Generate the `priority_group` cluster specifier plugin in `gateway/gateway-controller/pkg/xds/failover_specifier.go` with `override_metadata_namespace: wso2.failover` and a default `priority_groups` list built from the configured targets, each group naming an existing per-provider cluster
- [X] T008 Attach the plugin to failover-enabled routes via `cluster_specifier_plugin` in `gateway/gateway-controller/pkg/xds/translator.go`, leaving per-provider clusters untouched
- [X] T008a Read the `model-failover` policy off an operation and resolve its targets to clusters in `gateway/gateway-controller/pkg/transform/failover.go`, dropping a target whose provider names no upstream definition rather than rejecting the deployment (FR-006a). Without this step nothing ever populates `Route.Failover` and every generated-config task above is unreachable at runtime
- [X] T008b Call `buildFailoverConfig` when constructing each route in `gateway/gateway-controller/pkg/transform/restapi.go`, passing a provider-to-cluster resolver built from the API's upstream definitions. Cluster names derive from the upstream definition name, never from the provider alias a policy author writes, so only the control plane can perform this mapping — which is why the policy publishes group names alone
- [X] T008c [P] Unit-test resolution and the silent no-op in `gateway/gateway-controller/pkg/transform/failover_test.go`: an absent policy yields no config, one unresolvable target leaves the rest of the chain intact, and a policy where nothing resolves yields no config at all
- [X] T008d [P] Pin the group-name and metadata-namespace contract on BOTH sides — `gateway/gateway-controller/pkg/xds/failover_specifier_test.go` and `gc:policies/model-failover/publish_test.go` — with the identical literals. A rename on either side makes Envoy fall back to the configured order silently: the request still succeeds, so no "does failover work" test can catch it
- [X] T009 Generate the route `retry_policy` in `gateway/gateway-controller/pkg/xds/translator.go` with `num_retries` = chain length − 1, `retry_on` always including `retriable-status-codes`, `retriable_status_codes` from `failureConditions.statusCodes`, `per_try_timeout` from `perAttemptTimeoutMs`, and `per_try_idle_timeout` to bound a stalled stream
- [X] T010 Set `refresh_cluster_on_retry: true` on every generated failover retry policy in `gateway/gateway-controller/pkg/xds/translator.go` — without it the mechanism is a silent no-op (POC probe B3)
- [X] T011 Ensure `rate_limited_retry_back_off` is never emitted and `retry_back_off.base_interval` stays small in `gateway/gateway-controller/pkg/xds/translator.go`; honouring a failing provider's `Retry-After` (300s default ceiling) before trying a *different* provider is backwards for failover
- [X] T012 Verify `NormalizePath`, `MergeSlashes` and `PathWithEscapedSlashesAction` remain set on every generated HCM including failover routes, per `.claude/rules/go-control-plane-xds-security.md` directive 6, in `gateway/gateway-controller/pkg/xds/translator.go` — verified: there is exactly one `hcm.HttpConnectionManager{}` construction (translator.go:1524) and it sets all three; failover adds no new listener type, and `translator_test.go:1507` already pins the settings
- [X] T013 Scope all generated failover config to the attaching operation in `gateway/gateway-controller/pkg/xds/translator.go` so shared provider clusters used by other APIs are never mutated (FR-030c) — the specifier plugin is named `failover-groups/<routeKey>`, collected per translation pass, and references existing clusters by name without rewriting any of them
- [X] T014 [P] Implement schema-level validation **only** in `gateway/gateway-controller/pkg/config/failover_validator.go`: numeric ranges per `contracts/model-failover-policy.yaml` (`errorRatePercent` 0–100, `windowSeconds` >= 1, `minimumRequests` >= 1, `p95LatencyMs` >= 0, `maxDurationSeconds` >= `durationSeconds`, `backoffResetSeconds` >= 1, `perAttemptTimeoutMs` < operation request timeout). Unresolvable model/provider references MUST NOT be rejected (FR-006a). Single-field ranges are already enforced generically from `policy-definition.yaml`; this file adds only the two cross-field rules a JSON schema cannot express, wired into both `validatePolicy` and `validateAttachedPolicyParams` so no attachment path can skip them
- [x] ~~T015~~ **WITHDRAWN 2026-10-29.** Reported the routing-policy suspension supersession at deployment time (FR-026b). With ~~T072~~ withdrawn there is no supersession to report — a routing policy's suspension is untouched by failover being attached.
- [X] T016 [P] Add generated-config assertions in `gateway/gateway-controller/pkg/xds/failover_specifier_test.go` for plugin type, metadata namespace, default group order, and that per-provider clusters are unmodified
- [X] T017 [P] Add retry-policy assertions in `gateway/gateway-controller/pkg/xds/translator_test.go` for `num_retries`, `per_try_timeout`, `per_try_idle_timeout`, presence of `retriable-status-codes` in `retry_on`, **presence of `refresh_cluster_on_retry: true`**, and **absence of `rate_limited_retry_back_off`**

### 2b. Runtime — metadata write path and per-attempt position

**Verified at runtime 2026-10-26 — this phase is the blocker, and the reason is sharper than
the plan assumed.** An end-to-end run (`IT_FEATURE_PATHS=features/failover-core.feature`)
confirmed the whole control-plane half works: the specifier generates two correctly-named
groups, `refresh_cluster_on_retry` fires, `x-envoy-attempt-count: 2` reaches the client, and
Envoy's `/clusters` counters show attempt 2 genuinely dispatched to the *second* group's
cluster.

The request still failed, because **switching the cluster changes nothing for an LlmProxy
additional provider**. Every provider cluster is a loopback to the same socket
(`127.0.0.1:8080`); what actually selects a provider is the route's `regex_rewrite`
substitution — `/fo-rescue-proxy(.*)` → `/fo-dead-ctx\1` — which is computed once per route
from the route's compiled-in upstream and is byte-identical on every attempt. So both attempts
re-entered the gateway on the *primary* provider's context and hit the same dead upstream.

The POC did not surface this because it used distinct real clusters, not loopback clusters.
Consequence: T021–T023 must rewrite the provider context path (and credentials) per attempt —
re-pointing the cluster alone is insufficient, and no amount of control-plane work can
substitute for it.

**Probe D (2026-10-26) validated the fix.** See `poc-results.md` § Probe D. An upstream-position
filter runs exactly once per attempt and its `:path` mutation takes effect for that attempt;
a chain-indexed rewrite drove attempt 1 to provider A and attempt 2 to provider B, and the
client received 200. Three constraints the implementation must honour:

1. **Mutations accumulate across attempts.** The upstream filter sees the previous attempt's
   edits, not a pristine request. Compute the rewrite from an immutable stashed original
   (dynamic metadata), never from the current `:path` — a naive prefix yields
   `/ctx-attempt-2/ctx-attempt-1/...`.
2. **The filter cannot learn the selected cluster.** `upstreamClusterName()` and
   `upstreamHost()` are both nil at the upstream position. Derive the target from
   `x-envoy-attempt-count` indexed into the published chain.
3. **`include_request_attempt_count: true` is required on the virtual host** (note: NOT
   `include_attempt_count_in_request`) or the attempt number is invisible upstream. This is a
   second, distinct flag from the `include_attempt_count_in_response` already generated by
   T022b.

**Decision (2026-10-26): the upstream position hosts ext_proc, not a failover-specific filter.**
The platform is introducing upstream and downstream policy types so an author can specify any
policy to run on the upstream path. That mechanism must exist regardless, so a bespoke Lua
path-rewrite for failover would be thrown away. Sequencing therefore becomes: upstream ext_proc
position (T021) → execution classes (T024) → failover's own per-attempt handler (T022/T023).

Two of Probe D's constraints become **platform contract** questions rather than failover details,
because they now apply to every author-written upstream policy:

- *Mutation accumulation.* An upstream policy that appends a header appends it again on each
  retry. Probe D showed this concretely (`/ctx-attempt-2/ctx-attempt-1/...`). The kernel must
  either hand each attempt a pristine request or document the re-entrancy contract loudly;
  "the policy author will remember" is not a design.
- *Target identity.* **Resolved by Probe E (2026-10-26): ext_proc receives correct per-attempt
  identity.** `xds.cluster_name` was `provider_a` on attempt 1 and `provider_b` on attempt 2,
  with `upstream.address` matching. The attempt-index-into-chain workaround Probe D forced is
  therefore unnecessary and should NOT be built: a per-attempt policy learns its target from
  the attributes, so it never has to know the failover chain at all. Attributes arrive
  namespaced as `envoy.filters.http.ext_proc.<name>`; `xds.route_name` came back empty.


- [X] T018 **Already satisfied — verified live 2026-10-26.** The engine writes a policy's `DynamicMetadata` straight through; the IT run showed `wso2.failover.priority_groups` reaching Envoy in the ext_proc response and the specifier acting on it. Original task: Implement the priority-groups metadata write path in `gateway/gateway-runtime/policy-engine/internal/kernel/` so a policy can publish `priority_groups` (ordered list of `{ name, clusters: [ { cluster_name, weight } ] }`) under namespace `wso2.failover`
- [ ] T019 **Likely already satisfied — needs one confirming test, not an implementation.** `internal/kernel/translator.go` sets `ClearRouteCache: true` on the metadata paths; confirm it fires on the failover publish specifically, since without it attempt 1 uses the pre-metadata route. Original task: Ensure the route cache is cleared after that metadata write in `gateway/gateway-runtime/policy-engine/internal/kernel/`, so attempt 1 honours the per-request order rather than the configured default (POC probe B2); the kernel already sets `ClearRouteCache: true` — confirm it applies on this path
- [ ] T020 **Confirmed NOT done** — nothing in `internal/` references the namespace. Lower risk than it reads, since a client cannot set Envoy dynamic metadata directly, but it stays open as defence in depth against a future header-to-metadata filter. Original task: Strip any inbound header or metadata named `wso2.failover` at ingress in `gateway/gateway-runtime/policy-engine/internal/kernel/` and reject the request if present — an accepted client value would let a caller steer traffic to an arbitrary cluster
- [X] T021a Set `mutation_rules: { allow_all_routing: true }` on the upstream ext_proc filter in `gateway/gateway-runtime/router/` — without it ext_proc silently drops any mutation of `:path`, `:method`, `:authority`, `:scheme` or `host`, with no error (Probe E). This widens what every upstream policy may rewrite, so it is a platform decision to make deliberately, not a flag to copy in
- [ ] T021b Decide and document the per-attempt re-entrancy contract for upstream policies in `gateway/gateway-runtime/policy-engine/internal/kernel/`: either hand each attempt a pristine request or state loudly that a policy sees the previous attempt's mutations. Probes D and E both confirmed accumulation (attempt 2 arrives carrying attempt 1's rewritten `:path`). An author-written policy that appends a header will append it twice on a retry — "the author will remember" is not a design (FR-012a)
- [X] T020a Generate `include_request_attempt_count: true` on the virtual host alongside the existing `include_attempt_count_in_response` (T022b) in `gateway/gateway-controller/pkg/xds/translator.go` — without it the per-attempt filter cannot tell which attempt it is serving (Probe D)
- [X] T021r0 **PREREQUISITE, done** — key the empty alias on the primary provider's own API-scoped cluster rather than the shared main upstream slot (FR-043b). Surfaced by checking whether per-cluster placement fits this topology; without it, two LlmProxies with loopback primaries publish different per-attempt chains under the same key and one API's credential reaches another API's request
- [X] T021r **DONE — the upstream filter chain is attached PER CLUSTER.** Attach via `HttpProtocolOptions.http_filters` on each cluster's `typed_extension_protocol_options`, the way Agent Router's extension server does, instead of `router.Router.UpstreamHttpFilters` (FR-043a). The current placement is listener-wide, which is why `policy_engine.upstream_processing` had to default off — per-cluster placement removes that tax by construction, so the flag and the planned `ExtProcPerRoute` gating (T024g) both become unnecessary. Everything else from T021 is kept: `allow_all_routing`, the request attributes, and the gRPC position metadata. `attachUpstreamFilters` merges into any protocol options a cluster already carries — overwriting would silently downgrade an HTTP/2 cluster to HTTP/1 — and is idempotent so re-translation does not stack duplicates. `upstream_processing` now DEFAULTS ON and remains only as an operator kill-switch; the IT config no longer opts in
- [X] T021 DONE (superseded by T021r) — verified at runtime 2026-10-27: the engine logs show the per-attempt handler running with the correct cluster and attempt number, for both the request and response halves. Control plane generates the chain; stream identification is structural — the router now generates `upstream_http_filters` (ext_proc + upstream_codec) behind `policy_engine.upstream_processing`, off by default because the chain is listener-wide. REMAINING: the policy engine must serve this second ext_proc stream, which is the runtime half. Original task: Wire the upstream filter position (`envoy.filters.http.upstream`) in `gateway/gateway-runtime/router/` so a per-attempt handler runs on every retry. **Use `envoy.filters.http.ext_proc`, not Lua**: the platform is introducing upstream and downstream policy *types* so an author can target the upstream position directly, so this position must host the general policy engine rather than a failover-specific filter. Failover is the first consumer of that mechanism, not a special case
- [X] T021c Identify an upstream stream STRUCTURALLY rather than by inference: the router sets `x-wso2-policy-position: upstream` as gRPC initial metadata and the engine reads it in `internal/kernel/upstream_position.go`. Necessary because Envoy keys ext_proc attributes by the filter's CANONICAL type, not its instance name, so the distinct filter name does not separate the two streams in the payload — and guessing the position means guessing which chain runs, which is the one that injects credentials (GO-AUTH-017). Every ambiguous case resolves to downstream, the safe direction
- [X] T024x **DONE — the provider context path is rewritten per attempt, with NO new transport.**

  The transport problem dissolved: a route config already carries `upstream_definition_paths` (definition name to base path) and `upstream_base_path` (the route's own), and the engine already receives both. The cluster name encodes the definition it was built from, so matching the two recovers the provider's context. Adding `xds.route_name` to the upstream filter's request attributes was the only configuration change.

  This avoided BOTH earlier options and their costs: no field on the SDK's `PolicyChain` (which is consumed as a published version, so it would have needed an SDK release), and no `routeSignatureView` change (so the stale-chain hazard never arose). Recorded because the hazard was previously described as the thing blocking this task — it turned out to be avoidable rather than manageable.

  The rewrite is applied in the handler rather than by a policy, because it must happen for EVERY attempt including ones whose provider has no authored policies. Built from the BASELINE path, never the incoming one, or a retry compounds it (FR-038).

  *What is needed:* on an attempt routed to provider P, `:path` must become `P's context + the operation path`. The upstream handler already has the cluster and may mutate `:path` (`allow_all_routing` verified in place). What it lacks is the provider's base path.

  *Why it needs new transport:* the downstream path solves the equivalent problem with `execCtx.upstreamDefinitionPaths`, built during route resolution — but an upstream stream has no execution context by design (it resolves no route and initialises no API metadata, which is what keeps once-per-request work off the retry path). The base path is also not derivable from the cluster name, which encodes the definition name but not the context.

  *The rewrite itself:* `newPath = clusterBasePath + strings.TrimPrefix(path, routeBasePath)`. Both base paths are already on the RDC — each provider cluster carries its own `UpstreamCluster.BasePath`, and the route's is `UpstreamClusters[route.Upstream.ClusterKey].BasePath`. Strip from the BASELINE path (T024f), never from the incoming one, or a retry compounds the previous attempt's rewrite.

  *Transport options, in preference order:*
    1. Extend the upstream chain's published resource with `upstream_base_path` and `route_base_path`. Reuses the existing resource type and receive path. **Caution:** `internal/xdsclient/signature.go`'s `routeSignatureView` is PROJECTED, not hashed wholesale — a new field consumed by chain building MUST be added there too, or reconciliation silently reuses a stale chain. The file says so explicitly, and its completeness test does not catch the omission; `TestRouteSignatureView_Completeness` needs a matching mutator.
    2. A separate per-cluster resource type. More machinery, no stale-chain hazard.

  Option 1 is smaller but has the sharper failure mode; whoever implements it should read that warning first.
 Verified 2026-10-27 that the chain runs and can mutate headers, but the route's `regex_rewrite` points every attempt at the PRIMARY provider's context, so a loopback attempt lands on the wrong provider regardless of which cluster was selected. This is the last thing between the current state and a working cross-provider failover
- [ ] T022 Expose the selected provider cluster to the upstream handler in `gateway/gateway-runtime/policy-engine/internal/kernel/`, so per-attempt credential injection and request translation know which target they are serving
- [ ] T023 Ensure the upstream handler reads the **current buffered body** as mutated by the downstream chain, never an earlier snapshot, in `gateway/gateway-runtime/policy-engine/internal/kernel/` (FR-012a)
- [ ] T024 **(load-bearing, not optional)** Add the `PolicyExecutionClass` declaration to `gateway/gateway-runtime/policy-engine/internal/registry/` with values `once-per-client-request` (downstream position) and `per-attempt` (upstream position), defaulting to `once-per-client-request` when undeclared so an undeclared side-effecting policy fails safe (FR-012f)
- [ ] T025 [P] Unit-test the metadata write path, route-cache clearing and ingress stripping in `gateway/gateway-runtime/policy-engine/internal/kernel/failover_metadata_test.go`

- [ ] T024t Document in the policy description that a requested model matching no chain still traverses the CONFIGURED group order, because the route carries its cluster specifier unconditionally — publishing no ordering does not bypass failover (FR-050, verified 2026-10-26). Recommend a `"*"` catch-all for a defined destination

- [X] T024u1 Document the failover-vs-`cluster_header` precedence at the route-action switch in `gateway/gateway-controller/pkg/xds/translator.go` and warn at generation time when failover displaces header routing, so the change is visible in logs rather than silent (FR-051, FR-053)
- [ ] T024u2 Decide the policy-combination rule: reject `model-failover` alongside a policy that depends on `x-target-upstream` steering to an arbitrary upstream definition (dynamic-endpoint and similar), following the precedent in `pkg/config/llm_validator.go`. Rejecting the generic condition is NOT an option — `UseClusterHeader` is true for every multi-provider LlmProxy, which is exactly where failover is wanted
- [ ] T024u3 Extend the published-ranking contract to cover non-model cluster steering, or record that failover and dynamic-endpoint style redirects are not composable. Today a routing policy can express "use this provider" as a ranking, but there is no way to express "use this arbitrary upstream definition" (FR-052)
- [ ] T024u4 [P] Add a generated-config test asserting that a route with BOTH failover and `UseClusterHeader` emits the specifier plugin and not `cluster_header`, pinning the precedence so a future reorder of the switch cannot silently flip it

- [X] T024w5 **CLOSED POSITIVE (Probe H, 2026-10-27)** — all three questions yes: honoured with a route-config retry policy, raises the budget above the configured `num_retries` (2→3, reaching a previously unreachable group), and stripped from client requests (the filter chain saw `nil`, so a caller cannot inflate their own spend). Original task: Probe whether `x-envoy-max-retries` lets the downstream policy size the attempt budget PER REQUEST, which would solve T024w4 exactly rather than by worst-case bound. Must establish three things: (a) the header is honoured when the retry policy comes from route config rather than the router filter; (b) it can raise the budget above the configured `num_retries`, not only lower it; (c) Envoy strips it from client requests on this listener — otherwise a caller inflates their own retry budget and multiplies inference spend, which is a cost and abuse problem, not just a correctness one
- [X] T024w4 Set `x-envoy-max-retries` to the length of the ranking it publishes, in the same downstream pass, in `gc:policies/model-failover/failover.go` (FR-059a). Probe H proved this works and is client-safe, so order AND length both become per-request. **Supersedes the deploy-time counting approach** — the controller no longer needs to anticipate the longest possible ranking. Original alternative: Size the attempt budget for the longest ranking the operation can produce, not for model-failover's own chain. `buildFailoverRetryPolicy` sets `num_retries = ChainLength() - 1` from the CONFIGURED members, so a routing policy ranking five targets on a two-member failover config gets two attempts and the rest are silently never tried. T024w frees the ORDER; this frees the LENGTH. Options: count (not enumerate) the attached routing policy's targets, or take an explicit configured ceiling — the latter is defensible on its own since every attempt is a billed inference call
- [X] T024w6 [P] Assert in a generated-config test that the HCM sets no `internal_address_config`, so downstream clients stay EXTERNAL and Envoy keeps sanitizing `x-envoy-*`. Probe H's client-safety result depends entirely on this; if someone later adds an internal-address range for an unrelated reason, `x-envoy-max-retries` silently becomes a client-controllable inference-spend multiplier with no other symptom
- [X] T024w1 Pass a provider-to-cluster map to the policy at deploy time via `systemParameters` in `gateway/gateway-controller/pkg/transform/failover.go` (FR-058). The cluster depends on the provider alone, so this map is small and static
- [X] T024w2 Publish COMPLETE groups from `gc:policies/model-failover/publish.go` — `{name, clusters:[{cluster_name, weight}]}` rather than name only (FR-057). Probe B already demonstrated inline clusters work; weights must be integral, since the specifier parses weight as a number and rejects anything below 1
- [X] T024w3 Make `"*"` members resolve (FR-059): the controller keeps the model literal in the configured group while the policy substitutes the client's model, so under name-only publication a wildcard member is permanently unresolvable and silently falls back by position. Add a test pinning a wildcard member end to end
- [ ] ~~T024v1~~ SUPERSEDED by T024w: with clusters published inline the controller no longer needs to pre-enumerate every reachable model. Original text: Generate a priority group for EVERY `(model, provider)` reachable on the operation in `gateway/gateway-controller/pkg/transform/failover.go` — flatten the attached routing policy's targets (`model-round-robin` / `model-weighted-round-robin` `models[]`) into the same group set as model-failover's own members, de-duplicated by identity (FR-054). Without this a routing policy's ranking cannot be honoured for anything it alone knows about
- [ ] T024v2 **Reduced to defence in depth by T024w2** — a published group now carries its own cluster whenever the controller supplied a map, so position-fallback is only reachable when the map is absent (an older controller paired with a newer policy). Original task: Drop any chain entry that has no configured group before publishing, in `gc:policies/model-failover/publish.go` (FR-056). Probe G showed Envoy does NOT skip an unresolvable group name — it falls back to the configured group at that position, which can send the request to a target the ranking never nominated and burn the whole attempt budget there
- [x] T024v3 [P] Integration-test the cross-policy case: attach `model-round-robin` and `model-failover` to one operation, with the routing policy ranking a target that failover does not list, and assert the request is served by the ranked target rather than the operator's first configured one (FR-054, FR-055)

### 2b-ter. Model-keyed fallback chains (schema rework, FR-045–FR-050)

The policy's user-facing schema now follows the reviewed design document: `targets[]` are chains
keyed by the **client-requested** model, each with its own `fallbacks[]`, with `"*"` catch-all and
wildcard members. This replaces the flat single-chain shape and invalidates the parsing, chain
resolution and controller transform written against it.

- [X] T024k Rewrite `gc:policies/model-failover/policy-definition.yaml` to the reviewed shape: nested `targets[].fallbacks[]`, `statusCodes`, `perTryTimeout`, `suspendDuration`, `suspendAfterFailures`, `maxSuspendMultiplier`. Omits `recoverAfterSuccesses` (probe recovery was superseded by escalating backoff) and keeps `backoffResetSeconds`, which escalating backoff needs
- [X] T024l Rewrite parsing in `gc:policies/model-failover/config.go` and the chain types in `target.go` for nested chains, and carry the new parameter names through
- [X] T024m Select the chain by requested model in `gc:policies/model-failover/chain.go`: exact match wins, `"*"` is the catch-all, no match means pass through unchanged (FR-045, FR-046, FR-050)
- [X] T024n Resolve a `"*"` member to the client's own requested model, with one health circuit covering every model it serves (FR-047), and make a fallback with no `provider` inherit its chain's target provider rather than the proxy primary (FR-048)
- [X] T024o Switch `Mode()` in `gc:policies/model-failover/failover.go` to buffer the request body and read the requested model from it (FR-049). This is a real cost the flat shape avoided — the reviewed design accepts it ("Request body is buffered (model lookup and replay)"). Confirm an operation that does NOT attach the policy is not forced to buffer
- [X] T024p Flatten every member of every chain into priority groups in `gateway/gateway-controller/pkg/transform/failover.go` and `pkg/xds/failover_specifier.go` — the controller cannot know which chain a request will select, so all members need groups and the policy orders the matching subset per request
- [X] T024q Rework `gateway/gateway-controller/pkg/config/failover_validator.go` and its tests for the new parameter names, and add the deploy-time rejections the design document lists: unknown provider, target with no fallback, duplicate model or member, bad status codes, `perTryTimeout` x chain length over the route timeout
- [X] T024r [P] Update `gateway/it/features/failover-core.feature` and the controller/policy unit tests to the new schema; every existing test using the flat `targets[]` shape is now invalid

### 2b-bis. Upstream and downstream policy types (platform capability, FR-035–FR-044)

Carried in from the original design document's "Per-provider upstream policies" and "Upstream
ext_proc and existing policies" sections, and the 2026-10-09 design review ("We decided to go with
the upstream ext proc. Under `spec.upstream` or `spec.providers` we will add a section called
`policies`"). Probe E validates the mechanism. These tasks are a **general platform capability**:
model failover is the first consumer, so they live here rather than in a user-story phase, and
nothing in Phase 3+ can work without them.

- [ ] T030k **Grade failover health from the ACCESS LOG, not only the response-headers phase.** A stream that opens `200` and dies mid-body is currently recorded as a SUCCESS, so a provider that consistently fails mid-stream looks like the healthiest member and attracts more traffic. Envoy reports the true outcome as `upstream_reset_after_response_started{...}` in `response_code_details` on the access-log entry. The plumbing exists: `internal/utils/access_logger_server.go` already runs the service, `internal/analytics/analytics.go:441` already reads the field, `internal/kernel/fault_source.go:125` already matches the `upstream_reset` prefix. Mid-stream RETRY is closed by Envoy design (issue #5876) and is not in scope here — only the grading.
- [ ] T030l **Decide `reset` vs `reset-before-request` in the generated retry policy.** `reset` retries an upstream reset that happens after the upstream received the FULL request, so an LLM prompt already being processed can be re-sent to another provider — billed twice and potentially answered twice. `reset-before-request` retries only when no bytes were sent upstream. Currently we emit the broader `reset` by inheritance, not by decision.
- [ ] T030m **Resolve failover member hostnames at deploy time and surface the ones that do not.** Today `validateUpstreamUrl` (`pkg/config/api_validator.go:158`) is purely syntactic — scheme, parseability, non-empty host — and nothing resolves. Measured consequence: a member whose hostname does not resolve leaves its cluster with **zero hosts**, so Envoy answers locally with `no_healthy_upstream`, which is not an attempt and therefore not retriable; that request is lost. Suspension already covers request 2 onward (measured), so this closes the remaining first-request gap for the cause that actually occurs — a typo or a decommissioned host that was never right.
  **Scope**: `spec.upstream.main`/`sandbox`, `spec.upstreamDefinitions[]`, and the provider entries an `LlmProxy` resolves failover targets against.
  **Decide: warn or reject?** Recommend **warn**, not reject. The controller and the data plane can hold different DNS views (split-horizon, separate namespaces), so a controller-side failure is not proof the gateway cannot reach it — and rejecting a deploy on a transient DNS blip is worse than the failure it prevents. This also keeps faith with the standing directive that unresolvable configuration degrades rather than blocking a deployment. A reject option should be an explicit, off-by-default opt-in if anyone wants it.
- [x] T024a0 **DEFECT, found 2026-10-28 — FIXED 2026-10-29.** Per-attempt `policies` support exists only on the LEGACY provider shape. `spec.providers[]` with `isPrimary` is the **canonical** shape — `NormaliseLLMProxyAttachments` calls the other one "legacy", and it is the shape the approved design document's own example uses — but `LLMProxyProviderEntry` has no `policies` field, `LLMProxyAttachment` does not carry one through normalisation, and `buildUpstreamPolicyChains` reads `spec.AdditionalProviders` only. Consequence for a proxy authored canonically: no per-attempt chain is built for any non-primary provider, so a failover attempt reaches that provider **without its transformer and without its credential** — silently, and it is the exact failure this feature exists to prevent. Fix: add `policies` to `LLMProxyProviderEntry`, carry it on `LLMProxyAttachment`, and build chains from the normalised attachments rather than from `AdditionalProviders`.

  **Done.** `policies` added to `LLMProxyProviderEntry` in `api/management-openapi.yaml` and regenerated; `LLMProxyAttachment` carries it from both shapes; `buildUpstreamPolicyChains` now walks `NormaliseLLMProxyAttachments` instead of `spec.AdditionalProviders`. Two things surfaced while fixing it:
  - **The same omission existed in validation.** `ValidateLLMProxyPolicies` also walked `AdditionalProviders` directly, so a canonically-authored proxy could list `model-failover` per upstream — the FR-040 violation this feature explicitly refuses — and deploy clean, with its per-attempt policy names and versions never resolved. Fixed the same way, with field paths still naming the entry the operator authored (`spec.providers[i]` vs `spec.additionalProviders[i]`) via `SourceIndex`.
  - **Two existing tests built impossible config.** `proxyWithProviders` produced a spec with `additionalProviders` and no primary `provider`, and one test passed an entirely empty `LLMProxyConfigData`. Both only worked because the builder never normalised. Corrected rather than worked around.

  Regression tests added in `pkg/transform/upstream_chains_test.go` (canonical shape, canonical primary, both-shapes-agree) and `pkg/config/policy_validator_llm_test.go` (FR-040 on both shapes). **Each was run against the pre-fix code and fails there** — the canonical cases report `map[]` and the legacy ones still pass, so the tests discriminate rather than merely pass. Full controller suite green (35 packages).

  **Verified at runtime 2026-10-29** — see `poc-results.md` § "Canonical provider shape, end to end". Doing so exposed a THIRD defect the unit tests could not reach: `LLMTransformer.Transform` (`pkg/transform/llm.go:151`) dereferenced `proxy.Spec.Provider.Id`, which is nil in the canonical shape, and `processEventSafely` recovered the panic — so the management API answered `201`, `status.state` read `deployed`, the log said "Artifact deployed successfully", and the gateway served `404 no-api-found`. A canonically-authored proxy could not be deployed at all, silently. Introduced by this feature, in the block that points the empty alias at the primary's API-scoped cluster. Fixed via `PrimaryLLMProxyAttachment`; matching on `EffectiveName()` instead of `Id` additionally closes a latent legacy bug where an aliased primary missed the lookup and left the empty alias on the SHARED main slot — the cross-API credential collision that block exists to prevent. End-to-end result: `200`, `attempt-count: 2`, `/anthropic-upstream/v1/messages`, Anthropic-shaped body, `x-api-key`, `anthropic-version`, and the author's own per-attempt marker header all present on the failover attempt.
- [~] T024a LlmProxy `additionalProviders[].policies` DONE — added to `api/management-openapi.yaml` and regenerated (the only non-blob change is the new field). REMAINING: `LlmProvider` `upstream.policies` and `RestApi` `upstream.main/sandbox.policies`. Original task: Add the `policies` list to each upstream in the management API schema and generated types: `LlmProxy` `providers[].policies`, `LlmProvider` `upstream.policies`, `RestApi` `upstream.main.policies` and `upstream.sandbox.policies` (FR-036). Note `RestApi` gets this too — upstream policies are not an LLM-only feature
- [~] T024z **MEASURED, NOT DOING YET — threshold stated** — hold the post-downstream body in engine memory and replace it from the headers phase (`CONTINUE_AND_REPLACE`) instead of re-sending it per attempt. Both sides measured per REQUEST 2026-10-27 (two attempts, body in both directions): wire costs 48 µs / 67 KB at 10 KB, 573 µs / 482 KB at 100 KB, 2.72 ms / 4.5 MB at 1 MB; memory is FLAT at ~180 µs / 21 KB regardless of size, costing ~10 MB resident per 1000 in-flight 10 KB bodies and ~101 MB at 100 KB. Threshold: not worth it for prompts <= 10 KB; worth taking for >= 100 KB with a material failover rate, and then with a SIZE CEILING so resident memory stays bounded. An earlier single-round-trip measurement understated the wire cost by ~4x and its conclusion is superseded. NOTE: this is NOT blocked by how transformer policies are written; the engine populates `reqCtx.Body` itself, so the source can change without touching a policy. See `poc-results.md` § Measurement
- [X] T024y Support the request BODY at the upstream position — a transformer rewrites the payload and implements that as `OnRequestBody`, so without a body phase it is never called and a fallback provider receives the request in the PRIMARY's format. The headers phase returns a `mode_override` asking for a BUFFERED body only when `chain.RequiresRequestBody`, so a header-only chain still pays no buffering cost. The body is re-sent per attempt rather than held in memory: that is the price of letting the EXISTING transformer policies work unchanged, since they take the body through the standard policy interface rather than from engine-internal state (Agent Router can avoid it because its translator is called directly, not as a generic policy)
- [~] T024b BUILDERS DONE, CUT-OVER PENDING — `pkg/transform/upstream_chains.go` compiles a chain per provider alias in the fixed order (author policies → transformer → auth LAST), carried on `RuntimeDeployConfig.UpstreamPolicyChains`. Deliberately NOT yet consumed: the transformer and credential still run downstream gated on the selected provider, exactly as before, because moving them only becomes safe once the runtime dispatches these chains (T024d/T024e) and doing both at once leaves no working state in between. Chains are keyed by CLUSTER NAME, not provider alias, so the runtime looks one up directly from the `xds.cluster_name` Envoy supplies per attempt rather than carrying a translation table; `RuntimeDeployConfig.ProviderClusters` carries the alias-to-cluster mapping, computed once where routes are built. The real transformer/auth builders are now wired: `TransformWithUpstreamPolicies` returns each provider's per-attempt work as a NAMED `ProviderPolicies{Transformer, Auth}` — not a positional list, which would apply a credential-only provider's key as a translator. Gated behind `policy_engine.per_attempt_provider_policies`, OFF by default: when off, behaviour is byte-identical to before (the policies stay downstream); when on, they move to the per-attempt chain and are removed from the downstream one, so they can never run twice. REMAINING: exercise the per-attempt placement at runtime, then flip the default. Original task: Compile per-upstream chains in `gateway/gateway-controller/pkg/transform/` in the controller-fixed order: author policies → transformer → authentication, with authentication LAST so no author-written policy observes the credential (FR-039)
- [X] T024c Reject `model-failover` in any upstream's `policies` list at validation time in `gateway/gateway-controller/pkg/config/` — it is attached once in the operation's policies and its per-attempt work is added by the controller (FR-040)
- [X] T024b2 Publish `UpstreamPolicyChains` in the policy-xDS snapshot (`gateway-controller/pkg/policyxds/snapshot.go`) under the `upstream:<cluster>` key namespace, reusing the existing PolicyChain resource type so the engine's receive path serves both. The engine stores every chain in one keyed map, so no receive-side change was needed; `Kernel.UpstreamChainFor` looks one up from the `xds.cluster_name` attribute. The prefix is a cross-module contract pinned by a test on each side — a mismatch is silent, since the engine would simply find no chain and the attempt would run unmodified
- [X] T024d Dispatch the per-attempt chain for the selected upstream only, in `gateway/gateway-runtime/policy-engine/internal/kernel/`, so an attempt never runs another upstream's transformer or credential (FR-041). **Use the ext_proc `xds.cluster_name` attribute as the gate** — Probe E confirmed it carries correct per-attempt identity, which removes the original design's need for a per-attempt `model-failover` copy whose only job was to set `selected_provider`
- [X] T024er **DONE — the per-attempt chain is REQUEST-ONLY.** `ResponseHeaderMode: SKIP` on the upstream processor; `handleUpstreamResponseHeaders`, `upstreamResponseHeadersResponse`, `passthroughResponseHeaders` and the `upstreamAttempt` per-stream state are all deleted (FR-037a). Response work moves downstream, which already knows which target answered via the attempt count. Agent Router does this deliberately — *"to avoid the shenanigans around the retry+the upstream filter"* — and the per-stream state we needed exists precisely because of that interaction: a response at the upstream position carries no cluster attribute to re-resolve its chain from
- [X] T024e DONE (superseded by T024er) — both directions, verified at runtime — `internal/kernel/upstream_dispatch.go` runs the chain through the SAME `executor.ExecuteRequestHeaderPolicies` the downstream path uses, with `Downstream == nil`, and the response half through `ExecuteResponseHeaderPolicies`, which already iterates in reverse so each policy unwraps in the order it wrapped. Required correcting the generated filter's ResponseHeaderMode from SKIP to SEND: a provider's transformer cannot convert that provider's response back without the response phase, so skipping it would leak the provider's native format to a client that asked in another. Original task: Call the same `On*` methods for upstream policies with `Downstream == nil`, request steps in chain order and response steps reversed, so one policy interface serves both positions (FR-037)
- [X] T024fr **DONE — the baseline is SERVER-SIDE state keyed by request**, not round-tripped through dynamic metadata (FR-038a). The current mechanism writes the baseline on attempt 1 and reads it back on attempt 2, which rests on an assumption never measured — that an *upstream* ext_proc's metadata write survives into the next attempt. Agent Router keeps the post-downstream request in the ext_proc server's own memory and sets `RequestBodyMode: NONE` because of it. Holding it server-side removes the assumption instead of needing a probe to confirm it, and saves a metadata round trip per attempt. `attemptBaselineStore` keys on `x-request-id`, is first-write-wins (a later attempt must never overwrite the first attempt's shape), and is TTL-bounded with a sweep on growth — nothing deletes an entry on the last attempt because nothing knows which attempt is last
- [~] T024f MECHANISM DONE (superseded by T024fr) — the baseline is captured on attempt 1 and carried in dynamic metadata, which persists across attempts of one request (the same mechanism the priority groups use). It cannot be published by the downstream pass: Envoy applies the route rewrite AFTER ext_proc, so the engine never sees the exact bytes attempt 1 sends. The upstream handler resets path/method/authority from it before running the chain, so a policy never observes the previous attempt's edits. NEEDS RUNTIME VERIFICATION: that an upstream ext_proc's dynamic-metadata write survives into the next attempt is reasoned, not measured. Original task: Start every attempt from the POST-DOWNSTREAM baseline in `gateway/gateway-runtime/policy-engine/internal/kernel/` (FR-038, FR-038a): snapshot the request once when the downstream chain finishes, and reset to that snapshot before each upstream invocation. Each attempt KEEPS every downstream mutation (masking, redaction, guardrail rewrites — FR-012a) and DISCARDS the previous attempt's upstream mutations (prior provider's path, transformer, credential). Not the raw client request: re-deriving from that would strip the downstream work, and re-running the downstream chain to recover it would charge quota and analytics once per attempt (FR-012d). Probes D and E both observed accumulation in raw Envoy — attempt 2 arrived carrying attempt 1's rewritten `:path`. This is the single biggest usability risk in the feature: an author-written upstream policy that appends a header will append it twice on a retry. Supersedes T021b, which asked whether to fix this or document it; the original design answers it — "Each attempt starts from the original request"
- [~] T024g **NOT NEEDED — dissolved by T021r.** Per-cluster filter placement satisfies FR-043 by construction: a cluster with no per-attempt work carries no filter, so there is nothing to gate. Original task: Generate the upstream chain only where per-attempt work exists, so an API with no upstream policies pays no second ext_proc round trip (FR-043). The original design states this as an exception for a multi-provider LlmProxy with no `model-failover`; re-derive the condition against the `xds.cluster_name` gate from T024d rather than copying it, since the stated reason for that exception ("nothing tells the upstream phase which provider an attempt is going to") no longer holds
- [ ] T024h Fail the attempt closed when the per-attempt chain cannot run, strip the internal provider-selection header before the response reaches the client, and ensure no upstream credential reaches the downstream response path (FR-042), per `.claude/rules/error-handling.md` and `authentication_authorization.md`
- [X] T024i1 Formalise transport-failure accounting by inference (FR-044): attempt-count attribution already records a transport-dead member as failed whenever a LATER attempt returns a real response, so the common case works. Correct FR-009/FR-018 and `classify.go` to state that rather than implying the policy observes transport errors directly — `isTransportFailure` cannot fire as written, since Envoy never invokes ext_proc for its own local replies
- [X] T024i2 **CLOSED NEGATIVE (Probe F, 2026-10-26)** — outlier detection cannot be used. Ejection works (`consecutive_local_origin_failure` + `split_external_local_origin_errors` detects the dial failure), but with the default panic threshold a single-host provider cluster drops to 0% healthy, Envoy enters panic mode and dials the dead host anyway, so ejection is cosmetic; and with `healthy_panic_threshold: 0` the ejected cluster returns `503 no healthy upstream` with `attempts=1` — not retryable, chain never reaches group 2. Disabling panic mode converts a working failover into a hard client failure, so it is strictly worse than leaving it on. The design document'"'"'s rejection holds for `priority_group`, not just composite. Original task text: The design document rejected it on composite clusters ("the composite still sends attempt 1 to an ejected leaf → 503 no healthy upstream, not retryable"), but the IT run showed a zero-host cluster under `priority_group` return a 503 that WAS retried into group 2. If this holds, Envoy handles transport-dead members natively and T024i3 is unnecessary — run this before building it
- [~] T024i3 **NOT DOING — limit accepted 2026-10-26.** The gap is confined to a chain whose members all fail at transport level, where suspension has nowhere healthier to route; the common case is already covered by the attempt-count inference (T024i1). Recorded as a user-facing limit in the policy description and as FR-009a/FR-018h instead of building a feedback loop. Revisit only if the all-dead-chain case proves to matter in practice. Original plan: feed transport failures into the health store over the EXISTING access-log pipeline. The router already streams access logs over gRPC (`createAccessLogConfig`, ALS cluster in `pkg/xds/translator.go`) carrying `%RESPONSE_FLAGS%` and `%UPSTREAM_TRANSPORT_FAILURE_REASON%`. Add the policy engine as a second sink so those records land in the same process as the in-memory health store — no new component or protocol
- [~] T024i-orig SUPERSEDED by T024i1/T024i2/T024i3 — original wording: Envoy does not invoke ext_proc for its own local replies, so a connection failure, reset, or per-attempt timeout produces NO policy invocation at either position. Today `classify.go`'s `isTransportFailure` assumes the response phase sees them — it does not. Either source these outcomes elsewhere (access log, stats) or document that transport failures fail over but never contribute to suspension, and correct `classify.go` and FR-009/FR-018 to match
- [ ] T024j [P] Integration-test that an upstream policy on provider B does not run on an attempt routed to provider A, and that provider A's credential never appears in a request to provider B (FR-041, FR-042)

### 2c. Core verification — no policy module involved

- [X] T026 Write `gateway/it/features/failover-core.feature` tagged `@model-failover` and register it in `gateway/it/suite_test.go`, asserting that a request to a failing target is rescued by the next target in the chain and that `x-envoy-attempt-count` reports two attempts
- [ ] T027 Add a scenario to `gateway/it/features/failover-core.feature` asserting that with `refresh_cluster_on_retry` absent the same cluster is retried and failover does not occur — pinning the failure mode so a future config change cannot silently reintroduce it
- [X] T028 Add scenarios to `gateway/it/features/failover-core.feature` asserting a healthy first target is never re-attempted (attempt count 1), that a non-failover-worthy status is returned as-is on the first attempt, and that an exhausted chain returns the upstream failure rather than a synthesised one
- [ ] T029b Re-register `features/failover-core.feature` in `gateway/it/suite_test.go` once T021–T023 land; it is deliberately out of the default suite today because its core scenario cannot pass, and a permanently red shared suite trains everyone to ignore it
- [ ] T029c Provide a controllable failure backend for the IT suite: `ghcr.io/wso2/api-platform/sample-service` is a pure echo that reports `?statusCode=` back in its `query` field and still answers 200, so no scenario can currently force a chosen upstream status. Several existing round-robin/weighted scenarios assume otherwise and are presumably not asserting what they appear to
- [X] T029 Add a scenario to `gateway/it/features/failover-core.feature` asserting a policy whose targets all name undeployed providers is a silent no-op: the deployment is accepted, the request succeeds normally, and no attempt-count header is generated at all (FR-030b, FR-030e)
- [ ] T029a Add a scenario to `gateway/it/features/failover-core.feature` asserting that when attempts overrun the group list Envoy repeats the **last group** and never routes to a cluster absent from the published groups

**Checkpoint**: ranked, same-request failover works end to end with zero policy code.

---

> **END TO END, 2026-10-27**: cross-provider failover works against a running gateway — 200, two
> attempts, the fallback's own context, and the fallback's own per-attempt policy. See
> `poc-results.md` § END TO END. What remains for US1 is the transformer/auth migration (so a
> cross-vendor fallback is correctly formed and credentialed), suspension behaviour, and the
> exhaustion response.

## Phase 3: User Story 1 — Automatic fallback to a healthy model (P1)

**Goal**: A failing primary no longer reaches the client; the next target serves the request.

**Independent test**: Primary forced to fail, one healthy fallback, single request → `200` from the
fallback. Contrast: `model-round-robin` alone returns `500`
(`gateway/it/features/model-round-robin.feature:849`).

- [X] T030 [US1] Create the policy module `gc:policies/model-failover/` with `go.mod`, Apache-2.0 headers, and `policy.yaml` copied from `specs/001-model-failover-policy/contracts/model-failover-policy.yaml`
- [X] T031 [US1] Implement configuration parsing and defaulting in `gc:policies/model-failover/config.go` with exact values from data-model.md §1: `statusCodes` default `[429, 500, 502, 503, 504]` each in 400–599, `onTransportFailure` default `true`, `consecutiveFailures` default `3` minimum `0` where `0` disables, `durationSeconds` default `30` minimum `0`, `maxDurationSeconds` default `600`, `backoffResetSeconds` default `600`, `errorRatePercent` default `50` range 0–100, `windowSeconds` default `60` minimum `1`, `minimumRequests` default `20` minimum `1`, `p95LatencyMs` default `0` minimum `0`, `perAttemptTimeoutMs` default `30000` minimum `1`
- [X] T032 [US1] Implement `Target` identity as the `(model, provider)` pair in `gc:policies/model-failover/target.go`, where `model` has `minLength: 1` and `provider` is optional, with the pair — never `model` alone — as the health key
- [X] T033 [US1] Implement chain resolution in `gc:policies/model-failover/chain.go` producing the ordered target list, de-duplicated keeping each target's **first** position, skipping targets that do not resolve (FR-006a)
- [X] T034 [US1] Implement priority-group publication in `gc:policies/model-failover/publish.go`, rendering the resolved chain as an ordered `priority_groups` list into metadata namespace `wso2.failover` on the single downstream pass
- [X] T035 [US1] Make the policy inert when nothing resolves, in `gc:policies/model-failover/publish.go` — no metadata written, no telemetry, behaviour indistinguishable from the policy not being attached (FR-030b, FR-030e)
- [ ] T036 [US1] Declare execution classes in `gc:policies/model-failover/policy.yaml`: the publishing step `once-per-client-request`, the per-attempt adaptation step `per-attempt`
- [X] T037 [US1] Implement failover-worthy outcome classification in `gc:policies/model-failover/classify.go` for configured status codes plus transport failures (connection failure, reset, unavailable upstream, per-attempt timeout) which trigger regardless of `statusCodes`
- [X] T038 [US1] Ensure non-failover-worthy responses pass through unchanged in `gc:policies/model-failover/classify.go` so a client-input error is returned as-is with no fallback
- [ ] T039 [US1] Implement the exhaustion response in `gc:policies/model-failover/exhaustion.go` returning one consistent sterile error with no upstream body, provider hostname, or credential, per `.claude/rules/error-handling.md`
- [ ] T040 [US1] Pin `model-failover` at `v0.1.0` in `gateway/build-manifest.yaml` using the existing `gomodule:` entry convention
- [ ] T041 [P] [US1] Write `gateway/it/features/model-failover.feature` tagged `@model-failover` covering acceptance scenarios 1–5 of User Story 1, using the sample backend's `?statusCode=NNN` mechanism
- [ ] T042 [P] [US1] Add a regression scenario to `gateway/it/features/model-failover.feature` asserting a failing primary with a healthy fallback never returns `5xx` to the client
- [ ] T042a [P] [US1] Add a client-invisibility assertion to `gateway/it/features/model-failover.feature`: a request that fails over returns **exactly one** response with no 3xx, no duplicate or concatenated body, and no header revealing that more than one attempt occurred (FR-012g)
- [ ] T043 [P] [US1] Unit-test chain resolution, inert behaviour and group rendering in `gc:policies/model-failover/chain_test.go` and `publish_test.go`

**Checkpoint**: MVP — the headline value of issue #3356 is delivered.

---

## Phase 4: User Story 2 — Same-model failover across provider configurations (P1)

**Goal**: One model in two regions fails over between provider configurations with independent health.

**Independent test**: Two targets, same `model`, different `provider`; first forced to fail; second
serves with its own credentials; the first's suspension leaves the second healthy.

- [X] T022b Generate `IncludeAttemptCountInResponse` on the virtual host **only** when one of its routes fails over, in `gateway/gateway-controller/pkg/xds/translator.go`, so `x-envoy-attempt-count` reaches the policy's response phase while a vhost with no failover keeps byte-identical responses (FR-030i)
- [X] T022c Attribute the response outcome to the target that produced it in `gc:policies/model-failover/failover.go` using the attempt count: attempts 1..N-1 are recorded as failures, attempt N takes the final status. Without this a target that fails on every request is recorded healthy whenever failover rescued it, and is never suspended (FR-030h)
- [X] T022d Treat a missing or unparseable attempt count as one attempt in `gc:policies/model-failover/failover.go`, grading only the first target rather than inventing failures for targets that may not have been reached (FR-030j)
- [X] T022e [P] Unit-test outcome attribution in `gc:policies/model-failover/failover_test.go`: a two-attempt request with a successful final status must record target 1 failed and target 2 succeeded, and a target failing on every rescued request must reach suspension (SC-014a)
- [ ] T044 [US2] Implement per-attempt provider resolution in `gc:policies/model-failover/upstream.go`, applying the selected cluster's credentials and endpoint configuration at the upstream position
- [ ] T045 [US2] Enforce health-key independence for identical `model` values under different `provider` values in `gc:policies/model-failover/target.go`
- [ ] T046 [P] [US2] Add same-model multi-provider scenarios to `gateway/it/features/model-failover.feature` covering both acceptance scenarios of User Story 2
- [ ] T047 [P] [US2] Unit-test health-key independence in `gc:policies/model-failover/target_test.go`

---

## Phase 5: User Story 3 — Cross-provider failover with adaptation and configured fallbacks (P2)

**Goal**: A different vendor serves as fallback with correct auth and format; a premium target can be
held out of rotation.

**Independent test**: Vendor Y configured only on the failover policy while a routing policy spreads
load across in-house models — Y gets zero traffic while they are healthy, and serves once all are down.

- [ ] T048 [US3] Apply per-attempt request and response translation for the attempt's vendor in `gc:policies/model-failover/upstream.go`, composing with the existing `openai-to-*-transformer` policies rather than duplicating translation
- [x] T049 [US3] **DONE 2026-10-29, semantics revised.** `gc:policies/model-failover/chain.go` now attempts the configured fallbacks for the first target IMMEDIATELY after it, ahead of the rest of a published ranking — not as a last-resort tail. The original tail semantics let a routing policy silently outrank a fallback the publisher wrote deliberately (FR-021, FR-001b). Covered by `chain_resolve_test.go`, `chain_selection_by_target_test.go`, and **verified at runtime 2026-10-29** — see `poc-results.md` § "Merge rule, end to end". The runtime check was built to DISCRIMINATE: round-robin alternates between a target that has a configured chain and one that does not, and both are made to fail. Result over six requests, 3× alternating: configured target → `200`, `attempt-count: 2`, served by `/anthropic-upstream/…` (its own fallback fired); unconfigured target → `503`, `attempt-count: 1` (no fallback borrowed). The previous semantics would have rescued the second case, so the alternation is the proof, not the passing suite — the first version of this change passed every test while being wrong.
- [ ] T050 [US3] Attribute usage and cost to the serving target in `gc:policies/model-failover/attribution.go` so `llm-cost` and analytics reflect the vendor that actually served the request (FR-016)
- [ ] T051 [P] [US3] Add cross-provider and held-out-of-rotation scenarios to `gateway/it/features/model-failover.feature` covering all three acceptance scenarios of User Story 3
- [ ] T052 [P] [US3] Unit-test tail semantics in `gc:policies/model-failover/chain_test.go`

---

## Phase 6: User Story 4 — Suspension, aggregated criteria and escalating backoff (P2)

**Goal**: A known-down target stops costing every request an attempt; intermittent degradation is
detected; a target that keeps failing is held out for progressively longer.

**Independent test**: Three suspension paths each trip independently; a suspension that expires
returns the target to full rotation; a further failure doubles the duration up to the cap.

- [X] T053 [US4] Implement the `TargetHealth` store and state machine in `gc:policies/model-failover/health.go` with states `healthy` | `suspended` only — **no probing state** — and the transitions in data-model.md §5, using atomic per-key updates so concurrent requests cannot skip a threshold or double a suspension twice for one failure
- [X] T054 [US4] Define the replaceable health-state boundary in `gc:policies/model-failover/health_store.go` (FR-020a) so a shared cross-instance implementation can later be substituted without changing configuration, per-request semantics or telemetry; ship the in-memory per-instance implementation behind it
- [X] T055 [US4] Implement consecutive-failure suspension in `gc:policies/model-failover/health.go`: suspend at `consecutiveFailures`, reset the count on any success (FR-018)
- [X] T056 [US4] Implement windowed error-rate suspension in `gc:policies/model-failover/window.go`: rolling counters over `windowSeconds` expiring **by time, not request count**, suspending when the rate crosses `errorRatePercent` with at least `minimumRequests` in the window (FR-018a/b)
- [X] T057 [US4] Implement aggregate latency suspension in `gc:policies/model-failover/window.go` using a bounded latency reservoir over `windowSeconds`, suspending when p95 crosses `p95LatencyMs` subject to `minimumRequests` (FR-018e)
- [ ] T058 [US4] Keep `perAttemptTimeoutMs` as a per-request bound only in `gc:policies/model-failover/classify.go` — one slow request fails over and counts toward the failure and latency samples, but never suspends on its own (FR-018f)
- [X] T059 [US4] Implement escalating backoff in `gc:policies/model-failover/health.go`: a re-suspension doubles `currentDurationSeconds`, capped at `maxDurationSeconds`; the duration resets to base only after `backoffResetSeconds` of healthy operation (FR-019a/b/c)
- [X] T060 [US4] Return a target to **full** rotation when its suspension expires in `gc:policies/model-failover/health.go` — no probe phase, no sampled share (FR-019)
- [ ] T061 [US4] Exclude suspended targets from the published priority groups in `gc:policies/model-failover/publish.go` so a suspended target is never attempted and the skip costs no attempt (FR-025)
- [ ] T062 [US4] Return the exhaustion error immediately when every target is suspended in `gc:policies/model-failover/exhaustion.go`, without burning per-attempt timeouts
- [ ] T063 [P] [US4] Add suspension, escalation and recovery scenarios to `gateway/it/features/model-failover.feature` covering acceptance scenarios 1–6, 4a and 4b of User Story 4
- [ ] T064 [P] [US4] Add a scenario asserting a target failing roughly one request in three — never consecutively — is suspended once `minimumRequests` is met, and the record names the rate criterion (SC-011a)
- [ ] T065 [P] [US4] Add a scenario asserting a target whose p95 crosses the threshold is suspended even though no single request exceeded `perAttemptTimeoutMs` (SC-011b)
- [X] T066 [P] [US4] Unit-test the health state machine, all three suspension paths, backoff doubling, cap and reset in `gc:policies/model-failover/health_test.go` and `window_test.go`

---

## Phase 7: User Story 6 — Load spreading and failover together (P3)

**Goal**: A routing policy's ranking is obeyed exactly, and a suspended target stops receiving its share.

**Independent test**: Routing policy publishes `[B, C, A]`, B fails → C then A attempted in that
order; the downstream chain runs once; once B is suspended, published rankings exclude it.

- [x] T066a **DONE 2026-10-29.** Pinned as `[]map[string]string` with keys `model` (required) and `provider` (optional), under `model_routing.published_ranking`. **Built-in types only, deliberately** — a producer lives in another Go module, and the consumer also accepts `[]any` of `map[string]string`/`map[string]any` for a generically-assembled list. The contract's resolution example was also corrected: it still showed the superseded tail semantics. [US6] **Pin the wire representation** in `contracts/target-preference-list.md`. The contract fixes the *config* shape ("same shape as `model-round-robin.models[]`") but never the in-memory metadata representation, and the implementation filled that silence with a module-local struct. State the concrete carrier explicitly — a plain `[]map[string]any` with `model`/`provider` string keys, or an SDK-owned named type that every policy module imports — and state that a module-local struct is NOT permissible, with the reason (below). Producer and consumer cannot agree on a shape the contract does not name.
- [x] T066b **DONE 2026-10-29.** `publishedRanking` now calls `decodeRanking` instead of asserting `raw.([]Target)`. An unreadable value degrades to the single-selection path rather than failing the request, and one malformed entry costs that entry rather than the producer's whole ranking. [US6] **Fix the consumer's type assertion** in `gc:policies/model-failover/failover.go`. `publishedRanking` reads `raw.([]Target)` against model-failover's OWN named type. A routing policy in a different module publishes its own `[]Target`, and Go type assertions require *identical* types, not structurally compatible ones — so the assertion fails even when a well-behaved producer does exactly the right thing, and the code falls silently through to the single-selection path. Decode the T066a representation instead. **Verified 2026-10-27**: the multi-member path is dead code today — nothing publishes the key, and nothing could be read if it did (`poc-results.md`).
- [x] T066c **DONE 2026-10-29.** `ranking_decode_test.go` publishes from `producerTarget`, a type declared outside the consumer's own vocabulary, and asserts through `publishedRanking` — not `decodeRanking`. That distinction matters: the defect was at the call site, so a test calling the decoder directly passes against the broken reader and proves nothing. **Both foreign-producer tests were run against the pre-fix reader and fail there.** [P] [US6] Regression-test the cross-module case in `gc:policies/model-failover/`: publish a ranking as a type declared OUTSIDE the failover package (simulating a producer module) and assert the consumer accepts it. A test using failover's own `[]Target` passes against the broken reader and proves nothing — that is precisely why this defect survived.
- [x] T067 **DONE 2026-10-29.** Multi-member consumption was in fact completed by T049's merge-rule rewrite — `resolveChain` already walks `published[1:]` after the configured fallbacks. What was genuinely missing was a test driving the WHOLE POLICY with a multi-member ranking in the pinned carrier; every existing test reached `resolveChain` directly with an already-decoded `[]Target`, which is precisely the gap the T066b defect lived in — the resolution logic was right and the value never reached it. Added `ranking_multimember_test.go` (5 tests: fallbacks between ranked targets, unconfigured lead borrows nothing, de-duplication across ranking and fallbacks, same model under different providers kept distinct, single selection ≡ one-element ranking). Four fail against the pre-T066b reader; the fifth is an equivalence assertion that correctly holds either way. [US6] (depends on T066a/T066b) Implement MULTI-MEMBER preference-list consumption in `gc:policies/model-failover/chain.go` per `contracts/target-preference-list.md`. The single-selection path is done and tested (`chain_resolve_test.go`, `chain_selection_by_target_test.go`): first target from the ranking, that target's configured fallbacks next, remainder of the ranking after. What remains is consuming a ranking of more than one member, which nothing publishes yet — see T066a/T066b for why the carrier has to be fixed first.
- [x] ~~T068~~ **WITHDRAWN 2026-10-29 — implemented, then reverted by design decision.** Failover must NOT publish its health state to other policies. Suspension is per-instance state private to failover, and exposing it makes a routing policy depend on an internal view it cannot reason about or verify.
  It also buys nothing: `resolveChain` already filters suspended targets out wherever they appear, including the leading one, so a suspended target is never attempted no matter what a routing policy nominated. And it could not have been delivered as specified — both policies choose in the request-header phase, which runs in author-written `operationPolicies` order, so a routing policy placed first decides before failover could publish.
  **The flow is one-way by design**: a routing policy publishes its ranking, failover consumes it, nothing goes back. Reverted: `UnavailableTargets`, `encodeTargets`, the `model_routing.unavailable_targets` key, the header-phase publication, and `unavailable_publish_test.go`. FR-026 withdrawn, FR-027 revised to place the duty on chain resolution rather than on the producer, FR-019f rescoped to read-only health queries generally.

  Original task text, for the record: *Publish the healthy target set for the operation from `gc:policies/model-failover/health.go` so routing policies can rank over healthy targets only (FR-026).*
- [x] T069 **DONE 2026-10-29.** `publishedRanking` falls through to the single-selection keys and returns a one-element list, and `resolveChain` continues into the configured chain from there. Covered by `TestMultiMemberRanking_SingleSelectionBehavesAsAOneElementRanking` (asserts the two paths produce an identical chain), `TestPublishedRanking_UnreadableValueDegradesToSingleSelection`, and the existing `chain_selection_by_target_test.go` cases. [US6] Handle non-adopting routing policies in `gc:policies/model-failover/chain.go` by treating a single published selection as a one-element list and continuing into the configured chain (FR-023)
- [x] T070 **DONE 2026-10-29.** `model-round-robin` publishes its full rotation starting at the selected cursor position, under `model_routing.published_ranking`, in the pinned `[]map[string]string` carrier. Set ALONGSIDE the single-selection keys, never instead of them, so every existing consumer is unaffected (FR-026c). `selectNextAvailableModel` gained an index-returning variant — rotating from the selected position requires knowing where it landed, and searching back by value would be wrong for a configuration listing the same model under two providers. Covered by `rotation_publish_test.go` (6 tests). [US6] Publish the ordered target preference list from `gc:policies/model-round-robin/` as the full rotation starting at the current cursor (FR-028). **This is the ONE adopting policy in scope** — it exists to prove the contract end to end, not to roll the capability out. Chosen because its ranking is deterministic and therefore assertable in an integration test.
- [ ] ~~T071~~ **OUT OF SCOPE (2026-10-29).** Publishing a ranking from `gc:policies/model-weighted-round-robin/` is a straight repeat of T070 against a second policy and proves nothing T070 has not. Updating the remaining routing and load-balancing policies is separate work.
- [x] ~~T072~~ **WITHDRAWN 2026-10-29 — implemented, then reverted by design decision.** A routing policy's own suspension is NOT superseded when failover is attached; the two health views are independent and both correct.
  The requirement assumed a double count that does not occur. The downstream chain is entered **once per client request** — retries re-invoke only upstream filters — so a routing policy observes the FINAL status. A request failover rescued reports `200` and suspends nothing. A `5xx` arrives only once the chain is exhausted, which means every member failed *including the one the routing policy selected*, so suspending it is accurate — not a double count, and not a misattribution (the misattribution argument raised while reverting ~~T068~~ was wrong for the same reason).
  Reverted: `FailoverAttachedParam`, `policiesSupersededByFailover`, `InjectFailoverSupersession` and its call site, round-robin's `FailoverAttached` field and `suspensionEnabled()`, and 11 tests. FR-026a and FR-026b withdrawn; FR-026c now trivially satisfied. **T070 is unaffected** — publishing the rotation was always the correct direction, and the v1.2.0 bump stands on it alone.
- [x] T073 **DONE 2026-10-29.** `model-round-robin` v1.1.1 → **v1.2.0** in both `policy-definition.yaml` and `gateway/build-manifest.yaml`. A minor bump: publishing a new metadata key is purely additive and no existing behaviour changes. Only the one adopting policy; no other policy version changed.

- [ ] T074 [P] [US6] Write `gateway/it/features/model-failover-routing-layered.feature` tagged `@model-failover @routing-layered` covering all seven acceptance scenarios of User Story 6, asserting the published groups match the ranking and the downstream chain runs once
- [ ] T075 [P] [US6] Add a scenario to `gateway/it/features/model-failover-routing-layered.feature` asserting a non-adopting routing policy still composes via the one-element-list path
- [ ] T076 [P] [US6] Add a no-regression scenario asserting a routing policy deployed without failover behaves exactly as before, including its own `suspendDuration` (FR-026c, SC-012)

---

## Phase 8: User Story 5 — Observability of failover decisions (P3)

**Goal**: An operator can reconstruct any failover, and any suspension, from telemetry alone.

**Independent test**: Drive a failed-over request; telemetry names each attempted target, each
failure reason, the ranking source and the serving target.

- [ ] T077 [US5] Emit the `FailoverAttemptRecord` from `gc:policies/model-failover/telemetry.go` with attempts, outcomes, failure reasons, serving target, ranking source and skipped targets (FR-031)
- [ ] T078 [US5] Emit suspension and recovery records from `gc:policies/model-failover/telemetry.go` naming **which criterion fired and its observed value** — consecutive count, error rate with sample size, or latency percentile — plus the duration applied and escalation step (FR-018d, FR-019d)
- [ ] T079 [US5] Expose per-target counters for failovers, suspensions by criterion, recoveries and chain exhaustions in `gc:policies/model-failover/metrics.go` (FR-033)
- [ ] T080 [US5] Separate attempt-level from request-level reporting in `gc:policies/model-failover/telemetry.go` using Envoy's `upstream_request_attempt_count`, so analytics and cost report once per client request (SC-011)
- [ ] T081 [US5] Exclude credentials, authorization headers and prompt or completion content from all emitted telemetry in `gc:policies/model-failover/telemetry.go` (FR-034)
- [ ] T082 [P] [US5] Add observability scenarios to `gateway/it/features/model-failover.feature` covering all four acceptance scenarios of User Story 5, including the absence assertions
- [ ] T083 [P] [US5] Unit-test telemetry redaction and suspension-reason reporting in `gc:policies/model-failover/telemetry_test.go`

---

## Phase 9: Polish & Cross-Cutting Concerns

- [ ] T084 Write the body-mutation preservation test in `gateway/it/features/model-failover.feature` combining `pii-masking-regex` with failover and asserting on the bytes the **fallback upstream received**, not on policy logs (FR-012c)
- [ ] T085 Write the cross-API isolation test in `gateway/it/features/model-failover.feature`: two APIs sharing one provider cluster, one with failover and one without, asserting the second is unchanged against a no-failover deployment (FR-030d, SC-012a)
- [ ] T086 [P] Add a scenario to `gateway/it/features/model-failover.feature` asserting the policy attached to a kind with no model concept deploys successfully and is inert, with no telemetry about being inert (FR-030b/e)
- [ ] T086a [P] Add a scenario to `gateway/it/features/model-failover.feature` deploying the **same** failover configuration under both `kind: RestApi` and `kind: LlmProxy` and asserting identical failover behaviour in each, covering the positive half of applicability that T086 only tests negatively (FR-030a)
- [ ] T087 [P] Add a streaming-safety scenario to `gateway/it/features/model-failover.feature` asserting no failover occurs once response headers or streamed content have reached the client (FR-013)
- [ ] T088 [P] Add a client-disconnect scenario to `gateway/it/features/model-failover.feature` asserting traversal stops when the caller disconnects mid-chain
- [ ] T089 [P] Add a side-effect accounting scenario to `gateway/it/features/model-failover.feature` asserting one unit of quota, one rate-limit count, tokens billed for the serving attempt only, and one analytics record for a request that failed over twice (SC-011)
- [ ] T089a Add tag filtering to `gateway/it/suite_test.go` — set `Tags` in the `godog.Options` block (currently unset, so every feature runs) from an env var defaulting to `~@load`, so load scenarios are excluded from the default suite and CI
- [ ] T089b Add a `test-load` target to `gateway/it/Makefile` running the suite with `@load` instead of the default exclusion, documented in `gateway/it/README.md` as a **local-only** check not run by CI
- [ ] T089c Write a sustained-load check in `gateway/it/features/model-failover-load.feature` tagged `@model-failover @load`: drive **at least 10,000 requests** with one target failing continuously and assert no target is attempted more than once per request and the chain never loops, by comparing per-target hit counts against request count (SC-008). **Local only** — excluded from CI by T089a
- [ ] T089d [P] Measure failover-path latency in `gateway/it/features/model-failover-load.feature` (same `@load` tag): assert added end-to-end latency during a primary outage stays within `perAttemptTimeoutMs` plus the fallback's own response time, and drops to effectively zero once the primary is suspended (SC-003). **Local only**
- [ ] T090 [P] Document per-instance health detection in `gc:policies/model-failover/policy.yaml`, stating that across N instances a target may absorb up to N times the configured threshold before every instance suspends it (FR-020b)
- [ ] T091 Run `grep -rlE "resource: /(gateway|gateway/gateway-controller)" kb` and update every feature doc whose `sources:` lists a touched path, adding new surface to `sources:` and the Entry points table, per `.claude/rules/feature-docs.md`
- [ ] T092 Verify Codecov coverage has not regressed for every touched package by running `make test` in `gateway/it` and the module unit suites, per Constitution Principle IV
- [ ] T093 Execute `specs/001-model-failover-policy/quickstart.md` V0–V10 end to end against a built gateway and record outcomes

---

## Dependencies

```
Phase 1 Setup (version pins)
  ↓
Phase 2 Foundational — CORE PLATFORM (controller + runtime + Envoy config), no policy code
  2a control plane ─→ 2b runtime ─→ 2c core verification
  ↓ CHECKPOINT: ranked failover works with hand-supplied groups
  ├─→ Phase 3 US1 (P1) ─→ Phase 4 US2 (P1)
  ├─→ Phase 5 US3 (P2)        [needs US1 chain + publication]
  ├─→ Phase 6 US4 (P2)        [needs US1 publication]
  ├─→ Phase 7 US6 (P3)        [needs US1 + US4 health]
  └─→ Phase 8 US5 (P3)        [needs US1 + US4 records]
                ↓
         Phase 9 Polish
```

**Core-first directive honoured**: the first `gc:policies/` file is touched at T030. Everything in
Phases 1–2 is version pins, control plane, runtime, or Envoy configuration.

**Two tasks pin failure modes rather than features**: T027 asserts the `refresh_cluster_on_retry`
failure mode, and T017 asserts the absence of `rate_limited_retry_back_off`. Both encode POC findings
so a later config change cannot silently reintroduce them.

**Story independence**: US1 is a complete deliverable alone. US2 extends it with provider-scoped
identity. US3, US4 and US6 each add a separable capability. US5 is additive telemetry.

## Parallel Execution Examples

- **Phase 1**: T004 and T005 are independent of each other and of the pins
- **Phase 2a**: T014 and T015 share a validator file — sequence them; T016 and T017 are different test files
- **Phase 3**: T041, T042, T043 after T030–T040 land
- **Phase 6**: T063–T066 are different scenarios and test files
- **Phase 7**: T070 and T071 are different policy modules
- **Phase 9**: T086, T086a, T087, T088, T089, T090 are independent. T089a → T089b → T089c is a chain (filtering must exist before a `@load` scenario can be excluded); T089d is [P] with T089c once the file exists

## Implementation Strategy

**Core first.** Phases 1–2 deliver a gateway that walks a ranked list of clusters across attempts,
verified by `@failover-core` integration tests with no policy module. That is a releasable platform
capability and de-risks everything after it.

**MVP = Phase 1 + Phase 2 + Phase 3 (US1).** That removes the client-side retry burden issue #3356
was filed about.

**Increment 2**: US2 (regional failover — the most common real-world trigger, and ECI points 1–2).

**Increment 3**: US3 + US4 (cross-vendor resilience; US4 carries ECI points 3–5).

**Increment 4**: US6 + US5 (ranking adoption in the routing policies, and ECI point 6).

**Risk note**: the mechanism is proven (POC probes B1/B2/B4) and the Envoy 1.40.0 bump is CI-validated
against a 1.39.0 baseline. The remaining risk is concentrated in T023 (mutated body reaching attempt
2), which guards a security control — treat a failure there as blocking, not as a known issue.
