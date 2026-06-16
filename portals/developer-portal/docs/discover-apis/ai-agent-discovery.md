# AI Agent Discovery

The Developer Portal has built-in support for AI agent discoverability. Every published API is automatically exposed through a set of machine-readable endpoints that AI agents, LLM-powered assistants, and agentic frameworks can use to discover, understand, and invoke APIs — without human assistance.

## `llms.txt`: The Entry Point for Agents

The portal dynamically generates an `llms.txt` file — a plain-text Markdown index designed as the entry point for AI agents. It provides a structured overview of everything the portal exposes for AI consumption.

**Endpoint:**

```
GET /{orgName}/views/{viewName}/llms.txt
```

**What it contains:**

- The portal's name and description
- Any [LLM Instructions](../administer/llm-instructions.md) configured by the portal admin
- A list of all agent-visible APIs, each with its name, description, and a link to its Markdown documentation
- A list of all published, agent-visible API Workflows

An agent that starts at `llms.txt` immediately understands the scope of available APIs and workflows without crawling the entire portal.

> **Tip:** Portal admins can enrich `llms.txt` with high-level guidance for agents — how APIs are organized, common authentication patterns, and recommended workflows — using [LLM Instructions](../administer/llm-instructions.md).

---

## Machine-Readable API Endpoints

Beyond `llms.txt`, the portal exposes all content as clean Markdown and raw specification files. These endpoints require no authentication and no JavaScript rendering — agents can fetch them reliably as plain text.

### API Catalog and Per-API Documentation

| Endpoint | Description |
|---|---|
| `/{orgName}/views/{viewName}/apis.md` | All agent-visible APIs as a single Markdown document |
| `/{orgName}/views/{viewName}/api/{apiHandle}.md` | Full documentation for a specific API in Markdown |

### API Specifications

| Endpoint | Description |
|---|---|
| `/{orgName}/views/{viewName}/api/{apiHandle}/docs/specification.json` | OpenAPI or AsyncAPI specification (JSON) |
| `/{orgName}/views/{viewName}/api/{apiHandle}/docs/specification.graphql` | GraphQL schema SDL |
| `/{orgName}/views/{viewName}/api/{apiHandle}/docs/specification.xml` | SOAP WSDL |

### MCP Server Catalog

| Endpoint | Description |
|---|---|
| `/{orgName}/views/{viewName}/mcps` | MCP server catalog |
| `/{orgName}/views/{viewName}/mcp/{apiHandle}.md` | Full documentation for a specific MCP server |
| `/{orgName}/views/{viewName}/mcp/{apiHandle}/docs/specification.json` | MCP server specification |

### API Workflows

| Endpoint | Description |
|---|---|
| `/{orgName}/views/{viewName}/api-workflows.md` | All published, agent-visible workflows as Markdown |
| `/{orgName}/views/{viewName}/api-workflows/{handle}/arazzo.json` | Arazzo specification for a specific workflow |

---

## How Agents Navigate the Portal

A typical agent discovery flow:

1. **Start at `llms.txt`** — fetch the portal's `llms.txt` to get an overview of available APIs and workflows.
2. **Browse the API catalog** — fetch `apis.md` to read descriptions of all APIs at once.
3. **Retrieve per-API documentation** — once the agent identifies a relevant API, fetch `/{orgName}/views/{viewName}/api/{apiHandle}.md` for full documentation.
4. **Fetch the specification** — retrieve the OpenAPI, GraphQL, or AsyncAPI specification for precise endpoint and parameter details.
5. **Follow a workflow** — if a published workflow matches the task, retrieve the Arazzo specification and agent prompt to follow a vetted, step-by-step call sequence.

---

## Visibility Controls

By default, all published APIs and MCP servers are agent-visible. API publishers can hide specific APIs or workflows from agent-facing surfaces without affecting their visibility to human users.

Portal admins can also configure which content surfaces are exposed via the admin settings.

---

## Related

- [LLM Instructions](../administer/llm-instructions.md) — configure high-level context for agents
- [API Workflows](api-workflows.md) — discover and use multi-step workflows
- [API Documentation](api-documentation.md) — reading API specs and docs as a human user
