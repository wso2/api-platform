# Connect Claude Desktop to an MCP Server via WSO2 AI Gateway

This sample demonstrates how to connect Claude Desktop to an MCP server through the WSO2 AI Gateway with OAuth2 authentication. A mock Reading List MCP backend is wired up locally so no cloud account is needed — Claude Desktop can discover and invoke the Reading List tools (`listBooks`, `addBook`, `deleteBook`) directly from a conversation. The gateway enforces JWT authentication on every MCP request, rejecting unauthenticated calls with `401 Unauthorized`.

## Prerequisites

- Docker and Docker Compose
- `curl`, `wget`, or `unzip` on your host
- Node.js 18+ (for Claude Desktop integration via `mcp-remote`)
- [Claude Desktop](https://claude.ai/download) installed

## Getting Started

```bash
sh setup.sh
```

`setup.sh` does the following in order:

1. Creates `.env` from `.env.example` automatically
2. Downloads and unzips the official WSO2 AI Gateway distribution
3. Injects a JWT keymanager entry into the gateway config, pointing to the mock JWKS endpoint
4. Starts a WireMock container that acts as both the MCP backend and the JWKS server
5. Starts the WSO2 AI Gateway stack (controller + runtime) via Docker Compose
6. Waits for the gateway controller to become healthy
7. Connects the WireMock container to the gateway's internal Docker network
8. Registers the MCP proxy with `mcp-auth` policy via the management API
9. Polls the traffic endpoint until routes are live before exiting

Once `setup.sh` completes, the gateway is fully ready to accept MCP requests.

## Try It Out

**Unauthenticated request — expect `401`:**

```bash
curl -sk -X POST https://localhost:8443/reading-list/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
```

**Authenticated request — expect `200`:**

```bash
source .env
curl -sk -X POST https://localhost:8443/reading-list/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${BEARER_TOKEN}" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
```

**List tools:**

```bash
curl -sk -X POST https://localhost:8443/reading-list/mcp \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${BEARER_TOKEN}" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

Expected response:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {"name": "listBooks", "description": "Get all books currently on the reading list"},
      {"name": "addBook",   "description": "Add a new book to the reading list by providing the title, author, and status"},
      {"name": "deleteBook","description": "Remove a book from the reading list by its ID"}
    ]
  }
}
```

## Verify with test.sh

`test.sh` runs 6 assertions: an unauthenticated `initialize` (expect `401`), then authenticated `initialize`, `tools/list`, `tools/call listBooks`, `tools/call addBook`, and `tools/call deleteBook` (all expect `200`).

```bash
sh test.sh
```

Expected output:

```
=== Test 1: No token — expect HTTP 401 ===
✔  initialize (no token) (HTTP 401)

=== Test 2: initialize with valid token — expect HTTP 200 ===
✔  initialize (valid token) (HTTP 200)

=== Test 3: tools/list — expect HTTP 200 with 3 tools ===
✔  tools/list (HTTP 200)

=== Test 4: tools/call listBooks — expect HTTP 200 ===
✔  tools/call listBooks (HTTP 200)

=== Test 5: tools/call addBook — expect HTTP 200 ===
✔  tools/call addBook (HTTP 200)

=== Test 6: tools/call deleteBook — expect HTTP 200 ===
✔  tools/call deleteBook (HTTP 200)

✔  PASSED — All 6 tests passed.
```

## Use with Claude Desktop

`configure-claude.sh` patches your Claude Desktop config and restarts the app:

```bash
sh configure-claude.sh
```

Then try these prompts in Claude Desktop:

- *"What books are on my reading list?"*
- *"Add 'The Pragmatic Programmer' by Andy Hunt with status to_read"*
- *"Delete book with ID 3 from my reading list"*

> **Note:** Responses are static mock data — changes are not persisted. This sample demonstrates MCP tool invocation through the gateway, not a real database backend.

## How It Works

```
Claude Desktop
     │  HTTPS  Authorization: Bearer <token>
     ▼
WSO2 AI Gateway  ──► fetches public key from WireMock /jwks
     │               validates JWT signature, issuer, expiry
     │  (proxies MCP JSON-RPC on auth success)
     ▼
WireMock
     ├── POST /mcp  →  MCP tool responses (initialize, tools/list, tools/call)
     └── GET  /jwks →  RSA public key for JWT validation
```

The Bearer token in `.env` is a pre-signed RS256 JWT (expires 2099) with an `org: sampleorganization` claim. It was signed with an RSA private key at sample-build time — the private key was discarded and only the public key is kept in `wiremock/mappings/jwks.json`. The gateway validates the token's signature against this public key on every request.

## Configuration

| Variable       | Default        | Description                                      |
|----------------|----------------|--------------------------------------------------|
| `BEARER_TOKEN` | *(pre-signed)* | RS256 JWT used in the `Authorization` header     |
| `MGMT_PORT`    | `9090`         | Gateway management API port                      |
| `HEALTH_PORT`  | `9094`         | Gateway health check port                        |
| `TRAFFIC_PORT` | `8443`         | Gateway HTTPS traffic port                       |
| `MAX_RETRIES`  | `30`           | Max readiness poll attempts before giving up (2s interval) |

## What's Running

| Container | Role | Host Port |
|---|---|---|
| `wso2apip-ai-gateway-*-gateway-runtime-1` | WSO2 AI Gateway (traffic + policy engine) | 8080, 8443 |
| `wso2apip-ai-gateway-*-gateway-controller-1` | Gateway control plane + management API | 9090 |
| `mock-mcp-reading-list` | WireMock — mock MCP backend + JWKS server | 8082 |

## Teardown

Stops and removes the WireMock container and the full gateway stack (including volumes).

```bash
sh teardown.sh
```
