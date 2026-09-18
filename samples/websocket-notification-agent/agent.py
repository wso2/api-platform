#!/usr/bin/env python3
# Agent matching the "Build an AI agent that reacts to WebSocket
# notifications" guide. Connects to a WebSocket API proxy in WSO2 API
# Platform Cloud, and an MCP proxy and an LLM provider in AI Workspace,
# using your own deployed resources' URLs and credentials.
import asyncio
import json
import os
import sys

import websockets
from google import genai
from google.genai import types as genai_types
from mcp import ClientSession
from mcp.client.streamable_http import streamable_http_client

REQUIRED_ENV_VARS = ["STOCK_WS_URL", "WS_ACCESS_TOKEN", "MCP_URL", "LLM_URL", "LLM_API_KEY"]

missing = [name for name in REQUIRED_ENV_VARS if not os.environ.get(name)]
if missing:
    print(f"[agent] Missing required environment variable(s): {', '.join(missing)}", file=sys.stderr)
    print("[agent] See README.md for where to get each one.", file=sys.stderr)
    sys.exit(1)

STOCK_WS_URL = os.environ["STOCK_WS_URL"]
WS_ACCESS_TOKEN = os.environ["WS_ACCESS_TOKEN"]
MCP_URL = os.environ["MCP_URL"]
LLM_URL = os.environ["LLM_URL"]
LLM_API_KEY = os.environ["LLM_API_KEY"]
MODEL = os.environ.get("MODEL", "gemini-3.6-flash")
THRESHOLD = float(os.environ.get("THRESHOLD", "1.5"))


async def handle_notification(notification, tools_session, gemini_tools, gemini):
    symbol = notification["symbol"]
    price = notification["price"]
    change_percent = notification["change_percent"]

    if abs(change_percent) <= THRESHOLD:
        print(f"[agent] {symbol} {change_percent}% -- below threshold, skipping")
        return

    print(f"[agent] {symbol} {change_percent}% at ${price} -- threshold crossed, asking Gemini...")

    response = gemini.models.generate_content(
        model=MODEL,
        contents=(
            f"A stock notification just fired.\nSymbol: {symbol}\n"
            f"Price: ${price}\nChange: {change_percent}%\n\n"
            "Decide which single tool to call to handle it."
        ),
        config=genai_types.GenerateContentConfig(
            tools=[genai_types.Tool(function_declarations=gemini_tools)]
        ),
    )

    for part in response.candidates[0].content.parts:
        if not part.function_call:
            continue
        name = part.function_call.name
        args = dict(part.function_call.args)
        print(f'[agent] Gemini chose "{name}" with arguments {args}')
        result = await tools_session.call_tool(name, arguments=args)
        text = result.content[0].text if result.content else ""
        print(f"[agent] tool result: {text}")
        return

    print("[agent] Gemini did not choose a tool.")


async def run_agent():
    # verify=False: the self-hosted AI gateway from the guide's Step 3 provisions
    # a self-signed TLS certificate by default, which Python rejects otherwise.
    # If you've since put a CA-signed certificate on the gateway, drop this.
    http_options = genai_types.HttpOptions(
        base_url=LLM_URL,
        headers={"X-API-Key": LLM_API_KEY},
        client_args={"verify": False},
    )
    gemini = genai.Client(api_key="placeholder", http_options=http_options)

    async with streamable_http_client(MCP_URL) as (read, write):
        async with ClientSession(read, write) as tools_session:
            await tools_session.initialize()
            tools = await tools_session.list_tools()
            gemini_tools = [
                genai_types.FunctionDeclaration(
                    name=t.name, description=t.description, parameters=t.input_schema
                )
                for t in tools.tools
            ]

            ws_headers = {"Authorization": f"Bearer {WS_ACCESS_TOKEN}"}
            async with websockets.connect(STOCK_WS_URL, additional_headers=ws_headers) as ws:
                print(f"[agent] connected to {STOCK_WS_URL}")
                async for message in ws:
                    notification = json.loads(message)
                    await handle_notification(notification, tools_session, gemini_tools, gemini)


if __name__ == "__main__":
    asyncio.run(run_agent())
