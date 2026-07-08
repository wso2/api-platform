# What is the Developer Portal?

The Developer Portal is a self-hosted web application that acts as the front door between your APIs and the developers who consume them. API publishers register their APIs in the portal, and developers discover, subscribe, and generate credentials — all without needing direct access to the underlying infrastructure.

## Key capabilities

| Capability | Description |
|---|---|
| **API catalog** | Browse and search REST, AsyncAPI, GraphQL, and SOAP APIs with full documentation and try-out |
| **Developer applications** | Logical containers for OAuth2 credentials; a developer can have multiple apps (web, mobile, CLI) each linked to independent OAuth client IDs |
| **Subscription management** | Subscribe directly to APIs under named plans (e.g. Gold, Free) that enforce rate limits and quotas |
| **API key generation** | Generate, rotate, and revoke API keys bound to a specific API |
| **OAuth2 credentials** | Link an OAuth client ID (created directly in a key manager) to an application and generate access tokens for OAuth2-secured APIs |
| **Webhook event delivery** | Real-time delivery of API key and subscription lifecycle events to gateways and external systems |
| **MCP server registry** | Discover and integrate Model Context Protocol (MCP) servers alongside REST APIs |
| **API workflows** | Publish and consume multi-step API call sequences (Arazzo format) for human and AI agent use |
| **AI-friendly discovery** | Machine-readable `llms.txt`, per-API Markdown docs, and OpenAPI/AsyncAPI specs for AI agent integration |
| **Theming** | Customize the portal's look and feel per organization and per API |

## Gateway-agnostic, unified developer experience

The portal is designed to work with any API gateway. It does not embed gateway-specific logic — instead it communicates with gateways through a generic webhook event outbox. When a developer generates an API key, subscribes, or revokes a key, the portal fires a signed event to every registered gateway subscriber. Each gateway adapter listens for these events and enforces access in its own way.

This means you can:

- Connect multiple gateways of different types to the same portal simultaneously
- Replace or swap out a gateway without changing how developers interact with the portal
- Run the portal in a fully standalone mode (no live gateway required) and replay events later


## Multi-tenancy

The portal is multi-tenant. Each **organization** gets its own branded space, and within an organization you can have multiple **views** for different audiences (e.g. internal teams vs. external partners). Users are routed to their organization automatically based on a claim in their IdP token.

## Next steps

- [Quick Start](quick-start.md) — get the portal running in minutes with Docker
- [Core Concepts](concepts.md) — understand the key building blocks
- [Administer](../administer/manage-organizations.md) — set up organizations, views, and integrations
