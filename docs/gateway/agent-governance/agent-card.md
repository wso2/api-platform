# Agent Card

An Agent Card is the document an Agent2Agent (A2A) client reads before it sends anything. The card names the agent's skills, the protocol bindings it supports, and the addresses those bindings are served at. Whatever the card advertises is where clients go.

That last point is what makes card serving a gateway concern. An agent's own card advertises the agent's own addresses, so a client configured from it talks to the agent directly, past the gateway and every policy attached to it.

## How the gateway serves a card

An agent has two cards, and each one is produced in one of two modes.

The **public card** is served at `/.well-known/agent-card.json`, below the agent's context. It's open by default: operation-level authentication policies don't reach it, and it requires a credential only if you add an authentication policy to its own `agentCard.public.policies` list. The **protected card**, also called the extended card, is served through the `GetExtendedAgentCard` operation and carries whatever the agent only shows authenticated callers.

The two modes decide where the document comes from:

| Mode | Where the document comes from | What reaches the agent |
|---|---|---|
| `managed` | A document you write into the configuration. The gateway validates it at deploy time, stores it, and answers from its own copy. | Nothing. The request never reaches the agent. |
| `passthrough` | The agent's own card, proxied through the gateway. | The card request, forwarded upstream. |

Configure both under `spec.a2a.agentCard`:

```yaml
  a2a:
    agentCard:
      public:
        mode: managed
        content: { ... }
      protected:
        mode: passthrough
```

The whole `agentCard` block is optional. An agent that omits it serves its public card in `passthrough` mode at the default path, with interface URL rewriting enabled and no card policies.

## Serve a managed public card

Use `managed` when you want the card to advertise the gateway rather than the agent, and you want to decide exactly what it says. Set `mode: managed` and supply the document under `content`:

```yaml
  a2a:
    protocolVersion: "1.0"
    operationConfigs:
      transports:
        - protocolBinding: JSONRPC
          pathPrefix: /
        - protocolBinding: HTTP+JSON
          pathPrefix: /v1
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
            "streaming": true
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
```

The card is written as embedded JSON because JSON object syntax is valid YAML, and that's the form a card copied out of an A2A agent arrives in. The gateway stores and serves the bytes exactly as supplied, so extension fields survive.

### What the controller checks at deploy time

The gateway validates a managed card against the full A2A Agent Card model for the agent's `protocolVersion`, and rejects the deployment rather than serving a card that would send clients somewhere wrong.

The interface rules are the ones worth knowing before you write a card:

- Every configured transport needs an interface advertising its binding, and no interface may advertise a binding the transports don't expose.
- Each interface's `protocolVersion` must equal the agent's `spec.a2a.protocolVersion`.
- Each `url` must be absolute and use `https`, with no userinfo, query string, or fragment. Because the URL names the router's HTTPS listener, it doesn't match the plaintext port you use for local testing.
- Each `url` path must equal the gateway path for that transport, which is `context` plus the transport's `pathPrefix`.
- The document must not carry a `signatures` block, and must not exceed 1 MiB once encoded.

### How a managed card is served

The gateway answers the card request itself, at the request-header phase, and includes a strong `ETag` derived from the stored bytes. A client that sends `If-None-Match` with a matching tag gets a `304` with no body.

```bash
curl -i http://localhost:8080/trip-planner/.well-known/agent-card.json
```

## Proxy the agent's own public card

Use `passthrough` when the agent already publishes a card you're happy with. The gateway forwards the request and returns the agent's own document:

```yaml
    agentCard:
      public:
        mode: passthrough
```

By default the gateway rewrites the addresses in the response before returning it. Each entry in `supportedInterfaces` gets the gateway endpoint for that entry's protocol binding, built from the scheme and authority the client reached the gateway on. Clients configured from the card then come back through the gateway.

Three consequences follow from rewriting:

- **Only the bindings your transports expose are rewritten.** An interface on a binding this gateway doesn't serve keeps the agent's own URL, so a client selecting that binding reaches the agent directly, outside your policies.
- **The `signatures` block is dropped,** because it no longer covers the bytes being returned. The gateway doesn't sign a card it didn't write, so a rewritten card is unsigned.
- **The response is buffered,** up to 1 MiB.

Turn rewriting off to forward the proxied response byte for byte, signatures included:

```yaml
    agentCard:
      public:
        mode: passthrough
        rewriteUrls: false
```

Do that only when clients are meant to reach the agent directly, because that's what it arranges.

> [!NOTE]
> **When the gateway can't rewrite safely**
>
> A successful response the gateway can't rewrite fails with a gateway error rather than being forwarded or partly rewritten. That covers a body that isn't a card object, an absent or empty `supportedInterfaces`, an interface advertising a protocol version the gateway doesn't serve on that binding, a body over the size ceiling, and a request whose scheme or authority can't be established.

`rewriteUrls` belongs to `passthrough` only. Setting it on a managed card is rejected at deploy time in either polarity, because the gateway already owns that document and validates its interfaces against the configured transports instead.

## Change the card path

The public card is served at `/.well-known/agent-card.json`, relative to the agent's context. Set `path` to serve it somewhere else:

```yaml
    agentCard:
      public:
        mode: managed
        path: /card.json
        content: { ... }
```

A custom path replaces the default route rather than adding an alias. In `passthrough` mode it changes the gateway-facing path only, not the path the gateway fetches upstream.

## Apply policies to card serving

The public card has its own policy list, which runs only for card requests:

```yaml
    agentCard:
      public:
        mode: managed
        policies:
          - name: cors
            version: v1
        content: { ... }
```

Operation policies never run for public card serving, and card policies never run for operations. See [Apply policies](apply-policies.md).

## Serve the protected Agent Card

The protected card is the authenticated extended card, served through the `GetExtendedAgentCard` operation. It isn't a document at a location, so it has no path and no policy list of its own — it runs that operation's chain, which is the common operation policies followed by any matching per-operation entry.

**The gateway requires the request to have been authenticated before it returns or proxies the protected card, and answers `401` otherwise.** This applies in every mode and isn't configurable. An agent that attaches no authentication policy therefore fails closed instead of publishing its extended card.

That's also what an omitted `protected` block means. Writing the block out only chooses how the card is produced:

```yaml
    agentCard:
      protected:
        mode: passthrough
        rewriteUrls: true
```

In `passthrough` mode the gateway forwards the authenticated request and proxies the agent's own extended card, rewriting interface URLs unless you turn that off. In `managed` mode it serves the `content` you supply, correctly for the binding the caller used: the bare card on HTTP+JSON, and a JSON-RPC result envelope echoing the caller's request id on JSON-RPC.

Two differences from the public card are worth noting. A protected card response carries `Cache-Control: no-store` and no `ETag`, because it's authenticated and the JSON-RPC binding is a `POST`, so there's no conditional-`GET` contract to take part in. And when the public card is `managed`, it must declare `capabilities.extendedAgentCard: true`, since that's what tells a client the operation exists at all.

To make the protected card reachable, attach an authentication policy to the agent's operations:

```yaml
    operationConfigs:
      policies:
        - name: jwt-auth
          version: v1
          params:
            issuers:
              - PrimaryIdp
```

Then request it on either binding:

```bash
curl -s -H 'A2A-Version: 1.0' \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/trip-planner/v1/extendedAgentCard
```

> [!NOTE]
> **Agent Card signing**
>
> The gateway rejects `signing.enabled: true` on either card at deploy time, with the message `Agent Card signing is not supported yet`. Omit the `signing` block, or set `enabled: false`.

## Related topics

- [Expose an agent](expose-an-agent.md) — the transports and context that decide what a managed card's interface URLs have to say.
- [Apply policies](apply-policies.md) — the three policy scopes, including the card-only one.
- [Authenticate clients](authenticate-clients.md) — the policies that make the protected card reachable.
- [Agent configuration reference](../reference/agent-configuration.md) — every `agentCard` field and its default.
