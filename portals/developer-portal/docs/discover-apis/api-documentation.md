# API Documentation

Each API in the Developer Portal can have documentation sections covering endpoints, schemas, security mechanisms, usage guides, and more.

## View API Documentation

1. Click **APIs** from the sidebar.
2. Select an API from the catalog.
3. Click the **Documentation** tab (or click **Documentation** in the API header section).

The Documentation tab lists all documentation sections published for the API. Select a section to read its content.

## API Specification

The **API Specification** (or **Definition**) tab shows the full API definition:

- **REST APIs** — OpenAPI/Swagger rendered interactively
- **AsyncAPI** — AsyncAPI specification
- **GraphQL** — GraphQL schema SDL
- **SOAP** — WSDL

## Try-Out Console

The **Try-Out** tab provides an interactive console where you can make real API calls directly from the portal. To use the try-out console:

1. Open an API and go to the **Try-Out** tab.
2. Select the operation you want to test.
3. Provide required parameters and a request body if needed.
4. Enter your access token or API key.
5. Click **Execute** to send the request and see the response.

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
