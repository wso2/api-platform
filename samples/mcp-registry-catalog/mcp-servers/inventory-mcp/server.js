#!/usr/bin/env node
// Minimal MCP server (streamable HTTP transport, JSON responses) exposing one
// tool: check_stock. No external dependencies -- plain Node http, so the
// image builds with no npm install and nothing to pull at runtime.
'use strict';

const http = require('http');

const PORT = process.env.PORT || 8080;
const SERVER_NAME = 'inventory-mcp';
const SERVER_VERSION = '1.0.0';

const STOCK = {
  'sku-1001': { name: '27-inch Monitor', quantity: 42, warehouse: 'EU-West' },
  'sku-2002': { name: 'Mechanical Keyboard', quantity: 0, warehouse: 'EU-West' },
  'sku-3003': { name: 'USB-C Dock', quantity: 17, warehouse: 'APAC' },
};

const TOOLS = [
  {
    name: 'check_stock',
    description: 'Look up current stock quantity and warehouse for a SKU.',
    inputSchema: {
      type: 'object',
      properties: { sku: { type: 'string', description: 'Stock keeping unit, e.g. "sku-1001"' } },
      required: ['sku'],
    },
  },
];

function callTool(name, args) {
  if (name !== 'check_stock') {
    throw new Error(`Unknown tool: ${name}`);
  }
  const sku = String(args && args.sku || '').toLowerCase();
  const item = STOCK[sku];
  if (!item) {
    return { content: [{ type: 'text', text: `No stock record for SKU "${args.sku}".` }], isError: true };
  }
  const text = `${item.name} (${args.sku}): ${item.quantity} units in ${item.warehouse}`;
  return { content: [{ type: 'text', text }] };
}

function handleRpc(body) {
  if (typeof body !== 'object' || body === null || Array.isArray(body) || typeof body.method !== 'string') {
    return { jsonrpc: '2.0', id: null, error: { code: -32600, message: 'Invalid Request' } };
  }
  const { id, method, params } = body;
  switch (method) {
    case 'initialize':
      return {
        jsonrpc: '2.0', id,
        result: {
          protocolVersion: '2025-06-18',
          capabilities: { tools: {} },
          serverInfo: { name: SERVER_NAME, version: SERVER_VERSION },
        },
      };
    case 'tools/list':
      return { jsonrpc: '2.0', id, result: { tools: TOOLS } };
    case 'tools/call':
      try {
        return { jsonrpc: '2.0', id, result: callTool(params.name, params.arguments) };
      } catch (err) {
        return { jsonrpc: '2.0', id, error: { code: -32602, message: err.message } };
      }
    default:
      return { jsonrpc: '2.0', id, error: { code: -32601, message: `Method not found: ${method}` } };
  }
}

const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    return res.end(JSON.stringify({ status: 'ok' }));
  }
  if (req.method !== 'POST') {
    res.writeHead(404);
    return res.end();
  }
  let raw = '';
  req.on('data', (chunk) => { raw += chunk; });
  req.on('end', () => {
    let parsed;
    try {
      parsed = JSON.parse(raw);
    } catch {
      res.writeHead(400, { 'Content-Type': 'application/json' });
      return res.end(JSON.stringify({ jsonrpc: '2.0', id: null, error: { code: -32700, message: 'Parse error' } }));
    }
    const result = handleRpc(parsed);
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(result));
  });
});

server.listen(PORT, () => {
  console.log(`[${SERVER_NAME}] listening on :${PORT}`);
});
