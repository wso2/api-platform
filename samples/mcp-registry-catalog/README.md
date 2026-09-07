# MCP Registry Catalog

An internal MCP catalog for the WSO2 API Platform. This sample publishes two MCP proxies: **Weather MCP Server** (`weather-mcp`) and **Inventory MCP Server** (`inventory-mcp`) to the **API Portal** via its **Management API**, and lets consumers discover them through the same portal's **MCP Registry API**.


## What this demonstrates

The API Portal functions as an **MCP Hub**. Every MCP server published to it shows up both as a browsable catalog page for humans, and as structured JSON for tooling, through a Registry API modeled on the [official MCP registry schema](https://modelcontextprotocol.io/). That split is the point of this sample.

- **`publish.sh`** Authenticates against the Platform API's **Management API** and pushes MCP server metadata (a manifest + a tools schema) directly to the portal.
- **`discover.sh`** acts as a consumer or agent. This queries the public, unauthenticated **MCP Registry API** and gets back the same catalog as clean JSON, ready for a script, an agent's discovery loop, or another registry to mirror.
- The **Portal UI** shows the same two servers rendered as an MCP hub a human would browse.

This is the shape of **enterprise MCP governance**. MCP servers get published once, through an API, and are then queryable by both humans and machines from one source of truth.

## Prerequisites

- `docker`, `docker compose` (v2), `bash`, `curl`, `jq`, `openssl`

## Quick Start

Run these in order, from this directory:

### Step 1: Start the stack

```bash
./setup.sh
```

Generates TLS certs/keys and admin credentials, then runs `docker compose up -d --build` to start the following components at once:

- **platform-api** - Management API / control plane
- **api-portal** - the Dev Portal / MCP Hub UI + Registry API
- **gateway-controller** + **gateway-runtime** - the AI Gateway
- **weather-mcp** + **inventory-mcp** - the two sample MCP servers

`setup.sh` then waits until Platform API and API Portal report healthy, and prints the generated admin credentials once. They're also saved to `.env` for the next two scripts to reuse.

Check anytime with `docker compose ps`, or watch a component live with `docker compose logs -f api-portal`.

### Step 2: Publish the MCP servers

```bash
./publish.sh
```

Logs in to the Management API and pushes both `weather-mcp` and `inventory-mcp` to the portal's catalog.

### Step 3: Discover them via the Registry API

```bash
./discover.sh
```

Queries the MCP Registry API and prints the catalog JSON. This is the machine-readable side of the same data you'll see in the UI next.

### Step 4: See them in the portal UI

Open **`https://localhost:9543/api-portal/default/views/default`** in a browser.

You should see two MCP servers, **Weather MCP Server** and **Inventory MCP Server**, listed in the catalog.

### Step 5: Clean up

```bash
./teardown.sh
```

See [Cleanup](#cleanup) below for the full-reset variant.

## What you should see

**`./discover.sh`** prints the full Registry API response, then a summary:

```
✔ 2 MCP server(s) in the catalog:
  - Weather MCP Server v1.0.0 -> http://weather-mcp:8080/mcp
  - Inventory MCP Server v1.0.0 -> http://inventory-mcp:8080/mcp
```

The raw JSON follows the MCP registry's own server-record shape (`$schema`, `name`, `version`, `remotes`, `_meta`) - this is the same format a real MCP registry aggregator would expect.

## How publish.sh works

Each of `mcp-servers/weather-mcp/` and `mcp-servers/inventory-mcp/` carries:

- `mcp.yaml`: a Developer Portal manifest (`kind: MCP`) describing the server: name, version, description, and its endpoint URL.
- `schemaDefinition.yaml`: the tools it exposes, in the platform's canonical flat schema (`type: TOOL`, `name`, `description`, `inputSchema`).
- `server.js`: a minimal, dependency-free MCP server (streamable HTTP transport) implementing those tools, so the endpoint pointed by the catalog entry is live and answers `tools/list` / `tools/call`, not just metadata.

`publish.sh` logs in once against the Platform API (`POST /api/portal/v0.9/auth/login`), then for each server sends a multipart request to the Management API. The whole API Portal app (UI, Management API, Registry API) is mounted under a fixed `/api-portal` prefix. Only `/health` sits outside it:

```bash
curl --cacert resources/certificates/cert.pem -X POST "https://localhost:9543/api-portal/api/v0.9/mcp-servers" \
  -H "Authorization: Bearer $TOKEN" \
  -F "metadata=@mcp.yaml" \
  -F "definition=@schemaDefinition.yaml;type=application/yaml"
```

`resources/certificates/cert.pem` is the self-signed cert `setup.sh` generates for Platform API + API Portal -- `--cacert` verifies against that specific cert instead of skipping TLS verification with `-k`.

Re-running `publish.sh` is safe: a `409 Conflict` (already published) is followed by a `PUT` to the same resource instead, so the script converges rather than failing.

## How discover.sh works

The Registry API needs no authentication. It's meant to be consumed by anyone with network access to the portal, the same way a public package registry's read API works:

```bash
curl --cacert resources/certificates/cert.pem "https://localhost:9543/api-portal/registry/default/v0.1/servers"
```

## Files

| Path | Description |
|---|---|
| `docker-compose.yaml` | Platform API, API Portal, AI Gateway, two MCP servers |
| `setup.sh` | provisions certs/keys, starts the stack |
| `publish.sh` | pushes both MCP proxies via the Management API |
| `discover.sh` | queries the MCP Registry API, prints the catalog |
| `teardown.sh` | stops the stack (`--volumes` for a full reset) |
| `scripts/lib.sh` | shared bash helpers (logging, login, health checks) |
| `configs/config.toml` | Platform API + API Portal configuration |
| `configs/role-to-scope-mapping.yaml` | role -> scope grants (`ap_admin` used here) |
| `configs/gateway-config.toml` | AI Gateway configuration (standalone mode) |
| `resources/` | generated by `setup.sh` (git-ignored): certs, keys |
| `mcp-servers/weather-mcp/` | `Dockerfile`, `server.js`, `mcp.yaml`, `schemaDefinition.yaml` |
| `mcp-servers/inventory-mcp/` | `Dockerfile`, `server.js`, `mcp.yaml`, `schemaDefinition.yaml` |

## Cleanup

```bash
./teardown.sh             # stops containers, keeps data volumes and generated secrets
./teardown.sh --volumes   # full reset: also removes volumes, certs, keys, .env
```
