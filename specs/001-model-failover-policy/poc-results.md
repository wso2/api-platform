# Stage 0 POC results — Option A vs Option B

**Run**: 2026-10-23 · **Harness**: `gateway/it/spikes/failover/` · **Branch**: `poc/failover-envoy-probes`

Three fake providers on fixed IPs (`172.28.0.11/.12/.13`) that echo what they received and can be
forced to fail, plus one Envoy. Option A probes ran on **envoyproxy/envoy:v1.39.0** (what we ship
today); Option B probes on **envoyproxy/envoy:v1.40.0**.

Every result below was observed, not inferred.

---

## Scorecard

| # | Question | Option A (Override Host, 1.39.0) | Option B (priority_group, 1.40.0) |
|---|---|---|---|
| 1 | Per-request order honored on attempt 1 | **PASS** | **PASS — only with route-cache clear** |
| 2 | Ordered traversal across retries | **PASS** | **PASS** |
| 3 | Each target attempted at most once | **PASS** (1 hit per provider) | **PASS** |
| 4 | Per-provider clusters kept intact | **PASS** (via internal listeners) | **PASS** (native) |
| 5 | Body mutations survive to the fallback attempt | **PASS** | **PASS** |
| 6 | Excluded target stays untouched when attempts overrun | **FAIL** | **PASS** |
| 7 | Config accepted without custom builds | **PASS** | **PASS** |
| 8 | Silent-failure modes found | 1 (overrun) | 2 (missing flag, missing cache clear) |

---

## What each probe showed

### A1 — ordering (PASS)

List `[c,a,b]` → served by **c**. `[b,c]` → **b**. `[a]` → **a**. With no list at all, the fallback
round-robin served normally, so a policy that writes nothing degrades to ordinary routing rather than
breaking.

### A2 — traversal (PASS)

Providers a and c forced to 500, list `[a,c,b]`:

```
HTTP=200, served by b
provider-a hits: 1   provider-b hits: 1   provider-c hits: 1
```

Exactly one attempt per target, in order, client never saw the failures.

### A3 — overrun (**FAIL — the important one**)

List `[a,c]` (two entries, both failing) while the retry policy allowed three attempts. Provider **b
was not in the list at all**:

```
HTTP=200, served by b
provider-a: 1   provider-b: 1   provider-c: 1
```

**A target the policy deliberately excluded received traffic and served the request.** In production
that excluded target is a *suspended* one — so with Option A, suspension is silently overridden
whenever `num_retries` exceeds the list length. Omitting a target from the list is not sufficient to
keep traffic away from it.

### A4 — internal listener addressing (PASS, with an asterisk)

This was Option A's make-or-break question. Trace logging revealed the host-map keys:

```
Looking up BOGUS-PROBE-VALUE in envoy://int-provider-b/, envoy://int-provider-a/
```

Using those strings in the override list works — ordering honored, traversal correct, each provider
reached through its own cluster behind its own internal listener:

```
order=[a,b] -> a     order=[b,a] -> b     order=[a only] -> a     order=[b only] -> b
```

**Asterisk**: the documentation promises `IP:Port`. The `envoy://<listener>/` form works because the
lookup is a plain string match against the host's address string. It is undocumented behaviour that
could change in any release.

*(An earlier reading suggested ordering was unreliable here; that was a stale connection to a
container I had recreated mid-probe — a harness artifact, not Envoy. The clean re-run is above.)*

### B1 — attempt-indexed cluster walking (PASS)

Configured groups rank-1=a, rank-2=b, rank-3=c with a and b failing → served by **c**. Three separate
clusters, each keeping its own configuration, walked by attempt number.

### B2 — per-request override (PASS, conditional)

A Lua filter standing in for our policy wrote the order into dynamic metadata. The access log confirms
the metadata arrived intact:

```
md={"priority_groups":[{"clusters":[{"cluster_name":"provider_c",...}],"name":"rank-1"}, ...]}
```

**But the first attempt ignored it** — `cluster=provider_a attempts=1` when the override said `c`
first. The route, and therefore the cluster, had already been resolved before the metadata was
written.

Adding `clearRouteCache()` after writing the metadata fixed it completely:

```
override=[c,b,a] -> provider_c    override=[b,a,c] -> provider_b    override=[a,c,b] -> provider_a
```

**Relevance to us**: our policy-engine kernel already sets `ClearRouteCache: true` on its responses,
so this is satisfied by existing behaviour — but it must be asserted by a test, because without it
the first pick silently falls back to the configured order.

### B3 — missing `refresh_cluster_on_retry` (**fails loudly in the worst way**)

With the flag set to false, override `[a,b,c]`, a failing and b healthy:

```
HTTP=500 returned to the client
cluster=provider_a attempts=3
provider-a hits: 3    provider-b hits: 0
```

Not merely a no-op: Envoy hammered the **same failing provider three times** and returned the error,
while the healthy next target received nothing. Worse than no failover — it triples load on a backend
that is already failing.

### B4 — overrun (PASS, and the key contrast with A3)

Override `[a,c]` (both failing), three attempts allowed, provider b excluded:

```
HTTP=500, cluster=provider_c attempts=3
provider-a: 1    provider-b: 0    provider-c: 2
```

The surplus attempt **repeated the last group** instead of reaching for an excluded target. Provider b
— the stand-in for a suspended provider — was never touched. This is the behaviour A3 failed.

### C — body-mutation preservation (both PASS)

A Lua filter masked `SECRET-12345` → `[MASKED]` in the request body, then attempt 1 was forced to fail.

| | Attempt 1 saw | Fallback attempt saw |
|---|---|---|
| Option A | `{"prompt":"my key is [MASKED]"}` | `{"prompt":"my key is [MASKED]"}` |
| Option B | `{"prompt":"my key is [MASKED]"}` | `{"prompt":"my key is [MASKED]"}` |

Neither leaked the unmasked body to the second provider. The defect class seen in a comparable product
did not reproduce here with filter-based mutation.

---

## What this changes

**Option A is viable** — its decisive unknown (internal-listener addressing) works. But probe A3 found
a safety gap that is not merely theoretical: an excluded, suspended target can receive live traffic
whenever the attempt budget exceeds the list length. Correct `num_retries` is the *only* thing
preventing it, and nothing in the proxy enforces the relationship.

**Option B is viable and safer on the one axis that matters most.** Its overrun behaviour keeps
excluded targets untouched, and it needs no internal listeners, no undocumented address form, and no
endpoint flattening. It carries two silent failure modes instead of one — but both are
configuration-generation concerns we control and can test, rather than runtime behaviour we must work
around.

## Recommendation

**Option B**, if the Envoy 1.40.0 upgrade is acceptable.

The deciding evidence is A3 versus B4. Both options deliver failover; only B keeps traffic away from a
target the policy excluded. Since suspension is the mechanism protecting a degraded provider from more
load, an option that can silently route to a suspended target is carrying a real operational hazard.

If the upgrade is not acceptable now, Option A can ship — but `num_retries` alignment must be enforced
at deployment time and covered by a test that fails loudly, because it is the single control standing
between a suspended provider and live traffic.

## Not yet proven

- The full IT suite has not been run against Envoy 1.40.0 — this remains Option B's real risk and is
  independent of the mechanism (branch `chore/envoy-1.40-trial`).
- Mutation here came from a Lua filter; our real chain mutates via ext_proc. The defect class in the
  comparable product involved an ext_proc upstream filter rebuilding the body from a snapshot. Probe C
  should be repeated against the real policy engine before FR-012a is considered settled.
- No controller code and no real policy were built. These probes test the Envoy mechanisms only.

---

## Probe D — per-attempt upstream filter (2026-10-26)

Run after the first end-to-end IT run showed the control plane working while the
feature still failed. Configs: `gateway/it/spikes/failover/d1-upstream-filter.yaml`,
`d2-attempt-indexed-rewrite.yaml`. Envoy v1.40.0, provider_a always fails,
provider_b healthy.

### Why this probe was needed

The IT run proved Envoy switches clusters correctly on retry. It also proved that
does not help: every LlmProxy provider cluster is a loopback to the same socket
(`127.0.0.1:8080`), and the thing that actually selects a provider is the route's
`regex_rewrite` substitution — fixed per route, identical on every attempt. Both
attempts re-entered the gateway on the primary provider's context.

So the open question was whether a filter at the upstream position can re-point
the request per attempt. If not, T021–T023 needed a different approach entirely.

### D1 — does the upstream chain run per attempt, and can it mutate `:path`?

**Both yes.** One client request produced:

| Position | Invocations |
|---|---|
| downstream HTTP filter | 1 |
| upstream HTTP filter | 2 (one per attempt) |

and the backends confirmed each attempt carried a *different* path.

Two constraints surfaced that shape the implementation:

1. **Mutations accumulate across attempts.** The upstream filter does not receive
   a pristine copy of the request; it sees whatever the previous attempt left.
   A naive prefix produced `/ctx-attempt-2/ctx-attempt-1/chat/completions`. The
   rewrite MUST be computed from an immutable source, never from the current
   `:path`.
2. **`StreamInfo` at the upstream position exposes neither `upstreamClusterName()`
   nor `upstreamHost()`** (both are nil in Lua). The filter cannot ask which
   cluster was selected, so it must derive the target from the attempt *number*
   plus the chain the downstream filter already published.

### D2 — the resulting pattern, validated

Downstream filter stashes `original_path` and the ordered chain in dynamic
metadata; upstream filter reads `x-envoy-attempt-count`, indexes the chain, and
replaces `:path` built from the stashed original:

```
PROBE downstream published chain, original_path=/chat/completions   <- once
PROBE upstream attempt=1 -> /ctx-a/chat/completions                 <- provider_a, failed
PROBE upstream attempt=2 -> /ctx-b/chat/completions                 <- provider_b, 200
ACCESSLOG cluster=provider_b attempts=2 status=200
```

Client received 200. Each attempt reached a different provider context. This is
the design T021–T023 should implement.

### Incidental findings

- The virtual-host field is `include_request_attempt_count` (not
  `include_attempt_count_in_request`); it is what makes `x-envoy-attempt-count`
  visible to the upstream filter. Without it the filter cannot tell which attempt
  it is serving.
- Envoy 1.40.0 logs its supported upstream filters at startup. The list includes
  **both `envoy.filters.http.lua` and `envoy.filters.http.ext_proc`**, plus
  `header_mutation`, `credential_injector`, `buffer`, `upstream_rbac` and `wasm`.
  So the light (Lua/header_mutation) and heavy (ext_proc policy chain) options
  are both available — the choice is about scope, not feasibility.
- `credential_injector` being available at the upstream position is notable: it
  suggests per-attempt provider credentials may not require the full ext_proc
  chain either.

---

## Probe E — ext_proc at the upstream position (2026-10-26)

Run after the decision that the upstream position must host the general policy
engine: the platform is introducing upstream and downstream policy *types*, so an
author can target the upstream path directly. Failover is the first consumer of
that mechanism, not a special case, and a bespoke Lua filter would be thrown away.

Config: `gateway/it/spikes/failover/e1-upstream-extproc.yaml`; throwaway gRPC
server in `gateway/it/spikes/failover/extproc/`. Envoy v1.40.0.

### E1 — is ext_proc invoked once per retry attempt? **Yes**

Probe D proved the *position* runs per attempt using Lua; ext_proc is a different
filter with a gRPC round trip, so it needed its own confirmation.

```
PROBE call attempt=1 path=/chat/completions
PROBE call attempt=2 path=/ctx-a/chat/completions
```

### E2 — do request_attributes carry per-attempt target identity? **Yes** — and this deletes the Probe D workaround

| Attempt | `xds.cluster_name` | `upstream.address` |
|---|---|---|
| 1 | `provider_a` | `172.28.0.11:8080` |
| 2 | `provider_b` | `172.28.0.12:8080` |

Both are correct **per attempt**. This is strictly better than Probe D's
constraint 2, where Lua exposed neither `upstreamClusterName()` nor
`upstreamHost()` and the target had to be inferred by indexing the published
chain with `x-envoy-attempt-count`. With ext_proc:

- the attempt-index workaround can be **deleted**;
- every upstream policy gets real target identity for free, rather than each one
  reinventing an inference;
- a policy no longer has to know the chain to know which target it is serving,
  which keeps per-attempt policies independent of failover.

Note `xds.route_name` came back empty. Attributes are namespaced on arrival as
`envoy.filters.http.ext_proc.<attribute>`.

### E3 — what does the round trip cost? ~1ms per attempt

| Call | Server-side handling |
|---|---|
| first (cold gRPC stream) | 17.1 ms |
| warm | 0.60 ms |
| warm (retry attempt) | 1.06 ms |

Envoy closes the gRPC stream after the headers response when every other
processing mode is `SKIP`, so each attempt opens a fresh stream. Sub-millisecond
warm, but this now sits in the path of every request carrying an upstream policy,
not just retries — worth a load test before it ships (T044-series).

### Mutation accumulation re-confirmed at the ext_proc layer

Attempt 2 arrived with `:path` and the `request.path` attribute both set to
`/ctx-a/chat/completions` — attempt 1's rewrite, not the client's original. The
probe server was unaffected because it rebuilds the path from a fixed source, but
this re-proves Probe D's constraint 1 for ext_proc specifically. **Every upstream
policy sees the previous attempt's mutations.** The kernel must hand each attempt
a pristine request or document the re-entrancy contract explicitly.

### `mutation_rules.allow_all_routing` is mandatory

ext_proc refuses to mutate `:path`, `:method`, `:authority`, `:scheme` or `host`
unless `mutation_rules: { allow_all_routing: true }` is set — the mutation is
dropped silently otherwise, with no error. Any upstream policy that re-points a
request needs it, so this is a platform-level configuration decision rather than
a failover one, and it widens what *every* upstream policy may rewrite.

### Not a finding: cold-start 504

The first request against a freshly created stack returned 504 twice. It did not
reproduce: three consecutive Envoy restarts gave 9/9 × 200. The cause was the
Python provider containers still starting, not ext_proc.

---

## Probe F — outlier detection under priority_group (2026-10-26)

**Result: NEGATIVE. The design document's rejection of outlier detection holds for
`priority_group` too. Do not use it.**

Config: `gateway/it/spikes/failover/f1-outlier-detection.yaml`. Envoy v1.40.0. `provider_dead`
points at a live container on a closed port (immediate `ECONNREFUSED`); `provider_b` is healthy;
groups are `[dead, b]` with `refresh_cluster_on_retry`.

### The hypothesis, and why it was wrong

C3 asked whether Envoy could suspend transport-dead members itself, since ext_proc is never
invoked for Envoy's own local replies and the policy therefore cannot see a connection failure.
The design document had rejected outlier detection — *"Disproven live: the composite still sends
attempt 1 to an ejected leaf → 503 'no healthy upstream', not retryable (envoy#11307)"* — but that
was the **composite** cluster, and the IT run had shown a zero-host cluster under `priority_group`
return a 503 that *was* retried. That observation came from an inner loopback hop rather than an
ejected cluster, which is why it needed its own probe. The hypothesis did not survive it.

### What was measured

Ejection itself works. `consecutive_local_origin_failure` with
`split_external_local_origin_errors: true` detects connection failures, and the host is marked:

```
cluster.provider_dead.outlier_detection.ejections_active: 1
cluster.provider_dead.outlier_detection.ejections_detected_consecutive_local_origin_failure: 1
provider_dead::172.28.0.13:9999::health_flags::/failed_outlier_check
```

What it buys depends entirely on panic mode, and neither setting is usable:

| `healthy_panic_threshold` | Dead host dialled? | Client result | Verdict |
|---|---|---|---|
| default (50%) | **Yes** — `upstream_cx_connect_fail` climbed 9 → 12 over 3 requests, `lb_healthy_panic` incrementing | 200, 2 attempts | Failover works, ejection is **cosmetic** — every request still pays the dead dial |
| `0` | No — dial count flat at 2 across 3 requests | **503, 1 attempt** | Dead host avoided, but **failover is broken**: the local reply is not retried and the chain never reaches group 2 |

A provider cluster normally has **one** host. Ejecting it leaves 0% healthy, which is below the
default 50% panic threshold, so Envoy enters panic mode and routes to all hosts regardless of
health — that is why ejection changes nothing by default. Disabling panic mode makes the ejection
real and immediately reproduces the document's finding: `503 no healthy upstream`, not retryable,
`attempts=1`.

### Consequence

Disabling panic mode would turn a *working* failover into a hard client-facing failure, so it is
strictly worse than leaving it on, and leaving it on makes outlier detection pointless here. **The
document's architecture limit stands.** T024i2 is closed negative; proceed to T024i3 (feed
transport failures to the health store over the existing access-log pipeline) or accept the
documented limit that transport failures fail over but never suspend.

The attempt-count inference (T024i1) is unaffected and still covers the common case: a
transport-dead member followed by a healthy one is recorded as failed, because the policy does run
on the later real response.

---

## Probe G — an unconfigured group name (2026-10-26)

**Result: the routing-policy ranking contract has a hole. A ranked target that model-failover does
not itself configure has no group to resolve to, and Envoy's fallback for that is actively harmful
rather than merely ineffective.**

Config: `gateway/it/spikes/failover/g1-unknown-group.yaml`. Route configures `rank-1 → provider_a`
(always fails) and `rank-2 → provider_b` (healthy); `num_retries: 2`. The Lua stand-in publishes
group **names** verbatim, exactly as the real policy does.

### Why this was probed

`resolveChain` puts a routing policy's published ranking FIRST, ahead of the configured chain —
that is the "obey the ordering decided by other policies" requirement. But the controller builds
the configured groups from the model-failover policy's own `targets` only
(`flattenChainMembers(params["targets"])`), and the policy publishes **names**, which Envoy
resolves against those configured groups. So a routing policy can rank a `(model, provider)` pair
that has no group.

### Measured

| Published order | Attempts | Served by | Reading |
|---|---|---|---|
| `rank-2, rank-1` | 1 | provider_b | Reorder honoured |
| `rank-1, rank-2` | 2 | provider_b | Configured order, a fails then b |
| `rank-2, t:unknown:x` | 1 | provider_b | Known name at position 1 is honoured |
| `t:unknown:x, rank-2` | 2 | provider_b | Unknown name consumed position 1 |
| `t:unknown:x, t:p:q` | 2 | provider_b | Both unknown |
| **`t:unknown:x, rank-1`** | **3** | **provider_a** | **Burned the whole budget on the failing provider** |

An unknown name is **not skipped**. It falls back to the **configured group at that position**. The
last row is the damaging case: the ranking asked for an unknown target then `rank-1`, and the
request made three attempts entirely against the failing `provider_a`, never reaching the healthy
`provider_b` at all.

### Consequence

A routing policy that ranks anything outside model-failover's own `targets` does not merely get
ignored — it can steer the request to a target its ranking never nominated and exhaust the attempt
budget there. Nothing fails loudly; the client may still get a 200 from a later attempt, so the
discarded ranking leaves no signal.

### Fix

The controller must generate a priority group for **every `(model, provider)` reachable on the
operation**, not only those listed in model-failover's own parameters — which means flattening the
attached routing policy's target list (`model-round-robin` / `model-weighted-round-robin`
`models[]`, and any future ranking policy) into the same group set. Both lists are known at deploy
time, so this is control-plane work, not a runtime lookup. Tracked as T024v.

---

## Probe H — per-request attempt budget via `x-envoy-max-retries` (2026-10-27)

**Result: POSITIVE on all three questions. The downstream policy can size the attempt budget to the
ranking it just published, and a client cannot inflate its own budget.**

Config: `gateway/it/spikes/failover/h1-max-retries.yaml`. Envoy v1.40.0. Three groups —
`rank-1 → provider_a` (fails), `rank-2 → provider_c` (fails), `rank-3 → provider_b` (healthy) —
with `num_retries: 1` in the **route** config, so only 2 attempts are configured and the healthy
third group is unreachable. Reaching provider_b is therefore unambiguous proof the budget grew.

### Why this was probed

Dynamic metadata overrides the cluster specifier's groups but has no hook for retry count, so a
policy could reorder a chain freely and lengthen it not at all. `num_retries` is generated from
`ChainLength() - 1` — the publisher's own configured members — which silently truncates any longer
ranking a routing policy produces.

### Measured

| Test | `x-envoy-max-retries` set by | Attempts | Status | Served by |
|---|---|---|---|---|
| baseline | nobody | 2 | 500 | provider_c |
| client trust | **the client** (curl) | **2** | 500 | provider_c |
| policy raise | **the Lua policy stand-in** | **3** | 200 | provider_b |
| overrun | policy, value 10 | 3 | 200 | provider_b |

- **(a) Honoured with a route-config retry policy** — yes. The budget changed without the retry
  policy living in the router filter.
- **(b) Can RAISE above the configured `num_retries`** — yes, 2 → 3. The previously unreachable
  third group served the request.
- **(c) Stripped from client requests** — yes. The client-set header had no effect at all, and the
  filter chain observed `x-envoy-max-retries=nil`: Envoy sanitized it before any filter ran. A
  caller cannot inflate their own retry budget and multiply inference spend.

The value 10 against a 3-group list still produced 3 attempts, consistent with Probe B4 — overrun
repeats the last group rather than reaching for an unranked cluster.

### Consequence

T024w4's deploy-time worst-case counting is unnecessary. The downstream policy should set
`x-envoy-max-retries` to the length of the ranking it publishes, in the same pass. Order and length
then both become per-request, which is what FR-021 asks for.

### One caveat to carry into the real config

(c) holds because Envoy classifies the downstream as **external**, and sanitization of `x-envoy-*`
applies to external requests only. A downstream classified as *internal* — via
`internal_address_config` on the HCM — would NOT have the header stripped. The generated gateway
config sets no `internal_address_config` today, so the default applies and clients are external.
That is a property worth asserting in a test rather than assuming, because the day someone adds an
internal-address range for an unrelated reason, this becomes a client-controllable spend
multiplier with no other symptom.

---

## Runtime finding — ext_proc refuses `x-envoy-*` mutations (2026-10-27)

First end-to-end run of the per-attempt dispatch returned **500 on every failover request**:

```
[error][ext_proc] mutation_utils.cc:247
Header x-envoy-max-retries may not be modified. Returning error
```

### Why Probe H did not catch it

Probe H established that Envoy honours `x-envoy-max-retries`, that it can raise the budget above
the configured `num_retries`, and that a client cannot set it. All three still hold. What it did
not establish is whether **this gateway's policy path** is allowed to set it — because the probe
used a Lua filter, and Lua is not subject to ext_proc's `HeaderMutationRules` at all.

ext_proc refuses `x-envoy-*` mutations unless `allow_envoy` is set, and this deployment also sets
`disallow_is_error: true`, which turns that refusal into a failed request rather than a dropped
mutation. So the policy's attempt-budget header failed the whole request.

### Fix

`allow_expression: "^x-envoy-max-retries$"` on the downstream processor — exactly one header.

`allow_envoy: true` would have been the one-line fix and is the wrong one: it lets **any**
downstream policy rewrite **every** `x-envoy-*` header, including
`x-envoy-upstream-rq-timeout-ms` and the retry-policy overrides. A policy bug there is a
per-request denial of service against the upstream, reachable by any policy author.

### Lesson for the remaining probes

A probe that stands in for a policy with Lua proves what **Envoy** does, not what the **policy
engine is permitted** to do. The two differ wherever ext_proc applies its own rules — header
mutation being the obvious one. Worth re-reading the earlier probes with that distinction in
mind before relying on them.

---

## Runtime verification — the per-attempt position works (2026-10-27)

After fixing the `x-envoy-max-retries` mutation rule above, the engine's own logs confirm the
whole per-attempt path end to end:

```
running per-attempt policy chain    cluster=upstream_LlmProxy_<uuid>_alive-provider attempt=1 policies=1
running per-attempt response chain  cluster=upstream_LlmProxy_<uuid>_alive-provider attempt=1 status=503
running per-attempt policy chain    cluster=upstream_LlmProxy_<uuid>_alive-provider attempt=2 policies=1
```

Each line is a separate thing working:

| Observed | Confirms |
|---|---|
| the handler ran at all | the router generated `upstream_http_filters`, and the engine recognised the stream as upstream from the gRPC position metadata — not inferred |
| `cluster=upstream_LlmProxy_..._alive-provider` | `xds.cluster_name` reaches the engine per attempt, as Probe E predicted |
| `policies=1` | the chain published under `upstream:<cluster>` was found — the cross-module key contract holds |
| both "policy chain" AND "response chain" | the response half runs, so a provider's transformer will be able to convert its response back |
| `attempt=1` then `attempt=2` | `include_request_attempt_count` works and the attempt number is readable |

So T021, T024d and T024e are verified against a running gateway, not just unit-tested.

### What is still not working, and why

The request itself returns 503 `no healthy upstream`. That is the **original** gap, unchanged:
the cluster switches per attempt but the route's `regex_rewrite` still points every attempt at
the PRIMARY provider's context, so the loopback lands on the dead provider either way.

The per-attempt chain now exists and runs — what it does not yet contain is anything that
rewrites the provider context path. That is the transformer/auth migration (the builders are
still stubs returning nil), plus a context-path rewrite for the selected provider. Until those
land, a per-attempt policy can stamp headers but cannot re-point the attempt.

---

## END TO END — cross-provider failover works (2026-10-27)

First green rescue, against a running gateway:

```
HTTP/1.1 200 OK
x-envoy-attempt-count: 2
path:  /alive-upstream/chat/completions
X-Attempt-Provider: ["alive"]
```

Four things proven by one response:

| Evidence | Proves |
|---|---|
| `200 OK` | the client receives a success it never had to retry for (FR-008, FR-015) |
| `attempt-count: 2` | attempt 1 failed and Envoy retried within the same client request |
| `path: /alive-upstream/...` | the provider re-point worked — attempt 2 reached the FALLBACK's own context, not the primary's (T024x) |
| `X-Attempt-Provider: alive` | the fallback's OWN per-attempt policy ran, and only on the attempt routed to it (FR-035, FR-041) |

The last row is the one that cannot be faked: the primary provider carries no policies at all, so
that header could only come from the fallback's chain executing on attempt 2.

Envoy's access log shows the full shape:

```
attempt 1: POST /pa-dead-ctx/chat/completions  -> /dead/chat/completions   -> 503 UH
attempt 2: POST /pa-alive-ctx/chat/completions -> /alive-upstream/chat/... -> 200
```

### A diagnostic note worth keeping

The first run on this build still returned 503 and I went looking for a bug in the re-point. The
access log showed it had been working all along — attempt 2 was already landing on
`/pa-alive-ctx`. The actual failure was `SERVFAIL` resolving `sample-backend` from the runtime
container, caused by repeated `docker compose up -d --force-recreate` cycles leaving Docker's DNS
inconsistent. A full `down -v` then `up` fixed it.

Checking the access log before changing code is what avoided reporting a feature bug that did not
exist. Repeated partial recreation of the IT stack is unreliable; prefer a clean cycle.

---

## Measurement — cost of re-sending the body per attempt (2026-10-27)

The per-attempt chain asks Envoy for a BUFFERED body whenever a provider has a transformer, so a
request that fails over carries its payload to the engine once per attempt instead of once. The
alternative — holding the post-downstream body in engine memory and replacing it from the headers
phase — trades bandwidth for resident memory, so it is only worth building if the bandwidth costs
something. Measured rather than assumed.

Benchmark: `gateway/it/spikes/failover/extproc/bodycost_test.go`. One ext_proc round trip over
loopback gRPC carrying a body of size N, with the response also carrying a body (a transformer
rewrites the payload, so a request-only measurement would understate it). 2000 iterations, two
runs.

| Body | Round trip | **Marginal vs headers-only** | Allocations |
|---|---|---|---|
| none (headers only) | 88–92 µs | baseline | 10 KB |
| 1 KB | 91 µs | **~0 µs** | 13 KB |
| 10 KB | 103–107 µs | **~15 µs** | 33 KB |
| 100 KB | 259 µs | **~170 µs** | 281 KB |
| 1 MB | 1.20–1.27 ms | **~1.14 ms** | 2.8 MB |

### SUPERSEDED by the per-request measurement below

The figures above are for ONE round trip. A real request makes two attempts, and the response
carries the body as well (a transformer returns `CONTINUE_AND_REPLACE`), so the per-request cost
is roughly four times the marginal figure in that table. The conclusion drawn from it was
correspondingly too optimistic; see "Measurement — wire vs resident" below.

### Original conclusion (from single round trips)

Even a 1 MB prompt adds **~1.1 ms per extra attempt**, against an LLM call measured in hundreds of
milliseconds to tens of seconds. For a typical chat request (10 KB) the cost is ~15 µs — about
0.003% of a 500 ms completion.

The striking result is the **~90 µs floor**: per-attempt stream setup costs more than the body
does for anything under roughly 50 KB. Optimising the body while leaving a fresh gRPC stream per
attempt would be optimising the smaller term.

### The number that might matter is allocation, not latency

A 100 KB body allocates ~280 KB per round trip, a 1 MB body ~2.8 MB — roughly 3x the payload.
At high concurrency that is real GC pressure: 1000 rps of 100 KB failover attempts is ~280 MB/s
of allocation.

That is the case worth revisiting, and it is an argument for the in-memory approach only if
holding bodies resident proves cheaper than allocating them transiently — which is not obvious
and would need its own measurement under load.

### Caveats

Loopback TCP with a trivial echo server. Production uses UDS (likely faster) and a real engine
(slower, but that cost is not the body's). The numbers bound the *marginal* cost of one extra
body send, which is the decision at hand, and nothing more.

---

## Measurement — wire vs resident body (2026-10-27)

Both sides of the trade, measured per REQUEST rather than per round trip.
Benchmark: `gateway/it/spikes/failover/extproc/residentcost_test.go`. Two attempts per request
(primary fails, fallback answers), body carried in both directions, 1000 iterations.

| Prompt | wire | memory | saved | wire alloc/req | memory alloc/req |
|---|---|---|---|---|---|
| 10 KB | 234 µs | 186 µs | 48 µs (20%) | 67 KB | 21 KB |
| 100 KB | 573 µs | 179 µs | **394 µs (69%)** | 482 KB | 21 KB |
| 1 MB | 2.72 ms | 167 µs | **2.55 ms (94%)** | 4.5 MB | 21 KB |

The memory approach is flat in body size — nothing crosses the wire, so a 1 MB prompt costs the
same as a 10 KB one.

What it costs instead, measured by holding N bodies and reading heap:

| In-flight x size | Resident |
|---|---|
| 1000 x 10 KB | 10 MB |
| 1000 x 100 KB | 101 MB |
| 5000 x 100 KB | 508 MB |

### Correction to the earlier conclusion

The previous section measured a single round trip and reported ~15 µs for a 10 KB body, calling
the optimisation unnecessary. A real request is ~4x that: two attempts, each carrying the body in
both directions. The per-request figure is 48 µs at 10 KB, and the gap widens sharply with size.

The allocation difference is the larger finding: **23x less at 100 KB** (482 KB -> 21 KB per
request). That is the GC-pressure concern from the earlier section, now quantified rather than
speculated about.

### Decision

Still not building it now, but on a stated threshold rather than a hunch:

- **Prompts <= 10 KB**: 48 µs and 46 KB per failover. Not worth the resident memory or the code.
- **Prompts >= 100 KB with a material failover rate**: 394 µs and 461 KB saved per request, for
  ~100 MB resident per 1000 concurrent in-flight. Worth taking.

If built, it needs a **size ceiling**: hold bodies below a threshold, fall back to the wire above
it. That bounds resident memory while capturing most of the benefit, since small bodies are
exactly where holding is cheapest and large ones are where resident cost bites.

---

## Probe I — can the upstream position run a BODY phase? (2026-10-27)

**Why.** The per-attempt chain was running its header phase (the credential reached the fallback
provider) but its body phase never ran, so a transformer never rewrote the payload and the
fallback received the request in the primary's format. Before hunting a code bug, the capability
itself had to be established: Agent Router deliberately keeps per-attempt bodies in filter memory
and configures `request_body_mode: NONE`, which is consistent with the upstream position not
supporting a body phase at all.

**Configs.** `gateway/it/spikes/failover/i1-upstream-body.yaml` (static `BUFFERED`),
`i2-mode-override.yaml` (`NONE` + server-sent `mode_override`), `i3-per-cluster.yaml` (same, but
the filter attached per-cluster via `HttpProtocolOptions.http_filters` rather than router-wide).

| Question | Result |
|---|---|
| I1. Does the upstream position deliver a request body with static `request_body_mode: BUFFERED`? | **YES** — 65 bytes received, on every attempt |
| I2. Does a server-sent `mode_override` turn the body phase on at the upstream position? | **YES** — honoured per attempt, with `allow_mode_override: true` |
| I3. Does it still work when the filter is attached **per-cluster** rather than router-wide? | **YES** — identical behaviour |

**Conclusion: the mechanism was never the problem.** Envoy supports a per-attempt body phase at
the upstream position, in both filter placements, driven by `mode_override`. Holding bodies in
filter memory is therefore an optimisation (see the body-cost measurements above), not a
requirement — the per-attempt design stands as specified.

### The actual defect this probe isolated

Envoy attaches `request_attributes` (and the metadata context) to the **first message of an
ext_proc stream only**. The headers message carries `xds.cluster_name`; the `RequestBody` message
that follows carries nothing. The body handler was deriving the cluster from the request it was
handed, so on the body message it resolved to `""`, found no chain, and returned a passthrough —
silently skipping the transformer.

This is invisible in the ordinary (downstream) position, where the chain is keyed by route rather
than by cluster, so no existing code exercised it.

**Fix.** `upstreamAttemptState` (`internal/kernel/upstream_position.go`) carries the cluster and
metadata resolved by an attempt's headers phase into that same attempt's body phase. An upstream
stream serves exactly one attempt — Envoy opens a fresh one per retry — so one value per stream
needs no keying and no eviction.

**Standing constraint for any future upstream-position work:** anything derived from
`request_attributes` must be resolved during the headers phase and carried forward. Later messages
on the same stream cannot re-derive it.

### Two further defects the same runtime test surfaced

Both sit behind the first one — each was only reachable once the body phase actually ran.

1. **The body arrived flagged absent.** `policy.Body.Present` was left at its zero value. Every
   body-rewriting policy in this repo opens with `if Body == nil || !Body.Present ||
   len(Content) == 0 { nothing to do }`, so the chain ran, the transformer returned no
   modification, and the fallback provider received the primary's payload format — no error
   anywhere. Fixed by `upstreamBodyContext`, which states the truth for a BUFFERED body: present
   and complete (FR-037c).

2. **The body context had no headers, which panicked the engine.** The executor writes a policy's
   `HeadersToSet` straight into the map behind `ctx.Headers`; with no headers that map is nil, so
   the first per-attempt policy to set a header panicked with `assignment to entry in nil map`,
   killing the ext_proc stream and returning a 500
   (`ext_proc_error_gRPC_error_13{upstream_reset_after_response_started}`). The body phase now
   starts from the attempt's post-headers-phase header set, copied across the boundary, and emits
   whatever it changes back to Envoy (FR-037d).

The common shape across all three: **a per-attempt phase that silently does nothing looks exactly
like a per-attempt phase that has nothing to do.** The header phase succeeding (the credential was
correct on the fallback) made it look like the feature worked.

---

## The published ranking was never reaching Envoy (2026-10-27)

Found while tracing why the per-attempt model substitution never fired. It invalidates how every
earlier runtime observation in this document should be read, so it is recorded at the top level
rather than as a footnote to Probe I.

**The downstream ext_proc filter's `receiving_namespaces` listed only the analytics namespace.**
Envoy drops a dynamic-metadata update for any namespace absent from that list — it logs
`processing response included dynamic metadata for namespace not configured for receiving:
wso2.failover` and carries on. So the failover policy computed a ranking, published it, and Envoy
discarded it on arrival, every time.

**Why nothing looked wrong.** With no published groups, `priority_group` falls back to the group
configured at that POSITION — the behaviour Probe G measured and flagged as a hazard. The request
still failed over, still reached the fallback provider, and still returned `200` with
`x-envoy-attempt-count: 2`. Every symptom available at the edge said the feature worked. What was
actually being ignored was everything the policy decides: the published order, the health state,
the suspensions, and the per-request chain length.

**What this means for the earlier results.** The mechanism probes (A, B, D, E, G, H, I) measured
Envoy behaviour directly and stand unchanged. But every end-to-end run before this fix exercised
positional fallback, not the published ranking — so none of them was evidence that ranking,
suspension, or health attribution works end to end. Those have now been re-verified; see below.

**Fix.** `wso2.failover` added to `receiving_namespaces` (so Envoy accepts the ranking at all) and
to `forwarding_namespaces` on both filter positions (so the per-attempt position can read back the
ranking that request installed). Regression tests assert both, on both filters.

**The general lesson, and it is the same one as Probe I's:** a silent-drop failure in this design
is indistinguishable from success at the edge. Neither the status code, nor the attempt count, nor
the target that answered can tell you whether the policy's decision was honoured — only the
metadata path can. Any future work here needs an assertion on the decision, not just the outcome.

---

## Re-verification after the metadata fix (2026-10-27)

Everything below was run against the live IT stack **after** `wso2.failover` started reaching
Envoy. These are the first runs in which the published ranking actually drove routing.

### The ranking drives routing

The proof is indirect but conclusive: **positional fallback is static.** If the order a request
takes ever changes in response to health state, the published ranking must be what Envoy is
reading. Every result below turns on exactly that, so each one re-confirms it.

### Suspension, and escalating backoff

Fixture: `primary(dead) → anthropic(alive)`, `suspendAfterFailures: 1`, fresh health store.

| Step | `x-envoy-attempt-count` | Meaning |
|---|---|---|
| request 1 | 2 | both tried; the primary fails and is suspended |
| request 2 | 1 | the primary is **skipped** — straight to the fallback |
| 1st suspension | ~22s | configured base 20s (2s poll granularity) |
| 2nd suspension | ~41s | doubled, as specified |

The attempt count dropping to 1 is the whole result: a static positional fallback can never
reorder, so the ranking is being honoured.

A first attempt at measuring this was misleading — the timer started when the *command* started,
not when the suspension did, and the store still held state from a previous run. Read correctly,
those numbers agreed (a 22s remainder of an already-doubled 60s suspension, then ~121s for the
next doubling); the table above is the clean run from a reset store.

### Attribution across a multi-member chain

With a two-member chain the failing member *is* member[0], so "grades the member that failed" and
"always grades the first member" are indistinguishable. A third member discriminates.

Fixture: `primary(dead) → dead2(dead) → anthropic(alive)`.

| Step | Result | Meaning |
|---|---|---|
| request 1 | 200, **3 attempts**, model `claude-sonnet-4-5-20250929` | full chain ran; the budget sized to the ranking (FR-059a) |
| request 2 | 200, **1 attempt** | **both** dead members were suspended — not just member[0] |

Had attribution graded only the first member, request 2 would have taken 2 attempts (dead2, then
anthropic). It took 1. Attribution by attempt count across the whole chain works.

The run also confirms two things incidentally: the healthy member was never suspended despite
being graded on every request, and the model substitution stayed correct when the surviving member
was reached on **attempt 1** — which is the cluster-keyed lookup of FR-057b doing its job, since an
attempt-index lookup would have read the wrong member the moment the chain shortened.

### Composition with a routing policy (T024v3)

`model-round-robin` and `model-failover` attached to the same operation. Round-robin rotates
between `gpt-4o`@primary (dead) and `claude-sonnet-4-5-20250929`@anthropic (alive); failover
configures `claude@anthropic` as the fallback for `gpt-4o`. Suspension disabled, so health state
cannot mask the alternation being measured.

| request | attempts | reading |
|---|---|---|
| 1, 3, 5 | 2 | round-robin picked the dead primary; failover rescued via its own fallback |
| 2, 4, 6 | 1 | round-robin picked anthropic; succeeded first time |

All 200, correct model substituted in both cases. Failover starts its chain from whatever the
routing policy decided — the designed behaviour, now demonstrated.

### What that test also exposed: the multi-member handoff is not wired

`resolveChain` merges a published ranking ahead of the configured fallbacks, and FR-021 promises
that **order and length** both come from that ranking. Only the *order of the first member* is
actually deliverable today:

- **Nothing publishes the list key.** `model_routing.published_ranking` is declared in
  `model-failover/publish.go` and referenced nowhere else in the policy set. `model-round-robin`
  publishes only its single selection (`model_roundrobin.selected_model` / `selected_provider`),
  which is the path the test above exercised.
- **The reader could not accept it anyway.** `publishedRanking` asserts `raw.([]Target)` against
  model-failover's own named type. A routing policy in a different module publishes its own
  `[]Target`, and Go type assertions require *identical* types, not structurally compatible ones —
  so the assertion fails even when a well-behaved publisher does exactly the right thing.

So a routing policy can hand failover **one** target (works, verified) but not **a list** (no
publisher, and an unusable reader). Fixing it needs a serialisation-friendly shared shape — an
SDK-owned type or a plain `[]map[string]any` — rather than a module-local struct. Until then,
length always comes from failover's own configuration.

---

## Nested failover: proxy AND provider both configured (2026-10-28)

Tested because the interaction was reasoned about but never run. An `LlmProxy` with failover across
providers, and one of those providers (`LlmProvider`) carrying its own failover across two of its
`upstreamDefinitions`.

### It works, and the two levels are independent

| | outer (proxy route) | inner (provider route) |
|---|---|---|
| `upstream_request_attempt_count` | 1 | **2** |
| outcome | 200 to the client | dead-region refused, live-region answered |

The provider rescued itself internally and the proxy never needed a second attempt. The two operate
on genuinely different axes — the proxy chooses *which provider*, the provider chooses *which of its
own upstreams* — so nesting is coherent rather than a conflict.

Two consequences, both now observed rather than predicted:

- **Budgets multiply.** Outer and inner budgets are set independently on their own routes. Three
  providers each with two regions is up to nine upstream calls for one client request. Nothing
  bounds the product.
- **Attribution is masked.** The proxy saw a clean success on attempt 1, so it never learns that
  provider was degraded and never suspends it. Sometimes right — it *did* self-heal — but a
  provider that is permanently half-broken stays at full share forever.

### The finding that matters more: a member with no resolvable hosts is NOT failed over

The first run of this test returned **503 `no_healthy_upstream`** with the inner route at
`upstream_request_attempt_count: 1` — the provider's failover never engaged, despite the
`priority_group` specifier being correctly generated on its route.

The difference is the kind of failure:

| Member's upstream | Envoy produces | Retried? | Failover engages? |
|---|---|---|---|
| Host does not resolve | local 503 `no_healthy_upstream` | **No** | **No** |
| Resolves, refuses connection | `connect-failure` | Yes | Yes |

`retry_on` covers upstream responses and transport failures. A cluster with zero resolvable hosts
never produces either — the router answers locally, and that reply is terminal.

**Why none of our earlier testing showed this.** At the proxy level every provider sits behind a
*loopback* cluster (`127.0.0.1:<listener>`), which is always healthy. A dead provider's DNS failure
happens on the *inner* route and comes back to the proxy as an ordinary upstream 503 — which is
retriable. The loopback indirection hides the whole class.

**Where it bites.** Any *direct* upstream: `RestApi` `upstream.main`/`sandbox`, and an
`LlmProvider`'s own `upstreamDefinitions`. That is precisely the half of this feature that is not
built yet — so the limitation would be discovered during that work rather than before it.

Practical severity is narrower than it first reads: a provider that is merely *down* resolves and
refuses, which fails over correctly. This affects members whose DNS does not resolve at all, which
is closer to a misconfiguration than an outage. It still needs stating, because the failure is a
hard 503 to the client with no failover attempted.

### Mitigation: suspension turns it from permanent into one-request-per-window

Same fixture, dead-region unresolvable, but with `suspendDuration: 60` / `suspendAfterFailures: 1`
instead of suspension disabled:

| request | result |
|---|---|
| 1 | **503** — nothing is retriable, the client takes the failure |
| 2 | **200** — the member is suspended and excluded from the ranking |
| 3 | **200** |

The local 503 still returns through the downstream chain, so the policy's response phase runs,
grades the member, and suspends it. Failover cannot rescue the *first* request, but every
subsequent one routes around the dead member.

**This only works if suspension is enabled, and it is OFF by default** — `suspendDuration` defaults
to `0`, documented as "set to 0 to disable suspension tracking entirely". The earlier run 503'd
indefinitely for exactly that reason. A publisher who never touches the advanced parameters gets no
suspension, no escalating backoff, no probe recovery, and no mitigation here.

| Mitigation | Covers | Cost |
|---|---|---|
| Suspension (built, off by default) | any runtime failure, DNS included | one failed request per window |
| `LOGICAL_DNS` rather than `STRICT_DNS` | a host that *was* resolving and stops — the stale address is retained, so it degrades to a retriable `connect-failure` and even request 1 survives | one host per cluster; loses spreading across A records. No help for a never-resolved host |
| Deploy-time resolution check | typos, decommissioned hosts — loud at deploy instead of silent at runtime | cannot catch later DNS loss |
| Envoy `retry_on` | — | **not available**: no field covers locally-generated replies |

Recommended pairing: **suspension on by default** plus a **deploy-time resolution check** —
misconfiguration caught before deploy, runtime DNS loss self-correcting after one request.
`LOGICAL_DNS` only if that single first failure proves unacceptable.

### What the policy can and cannot see (2026-10-28, measured)

| Failure | Fails over | Recorded against the member |
|---|---|---|
| Upstream 429 / 5xx | yes | yes |
| Resolves but refuses / resets | yes | yes |
| **Host does not resolve** | **no** — local reply, nothing to retry from | yes |
| **Failure after response headers** | **no** — Envoy cannot retry | **no — graded as a SUCCESS** |
| **Slow but working** | n/a | **no** — `perTryTimeout` and `p95LatencyMs` both default to 0 |
| **200 carrying an error** | n/a | **no** — status codes only, by design |
| **A member never attempted** | n/a | **no** — nothing observes an unused member |

**Correction to the policy's documented limitation.** It claimed *"only a chain where every member
is transport-dead goes unrecorded."* Measured: with every member dead, **three suspensions were
still recorded** — the local 503 travels back through the downstream chain and the response phase
grades it normally. The claim was reasoned, not measured, and is now rewritten in
`policy-definition.yaml`.

**The shape of the gaps.** Everything caught is caught from the outcome of a real request. The
blind spots are all places where there is no outcome to read: nothing has gone wrong yet (slow),
nothing was tried (unattempted member), the outcome already looked fine (post-headers failure,
200-with-error), or the outcome is too coarse (nested attribution). Closing them means measuring
health rather than inferring it — active health checks — which is where the Agent Router
comparison also points.

## Post-header failures: what Envoy can and cannot do (2026-10-28)

Searched upstream rather than assuming. Two separate problems, with different answers.

### Retrying mid-stream — closed by design, not a gap

Envoy issue #5876 ("Retry on mid-stream resets result in 503 but no retry") is answered directly:
**mid-stream retries are not supported by design** — once response headers have been sent there is
nothing sensible to do but reset. There is no feature, flag or pending change. This is structural
to HTTP, not an Envoy limitation: a `200` cannot be un-sent.

So the failover half of the streaming gap stays. Only client-side retry or full response buffering
can address it, and buffering defeats the point of streaming.

### Grading it correctly — fixable today, no Envoy change

The more damaging half is ours, not Envoy's: we grade health from the **response-headers** phase,
which saw `200`, so a stream that dies mid-body is recorded as a **success**. A provider that
consistently fails mid-stream therefore looks like the healthiest member of the chain and attracts
more traffic.

Envoy does report the true outcome — `upstream_reset_after_response_started{...}` in
`response_code_details` on the **access log entry**, which is emitted after the stream completes.

And the plumbing already exists in the engine:

- `internal/utils/access_logger_server.go` — the engine already runs an access-log service
- `internal/analytics/analytics.go:441` — already reads `GetResponseCodeDetails()`
- `internal/kernel/fault_source.go:125` — already matches the `upstream_reset` prefix

So failover health can be graded from the access-log entry instead of, or in addition to, the
response-headers phase. That closes the mis-grading entirely without touching Envoy.

### Second finding from the same search: `retry_on: reset` and expensive POSTs

The generated retry policy is
`5xx,connect-failure,refused-stream,reset,retriable-status-codes`.

Per Envoy's docs, `reset` retries any upstream reset **before a response, including after the
upstream has received the full request** — so a non-idempotent POST can be sent twice.
`reset-before-request` retries only when Envoy has sent no bytes upstream.

For LLM traffic this is not academic: a provider that has received the whole prompt and is
generating can be reset, and the same prompt is then sent to another provider — **billed twice, and
potentially answered twice**. Worth an explicit decision rather than inheriting the broader value.


---

## Merge rule, end to end (2026-10-29)

**Question.** When a routing policy picks the target, do the failover policy's
configured fallbacks apply to *that* target — and only to that target?

**Why an ordinary test was not enough.** The first implementation of this rule
passed the entire unit suite while being wrong: it replaced the configured
target with routing's pick, which happens to agree whenever the two coincide.
The runtime check below was therefore built to *discriminate* — to produce a
different observable result under the old semantics than under the new.

### Fixture

| Piece | Value |
|---|---|
| `model-round-robin` (v1) | rotates `gpt-4o` (primary) ↔ `mistral-large` @ `mistral` |
| `model-failover` (v0) | one target: `gpt-4o` → fallback `claude-sonnet-4-5-20250929` @ `anthropic` |
| `tx-openai-provider` (primary) | `http://sample-backend:9998` — refuses |
| `tx-mistral-provider` | `http://sample-backend:9999` — refuses |
| `tx-anthropic-provider` | `http://sample-backend:9080/anthropic-upstream` — alive |
| suspension | disabled on both policies, so ordering is the only variable |

Both rotation members fail, and exactly one of them has a configured chain.

### Result — `POST /tx-proxy/chat/completions`, six requests

| # | Routing picked | HTTP | `x-envoy-attempt-count` | Served by |
|---|---|---|---|---|
| 1 | `gpt-4o` (configured) | `200` | `2` | `/anthropic-upstream/chat/completions` |
| 2 | `mistral-large` (not configured) | `503` | `1` | — |
| 3 | `gpt-4o` | `200` | `2` | `/anthropic-upstream/chat/completions` |
| 4 | `mistral-large` | `503` | `1` | — |
| 5 | `gpt-4o` | `200` | `2` | `/anthropic-upstream/chat/completions` |
| 6 | `mistral-large` | `503` | `1` | — |

**Both halves hold.** A configured target's own fallback fires (rows 1/3/5).
An unconfigured target borrows nothing and is not rescued (rows 2/4/6) — under
the previous semantics those rows would have returned `200` via anthropic. The
alternation is the evidence.

### Incidental findings

- **Policy versions are major-only.** `version: v1.1.1` is rejected at admission
  (`version must be major-only`); the attachment carries `v1` while the policy
  definition carries `v1.1.1`.
- **The request path carries no version segment.** Chains are keyed
  `POST|/tx-proxy/chat/completions|*`; a request to
  `/tx-proxy/v1.0/chat/completions` matches the `POST|/tx-proxy/*|*` wildcard
  chain instead and runs neither routing nor failover. This is the suite's
  existing convention (every `failover-core.feature` scenario calls
  `/<context>/chat/completions`), not a defect — noted because the version-
  prefixed form fails *silently*, returning a plausible `200` from the primary
  with no indication the policies never ran.

---

## Canonical provider shape, end to end (2026-10-29)

T024a0's controller-side fix was unit-tested only, so it was run at runtime. Two
defects surfaced that no unit test had reached — the second of them severe.

### Defect: a canonical proxy produced no routes at all

`LLMTransformer.Transform` (`pkg/transform/llm.go:151`) dereferenced
`proxy.Spec.Provider.Id` unconditionally. `spec.Provider` is **nil** in the
canonical shape, so it panicked — and `processEventSafely` recovered:

```
level=ERROR msg="Recovered from panic while processing event" event_type=LLM_PROXY action=CREATE
runtime error: invalid memory address or nil pointer dereference
  transform.(*LLMTransformer).Transform  pkg/transform/llm.go:151
  xds.(*Translator).TranslateConfigs     pkg/xds/translator.go:924
```

The management API answered `201`, `status.state` read `deployed`, the log said
*"Artifact deployed successfully"* — and the gateway served `404` with
`route_name: "no-api-found"`. A canonically-authored proxy could not be deployed
at all, and nothing said so. This was introduced by this feature, in the block
that points the empty alias at the primary's API-scoped cluster.

**Fixed** by resolving through `PrimaryLLMProxyAttachment`. Matching on
`EffectiveName()` rather than `Id` also closes a latent legacy bug: a legacy
primary that sets `as` is named by that alias in its own loopback definition, so
the `Id` lookup missed and left the empty alias on the **shared** main slot —
the cross-API credential collision that block exists to prevent.

### Verification

Proxy `tx-canon-proxy`, authored canonically — primary `tx-mistral-provider`
(refuses, forcing failover), fallback `anthropic` carrying `auth`, `transformer`,
and the new `policies` field.

Chain built for the fallback's cluster, in the FR-039 order:

```
upstream:upstream_LlmProxy_01a18663…_anthropic
  -> [model-failover, set-headers, openai-to-anthropic-transformer, set-headers]
      per-attempt    author's own   transformer                     credential (last)
```

`POST /tx-canon/chat/completions` → `200`, `x-envoy-attempt-count: 2`:

| Observation | Value | Proves |
|---|---|---|
| `path` | `/anthropic-upstream/v1/messages` | transformer rewrote the path |
| body | `{"max_tokens":4096,…,"model":"claude-sonnet-4-5-20250929"}` | transformer rewrote the body |
| `x-api-key` | `canon-secret-key` | credential applied |
| `anthropic-version` | `2023-06-01` | transformer header applied |
| `x-canon-upstream-policy` | `ran` | the author's `policies` on the canonical entry ran |

All four pieces the defect silently dropped are present on the failover attempt.

---

## Layered routing and failover, end to end (2026-10-30)

First runtime proof that a ranking published by one policy module is consumed
by another. Everything before this was unit-tested on each side of the module
boundary separately.

### Setup

`model-round-robin` v1.2.0 rotating two targets, `model-failover` attached with
**no fallbacks of its own** — so the only depth available is the ranking
round-robin published. The dead target is a resolvable host with a closed port
(`connect-failure`, retriable); see the correction below for why.

### Result

```
ranking key present   go_type=[]map[string]string   decoded=2
published failover chain   members=2
```

The pinned carrier crosses the module boundary and decodes. Attempt counts:

| Request | Rotation leads with | HTTP | attempt-count |
|---|---|---|---|
| 1 | `gpt-4o` (dead) | 200 | **2** |
| 2 | `claude@alive` | 200 | **1** |
| 3 | `gpt-4o` (dead) | 200 | **2** |
| 4 | `claude@alive` | 200 | **1** |

**2, 1, 2, 1.** The alternation is the proof that round-robin advances once per
CLIENT request, not once per attempt — had the cursor moved on the retry in
request 1, request 2 would also have taken two attempts and the pattern would
read 2, 2, 2, 2.

### Suspension skip

Same fixture with `suspendDuration: 300`, `suspendAfterFailures: 1`:

| Request | Rotation leads with | attempt-count | |
|---|---|---|---|
| 1 | `gpt-4o` | **2** | fails, rescued — and is suspended |
| 2 | `claude@alive` | **1** | |
| 3 | `gpt-4o` **again** | **1** | round-robin still nominates it; failover skips it |

Request 3 is the discriminator. Round-robin never learned the target was
suspended — it is not told, by design — and nominated it anyway. A count of 2
would mean the suspension was ignored and the dead target attempted. A count of
1 proves both that the suspension is honoured and that **a skip costs no
attempt**: had skips consumed budget, a chain of mostly-suspended targets would
exhaust itself stepping over dead entries before reaching a live one.

### Two corrections this run forced

- **The test used an un-rescuable failure mode.** The dead target was on an
  unresolvable hostname, copied from `failover-core.feature`. That yields
  Envoy's local `no_healthy_upstream`, which is NOT retriable — this document
  already recorded that, and the test contradicted it. Every request failed
  with one attempt and no failover. Switched to a resolvable host with a closed
  port. **`failover-core.feature` has the same flaw** and has never run in the
  default suite.
- **Wrong parameter name.** The fixture used `consecutiveFailures`; the policy
  schema declares `suspendAfterFailures`. Rejected at deploy with
  `Additional property consecutiveFailures is not allowed`.

### What actually caused the hour of misdiagnosis

The first run showed `members=1` with round-robin visibly publishing a correct
two-element ranking, which read as a defect in the consumer. It was not. The
deployed image had been built from `gateway/dev-policies/model-failover`, a
manual copy that had never received the T066b decode fix — it still asserted
`raw.([]Target)`, failed for `[]map[string]string`, and fell through to the
single-selection path.

The copy is what the image compiles, not the source repo, and nothing warned
that the two had diverged. `gateway/sync-dev-policies.sh` now mirrors each dev
policy and `--check` reports drift without writing.

---

## Timeouts: which slow targets fail over, and which legitimate ones get cut (2026-10-30)

Prompted by the question "what about a timeout from the actual target?". Probed with
`it/spikes/failover/t1-idle-timeout.yaml`, which mirrors the retry policy the controller
generates today: `per_try_idle_timeout` set, `per_try_timeout` absent.

### What the two timeouts do

| | Bounds | On firing |
|---|---|---|
| `per_try_timeout` | one attempt's total duration | attempt failed → **retriable**, failover engages |
| `per_try_idle_timeout` | silence within one attempt | attempt failed → **retriable**, failover engages |
| route `timeout` | the whole request, retries included | local `504` → **terminal**, no failover |

The route timeout is a local reply, so it joins `no_healthy_upstream` in the
not-retriable class. The two per-try bounds are both retriable.

### Measured

**`per_try_idle_timeout` bounds TIME TO FIRST BYTE, not merely a mid-stream stall.**
A target stalling 20s before sending anything, against a 5s idle timeout:

```
http=200  elapsed=5.3s  attempts=2  cluster=provider_b
```

Cancelled at 5s and failed over — despite being healthy and about to answer `200`.

**Streaming with sub-threshold gaps never trips it.** Six chunks 3s apart, 15s total,
same 5s idle timeout:

```
http=200  elapsed=15s  attempts=1  cluster=provider_a
```

### The defect this exposes is the OPPOSITE of the one suspected

The concern was that a slow target would not fail over. It does. The real problem is that
a *legitimately* slow one does, when it should not.

A non-streaming completion sends **nothing** until the answer is ready. From the proxy's
side that is indistinguishable from a hang, and the idle timer does not care which it is.
With the current `perTryIdleTimeout` default of **10s**, any non-streaming completion
taking longer than 10 seconds is cancelled and failed over — which for LLM traffic is
routine, not exceptional.

| Request shape | Behaviour today | Correct? |
|---|---|---|
| Streaming, target hangs before first token | failed over at 10s | yes |
| Streaming, target stalls mid-stream >10s | failed over | yes |
| Streaming, slow but steady (gaps <10s) | served, any total duration | yes |
| **Non-streaming, legitimately takes >10s** | **failed over at 10s** | **no** |

The effective ceiling for a non-streaming completion is therefore 10s, and the request
then burns a second target for an answer the first was about to give.

### Why it cannot simply be made per-request

`x-envoy-upstream-rq-per-try-timeout-ms` overrides `per_try_timeout` per request, but
**there is no header equivalent for `per_try_idle_timeout`** — confirmed against the full
`RetryPolicy` field set in go-control-plane v1.40.0. It is static route configuration.

So the policy cannot read `"stream": true` from the body and pick a bound, even though it
already parses that body. The asymmetry is structural: streaming wants a SHORT idle bound,
non-streaming wants a LONG one, and only one value exists per route.

### Options

| | Approach | Cost |
|---|---|---|
| **A** | Raise the `perTryIdleTimeout` default well above the longest expected non-streaming completion | A hung streaming target takes that long to fail over |
| **B** | Generate separate routes for streaming and non-streaming; the policy rewrites `:path` on `"stream": true` and clears the route cache | Doubles the route count; needs `allow_all_routing`, which is already enabled (T021a) |
| **C** | Set `per_try_timeout` per request via the header for non-streaming | **Does not work** — the idle timer fires first regardless |
| **D** | Disable the idle timeout by default | Strictly worse than A: a hang then reaches the route timeout, which is terminal, so failover never engages at all |

**A is the immediate fix**: a correct answer that is slow to fail over beats a fast failover
of requests that were never broken. **B is the only option that gets both**, and its
machinery already exists.

### Secondary finding: the route timeout is itself tight for LLM

`RouteTimeoutMs` defaults to **60000** (`config.go:1428`). That bounds the whole request,
retries included, and firing it is terminal. Raising `perTryIdleTimeout` above 60s is
therefore pointless — the route timeout wins. Any fix has to move both, and 60s is already
short for a long non-streaming completion before failover is in the picture at all.

---

## An unresolvable host DOES fail over on an LlmProxy (2026-11-02)

Settled by reverting the layered feature's dead upstreams to unresolvable
hostnames and running it. The three failover scenarios **passed**.

### Why — the loopback hop

An LlmProxy request takes two hops through the same Envoy:

```
client → :8080  /my-proxy/chat/completions
                     │   ← the failover retry policy is on THIS route
                     ▼
         cluster upstream_LlmProxy_<uuid>_anthropic
             = 127.0.0.1:8080, base path /anthropic-ctx     ← loopback
                     │
                     ▼
         :8080  /anthropic-ctx/chat/completions
                     │   ← the LlmProvider's own route
                     ▼
         cluster api.anthropic.com      ← DNS fails HERE
```

The DNS failure lands on the **inner** route; the retry policy sits on the
**outer** one. From the outer route's view its cluster is `127.0.0.1:8080`,
which is always healthy — it connected and received a response that happens to
be a `503`, and an upstream `503` matches `retry_on: 5xx`.

| Whose view | What it sees |
|---|---|
| Inner route | local `no_healthy_upstream` — terminal |
| Outer route, where failover lives | upstream returned `503` — **retriable** |

The loopback launders a terminal local reply into a retriable status. On a
DIRECT upstream there is no second hop, so the local reply lands on the route
carrying the retry policy and nothing can be retried. **The limitation recorded
earlier is real, but only for direct upstreams.**

This does not make failover catch everything. The rule is unchanged: retriable
means something reached an upstream. A committed response, the route's overall
timeout, a `4xx`, a client disconnect and an exhausted attempt budget are all
still terminal.

### Two corrections this forced

- **A fix applied earlier was unnecessary.** The layered feature's dead
  upstreams had been changed from unresolvable hostnames to refusing ports.
  That change was made in the same step as rebuilding a stale policy copy, and
  the stale policy was the entire cause — the hostnames were never the problem.
  Reverted, and the feature's prose now carries the loopback explanation
  instead of the claim that a refusing port is required.
- **A real defect in the feature file.** One scenario asserted
  `x-envoy-attempt-count: 1` against a proxy with NO failover attached. The
  controller sets `IncludeAttemptCountInResponse: t.vhostHasFailover(routes)`,
  so that header does not exist there; the assertion was testing for the
  presence of a feature the scenario deliberately omits. It survived manual
  `curl` verification because that scenario was never exercised by hand.

Result after both: **17 scenarios, 199 steps, all passed.**

---

## The attempt-count header is virtual-host scoped (2026-11-02)

Found by the cross-API isolation scenario (T085), which is what that scenario is
for.

**FR-030d and SC-012a say an API that does not attach failover behaves
identically whether or not another API does. For one response header, it does
not.**

`IncludeAttemptCountInResponse` is set per VIRTUAL HOST when any route in it
has failover:

```go
IncludeAttemptCountInResponse: t.vhostHasFailover(routes)
```

Two LlmProxies on the same vhost therefore share the setting. Attaching
failover to API A adds `x-envoy-attempt-count: 1` to API B's responses, though
B has no failover and never retries.

**It cannot be scoped away by configuration.** Checked against the full
`VirtualHost` and `Route` field sets in go-control-plane v1.40.0:
`include_attempt_count_in_response` exists on `VirtualHost` only. There is no
per-route equivalent to set.

### Severity, and the options

Low. The header is informational, carries no identity, and says only that some
API on this gateway uses failover. Nothing in a response body or status changes,
and no credential or routing decision crosses between the APIs — the same
scenario asserts both of those and they hold.

| Option | Cost |
|---|---|
| Accept and document | FR-030d is not strictly true, and says so |
| `response_headers_to_remove` on non-failover routes | Restores the requirement; adds a header mutation to unrelated routes in any mixed vhost, widening this feature's blast radius beyond APIs that use it |
| Relax SC-012a to exclude informational Envoy headers | Honest, and narrows a requirement that was written without this constraint in view |

Not fixed. The integration scenario asserts the header as it actually behaves,
with a comment pointing here, so the day someone scopes it properly that line
fails and is updated deliberately rather than silently drifting.

### The value is never wrong, and the upstream side crosses too

What crosses is the header's PRESENCE, not its value. Same run, same vhost,
consecutive requests: API A reported `2` and API B reported `1`. Envoy's attempt
counter is per-request stream state; the virtual-host flag only decides whether
that per-request number is emitted. A shared switch, not a shared counter.

The failover policy reads the header off its own response context, and API B has
no failover policy at all, so nothing grades anything for B.

`IncludeRequestAttemptCount` is enabled on the same virtual host and is required
— it is how the per-attempt chain learns which attempt it is serving. So B's
UPSTREAM also receives an `x-envoy-attempt-count` header it would not otherwise
see. Same character as the response side: informational, correct value, on a
request that never retries.

### What the scenario still proves

The isolation that matters holds. API B's request carried its own policy's
marker and none of API A's credential, with an echo backend reporting what it
actually received — so A's per-attempt chain did not run on B's request, which
is the claim the design document makes.
