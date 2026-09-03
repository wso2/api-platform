#!/usr/bin/env node
// Minimal MCP server (streamable HTTP transport, JSON responses) exposing one
// tool: get_forecast. No external dependencies -- plain Node http, so the
// image builds with no npm install and nothing to pull at runtime.
'use strict';

const http = require('http');

const PORT = process.env.PORT || 8080;
const SERVER_NAME = 'weather-mcp';
const SERVER_VERSION = '1.0.0';

const FORECASTS = {
  colombo: { conditions: 'Humid with afternoon showers', highC: 30, lowC: 25 },
  london: { conditions: 'Overcast, light drizzle', highC: 16, lowC: 11 },
  tokyo: { conditions: 'Clear skies', highC: 24, lowC: 18 },
};

const TOOLS = [
  {
    name: 'get_forecast',
    description: 'Get a short-term weather forecast for a city.',
    inputSchema: {
      type: 'object',
      properties: { city: { type: 'string', description: 'City name, e.g. "Colombo"' } },
      required: ['city'],
    },
  },
];

function callTool(name, args) {
  if (name !== 'get_forecast') {
    throw new Error(`Unknown tool: ${name}`);
  }
  const city = String(args && args.city || '').toLowerCase();
  const forecast = FORECASTS[city];
  if (!forecast) {
    return { content: [{ type: 'text', text: `No forecast data for "${args.city}".` }], isError: true };
  }
  const text = `${args.city}: ${forecast.conditions}, high ${forecast.highC}C / low ${forecast.lowC}C`;
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
