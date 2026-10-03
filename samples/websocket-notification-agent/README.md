# WebSocket Notification Agent

## Overview

This sample runs an agent that connects to a stock notification WebSocket stream, and for any price movement that crosses an alert threshold, asks an LLM which tool to call in response, then actually calls it. Below-threshold ticks never reach the LLM at all. The agent only escalates to a model when something's actually worth deciding on.

`agent.py` calls your own deployed WSO2 API Platform Cloud and AI Workspace resources: a WebSocket API proxy, an MCP proxy, and an LLM provider, with credentials.

None of the three tools: `log_watch`, `send_alert`, or `escalate` do anything real. Every tool just logs a line on `tools-server` and returns a status. This demonstrates the LLM's decision and the governed call to invoke it, not the consequence.

## What this demonstrates

- **A rule-based filter in front of an LLM call**: a plain threshold check (`|change_percent| > 1.5`) decides *whether* to involve the LLM at all, keeping routine ticks cheap and silent.
- **LLM-driven tool selection**: when the threshold is crossed, the agent asks the LLM to choose one of three tools (via standard function-calling), rather than just generating text.
- **A real, executable consequence**: the LLM's chosen tool is actually invoked through a governed MCP proxy, not just described though each tool only prints a message and returns a status; none of them have a real side effect (see `tools-server/server.js` below).
- **Every hop governed**: the WebSocket stream, the LLM call, and the tool call each go through their own WSO2-managed proxy or provider, none of them called directly.

## Files

| File | Purpose |
|---|---|
| README.md | This file |
| agent.py | The agent: connects to the WebSocket stream, asks Gemini for a decision through the LLM provider, and calls the chosen tool through the MCP proxy |
| requirements.txt | Python dependencies for `agent.py` |
| tools-server/server.js | A minimal hand-rolled MCP server exposing `log_watch`, `send_alert`, and `escalate`, fronted by the MCP proxy. Each tool only logs a line and returns a status. There's no real alerting or ticketing system behind them |
| package.json | Declares the `start:tools-server` script |
| .gitignore | Ignores `node_modules`, Python virtual environments, and OS files |

## Prerequisites

| Tool | Purpose |
|---|---|
| Node.js 18 or later | Runs `tools-server` |
| Python 3.9 or later, pip | Runs `agent.py` |
| A WSO2 API Platform Cloud account, and access to AI Workspace | Where you deploy the WebSocket API, MCP proxy, and LLM provider this sample calls |

## Set up

This sample doesn't provision anything on its own. Follow the [companion guide](https://wso2.com/api-platform/docs/guides/websocket/build-an-ai-agent-for-websocket-notifications/) to:

1. Deploy `tools-server` publicly, and create and deploy an MCP proxy in front of it.
2. Set up an AI gateway, and create and deploy an LLM provider for Gemini.
3. Publish the WebSocket API from the [previous guide's sample](https://github.com/wso2/api-platform/tree/main/samples/websocket-notification-system), subscribe to it, and generate an access token.

Each of those steps tells you which environment variable below to collect the resulting URL or credential into.

**Run `tools-server` locally:**

```shell
npm run start:tools-server
```

Then deploy it (or a tunnel to it) publicly, so the MCP proxy has something to reach.

## Run agent.py

**Install dependencies:**

```shell
pip install -r requirements.txt
```

**Required environment variables:**

| Variable | Where it comes from |
|---|---|
| `STOCK_WS_URL` | Your deployed WebSocket API proxy's `wss://` invoke URL, from the Developer Portal's API Documentation page (guide Step 5) |
| `WS_ACCESS_TOKEN` | An access token from that application's **Manage Keys** page in the Developer Portal (guide Step 5) |
| `MCP_URL` | Your deployed MCP proxy's invoke URL, ending in `/mcp` (guide Step 3) |
| `LLM_URL` | Your deployed LLM provider's invoke URL (guide Step 4) |
| `LLM_API_KEY` | The API key generated for that LLM provider (guide Step 4) |

Optional: `MODEL` (defaults to `gemini-3.6-flash`), `THRESHOLD` (defaults to `1.5`).

**Run it:**

```shell
export STOCK_WS_URL="wss://<your-stock-notifications-api-url>"
export WS_ACCESS_TOKEN="<your-websocket-api-access-token>"
export MCP_URL="https://<your-mcp-proxy-invoke-url>/mcp"
export LLM_URL="https://<your-llm-provider-invoke-url>"
export LLM_API_KEY="<your-llm-provider-api-key>"
python agent.py
```

`agent.py` exits immediately with a clear message if any required variable is missing, rather than failing partway through.

## What you should see

Running `python agent.py` connects to the stock stream and starts reacting to ticks. Most lines look like:

```
[agent] CONTOSO 0.11% -- below threshold, skipping
```

Every so often, a tick crosses the ±1.5% threshold:

```
[agent] CONTOSO 1.78% at $210.38 -- threshold crossed, asking Gemini...
[agent] Gemini chose "log_watch" with arguments {'symbol': 'CONTOSO', 'note': 'Minor price change of 1.78% observed, price at $210.38.'}
[agent] tool result: {"status":"logged","symbol":"CONTOSO","note":"Minor price change of 1.78% observed, price at $210.38."}
```

## Cleanup

Stop `tools-server` and `agent.py` with <kbd>Control+C</kbd>. To remove the cloud resources, delete the MCP proxy, LLM provider, and AI gateway from AI Workspace, and the application from the Developer Portal.


## Try the guide

This sample follows the guide's exact pattern. If you haven't yet, walk through it to provision the WebSocket API, MCP proxy, and LLM provider this sample calls:

[Build an AI agent that reacts to WebSocket notifications](https://wso2.com/api-platform/docs/guides/websocket/build-an-ai-agent-for-websocket-notifications/)
