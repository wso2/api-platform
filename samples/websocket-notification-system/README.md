# WebSocket-Based Real-Time Notification System

> Companion sample for the [Build a WebSocket-based real-time notification system](https://wso2.com/api-platform/docs/guides/websocket/build-a-websocket-notification-system/) guide.

## Overview

This sample runs a local real-time notification system: a WebSocket server that picks a random sample stock symbol every 3 seconds, generates a simulated price for it, and broadcasts the result to every connected client, plus a few subscriber clients that connect to it. You run one script to start it, then one script to watch broadcast delivery happen in real time. No cloud account is required for this part.

`server.js` is the exact backend the companion guide walks you through deploying publicly and fronting with a WebSocket API proxy on WSO2 API Platform Cloud. The WebSocket API proxy feature itself is available on WSO2 API Platform Cloud only — the self-hosted API Gateway doesn't document a way to expose a WebSocket API, so this sample doesn't stand up a local gateway the way some other samples do. An optional section further down shows how to expose this same local server publicly and front it with a real WebSocket API proxy, so you can see it governed end to end.

## What this demonstrates

- **Broadcast delivery**: a single published tick reaches every client connected at that moment, with the identical symbol, price, and timestamp.
- **No effect from a dropped connection**: one client disconnecting doesn't stop ticks from reaching the others.
- **No history replay**: a client that connects late only sees ticks published after it connected, not ones published before.

## Files

| File | Purpose |
|---|---|
| README.md | This file |
| server.js | The stock notification server: generates a simulated price tick every 3 seconds and broadcasts it to every connected client. Identical to the backend in the companion guide. |
| package.json | Declares the `ws` dependency |
| asyncapi.yaml | AsyncAPI contract describing the notification channel and message shape. Used by the companion guide's Import API Contract step. |
| client/subscriber.js | A minimal subscriber client used by `demo.sh` to simulate a dashboard |
| scripts/lib.sh | Shared bash helpers (logging, `require_cmd`) |
| setup.sh | Installs dependencies, starts the local notification server |
| demo.sh | Connects subscriber clients so you can watch broadcast delivery |
| teardown.sh | Stops the server, cleans up local log and PID files |

## Prerequisites

- `bash`
- Node.js 18 or later, and npm

## Quick start

```shell
./setup.sh
./demo.sh
```

## What you should see

`demo.sh` connects three subscriber clients (`dashboard-1`, `dashboard-2`, `dashboard-3`), then walks through three steps:

1. **Broadcast to everyone**: waits for a tick and prints the line each of the three clients received. All three show the identical symbol, price, and timestamp.
2. **Disconnect doesn't break delivery**: disconnects `dashboard-2`, waits for the next tick, and shows `dashboard-1` and `dashboard-3` still receiving it while `dashboard-2` receives nothing.
3. **No replay for late joiners**: connects a fourth client, `dashboard-4`, after several ticks have already been published, then waits for the next tick. `dashboard-4`'s log has only that one line, while the others have accumulated more.

## Optional: front this with a WSO2 API Platform WebSocket API proxy

If you have a WSO2 API Platform Cloud account and want to see this same server governed by a real WebSocket API proxy, as covered in the [companion guide](https://wso2.com/api-platform/docs/guides/websocket/build-a-websocket-notification-system/), expose it publicly first.

1. With `./setup.sh` already running, install [`cloudflared`](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) and start a quick tunnel to the local WebSocket port. This doesn't require a Cloudflare account:

    ```shell
    cloudflared tunnel --url http://localhost:8080
    ```

    Cloudflare prints a `https://<random-subdomain>.trycloudflare.com` URL. Your WebSocket endpoint is the same host with a `wss://` scheme, for example `wss://<random-subdomain>.trycloudflare.com`.

2. In the [API Platform Console](https://console.bijira.dev), create a WebSocket API proxy using **Import API Contract** with this repository's [`asyncapi.yaml`](asyncapi.yaml), then replace the pre-filled **Target** value with the tunnel URL from step 1. Follow the companion guide from Step 3 onward to configure, deploy, and test it.

3. In the built-in **WebSocket Console**, connect to your deployed proxy, and wait a few seconds.

    A tick arrives in the Console exactly as your local server generated it, proxied through WSO2 API Platform Cloud. Since the server publishes on its own schedule, you don't need to trigger anything yourself.

Keep the `cloudflared` tunnel running for as long as you want the cloud-hosted proxy to reach your local server. Closing it breaks that connection; your local `demo.sh` flow keeps working without it.

## Cleanup

```shell
./teardown.sh
```

This stops the local notification server and removes local log and PID files. It does **not** remove any WebSocket API proxy you created in WSO2 API Platform Cloud — delete that manually from the console if you no longer need it.

## Troubleshooting

| Symptom | Resolution |
|---|---|
| `setup.sh` reports the server didn't become healthy | Check `.server.log` for a stack trace. A common cause is port 8080 already being in use — set `NOTIFICATION_PORT` to a different value and re-run. |
| `demo.sh` exits immediately with "Notification server isn't running" | Run `./setup.sh` first. |
| A client log file shows a connection error instead of "connected" | The server isn't listening on the port the client expects. Confirm `NOTIFICATION_PORT` matches between `setup.sh` and `demo.sh` if you overrode the default. |
| The WebSocket Console in WSO2 API Platform Cloud never receives a tick | Confirm the `cloudflared` tunnel is still running and the proxy's endpoint URL matches the current tunnel URL — quick tunnel URLs change every time you restart `cloudflared`. |
