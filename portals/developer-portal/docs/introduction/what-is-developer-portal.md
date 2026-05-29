# What is the Developer Portal?

The Developer Portal is a self-hosted web application that acts as the front door between your APIs and the developers who consume them. API publishers register their APIs in the portal, and developers discover, subscribe, and generate credentials — all without needing direct access to the underlying infrastructure.

## Key capabilities

| Capability | Description |
|---|---|
| **API catalog** | Browse and search REST, AsyncAPI, GraphQL, and SOAP APIs with full documentation and try-out |
| **Developer applications** | Logical containers for OAuth2 credentials; a developer can have multiple apps (web, mobile, CLI) each with independent consumer key/secret pairs |
| **Subscription management** | Subscribe directly to APIs under named plans (e.g. Gold, Free) that enforce rate limits and quotas |
| **API key generation** | Generate, rotate, and revoke API keys bound to a specific API |
| **OAuth2 credentials** | Generate consumer key/secret pairs and access tokens for OAuth2-secured APIs |
| **Webhook event delivery** | Real-time delivery of API key and subscription lifecycle events to gateways and external systems |
| **MCP server registry** | Discover and integrate Model Context Protocol (MCP) servers alongside REST APIs |
| **API workflows** | Publish and consume multi-step API call sequences (Arazzo format) for human and AI agent use |
| **AI-friendly discovery** | Machine-readable `llms.txt`, per-API Markdown docs, and OpenAPI/AsyncAPI specs for AI agent integration |
| **Theming** | Customize the portal's look and feel per organization and per API |

## How it fits in the platform

```
API Publisher
     │
     │  registers APIs, uploads definitions
     ▼
┌─────────────────────────┐
│     Developer Portal    │  ← this component
│  (Node.js / Express)    │
│  PostgreSQL backend     │
└──────────┬──────────────┘
           │  webhook events (API key lifecycle, subscriptions)
           ▼
    API Gateway ──────────► API Consumer (invokes API)
                                  ▲
                                  │ discovers, subscribes, gets credentials
                           Developer Portal UI
```

## Multi-tenancy

The portal is multi-tenant. Each **organization** gets its own branded space, and within an organization you can have multiple **views** for different audiences (e.g. internal teams vs. external partners). Users are routed to their organization automatically based on a claim in their IdP token.

## Next steps

- [Quick Start](quick-start.md) — get the portal running in minutes with Docker
- [Core Concepts](concepts.md) — understand the key building blocks
- [Administer](../administer/manage-organizations.md) — set up organizations, views, and integrations
