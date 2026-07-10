# Build an AI App with Claude Code that Calls Governed Backend APIs


## Overview

This sample demonstrates how to build an AI app with Claude Code that calls a Reading List API governed by a self-hosted WSO2 API Gateway. The gateway enforces API key authentication on every request the agent makes — no cloud account required.

```
┌─────────────────┐   X-API-Key    ┌────────────────────────┐   HTTPS   ┌─────────────────────────────────────────┐
│   Claude Code   │ ─────────────► │  WSO2 API Gateway      │ ────────► │  Reading List API (live backend)        │
│   (AI agent)    │                │  (Docker, port 8080)   │           │  apis.bijira.dev/.../reading-list-api   │
│                 │                │                        │           │                                         │
│ uses            │                │  • API key auth        │           │  OpenAPI spec:                          │
│ api_client.py   │                │  • access logs         │           │  github.com/wso2/bijira-samples         │
└─────────────────┘                └────────────────────────┘           └─────────────────────────────────────────┘
```

## What You Will Learn

- How to deploy a REST API proxy on a self-hosted WSO2 gateway
- How to enforce API key authentication at the gateway layer (zero backend changes)
- How to configure Claude Code to call APIs through a governed gateway using `CLAUDE.md` and `api_client.py`
- How every AI-generated API call is authenticated and logged under a named identity

## Prerequisites

- Docker and Docker Compose
- `curl` and `unzip` on your host
- Python 3.8+ (stdlib only — no extra packages needed)
- Claude Code CLI — [install guide](https://docs.anthropic.com/en/docs/claude-code)

## Files

```
reading-list-api.yaml   REST API proxy definition (deployed to the gateway)
api_client.py           Helper: attaches X-API-Key to all requests
CLAUDE.md               Claude Code briefing: available operations and rules
setup.sh                Automated setup (download → start → deploy API → generate key → write settings)
test.sh                 End-to-end test suite (run after setup, before Claude Code)
teardown.sh             Automated teardown
```

## Setup

```bash
./setup.sh
```

The script performs these steps in order:

1. Downloads `wso2apip-api-gateway-1.1.0.zip`
2. Starts the Docker Compose gateway stack
3. Waits for the gateway to become healthy
4. Deploys the Reading List API proxy via the management API
5. Writes `.claude/settings.json` with `API_BASE_URL` (API key is generated on first use)

### Endpoints after setup

| | URL |
|---|---|
| Reading List API (via gateway) | `http://localhost:8080/reading-list/v1/books` |
| Gateway health | `http://localhost:9094/health` |
| Management API | `http://localhost:9090/api/management/v0.9` |

## Verify the Setup

Before starting Claude Code, run the test suite to confirm the gateway, API deployment, and API key are all working correctly:

```bash
./test.sh
```

### Expected Results

```
--- Pre-flight checks ---
[INFO]  API key loaded from settings.json.
[INFO]  Gateway is healthy.

--- Authentication enforcement ---
[PASS]  GET /books (no API key) → HTTP 401
[PASS]  GET /books (wrong API key) → HTTP 401

--- GET /books ---
[PASS]  GET /books (valid key) → HTTP 200

--- POST /books ---
[PASS]  POST /books → HTTP 201
[PASS]  POST /books returned id: <uuid>

--- GET /books/{id} ---
[PASS]  GET /books/<uuid> → HTTP 200

--- PUT /books/{id} ---
[PASS]  PUT /books/<uuid> (status=reading) → HTTP 200

--- DELETE /books/{id} ---
[PASS]  DELETE /books/<uuid> → HTTP 200

--- api_client.py smoke test ---
[PASS]  api_client.py executed successfully

============================================================
 Results: 9 passed, 0 failed
============================================================
```

> **Note:** The live backend at `apis.bijira.dev` does not persist changes between sessions. Added/updated/deleted books will not be visible on the next run — this is expected.

## Using Claude Code

Start Claude Code from this directory:

```bash
claude
```

Because `CLAUDE.md` is present, Claude Code is automatically briefed on the API. Try these prompts:

```
Add "The Lord of the Rings" by J. R. R. Tolkien to my reading list.
```

```
List all my books.
```

```
I just started reading The Lord of the Rings — update its status.
```

```
Show me everything I haven't started yet.
```

Claude Code will generate and execute Python code using `api_client.py`. On first use, `api_client.py` automatically calls the gateway management API to generate an API key and saves it to `.claude/settings.json`. From that point on, the saved key is reused.

## How It Works

### API key bootstrap

`api_client.py` self-bootstraps on first import:

1. Reads `.claude/settings.json` for `API_BASE_URL` and `API_KEY`
2. If `API_KEY` is empty, calls `POST /api/management/v0.9/rest-apis/reading-list-api/api-keys` to generate a key
3. Writes the new key back to `.claude/settings.json` so it persists across sessions
4. Attaches `X-API-Key: <key>` to every subsequent request

### reading-list-api.yaml

```yaml
spec:
  context: /reading-list/v1
  upstream:
    main:
      url: https://apis.bijira.dev/samples/reading-list-api-service/v1.0
  policies:
    - name: api-key-auth       # Rejects requests without a valid key
      version: v1
      params:
        key: X-API-Key
        in: header
```

The gateway validates `X-API-Key` and forwards authenticated requests to the live upstream. Invalid or missing keys receive HTTP 401 before reaching the backend.

### API contract

The Reading List API follows the public OpenAPI spec at
`github.com/wso2/bijira-samples/blob/main/reading-list-api/openapi.yaml`.

| Operation | Function | Notes |
|---|---|---|
| `GET /books` | `books_list()` | Returns `{"books": [...]}` |
| `POST /books` | `books_add(title, author, status)` | `status`: `to_read` · `reading` · `read` |
| `GET /books/{id}` | `books_get(id)` | |
| `PUT /books/{id}` | `books_update(id, status)` | Only `status` can be changed |
| `DELETE /books/{id}` | `books_delete(id)` | |

## Teardown

```bash
# Stop the stack
./teardown.sh

# Also remove the extracted directory, downloaded zip, and settings
./teardown.sh --clean
```

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `setup.sh` fails at health check | Docker images still pulling — wait and retry |
| `test.sh` reports HTTP 401 for valid key | API key mismatch — delete `.claude/settings.json` and re-run `setup.sh` |
| `test.sh` reports HTTP 404 on `/books` | API not yet deployed — re-run `setup.sh` |
| `test.sh` exits with "Gateway is not healthy" | Gateway not running — run `setup.sh` first |
| `api_client.py` exits with "settings not found" | Run `setup.sh` first |
| API key generation fails | Gateway may not be healthy — check `http://localhost:9094/health` |
