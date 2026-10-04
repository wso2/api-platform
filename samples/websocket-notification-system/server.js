#!/usr/bin/env node
// Stock notification server used by the companion guide and by setup.sh /
// demo.sh in this sample. Picks a random sample symbol every few seconds,
// generates a simulated price for it, and broadcasts the result to every
// client currently connected. There's no message history or replay: a
// client only receives notifications broadcast while it's connected.
'use strict';

const { WebSocketServer } = require('ws');

const PORT = process.env.PORT || 8080;
const SYMBOLS = ['ACME', 'NORTHWIND', 'CONTOSO', 'FABRIKAM'];
const TICK_INTERVAL_MS = 3000;

const prices = new Map(SYMBOLS.map((symbol) => [symbol, 100 + Math.random() * 100]));
const wss = new WebSocketServer({ port: PORT });

wss.on('connection', (ws) => {
  console.log(`Client connected (${wss.clients.size} total)`);
  ws.on('close', () => console.log(`Client disconnected (${wss.clients.size} total)`));
});

function publishTick() {
  const symbol = SYMBOLS[Math.floor(Math.random() * SYMBOLS.length)];
  const previousPrice = prices.get(symbol);
  const changePercent = Number(((Math.random() - 0.5) * 4).toFixed(2));
  const price = Number((previousPrice * (1 + changePercent / 100)).toFixed(2));
  prices.set(symbol, price);

  const notification = {
    symbol,
    price,
    change_percent: changePercent,
    timestamp: new Date().toISOString(),
  };
  const payload = JSON.stringify(notification);

  for (const client of wss.clients) {
    if (client.readyState === client.OPEN) {
      client.send(payload);
    }
  }
}

setInterval(publishTick, TICK_INTERVAL_MS);
console.log(`Stock notification server listening on port ${PORT}`);
