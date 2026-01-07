# API Platform AI Gateway

A gateway for managing and securing AI traffic, including Large Language Model (LLM) APIs and Model Context Protocol (MCP) servers.

## Quick Start

- [LLM Quick Start Guide](llm/quick-start-guide.md) - Set up the gateway and route traffic to LLM providers like OpenAI
- [MCP Quick Start Guide](mcp/quick-start-guide.md) - Set up the gateway and route traffic to MCP servers

## Key Concepts

### LLM Provider Template

An LLM Provider Template defines the characteristics and behaviors specific to an AI service provider, such as OpenAI, Azure OpenAI, or other LLM platforms. It describes how the gateway should interpret and extract usage and operational metadata, including prompt, completion, total, and remaining token information, as well as request and response model metadata.

Following templates are shipped out-of-the-box

- OpenAI
- Azure OpenAI
- Anthropic
- AWS Bedrock
- Azure AI Foundry
- Gemini

### LLM Provider

An LLM Provider represents a connection to an AI backend service such as OpenAI, Azure OpenAI, or other LLM APIs. Platform administrators configure LLM Providers to define:

- The LLM Provider Template
- The upstream LLM service URL
- Authentication credentials (API keys, tokens)
- Access control rules for which endpoints are exposed
- Budget control policies, such as token-based rate limiting
- Organization-wide policies such as guardrails

Once configured, the LLM Provider allows traffic to flow through the gateway to the AI backend.

### LLM Proxy

An LLM Proxy allows developers to create custom API endpoints that consume an LLM Provider, while inheriting administrator-enforced access control, budgeting and organization-wide policies defined at the provider level. Each proxy gets its own URL context (e.g., `/assistant`) and can have its own policies applied. This enables:

- Multiple AI applications to share a single LLM Provider
- Per-application policies such as prompt management and guardrails
- Separation between platform administration and application development

### MCP Proxy

An MCP Proxy routes Model Context Protocol traffic to MCP servers. MCP is a protocol that enables AI assistants to interact with external tools and data sources. With MCP Proxies, you can:

- Expose MCP servers through a centralized gateway
- Apply authentication and access control to MCP traffic
- Manage multiple MCP servers from a single control plane

## Default Ports

| Port | Service | Description |
|------|---------|-------------|
| 8080 | Router | HTTP traffic |
| 8443 | Router | HTTPS traffic |
| 9090 | Gateway-Controller | REST API |

## Architecture

```
                           ┌─────────────────┐
                           │ LLM Providers   │
                           │ (OpenAI, etc.)  │
                           └────────▲────────┘
                                    │
┌──────────┐    ┌──────────────┐    │
│ AI Apps  │───▶│  AI Gateway  │────┤
└──────────┘    └──────────────┘    │
                                    │
                           ┌────────▼────────┐
                           │  MCP Servers    │
                           └─────────────────┘
```

**How it works:**

1. Administrators configure LLM Providers and MCP Proxies via the Gateway-Controller API
2. Developers create LLM Proxies to build AI applications on top of available providers
3. The gateway routes traffic, applies policies, and manages authentication

## Documentation

| Section | Description |
|---------|-------------|
| [llm/](llm/) | LLM provider configuration, guardrails, prompt management, and semantic caching |
| [mcp/](mcp/) | MCP proxy setup and policies |
| [observability/](observability/) | Logging and tracing configuration |
| [analytics/](analytics/) | Analytics integrations (Moesif) |
| [ai-gateway-rest-api/](ai-gateway-rest-api/) | REST API authentication and usage |
