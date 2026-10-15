# OpenAI-Compatible Error Formatting for LLM APIs

## Status

Implemented, and on by default. A policy-engine configuration flag turns it off.

## Summary

The gateway can format gateway-produced failures on `LlmProvider` and `LlmProxy` routes as an
OpenAI-compatible HTTP error envelope. This allows OpenAI SDKs to parse authentication,
authorization, throttling, guardrail, routing, upstream, and internal gateway failures through
their normal API-error path.

The feature is enabled by default: an LLM client is an OpenAI SDK far more often than not, and it
cannot read the bodies the policy catalogue writes for REST APIs. Deployments whose clients parse
those bodies turn it off. When enabled, it changes only error bodies on matched LLM routes. It does
not change HTTP status codes, successful responses, or backend-authored error documents.

The canonical response is:

```json
{
  "error": {
    "message": "Invalid API key",
    "type": "authentication_error",
    "param": null,
    "code": "900901"
  }
}
```

The gateway may add two WSO2 extensions inside `error`:

- `error_id`, for correlation of policy-engine failures with gateway logs;
- `guardrail`, for client-safe guardrail intervention metadata.

These extensions are accepted by tolerant OpenAI SDK clients but are not fields in OpenAI's core
four-field HTTP error object. Deployments requiring strict schema validation must account for this
extension contract.

## Motivation

An LLM route can fail before its backend produces a response. Examples include:

- API-key authentication rejection;
- authorization rejection;
- gateway rate limiting;
- request or response guardrail intervention;
- request validation or request-size rejection;
- no healthy upstream;
- upstream connection failure or timeout;
- an internal policy-engine failure.

Historically, the body was whatever the rejecting policy or Envoy wrote. Common shapes included
plain text, ordinary REST JSON, empty bodies, and policy-specific guardrail documents. OpenAI SDKs
expect an `error` object and may otherwise surface an empty or unhelpful exception.

The gateway therefore needs one predictable envelope for failures it owns while preserving bodies
owned by the backend or explicitly authored by an operator's fault policy.

## Goals

- Make gateway-produced LLM errors readable by OpenAI SDKs.
- Preserve the original HTTP status code.
- Preserve a meaningful client-safe error message.
- Preserve stable WSO2 fault codes when present.
- Respect operator-authored fault-policy bodies.
- Preserve backend/provider error documents byte-for-byte.
- Leave already OpenAI-shaped policy bodies unchanged.
- Avoid buffering successful responses solely for error formatting.
- Keep the previous bodies available: disabling the feature restores them exactly.
- Reuse the gateway's existing protocol-aware fault-formatting pipeline.

## Non-goals

- Converting arbitrary provider errors into OpenAI errors.
- Making Anthropic or other provider responses look like OpenAI responses.
- Changing HTTP status codes or retry semantics.
- Synthesizing complete Chat Completions or Responses API success objects.
- Representing a gateway guardrail as though it were a model-generated refusal.
- Reformatting a response after a streamed response has already started.
- Formatting a request that matched no route, because it has no API kind.
- Decoding and rewriting encoded error bodies when no response-body policy requested them.

## Configuration

```toml
[policy_engine.llm_errors]
disable_openai_format = false
```

The default is `false`, so the envelope is on. Set `disable_openai_format = true` to send errors as
they are produced: the bodies policies write themselves and the router's own replies. Fault
policies run either way. The shipped config template and the Helm chart both default to `false`.

The key names the envelope rather than "the formatter": in the user docs "formatter" is the
`error-response-formatter` fault policy, which this setting does not affect.

The Helm value is:

```yaml
gateway:
  configs:
    policy_engine:
      llm_errors:
        disable_openai_format: false
```

At startup, `!disable_openai_format` is passed to `WithLLMOpenAIErrors`. Enabling it adds
`LlmProvider` and `LlmProxy` to the formatter's enabled API-kind set without removing the kinds
already enabled by the product, such as `Agent`.

## Applicability

Formatting is considered only when all of the following are true:

1. The feature is enabled.
2. The request matched an `LlmProvider` or `LlmProxy` route.
3. The response is a fault according to the gateway fault flow.
4. The request is not `HEAD`.
5. No higher-precedence author has made the final body decision.

Successful responses are never formatted.

## Wire Contract

### Core fields

| Field | Type | Source |
|---|---|---|
| `error.message` | string | `Fault.Message`; otherwise a message recovered from a producer-authored body; otherwise HTTP status text; otherwise a generic fallback |
| `error.type` | string | Mapped from `Fault.Type`, falling back to the HTTP status |
| `error.param` | string or null | Currently always `null`; the gateway fault contract does not carry an OpenAI parameter path |
| `error.code` | string or null | `Fault.Code`, or `null` when no stable gateway code exists |

`param` and `code` are always emitted. Unknown values are represented as JSON `null`.

### Type mapping

| Gateway fault type | OpenAI-compatible type |
|---|---|
| `authentication` | `authentication_error` |
| `authorization` | `permission_error` |
| `throttling` | `rate_limit_error` |
| `guardrail` | `invalid_request_error` |
| `validation` | `invalid_request_error` |
| `requestSize` | `invalid_request_error` |
| `routing` | `not_found_error` |
| `upstream` | `server_error` |
| `internal` | `server_error` |

When the gateway fault type is absent or unknown:

| HTTP status | Type |
|---|---|
| `401` | `authentication_error` |
| `403` | `permission_error` |
| `404` | `not_found_error` |
| `429` | `rate_limit_error` |
| other `4xx` | `invalid_request_error` |
| all other statuses | `server_error` |

OpenAI SDK exception classes are primarily selected from the HTTP status. The `type` field is
retained as useful structured metadata, but clients must tolerate values they do not recognize.

### `error_id` extension

`error_id` is emitted only for failures generated by the policy engine itself. It correlates the
client-visible response with the gateway log entry containing the underlying cause.

```json
{
  "error": {
    "message": "An unexpected error occurred.",
    "type": "server_error",
    "param": null,
    "code": null,
    "error_id": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

It is not normally added to ordinary policy rejections or backend errors.

### `guardrail` extension

A guardrail rejection can include client-safe intervention metadata:

```json
{
  "error": {
    "message": "Violation of applied word count constraints detected",
    "type": "invalid_request_error",
    "param": null,
    "code": "906201",
    "guardrail": {
      "interveningGuardrail": "word-count-guardrail",
      "action": "GUARDRAIL_INTERVENED",
      "actionReason": "Violation of applied word count constraints detected",
      "direction": "REQUEST",
      "assessments": {
        "assessments": "Expected word count to be between 10 and 500 words."
      }
    }
  }
}
```

| Field | Meaning |
|---|---|
| `interveningGuardrail` | Name of the guardrail policy that intervened |
| `action` | Machine-readable action, currently normally `GUARDRAIL_INTERVENED` |
| `actionReason` | Short client-safe explanation of the intervention |
| `direction` | `REQUEST` when caller input was blocked; `RESPONSE` when provider output was blocked |
| `assessments` | Guardrail-specific evidence; present only when the policy's `showAssessment` setting permits disclosure |

The formatter never emits `Fault.Description`. For response guardrails it can contain the exact
provider content the guardrail was intended to prevent from leaving the gateway.

The guardrail block represents a gateway policy rejection. It does not claim that the model
produced an OpenAI `refusal`, `finish_reason: content_filter`, or an incomplete Responses object.

## Body Ownership and Precedence

Formatting is governed by who authored the body, not merely by whether bytes exist.

The precedence order is:

1. An operator's fault-policy body.
2. A backend/provider error document.
3. An already OpenAI-shaped producer body.
4. The built-in OpenAI formatter.

The policy execution context records body provenance as one of:

- producing policy;
- fault policy;
- backend;
- gateway/formatter.

### Producing-policy exception

For most protocol shapes, a producing policy that writes a body without a `Fault` owns the final
body. The OpenAI shape intentionally makes one exception: a producer-authored ordinary JSON or
plain-text body is reshaped so an OpenAI SDK can read it.

Before reshaping, the formatter attempts to recover a message in this order:

1. nested `error.message`;
2. top-level `message`;
3. `error_description`;
4. `detail`;
5. string-valued `error`;
6. `description`;
7. plain text, capped at 512 bytes;
8. HTTP status text.

Non-string JSON values, invalid UTF-8, HTML/XML documents, and malformed JSON are not copied into
the message.

An object already containing `error.message` is treated as OpenAI-shaped and passed through
byte-for-byte to avoid losing provider- or policy-specific `param`, `code`, and other fields.

### Backend, router, and gateway faults

OpenAI formatting and upstream fault-policy execution are separate decisions. The
`policy_engine.fault_policies.handle_upstream_faults` setting controls whether the fault-policy
chain processes backend and router failures; it does not enable or disable the OpenAI formatter.
Gateway-originated faults, including guardrail interventions, remain eligible for fault-policy
processing regardless of this setting.

When OpenAI-compatible formatting is enabled, body ownership determines the final result:

| Fault source and resulting body | Final behavior |
| --- | --- |
| Gateway fault or guardrail; fault policy writes no body | Built-in formatter writes the OpenAI envelope |
| Gateway fault or guardrail; fault policy writes a body | Fault-policy body is preserved |
| Router-generated error body, such as `no healthy upstream` | Built-in formatter may replace it with the OpenAI envelope |
| Backend/provider error with a non-empty body | Backend body is preserved byte-for-byte |
| Backend/provider error with an empty body | Passed through with no body; the provider decided not to send one |
| Any fault for which a fault policy writes a body | Fault-policy body has highest precedence |

With `handle_upstream_faults = true`, backend and router failures run through the configured fault
policies. A policy may inspect the buffered upstream error and change its status, headers, fault
details, or body. A status-, header-, or fault-detail-only change does not claim body ownership:
an existing backend body is still preserved, while a replaceable router body or an eligible empty
body can still receive the OpenAI shape. If the policy explicitly writes or clears the body, that
body decision is authoritative and the built-in formatter does not overwrite it.

With `handle_upstream_faults = false`, backend and router failures do not run through the fault-policy
chain. OpenAI formatting can still run independently, subject to the same ownership and
response-body-processing rules. In particular, enabling OpenAI formatting does not translate a
non-empty Anthropic or other provider error into OpenAI format.

The encoded-body and already-started streaming limitations described under Response-Body
Processing still apply; “may synthesize” above means the response reached an eligible formatting
path.

### Provider-specific fault-policy override

OpenAI-compatible formatting may be enabled for all LLM providers while an operator preserves a
different provider contract on selected routes. A fault policy can identify the provider using
trusted route or provider metadata and explicitly write that provider's error document. Because a
fault-policy-authored body has the highest precedence, the built-in OpenAI formatter does not
replace it.

For example, an Anthropic-specific policy may write:

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Request blocked by a guardrail"
  }
}
```

The policy must write the response body and set the appropriate content type. Merely changing the
status, headers, or internal fault description does not claim body ownership; in that case the
built-in OpenAI formatter still supplies the final error body. The policy is also responsible for
mapping gateway fault codes and guardrail details into the selected provider's contract.

This override is suitable for operator customization. If another provider format becomes a
first-class platform capability, a formatter selected from trusted route metadata is preferable to
duplicating protocol translation across fault policies.

## Architecture

The feature extends the existing protocol-aware fault formatter used by Agent/A2A errors.

```text
Policy, router, or engine produces a fault
                    |
                    v
Kernel determines status, provenance, and authorship
                    |
                    v
Fault policies run, when configured and eligible
                    |
                    v
ShouldFormat applies kind and ownership gates
                    |
                    v
Negotiate selects ShapeOpenAI for LLM API kinds
                    |
                    v
openAIRenderer creates the envelope
                    |
                    v
Translator applies body and Content-Type mutations
```

### Formatter registry

`ShapeOpenAI` is registered alongside JSON, JSON-RPC, event-stream JSON-RPC, and XML shapes. Shape
selection is based on API kind before generic content negotiation. Therefore an LLM caller's
`Accept` header cannot accidentally select the generic JSON renderer.

### API-kind enablement

Negotiation and enablement are separate:

- `Negotiate` knows that an LLM route uses `ShapeOpenAI`.
- The formatter kind set decides whether formatting is active for that route.

With the feature disabled, the LLM kind fails the enablement gate and existing behavior is
preserved even though negotiation knows about the shape.

## Response-Body Processing

Some router failures have a body, such as Envoy's plain-text `no healthy upstream` response. If
the route has no response-body policy, Envoy would normally never send that body to the policy
engine.

At response headers, the gateway therefore enables buffered response-body processing only for the
current response when:

- OpenAI-compatible errors are enabled;
- the route is an LLM route;
- the status is at least `400`;
- the chain does not already require the response body;
- a body is expected;
- the response body is not encoded.

Successful responses remain in response-body mode `NONE` when no policy needs their body.

When the buffered body arrives, it enters the same fault handling and provenance path as a route
that already required response-body processing. Backend-authored error documents pass through;
router-generated bodies can be replaced.

### Bodyless errors

Envoy sends no response-body callback for a bodyless response. On a route that already requires
response-body processing, the kernel executes the body phase inline with an empty body. When the
formatter adds a body from that response-header phase, the ext_proc response uses
`CONTINUE_AND_REPLACE`; otherwise Envoy would discard the body mutation while retaining its new
`Content-Length`.

A bodyless upstream or router error on a route with no response-body requirement does not currently
enter that inline body path and is therefore not covered by formatter-only buffering. This is a
known boundary of the current implementation.

### Encoded errors

The formatter-only buffering path does not rewrite an encoded body. Replacing it safely would also
require decoding the body and reconciling `Content-Encoding`. Routes that already require response
body processing continue to use the gateway's existing decompression and recompression behavior.

### Streaming

An error returned before a stream begins is a normal JSON HTTP error, even when the original request
asked for streaming.

Once streamed response headers or body chunks have reached the client, the HTTP status and prior
bytes are committed. The formatter does not attempt to replace that stream with an HTTP error.
Mid-stream guardrail termination remains the responsibility of streaming policy behavior.

## Guardrail Semantics

Request and buffered response guardrails both use the HTTP error envelope. This is deliberate:

- a request guardrail may reject before any provider is called;
- a response guardrail is a gateway policy decision, not a model-generated refusal;
- synthesizing a complete Chat Completions or Responses success object would require preserving and
  transforming endpoint-specific IDs, model metadata, usage, choices/output items, and streaming
  state.

Applications therefore observe a gateway guardrail as an SDK API exception, not as a successful
model response containing `refusal` or `finish_reason: content_filter`.

Guardrail statuses should remain non-retryable `4xx` responses. Mapping an intervention to `429` or
`5xx` could cause SDKs to retry automatically, generating repeated cost and repeated blocked output.

## Compatibility

### Disabled mode

When disabled:

- LLM kinds are not added to the enabled formatter set;
- no formatter-specific buffering is requested;
- policy, router, engine, and backend errors retain their previous bodies.

### Enabled mode

When enabled:

- successful responses are unchanged;
- HTTP statuses are unchanged;
- gateway-produced LLM errors use the OpenAI envelope;
- producer-authored non-OpenAI bodies may be reshaped;
- fault-policy bodies remain authoritative;
- backend/provider error documents remain authoritative;
- already OpenAI-shaped bodies remain byte-for-byte unchanged.

### A2A isolation

Agent/A2A formatting continues to use transport-aware JSON-RPC or event-stream JSON-RPC shapes. The
OpenAI producer-body exception is gated by `ShapeOpenAI` and does not alter A2A authorship behavior.

## Security and Privacy

- `Fault.Description` is never rendered by built-in formatters.
- Guardrail assessments are emitted only when the guardrail policy already opted into disclosure.
- Invalid or document-like producer bodies are not copied wholesale into `error.message`.
- Plain-text messages are capped at 512 bytes.
- Internal errors expose a correlation ID, not their underlying cause.
- Backend error documents are preserved rather than parsed and re-exposed through a potentially
  lossy transformation.

## Observability

The formatter logs why formatting was applied or skipped. Useful reasons include:

- formatter not enabled for the API kind;
- body already authored;
- policy body already in the OpenAI shape;
- no registered renderer for the negotiated shape;
- formatted as OpenAI.

`error_id` connects client-visible internal errors to gateway logs. Fault analytics independently
retain direction, policy attribution, source, original status, guardrail metadata, and the WSO2
fault code.

## Failure Handling

The renderer must always return valid JSON. If JSON marshaling fails—for example because a
guardrail assessment contains a value unsupported by `encoding/json`—it returns a minimal envelope:

```json
{
  "error": {
    "message": "An unexpected error occurred.",
    "type": "server_error",
    "param": null,
    "code": null
  }
}
```

Formatting failure must never turn a served error into a dropped request.

## Testing Strategy

### Unit tests

The formatter tests cover:

- core envelope and nullable fields;
- message fallback behavior;
- type mapping by fault type and status;
- `error_id` inclusion and omission;
- complete guardrail metadata, including uppercase direction;
- already OpenAI-shaped body detection;
- legacy JSON and plain-text message recovery;
- malformed, non-string, HTML/XML, long, and invalid-UTF-8 bodies;
- preservation of the shipped formatter kind set.

Kernel tests cover:

- the feature defaulting off;
- option enablement without disturbing Agent formatting;
- producing-policy body reshaping;
- described rejection formatting;
- fault-policy body precedence;
- response guardrail formatting;
- backend error pass-through;
- router failure formatting;
- per-error buffering on routes without body policies;
- no buffering for successful responses;
- bodyless error delivery with `CONTINUE_AND_REPLACE`.

### Integration tests

The integration suite should verify both `LlmProvider` and `LlmProxy`, including:

- authentication and authorization failures;
- rate limiting;
- request and response guardrails;
- router/no-healthy-upstream failures;
- empty error bodies;
- ordinary JSON and plain-text producer bodies;
- already OpenAI-shaped producer bodies;
- backend OpenAI and non-OpenAI error pass-through;
- successful non-streaming and streaming responses;
- `HEAD` behavior;
- the disabled configuration.

Tests should validate HTTP status, content type, body schema, message selection, type and code mapping,
guardrail direction, and pass-through byte identity where promised.

## Rollout

1. Ship the configuration enabled by default, and call out the changed LLM error bodies in the
   release notes, with `disable_openai_format = true` as the way to keep the previous ones.
2. Verify SDK exception behavior, especially retry handling for guardrail statuses.
3. Monitor error counts, response sizes, policy-engine memory, and ext_proc message-limit failures.
4. Document WSO2 extensions for clients that perform strict schema validation.

Rollback requires setting `disable_openai_format = true` and restarting or redeploying the policy
engine with the updated configuration.

## Alternatives Considered

### Keep existing policy-specific bodies

Rejected because OpenAI SDKs cannot consistently extract messages from plain text or arbitrary REST
JSON.

### Convert backend/provider errors too

Rejected because providers own their error contracts. Rewriting them can discard actionable detail
and corrupt already compatible responses.

### Implement formatting as a fault-policy entry

Rejected because protocol conformance could be omitted or ordered incorrectly. Formatting is a
default applied after fault policies, while an operator-authored fault-policy body can still override
it.

### Synthesize endpoint-native refusal responses

Not selected for this feature. Request guardrails may run before a provider exists, and response
guardrails would require endpoint- and stream-aware transformation of full Chat Completions or
Responses objects. That is a separate feature rather than a responsibility of the generic HTTP
error formatter.

### Enable LLM formatting unconditionally

Rejected because some existing LLM clients rely on the policies' own error bodies. The feature is
on by default but stays switchable, so those deployments keep a way back.

### Keep the feature opt-in

Rejected: almost every LLM client is an OpenAI SDK that cannot read the policies' own bodies, so an
opt-in leaves the common case broken until each operator discovers the setting.

## Open Questions

- Should a future strict mode omit the WSO2 `error_id` and `guardrail` extensions?
- Should `error_id` move to a response header in strict mode?
- Should the gateway add a first-class OpenAI `param` field to the fault contract?
- Should response guardrails eventually support endpoint-native refusal transformation?
- Should confirmed backend errors skip formatter-only buffering as a measured optimization?
