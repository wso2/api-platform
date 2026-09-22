#!/usr/bin/env node
// A minimal, hand-rolled MCP server (JSON-RPC over HTTP, no SDK dependency)
// exposing three tools an agent can call in response to a stock notification.
// The MCP proxy from the companion guide fronts this server; agent.py never
// calls it directly.
'use strict';

const http = require('http');

const PORT = process.env.PORT || 8091;

const MAX_BODY_BYTES = 1024 * 1024;

const TOOLS = [
  {
    name: 'log_watch',
    description: 'Record a low-priority observation about a stock price movement for later review.',
    inputSchema: {
      type: 'object',
      properties: {
        symbol: { type: 'string', description: 'The stock symbol' },
        note: { type: 'string', description: 'A short note about what was observed' },
      },
      required: ['symbol', 'note'],
    },
  },
  {
    name: 'send_alert',
    description: 'Raise a visible alert about a significant stock price movement.',
    inputSchema: {
      type: 'object',
      properties: {
        symbol: { type: 'string', description: 'The stock symbol' },
        message: { type: 'string', description: 'The alert message' },
      },
      required: ['symbol', 'message'],
    },
  },
  {
    name: 'escalate',
    description: 'Flag a stock price movement for human follow-up. Reserve this for severe or unusual cases.',
    inputSchema: {
      type: 'object',
      properties: {
        symbol: { type: 'string', description: 'The stock symbol' },
        reason: { type: 'string', description: 'Why this needs human attention' },
      },
      required: ['symbol', 'reason'],
    },
  },
];

function callTool(name, args) {
  switch (name) {
    case 'log_watch':
      console.log(`[tools-server] log_watch: ${args.symbol} -- ${args.note}`);
      return { status: 'logged', symbol: args.symbol, note: args.note };
    case 'send_alert':
      console.log(`[tools-server] send_alert: ${args.symbol} -- ${args.message}`);
      return { status: 'alert_sent', symbol: args.symbol, message: args.message };
    case 'escalate':
      console.log(`[tools-server] escalate: ${args.symbol} -- ${args.reason}`);
      return { status: 'escalated', symbol: args.symbol, reason: args.reason };
    default:
      throw new Error(`Unknown tool: ${name}`);
  }
}

function respond(res, status, body) {
  res.writeHead(status, body ? { 'Content-Type': 'application/json' } : undefined);
  res.end(body ? JSON.stringify(body) : undefined);
}

const server = http.createServer((req, res) => {
  if (req.method !== 'POST') {
    respond(res, 404);
    return;
  }

  let body = '';
  let received = 0;
  req.on('data', (chunk) => {
    received += chunk.length;
    if (received > MAX_BODY_BYTES) {
      respond(res, 413, { jsonrpc: '2.0', id: null, error: { code: -32600, message: 'Request body too large' } });
      req.destroy();
      return;
    }
    body += chunk;
  });

  req.on('end', () => {
    if (res.writableEnded) {
      return;
    }

    let request;
    try {
      request = JSON.parse(body || '{}');
    } catch (err) {
      respond(res, 400, { jsonrpc: '2.0', id: null, error: { code: -32700, message: 'Parse error' } });
      return;
    }

    // JSON.parse accepts null, arrays and bare values. Destructuring those
    // would throw outside this handler and take the process down.
    if (request === null || typeof request !== 'object' || Array.isArray(request)) {
      respond(res, 400, { jsonrpc: '2.0', id: null, error: { code: -32600, message: 'Invalid Request' } });
      return;
    }

    const { id, method, params } = request;

    // A request with no id is a notification (e.g. notifications/initialized)
    // and gets a bare 202 with no body, per the MCP transport spec.
    if (id === undefined) {
      respond(res, 202);
      return;
    }

    if (method === 'initialize') {
      // Echo back whatever protocol version the client asked for, rather than
      // hardcoding one -- WSO2 API Platform Cloud's MCP proxy only supports
      // 2025-03-16, while the self-hosted gateway's Mcp resource declares
      // 2025-06-18, so this server needs to work with both.
      const requestedVersion = (params && params.protocolVersion) || '2025-06-18';
      respond(res, 200, {
        jsonrpc: '2.0',
        id,
        result: {
          protocolVersion: requestedVersion,
          capabilities: { tools: {} },
          serverInfo: { name: 'stock-agent-tools', version: '1.0.0' },
        },
      });
      return;
    }

    if (method === 'tools/list') {
      respond(res, 200, { jsonrpc: '2.0', id, result: { tools: TOOLS } });
      return;
    }

    if (method === 'tools/call') {
      try {
        const result = callTool(params.name, params.arguments || {});
        respond(res, 200, {
          jsonrpc: '2.0',
          id,
          result: { content: [{ type: 'text', text: JSON.stringify(result) }] },
        });
      } catch (err) {
        respond(res, 200, { jsonrpc: '2.0', id, error: { code: -32602, message: err.message } });
      }
      return;
    }

    respond(res, 200, { jsonrpc: '2.0', id, error: { code: -32601, message: `Method not found: ${method}` } });
  });
});

server.listen(PORT, () => {
  console.log(`Agent tools MCP server listening on port ${PORT}`);
});
