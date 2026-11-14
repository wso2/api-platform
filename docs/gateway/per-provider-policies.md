# Per-provider policies

A policy attached under `spec.providers[].policies` on an `LlmProxy` runs **once for every
attempt sent to that provider**, not once per client request. That one sentence is the whole
difference from every other policy attachment, and it is the thing to hold in mind while writing
one.

It exists because a request can be served by more than one provider. When `model-failover` is
attached and the first provider fails, the gateway tries the next one — and the second provider
needs its own credential, its own request format and its own upstream path. Those cannot be
applied once at the start, because at the start nobody knows which provider will serve the
request.

---

## Quick start

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProxy
metadata:
  name: chat-proxy
spec:
  displayName: Chat Proxy
  version: v1.0
  context: /chat
  providers:
    - id: openai-provider
      isPrimary: true
      policies:                       # runs on every attempt routed to OpenAI
        - name: set-headers
          version: v1
          params:
            request:
              headers:
                - name: x-provider-tag
                  value: openai
    - id: anthropic-provider
      alias: anthropic
      transformer:                    # NOT a policies entry -- see below
        type: openai-to-anthropic
        version: v1
      policies:                       # runs on every attempt routed to Anthropic
        - name: set-headers
          version: v1
          params:
            request:
              headers:
                - name: x-provider-tag
                  value: anthropic
  operationPolicies:
    - name: model-failover            # what causes a second attempt to happen at all
      version: v0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            targets:
              - model: gpt-4o
                fallbacks:
                  - model: claude-sonnet-4-5-20250929
                    provider: anthropic
```

A request asking for `gpt-4o` goes to OpenAI. If OpenAI fails, the same client request is sent to
Anthropic — and Anthropic's `policies` run for that second attempt, OpenAI's do not run again.

---

## What each attempt starts from

**Every attempt begins from the same post-downstream baseline.** The gateway snapshots the
request as it stood when the ordinary (`policies` / `operationPolicies`) chain finished, and
resets to that snapshot before each attempt's per-provider chain runs.

So each attempt:

- **keeps** everything the ordinary chain did — PII masking, guardrail rewrites, prompt
  decoration, templating, compression. Those ran once for the client request and apply to every
  attempt. A fallback provider must never receive content the first provider was not allowed to
  receive.
- **does not see** what a previous attempt's per-provider chain did — not its path rewrite, not
  its request format, and not its credential.

### Why this matters for what you write

**You do not need to undo your own work.** A policy that sets a header, appends a header, removes
a header, or rewrites the path can do so unconditionally. On the next attempt that mutation is
gone before the next provider's chain runs.

Without that reset, a policy appending a header would append it twice on a retry — and worse, the
first provider's `Authorization` header would travel to the second provider. A key handed to a
third party, with the client seeing nothing wrong because the request still succeeds.

**You do not need to check whether you already ran.** Writing

```go
// Not needed — the previous attempt's value is already gone.
if _, exists := reqCtx.Headers.Get("x-provider-tag"); exists {
    return policy.UpstreamRequestHeaderModifications{}
}
```

guards against something that cannot happen. Worse, it can be actively wrong: if the *client*
sent that header, or the ordinary chain set it, the value is part of the baseline and present on
every attempt — so the guard makes your policy do nothing at all.

**Side effects are a different matter.** The reset covers the request, not the world. A
per-provider policy that charges a quota, writes to a store, or calls an external service does so
**once per attempt**, and a request served on the third attempt will have done it three times. If
an action must happen once per client request, it belongs in `operationPolicies`, which the
gateway enters exactly once.

---

## What else is in the per-attempt chain, and where

Your entries are not the whole chain. The gateway appends two things **after** them, in this
order:

```
[ your policies ]  ->  [ this provider's transformer ]  ->  [ this provider's auth ]
```

Two consequences worth knowing:

- **A policy you list here never sees the credential.** Authentication is appended last, so the
  `Authorization` (or `x-api-key`) header does not exist yet while your policy runs. If you are
  trying to read or modify it, you are in the wrong place.
- **A transformer is configured, not listed.** Put it in the entry's `transformer` field, not in
  `policies`. The gateway injects it as a conditional policy scoped to this provider and passes
  the provider's `alias` to it as its target upstream, which is how it knows to activate. Listing
  a transformer under `policies` yourself means configuring that condition by hand.

`model-failover` must **not** appear here either. It is attached once under `operationPolicies`,
and the gateway adds whatever per-attempt work it needs on its own.

---

## Which phases run per attempt

`OnRequestHeaders` and `OnRequestBody`. The response phases are not per-attempt: only one
response is ever delivered to the client, so `OnResponseHeaders` and the response body phases run
once, for the attempt that actually produced that response.

A per-provider policy that rewrites the request body must expect its input to be the baseline
body every time — not its own previous output. This is what makes a cross-vendor chain work: the
Anthropic transformer receives the original OpenAI-shaped payload on attempt 2, not the payload
it would have produced had it run on attempt 1.

---

## One degraded case to know about

The baseline is held in the gateway's own memory, keyed by the request id Envoy stamps on every
request, and it expires on a timer sized well above any real request. If that entry is missing
when a retry arrives — it expired mid-request, or the request carries no id — that attempt
becomes its own baseline: the previous attempt's mutations are not undone, and its headers are
not restored.

The gateway logs a warning naming the attempt and cluster rather than failing the request,
because failing closed there would turn a degraded retry into a lost request. If you see

```
per-attempt baseline missing on a retry; this attempt cannot be reset to the post-downstream request
```

treat it as a real signal: on that request, and only that request, the guarantees above did not
hold.
