# Gateway error codes

Every error the gateway produces carries a `code` in its `FaultDetails`. This is the reference
for what those codes are and how to pick one when adding a new failure.

The short version: **codes are six-digit numeric strings, drawn from an existing registry
wherever one already covers the condition.** They are not a new namespace.

---

## Range membership is part of the contract

This is the part that is easy to miss, and the reason a code cannot simply be allocated at
will.

Analytics assigns a fault **category** by testing which range a code falls in
(`start <= code < end`). A code outside every range is categorised as `other`, however specific
its meaning.

| Category | Range |
|---|---|
| Authentication | `900900` – `901000` |
| Throttling | `900800` – `900900` |
| Target (upstream) connectivity | `101500` – `101600` |
| WebSocket target | `1002` – `1015` |

So a new code for an upstream failure has to land **inside** `101500`–`101600`, or analytics
stops seeing it as an upstream failure at all. Picking a number outside the range does not just
lose a label — it silently reclassifies the event.

## Who owns which block

Ranges above decide the *category*. This decides who may allocate a number at all.

| Block | Owner | Notes |
|---|---|---|
| `101500`–`101600` | Synapse | Target connectivity. Reuse only. |
| `303001` | Reserved | Endpoint suspended. |
| `900000`–`904999` | Reserved (data plane) | Reuse only. Frontier is `904015`. |
| `905000`–`905999` | This gateway (engine) | Conditions the reserved blocks do not cover. |
| `906000`–`906399` | This gateway (guardrail) | Every guardrail rejection. See below. |
| `960000`–`964999` | WSO2-shipped policies | Allocated per policy. Frontier is `962401`. |
| `965000`–`969999` | **Reserved for users** | See below. |
| `990000`–`999999` | Reserved (control plane) | Not for data-plane bodies. |

Blocks not listed are unallocated. Do not use one — an unallocated number carries no more
information than the HTTP status beside it, and forecloses the block for whoever needs it later.

---

## The codes the gateway emits

### Upstream and routing — produced by the router, described by the engine

| Condition | Code | Origin |
|---|---|---|
| Could not connect / connection reset | `101503` | Synapse `NHTTP_CONNECTION_FAILED` |
| Upstream did not respond in time | `101504` | Synapse `NHTTP_CONNECTION_TIMEOUT` |
| No healthy host to route to | `303001` | endpoint suspended |
| Router failure, no more specific cause | `101599` | top of the target-failure range |
| Request matched no API | `900906` | resource not found |

`303001` sits outside the target range but is still classified as a target fault, because
the classifier handles it with an explicit case. It is a *distinct sub-category* —
`CONNECTION_SUSPENDED` rather than `CONNECTION_TIMEOUT` — which mirrors the distinction the
router itself draws between `no_healthy_upstream` and `upstream_reset`.

`101599` is the top of the target range on purpose: it has to be inside the range to classify
correctly, and Synapse allocates upward from `101500`, so the top is the far end from whatever
it takes next.

### Produced by the engine itself

| Condition | Code |
|---|---|
| Policy chain failed to execute | `905003` |
| Request body over the decompression ceiling | `905004` |
| Route has no policy chain | `905005` |
| Resolver could not turn the request into an operation | `905006` |
| Content coding the gateway cannot decode | `905007` |

`905005` is distinct from `905003` because it is a different thing to fix: the chain never
arrived, rather than arrived and failed. Both are opaque 500s to the caller.

Resolution failures are coded by the **status**, never one code per internal failure kind. The
status already tells a caller which class of thing went wrong, so a code tracking it discloses
nothing further — whereas a code per kind would let a caller tell an unparseable body from an
invalid one, which is resolver internals. So the four conditions that all mean `400` share
`905006`, a missing operation reuses `900906` (the same event as a request that matched no API),
an oversized payload reuses `905004`, and every 500 among them is `905003`. The kind still
reaches the log, the metric and the span.

No reserved block has a code or a category for these, so they are allocated in `905xxx` —
above the `904015` frontier, so they cannot collide with a control-plane code. They classify as
`other`, which is the honest outcome: there is no existing category to claim, and squatting
inside a reserved range to borrow its
label would mis-report them as target or auth failures.

### Emitted by policies

A policy describes its own rejection, so these are the policy's to choose. Reuse these:

| Condition | Code | Name |
|---|---|---|
| No credentials presented | `900902` | `API_AUTH_MISSING_CREDENTIALS` |
| Credentials rejected | `900901` | `API_AUTH_INVALID_CREDENTIALS` |
| Authenticated but not permitted | `900908` | `API_AUTH_FORBIDDEN` |
| Token expired | `900903` | `API_AUTH_ACCESS_TOKEN_EXPIRED` |
| Scope insufficient | `900910` | `INVALID_SCOPE` |
| Subscription inactive | `900909` | `SUBSCRIPTION_INACTIVE` |
| API blocked | `900907` | `API_BLOCKED` |
| Other authentication failure | `900900` | `API_AUTH_GENERAL_ERROR` |
| Guardrail intervened, kind unspecified | `906000` | `GuardrailCodeIntervened` |
| Throttled — API level | `900800` | `API_THROTTLE_OUT_ERROR_CODE` |
| Throttled — resource level | `900802` | `RESOURCE_THROTTLE_OUT_ERROR_CODE` |
| Throttled — application level | `900803` | `APPLICATION_THROTTLE_OUT_ERROR_CODE` |
| Throttled — subscription level | `900804` | `SUBSCRIPTION_THROTTLE_OUT_ERROR_CODE` |
| Blocked by policy | `900805` | `BLOCKED_ERROR_CODE` |

### Guardrail codes: `906000`–`906399`

Every guardrail rejection carries a code from this one block, so "was this a guardrail
rejection?" is a single range test — the same rule every other category follows. The codes are
in the SDK as `policy.GuardrailCode*` (Go) and
`apip_sdk_core.policy.v1alpha2.GuardrailCode` (Python).

`906000` is the generic: use it when the guardrail has nothing more specific to say, which is
the common case, since the policy name usually already answers it. Reach for a specific code
when a caller has to branch on which rejection it was — a self-harm rejection and a violence
rejection may need routing to different places, and only the code can tell them apart.

It sits at the head of the block, outside all four groups, so a group test never matches it.

| Group | Range | Question it answers |
|---|---|---|
| Content safety | `906001`–`906099` | what the content was judged to **be** |
| Sensitive data | `906100`–`906199` | what the content **contained** |
| Shape | `906200`–`906299` | what **shape** the content had |
| Intent | `906300`–`906399` | what the content **meant** |

| Condition | Code | Constant |
|---|---|---|
| Rejected, kind unspecified | `906000` | `GuardrailCodeIntervened` |
| Hate | `906001` | `GuardrailCodeHate` |
| Sexual | `906002` | `GuardrailCodeSexual` |
| Self-harm | `906003` | `GuardrailCodeSelfHarm` |
| Violence | `906004` | `GuardrailCodeViolence` |
| Harassment | `906005` | `GuardrailCodeHarassment` |
| Dangerous or illegal activity | `906006` | `GuardrailCodeDangerousActivity` |
| Profanity | `906007` | `GuardrailCodeProfanity` |
| Prompt injection / jailbreak | `906008` | `GuardrailCodePromptInjection` |
| Unsafe, no category given | `906099` | `GuardrailCodeUnsafeContent` |
| PII detected | `906101` | `GuardrailCodePII` |
| Credential or secret detected | `906102` | `GuardrailCodeCredential` |
| Word count | `906201` | `GuardrailCodeWordCount` |
| Sentence count | `906202` | `GuardrailCodeSentenceCount` |
| Content length | `906203` | `GuardrailCodeContentLength` |
| Schema mismatch | `906204` | `GuardrailCodeSchema` |
| Pattern match | `906205` | `GuardrailCodePattern` |
| URL not allowed | `906206` | `GuardrailCodeURL` |
| Semantic match against a blocked prompt | `906301` | `GuardrailCodeSemanticMatch` |

The grouping is part of the contract. A caller asking "was this any content-safety rejection?"
tests `906001 ≤ code < 906100` rather than enumerating categories, so a new category must land
inside its own group. Allocate within a group; never append to the end of the block.

### Policy codes: `960000`–`964999` shipped, `965000`–`969999` reserved for users

A policy — WSO2's or a customer's — will sometimes reject for a reason no reserved block
covers: a translation failure, a domain rule, a tenant-specific check. Neither belongs in a
reserved range,
so the `96xxxx` space carries both, split so the two can never collide:

| Sub-block | Who allocates |
|---|---|
| `960000`–`964999` | **WSO2-shipped policies.** One base code per policy, with `+1`/`+2` for that policy's own sub-conditions (e.g. `962000` invalid body, `962001` translation failed, `962002` provider stream error). |
| `965000`–`969999` | **Customer policies.** WSO2 does not allocate here, so a code placed here will not later collide with a product code. |

> **The split exists because the shipped policies needed it.** An earlier revision of this
> document reserved the whole of `960000`–`969999` for users and promised WSO2 would never
> allocate inside it — while the shipped policies were already using 44 codes from `960200` to
> `962401`. A customer following that promise and picking `960800` would have collided with
> `mcp-ratelimit`. Splitting the block was chosen over renumbering 44 client-visible codes
> across twenty policy modules; the user-facing guarantee is unchanged in substance, only
> narrowed to a sub-block WSO2 is committed to staying out of.

The block is deliberately outside every classification range, so a code in it is categorised as
`other`. That is the correct outcome rather than a limitation — the ranges encode an existing
taxonomy, and a condition it has no code for has no place in it. Reaching into a range to
borrow a label would mis-report the failure as an auth, throttling or upstream fault.

So the choice is: **if a reserved block has a code for the condition, reuse it; if none does,
allocate in `965000`–`969999`.** Do not invent a number outside both — a code in no range and no
reserved block is indistinguishable from a typo.

Subdivide the block per policy or per team rather than allocating one code at a time, and keep
the allocation somewhere a reviewer can find it:

```go
// Codes 965100-965199 are this policy's.
const (
    codeContractExpired   = "965100"
    codeRegionNotLicensed = "965101"
)
```

`965000`–`969999` is for deployment-specific conditions. A code that would be useful to every
user of the product belongs in the shared registry instead — raise it rather than allocating it
privately. Note `961000` is **not** available for this: it is `redirect`'s misconfiguration
code, which is exactly the collision the split above prevents.

---

## Where these are defined

Two places, for two audiences, pinned against each other by a test.

**Policies read the SDK**: `sdk/core/policy/v1alpha2/fault_codes.go` holds every code as a
string constant — `FaultCode*` for the reused codes, `GuardrailCode*` for content
rejections — alongside `FaultType*`, `FaultSource*` and the range boundaries. This is the file
to reach for when writing a policy. It exists because a policy lives in its own module and
cannot import an `internal` package, so before it the shipped catalogue re-typed nine shared
codes as string literals **seventy-one times**, with no declaration to check them against.

**The engine reads analytics**: `gateway/gateway-runtime/policy-engine/internal/analytics/constants.go`
holds the **integer** form, which analytics events need, plus the classification ranges.

Neither can import the other, so agreement is enforced by test rather than by construction.
`internal/kernel` is the only package that sees both: `TestSDKFaultCodesMatchAnalytics`,
`TestSDKClassificationRangesMatchAnalytics` and `TestGatewayFaultCodes` fail if the two drift.
The engine's own private constants (`internal/kernel/error_source.go`) are now aliases for the
SDK's, so the string form has exactly one declaration in the product.

Note the asymmetry, and that it is the direction to prefer: `constants.go` holds only the
reused codes. Codes this gateway defined — the 905xxx engine block and the 906xxx guardrail
sub-codes — are declared in the SDK alone, so for those there is no second copy and nothing to
keep in step.

The guardrail sub-codes are declared **only** in the SDK (`sdk/core/policy/v1alpha2`,
`GuardrailCode*`, mirrored as `GuardrailCode` in the Python SDK) — block bounds included. There
is no second copy in `constants.go`, so for this family there is no drift to guard against:
`TestGuardrailCodesLandInTheirOwnGroup` (SDK) checks group membership and uniqueness,
`TestGuardrailBlockCollidesWithNothingTheEngineEmits` (kernel) checks the block overlaps no
classification range and no engine code, and `test_guardrail_codes_match_the_go_sdk`
(Python SDK) checks the two languages spell every code the same way.

Other policy codes have no such home yet: a policy in a separate module repeats the literal.
That is a real gap — see the note at the end of [fault-policies.md](fault-policies.md).

---

## Adding a code

1. **Does a reserved block already have one for this condition?** Check the tables above. If
   so, use it — do not allocate a parallel code for the same thing.
2. **Which category does the failure belong to?** If it is an upstream, auth, throttling or
   guardrail failure, the code must fall inside that range or it will be categorised as
   `other`.
3. **Only if neither applies**, allocate a new code: `905xxx` for a condition inherent to the
   gateway, `96xxxx` for one specific to a deployment. Both classify as `other`.
4. Declare it in the SDK (`fault_codes.go`), derive it where it is used, and extend
   `TestGatewayFaultCodes`.

### Two codes deliberately not reused

`900967` is a control-plane internal-error code, and reaches a data-plane body only through one
policy's bug. It is not a data-plane code.

The control-plane code set as a whole is not a source for these: many of its codes carry more
than one meaning, and where it overlaps the data plane the two frequently disagree. The
data-plane codes above are a smaller, internally consistent registry; that is the one to reuse.
