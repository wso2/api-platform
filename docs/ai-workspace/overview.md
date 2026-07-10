# AI Workspace Overview

AI Workspace is a self-hosted control plane for managing AI gateway runtimes. It provides a single interface for configuring LLM providers, creating application-facing proxy endpoints, applying guardrails and rate limits, connecting MCP servers, and observing traffic across your AI infrastructure.

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  AI Workspace                   │
│           (React SPA + nginx container)         │
└───────────────────┬─────────────────────────────┘
                    │ REST API calls
┌───────────────────▼─────────────────────────────┐
│                 Platform API                    │
│        (Go server + PostgreSQL/SQLite)          │
└───────────────────┬─────────────────────────────┘
                    │ control-plane gRPC
        ┌───────────▼──────────┐
        │    AI Gateway        │
        │  (Envoy + policy     │
        │   engine runtime)    │
        └──────────────────────┘
```

The AI Workspace UI talks exclusively to the Platform API. The Platform API provisions configuration into one or more deployed AI Gateway runtimes. Gateways call the Platform API's control-plane endpoint to receive their configuration and then proxy LLM traffic.

## Key Features

### AI Gateways

Register and monitor gateway runtimes. Each gateway is identified by a registration token that the runtime uses to connect to the Platform API. The UI shows connectivity status (active/inactive) and provides the setup instructions (environment files, Helm values) needed to deploy a gateway.

See [AI Gateways](features/ai-gateways.md).

### LLM Provider Templates

Reusable blueprints for connecting to an upstream LLM service — endpoint, authentication, OpenAPI specification, and token/model mappings. Ships with built-in templates (OpenAI, Anthropic, Azure OpenAI, Azure AI Foundry, Gemini, Mistral, AWS Bedrock) and supports custom, versioned templates that providers can be created from.

See [LLM Provider Templates](features/llm-provider-templates.md).

### LLM Providers

Configure connections to upstream LLM services. Supported providers: OpenAI, Anthropic, Azure OpenAI, Azure AI Foundry, Google Gemini, and Mistral. Provider credentials and API keys are stored securely and deployed to gateways. You control which models from each provider are exposed.

See [LLM Providers](features/llm-providers.md).

### LLM Proxies

Create optional application-facing proxy endpoints on top of providers. Each proxy has its own API key, allowing you to isolate consumers and apply per-proxy controls — authentication, guardrails, traffic limits — without affecting other consumers of the same provider.

See [LLM Proxies](features/llm-proxies.md).

### MCP Proxies

Connect remote Model Context Protocol (MCP) servers through managed proxy endpoints. Policies can be applied to MCP traffic for authentication, authorization, request rewriting, and access control.

See [MCP Proxies](features/mcp-proxies.md).

### GenAI Applications

Group multiple API keys under a named application. Tracks token usage, request counts, and cost per application, giving you application-level visibility into your AI spend.

See [GenAI Applications](features/genai-applications.md).

### Policies & Guardrails

Apply policies at the provider or proxy level:

- **Guardrails**: Semantic Prompt Guard, PII Masking (regex), Azure Content Safety, Word Count, Sentence Count
- **Rate Limiting**: Token-based, cost-based, and basic request-count limits
- **Traffic Controls**: Model Round Robin, Semantic Cache, Prompt Template, Prompt Decorator

See [Policies](features/policies.md).

### Insights

Embedded Moesif-powered analytics dashboard showing real-time request traffic, token usage trends, cost analysis, error rates, and guardrail trigger metrics.

See [Insights](features/insights.md).

## Multi-Tenancy

AI Workspace supports organizations and sub-organizations. Each organization has its own scoped set of gateways, providers, proxies, and applications. In OIDC mode, organization identity is derived from JWT claims emitted by the IDP.

## Authentication Modes

| Mode | Use case |
|------|----------|
| `basic` | Local development, demos — no IDP required |
| `oidc` | Production — delegates to an external OIDC-compliant IDP |

See [Authentication Overview](authentication/README.md) to choose and configure an auth mode.
