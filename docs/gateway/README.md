# API Platform AI Gateway

A gateway for managing and securing AI traffic, including Large Language Model (LLM) APIs, Model Context Protocol (MCP) servers, and Agent2Agent (A2A) agents.

## Why use the AI Gateway

Run the AI Gateway when AI traffic needs the controls you already apply to your APIs. With it, you can:

- Apply guardrails that validate, filter, or transform content before it reaches a model or a client.
- Serve one OpenAI-compatible endpoint that routes requests to multiple LLM providers.
- Expose MCP servers through a central gateway, and apply authentication and access control to MCP traffic.
- Give an A2A agent one governed address, serve its Agent Card from the gateway, and apply policies to individual A2A operations. See [Agent governance](agent-governance/index.md).
- Collect logs, traces, and analytics for the traffic the gateway handles.
- Run the gateway on its own, or register it with AI Workspace to govern the gateways across your organization.

Two roles share the gateway. A platform administrator configures LLM providers, the credentials they use, and the policies that apply organization-wide. An AI developer creates LLM proxies on top of those providers, and adds the policies a single application needs.

## Quick start

- [LLM quick start guide](quick-start-guide.md) — Start the gateway and route traffic to an LLM provider such as OpenAI
- [A2A agent quick start guide](agent-governance/quick-start-guide.md) — Put an A2A agent behind the gateway and invoke it on both protocol bindings

## Key concepts

### LLM Provider Template

An LLM Provider Template defines the characteristics and behaviors specific to an AI service provider. It describes how the gateway interprets and extracts usage and operational metadata, including prompt, completion, total, and remaining token information, as well as request and response model metadata.

The gateway ships with these templates, loaded at startup:

| Template ID | Provider |
|-------------|----------|
| `openai` | OpenAI |
| `azure-openai` | Azure OpenAI |
| `anthropic` | Anthropic |
| `gemini` | Gemini |
| `mistralai` | MistralAI |
| `awsbedrock` | AWS Bedrock |
| `azureai-foundry` | Azure AI Foundry |

### LLM Provider

An LLM Provider represents a connection to an AI backend service such as OpenAI, Azure OpenAI, or another LLM API. Platform administrators configure LLM Providers to define:

- The LLM Provider Template
- The upstream LLM service URL
- Authentication credentials (API keys, tokens)
- Access control rules for which endpoints are exposed
- Budget control policies, such as token-based rate limiting
- Organization-wide policies such as guardrails

### LLM Proxy

An LLM Proxy lets developers create their own endpoint that consumes an LLM Provider, while inheriting the access control, budgeting, and organization-wide policies defined on the provider. Each proxy gets its own URL context (for example, `/assistant`) and can carry its own policies. This enables:

- Multiple AI applications to share a single LLM Provider
- Per-application policies such as prompt management and guardrails
- Separation between platform administration and application development

### MCP Proxy

An MCP Proxy routes Model Context Protocol traffic to MCP servers. MCP lets AI assistants interact with external tools and data sources. With MCP Proxies, you can:

- Expose MCP servers through a centralized gateway
- Apply authentication and access control to MCP traffic
- Manage multiple MCP servers from a single control plane

### Agent

An Agent fronts an Agent2Agent (A2A) agent. A2A is a protocol that lets one AI agent discover another and delegate work to it. With Agents, you can:

- Expose an A2A agent on the JSON-RPC binding, the HTTP+JSON binding, or both
- Serve an Agent Card that advertises the gateway rather than the agent
- Apply authentication and rate limits to individual A2A operations

See [Agent governance](agent-governance/index.md).

## How it works

The following diagram shows the artifacts a request passes through:

```
     AI apps              MCP clients            A2A clients
        │                      │                      │
        ▼                      ▼                      ▼
┌───────────────────────────────────────────────────────────┐
│ AI Gateway                                                │
│                                                           │
│  ┌───────────────┐    ┌───────────────┐   ┌────────────┐  │
│  │   LLM Proxy   │    │   MCP Proxy   │   │   Agent    │  │
│  └───────┬───────┘    └───────┬───────┘   └─────┬──────┘  │
│          │                    │                 │         │
│          ▼                    │                 │         │
│  ┌───────────────┐            │                 │         │
│  │  LLM Provider │            │                 │         │
│  └───────┬───────┘            │                 │         │
└──────────┼────────────────────┼─────────────────┼─────────┘
           ▼                    ▼                 ▼
     LLM services          MCP servers        A2A agents
```

Client traffic reaches the gateway on the router ports: 8443 for HTTPS and 8080 for HTTP. An AI application calls an LLM Proxy at its own URL context, such as `/assistant`. An MCP client calls an MCP Proxy at its context, and an A2A client calls an Agent at its context.

An LLM Proxy names the provider it consumes in its `provider.id` field, so a request that arrives at the proxy leaves through that LLM Provider. The provider holds the template, the upstream service URL, the credentials for that service, and the `accessControl` rules that decide which upstream endpoints it exposes. An MCP Proxy and an Agent route to their upstream directly, without a provider.

You attach policies on an LLM Proxy, an LLM Provider, an MCP Proxy, or an Agent. A request through an LLM Proxy runs the proxy's policy chain first, then the provider's chain. An Agent resolves each request to an A2A operation and can run a different chain per operation. See [Apply policies to an agent](agent-governance/apply-policies.md).

When an upstream service streams its response, the gateway relays it to the client chunk by chunk instead of buffering the whole response. This holds for LLM providers, LLM proxies, and A2A streaming operations. See [Streaming and timeouts](agent-governance/streaming-and-timeouts.md) for how agent streams are timed out.

## Default ports

| Port | Service | Description |
|------|---------|-------------|
| 8080 | Router | HTTP traffic |
| 8443 | Router | HTTPS traffic |
| 9090 | Gateway-Controller | Management REST API |
| 9094 | Gateway-Controller | Admin API (health) |

## Documentation

| Section | Description |
|---------|-------------|
| [Quick start guide](quick-start-guide.md) | Start the gateway and route a first LLM request |
| [Agent governance](agent-governance/index.md) | Expose, secure, and govern A2A agents |
| [Agent configuration reference](reference/agent-configuration.md) | Every field of the `Agent` artifact and its deploy-time rules |
| [Gateway REST APIs](../rest-apis/gateway/README.md) | Gateway-Controller management API reference |
