# A2A Trip Planner — integration-test agent

A trip-planning agent that speaks the A2A protocol over both HTTP bindings. It is
the upstream behind every `Agent` deployed by the gateway integration tests
(`gateway/it/features/agent_*.feature`).

**It is a real A2A agent, not a mock.** It is built on the official
[`a2a-sdk`](https://pypi.org/project/a2a-sdk/), and everything protocol-shaped —
the task lifecycle, SSE framing, error shapes, the eleven operations, the
capability gating — is the SDK's. That is deliberate: the gateway's A2A tests
exist to check the gateway against the *protocol*, and a hand-written stand-in
would only ever check it against whatever framing the stand-in happened to
choose. It lives under `tests/mock-servers/` for consistency with its neighbours,
not because it mocks anything.

What *is* ours is the content. Every string the agent produces is fixed and
derived from the request, so a feature file can assert on it exactly rather than
matching loosely and passing on a wrong answer.

---

## Layout

```
/                            JSON-RPC binding — all eleven operations, one endpoint
/v1/...                      HTTP+JSON binding — one route per operation
/.well-known/agent-card.json public Agent Card
```

These two prefixes are the *agent's* own layout, not the gateway's. An `Agent`
resource declares them as `pathPrefix` values, and a `pathPrefix` travels
upstream with the request (only `spec.context` is stripped), so these paths and
those prefixes have to agree:

```yaml
transports:
  - protocolBinding: JSONRPC
    pathPrefix: /
  - protocolBinding: HTTP+JSON
    pathPrefix: /v1
```

## What it does

Ask it to plan a trip and it produces a deterministic itinerary.

| Message | Result |
|---|---|
| `Plan a 2 day trip to Galle` | completed task, itinerary artifact |
| `Plan a 3 day trip to Ella slowly` | task parked in `TASK_STATE_WORKING` for the configured hold |
| anything unparseable | defaults: destination `Kandy`, 3 days |

The artifact text is fully determined by destination and day count:

```
Trip plan for Galle: 2 days
Day 1: Temple visits in Galle
Day 2: Botanical gardens in Galle
```

Activities cycle through `Temple visits, Botanical gardens, A lake walk,
Tea country day trip, Local markets`. Day count is clamped to 1–14.

**Slow mode** (`slowly` anywhere in the message) is what makes `GetTask`,
`ListTasks`, `CancelTask` and `SubscribeToTask` meaningful. Without it every task
is terminal before the next request arrives, and those four operations would all
be exercised against a finished task — which is not the state any of them exist
for. A cancel genuinely stops the hold rather than just publishing a cancelled
status.

**Streaming** requests emit paced status updates (`Planning step 1/3: checking
flights`, …) before the artifact, so a client can observe an event arriving while
the task is still running. That gap is the whole point: a buffered response
delivered in SSE framing contains the same events in the same order, and the only
thing distinguishing it from a real stream is *when* the first one arrives.

## Running it

### Locally, with Python

Requires Python 3.12+.

```bash
cd tests/mock-servers/a2a-trip-planner
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python main.py
# Trip Planner listening on http://0.0.0.0:9099 (JSON-RPC /, HTTP+JSON /v1)
```

### As a container

```bash
docker build -t a2a-trip-planner:local .
docker run --rm -p 9099:9099 a2a-trip-planner:local
```

### As part of the integration-test stack

It comes up automatically with `docker-compose.test.yaml` as service
`a2a-trip-planner`, published on host port `9099`. Other containers reach it at
`http://a2a-trip-planner:9099`, which is what the `Agent` resources in the
feature files use as their upstream.

## Configuration

All optional; the defaults give a fast, deterministic agent.

| Variable | Default | Purpose |
|---|---|---|
| `TRIP_BIND_HOST` | `0.0.0.0` | Bind address |
| `TRIP_PORT` | `9099` | Port |
| `TRIP_RPC_PATH` | `/` | Where the JSON-RPC binding is mounted |
| `TRIP_REST_PATH` | `/v1` | Where the HTTP+JSON binding is mounted |
| `TRIP_STREAM_STEPS` | `3` | Paced status updates before the artifact |
| `TRIP_STREAM_STEP_DELAY` | `0.4` | Seconds between them |
| `TRIP_SLOW_HOLD_SECONDS` | `30` | How long slow mode holds a task in `working` |
| `TRIP_SLOW_TICK` | `1.0` | Keep-working status interval during the hold |
| `TRIP_PUBLIC_URL` | `http://localhost:$TRIP_PORT` | Base URL advertised in the agent's own card |

The compose stack sets `TRIP_STREAM_STEP_DELAY=0.5` and
`TRIP_SLOW_HOLD_SECONDS=60`.

---

## Read this before writing a client

Four things bite, and all four are the SDK's behaviour rather than this
fixture's. Every command below was run against the container.

### 1. `A2A-Version: 1.0` is mandatory on every operation request

A2A 1.0 §3.6.1 makes declaring the protocol version the *client's* job, and
§3.6.2 says an absent or empty value means `0.3` — which a 1.0 agent rejects.
Every dispatcher method is decorated with `validate_version`, so this is not
optional on any operation, on either binding.

The failure differs by binding, which is useful for telling them apart:

```console
$ curl -s -X POST http://localhost:9099/ -H 'Content-Type: application/json' \
    -d '{"jsonrpc":"2.0","id":1,"method":"ListTasks","params":{}}'
{"error":{"code":-32009,"message":"A2A version '0.3' is not supported by this handler. Expected version '1.0'."...

$ curl -s -o /dev/null -w '%{http_code}\n' http://localhost:9099/v1/tasks
400
```

Note the JSON-RPC failure rides an **HTTP 200**. The gateway does not inject or
validate this header — Section 8A of the implementation plan, the pre-resolution
guard that would have, is not implemented — so a client talking through the
gateway must send it too.

The Agent Card is exempt: it is a static discovery document, not an operation.

### 2. Task identifiers are a flat `id`, never `name: "tasks/<id>"`

The v0.3 resource-name shape is rejected outright
(`GetTaskRequest has no field named "name"`). Push-notification configs take
`{"taskId": …, "id": …}`.

### 3. `configuration.returnImmediately` is how you get a live task

Without it, a `SendMessage` in slow mode blocks for the entire hold. (`blocking`
is not a field.)

### 4. The two bindings frame SSE payloads differently

```
JSON-RPC:   data: {"result": {"statusUpdate": {...}}, "id": 1, "jsonrpc": "2.0"}
HTTP+JSON:  data: {"statusUpdate": {...}}
```

An assertion written against one shape will not match the other.

---

## Worked examples

```bash
V='A2A-Version: 1.0'
H='Content-Type: application/json'
```

### The cards

```bash
curl -s http://localhost:9099/.well-known/agent-card.json     # skills: [plan_trip]
curl -s -H "$V" http://localhost:9099/v1/extendedAgentCard    # skills: [plan_trip, book_trip]
```

`book_trip` appears **only** in the extended card. The gateway serves the public
card itself, so its presence in a response is proof the request reached the
agent — which is exactly how the `GetExtendedAgentCard` test distinguishes
"proxied upstream" from "answered locally".

### SendMessage

```bash
curl -s -X POST http://localhost:9099/v1/message:send -H "$H" -H "$V" \
  -d '{"message":{"messageId":"m1","role":"ROLE_USER",
       "parts":[{"text":"Plan a 2 day trip to Galle"}]}}'
```

The JSON-RPC equivalent wraps the same params, and its result is under `result`:

```bash
curl -s -X POST http://localhost:9099/ -H "$H" -H "$V" \
  -d '{"jsonrpc":"2.0","id":1,"method":"SendMessage",
       "params":{"message":{"messageId":"m1","role":"ROLE_USER",
                 "parts":[{"text":"Plan a 2 day trip to Galle"}]}}}'
```

### A live task: create, inspect, cancel

Note the `${TASK}` braces — in zsh, `$TASK:subscribe` parses as a parameter
modifier and fails with `bad substitution`.

```bash
TASK=$(curl -s -X POST http://localhost:9099/v1/message:send -H "$H" -H "$V" \
  -d '{"message":{"messageId":"m2","role":"ROLE_USER",
       "parts":[{"text":"Plan a 3 day trip to Ella slowly"}]},
       "configuration":{"returnImmediately":true}}' \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["task"]["id"])')

curl -s -H "$V" "http://localhost:9099/v1/tasks/${TASK}"   # TASK_STATE_WORKING
curl -s -H "$V"  http://localhost:9099/v1/tasks            # ListTasks

curl -s -X POST -H "$V" "http://localhost:9099/v1/tasks/${TASK}:cancel"
# TASK_STATE_CANCELED
```

### Streaming

```bash
curl -sN -X POST http://localhost:9099/v1/message:stream -H "$H" -H "$V" \
  -d '{"message":{"messageId":"m3","role":"ROLE_USER",
       "parts":[{"text":"Plan a 2 day trip to Kandy"}]}}'
```

`-N` matters: without it curl buffers and you lose the timing that makes this a
stream. Responses are `text/event-stream` + `Transfer-Encoding: chunked` with
**no** `content-length`.

`SubscribeToTask` re-attaches to a running task. The SDK registers both verbs;
the gateway routes only `POST` (the specification document and the proto disagree
— see `gateway/gateway-controller/specification/a2a/v1.0/SOURCE`):

```bash
curl -sN -X POST -H "$V" "http://localhost:9099/v1/tasks/${TASK}:subscribe"
```

### Push notification configs

```bash
curl -s -X POST "http://localhost:9099/v1/tasks/${TASK}/pushNotificationConfigs" \
  -H "$H" -H "$V" -d '{"id":"cfg-1","url":"https://example.invalid/hook"}'
curl -s -H "$V" "http://localhost:9099/v1/tasks/${TASK}/pushNotificationConfigs/cfg-1"
curl -s -H "$V" "http://localhost:9099/v1/tasks/${TASK}/pushNotificationConfigs"
curl -s -X DELETE -H "$V" "http://localhost:9099/v1/tasks/${TASK}/pushNotificationConfigs/cfg-1"
```

## Operation reference

All eleven, both bindings. JSON-RPC method names are the canonical operation
names; params go in `params`.

| Operation | JSON-RPC params | HTTP+JSON |
|---|---|---|
| `SendMessage` | `{"message": …}` | `POST /v1/message:send` |
| `SendStreamingMessage` | `{"message": …}` | `POST /v1/message:stream` |
| `GetTask` | `{"id": "<task>"}` | `GET /v1/tasks/<task>` |
| `ListTasks` | `{}` | `GET /v1/tasks` |
| `CancelTask` | `{"id": "<task>"}` | `POST /v1/tasks/<task>:cancel` |
| `SubscribeToTask` | `{"id": "<task>"}` | `POST /v1/tasks/<task>:subscribe` |
| `CreateTaskPushNotificationConfig` | `{"taskId": …, "id": …, "url": …}` | `POST /v1/tasks/<task>/pushNotificationConfigs` |
| `GetTaskPushNotificationConfig` | `{"taskId": …, "id": …}` | `GET /v1/tasks/<task>/pushNotificationConfigs/<id>` |
| `ListTaskPushNotificationConfigs` | `{"taskId": …}` | `GET /v1/tasks/<task>/pushNotificationConfigs` |
| `DeleteTaskPushNotificationConfig` | `{"taskId": …, "id": …}` | `DELETE /v1/tasks/<task>/pushNotificationConfigs/<id>` |
| `GetExtendedAgentCard` | `{}` | `GET /v1/extendedAgentCard` |

## Errors are never streamed

A failed streaming call comes back as a buffered JSON document, not a truncated
event stream:

| Case | Status | Content-Type | Framing |
|---|---|---|---|
| streaming success, both bindings | 200 | `text/event-stream` | chunked, no content-length |
| bad params, JSON-RPC | **200** | `application/json` | content-length |
| bad params, HTTP+JSON | 400 | `application/json` | content-length |
| subscribe to a missing task | 404 | `application/json` | content-length |

This is load-bearing for the gateway, not a detail. Nothing in the policy engine
knows which operations are streaming ones — it decides from what the upstream
framed, recognising chunked or `text/event-stream`. That heuristic is sufficient
only *because* an A2A error is never framed as an event stream, so a chunked JSON
error cannot be mistaken for a stream.

## Files

| File | Contents |
|---|---|
| `main.py` | Route assembly and uvicorn startup |
| `agent.py` | Cards, message parsing, itinerary rendering, the `AgentExecutor` |
| `config.py` | Environment-sourced configuration |
| `requirements.txt` | Pinned exactly — a floating SDK version would let an upstream change silently alter what the gateway is tested against |

## Capability flags

Three flags on the card gate operations in the SDK's request handler. A missing
one does not degrade gracefully; it turns the gated operations into errors. All
three are on, and `push_config_store` must also be supplied or the push
operations fail despite the flag.

| Flag | Gates |
|---|---|
| `streaming` | `SendStreamingMessage`, `SubscribeToTask` |
| `push_notifications` | the four `*TaskPushNotificationConfig` operations |
| `extended_agent_card` | `GetExtendedAgentCard` |
