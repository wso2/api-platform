# Global Policies for LLM Providers and LLM Proxies

Global policies apply a single policy across **every operation** of an LLM Provider or LLM Proxy, rather than to one path and method at a time. They are the natural way to express a provider-wide control — a token budget for the whole provider, a single request ceiling shared across all endpoints, or a guardrail that must run on every route.

Policies attached to an LLM Provider or Proxy now fall into two lists:

| List | Scope | Counter behaviour |
|---|---|---|
| **Global policies** | The whole provider or proxy — applies to every operation | One shared counter across all routes |
| **Operation policies** | A specific path and method | An independent counter per route |

The older flat **policies** list is deprecated but still honoured — see [Legacy policies](#legacy-policies) below.

> Global policies are supported on **LLM Providers** and **LLM Proxies**. They do not apply to MCP Proxies.

## Global vs. Operation Policies

The difference is the *scope of the counter* (for rate limits) and the *breadth of application* (for guardrails).

- A **global** rate limit maintains **one shared bucket** for the entire provider or proxy. Traffic on any route draws down the same allowance. If a provider has a global limit of 100 requests/hour, then 60 requests to `/chat/completions` plus 40 to `/embeddings` exhausts it — the next request to *either* route is rejected.
- An **operation** rate limit maintains an **independent bucket per (path, method)**. A limit of 100 requests/hour attached to `/chat/completions` and `/embeddings` allows 100 on each, counted separately.

The same distinction holds for guardrails: a global guardrail runs on every operation, while an operation guardrail runs only on the paths and methods it is attached to.

### Which to choose

- Use a **global policy** when the control describes the provider or proxy as a whole: "this provider may consume at most X tokens per hour," "mask PII on every request regardless of endpoint."
- Use an **operation policy** when different endpoints need different treatment: a stricter limit on an expensive completion route, a guardrail only on a route that accepts free-text input.

## Configuring Policies

Policies are configured from the **Guardrails** and **Rate Limiting** tabs of an LLM Provider or LLM Proxy in the AI Workspace UI.

When you attach a policy, choose its scope:

- **Global** — the policy is added to the global policies list and applies across the whole provider or proxy. Global policies are deduplicated by name and version; re-adding a policy with the same name overwrites its parameters.
- **Resource** (operation) — the policy is attached to the specific model resource / path you are editing. One operation policy can accumulate multiple paths; adding the same policy to another resource appends that path rather than creating a duplicate.

Removing a global policy removes it from the whole provider or proxy. Removing an operation policy drops just that path; when the last path of a policy is removed, the policy itself is removed.

## Evaluation Order

For each request, policies run **global first, then operation**:

```
request → global policies → operation policies → LLM upstream
```

This ordering matters for rate limiting. Because the global limit is evaluated first, it counts **every request attempt** — including requests that a tighter operation policy later rejects. See below.

## Rate-Limit Semantics

Global rate limits are a **hard ceiling on total traffic** and are counted **per attempt, not per success**:

- A global limit maintains one shared bucket. Exhausting it via one route rejects requests on *all* routes.
- When a global limit and an operation limit are both present, the global limit is evaluated first and increments its shared counter on **every request** — even requests that the operation limit subsequently rejects.

**Example.** A provider has a global limit of 20 requests/hour and an operation limit of 5 requests/hour on `/chat/completions`:

- Requests to `/chat/completions` are rejected once that route hits 5 — but those 5 (plus any further rejected attempts) still count against the global bucket.
- A different route with no operation policy — say `/embeddings` — can be rejected purely because the global bucket of 20 is exhausted, even though it never had its own limit.

Rejected requests receive HTTP `429` with the body `Rate limit exceeded`.

> **Advanced rate limit and shared counters.** The advanced rate-limit policy keys its counter per route by default, which would silo the limit per operation. When you attach it as a *global* policy, the platform automatically configures it to key on the provider or proxy instead, so a single bucket is shared across all routes — matching the behaviour of a global basic rate limit. Any key-extraction setting you configure explicitly is preserved.

## Legacy Policies

The original flat **policies** list is **deprecated** but continues to work, so existing configurations behave exactly as before.

When a configuration is created or edited through the AI Workspace UI, each legacy entry is migrated into the new lists automatically, according to its scope:

- A **route-specific** legacy entry — one scoped to a specific path, or to specific methods — becomes an **operation policy**.
- A **provider-wide** legacy entry — one that applied to all paths and all methods — becomes a **global policy**.

You should pick **one style per resource**: either the deprecated flat `policies` list, or the new global/operation lists. A configuration that mixes a non-empty legacy `policies` list with the new lists is rejected. (Global and operation policies together are always fine — they are complementary.)

## Gateway Version Requirements

Global and operation policies require a gateway running a version that understands the two-list format. When you deploy to an **older gateway**, the Platform API automatically down-converts the two lists back into the flat legacy format before sending the artifact, so deployments to older gateways continue to work — global policies are encoded as provider-wide (all-paths, all-methods) legacy entries. This conversion is transparent; you configure policies the same way regardless of the target gateway version.

## Example

A provider with a global token budget shared across all operations, plus a stricter per-route request limit on the completions endpoint:

```yaml
globalPolicies:
  - name: advanced-ratelimit
    version: v1
    params:
      quotas:
        - name: request-limit
          limits:
            - limit: 10000
              duration: "1h"
operationPolicies:
  - name: basic-ratelimit
    version: v1
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          limits:
            - requests: 100
              duration: "1h"
```

`globalPolicies` carries its `params` at the top level (one bucket for the whole provider or proxy). `operationPolicies` nest `params` under each `paths[]` entry (one bucket per path and method).
