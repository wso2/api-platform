# MCP Tool Poisoning Demo

A defanged malicious MCP server, caught by a WSO2 API Platform gateway policy.

This sample demonstrates **MCP tool poisoning attack** from the client's point of view. It also illustrates how attaching an **MCP Access Control** policy to an MCP Proxy in WSO2 API Platform stops the poisoned tool from reaching a consumer, without you having to trust every MCP server you connect to.

## DISCLAIMER: Please read before running

- This sample license under the [Apache 2.0 License](https://github.com/wso2/api-platform-samples/blob/main/LICENSE) is provided for demonstrational purposes only, and is provided "AS IS," without any implied or express warranties or support.
- Use at your own risk, in an isolated test environment only. Never run it on production systems or networks you don't control.
- To the maximum extent permitted by law, WSO2 accepts no liability for any loss or damage arising from its use.

## About the Demo Artifact

- This artifact is a demo for educational purposes. It is not a working exploit and is not intended to compromise anything.
- The `evil-server` is a static [WireMock](https://wiremock.org/) stub. It does not execute code, read files, or contact the network on its own. It only serves a canned JSON response over HTTP.
- The injection payload embedded in the poisoned tool's `description` field is **plain, visible text**. It is not obfuscated, encoded, or hidden. Anyone reading the raw JSON can understand the injection payload.
- [`tools-list.json`](http://evil-server/mappings/tools-list.json) also carries a plain-English `_disclaimer` field explaining it's a demo, right next to the poisoned tool. It's in the response WireMock serves (not just the file on disk). So anyone who calls the `evil-server` directly using a curl, not just people reading this README, sees the context immediately.
- The payload's exfiltration target is `http://example.com`, a domain [reserved by IANA for documentation and examples](https://www.iana.org/help/example-domains) (RFC 2606). It is never actually contacted by anything in this demo.
- This demo never calls `tools/call`. It only calls `tools/list`, so the payload is inspected as data. Nothing in this repository ever acts on the instructions it contains. No LLM or agent is in the loop.
- **Do not point a real AI agent or MCP client** with file-system or network tool access at the poisoned tool description and ask it to "just try it." The whole point of the demo is that an agent *would* follow those instructions if nothing filtered them out first.

## What this demonstrates

MCP tool poisoning is a prompt-injection technique where a malicious or compromised MCP server embeds hidden instructions inside a tool's `description` field. A human skimming a tool list rarely reads full descriptions closely. However, an LLM client sends the *entire* description to the model as part of its context on every turn. A poisoned description can instruct the model to exfiltrate secrets, read files it wasn't asked to read, or misuse other tools, all while looking like an ordinary tool to the person who approved the connection.

MCP governance in WSO2 API Platform closes this type of blind spot. Instead of trusting every upstream MCP server, you design an **MCP Proxy** for it in **AI Workspace** and let the **AI Gateway** enforce policy on every request at runtime.

This demo uses the **MCP Access Control** policy. The policy can allow or deny access to specific resources with exceptions. The gateway independently applies these access rules to a proxy's tools, resources, and prompts. The policy doesn't inspect *content,* as there's no scan for prompt-injection patterns. It enforces which tool *names* a consumer is allowed to see or invoke at all. Configured as **default-deny**, only tools you've explicitly reviewed and allowlisted are ever exposed. So once `get_weather_report` is identified as poisoned (as pass 1 of this demo shows), simply *not* allowlisting it is enough to keep it from ever reaching a consumer, regardless of what the upstream MCP server does or changes next.

This gives you a governance point for MCP the same way an API gateway has always given you one for REST/GraphQL: one place to see, audit, and control what's actually being exposed. Trust shifts from "whatever the upstream MCP server happens to send" to "what an operator has explicitly approved."

- **`demo.sh` Pass 1: directly served, so exposed.** The client talks straight to the `evil-server`. Whatever it sends back is what the client gets (poisoned tool included).

- **`demo.sh` Pass 2: fronted by an MCP Proxy with a policy applied, so filtered.** The client talks to the MCP Proxy on AI Gateway. The proxy still fetches the same tool list from the `evil-server` behind the scenes, but the MCP Access Control policy checks it against an allowlist before responding, so only the approved tool makes it back to the client.

## Files

| File | Purpose |
| :---- | :---- |
| README.md | This file |
| setup.sh | Starts the `evil-server`, creates `.env` |
| demo.sh | Runs `pass1` / `pass2` / `all` |
| teardown.sh | Stops the `evil-server`, cleans up temp files |
| .env.example | Copy to `.env`; fill in after configuring the gateway |
| scripts/lib.sh | Shared bash helpers (logging, MCP request helper) |
| evil-server/mappings/initialize.json | WireMock stub for the MCP `initialize` call |
| evil-server/mappings/notifications-initialized.json | WireMock stub for the `notifications/initialized` handshake step |
| evil-server/mappings/tools-list.json | WireMock stub for `tools/list` \-- contains the poisoned tool description |

## Prerequisites

- `bash`, `curl`, `jq`
- `docker` (used to run the WireMock `evil-server`)
- **WSO2 API Platform** with its two components running:
  - **AI Workspace** \-- where you design the MCP Proxy and attach policies
  - **AI Gateway** \-- the runtime that serves the deployed proxy and enforces those policies on every request (this is what pass 2 talks to)

## Quick start (Pass 1: No gateway needed)

```shell
./setup.sh
./demo.sh pass1
```

You should see two tools returned by the `evil-server`'s `tools/list`, one of them flagged `POISONED` with the embedded instruction printed to the terminal. Nothing else is required for pass 1\.

## Configure the gateway (required for Pass 2\)

Pass 2 needs an MCP Proxy defined in AI Workspace, deployed to AI Gateway, with the MCP Access Control policy attached. Do this after `./setup.sh` has started the `evil-server`.

### 1\. Start AI Workspace and AI Gateway

Start (or restart) your local AI Workspace and AI Gateway containers.

### 2\. Create the MCP Proxy in AI Workspace

In AI Workspace, create a new **MCP Proxy** with the following details:

| Field | Value |
| :---- | :---- |
| Name | `Evil Tools MCP Proxy` |
| Context | `/evil-tools` |
| Version | `1.0.0` |
| MCP Proxy Endpoint URL | see below |

For the **MCP Proxy Endpoint URL**, the value depends on where AI Gateway is running relative to the `evil-server` container:

- **AI Gateway running as a Docker container on the same machine** \- Docker containers can't reach the host via `localhost`, so use:

```
http://host.docker.internal:8089/mcp
```

If that hostname isn't resolvable from inside the AI Gateway container, connect the `evil-server` container to AI Gateway's Docker network instead (`docker network connect <ai-gateway-network> mcp-poison-evil-server`) and use `http://mcp-poison-evil-server:8080/mcp` (container-to-container, internal port 8080).


- **AI Gateway running natively / on the same host network** (not containerized): use `http://localhost:8089/mcp` directly.
- **AI Gateway running on a different machine entirely**: use this machine's LAN IP instead of `localhost`, e.g. `http://192.168.1.20:8089/mcp`.

### 3\. Attach the MCP Access Control policy

On the Proxy's **Policies** tab in AI Workspace, click **Add Policies** and select **MCP Access Control**. In the panel, configure only the **tools** section. This proxy only exposes tools.

| Field | Value |
| :---- | :---- |
| tools.mode | `deny` |
| tools.exceptions | `list_open_invoices` |

**Note**: Press \<kbd\>Enter\</kbd\> after typing the value in the `tools.exceptions` field, so that it appears as a tag.

Here, every tool is denied except the ones listed as exceptions. Since `get_weather_report` (the poisoned tool) is deliberately left off the exceptions list, it gets denied not because its content was scanned, but because it was never explicitly approved. Click **Add**, then save the proxy.

### 4\. Deploy to AI Gateway

Deploy the proxy from AI Workspace to AI Gateway, then copy AI Gateway's invoke URL for it (and a subscription key or token if your deployment requires one).

### 5\. Point the demo at it

Copy `.env.example` to `.env` if `setup.sh` hasn't already done it for you, then set these variables:

```
GATEWAY_MCP_URL=<AI Gateway's invoke URL for the deployed MCP Proxy>
GATEWAY_API_KEY=<token, if required>
```

### 6\. Run Pass 2

```shell
./demo.sh pass2
```

Or run both passes back to back:

```shell
./demo.sh all
```

## What you should see

**Pass 1**: Both tools are returned. `get_weather_report` is flagged `POISONED` with the embedded instruction text printed to the terminal. The script warns that nothing is inspecting tool descriptions.

**Pass 2** (once the gateway is configured): Only `list_open_invoices` is returned. The script prints the differences between the two passes,  showing `get_weather_report` was removed by the gateway policy, and reports `0 poisoned tools returned`.

## Cleanup

```shell
./teardown.sh
```

This stops the `evil-server` container and removes local temp files. It does **not** remove the MCP Proxy you created, or stop AI Workspace or AI Gateway. Delete the proxy from AI Workspace and stop those containers manually if you no longer need them.  