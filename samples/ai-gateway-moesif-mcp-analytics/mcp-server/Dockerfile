# The MCP reference server, used here as the sample's backend. It implements
# every part of the protocol, so the traffic through the gateway is real MCP
# traffic rather than canned replies.
#
# Pinned so every run of this sample exposes the same set of tools.
FROM node:22-alpine

ARG SERVER_EVERYTHING_VERSION=2026.8.31
RUN npm install -g --no-fund --no-audit \
    "@modelcontextprotocol/server-everything@${SERVER_EVERYTHING_VERSION}"

# Listens for JSON-RPC over HTTP POST on /mcp.
ENV PORT=3001
EXPOSE 3001

CMD ["mcp-server-everything", "streamableHttp"]
