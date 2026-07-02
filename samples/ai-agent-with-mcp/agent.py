"""
Multi-server MCP agent — local WSO2 AI Gateway edition.

Traffic flow:
  agent  →  WSO2 MCP Gateway (port 8080)  →  crm/orders/kb WireMock containers
  agent  →  WSO2 LLM Gateway (port 8443)  →  api.anthropic.com

Run:
    python agent.py
or with a custom question:
    QUESTION="..." python agent.py
"""

import asyncio
import json
import os
import warnings

import anthropic
import httpx
from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client

# Suppress the InsecureRequestWarning produced by verify=False on the local
# gateway's self-signed TLS certificate.
warnings.filterwarnings("ignore", message="Unverified HTTPS request")

# WSO2 AI Gateway endpoints
# MCP Gateway (HTTP) — context paths registered in configure-gateway.sh
CRM_MCP_URL    = "http://localhost:8080/crm/mcp"
ORDERS_MCP_URL = "http://localhost:8080/orders/mcp"
KB_MCP_URL     = "http://localhost:8080/kb/mcp"

# LLM Gateway (HTTPS, self-signed cert)
LLM_GW_URL = "https://localhost:8443/claude-agent"

# Inbound API key for the LLM proxy (registered in configure-gateway.sh).
# Must match INBOUND_API_KEY in .env (default: demo-api-key).
INBOUND_API_KEY = os.environ.get("INBOUND_API_KEY", "demo-api-key")

MODEL = "claude-sonnet-4-6"


async def run_agent(user_question: str) -> None:
    # Open Streamable HTTP connections to all three MCP proxies
    async with streamablehttp_client(CRM_MCP_URL) as (r1, w1, _):
        async with streamablehttp_client(ORDERS_MCP_URL) as (r2, w2, _):
            async with streamablehttp_client(KB_MCP_URL) as (r3, w3, _):
                async with ClientSession(r1, w1) as crm:
                    async with ClientSession(r2, w2) as orders:
                        async with ClientSession(r3, w3) as kb:

                            await crm.initialize()
                            await orders.initialize()
                            await kb.initialize()

                            crm_tools    = await crm.list_tools()
                            orders_tools = await orders.list_tools()
                            kb_tools     = await kb.list_tools()

                            all_tools = (
                                crm_tools.tools
                                + orders_tools.tools
                                + kb_tools.tools
                            )

                            # Map tool name → the MCP session that owns it
                            session_map: dict = {}
                            for t in crm_tools.tools:
                                session_map[t.name] = crm
                            for t in orders_tools.tools:
                                session_map[t.name] = orders
                            for t in kb_tools.tools:
                                session_map[t.name] = kb

                            claude_tools = [
                                {
                                    "name": t.name,
                                    "description": t.description,
                                    "input_schema": t.inputSchema,
                                }
                                for t in all_tools
                            ]

                            # Anthropic client routed through the WSO2 LLM Gateway.
                            # The gateway's api-key-auth policy validates INBOUND_API_KEY
                            # and then forwards the request to Anthropic using the real
                            # Anthropic key configured in the LlmProvider.
                            # verify=False accepts the gateway's self-signed certificate.
                            client = anthropic.Anthropic(
                                base_url=LLM_GW_URL,
                                api_key=INBOUND_API_KEY,
                                http_client=httpx.Client(verify=False),
                            )

                            messages = [{"role": "user", "content": user_question}]

                            # Agentic reasoning loop
                            while True:
                                response = client.messages.create(
                                    model=MODEL,
                                    max_tokens=4096,
                                    tools=claude_tools,
                                    messages=messages,
                                )

                                messages.append(
                                    {"role": "assistant", "content": response.content}
                                )

                                if response.stop_reason == "end_turn":
                                    for block in response.content:
                                        if hasattr(block, "text"):
                                            print(block.text)
                                    break

                                # Dispatch each tool_use to the right MCP server
                                tool_results = []
                                for block in response.content:
                                    if block.type != "tool_use":
                                        continue

                                    session = session_map.get(block.name)
                                    if session is None:
                                        raise ValueError(f"Unknown tool: {block.name}")

                                    try:
                                        result = await session.call_tool(
                                            block.name, arguments=block.input
                                        )
                                        content = (
                                            result.content[0].text
                                            if result.content
                                            else ""
                                        )
                                    except Exception as exc:
                                        content = f"Error calling {block.name}: {exc}"

                                    tool_results.append(
                                        {
                                            "type": "tool_result",
                                            "tool_use_id": block.id,
                                            "content": content,
                                        }
                                    )

                                messages.append(
                                    {"role": "user", "content": tool_results}
                                )


if __name__ == "__main__":
    question = os.environ.get(
        "QUESTION",
        (
            "Look up customer C-4821 (John Smith). Get his latest order and its status. "
            "Then search the knowledge base for any return or shipping policies relevant "
            "to his situation, and summarise your findings."
        ),
    )
    asyncio.run(run_agent(question))
