#!/usr/bin/env node
// A minimal notification subscriber, used by demo.sh to simulate a
// dashboard client. Connects to the local stock notification server and
// prints every tick it receives while connected.
'use strict';

const WebSocket = require('ws');

const name = process.argv[2] || 'subscriber';
const serverPort = process.env.SERVER_PORT || 8080;
const url = `ws://localhost:${serverPort}`;

const ws = new WebSocket(url);

ws.on('open', () => {
  console.log(`[${name}] connected to ${url}`);
});

ws.on('message', (data) => {
  let notification;
  try {
    notification = JSON.parse(data.toString());
  } catch (err) {
    console.log(`[${name}] received unparseable message: ${data}`);
    return;
  }
  const time = new Date(notification.timestamp).toLocaleTimeString();
  const sign = notification.change_percent >= 0 ? '+' : '';
  console.log(`[${name}] ${time} ${notification.symbol} $${notification.price} (${sign}${notification.change_percent}%)`);
});

ws.on('close', () => {
  console.log(`[${name}] disconnected`);
});

ws.on('error', (err) => {
  console.error(`[${name}] error: ${err.message}`);
});

process.on('SIGTERM', () => ws.close());
process.on('SIGINT', () => ws.close());
