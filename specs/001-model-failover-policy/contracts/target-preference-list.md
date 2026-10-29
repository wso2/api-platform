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

| Policy | Adopts at launch | Ranking it publishes |
|---|---|---|
| `model-round-robin` | **Yes** | Full rotation starting at the current cursor |
| `model-weighted-round-robin` | **Yes** | Weighted draw order over the healthy set |
| `llm-header-router` | No — degrades | Single selection ⇒ one-element list |
| `typesafe-jev-model-routing` (vendor-owned) | No — degrades | Single selection ⇒ one-element list |

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
