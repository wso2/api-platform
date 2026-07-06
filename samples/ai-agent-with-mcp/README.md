# Build an AI Agent That Uses Aggregated MCP Tools from Multiple APIs

The agent connects to three MCP servers (CRM, Orders, Knowledge Base) through the WSO2 AI Gateway, and calls Anthropic Claude through the gateway's LLM proxy. No Bijira cloud account is needed — everything runs locally via Docker.

```
agent.py
  │
  ├─► WSO2 MCP Gateway  :8080  ─► crm-mcp    (WireMock :8001)
  │      /crm /orders /kb      ─► orders-mcp (WireMock :8002)
  │                            ─► kb-mcp     (WireMock :8003)
  │
  └─► WSO2 LLM Gateway  :8443  ─► api.anthropic.com
           /claude-agent
```

---

## Prerequisites

Make sure the following are installed before running the sample:

| Tool | Notes |
|------|-------|
| Docker with Compose plugin | Version 24 or later |
| Python 3.10 or later | For running the agent |
| `curl` and `unzip` | Used by `setup.sh` to download the gateway |
| An Anthropic API key | Get one at [console.anthropic.com](https://console.anthropic.com) |

---

## File structure

| File / Folder | Purpose |
|---------------|---------|
| `setup.sh` | One-command setup and run — downloads the gateway, starts all containers, registers proxies, then runs the agent |
| `teardown.sh` | Stops all containers and clears gateway state |
| `test.sh` | Verifies the setup is working and runs the agent |
| `configure-gateway.sh` | Registers the Anthropic LLM provider, LLM proxy, and three MCP proxies with the WSO2 AI Gateway Management API. Called automatically by `setup.sh` |
| `docker-compose.yml` | Defines the three WireMock MCP backend containers (crm-mcp, orders-mcp, kb-mcp) |
| `agent.py` | The AI agent — connects to all three MCP servers through the gateway and runs a tool-use loop with Claude |
| `requirements.txt` | Python dependencies (`anthropic`, `mcp`, `httpx`) |
| `.env.example` | Template for secrets — copy to `.env` and fill in your API key |
| `wiremock/` | Mock MCP backends for CRM, Orders, and Knowledge Base. Each subfolder (`crm/`, `orders/`, `kb/`) contains WireMock mapping rules and response data |

---

## Setup

**1. Configure your API key**

```bash
cp .env.example .env
# Open .env and set your Anthropic API key
```

**2. Make scripts executable**

```bash
chmod +x setup.sh teardown.sh test.sh configure-gateway.sh
```

**3. Run**

```bash
./setup.sh
```

`setup.sh` handles everything in order: downloads and starts the WSO2 AI Gateway, starts the three WireMock containers, connects them to the gateway's Docker network, registers all proxy resources, and then runs the agent with the default question.

The first run takes a few minutes because it downloads the gateway. Subsequent runs are faster since the download is cached.

---

## Running the tests

Run `test.sh` after `setup.sh` has completed. It first verifies that all MCP backends and gateway routes are reachable, then runs the agent and checks the response.

**Default smoke test** (verifies the agent returns info about John Smith / order O-9901):

```bash
./test.sh
```

**Custom question** (agent runs with your question; you verify the response yourself):

```bash
./test.sh "What is the return policy for gold tier members?"
./test.sh "Look up Priya Nair and tell me if she can still return her order."
```

The agent's full response is always printed to the terminal so you can read it regardless of pass/fail.

---

## Teardown

To stop all containers and clear the gateway's registered configuration:

```bash
./teardown.sh
```

This disconnects the MCP containers from the gateway network, stops all containers, and removes the gateway state volume. The next `./setup.sh` will start from a clean slate. The downloaded gateway zip is kept so it does not need to be re-downloaded.
