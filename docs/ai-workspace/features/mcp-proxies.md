# MCP Proxies

An MCP Proxy connects a remote Model Context Protocol (MCP) server through a managed gateway endpoint. This lets you apply enterprise controls — authentication, authorization, request rewriting, and access control — to MCP traffic that would otherwise go directly from AI agents to MCP servers.

## What is MCP?

The Model Context Protocol is an open standard that allows AI agents and LLMs to call external tools and data sources through a standardized interface. An MCP server exposes a set of "tools" (functions the agent can invoke) and "resources" (data the agent can read).

## Why Proxy MCP Traffic?

MCP servers are typically unprotected endpoints designed for direct agent access. Running them through an AI Gateway lets you:

- Require authentication before an agent can call an MCP server.
- Restrict which tools or resources an agent is allowed to access.
- Rewrite requests before they reach the MCP server (e.g. inject credentials).
- Log all MCP calls for audit and observability.

## Creating an MCP Proxy

1. Navigate to **MCP Proxies** in the left sidebar (listed under **External Servers**).
2. Click **New MCP Proxy**.
3. Enter:
   - **Name** — a human-readable identifier.
   - **Upstream URL** — the URL of the remote MCP server.
4. Click **Create**.

## Configuring Policies

After creation, attach policies to the proxy:

### Authentication

Require an API key or bearer token from agents calling the proxy. The gateway validates the credential before forwarding the request to the MCP server.

### Authorization

Restrict access to specific MCP tools or resources. Define an allowlist of tool names the agent is permitted to call through this proxy.

### Request Rewrite

Transform requests before they reach the upstream MCP server — add headers, inject credentials, or modify the request body.

### Access Control

IP-based or scope-based allow/deny rules applied before any policy chain.

## Deploying an MCP Proxy

1. Open the MCP proxy and click **Deploy**.
2. Select one or more target gateways.
3. Click **Deploy**.

Once deployed, agents connect to the gateway's MCP endpoint rather than the upstream MCP server directly. The gateway URL and any required credentials are shown on the proxy overview page.

## Managing an MCP Proxy

- **Edit** — update the name or upstream URL.
- **Policies** — add, update, or remove attached policies.
- **Undeploy** — remove from a gateway without deleting the proxy record.
- **Delete** — permanently removes the proxy from all gateways.
