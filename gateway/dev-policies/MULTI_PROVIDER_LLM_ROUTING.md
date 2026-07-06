# Multi-Provider LLM Routing & Provider Transformation

> One OpenAI-shaped endpoint, many LLM backends. Clients keep talking plain
> OpenAI Chat Completions; the gateway picks a provider, rewrites the request
> into that provider's native API, calls it with its own credentials, and
> rewrites the response back into the OpenAI shape.

---

## 1. What is this?

This is the **LLM proxy layer** of the WSO2 API Platform gateway. It lets you
publish a single endpoint that speaks the **OpenAI Chat Completions** wire
format (`POST /chat/completions`) while the actual upstream can be OpenAI,
Azure OpenAI, Anthropic, Mistral, or Gemini.

Two cooperating pieces make this work:

1. **Routing policy** (`openai-header-router`) — decides *which* provider a
   request should go to and publishes that decision.
2. **Transformation policies** (`openai-to-anthropic`, `openai-to-azure-openai`,
   `openai-to-mistral`, `openai-to-gemini`) — translate the OpenAI request into
   the selected provider's native format and translate the response back.

The control plane (management API + Kubernetes operator) wires these together
from a single `LlmProxy` resource that lists a primary `provider` and any
number of `additionalProviders`.

---

## 2. Why do we need it?

Every LLM vendor exposes a *different* HTTP API:

| Provider     | Path shape                                   | Auth header        | Body shape            |
|--------------|----------------------------------------------|--------------------|-----------------------|
| OpenAI       | `/v1/chat/completions`                       | `Authorization: Bearer` | OpenAI messages  |
| Azure OpenAI | `/openai/deployments/{model}/chat/completions?api-version=…` | `api-key` | OpenAI messages |
| Anthropic    | `/v1/messages`                               | `x-api-key`        | Anthropic messages    |
| Mistral      | `/v1/chat/completions`                       | `Authorization: Bearer` | OpenAI-ish       |
| Gemini       | `/{ver}/models/{model}:generateContent`      | `x-goog-api-key`   | `contents[]` + `parts`|

Without this layer, **every client** would have to know each provider's quirks,
hold each provider's keys, and be rewritten whenever you switch or add a vendor.

With this layer:

- **Clients write OpenAI once.** Switching from OpenAI to Gemini is a config
  change on the proxy, not a client change.
- **Provider credentials stay in the gateway.** Clients authenticate to the
  gateway with one key; the gateway holds and injects each provider's real key
  on the proxy→provider hop.
- **Provider selection is dynamic.** A single header (`x-provider`) can fan one
  endpoint out to several vendors — useful for A/B testing, cost routing,
  failover staging, and per-tenant provider choice.

---

## 3. Architecture

### 3.1 Big picture (control plane → gateway)

```
                          API PLATFORM (control plane)
  ┌───────────────────────────────────────────────────────────────────┐
  │  API Designer / Management Portal / Dev Portal                      │
  │              │                                                      │
  │       Platform API  ──────────────►  Gateway Controller             │
  │                                          │                          │
  │                                    Policy Engine ──► Router          │
  │                                          ▲                          │
  │                              Provider Transformation Policy          │
  └───────────────────────────────────────────────────────────────────┘
            ▲ pulls policy
       Gateway Builder ◄── Pulls Policy ── Policy HUB
```

The operator/management side takes an `LlmProxy` spec and emits the concrete
route + policy chain the gateway runs.

### 3.2 Request data flow (the bottom half of the diagram)

Example: a client sends an **OpenAI** request, the proxy targets **Azure OpenAI**.

```
  Client                Gateway Policy Engine                         Upstream
 (OpenAI    ┌─────────────────────────────────────────────┐
  format)   │  1. api-key-auth        validates client key │
   │        │  2. openai-header-router  picks provider,     │
   ▼        │       writes metadata[selected_provider]      │
 ┌──────┐   │  3. upstream-auth         injects provider key │   ┌─────────────┐
 │OpenAI│──►│  4. openai-to-azure-openai                     │──►│ AzureOpenAI │──► azure.com
 │ LLM  │   │       • rewrites BODY → Azure shape            │   │ (native API)│
 │Proxy │◄──│       • rewrites PATH → /openai/deployments/…  │◄──│             │◄── response
 └──────┘   │       • on response: Azure → OpenAI shape       │   └─────────────┘
   ▲        └─────────────────────────────────────────────┘
   └── OpenAI-shaped response returned to client
```

The transformation is **bidirectional**: request OpenAI→provider on the way in,
response provider→OpenAI on the way out.

---

## 4. The policies (`gateway/dev-policies/`)

These are Go policies built against the gateway SDK
(`github.com/wso2/api-platform/sdk`). See [README.md](./README.md) for how
dev-policies are built and registered.

### 4.1 `openai-header-router` — provider selection

- **Phase:** runs in the **request-header** phase (and re-publishes in the body
  phase as an idempotent fallback).
- **What it does:** reads a configured header (default `x-provider`), matches it
  case-insensitively against an ordered `mappings` list, and writes the chosen
  provider id into `SharedContext.Metadata["selected_provider"]`. Falls back to
  `defaultProvider` when the header is missing or unmatched.
- **Why header phase:** the proxy→provider auth-injection policy is gated on
  `selected_provider` and runs in the header phase — so the selection must
  exist *before* it evaluates. (See `Mode()` doc comment in
  [openaiheaderrouter.go](./openai-header-router/openaiheaderrouter.go).)

Key params:

| Param             | Purpose                                                     |
|-------------------|-------------------------------------------------------------|
| `headerName`      | Header to read (default `x-provider`).                      |
| `defaultProvider` | Provider id when header missing/unmatched. **Required.**    |
| `mappings[]`      | `headerValue → provider` rules, first match wins. **Required.** Duplicate header values are rejected. |

### 4.2 `openai-to-{anthropic,azure-openai,mistral,gemini}` — translators

Each translator:

- **Phase:** buffers request body + response body.
- **Gating (`shouldRun`):** the heart of single- vs multi-provider mode —
  - **Single-provider mode:** no router ran, so `selected_provider` is empty →
    the translator **always runs**.
  - **Multi-provider mode:** a router published a selection → the translator
    runs **only if** `selected_provider == its own id` (case-insensitive).
- **Request translation:** rewrites the JSON body into the provider's native
  shape and rewrites the upstream **path**; sets `UpstreamName` to its `id` so
  the request routes to the right upstream cluster.
- **Response translation:** rewrites the provider's non-streaming response back
  into an OpenAI `ChatCompletion`. Streaming (SSE) responses are passed through
  untouched.

Common params:

| Param        | Purpose                                                          |
|--------------|------------------------------------------------------------------|
| `model`      | Provider model name used in the rewritten request. **Required.** |
| `id`         | Provider this translator targets. Doubles as the upstream cluster name and the key matched against `selected_provider`. |
| `apiVersion` | (azure/gemini) API version segment in the rewritten path.        |

The Gemini translator ([openaitogemini.go](./openai-to-gemini/openaitogemini.go))
is the most involved example: it maps OpenAI `messages[]` to Gemini
`contents[]`/`systemInstruction`, `tools`→`functionDeclarations`,
`tool_choice`→`functionCallingConfig`, sampling params→`generationConfig`, and
images (`image_url`) to `inlineData`/`fileData`.

---

## 5. Control-plane wiring (`LlmProxy` resource)

A proxy declares one primary `provider` and zero-or-more `additionalProviders`.
Each can carry its own loopback `auth` (the credential the gateway uses on the
proxy→provider hop). See
[examples/openai-multi-provider-proxy.yaml](../examples/openai-multi-provider-proxy.yaml).

```yaml
spec:
  provider:
    id: openai-provider
    auth: { type: api-key, header: X-API-Key, value: <key> }
  additionalProviders:
    - id: anthropic-provider
      auth: { type: api-key, header: X-API-Key, value: <key> }
    - id: azure-openai-provider
      auth: { type: api-key, header: X-API-Key, value: <key> }
  policies:
    - name: openai-header-router       # 1. select provider
      params: { headerName: x-provider, defaultProvider: openai-provider, mappings: [...] }
    - name: openai-to-anthropic        # 2. translators, one per provider
      params: { model: claude-sonnet-4-5-20250929, id: anthropic-provider }
    - name: openai-to-azure-openai
      params: { apiVersion: "2024-02-15-preview", model: gpt-4o, id: azure-openai-provider }
```

### How the control plane turns this into a running chain

1. **Types** ([llmproxy_types.go](../../kubernetes/gateway-operator/api/v1alpha1/llmproxy_types.go)):
   new `LLMProxyAdditionalProvider` struct (`id`, optional `as` logical upstream
   name, optional `auth`).
2. **Validation** ([llm_validator.go](../gateway-controller/pkg/config/llm_validator.go)):
   validates each additional provider's `id`/`as` format, **rejects duplicate
   upstream names**, and validates upstream auth (`type: api-key` requires
   `header` + `value`).
3. **Transformation** ([llm_transformer.go](../gateway-controller/pkg/utils/llm_transformer.go)):
   emits an upstream-auth policy **per provider**, each guarded by an
   `ExecutionCondition` derived from `selected_provider`:
   - primary provider: runs when no selection exists **or** selection matches —
     `!('selected_provider' in request.Metadata) || request.Metadata['selected_provider'] == '<id>'`
   - additional providers: run only when selection matches —
     `'selected_provider' in request.Metadata && request.Metadata['selected_provider'] == '<id>'`
4. **Operator deploy** ([llmproxy_controller.go](../../kubernetes/gateway-operator/internal/controller/llmproxy_controller.go)
   + [management_upstream_auth_payload.go](../../kubernetes/gateway-operator/internal/controller/management_upstream_auth_payload.go)):
   resolves each provider's secret value (including `valueFrom` secret refs) and
   flattens it into the management payload the gateway-controller consumes.

---

## 6. Summary of the changes

The whole multi-provider LLM routing capability was built up across three change
sets — the initial feature, a refinement pass, and the current hardening +
Gemini work. Taken together they touch every layer: the **policies** that run in
the gateway, the **control-plane types and validation**, the **transformer** that
generates the policy chain, the **operator** that resolves credentials and
deploys, and the **examples** that show operators how to use it. Here is the
combined picture of what landed and why.

**Policies (gateway data plane).** Four routing/translation policies were added
first — `openai-header-router` (provider selection) plus the
`openai-to-anthropic`, `openai-to-azure-openai`, and `openai-to-mistral`
translators — and the current work adds the `openai-to-gemini` translator,
completing the provider set. The router publishes the chosen provider into
`metadata["selected_provider"]`; each translator gates itself on that value so
the same chain works in both single-provider mode (no selection → the one
translator always runs) and multi-provider mode (runs only when its `id`
matches).

**Control-plane types & validation.** `additionalProviders` became a
first-class concept: the operator CRD gained the `LLMProxyAdditionalProvider`
type (`id`, optional `as` logical upstream name, optional `auth`) along with the
generated deepcopy and CRD schema, and the management API / platform-api models
and services were extended to carry it through LLM deployment translation. New
validation in `llm_validator.go` enforces `id`/`as` formats, **rejects duplicate
upstream names**, and requires `header` + `value` for `api-key` upstream auth on
both the primary provider and every additional provider.

**Transformer (chain generation).** `llm_transformer.go` now emits an
upstream-auth policy **per provider**, each guarded by an `ExecutionCondition`
derived from `selected_provider` — the primary provider runs when no selection
exists *or* the selection matches it, while additional providers run only on an
exact match. This is what makes per-provider credentials inject correctly when
one endpoint fans out to many backends. Covered by `llm_transformer_test.go`.

**Operator (credential resolution & deploy).** The controller resolves each
provider's secret value — including `valueFrom` secret references — and flattens
it into the management payload the gateway-controller consumes, now for the
primary provider *and* each additional provider. Covered by the controller
tests.

**Examples & supporting policies.** The provider/proxy example manifests
(`*-provider.yaml`, `*-proxy.yaml`) were added and then refined, with
`openai-multi-provider-proxy.yaml` reworked into the canonical end-to-end
demonstration of the feature. The `slugify-body` and `analytics` policy
definitions were adjusted along the way to stay compatible with the updated
proxy chain.

---

## 7. What can you use it for?

- **Vendor-agnostic LLM access** — clients code once against OpenAI, you swap
  backends behind the proxy.
- **Header-based routing / A-B testing** — route `x-provider: anthropic` vs
  `x-provider: openai` from the same endpoint.
- **Cost / capability routing** — send cheap traffic to one provider, premium
  to another.
- **Centralized credential & policy enforcement** — keys, rate limits,
  analytics, and auth live on the gateway, not in clients.
- **Migration & failover staging** — bring a new provider online as an
  `additionalProvider` and shift traffic gradually via the router mappings.

---

## 8. Modes at a glance

| Mode             | Router present? | `selected_provider` | Translator behavior                 |
|------------------|-----------------|---------------------|-------------------------------------|
| Single-provider  | No              | empty               | The one translator always runs.     |
| Multi-provider   | Yes             | set by router       | Each runs only if its `id` matches. |
