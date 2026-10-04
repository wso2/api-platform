# Fault policies

**Fault policies** are an ordered list of policies attached to an API that run **only when a
request fails**. Use it to act on failures — notify an external system, record the fault, or
change what the client receives — without touching the success path.

It exists because a rejected request previously ended the story: a guardrail returned its status
and nothing else happened. Customising the response body was possible; *doing something* about
the failure was not.

---

## Quick start

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: orders-api-v1.0
spec:
  displayName: Orders API
  version: v1.0
  context: /orders/$version
  upstream:
    main:
      url: http://orders-backend:8080
  operations:
    - method: POST
      path: /submit
  policies:
    - name: regex-guardrail          # rejects bad payloads with 422
      version: v1
      params:
        request:
          enabled: true
          pattern: "^[A-Za-z0-9 ]+$"

  faultPolicies:                # run only when the request fails
    - name: log-message
      version: v1
      params:
        fault:                    # a fault entry takes a `fault` block
          logLevel: error
    - name: set-headers           # annotate the error response
      version: v1
      params:
        fault:
          headers:
            - name: x-fault-handled
              value: "true"
```

A `POST /orders/v1.0/submit` rejected by the guardrail now returns its 422 **and** invokes the
fault policies: the failure is logged and the response is annotated. A successful request runs
neither entry.

A fault entry is configured with a `fault` block, not `request` or `response` — those carry
success-path semantics, and a policy attached as a fault entry never sees them.

---

## What runs, and when

Entries are policies that implement the **`OnFault`** contract. That contract is what makes a
policy usable here: it is checked when the route's chain is built, and an entry without it is
dropped with an error in the gateway log rather than dispatched through some other hook.

```go
func (p *MyFaultPolicy) OnFault(
    ctx context.Context, errCtx *policy.FaultContext, params map[string]interface{},
) *policy.FaultResponse
```

The return is a **`*FaultResponse`**, not a response action. The response-phase oneof asks a
policy to choose between forwarding the upstream response and replacing it — a choice that does
not arise here, because there is no upstream response to forward. The response is already an
error, and every entry is editing the same object.

Return **`nil`** to change nothing, which is what a notifier, audit sink or metrics entry does.
Otherwise set only the fields you mean to change:

| Field | Effect |
|---|---|
| `StatusCode *int` | Overrides the error's status. `nil` keeps it. |
| `Body []byte` | Replaces the body. `nil` keeps it, `[]byte{}` clears it. Setting it also switches the gateway's own protocol formatting off for this response. |
| `Fault *FaultDetails` | Re-describes the failure for a renderer or a later entry. |
| `HeadersToSet` / `HeadersToAppend` / `HeadersToRemove` | Applied **over** the error's existing headers rather than replacing them. |
| `AnalyticsMetadata` / `DynamicMetadata` / `AnalyticsHeaderFilter` | As on the response path. |
| `Final bool` | Stops the rest of the fault chain. |

One field from `ImmediateResponse` is deliberately absent: `Headers` replaces the whole map,
where an entry annotating an error wants to add a header without discarding the ones the error
already carries. `Final` is separate
from the body and status because ending the chain and replacing the response are different
decisions that returning an `ImmediateResponse` would fuse into one.

#### Python policies

A Python policy becomes eligible the same way, by implementing `FaultPolicy`:

```python
from apip_sdk_core import FaultContext, FaultPolicy, ExecutionContext, FaultResponse, ProcessingMode

class FaultLoggerPolicy(FaultPolicy):
    def mode(self) -> ProcessingMode:
        return ProcessingMode()  # participates in no phase

    def on_fault(
        self, execution_ctx: ExecutionContext, ctx: FaultContext, params
    ) -> FaultResponse | None:
        ...
```

The executor reports the capability when the policy is initialised, and the gateway uses it to
decide whether the policy may be attached to the fault chain — so a Python policy without
`on_fault` is dropped with the same log line as a Go one, rather than reaching the fault path
and failing there.

`FaultPolicy` is independent of every other interface. A notifier can implement it and nothing
else, leaving `mode()` fully SKIP, and it will cost nothing on the normal path. `ctx` carries the
response fields directly — `ctx.response_status` and `ctx.response_body` read exactly as they do
in `on_response_body` — with the fault fields (`ctx.policy`, `ctx.original_status`, `ctx.error`,
`ctx.response_committed`, `ctx.route_key`) alongside them. It does **not** subclass
`ResponseContext`: the names match so a handler reads the same values, but the types are distinct,
because a fault handler reads a response that has already failed and cannot forward it. Returning
`None` leaves the error untouched.

### What your entry can read

`OnFault` receives an `FaultContext`: the response the client is receiving and the request that
produced it, plus a typed description of why the flow is running. The response fields are declared
on the type itself rather than inherited from `ResponseContext`, so what a handler may read is
exactly what is listed on it.

**The request identity is the same whatever phase failed.** Authority, scheme and vhost live on
the two request-phase contexts and on neither response-phase one, and the resolved upstream target
lives on all four — so the gateway supplies each from whichever phase context holds it, rather
than from the phase that happened to fail. A rejection in the request header phase therefore
reports the backend the request was *bound for*, with `Upstream.Response` left nil because the
request never got there. That nil is load-bearing: an empty snapshot would read as "the backend
answered with status 0".

| | Available |
|---|---|
| Response status, response headers | always |
| **The error body** | always |
| Request headers, path, method, body | always |
| Request authority, scheme, vhost | always |
| The route's resolved upstream target (`Upstream.Name`/`URL`/`BasePath`) | always |
| The upstream response snapshot (`Upstream.Response`) | only when the request reached the backend |
| API name, version, kind, context, operation path; request id | always |
| Authenticated subject, auth type, issuer | always (unset when authentication itself failed) |

Because the error body is always present, an entry can **transform** the error — wrap it, lift
fields out of it — not only replace it. That needed an opt-in under the previous contract.

### Why the fault chain is running

Typed fields on the context, not string metadata keys:

| Field | Meaning |
|---|---|
| `Fault.Code`, `Fault.Type` | **what failed**, as a stable identifier and a class |
| `Policy`, `PolicyVersion` | **which policy caused the failure**; empty when none did |
| `PolicyPhase` | **which phase that policy was in** — `request_headers`, `request_body`, `response_headers`, `response_body`; empty when `Policy` is |
| `OriginalStatus` | the status before a policy changed it; 0 when unchanged |
| `ResponseCommitted` | the client already has the response — see [streaming](#failures-during-a-streamed-response) |
| `RouteKey` | the matched route |

`Policy` is what lets a notification say *"the word-count guardrail rejected this response"* rather
than *"a 422 happened"*. It is **empty** when no policy caused the failure — an unreachable upstream,
for example — because naming one there would be misleading. Treat emptiness as "not caused by a
policy", not as missing data.

`PolicyPhase` completes that sentence: *which policy, which version, doing what.* It is the phase the
named policy was executing in, and it follows `Policy` exactly — **empty whenever `Policy` is**, for
the same reason. A router failure or an engine error names no policy, so there is no policy phase to
name; a failure raised mid-stream names neither, because once chunks are flowing the engine no longer
knows which entry interrupted them. The values come from `policy.PolicyPhase*` in the SDK, which are
the same strings the Python SDK's `ExecutionPhase` carries.

There is deliberately **no `Trigger` or `Source` field**, and `PolicyPhase` is not one in disguise.
The distinction is which question is being answered. `PolicyPhase` is *attribution* — it belongs
beside `Policy` and describes the policy that failed. A `Source`/`Trigger` field would report **where
the gateway noticed the failure**, which is engine routing state: it decides whether this chain runs
at all, is already settled by the time a handler is called, and invites handlers to branch on
internals. That is why `PolicyPhase` is empty rather than falling back to the engine's current phase
when no policy is involved — reporting one there would quietly turn it into the field we did not
add. Everything else a handler needs is answerable from what is there:

| Question | Read |
|---|---|
| Was this the router or a policy? | `Policy == ""` means no policy caused it |
| Which failure exactly? | `Fault.Code` — e.g. `101503` unreachable, `101504` timeout |
| What class of failure? | `Fault.Type` — `upstream`, `routing`, `internal`, a guardrail class |
| Can I still change the response? | `ResponseCommitted` |

### `Fault.Description` never reaches the client; `Fault.Guardrail` does

`Fault.Message` is the client-facing summary. `Fault.Description` is not: for a response
guardrail it is *the content the guardrail blocked*, so forwarding it would turn every
rejection into a disclosure of the thing that was stopped. No built-in renderer emits it, in
any shape. A fault policy still receives it and can put it in a notification, an audit sink or
a log; an operator who genuinely wants it in the client body asks for it with a
`${Description}` placeholder in an `error-formatter` template — a template the operator wrote
is the opt-in.

`Fault.Guardrail` is the opposite: it **is** returned, and the operator's `showAssessment`
parameter decides how much of it. The shipped guardrails attach the block on every intervention
— `interveningGuardrail`, `action` and `actionReason` name which guardrail fired and why, which
is safe to return — and fill in `assessments` only when `showAssessment` permits, because that
field carries the content the guardrail existed to stop.

So with `showAssessment` off you still get the block, minus `assessments`. The formatter makes
no safety judgement of its own; it renders what the policy chose to fill in. When fully
populated it is rendered in every shape:

```json
{
  "code": "906000",
  "type": "guardrail",
  "message": "Violation of applied word count constraints detected",
  "guardrail": {
    "interveningGuardrail": "word-count-guardrail",
    "action": "GUARDRAIL_INTERVENED",
    "actionReason": "Violation of applied word count constraints detected",
    "assessments": { "assessments": "Expected word count to be between 10 and 500 words." }
  }
}
```

Field names match the assessment envelope the shipped guardrails publish in their own bodies,
so a client that already parses that envelope reads this one too. For JSON-RPC the block travels
in the error object's `data`; for XML the assessments are carried as JSON text, because
`encoding/xml` cannot marshal an arbitrary map and dropping detail the operator opted into
would be worse than serialising it awkwardly.

`data` carries the `code` and `type` as well, for the same reason. A JSON-RPC error object's own
`code` is the protocol's — `-32602`, `-32603` — so the six-digit APIM code has nowhere else to
go, and without this an MCP caller would be the one caller that cannot see it:

```json
{"jsonrpc":"2.0","id":"call-7","error":{
  "code":-32602,
  "message":"Invalid MCP request params",
  "data":{"code":"960800","type":"validation"}}}
```

Yes, that is two fields named `code` in one document. They are different things at different
levels — the protocol's integer and APIM's string — and the names are kept because they match
the JSON and XML shapes, so `code` means the same thing in `data` here as it does at the top
level there. `error_id` and the guardrail block share the same slot when present, and
`description` never does.

There is deliberately no third "client-safe detail" field. A guardrail already has `Guardrail`
for what may be shown and `Description` for what may not, gated by a parameter that predates
this feature; adding another would mean two places to decide the same thing.

### `Fault.JSONRPC` carries what the gateway cannot derive

`Guardrail` is *domain* detail — what happened — so every renderer emits it. `Fault.JSONRPC` is
*protocol* detail: how the failure must be spelled on one particular wire. Only the JSON-RPC
shape consumes it; the JSON and XML renderers ignore it, correctly, since neither has anywhere
to put a JSON-RPC code.

It exists because two things are genuinely unavailable to the gateway:

| Field | Why the gateway cannot supply it |
|---|---|
| `Code` | It maps the HTTP status onto `-32600` (4xx) or `-32603` (5xx). `-32700` Parse error, `-32602` Invalid params and `-32000` Server error are not reachable from a status — and `-32700` and `-32600` are *both* 400s, so no status rule can tell them apart. |
| `ID` | The request id lives in the request **body**, and most errors are produced in the header phase before any body is read. |

A policy that parsed the request knows both, and says so:

```go
jsonRPCCode := -32602
return policy.ImmediateResponse{
    StatusCode: 400,
    Fault: &policy.FaultDetails{
        Code: "960800", Type: "validation", Direction: policy.DirectionRequest,
        Message: "Invalid MCP request params",
        JSONRPC: &policy.JSONRPCError{Code: &jsonRPCCode, ID: requestID},
    },
}
```

Both halves are independent and both may be omitted. An absent `Code` falls back to the
status-derived one; an absent `ID` renders as `null`, which JSON-RPC requires for a request
whose id could not be determined — guessing would be worse, since a wrong id correlates the
error with the wrong call.

This is the first of a per-protocol pattern rather than a special case for MCP. A SOAP fault
needs the same treatment for the same reason: `faultcode`/`faultsubcode` are no more derivable
from an HTTP status than a JSON-RPC code is.

### Which failures trigger it

| Failure | Triggered? |
|---|---|
| A policy or guardrail rejects the request (401, 403, 422, 429 …) | ✅ Always |
| A guardrail rejects the **response** (`word-count`, `content-length`, `regex` …) | ✅ Always |
| A policy **chain fails to execute** (500) or a body exceeds the size ceiling (413) | ✅ Always — side effects only, see below |
| The backend is unreachable or times out (503, 504) | ⚙️ Only when `handle_upstream_faults` is on |
| **The backend returns an error status** (4xx or 5xx) | ⚙️ Only when `handle_upstream_faults` is on — narrow further with a condition |
| An error raised while **streaming** a response | ⚠️ Side effects only — see [below](#failures-during-a-streamed-response) |
| No API matched the request (404 from the gateway) | ❌ Never — see [Limitations](#limitations) |
| Any successful response | ❌ Never |

Three things bound this: the **400 status floor**, fault policies being declared on the API in
the first place, and — for failures the gateway did not produce —
`policy_engine.fault_policies.handle_upstream_faults`, which is off by default.

### Which failures reach the chain, and how each says where it came from

A fault entry is told who produced the failure, in `FaultContext.Source`:

| `Source` | Meaning |
|---|---|
| `gateway` | a policy or guardrail rejection, or an engine failure — the gateway built this response |
| `router` | the proxy could not complete the attempt: connection refused, timeout, no healthy host |
| `backend` | the upstream returned this error status itself |
| `noRoute` | the request matched no API |
| `unknown` | the router sent no provenance — an older router, or a phase where it is not populated |

This is a distinction a status code cannot express: a backend answering `503` and the router
failing to reach it are both `503`. The gateway uses the proxy's own report of which actor
produced the response, so the two are told apart correctly rather than by threshold.

**A policy's own rejection always reaches the chain.** That is the contract a policy opts into
by declaring the fault, and it needs no configuration.

**Upstream and router failures are opt-in.** A backend `503`, a connection refused, a request
that matched no API — these reach your fault policies only when the engine is configured for it:

```toml
[policy_engine.fault_policies]
handle_upstream_faults = true          # default: false
```

Off by default, and the default is the point. Every previous generation of this gateway ran an
upstream error through the **response** policies, so switching them to the fault flow changes
where your existing mediation runs: a backend `503` that reaches a response transformer or a
response guardrail today would stop reaching it. That is a behavioural change, so you ask for it
rather than inherit it.

With it **off**, an upstream or router error is an ordinary response — your response policies run
over it exactly as before, including the analytics collector, and your fault policies do not run.
With it **on**, the fault policies handle it **instead of** your response policies — none of them
run over that response — and the collector runs at the end of the fault chain, where the
failure's code and class are resolved.

A failure is recognised by its status alone — `>= 400`. That is enough for an HTTP-shaped API and
deliberately not enough for GraphQL or A2A, which report a failure inside a `200` body; those
need body inspection and are not covered yet.

**Backend errors are included, not filtered.** With `handle_upstream_faults` on, a backend answering
`404` for a missing record or `500` for its own internal failure is that API behaving as its own
contract says — but you are the one running a gateway in front of it, and frequently the one who
has to translate its errors for your callers or hear about it when it starts failing. So the
failure reaches your chain, and narrowing is yours to declare rather than the gateway's to
assume:

```yaml
faultPolicies:
  - name: fault-notifier
    version: v1
    executionCondition: fault.Source == "gateway"   # skip the backend's own errors
```

Use `fault.Source != "backend"` to include `unknown` alongside gateway and router failures, which
matters if your router predates the provenance attribute.

**One thing the gateway does decide.** A body the backend sent is never rewritten. On a kind the
gateway formats, an upstream error document is treated as authored — the backend said it, so it
stands — while a bodyless upstream failure may still be rendered.

**Engine failures are reported, not reshaped.** Their body carries the correlation id that the
`x-error-id` header and the internal error log also carry, and that id is the whole mechanism for
tracing a 500 back to its cause — re-rendering the body would drop it. Fault policies still run,
so a notifier reports the failure with its code.

### An error status, or the policy's own word

A response reaches the fault chain when **either** holds:

- its status is **400 or above**, whoever set it, or
- the producing policy **declared** it with `IsFault: true`

```go
return policy.ImmediateResponse{
    StatusCode: 401,                   // 400+ — a fault on the status alone
    IsFault:    true,                  // and declared, which is what a migrated policy does
    Fault: &policy.FaultDetails{       // describes it, for a handler and a renderer
        Code: "900902", Type: policy.FaultTypeAuthentication, Message: "…",
    },
}
```

Two rules rather than one, because each covers what the other cannot.

**The status alone** misses a failure that answers `200` — a GraphQL error puts the failure in
the body and the status says nothing about it. Only the policy can know that.

**The declaration alone** misses every rejection from a policy written before this contract,
and misses upstream and router errors entirely, since neither has a policy to declare on its
behalf. That gap was not hypothetical: two shipped policies rejected with a 4xx and declared
nothing, so their rejections reached no handler —

- `content-length-guardrail`, on a request body it could not buffer — every *other* rejection
  in the same policy declared correctly
- one guardrail's streaming path, converting its own validation result

Neither was a decision; both were sites a migration missed. Under the combined gate no policy
needs migrating for its genuine rejections to be handled.

| Status | `IsFault` | Result |
|---|---|---|
| `≥ 400` | either | a failure — the status carries it |
| `< 400` | `true` | a failure — only the policy could say so |
| `< 400` | `false` | **not** a failure |

`Fault` is independent of the decision throughout. It says *what* failed, not *whether*
something did, so a deliberate sub-400 response can still carry an error-shaped body — a
redirect, an auth challenge — without entering the flow.

Whether a policy *supplied* a status is a separate question, answered by `StatusCode` being
non-nil, and it is what the trace uses to distinguish an overridden status from the upstream's
own. That question is computed independently and is unaffected by this gate.

#### What it costs

A deliberate non-failure carrying an error-shaped status now reaches the chain — `respond`
answering a configured `404`, `interceptor-service` applying an external interceptor's own
`503`. That is real, and it is narrowed on the **entry** rather than in the policy:

```yaml
faultPolicies:
  - name: fault-notifier
    version: v1
    executionCondition: 'fault.Status >= 500'     # skip configured 4xx responses
```

Which is the same place a deployment already narrows backend errors, and for the same reason:
the gateway cannot know which of your error-shaped responses you consider failures, but your
configuration can.

#### The streaming exception

`TerminateResponseChunk` keeps the declaration alone. Its status went out with the headers long
before the chunk and for a streamed response is almost always `200`, so reading it would
classify every mid-stream guardrail intervention as a success. See
[Deciding whether ending a stream is a failure](#deciding-whether-ending-a-stream-is-a-failure).

#### Upstream and router errors skip the response policies

With `handle_upstream_faults` on, an upstream or router error runs **only** the fault policies.
None of the API's response policies run over it — not the header phase, not the body phase — so
a backend `503` no longer reaches a response transformer, a response guardrail, or a header
policy such as CORS. Anything that response still needs, declare as a fault policy.

A backend that streams its error response (`text/event-stream`, chunked) is buffered, so the
fault policies see the whole body.

The analytics collector records from the end of the fault chain, where the failure's code and
class are resolved.

### Failure classes for `Type`

`Type` is the one field a caller can branch on without a code table, so it is worth spelling
consistently. The values live in the SDK — `policy.FaultType*` in Go,
`apip_sdk_core.policy.v1alpha2.FaultType` in Python — and a policy should use a constant rather
than a new string.

Classes a **policy** sets, describing why it rejected the exchange:

| Constant | Value | Use it when |
|---|---|---|
| `FaultTypeAuthentication` | `authentication` | the caller could not be identified — credential missing, malformed, expired, unverifiable |
| `FaultTypeAuthorization` | `authorization` | the caller was identified but is not permitted — scope, ACL, subscription, tool or resource |
| `FaultTypeThrottling` | `throttling` | a rate limit or quota was reached |
| `FaultTypeGuardrail` | `guardrail` | content inspection rejected the request or response — pair it with `Fault.Guardrail` |
| `FaultTypeValidation` | `validation` | the message did not satisfy a required shape — unparseable body, missing field, schema mismatch |
| `FaultTypeMediation` | `mediation` | a transformation, translation or rewrite step failed |
| `FaultTypeConfiguration` | `configuration` | the policy's own parameters are unusable — an *operator* error, so usually a 5xx |

Classes the **gateway** sets, for failures no policy produced. A policy has no reason to use
these, but they appear in client-visible bodies:

| Constant | Value | Emitted for |
|---|---|---|
| `FaultTypeUpstream` | `upstream` | no healthy host, connection reset, upstream timeout |
| `FaultTypeRouting` | `routing` | no API matched the request |
| `FaultTypeInternal` | `internal` | the gateway itself failed, e.g. a policy chain that could not run |
| `FaultTypeRequestSize` | `requestSize` | the request body exceeded the configured decompression ceiling |

The list is **canonical, not closed**: `Type` is a plain string and nothing validates it, so a
policy solving a problem none of these describes may use its own value — the same way it may
allocate its own code in the customer range. Check the list first, because two policies naming
one class differently makes the field useless for the consumers it exists for.

#### Diagnosing "I configured a fault policy and nothing happens"

Check the status first. A rejection below `400` is not a fault, however error-shaped its body:
a `302`, a `204` preflight, a `200` cache hit. That is the answer in most cases, and it is
visible in the response itself.

If the status *is* `400` or above, the failure is reaching the gateway by another route — see
the [trigger table](#which-failures-trigger-it) — or an `executionCondition` on the entry is
excluding it.

You can still narrow further with `executionCondition`:

```yaml
faultPolicies:
  - name: log-message                # every failure
    version: v1
    params:
      fault:
        logLevel: error

  - name: log-message                # 5xx only
    version: v1
    executionCondition: "response.ResponseStatus >= 500"
```

### Narrowing on the failure itself

A condition on a fault entry can read the failure, not just the request that caused it. These
are the `error.*` variables:

| Variable | Type | Notes |
|---|---|---|
| `fault.Source` | string | who produced it — see [the source table](#which-failures-reach-the-chain-and-how-each-says-where-it-came-from) |
| `fault.Code` | string | the stable identifier, e.g. `"906000"` |
| `fault.Type` | string | the failure class, e.g. `"guardrail"`, `"authentication"` |
| `fault.Direction` | string | `"Request"` or `"Response"` — which side was rejected |
| `fault.Message` | string | the client-safe summary |
| `fault.Policy` / `fault.PolicyVersion` | string | which policy failed; empty when none did |
| `fault.PolicyPhase` | string | the phase it was in, e.g. `"response_body"` |
| `fault.Status` | int | the response status |
| `fault.OriginalStatus` | int | the status before a policy changed it, `0` if none did |
| `fault.RouteKey` | string | the matched route |
| `fault.ResponseCommitted` | bool | true only mid-stream |

```yaml
faultPolicies:
  # Page on-call for infrastructure failures, not for a caller sending a bad token.
  - name: set-headers
    version: v1
    executionCondition: 'fault.Type == "upstream" || fault.Type == "internal"'
    params:
      fault:
        headers:
          - name: x-page-oncall
            value: "true"

  # Audit content rejections separately, and only outbound ones.
  - name: log-message
    version: v1
    executionCondition: 'fault.Type == "guardrail" && fault.Direction == "Response"'
```

#### Which variables are always populated, and which are not

This is the one thing to get right when writing a condition, because getting it wrong produces
**silence rather than an error**.

`fault.Status`, `fault.Source` and `fault.RouteKey` are always set — the gateway derives them
from the exchange itself. Everything under `fault.Code`, `fault.Type`, `fault.Direction`
and `fault.Message` comes from the *producing policy's* `Fault`, and is
empty whenever nothing described the failure. Three cases where that happens, and none of them
is unusual:

- **A policy that does not describe its rejection.** Any policy released before `FaultDetails`
  existed, and any policy that simply chose not to. Its 4xx still reaches your chain — the
  status is what routes it — but there is no code to match on.
- **A backend error**, when `handle_upstream_faults` is on and it therefore reaches the chain at all.
  The gateway deliberately describes nothing: another service's 500 is not the gateway's to
  classify, and a synthesized code would claim knowledge it does not have.
- **A rejection at a status the policy set with no `Fault` at all**, which is the same shape as
  the first case seen from the engine's side.

So a condition like `fault.Code == "900902"` matches *only* rejections that were described. It
does not fail, log, or 500 — it reads false, and the entry does not run:

```yaml
# Fires for every gateway failure, described or not.
executionCondition: 'fault.Source == "gateway"'

# Fires only for failures a policy described with this exact code. An older policy's 401
# reaches the chain and is skipped here.
executionCondition: 'fault.Code == "900902"'
```

**Rule of thumb:** narrow on `Status` and `Source` when the entry must see every failure of a
kind; narrow on `Code` or `Type` when you are picking *among described* failures
and skipping the rest is what you want.

Worth noting this is not a regression from the previous contract. Under the explicit opt-in an
undescribed rejection did not reach the chain at all, so a code-based condition never ran for
it either — the entry stayed silent then too. What changed is that unconditional entries and
`Status`/`Source` conditions now fire where they previously could not.

#### Two smaller notes

- **`fault.Description` is deliberately absent.** For a guardrail rejection it holds the content
  that was blocked, which is why every renderer withholds it; it is available to a fault
  *policy*, not to a condition. `Fault.Guardrail` is likewise policy-only. Use `fault.Policy`
  to branch on which guardrail acted, and `fault.Code` on what kind of intervention it was.
- **These variables exist in every phase, zeroed.** A condition on a normal (non-fault) policy
  that mentions `fault.Type` reads empty rather than failing — which is what it means there.
  A missing variable would be an evaluation error, so they are always supplied.

### Ordering

Entries execute **in the order declared**, top to bottom. Repeating a policy is allowed and is
often what you want:

```yaml
faultPolicies:
  - name: set-headers              # tag the response
    version: v1
    params:
      fault:
        headers:
          - name: x-fault-tagged
            value: "true"
  - name: log-message              # and record it
    version: v1
    params:
      fault:
        logLevel: error
```

### Per-operation fault policies

An operation can declare its own fault policies in addition to the API's:

```yaml
spec:
  faultPolicies:
    - name: log-message                  # every operation's failures
      version: v1
      params:
        fault:
          logLevel: error

  operations:
    - method: POST
      path: /payments
      faultPolicies:
        - name: set-headers              # this operation's failures, additionally
          version: v1
          params:
            fault:
              headers:
                - name: x-payments-fault
                  value: "true"
```

**Both levels run — the operation does not override the API.** A fault entry is a handler rather
than a setting, so two of them are additive: an operation-specific alert and an API-wide audit hook
both want to fire, and neither is a "more specific value" that should suppress the other.

**Operation-level entries execute first**, then API-level ones. That matches the order response
policies execute in: the operation is the more specific scope, so it acts on the error before the
API-wide handler sees it. For the example above, a failure on `POST /payments` notifies the payments
on-call hook, then the SIEM; a failure anywhere else notifies only the SIEM.

An operation with no fault policies of its own inherits the API's unchanged.

---

## Guarantees

- **It never runs on a successful response.** Fault entries are held separately from
  `policies`, so they cannot execute in the normal request or response phases.
- **It runs at most once per request.** A failure inside the fault chain cannot re-enter it.
- **A failing entry never fails the request.** If an entry errors, the client still receives the
  original error rather than a worse one; the failure is logged.
- **A misconfigured entry does not break the API.** An entry naming an unresolvable policy is
  rejected at deploy time; one that fails to instantiate at runtime is skipped with a log, and the
  API's normal traffic is unaffected.

---

## Changing the error response

A fault entry decides what happens by what it returns from `OnFault`.

### Annotate the error and forward it

```go
return &policy.FaultResponse{
    HeadersToSet: map[string]string{"x-fault-ref": "contact-support"},
}
```

Status, headers and body can all be changed this way, and the change is visible to entries later
in the fault chain.

### Replace the error outright

```go
status := 503
return &policy.FaultResponse{
    StatusCode:   &status,
    HeadersToSet: map[string]string{"content-type": "application/json"},
    Body:         []byte(`{"error":"unavailable","requestId":"..."}`),
    Final:        true,
}
```

`Final: true` **ends the fault chain** — entries after it do not run. It is opt-in and separate
from the body: replacing a body without setting it leaves the remaining entries free to annotate
what you wrote, which is usually what an operator listing several handlers wants.

Note the headers are *operations*, so the ones the error already carries survive alongside the
content-type set here — a `WWW-Authenticate` on an auth rejection, for instance. The old contract
expressed replacement as an `ImmediateResponse`, whose whole-map `Headers` discarded them.

### Do nothing to the response

```go
return nil
```

The usual case for a notifier: publish an event, leave the client's error exactly as it was. `nil`
says that directly, where an empty struct had to be inspected field by field to mean the same
thing.

> **If your fault policies appear to do nothing**, check the gateway log first:
>
> ```
> [chain-build] skipping fault-policies policy that does not implement OnFault
> ```
>
> The entry deployed successfully — the policy exists and its parameters validated — and was then
> dropped when the route's chain was built. The contract is checked at chain build, not at
> deployment, which is why the log is where the answer lives.

---

## Protocol-specific error bodies

Some clients cannot read the gateway's default JSON error body at all. An MCP client speaks
JSON-RPC, so `{"error":"Unauthorized"}` is not an error object it is required to understand; a SOAP
client needs a fault envelope matching the version it used.

So the gateway can supply one itself, **for the API kinds whose callers cannot read anything
else, when no policy authored a body**. A policy that rejects a request describes the failure in
its error object — code, type, message — and leaves the body alone; the gateway renders that into
whatever shape the caller's protocol needs.

### Shaping an error body yourself

Since the gateway does not format your kind, a fault policy is how an error body gets shaped —
and it is the supported way to do it. **Any** fault policy that sets a response body has decided
what the client receives.

The examples use `error-formatter`, a general-purpose formatter policy. It is **not part of the
gateway distribution** — it ships with the policy catalogue — so check it is in your build before
copying these verbatim.

The examples use `error-formatter`, a general-purpose formatter policy. It is **not part of the
gateway distribution** — it ships with the policy catalogue — so check it is in your build before
copying these verbatim.

```yaml
faultPolicies:
  - name: error-formatter               # authors a body -> gateway formatting stands down
    version: v1
    params:
      template:                         # structured, so it cannot emit malformed JSON
        errorRef: ${Code}
        detail: ${Message}
        status: ${Status}               # a number, because the leaf is exactly the placeholder
```

For a client that needs a specific well-known envelope there are presets:

```yaml
faultPolicies:
  - name: error-formatter
    version: v1
    params:
      preset: openai      # openai | canonical | jsonrpc | soap11 | soap12
```

Each preset carries its own `Content-Type`. `soap11` and `soap12` matter most: those two shapes
are **not** among the gateway's own renderers, so a policy is the only way to return a conformant
SOAP fault today.

> **Placeholders are `${Field}`, not `{{.Field}}`.** The gateway renders an API's whole spec
> through `text/template` at deploy time to resolve `{{ env }}` and `{{ secret }}`, so a
> `{{...}}` placeholder in a policy parameter is consumed by *that* pass and reaches the policy
> as the literal `<no value>`. This applies to any policy parameter, not just this one.

**There is no ordering requirement.** The signal is what your policy said, not where it ran — so
you cannot break this by declaring entries after it.

### What counts as "authored"

This is about the policy that **produced** the failure — the auth policy or guardrail that
rejected the request — not about fault policies. It decides whether the gateway may render over
the body that policy wrote.

The rule is one line: **describe an error and the gateway renders it; describe nothing and your
body stands.** It applies only on a kind the gateway formats; on every other kind nothing is
rendered regardless, so your body always stands.

| The rejecting policy returns | Treated as |
|---|---|
| `Fault` set, no body | **not authored** — the gateway renders |
| `Fault` set, plus a body | **not authored** — the body is a *fallback*, the gateway renders |
| a body, no `Fault` | **authored** — left exactly as-is |
| an explicitly empty body, no `Fault` | **authored** — the client gets nothing, as you asked |

Omitting `Fault` is therefore how a policy keeps a body it wrote — the custom-formatter path.

#### Why a body alongside a description is a fallback

Because of older gateways. One that predates this feature ignores `Fault` entirely and sends
whatever is in the body, so a policy that describes its failure and leaves the body nil produces
an **empty error response** there. A policy that must run on both writes both:

```go
return policy.ImmediateResponse{
    StatusCode: 401,                     // 400+ — the gateway reads this as a fault
    Headers:    map[string]string{"content-type": "application/json"},
    Fault:      err,                     // what a current gateway renders from
    Body:       p.legacyErrorBody(),     // what an older one sends instead
}
```

**Keep the body your policy already wrote.** Migrating to `Fault` does not mean deleting it: the
body you had is what an older gateway has always sent, so keeping it means that path does not
change at all. Inventing a *new* fallback shape would change what old gateways emit — churn on
the one path the fallback exists to protect.

If the fallback suppressed rendering, it would defeat its own purpose — every MCP API back to
REST-shaped JSON, every SOAP API back to whatever the policy happened to write. So it does not.

#### A body written by a fault policy IS authored

Unconditionally, even when the failure was described. A fault entry runs *after* the
description and with the `FaultContext` in hand, so its body is the later decision rather than a
stand-in — which is what lets an entry transform the error (wrap it, lift fields out of it)
without the formatter overwriting the result.

#### Requests that matched no route

A request that reached no API has no kind, so nothing in `supportedKinds` can name it, and the
reply Envoy already wrote is what the client gets.

#### Router failures

A router failure carries a body Envoy wrote (`no healthy upstream`, as plain text). It is not
empty, and no policy authored it — so on a formatted kind it *is* replaced. A rule based on "is
the body empty" would have left it, which is precisely the body a SOAP or MCP client cannot
parse.

### It never changes the status

Collapsing a 401 into a SOAP-conformant 500 would destroy the signal your analytics and client
retry logic depend on.

### Router failures are described too

An unreachable upstream, a timeout or a no-healthy-host reply has no policy behind it, so nothing
would otherwise describe it. The gateway fills that in from the proxy's own account:

| Failure | `code` | `type` |
|---|---|---|
| no healthy upstream | `303001` | `upstream` |
| could not connect / reset | `101503` | `upstream` |
| upstream timeout | `101504` | `upstream` |
| no API matched | `900906` | `routing` |

### Which API kinds can declare fault policies

| Kind | Declares fault policies | API-level field | Operation-level field |
|---|---|---|---|
| `RestApi` | ✅ | `faultPolicies` | `operations[].faultPolicies` |
| `Mcp` | ✅ | `faultPolicies` | `operations[].faultPolicies` |
| `LlmProvider` | ✅ | `globalFaultPolicies` | `operationFaultPolicies` |
| `LlmProxy` | ✅ | `globalFaultPolicies` | `operationFaultPolicies` |
| `WebSubApi` | ❌ | no policy support at all yet | — |

Both levels run, and **the operation-level entries run first**. They are additive rather than an
override: a fault entry is a handler, not a setting, so an operation-specific notification and an
API-wide audit hook both fire, and neither is a "more specific value" that should suppress the
other. Each entry is tagged with the scope that attached it (`attachedTo: route` or `api`), so a
handler can tell which level configured it.

The LLM kinds synthesize their operations rather than reading them from the spec, so an
`operationFaultPolicies` entry attaches by matching its `paths`/`methods` — the *same* mechanism
the normal `operationPolicies` list uses, including registering the paths it names and expanding
a wildcard operation against the template's declared resources. An operator who knows where
`operationPolicies` land does not have to learn a second set of rules.

That mechanism registers the named path, which matters because it is the only way a specific
path gets an operation at all: an `allow_all` provider derives nothing but catch-all `/*` routes,
so `/chat/completions` has no operation to attach to until something materializes one. Doing so
opens no new traffic — the expansion is bounded by the provider template's own resources, the
catch-all already served that path, and API-level policies apply either way. Where an
access-control exception denies a path for normal policies, it denies it here too, so the fault
list cannot reach a route access control meant to refuse.

**The name follows the kind's normal-path field, not the other way round.** `RestApi` and `Mcp`
call their ordinary list `policies`, so the fault list is `faultPolicies`. The LLM kinds call
theirs `globalPolicies` and `operationPolicies`, so theirs is `globalFaultPolicies`. One scope,
two spellings, each consistent with the field beside it — the reverse of the earlier
arrangement, where every kind used the LLM spelling and a `RestApi` ended up with `policies` and
`globalFaultPolicies` side by side.

Only `RestApi` has an operation scope. An MCP spec describes tools, resources and prompts
rather than HTTP operations, and the LLM kinds express operation scope as a flat
`operationPolicies` list keyed by path — neither has a nested operation to hang a fault entry
on, so both take the API-level list only.

Everything downstream of the declaration is **kind-agnostic**. Each non-REST kind is
normalised into a `RestApi` before route chains are built, so a kind only has to carry the list
across that conversion; chain building, the engine, and protocol rendering already work for
every kind. That is also why the JSON-RPC error shape has always applied to MCP APIs with no
configuration at all.

### Where the codes come from

`code` is a six-digit numeric string, reused from WSO2 APIM wherever APIM already has a code for
the condition — `900902` for missing credentials, `101503` for a failed upstream connection,
`303001` for no healthy host. It is not a new namespace.

Every guardrail rejection carries a code from one block, `906000`–`906399`: `906000` when the
guardrail has nothing more specific to say, or `906001` for hate, `906003` for self-harm,
`906201` for a word-count limit — via `policy.GuardrailCode*`. See
[Guardrail codes](error-codes.md#guardrail-codes-906000906399).

One thing to know before adding a code: the classifier assigns a fault **category** by testing
which range the number falls in, so a code outside the relevant range is silently recategorised as
`other`. And for a condition APIM has no code for, `960000`–`969999` is reserved for
deployment-specific codes — WSO2 never allocates there. Full reference, including the ranges,
the block map and how to pick a code: **[Gateway error codes](error-codes.md)**.

### Deciding whether ending a stream is a failure

A policy ends a stream by returning `TerminateResponseChunk`, and it does that for two unrelated
reasons: a guardrail intervening, and a clean close after the upstream's final event. They look
identical from outside.

The buffered actions declare too, and here there is not even a status to fall back on — it went
out with the headers, and for a streamed response it is almost always `200`. So an
**undeclared** termination is treated as *not* a failure, and a guardrail that wants to be
noticed sets `IsFault: true`, exactly as it would on a buffered rejection:

```go
return policy.TerminateResponseChunk{
    Body:    []byte("data: {\"error\":\"blocked\"}\n\n"),
    IsFault: true,
    Fault:   &policy.FaultDetails{Code: "906000", Type: "guardrail", Message: "…"},
}
```

Defaulting the other way would fire a notification on every successful stream, which breaks the
signal outright — worse than missing an undeclared intervention, which merely fails to add one.

This matches the buffered rejection default — silence means "not a failure" on both paths — so
there is one rule to learn rather than two, and it is the same rule for the same reason: the
gateway does not guess.

---

## Limitations

- **A request that matches no API is not covered.** The gateway answers unmatched paths before any
  API is identified, so there is no API — and therefore no fault policies — to select. Use
  gateway-level access logs for those.
- **A streamed response can be observed but not changed.** See
  [Failures during a streamed response](#failures-during-a-streamed-response).
- **A router too old to report response provenance reports `Source: unknown`.** The gateway
  distinguishes a router failure from a backend error using the proxy's own report of which actor
  produced the response. Without it, the failure still reaches your chain — it simply cannot say
  which of the two it was, so a condition testing `fault.Source == "gateway"` will not match it.
  Policy and guardrail rejections are unaffected, since those need no provenance.
- **`WebSubApi` cannot declare fault policies**, because it does not accept `policies` either —
  there is no schema for it yet. That is a gap in WebSub's policy support generally, not in the
  fault flow.
- **A non-empty fault chain turns response-body processing on for the route.** The fault path runs
  in that phase, so it has to. Declaring no fault policies avoids that cost entirely.
- **Upstream and router failures reach a declared chain only when
  `policy_engine.fault_policies.handle_upstream_faults` is on** (default: off) — with it off they stay on
  the response policies, as in every previous generation of this gateway. With it on, the
  response policies are skipped for them and every error
  status at or above 400 reaches the chain, including the backend's own, and an entry that should
  see only gateway failures must say so with an execution condition — see
  [Which failures reach the chain](#which-failures-reach-the-chain-and-how-each-says-where-it-came-from).

---

## What analytics records about a failure

When analytics or traffic logging is enabled, the collector runs as the last entry of every
API's fault chain and adds the resolved failure to the request's analytics event. It changes
none of the fields the event already had — `errorType`, `error.errorCode` (the HTTP status) and
`error.errorMessage` keep their meaning — and adds these to the `error` object:

| Field | Carries |
|---|---|
| `wso2ErrorCode` | The fault code, e.g. `900902`. Absent when the failure had none, such as a router failure. |
| `type` | The failure class (`authentication`, `guardrail`, `upstream`, …). |
| `direction` | `Request` or `Response` — which side was rejected. |
| `summary` | The client-facing message. |
| `policy`, `policyPhase` | The policy that failed and its phase. Absent when no policy did. |
| `source` | `gateway`, `backend`, `router` or `noRoute`. |
| `originalStatus` | The backend's status before a policy changed it. |
| `guardrail` | `{name, action, reason}` for a guardrail rejection. |
| `jsonRpcCode` | The JSON-RPC error code, for MCP and A2A. |

A word-count guardrail rejecting a backend response:

```json
"errorType": "OTHER",
"error": {
  "errorCode": 422,
  "errorMessage": "UNCLASSIFIED",
  "wso2ErrorCode": 906201,
  "type": "guardrail",
  "direction": "Response",
  "summary": "Violation of applied word count constraints detected",
  "policy": "word-count-guardrail",
  "policyPhase": "response_body",
  "source": "gateway",
  "originalStatus": 200,
  "guardrail": {
    "name": "word-count-guardrail",
    "action": "GUARDRAIL_INTERVENED",
    "reason": "Violation of applied word count constraints detected"
  }
}
```

The fault's `Description` and a guardrail's `Assessments` are never recorded: for a guardrail
they hold the content it blocked, and analytics events leave the gateway. A failure declared on a
status below 400 gets an `error` object too, with `errorCode` set to that status and
`errorMessage` `UNCLASSIFIED`.

## Operational guidance

Fault policies run exactly when things are already going wrong, so treat them as part of your
failure budget:

- **Keep it short.** Every entry adds work to the error path. Under a burst of guardrail
  rejections that cost is paid per rejected request.
- **Watch outbound calls.** `interceptor-service` makes a network call. A slow or unreachable
  destination extends the time your client waits for an error it is already going to receive.
  Give it a tight timeout.
- **Do not put the rejected content in the notification.** This matters most for guardrails: the
  payload a PII or prompt-injection guardrail blocked is precisely the data you do not want
  leaving the gateway. Send metadata — status, API, request id — not the body.
- **Validate the destination.** A fault webhook is an outbound request from inside your network.
  Point it at a host you control and confirm it is not reachable through an internal-only
  address.

---

## See also

- Generated field reference: [REST API schemas](../rest-apis/gateway/schemas.md) — `faultPolicies`
  on the RestApi and Mcp specs, `globalFaultPolicies` on the LLM kinds.
- `gateway/it/features/fault-policies.feature` — executable examples of every behaviour described
  here.
