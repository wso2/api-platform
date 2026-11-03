# Models

Two kinds work together to serve AI models from the gateway:

- A **Model endpoint** (`ModelEndpoint`) is the API clients call. It has a context, a wire format (OpenAI, Anthropic or Gemini), its own policies, its own keys and a model catalog. The gateway provisions one, `wso2`, at `/wso2/v1`; you can add more.
- A **Model** (`Model`) is what clients name in a request. It lists the endpoints that serve it, and its **targets**: models on your LLM providers that can answer for it.

A client calls an endpoint, names a Model, and the gateway routes the request to one of that Model's targets. When the target's provider does not speak the endpoint's format, the endpoint's transformer for that provider converts the request and the response.

```
client ──POST /wso2/v1/chat/completions {"model": "gpt-4o", ...}──▶ ModelEndpoint "wso2" (OpenAI)
                                                                    │
                                  Model "gpt-4o" ── targets ──┬─▶ openai-prod / gpt-4o
                                                              └─▶ azure-eu    / gpt-4o-deployment
```

Use Models when applications should name *what* they want — a model — and leave *where* it runs to the platform. You can then move a Model between providers, split its traffic or add a fallback, and no client changes its endpoint, key or request.

## Turn the feature on

Models are off by default. Turn them on in the gateway controller configuration:

```toml
[controller.models]
enabled = true

[controller.models.endpoint]
provision = true              # create the default endpoint "wso2" (the default)
context = "/wso2/v1"          # where the default endpoint is served
max_request_body_bytes = 0    # every endpoint's limit; 0 = router.http_listener.per_connection_buffer_limit_bytes
```

On startup the gateway creates the endpoint `wso2`:
- in the OpenAI format;
- authenticating with `api-key-auth` on `Authorization: Bearer <key>`, which is what every OpenAI client library sends;
- with transformers for the provider templates it ships with (see [Transformers](#transformers)).

If another artifact already serves the configured context when the gateway first provisions `wso2`, or after you change the context, the gateway refuses to start instead, so the endpoint never takes over something you own. Endpoint writes are checked the same way: an endpoint cannot take a context another endpoint or API serves.

Other kinds do not check model endpoints, so an API created later at an endpoint's context is not refused. Requests there may then reach either, and the controller logs the conflict at startup; move one of them.

While provisioning is on, `wso2` belongs to the gateway:
- It cannot be created or deleted through the API.
- Its context always comes from `controller.models.endpoint.context`.

An admin may still change its policies, transformers, display name and resilience settings. The transformers are written once, when the endpoint is first created, and an upgrade never rewrites them. With `provision = false`, `wso2` is an ordinary endpoint you create yourself.

## Model endpoints

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: ModelEndpoint
metadata:
  name: partners
spec:
  displayName: Partner Model Endpoint   # unique among the gateway's endpoints
  version: v1.0
  context: /partners/v1                 # required for every endpoint but wso2
  vhost: partners-ai.example.com        # optional
  inboundTemplate: openai               # the wire format; openai (default), anthropic or gemini
  policies:                             # optional; see below
    - name: api-key-auth
      version: v1
      params: { key: Authorization, in: header, valuePrefix: Bearer }
  transformers: [ ... ]                 # see Transformers
```

```bash
curl -u admin:admin -X POST http://localhost:9090/api/management/v1/model-endpoints \
  -H 'Content-Type: application/yaml' --data-binary @partners.yaml
```

| Field | Notes |
|---|---|
| `context`, `vhost` | Where the endpoint is served. No other endpoint or API may use the same context on the same vhost. |
| `inboundTemplate` | The format clients speak, named by the LLM provider template that describes it. It decides the operations served, where the model name is read and the shape of errors and the catalog. |
| `policies` | The endpoint's API-level policies, applied to every request before the Model's own. When `policies` is omitted, the endpoint authenticates with `api-key-auth` on `Authorization: Bearer <key>`. An explicit list is used as given, so `policies: []` serves the endpoint without authentication. A rate limit here is shared by every Model on the endpoint. |
| `faultPolicies` | Run when a request on the endpoint fails, after the Model's own fault policies. |
| `transformers` | How the endpoint reaches providers that do not speak its format. |
| `resilience` | Route timeouts for the endpoint's operations. |
| `deploymentState` | `undeployed` stops serving the endpoint and keeps it. The other endpoints are unaffected. |

Only admins can create, update or delete endpoints; developers can list and read them.

### Formats

| `inboundTemplate` | Operations, under the endpoint's context | Model read from | Typical client key header |
|---|---|---|---|
| `openai` | `POST /chat/completions`, catalog `GET /models` | body `model` | `Authorization: Bearer` |
| `anthropic` | `POST /v1/messages`, catalog `GET /v1/models` | body `model` | `x-api-key` |
| `gemini` | `POST /{v1,v1beta}/models/{model}:generateContent` and `:streamGenerateContent`, catalog `GET /{v1,v1beta}/models` | the path | `x-goog-api-key` |

For a Gemini endpoint the gateway rewrites the model segment of the path for the target. For the others it rewrites the `model` member of the body.

An Anthropic or Gemini endpoint usually authenticates with the header its clients send, so set `spec.policies` to read it, for example `{ key: x-api-key, in: header }`.

## Define a Model

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: Model
metadata:
  name: gpt-4o
spec:
  displayName: GPT-4o
  version: v1.0
  modelName: gpt-4o                # what clients send as the model
  endpoints: [wso2]                # the endpoints that serve it (required)
  targets:
    - name: openai
      provider:
        id: openai-prod            # an LLM provider
      upstreamModel: gpt-4o        # what the provider knows it as
```

```bash
curl -u admin:admin -X POST http://localhost:9090/api/management/v1/models \
  -H 'Content-Type: application/yaml' --data-binary @gpt-4o.yaml
```

Only admins can create, update or delete Models; developers can list and read them.

Then call it like any OpenAI model:

```bash
curl http://localhost:8080/wso2/v1/chat/completions \
  -H "Authorization: Bearer $KEY" -H 'Content-Type: application/json' \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}'
```

The endpoint's catalog, `GET /wso2/v1/models`, lists the Models it serves.

The response names the Model as well: its `model` (`modelVersion` on a Gemini endpoint) is the `modelName` the client sent, whichever target answered, in every streamed event as in a whole response. Which model served the request is reported in [analytics](#analytics), not to the client.

Things to know about names and endpoints:
- `modelName` is unique across Models and matched exactly.
- A Model is served on each endpoint it names. On an endpoint it does not name, it is unknown.
- Every endpoint a Model names must exist, deployed or not, so create the endpoint first. A save naming one that does not is refused with `400`.
- Every endpoint a deployed Model names must be able to serve it: for each of its providers, a transformer entry or the endpoint's own format (see [Transformers](#transformers)). A save that fails this is refused with `400`, naming the endpoint and why.
- An endpoint that a deployed Model names cannot be deleted. Remove it from those Models first. An undeployed Model does not hold it; once the endpoint is gone, remove it from that Model before saving it again.

Some requests are answered with an error in the endpoint's format, always after authentication, so an unauthenticated caller cannot discover which models exist:
- a request naming no Model;
- a request naming a Model the endpoint does not serve;
- a body that is not JSON.

On an OpenAI endpoint the error codes are `missing_model`, `model_not_found` and `invalid_json`. On an Anthropic endpoint the body is `{"type": "error", "error": {"type": "not_found_error", ...}}`, and on a Gemini endpoint it is `{"error": {"code": 404, "status": "NOT_FOUND", ...}}`.

## How a target's provider is reached

Each target names an LLM provider. How the gateway reaches it is a property of the provider, set by its `exposure`:

| Provider `exposure` | The target is called | With the credential |
|---|---|---|
| `none` | **directly**: the gateway calls the provider's upstream itself — one hop | the provider's own `upstream.auth` |
| `route` (default) | **through the provider's route**, over the gateway's loopback — two hops, so the provider's own policies apply too | the target's `auth`, presented to the provider's route |

A provider with `exposure: none` publishes no route of its own and issues no API keys; it is reachable only through the Models that target it. Use it for a provider that exists only to serve Models:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: openai-prod
spec:
  displayName: OpenAI (Models)
  version: v1.0
  template: openai
  exposure: none
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: Authorization
      value: 'Bearer {{ secret "openai-key" }}'
```

A target on a provider that keeps its route presents a credential the provider's route accepts. This is the same `auth` block an LLM proxy's provider takes. When the provider's route uses `api-key-auth`, issue a key on the provider and give it to the target:

```yaml
  targets:
    - name: shared
      provider:
        id: shared-openai          # exposure: route, with api-key-auth
      upstreamModel: gpt-4o
      auth:
        type: api-key
        header: X-API-Key
        value: '{{ secret "shared-openai-model-key" }}'
```

The credential is write-only: it is never returned on a read, and an update that omits it keeps the stored one.

A routed provider has some extra requirements:
- Its `accessControl` must allow the paths the endpoint writes to it (see `upstreamPaths` below).
- It must be served on a vhost the loopback can name, not a wildcard pattern.
- It cannot authenticate its route with JWT, basic auth, mutual TLS or subscriptions. For such a provider, set its exposure to `none` instead.

## Transformers

A provider whose template matches the endpoint's format passes through untouched. Any other provider needs a transformer: a policy that converts the endpoint's format to the provider's and back. An endpoint lists its transformers in `spec.transformers`:

```yaml
  transformers:
    - template: anthropic                         # or provider: <handle>
      type: openai-to-anthropic-transformer       # an installed transformer policy, or none
      version: v1
      params:
        anthropicVersion: "{apiVersion:2023-06-01}"
      upstreamPaths: [/v1/messages]
```

For each of a Model's providers, the endpoint uses the first of these that applies:

1. The entry naming the provider (`provider: <handle>`).
2. The entry naming the provider's template, by handle or template group (`template: <name>`).
3. No transformer, when the provider's template is the endpoint's format.

When none applies, the endpoint cannot serve the Model, and saving the Model as deployed is refused with `400`. The error names the endpoint, its format and the provider. The shipped transformers all read OpenAI requests, so on an Anthropic or Gemini endpoint a provider of another format needs a transformer you write. A deployed Model is also refused when an entry's `{apiVersion}` has no default and the provider does not set it, or when a routed provider's `accessControl` does not allow a path the endpoint would call it on. An undeployed Model is not checked until it is deployed.

A change to a provider is not checked this way. If one leaves an endpoint unable to serve a Model, the Model keeps serving on its other endpoints, and the controller log says why (`Model is not served on this model endpoint`).

Within an entry:
- `params` are passed to the policy. `{apiVersion}` (or `{apiVersion:<default>}`) is filled from the provider's `spec.upstream.apiVersion`. `providerId` is set by the gateway.
- `upstreamPaths` are the paths the transformer writes, with `{upstreamModel}` and `{apiVersion}` placeholders. They are used only to check that a routed provider's `accessControl` allows them.

The provisioned `wso2` endpoint starts with these entries:

| Template | Transformer | Provider API version (`spec.upstream.apiVersion`) |
|---|---|---|
| `openai` | none needed | — |
| `azure-openai` | `openai-to-azure-openai-transformer`; `upstreamModel` is the deployment name | required |
| `anthropic` | `openai-to-anthropic-transformer` | optional, default `2023-06-01` |
| `gemini` | `openai-to-gemini-transformer` | optional, default `v1beta` |
| `awsbedrock` | `openai-to-bedrock-transformer` (Converse) | — |
| `mistralai` | `openai-to-mistral-transformer` | — |

An entry whose policy this gateway build does not include is left out.

An endpoint you create starts with only the entries you give it: copy the ones it needs. To serve a provider with its own wire format, install a transformer policy for it and add an entry naming its template or the provider.

An endpoint update that would stop it serving a Model it serves today is refused with `409`, naming the Model. For example, removing the transformer one of the Model's providers needs, or changing the format.

## Route between targets

A Model with more than one target needs a routing policy, and any model routing policy works. Its parameters are the policy's own, as the policy hub describes them. Where a parameter names a model (`model`, `defaultModel`, `modelName`), give a target's name or its upstream model. Where it names a provider (`provider`, `defaultProvider`, `providerName`), give the target's provider id; it is optional.

```yaml
spec:
  modelName: chat
  endpoints: [wso2]
  targets:
    - name: primary
      provider: { id: openai-prod }
      upstreamModel: gpt-4o
    - name: secondary
      provider: { id: anthropic-prod }
      upstreamModel: claude-sonnet-4-5
  routing:
    policy:
      name: model-weighted-round-robin
      version: v1
    params:
      models:
        - model: primary            # or: model: gpt-4o, provider: openai-prod
          weight: 3
        - model: secondary
          weight: 1
```

The router's parameters are passed to it as written. The gateway matches each choice the router makes to a target, in this order:

1. a provider and upstream model that belong to one target;
2. a target's name, with no provider or that target's own;
3. a provider only one target uses;
4. an upstream model only one target uses;
5. otherwise the first target.

The request goes to the target matched, with that target's upstream model. Nothing is checked when the Model is saved: a value that matches no target, such as a misspelt name, sends its requests to the first target.

`llm-header-router` names only a provider, so its mappings give the target's provider id (`provider`, `defaultProvider`). The cost-based router's `modelBudgets[].model` is matched against the model the client asked for, which is the Model's `modelName`, not against a target.

A provider whose upstream credential is `oauth2` can be combined with other providers only when the router names the provider before the request body is read: a round-robin router whose entries give `provider`, or `llm-header-router`. The token is acquired before the body is read, and an entry that names only a model is matched after it.

## API keys

An endpoint that authenticates with `api-key-auth` takes two kinds of key:

| Key | Issued with | Reaches | Catalog lists |
|---|---|---|---|
| Endpoint key | `POST /model-endpoints/{name}/api-keys` | every Model the endpoint serves | every Model the endpoint serves |
| Model key | `POST /models/{name}/api-keys` | that Model only, on every endpoint serving it that uses `api-key-auth` | that Model only |

```bash
curl -u admin:admin -X POST http://localhost:9090/api/management/v1/model-endpoints/wso2/api-keys \
  -H 'Content-Type: application/json' -d '{"name": "team-a"}'
curl -u admin:admin -X POST http://localhost:9090/api/management/v1/models/gpt-4o/api-keys \
  -H 'Content-Type: application/json' -d '{"name": "gpt-4o-app"}'
```

A Model key used for another Model, or for a model that does not exist, gets the same `401` as an invalid key, so it cannot discover which models exist. An endpoint key works only on its own endpoint.

A Model can have keys only while at least one endpoint it names authenticates with `api-key-auth`; otherwise issuing one returns `409`. Deleting a Model revokes its keys, and deleting an endpoint revokes the endpoint's.

Admins and consumers can manage keys, as for any other API.

## Policies

A Model's `spec.policies` apply to every request for that Model, after the endpoint's own policies and before routing. They see the request and response in the endpoint's format, whichever provider serves it. So a token-based rate limit or a guardrail on a Model behaves the same whatever its targets are, and works on endpoints of any format. Rate limits in a Model's policies are counted per Model; rate limits in an endpoint's policies are shared by all its Models.

A Model's `spec.faultPolicies` run when a request for it fails, before the endpoint's own fault policies.

## Analytics

Requests are reported under the model endpoint that received them, with:
- the provider that actually served them, not the endpoint's template;
- as the model, the one that served them, as its provider named it (for example `gpt-4o-mini-2024-07-18`), and as the requested model, the Model's name;
- a `modelAnalytics` block naming the Model, its target and the upstream model.

A request to a routed provider is reported once, by the model endpoint.

## Limits

- An OpenAI endpoint serves chat completions only (`POST <context>/chat/completions`), plus the catalog. An Anthropic endpoint serves Messages, and a Gemini endpoint serves `generateContent` and `streamGenerateContent`.
- The gateway ships transformers from the OpenAI format only. An Anthropic or Gemini endpoint passes through providers on its own template; any other provider needs a transformer policy you install.
- The format a transformer reads is not checked. An entry naming an OpenAI transformer on an Anthropic endpoint is accepted, and the requests it handles are mistranslated.
- A provider change that leaves an endpoint unable to serve a Model is logged, not refused.
- Providers on the `azureai-foundry` template cannot be Model targets yet.
- A retry goes to the same target; a request is not failed over to another target.
- Cost reporting for Bedrock and Gemini targets may miss the model name, since their templates read it from the request path.
- A streamed response object or event line larger than 10 MB is forwarded as the provider sent it, still naming the upstream model.
- Models and model endpoints are managed on the gateway; control-plane distribution of them is not available yet.
