# Stock Notifications WebSocket Server

## Overview

This is a sample WebSocket server that simulates a real-time stock notification feed. Every 3 seconds, it picks one of four sample stock symbols at random, generates a simulated price for it, and broadcasts the result to every client connected at that moment.

```json
{"symbol":"ACME","price":145.32,"change_percent":1.24,"timestamp":"2026-09-16T10:04:11.203Z"}
{"symbol":"CONTOSO","price":88.71,"change_percent":-0.56,"timestamp":"2026-09-16T10:04:14.208Z"}
{"symbol":"FABRIKAM","price":112.05,"change_percent":2.03,"timestamp":"2026-09-16T10:04:17.211Z"}
```

`asyncapi.yaml` is the AsyncAPI contract describing this channel and its message shape.

## Files

| File | Purpose |
|---|---|
| `README.md` | This file |
| `server.js` | The stock notification server: generates a simulated price tick every 3 seconds and broadcasts it to every connected client |
| `package.json` | Declares the dependencies needed for this project |
| `asyncapi.yaml` | AsyncAPI contract describing the notification channel and message shape |

## Prerequisites

- Node.js 18 or later, and npm

## Run it locally

```shell
npm install
npm start
```

This starts the server on `ws://localhost:8080`. Set the `PORT` environment variable to use a different port.
