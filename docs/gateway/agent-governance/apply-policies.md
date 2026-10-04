# Apply policies to an agent

An `Agent` carries policies in three scopes. Which scope you write a policy into decides what traffic it sees, and the gateway resolves every request to a canonical Agent2Agent (A2A) operation before choosing the chain to run.

## The three scopes

The following table introduces the scopes in the order a request meets them:

| Scope | Where you write it | What it applies to |
|---|---|---|
| Public Agent Card | `spec.a2a.agentCard.public.policies` | Public Agent Card requests only |
| Common operation | `spec.a2a.operationConfigs.policies` | Every A2A operation |
| Per-operation | `spec.a2a.operationConfigs.operations[].policies` | One named operation |

The card scope and the operation scopes never overlap. Card policies don't run for operations, and operation policies don't run for public card serving. A policy you want on both goes in both places.

## Apply a policy to every operation

Policies under `operationConfigs.policies` run for every A2A operation, on either binding, before anything more specific:

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
        - name: jwt-auth
          version: v1
          params:
            issuers:
              - PrimaryIdp
```

This is where authentication usually belongs, because it's the one thing every operation needs. It also makes the protected Agent Card reachable, which fails closed until some policy in the agent's chain authenticates the request. See [Agent Card](agent-card.md).

## Apply a policy to one operation

Add an entry under `operations`, keyed by canonical operation name, to add policies for that operation alone:

```yaml
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
        - name: SendStreamingMessage
          policies:
            - name: basic-ratelimit
              version: v1
              params:
                limits:
                  - requests: 10
                    duration: "1m"
```

Here every operation needs a valid token, cancelling a task additionally needs the `trip:cancel` scope, and streaming messages are capped at ten requests a minute.

The canonical names are listed in [Expose an agent](expose-an-agent.md). A name outside the set defined by the agent's protocol version is rejected at deploy time, as is the same operation configured twice.

> [!IMPORTANT]
> **`operations` is not an allowlist**
>
> Listing an operation adds policies to it. It doesn't restrict the agent to the operations you listed, and an operation you leave out still runs the common policies and still serves traffic. To deny an operation, attach a policy that denies it.

## Execution order

For an A2A operation, the gateway runs the common policies in the order you wrote them, then the matching per-operation policies in the order you wrote them:

1. `spec.a2a.operationConfigs.policies`
2. `spec.a2a.operationConfigs.operations[].policies` for the resolved operation

Because both bindings resolve to the same canonical operation, a JSON-RPC call and its HTTP+JSON equivalent run the identical chain. You configure an operation once regardless of how clients reach it.

For a public Agent Card request, only `spec.a2a.agentCard.public.policies` runs.

The `GetExtendedAgentCard` chain has one addition you don't write: the gateway's own protected-card handling sits at the tail, after every policy you attached in either scope. It requires that one of your policies authenticated the request, and answers `401` without forwarding anything if none did.

## Apply a policy to card serving

Public Agent Card policies are a separate list on the card itself:

```yaml
    agentCard:
      public:
        mode: managed
        policies:
          - name: cors
            version: v1
        content: { ... }
```

Card serving is open discovery by default, so this scope usually carries cross-origin rules and rate limits. Authentication is an optional restriction here: an authentication policy in this list makes the card answer only callers with a credential, so clients without one can no longer discover the agent. The card's CORS preflight runs only the list's CORS policies, never its authentication policy.

## Which policies you can attach

An agent accepts the same policies as any other gateway artifact. Each policy's parameters are documented in the [Policy Hub](https://wso2.com/api-platform/policy-hub).

| Policy | What it does |
|---|---|
| [JWT Auth](https://wso2.com/api-platform/policy-hub/policies/jwt-auth) | Validates JWT access tokens, optionally requiring scopes |
| [API Key Auth](https://wso2.com/api-platform/policy-hub/policies/api-key-auth) | Validates an API key sent in a header or query parameter |
| [Basic Auth](https://wso2.com/api-platform/policy-hub/policies/basic-auth) | Enforces HTTP Basic Authentication |
| [Opaque Token Auth](https://wso2.com/api-platform/policy-hub/policies/opaque-token-auth) | Validates opaque OAuth 2.0 tokens through token introspection |

## Related topics

- [Authenticate clients](authenticate-clients.md) — a worked example of protecting an agent's operations.
- [Agent Card](agent-card.md) — how the card scopes differ from the operation scopes.
- [Streaming and timeouts](streaming-and-timeouts.md) — per-operation `resilience`, which sits beside `policies` in the same block.
