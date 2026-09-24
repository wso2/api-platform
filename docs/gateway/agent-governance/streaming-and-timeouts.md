# Streaming and timeouts

Two Agent2Agent (A2A) operations return a stream of events rather than a single response, and they stay open as long as the agent has something to say. A timeout tuned for a request-response API cuts those streams off mid-task, so the gateway treats them differently by default.

## The streaming operations

| Operation | HTTP+JSON | What it streams |
|---|---|---|
| `SendStreamingMessage` | `POST /message:stream` | Events for the task the message started |
| `SubscribeToTask` | `POST /tasks/{id}:subscribe` | Events for a task already running |

Both return Server-Sent Events (SSE). The gateway forwards the stream as the agent produces it and doesn't buffer, reorder, or reframe events. Streaming semantics belong to the agent.

A client discovers whether an agent streams at all from `capabilities.streaming` in its Agent Card. See [Agent Card](agent-card.md).

## Timeout defaults

A route timeout caps the time from request to upstream response. For a stream, that ceiling is the length of the whole stream, which is why the gateway disables it by default on the routes that carry one:

| Route | Default route timeout |
|---|---|
| The JSON-RPC endpoint | Disabled |
| HTTP+JSON streaming operations | Disabled |
| Every other HTTP+JSON operation | The gateway's global route timeout |

The JSON-RPC endpoint is disabled because it's one route. Every operation arrives at the same path, with the operation named in the body, so a route-level timeout there would apply to streaming and non-streaming calls alike.

This differs from REST APIs and LLM proxies, which fall back to the global route timeout on every route.

> [!NOTE]
> **The idle timeout is the liveness guard**
>
> Disabling the route timeout doesn't leave a dead connection open. The stream idle timeout still applies, and it's the right control for a stream: it measures the gap between events rather than the total duration, so it closes a stream that has stopped producing without cutting off one that's still working.

## Set timeouts on an agent

Set `resilience` at the agent level to apply a limit to every traffic-forwarding route:

```yaml
spec:
  displayName: Trip Planner
  version: v1.0
  context: /trip-planner
  upstream:
    url: http://host.docker.internal:9099
  resilience:
    timeout: 60s
    idleTimeout: 5m
```

Both values take a duration such as `500ms`, `30s`, `5m`, or `1h`. Setting either to `0s` disables it.

Set `timeout` here only when you want a hard ceiling on how long any single call may run, streams included. It overrides the disabled default, so a stream that outlives it is cut off.

## Set timeouts on one operation

Per-operation `resilience` sits beside `policies` in the same `operations` entry, and takes precedence over the agent-level value for that operation's route:

```yaml
  a2a:
    operationConfigs:
      operations:
        - name: SendMessage
          resilience:
            timeout: 30s
        - name: SendStreamingMessage
          resilience:
            idleTimeout: 2m
```

This is the combination most agents want: a firm ceiling on the request-response operations, and an idle timeout on the streaming ones so a stalled stream closes while a long, productive one doesn't.

Per-operation timeouts apply to HTTP+JSON routes, which is where each operation has a route of its own. On the JSON-RPC endpoint, every operation shares one route and therefore one timeout.

## Test a streaming operation

Request a stream and watch events arrive:

```bash
curl -N -s -X POST http://localhost:8080/trip-planner/v1/message:stream \
  -H 'Content-Type: application/json' \
  -H 'A2A-Version: 1.0' \
  -d '{"message":{"messageId":"m1","role":"ROLE_USER",
       "parts":[{"text":"Plan a 3 day trip to Ella slowly"}]}}'
```

`-N` disables `curl`'s output buffering, so events print as they arrive rather than at the end.

## Related topics

- [Expose an agent](expose-an-agent.md) — the transports that decide which routes exist to put a timeout on.
- [Apply policies](apply-policies.md) — rate limiting a streaming operation without limiting the rest.
- [Timeouts and resilience](https://github.com/wso2/docs-api-platform/blob/main/en/docs/ai-gateway/next/timeouts-and-resilience.md) — the gateway-wide timeout settings these values override.
