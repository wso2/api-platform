# Agent configuration reference

This page lists every field in the `Agent` artifact, its default, and the rules the gateway controller enforces when you deploy one. For the tasks these fields serve, start at [Agent governance](../agent-governance/index.md).

## Top-level fields

An `Agent` document carries four top-level fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `apiVersion` | string | Yes | Must be `gateway.api-platform.wso2.com/v1`. |
| `kind` | string | Yes | Must be `Agent`. |
| `metadata` | object | Yes | Resource metadata, including `name` and optional `annotations`. |
| `spec` | object | Yes | The agent configuration described below. |

## `spec`

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `displayName` | string | Yes | — | Human-readable name, 1 to 100 characters. Letters, numbers, spaces, hyphens, underscores, and dots only. |
| `version` | string | Yes | — | Agent version, matching `vMAJOR.MINOR`, such as `v1.0`. |
| `context` | string | No | — | Base path for every generated route. Starts with `/`, has no trailing slash, and is at most 200 characters. Omit it to serve the agent at the root of its virtual host. |
| `vhost` | string | No | — | Virtual host the agent is published under. Accepts a domain or subdomain, with a wildcard permitted in the left-most label, up to 253 characters. |
| `upstream` | object | Yes | — | The agent behind the gateway. See [`upstream`](#upstream). |
| `upstreamDefinitions` | array | No | — | Reusable upstream definitions, referenced by `upstream.ref`. |
| `deploymentState` | string | No | `deployed` | `deployed` or `undeployed`. An undeployed agent leaves router traffic but keeps its configuration, policies, and API keys. |
| `resilience` | object | No | — | Route timeouts for this agent's traffic-forwarding routes. See [`resilience`](#resilience). |
| `a2a` | object | Yes | — | A2A protocol configuration. See [`a2a`](#a2a). |

### `upstream`

| Field | Type | Required | Description |
|---|---|---|---|
| `url` | string | One of `url` or `ref` | Base URL the gateway forwards A2A traffic to. In public passthrough card mode, it's also where the gateway fetches the Agent Card from. |
| `ref` | string | One of `url` or `ref` | Name of an entry in `upstreamDefinitions`. |
| `auth` | object | No | Credential the gateway presents to the agent. See [Authenticate backends](https://github.com/wso2/docs-api-platform/blob/main/en/docs/ai-gateway/next/authenticate-backends.md). |

### `resilience`

Both values take a duration such as `500ms`, `30s`, `5m`, or `1h`. Setting either to `0s` disables it.

| Field | Type | Default | Description |
|---|---|---|---|
| `timeout` | string | Disabled on the JSON-RPC route and on streaming HTTP+JSON routes; the gateway's global route timeout elsewhere | Maximum time for the whole route, from request to upstream response. |
| `idleTimeout` | string | The listener's stream idle timeout | Per-route stream idle timeout. Remains the liveness guard for a stream whose route timeout is disabled. |

`resilience` also appears per operation, under `a2a.operationConfigs.operations[]`, where it takes precedence over this agent-level value for that operation's HTTP+JSON route. Each field falls back to the agent-level value on its own. Per-operation `resilience` doesn't apply to the JSON-RPC binding: every JSON-RPC operation shares one route, and that route takes only this agent-level value.

## `a2a`

| Field | Type | Required | Description |
|---|---|---|---|
| `protocolVersion` | string | Yes | The A2A protocol version this agent exposes. `1.0` is the supported value. |
| `operationConfigs` | object | Yes | Transports and policies. See [`a2a.operationConfigs`](#a2aoperationconfigs). |
| `agentCard` | object | No | Agent Card serving. See [`a2a.agentCard`](#a2aagentcard). |

`protocolVersion` selects the agent's operation set, its HTTP+JSON bindings, and the Agent Card model a managed card is validated against. An agent exposes exactly one version, and the gateway converts between versions for no one.

### `a2a.operationConfigs`

| Field | Type | Required | Description |
|---|---|---|---|
| `transports` | array | Yes | One or two entries, one per protocol binding. See [`transports`](#a2aoperationconfigstransports). |
| `policies` | array | No | Ordered policies applied to every A2A operation, before per-operation policies. Never applied to public Agent Card serving. |
| `operations` | array | No | Per-operation configuration, keyed by canonical operation name. See [`operations`](#a2aoperationconfigsoperations). |

### `a2a.operationConfigs.transports`

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `protocolBinding` | string | Yes | — | `JSONRPC` or `HTTP+JSON`. |
| `pathPrefix` | string | No | `/` | Gateway-facing path prefix, relative to `context`. For `JSONRPC` it's the endpoint path; for `HTTP+JSON`, operation paths are appended below it. |

A `pathPrefix` travels upstream with the request: the gateway strips only `context`. The prefix must therefore match the path the upstream agent serves that binding at.

At most two transports are allowed, and each binding may appear once.

### `a2a.operationConfigs.operations`

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Canonical A2A operation name, from the set defined by `protocolVersion`. |
| `policies` | array | No | Ordered policies applied after the common policies when this operation is selected. |
| `resilience` | object | No | Route timeouts for this operation's route. |

This array isn't an allowlist. An operation you leave out still serves traffic and still runs the common policies.

The eleven A2A 1.0 operation names are listed in [Expose an agent](../agent-governance/expose-an-agent.md).

### `a2a.agentCard`

| Field | Type | Required | Description |
|---|---|---|---|
| `public` | object | No | Public Agent Card serving. See [`public`](#a2aagentcardpublic). |
| `protected` | object | No | Authenticated extended Agent Card. See [`protected`](#a2aagentcardprotected). |

Omitting the whole block, or just `public`, serves the public card in `passthrough` mode at `/.well-known/agent-card.json`, with interface URL rewriting enabled and no card policies.

Omitting `protected` is not equivalent. It leaves the extended card guarded and is never turned into an explicit protected configuration.

#### `a2a.agentCard.public`

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `mode` | string | No | `passthrough` | `managed` serves a document the gateway holds; `passthrough` proxies the agent's own. |
| `content` | object | In `managed` mode | — | The complete Agent Card, embedded as JSON. Stored and served exactly as supplied. |
| `path` | string | No | `/.well-known/agent-card.json` | Gateway-facing card path, relative to `context`. Replaces the default route rather than adding an alias. |
| `rewriteUrls` | boolean | No | `true` | Rewrites `supportedInterfaces[].url` in a proxied response to the gateway's own endpoints. Valid in `passthrough` mode only. |
| `policies` | array | No | — | Ordered policies applied only to public card serving. |
| `signing` | object | No | — | Gateway card signing. Rejected at deploy time — see [Unsupported fields](#unsupported-fields). |

#### `a2a.agentCard.protected`

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `mode` | string | Yes when the block is written | `passthrough` | `managed` serves a document the gateway holds; `passthrough` forwards the authenticated request. |
| `content` | object | In `managed` mode | — | The complete extended Agent Card, embedded as JSON. |
| `rewriteUrls` | boolean | No | `true` | As for the public card. Valid in `passthrough` mode only. |
| `signing` | object | No | — | Rejected at deploy time — see [Unsupported fields](#unsupported-fields). |

The protected card has no `path` and no `policies` of its own. It's served through the `GetExtendedAgentCard` operation and runs that operation's chain.

The gateway requires the request to have been authenticated by a policy in the agent's own chain before it returns or proxies the protected card, and answers `401` otherwise. This applies in every mode and isn't configurable.

## Deploy-time validation

The controller rejects a deployment rather than serving a configuration that would behave differently from what it says. The rules below are the ones most likely to stop a first deployment.

### Mode rules

| Rule | Applies to |
|---|---|
| `managed` requires `content`. | Both cards |
| `passthrough` accepts neither `content` nor `signing`. | Both cards |
| `rewriteUrls` is valid in `passthrough` mode only, in either polarity. | Both cards |
| A public card that's `managed` alongside a configured protected card must declare `capabilities.extendedAgentCard: true`. | Public card |

### Managed card content rules

| Rule |
|---|
| The document must not carry a `signatures` block. |
| The encoded document must not exceed 1 MiB. |
| `supportedInterfaces` must be present and non-empty. |
| Every configured transport needs an interface advertising its binding, and no interface may advertise a binding the transports don't expose. |
| Each binding may be advertised once. |
| Each interface's `protocolVersion` must equal `spec.a2a.protocolVersion`. |
| Each `url` must be absolute, use `https`, and carry no userinfo, query string, or fragment. |
| Each `url` path must equal `context` plus that transport's `pathPrefix`. |
| An interface must not declare `tenant`. |

### Other rules

| Rule |
|---|
| `protocolVersion` must be a supported version. |
| At most two transports, one per binding. |
| An operation name must belong to the set defined by `protocolVersion`, and may be configured once. |
| Generated routes must not collide with each other. |

## Unsupported fields

| Field | Behavior |
|---|---|
| `a2a.agentCard.public.signing.enabled: true` | Rejected with `Agent Card signing is not supported yet; set enabled: false or omit the signing block`. |
| `a2a.agentCard.protected.signing.enabled: true` | Rejected with the same message, reported against the block you wrote. |

## Complete example

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: trip-planner-v1.0
  annotations:
    "gateway.api-platform.wso2.com/project-id": "default"
spec:
  displayName: Trip Planner
  version: v1.0
  context: /trip-planner
  vhost: agents.example.com
  upstream:
    url: http://host.docker.internal:9099
  resilience:
    idleTimeout: 5m
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /
        - protocolBinding: HTTP+JSON
          pathPrefix: /v1
      policies:
        - name: jwt-auth
          version: v1
          params:
            issuers:
              - PrimaryIdp
      operations:
        - name: CancelTask
          policies:
            - name: jwt-auth
              version: v1
              params:
                issuers:
                  - PrimaryIdp
                scopes:
                  allOf:
                    - "trip:cancel"
    agentCard:
      public:
        mode: managed
        content: {
          "name": "Trip Planner",
          "description": "Plans multi-day itineraries",
          "version": "1.0.0",
          "protocolVersion": "1.0",
          "supportedInterfaces": [
            {
              "protocolBinding": "JSONRPC",
              "protocolVersion": "1.0",
              "url": "https://agents.example.com/trip-planner"
            },
            {
              "protocolBinding": "HTTP+JSON",
              "protocolVersion": "1.0",
              "url": "https://agents.example.com/trip-planner/v1"
            }
          ],
          "capabilities": {
            "streaming": true,
            "extendedAgentCard": true
          },
          "defaultInputModes": ["text/plain"],
          "defaultOutputModes": ["text/plain"],
          "skills": [
            {
              "id": "plan_trip",
              "name": "Plan a trip",
              "description": "Plans an itinerary for a destination and day count",
              "tags": ["travel"]
            }
          ]
        }
      protected:
        mode: passthrough
```

## Related topics

- [Expose an agent](../agent-governance/expose-an-agent.md) — the fields that decide where an agent is reachable.
- [Agent Card](../agent-governance/agent-card.md) — what the card blocks above do at runtime.
- [Agent management](../../rest-apis/gateway/agent-management.md) — the management API operations that accept this configuration.
