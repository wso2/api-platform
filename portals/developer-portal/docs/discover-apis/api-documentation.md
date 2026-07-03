# API Documentation

Each API in the Developer Portal can have documentation sections covering endpoints, schemas, security mechanisms, usage guides, and more.

## View API Documentation

1. Click **APIs** from the sidebar.
2. Select an API from the catalog.
3. Click **Documentation** (in the API's banner, or the **Documentation** item in the API's sidebar submenu).

The Documentation page shows a left-hand navigation grouped by section. The **Specification** group links to the API's definition; any additional documentation sections published for the API (e.g. guides) appear as their own groups. Select an entry to read its content.

## API Specification

Under the **Specification** group, the **API Definition** link (**MCP Playground** for MCP servers) shows the full API definition:

- **REST APIs** — OpenAPI/Swagger rendered interactively (with per-operation try-it support)
- **AsyncAPI (WS / WebSub)** — AsyncAPI specification, with a separate **Tryout** link for an interactive console
- **GraphQL** — GraphQL schema SDL, with a separate **Tryout** link for an interactive GraphiQL console
- **SOAP** — WSDL

## Try It Out

For REST APIs, the specification viewer includes an interactive try-it panel per operation — no separate tab is needed. For WebSocket, WebSub, and GraphQL APIs, click the **Tryout** link under the **Specification** group to open a dedicated interactive console:

1. Open an API's Documentation page and click **Tryout** under **Specification**.
2. Select the environment/endpoint (e.g. Production or Sandbox) and provide required parameters.
3. Enter your access token or API key as required by the console.
4. Send the request and view the response.

## Machine-Readable Endpoints

API documentation and specifications are also available as plain text for AI agents and tooling:

| Resource | Endpoint |
|---|---|
| Full API documentation (Markdown) | `GET /{orgName}/views/{viewName}/api/{apiHandle}.md` |
| OpenAPI / AsyncAPI specification (JSON) | `GET /{orgName}/views/{viewName}/api/{apiHandle}/docs/specification.json` |
| GraphQL schema | `GET /{orgName}/views/{viewName}/api/{apiHandle}/docs/specification.graphql` |
| SOAP WSDL | `GET /{orgName}/views/{viewName}/api/{apiHandle}/docs/specification.xml` |

These endpoints require no authentication and return plain text, making them suitable for AI agents and automated tooling.

## Related

- [Search APIs](search-apis.md) — find APIs in the catalog
- [AI Agent Discovery](ai-agent-discovery.md) — how AI agents navigate the portal
- [Subscribe to an API](../consume-an-api/subscriptions.md) — subscribe your application to an API
