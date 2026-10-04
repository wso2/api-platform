# Agent governance

The AI Gateway connects clients to AI agents over the Agent2Agent (A2A) protocol. An agent keeps speaking A2A, and the gateway becomes the address clients use to reach it, the place discovery is answered from, and the point where access control applies.

This page is for the **AI developer** who deploys an agent behind the gateway, and the **platform administrator** who governs it.

## What is an AI agent?

An AI agent is an application that takes a request in natural language and works out how to satisfy it.

A large language model (LLM) on its own can only answer from what it was trained on. It can't read today's inventory, call your billing system, or book anything. An agent closes that gap by pairing a model with tools: functions the model can ask to have run, each with a name, a description, and parameters. The description is what the model reads when it decides which tool fits the request.

Ask an agent to plan a three-day trip and the work splits in two. The model reads the request, decides which tools to call and with what arguments, and turns the results into an answer. The agent application runs those calls and hands the results back. Neither half does the other's job.

What makes an agent worth reaching over a protocol is that it holds this together: the model, the tools, the memory of the conversation, and the state of work that takes longer than one exchange.

## What is Agent2Agent?

Agent2Agent is an open protocol for one agent to work with another. It was developed by Google and is maintained as an open specification.

It gives agents four things:

- **Discovery.** An agent publishes an Agent Card, a document naming its identity, its skills, the protocol bindings it supports, and the addresses those bindings are served at. A client reads the card before it sends anything.
- **Capability negotiation.** The card declares what the agent can do, such as whether it streams, and which content types it accepts and returns. A client can then use text, structured forms, or media according to what both sides support.
- **Task management.** Work that outlives a single request becomes a task with a state the client can poll, subscribe to, or cancel. A completed task carries artifacts, which are the results the agent produced.
- **Opacity.** Agents collaborate without exposing their internals. A client sees skills, tasks, and artifacts, never the other agent's model, tools, prompts, or memory.

That last point is what separates A2A from calling an agent's REST API. Neither side has to know how the other is built.

## How an A2A exchange works

A typical exchange runs through the following steps:

1. The client fetches the agent's Agent Card, usually from `/.well-known/agent-card.json`.
2. The client reads the card's skills and `supportedInterfaces`, and picks a protocol binding to use.
3. The client sends a message with `SendMessage`, or with `SendStreamingMessage` when it wants results as they're produced.
4. The agent accepts the work and answers with a task, carrying an identifier and a state.
5. The client follows the task: reading it with `GetTask`, subscribing to it with `SubscribeToTask`, or consuming the stream it already opened.
6. The agent's model decides which of its own tools to call, and the agent runs them. None of this is visible to the client.
7. The agent moves the task to a terminal state and attaches the artifacts it produced.
8. The client reads the artifacts, or cancels the task with `CancelTask` if it no longer wants the result.

For long-running work, the client can register a push notification configuration instead of waiting, and the agent calls back when the task changes.

The client's requests in these steps (sending a message, reading, subscribing to, or cancelling a task, and managing push notification configurations) are A2A operations, and the gateway resolves each such request to one before deciding what to do with it. Agent Card discovery is served separately, and the client's choice of binding and the agent's own tool calls never pass through the gateway as operations. The full list is in [Expose an agent](expose-an-agent.md).

## What the gateway adds

An agent that serves A2A directly gives you no place to put access control, and publishes its own address to every client that reads its card. Putting the gateway in front of it gives you four things:

- **One governed address.** Clients reach the agent at a gateway context path, on the gateway's own host and certificate. The agent itself doesn't have to be routable from the client network.
- **Authentication and rate limiting per operation.** Because the gateway resolves each request to a canonical operation first, you can require a scope on `CancelTask` that you don't require on `GetTask`.
- **Agent Card control.** The gateway serves a card you author, or proxies the agent's own card and rewrites the addresses in it so clients come back through the gateway rather than going direct.
- **A guarded extended card.** The authenticated extended Agent Card is protected whether or not you configure it, so an agent behind the gateway can't publish it by omission.

## The Agent artifact

You deploy an agent as a resource of `kind: Agent`, alongside the `LlmProvider`, `LlmProxy`, and `Mcp` artifacts described in [Key concepts](../README.md#key-concepts). One `Agent` fronts one upstream agent:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: Agent
metadata:
  name: trip-planner-v1.0
spec:
  displayName: Trip Planner
  version: v1.0
  context: /trip-planner
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

Everything A2A-specific sits under `spec.a2a`. The rest of the spec — `context`, `upstream`, `vhost`, `resilience` — works as it does on any other gateway artifact.

## Protocol version and bindings

The gateway serves A2A protocol version `1.0`. An `Agent` states one version in `spec.a2a.protocolVersion`, and that choice decides its operation set, its HTTP paths, and the Agent Card model its card is validated against.

The gateway performs no protocol version negotiation and no conversion between versions. A request that states a version other than the one the `Agent` exposes is rejected rather than downgraded.

A2A 1.0 defines eleven operations. The gateway serves them over the protocol's two HTTP bindings:

| Binding | How a request names its operation |
|---|---|
| `JSONRPC` | One endpoint, with the operation in the JSON-RPC `method` field |
| `HTTP+JSON` | One route per operation, with the operation in the method and path |

Expose either binding or both. The gateway resolves a request on either one to the same canonical operation, so a JSON-RPC call and its HTTP+JSON equivalent run the identical policy chain. For the operation table and the transport configuration, see [Expose an agent](expose-an-agent.md).

## What the gateway doesn't do

The gateway proxies A2A rather than reimplementing it. Task state, message content, artifact generation, tool calling, and streaming semantics all belong to the agent upstream.

Two exceptions are worth knowing about, because both are things the gateway answers itself:

- **Agent Card serving**, when you configure a managed card. The request never reaches the agent.
- **Protocol version validation**, which runs before anything is forwarded.

## In this section

| Page | What it covers |
|---|---|
| [Quick start guide](quick-start-guide.md) | Deploy an agent behind a running gateway and invoke it on both bindings. |
| [Expose an agent](expose-an-agent.md) | Context, upstream, transports, and the eleven A2A operations with their gateway paths. |
| [Agent Card](agent-card.md) | Serve a card the gateway holds, proxy the agent's own, and guard the extended card. |
| [Apply policies](apply-policies.md) | The three policy scopes an Agent carries and the order they run in. |
| [Authenticate clients](authenticate-clients.md) | Protect operations with API keys or JWTs, and issue keys for an agent. |
| [Streaming and timeouts](streaming-and-timeouts.md) | Long-lived streaming operations and the timeouts that apply to them. |

## Related topics

- [Agent configuration reference](../reference/agent-configuration.md) — every `spec` field, its default, and the rules the controller enforces at deploy time.
- [Agent management](../../rest-apis/gateway/agent-management.md) — the management API operations that create, update, and delete agents.
- [Key concepts](../README.md#key-concepts) — how the `Agent` artifact relates to LLM providers, LLM proxies, and MCP proxies.
