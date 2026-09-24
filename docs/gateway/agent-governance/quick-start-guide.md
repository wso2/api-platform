# Quick start guide

This guide puts a sample Agent2Agent (A2A) agent behind the AI Gateway, then invokes it through the gateway on both A2A protocol bindings. It takes about ten minutes.

## Prerequisites

- A running AI Gateway, with `ADMIN_USERNAME` and `ADMIN_PASSWORD` exported in the shell you run these commands from. See [Start the gateway](../quick-start-guide.md#start-the-gateway).
- The sample agent below runs on your Docker host, and the gateway reaches it at `host.docker.internal`. Docker Desktop resolves that name from inside a container. On Linux, add `--add-host=host.docker.internal:host-gateway` to the gateway's containers, or use the host's IP address in the agent configuration instead.

The `curl` commands on this page pipe their YAML payload in through a shell heredoc (`--data-binary @- <<'EOF'`), which PowerShell doesn't support. On Windows, either run them from Git Bash or Windows Subsystem for Linux (WSL), or save the YAML between the `EOF` markers to a file and post that file explicitly — note the `.exe`, since `curl` is an alias for `Invoke-WebRequest` in Windows PowerShell:

```powershell
curl.exe -X POST http://localhost:9090/api/management/v1/agents `
  -H "Content-Type: application/yaml" `
  -u "${env:ADMIN_USERNAME}:${env:ADMIN_PASSWORD}" `
  --data-binary "@trip-planner.yaml"
```

## Start the sample agent

The sample is a trip-planning agent built on the A2A SDK. It serves the JSON-RPC binding at `/`, the HTTP+JSON binding under `/v1`, and its own Agent Card at `/.well-known/agent-card.json`:

```bash
docker run -d -p 9099:9099 \
  --name a2a-trip-planner \
  rakhitharr/a2a-trip-planner:v1
```

Check that it's up before you go on:

```bash
curl -s http://localhost:9099/.well-known/agent-card.json
```

## Deploy the agent configuration

Deploy an `Agent` that fronts it. The `pathPrefix` values match the agent's own layout, because a prefix travels upstream with the request and only `spec.context` is stripped:

```bash
curl -X POST http://localhost:9090/api/management/v1/agents \
  -H "Content-Type: application/yaml" \
  -u "$ADMIN_USERNAME:$ADMIN_PASSWORD" \
  --data-binary @- <<'EOF'
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
EOF
```

This agent configures no `agentCard` block, so the gateway proxies the agent's own card and rewrites the addresses in it. To author a card the gateway holds and serves itself, see [Agent Card](agent-card.md).

## Fetch the Agent Card

An A2A client starts by reading the card, which tells it which bindings the agent supports and where to reach them:

```bash
curl -s http://localhost:8080/trip-planner/.well-known/agent-card.json
```

Look at `supportedInterfaces` in the response. The URLs name the gateway, not the agent, because the gateway rewrote them on the way back. A client configured from this card sends its traffic through the gateway and gets whatever policies you attach.

## Send a message over HTTP+JSON

Ask the agent to plan a trip. The `SendMessage` operation is a `POST` to `/message:send` below the HTTP+JSON prefix:

```bash
curl -s -X POST http://localhost:8080/trip-planner/v1/message:send \
  -H 'Content-Type: application/json' \
  -H 'A2A-Version: 1.0' \
  -d '{"message":{"messageId":"m1","role":"ROLE_USER",
       "parts":[{"text":"Plan a 2 day trip to Galle"}]}}'
```

The agent answers with a completed task carrying an itinerary artifact.

> [!IMPORTANT]
> **`A2A-Version` is mandatory**
>
> Every operation request states its protocol version, in an `A2A-Version` header or an `A2A-Version` query parameter. A2A 1.0 reads an absent value as version `0.3`, which this agent doesn't serve, so a request that omits it is rejected. The gateway validates the value and doesn't add it for you.
>
> Agent Card requests are exempt. A card is a discovery document rather than an operation, which is how a client learns the version to state.

## Send the same message over JSON-RPC

The JSON-RPC binding carries every operation on one endpoint, with the operation in the `method` field:

```bash
curl -s -X POST http://localhost:8080/trip-planner/ \
  -H 'Content-Type: application/json' \
  -H 'A2A-Version: 1.0' \
  -d '{"jsonrpc":"2.0","id":"1","method":"SendMessage",
       "params":{"message":{"messageId":"m2","role":"ROLE_USER",
       "parts":[{"text":"Plan a 2 day trip to Galle"}]}}}'
```

Both requests resolve to the same canonical `SendMessage` operation and run the same policy chain. That's what lets you govern an agent once and expose it on either binding.

## Clean up

Stop the sample agent when you're done with it:

```bash
docker stop a2a-trip-planner && docker rm a2a-trip-planner
```

To remove the agent configuration from the gateway:

```bash
curl -X DELETE http://localhost:9090/api/management/v1/agents/trip-planner-v1.0 \
  -u "$ADMIN_USERNAME:$ADMIN_PASSWORD"
```

## Next steps

- Understand the routes this configuration generated: [Expose an agent](expose-an-agent.md)
- Serve an Agent Card the gateway holds, and guard the extended card: [Agent Card](agent-card.md)
- Require a credential on the operations you just called: [Authenticate clients](authenticate-clients.md)
- Rate limit one operation without limiting the rest: [Apply policies](apply-policies.md)
