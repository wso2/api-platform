# Agent Router comparison — source-verified

Read against `github.com/theagentrouter/agent-router` (cloned 2026-10-27), not its documentation.
Agent Router is the closest comparable: Envoy-based, two ext_proc attachments, cross-provider AI
failover. The original design document's competitor analysis described it second-hand; this
checks the implementation.

Three of its choices are better than ours and are adopted. One is equivalent. The core mechanism
choice is NOT adopted, for a reason that holds.

---

## Adopted

### A1 — Upstream filters attach PER CLUSTER, not to the Router filter

`internal/extensionserver/post_translate_modify.go`:

```go
const httpProtocolOptions = "envoy.extensions.upstreams.http.v3.HttpProtocolOptions"
...
po.HttpFilters = append(po.HttpFilters, upstreamCodec)
cluster.TypedExtensionProtocolOptions[httpProtocolOptions] = poAny
```

`HttpProtocolOptions.http_filters` carries an upstream HTTP filter chain **on an individual
cluster**. This spec previously asserted no such placement existed — that only the Router's
listener-wide `upstream_http_filters` was available, and that per-route disabling via
`ExtProcPerRoute` was the only way to avoid taxing every request. That assertion was wrong.

**Consequence:** FR-043's cost concern dissolves. Only clusters with per-attempt work carry the
filter; everything else pays nothing, with no gating mechanism to build and no opt-in flag needed
to protect unrelated traffic. The cluster *is* the provider here, which makes per-cluster
placement a better fit than per-route would have been.

### A2 — The response phase belongs DOWNSTREAM, not at the upstream position

```go
// Response will be handled at the router filter level so that we could avoid the
// shenanigans around the retry+the upstream filter.
ResponseHeaderMode: extprocv3.ProcessingMode_SKIP,
```

They run no response processing at the upstream position at all, deliberately, to avoid the
retry/upstream-filter interaction.

We hit exactly that interaction: running the response half upstream required per-stream state
(`upstreamAttempt`) to carry the resolved chain from the request phase to the response phase,
because the response message carries no cluster attribute to re-resolve from.

**Consequence:** the per-attempt chain becomes request-only. Response work — including a
provider's response translation — happens once, downstream, which already knows which target
answered via the attempt count (FR-030f–FR-030j). Less state, no retry interaction, and the
reversed-order unwrapping is the downstream executor's existing behaviour.

### A3 — The post-downstream baseline lives in SERVER-SIDE per-request state

```go
// originalRequestBody is the original request body that is passed to the upstream filter.
originalRequestBody    *ReqT
originalRequestBodyRaw []byte
```

```go
// At the upstream filter, it can access the original body in its memory, so it can perform the
// translation as well as the authentication at the request headers. Hence, there's no need to
// send the request body to the extproc.
RequestBodyMode: extprocv3.ProcessingMode_NONE,
```

The ext_proc server holds the post-downstream request per request and the upstream filter never
re-receives it. Translation happens at the request-*headers* phase against that memory.

Our implementation captures the baseline on attempt 1 and round-trips it through dynamic
metadata, which rests on an assumption never measured: that an **upstream** ext_proc's metadata
write survives into the next attempt. Probe E showed metadata flowing *to* the filter, not back
out and forward again.

**Consequence:** adopt server-side state keyed by request, and the assumption disappears rather
than needing a probe. It also removes a metadata round trip per attempt.

---

## Equivalent — no change

### E1 — Target identity

They stamp the backend name into **upstream host metadata**
(`setEndpointMetadataBackendName`) and read it via `XDSUpstreamHostMetadataBackendNamePath`. We
read `xds.cluster_name`, confirmed per-attempt by Probe E.

Both are Envoy-native, both are correct per attempt, neither is inferred. Theirs is forced by
their topology (all providers are hosts of one cluster, so the cluster name cannot distinguish
them); ours is forced by ours (one cluster per provider). Keep `xds.cluster_name`.

---

## NOT adopted — the core mechanism

Agent Router fails over with **endpoint priority levels** inside a single cluster:

```go
endpoints.Priority = *backendRef.Priority
```

driven by a `BackendTrafficPolicy` retry with `numAttemptsPerPriority: 1` — Envoy Gateway's
surface for the `previous_priorities` retry predicate, so each retry excludes already-tried
priorities.

It is simpler than ours: no cluster-specifier plugin, no published-groups contract, no
group-name agreement across repositories. If static ordering were acceptable it would be the
better choice.

**It cannot satisfy FR-021.** Priorities live in the cluster's load assignment, pushed over xDS,
with no per-request override — there is no metadata hook analogous to
`override_metadata_namespace`. A routing policy's per-request ranking cannot change a priority
level. The same requirement that ruled out composite clusters rules this out.

Two incidental observations worth recording:

- **Their topology avoids the panic-mode problem Probe F found.** Panic mode is evaluated per
  cluster; with every provider a host of one cluster, ejecting the primary still leaves healthy
  hosts, so outlier detection remains usable for them. Ours hit panic mode because each provider
  is its own single-host cluster. This is a property of topology, not of outlier detection.
- **They share our attempt-budget limitation.** `numRetries` is static deploy-time policy with no
  per-request sizing — the problem FR-059a solves with `x-envoy-max-retries`.
