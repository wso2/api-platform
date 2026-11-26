# Contract: Target Preference List

**Between**: routing policies (producers) and `model-failover` (consumer)
**Status**: new contract introduced by this feature
**Backward compatibility**: a producer that does not implement it keeps working unchanged

---

## Why this exists

Routing policies today publish a **single** selection — `metadata["selected_provider"]`, consumed by
the translator policies (`gateway/examples/openai-multi-provider-proxy.yaml:18-25`). Failover needs
more than the winner: it needs the policy's **ranking**, so traversal after a failure follows the
algorithm's own judgement rather than a fixed list.

### Both keys are addressed generically, and that is load-bearing

`selected_provider` is **not** namespaced by producer, and the ranking key is namespaced by
*purpose* (`model_routing`) rather than by any one policy. A consumer of a shared channel must read
the channel's own address.

This was got wrong once and is worth recording. The consumer originally read
`model_roundrobin.selected_provider` — `model-round-robin`'s **private** key, introduced by
`64a2d3b` (2026-07-27) for that policy's own bookkeeping. Of the eight steering policies in
`wso2/gateway-controllers`, only that one writes it. Every other policy writes bare
`selected_provider`, the convention established by `77dcac0` (2026-07-09) and read by all five
`openai-to-*` transformers. So with failover attached, seven steering policies had their decision
read as *absent* and silently discarded while failover published its configured order instead — a
`cost-based-model-routing` request would still be billed at whichever provider the publisher
configured first, not the cheap one the policy chose.

Nothing failed and nothing was logged, because an absent selection is a legitimate state: it is what
a producer writes when it has nothing to say. That is why `TestContractKeysArePinned` asserts the
**literal** value and rejects any `.` in it. Every other test in the suite references the constant,
so a rename moves producer and consumer together and the suite stays green.

The ranking is published **once per client request** and carried across attempts, so no routing
algorithm is ever re-run mid-request — no re-classification, no re-scoring, no re-drawn weighting
(FR-022).

> **Status (2026-10-29): the carrier is pinned and the consumer accepts it (T066a/T066b/T066c).**
> Nothing publishes the list key yet — `model-round-robin` adopts it under T070 — so the
> multi-member path is still unexercised in production. What changed is that a conforming producer
> now *can* be read: the consumer previously asserted the metadata value against a struct declared
> inside `model-failover`, which no other module could satisfy (Go type assertions require
> identical types, not structurally compatible ones), so a correct producer failed silently and
> degraded to the single-selection path. The single-selection path works and is verified.

> **Consumer behaviour changed 2026-10-29.** A published ranking no longer defines the whole
> chain. The producer's **first** target is honoured, then the publisher's configured fallbacks
> take precedence, and the remainder of the ranking follows as further depth. A producer ranking
> `[A, D, C]` against a configured chain of `A → [B]` yields **A, B, D, C**. Producers are
> unaffected — publish your full ranking as before — but the ranking is no longer guaranteed to be
> traversed contiguously. See spec FR-021.

## Wire representation

The ranking travels in the policy engine's in-process shared request metadata, under the key:

```
model_routing.published_ranking
```

Its value MUST be a **`[]map[string]string`** — a slice of plain maps, best target first:

| Key | Required | Meaning |
|---|---|---|
| `model` | yes | AI model name. An entry with an empty or missing `model` is skipped, not an error. |
| `provider` | no | `additionalProviders[].as` alias, or the provider id. Absent or empty means the primary provider. |

```go
ranking := []map[string]string{
    {"model": "gpt-4o"},
    {"model": "claude-sonnet-4-5-20250929", "provider": "anthropic"},
}
metadata["model_routing.published_ranking"] = ranking
```

**Built-in types only, deliberately.** A producer lives in a different Go module from the
consumer, and a Go type assertion matches *identical* named types, not structurally compatible
ones — so a shared struct could only work if every producer imported a types package from the
consumer, which is the coupling this metadata channel exists to avoid. The existing
single-selection keys are plain `string` for the same reason, and they work across modules today.

The consumer also accepts a `[]any` whose elements are `map[string]string` or `map[string]any`
with string values, since a producer assembling the list generically naturally produces that.
Both decode identically. Anything else is treated as no ranking at all, and the request degrades
to the single-selection path rather than failing.

## The single-selection tier

A producer that has not adopted the ordered list publishes:

```
selected_provider = "<additionalProviders[].as alias or provider id>"
```

It carries a **provider and nothing else**, and the consumer reads it as *"the model the client
asked for, at this provider"* — yielding the one-element ranking `{requested model, provider}`.

That reading is not an assumption; it is how the rest of the chain already behaves. No
`openai-to-*` transformer derives a model from this key: each resolves the model from the request
body, falling back to its own configured override, and uses `selected_provider` only to decide
whether it is the transformer for this request. Provider selection and model selection are
orthogonal everywhere else, and this consumer matches that.

**An empty or absent value is not a selection.** `applyProviderRouting` *deletes* the key when its
choice is the primary provider, so empty means "no steering happened" and the configured order must
stand. A whitespace-only or non-string value is treated the same way.

**A producer that needs to name a different MODEL must use the ordered list.** The single-selection
channel cannot express it, and no generic key for it exists — inventing one would add a third
address to a channel that already has a working convention.

## Producer obligations

A routing policy that adopts this contract MUST:

1. **Publish an ordered list** of targets, best first, in shared request metadata. Each entry is a
   `(model, provider)` pair using the same shape as `model-round-robin.models[]`, where `provider`
   matches an `additionalProviders[].as` alias or the provider id.
2. **Rank over its own configured targets.** A producer does NOT see failover's health state, and
   does not need to: failover's suspension is private per-instance state, and chain resolution
   filters suspended targets out wherever they appear — including the leading one — so nominating
   one is harmless (FR-026 withdrawn, FR-027 revised). Where weighting applies, re-normalize over
   the producer's own view.
3. **Publish exactly once per client request.** On a re-entered attempt the policy MUST NOT
   recompute; the carried ranking is authoritative.
4. **Keep its own suspension setting unchanged.** It is not superseded when failover is attached
   (FR-026a withdrawn). The downstream chain is entered once per client request, so a producer
   observes only the final status: a rescued request reports success and suspends nothing, and a
   5xx arrives only on a fully exhausted chain, where the target it selected did fail. Both health
   views are independent and both correct.
5. **Behave exactly as before** whether or not a failover policy is attached (FR-026c) — including
   its own `suspendDuration`. Adopting this contract adds a published ranking and changes nothing
   else.

### Adoption at launch (FR-028, FR-029)

Audited against `wso2/gateway-controllers` on 2026-11-09 — every steering policy in the repo, not a
sample. "Degrades" now means it genuinely works through the single-selection tier; before the key
fix above it meant the policy's decision was dropped.

| Policy | Version | Adopts the ordered list | What failover sees |
|---|---|---|---|
| `model-round-robin` | v1.2.0 | **Yes** | Full rotation starting at the current cursor |
| `model-weighted-round-robin` | v1.1.1 | No — degrades | Single selection ⇒ one-element list |
| `cost-based-model-routing` | v0.9.0 | No — degrades | Single selection ⇒ one-element list |
| `intelligent-model-routing` | v0.9.0 | No — degrades | Single selection ⇒ one-element list |
| `llm-header-router` | v0.9.1 | No — degrades | Single selection ⇒ one-element list |
| `semantic-model-routing` | v0.9.0 | No — degrades | Single selection ⇒ one-element list |
| `time-based-model-routing` | v0.9.1 | No — degrades | Single selection ⇒ one-element list |
| `typesafe-jev-model-routing` (vendor-owned) | v0.8.0 | No — degrades | Single selection ⇒ one-element list |

`model-weighted-round-robin` was previously listed here as adopting at launch. It does not: it
writes `model_weighted_roundrobin.selected_*` plus bare `selected_provider`, and nothing else. Its
*pick* therefore reaches failover through the single-selection tier, but its weighted **order** does
not — the weights decide which provider leads and then the publisher's configured chain supplies the
rest. Adopting the ordered list is what would carry the draw order, and it has not.

### Steering that this contract does not cover

`dynamic-endpoint` (v1.0.1) and `interceptor-service`'s `DynamicEndpoint` reply steer to an
arbitrary **upstream definition**. That turned out NOT to be a vocabulary problem: a ranking already
addresses any upstream definition, because `provider` IS the definition name —
`BuildProviderClusterMap` keys its map by `def.Name` for every definition plus `""` for the primary,
and on an `LlmProxy` the `additionalProviders[].as` alias becomes exactly such a name. Measured: a
steered `Target{Model: gpt-4o, Provider: chosen}` resolved to that upstream's cluster for a pair the
control plane never configured as a group.

What is missing is only that these two write **no** `selected_provider` — they return `UpstreamName`
alone, which the engine resolves *after* the whole chain, into a channel the route no longer consults
once failover owns its cluster specifier. One line in each policy closes it, needing no change to
this contract or to `model-failover`. Not done here: different policies, different releases
(T024u8). Until then, attaching both deploys and leaves the steering inert, with a generation-time
warning as the only signal. Decision recorded as FR-053a / FR-052a.

No routing policy is *required* to change for failover to function (FR-023).

## Consumer obligations (`model-failover`)

1. **Treat a single selection as a one-element list** (FR-023).
2. **Treat an empty or absent list** as "no ranking" and fall back to the configured chain, recording
   the condition.
3. **Select the configured chain by the ranking's FIRST target**, not by the model the client
   requested (FR-021a). Attempt that chain's fallbacks immediately after it, ahead of the rest of
   the ranking (FR-021, FR-001b). Where the publisher configured nothing for that target and no
   `"*"` catch-all matches, there are no fallbacks — a chain written for one target is never
   borrowed for another.
4. **Continue through the remainder of the ranking** once those fallbacks are spent, in the order
   it was published (FR-024).
5. **De-duplicate**, keeping a target's first position; attempt each target at most once per request
   (FR-010).
5. **Filter against health** — skip suspended targets wherever they appear in the ranking; a skip is
   not an attempt (FR-025).
6. **Publish nothing back to producers.** The flow is one-way: a routing policy publishes its
   ranking, failover consumes it. Where a producer must change behaviour because failover is
   attached (FR-026a), the controller tells it at deployment time, not through request metadata.

## Resolution example

```
Configured chains:    B -> [E]                     # the publisher's fallback for B
Published ranking:    [B, C, A]                    # weighted draw, healthy-only
Suspended:            {C}

Chain is selected by the ranking's FIRST target, B — not by the model the client asked for.
B's configured fallbacks come next, ahead of the rest of the ranking. C is skipped on health.

ResolvedChain       = [B] ++ [E] ++ [C, A] filtered by health and de-duplicated
                    = [B, E, A]                    # C skipped, not attempted
Attempt budget      = 3
```

Had the ranking's first target been one the publisher configured nothing for, there would be no
fallbacks at all — the chain would be `[B, C, A]` filtered by health. A chain written for one
target is never borrowed for another (FR-021a).

## Failure modes this contract must not create

| Scenario | Required behavior |
|---|---|
| Producer publishes a suspended target | Consumer skips it; no attempt counted. Expected, not a producer error — a producer cannot see failover's health. |
| Producer publishes a target absent from `targets` | Attempted in ranking position (FR-025 of spec) |
| Producer and consumer disagree on health | Consumer's view governs; it is the single authority, and the only one that gates an attempt |
| Ranking metadata absent on a re-entered attempt | Treated as a defect, not re-derived: the request fails rather than silently re-ranking |
| Ranking arrives from the client | Rejected — the carrier is gateway-internal and stripped at ingress |
