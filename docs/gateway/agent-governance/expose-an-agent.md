# Expose an agent

An `Agent` gives an upstream Agent2Agent (A2A) agent an address on the gateway and decides which protocol bindings that address serves. This page covers the fields that shape those routes, and lists the gateway path each A2A operation ends up at.

## Configure the agent

A complete agent needs a display name, a version, an upstream, and an `a2a` block:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: trip-planner-v1.0
spec:
  displayName: Trip Planner
  version: v1.0
  context: /trip-planner
  vhost: agents.example.com
  upstream:
    url: http://host.docker.internal:9099
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /
        - protocolBinding: HTTP+JSON
          pathPrefix: /v1
```

Deploy it through the management API:

```bash
curl -X POST http://localhost:9090/api/management/v1/agents \
  -H "Content-Type: application/yaml" \
  -u "$ADMIN_USERNAME:$ADMIN_PASSWORD" \
  --data-binary "@trip-planner.yaml"
```

The table below covers the fields that decide where the agent is reachable. For every field, including the ones this page doesn't cover, see the [agent configuration reference](../reference/agent-configuration.md).

| Field | Required | What it does |
|---|---|---|
| `displayName` | Yes | Human-readable name, up to 100 characters. |
| `version` | Yes | Agent version, in the form `v1.0`. |
| `context` | No | Base path for every route the gateway generates. Omit it to serve the agent at the root of its virtual host. |
| `vhost` | No | Virtual host the agent is published under. Accepts a domain, a subdomain, or a wildcard in the left-most label. |
| `upstream.url` | Yes | Base URL the gateway forwards A2A traffic to. |
| `a2a.protocolVersion` | Yes | The A2A protocol version this agent exposes. |
| `a2a.operationConfigs.transports` | Yes | The protocol bindings to serve, and the path prefix for each. |

### Choose a context

`context` is the base path for every route: the transport prefixes and the Agent Card path both hang off it.

Omitting it serves the agent at the root of its virtual host, which is where an A2A client probes for `/.well-known/agent-card.json` during cold discovery. Give an agent its own virtual host when you want that, and a context when several agents share a host.

### Authenticate to the upstream agent

If the agent behind the gateway requires a credential, add it under `upstream.auth`:

```yaml
  upstream:
    url: http://host.docker.internal:9099
    auth:
      type: api-key
      header: x-api-key
      value: 12345
```

Store the value as a gateway secret rather than writing it into the configuration. See [Authenticate backends](https://github.com/wso2/docs-api-platform/blob/main/en/docs/ai-gateway/next/authenticate-backends.md).

## Expose protocol bindings

A2A defines two HTTP bindings, and `transports` decides which of them the gateway serves. List one or both:

| Binding | Shape | Typical prefix |
|---|---|---|
| `JSONRPC` | One endpoint carrying every operation, with the operation named in the JSON-RPC `method` field | `/` |
| `HTTP+JSON` | One route per operation, with the operation in the HTTP method and path | `/v1` |

`pathPrefix` is the path the binding is served at, relative to `context`. A prefix travels upstream with the request: the gateway strips `context` and forwards the rest. The prefixes you configure therefore have to match the paths the upstream agent actually serves.

The agent in the example above serves JSON-RPC at `/` and HTTP+JSON under `/v1`, so a request routes like this:

| Stage | Path |
|---|---|
| Client sends | `https://agents.example.com/trip-planner/v1/message:send` |
| Gateway forwards | `http://host.docker.internal:9099/v1/message:send` |

Exposing both bindings gives clients a choice without giving you two things to govern. The gateway resolves a request on either binding to the same canonical operation, and that operation's policy chain runs whichever binding the client picked.

## A2A operations and their gateway paths

A2A 1.0 defines eleven operations. Under JSON-RPC, a client names the operation in the `method` field, spelled exactly as the canonical name. Under HTTP+JSON, each operation has its own method and path, appended to that transport's prefix.

The following table gives both. Paths are relative to `context` plus the HTTP+JSON `pathPrefix`:

| Canonical operation | JSON-RPC `method` | HTTP+JSON |
|---|---|---|
| `SendMessage` | `SendMessage` | `POST /message:send` |
| `SendStreamingMessage` | `SendStreamingMessage` | `POST /message:stream` |
| `GetTask` | `GetTask` | `GET /tasks/{id}` |
| `ListTasks` | `ListTasks` | `GET /tasks` |
| `CancelTask` | `CancelTask` | `POST /tasks/{id}:cancel` |
| `SubscribeToTask` | `SubscribeToTask` | `POST /tasks/{id}:subscribe` |
| `CreateTaskPushNotificationConfig` | `CreateTaskPushNotificationConfig` | `POST /tasks/{id}/pushNotificationConfigs` |
| `GetTaskPushNotificationConfig` | `GetTaskPushNotificationConfig` | `GET /tasks/{id}/pushNotificationConfigs/{configId}` |
| `ListTaskPushNotificationConfigs` | `ListTaskPushNotificationConfigs` | `GET /tasks/{id}/pushNotificationConfigs` |
| `DeleteTaskPushNotificationConfig` | `DeleteTaskPushNotificationConfig` | `DELETE /tasks/{id}/pushNotificationConfigs/{configId}` |
| `GetExtendedAgentCard` | `GetExtendedAgentCard` | `GET /extendedAgentCard` |

These canonical names are what you write in `spec.a2a.operationConfigs.operations` to configure one operation on its own. See [Apply policies](apply-policies.md).

> [!NOTE]
> **`SubscribeToTask` is a `POST`**
>
> The gateway serves `SubscribeToTask` as a `POST`, following the A2A specification document. The protocol's own `.proto` definition disagrees with the specification here and maps it to `GET`. A client generated from the `.proto` sends `GET` and receives a `404`.

## State the protocol version on every request

A client states the A2A protocol version on every operation request, in either of two places:

- An `A2A-Version` header, as in `A2A-Version: 1.0`.
- An `A2A-Version` query parameter, as in `?A2A-Version=1.0`.

The gateway validates the stated version against the version the route exposes, before it binds the policy chain and before anything reaches the agent. A request that passes is forwarded with its header and query string byte for byte as sent, so the agent can apply the same rule itself.

Four things cause a rejection:

- **No version stated.** A2A 1.0 reads silence as version `0.3`, so an agent exposing `1.0` treats an absent value as a mismatch. This is what makes stating the version mandatory in practice.
- **A version other than the one the route exposes.** The gateway negotiates nothing and converts nothing, because an agent exposes exactly one version.
- **A value that isn't a canonical `Major.Minor` version.**
- **Conflicting or repeated values**, including the same value twice. Intermediaries collapse duplicates differently, so accepting them would make the effective version depend on the route a request happened to take. Sending both the header and the query parameter is fine as long as they agree exactly.

Agent Card requests are exempt from all of this. A card is a discovery document rather than an operation, and it's how a client learns which version to state.

## Undeploy without deleting

Set `deploymentState` to `undeployed` to take an agent out of router traffic while keeping its configuration, policies, and API keys:

```yaml
spec:
  deploymentState: undeployed
```

Set it back to `deployed`, the default, to restore it.

## Related topics

- [Agent Card](agent-card.md) — the discovery document clients read before sending any of the operations above.
- [Apply policies](apply-policies.md) — attach policies to every operation, or to one of them by canonical name.
- [Streaming and timeouts](streaming-and-timeouts.md) — why the streaming operations in the table get different timeout defaults.
- [Agent configuration reference](../reference/agent-configuration.md) — every field in the `Agent` spec.
