# Authenticate agent clients

An agent deployed without an authentication policy answers every caller that can reach it. This page shows you how to require a credential on an Agent2Agent (A2A) agent's operations, issue an API key for it, and call the agent with that key.

This page is for the **platform administrator** who protects the agent and issues keys, and the **AI developer** whose client sends them.

## Where authentication applies

Attaching an authentication policy does two things at once:

- **It protects the agent's operations.** Requests without a valid credential are rejected before they reach the agent.
- **It makes the protected Agent Card reachable.** The gateway requires an authenticated request before it returns the extended card, and answers `401` otherwise. An agent with no authentication policy therefore never publishes its extended card. See [Agent Card](agent-card.md).

Public Agent Card serving is separate. The authentication policies on operations don't apply to it, so the card stays open by default. A client reads the card to discover the agent before it holds any credential, so leave it open unless you want to restrict discovery. To do that, add an authentication policy to the card's own `agentCard.public.policies` list, and only clients that already hold a credential can discover the agent.

## Require an API key

Attach the `api-key-auth` policy to the agent's common operation policies:

```yaml
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /
        - protocolBinding: HTTP+JSON
          pathPrefix: /v1
      policies:
        - name: api-key-auth
          version: v1
          params:
            key: X-API-Key
            in: header
```

`params.key` names the header the gateway reads the key from, and `params.in` sets where to look. Header matching is case-insensitive, and the policy's default header name is `API-Key`.

Because this policy sits in the common scope, it covers every operation on both bindings. To require more of one operation — a scope on `CancelTask`, for instance — add a per-operation entry as well. See [Apply policies](apply-policies.md).

## Create a key for the agent

Create a key for the client that calls the agent. This command keeps the value in a shell variable rather than printing it to your terminal or shell history:

```bash
AGENT_CONSUMER_KEY=$(curl -s -X POST \
  http://localhost:9090/api/management/v1/agents/trip-planner-v1.0/api-keys \
  -u "$ADMIN_USERNAME:$ADMIN_PASSWORD" \
  -H "Content-Type: application/json" \
  -d '{"name":"trip-planner-client"}' \
  | jq -r '.apiKey.apiKey')
```

Replace *`trip-planner-v1.0`* with your agent's name and *`trip-planner-client`* with a name for the key. The `name` field is optional; without it the gateway generates an identifier.

Check that a key came back:

```bash
test -n "$AGENT_CONSUMER_KEY" && test "$AGENT_CONSUMER_KEY" != "null"
```

Keys are a 32-byte random value in hexadecimal, prefixed with `apip_`. Two things about the response matter:

- **The value is returned only when the key is created or regenerated.** Store it securely. If you lose it, regenerate the key rather than trying to read it back.
- **The response reports `remainingApiKeyQuota`,** so you can see how many more keys you can create.

## Call the agent with the key

Send the key in the header the policy names, alongside the mandatory protocol version:

```bash
curl -s -X POST http://localhost:8080/trip-planner/v1/message:send \
  -H 'Content-Type: application/json' \
  -H 'A2A-Version: 1.0' \
  -H "X-API-Key: ${AGENT_CONSUMER_KEY}" \
  -d '{"message":{"messageId":"m1","role":"ROLE_USER",
       "parts":[{"text":"Plan a 2 day trip to Galle"}]}}'
```

A request to a protected operation without a valid key returns `401`.

The public Agent Card needs no key, which is how a client finds the agent in the first place:

```bash
curl -s http://localhost:8080/trip-planner/.well-known/agent-card.json
```

## Require a JWT instead

To validate JSON Web Tokens (JWTs) against an identity provider, attach `jwt-auth` in the same place:

```yaml
      policies:
        - name: jwt-auth
          version: v1
          params:
            issuers:
              - PrimaryIdp
```

Add `scopes` to a per-operation entry to require more of a particular operation:

```yaml
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
```

Where authentication sits among an agent's policies is your decision. The gateway's protected-card check only asks whether some policy in the chain authenticated the request.

## Manage keys

The management API exposes five operations for the key lifecycle, all requiring Basic Auth and the `admin` or `consumer` role. Replace *`{id}`* with the agent's name and *`{apiKeyName}`* with the key's name.

| Operation | Request | What it does |
|---|---|---|
| Create | `POST /agents/{id}/api-keys` | Generates a key for the agent and returns its value. |
| List | `GET /agents/{id}/api-keys` | Lists the agent's keys. Values aren't included. |
| Regenerate | `POST /agents/{id}/api-keys/{apiKeyName}/regenerate` | Issues a new value and returns it. The previous value stops working. |
| Update | `PUT /agents/{id}/api-keys/{apiKeyName}` | Sets a custom value on a key, for injecting an externally issued one. |
| Revoke | `DELETE /agents/{id}/api-keys/{apiKeyName}` | Revokes a key, so it can no longer authenticate requests. |

Regenerate a key when you rotate credentials, and revoke one as soon as you believe it's exposed. Use a separate key for each client and each environment, so revoking one doesn't interrupt the others.

For the full request and response reference, see [Agent management](../../rest-apis/gateway/agent-management.md).

## Related topics

- [Apply policies](apply-policies.md) — the scopes an authentication policy can sit in, and the order policies run in.
- [Agent Card](agent-card.md) — why the protected card depends on the policies described here.
- [Authenticate backends](https://github.com/wso2/docs-api-platform/blob/main/en/docs/ai-gateway/next/authenticate-backends.md) — the credential the gateway presents to the agent upstream, which is a separate concern.
- [Secure the management API](https://github.com/wso2/docs-api-platform/blob/main/en/docs/ai-gateway/next/setup-and-deployment/secure-the-management-api.md) — authentication on the control plane you issue these keys through.
