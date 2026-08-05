# Getting Started

The Chat Service API is a WebSocket API: one long-lived connection carries messages in both
directions. Anything you send is broadcast to every other client connected to the same room, and
anything they send arrives on your socket.

## Endpoint

```
wss://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/godzilla/chat-service-api/v1.0
```

There is a single channel (`/`) — the whole API is that one socket. Unlike a REST API there are no
paths or methods to choose between.

## Message format

Every frame, in either direction, is a JSON object. Only `type` is always present — it
selects the frame's shape, and the service ignores any frame whose type it doesn't
recognise. `message` carries the content of a chat frame, so frames of other types don't
have one.

| Field | Type | Always present | Description |
|---|---|---|---|
| `type` | string | yes | Frame type, e.g. `connect` or `message` |
| `message` | string | no | The message content, on chat frames |

```json
{ "type": "message", "message": "Hello from user1" }
```

## Connecting

With [`websocat`](https://github.com/vi/websocat) from a terminal — type a JSON frame and press
enter to send it; incoming frames print as they arrive:

```bash
websocat "wss://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/godzilla/chat-service-api/v1.0"
```

From the browser:

```js
const socket = new WebSocket(
  'wss://db720294-98fd-40f4-85a1-cc6a3b65bc9a-prod.e1-us-east-azure.choreoapis.dev/godzilla/chat-service-api/v1.0'
);

socket.onopen = () => {
  socket.send(JSON.stringify({ type: 'connect', message: 'user1 joined' }));
};

socket.onmessage = (event) => {
  const frame = JSON.parse(event.data);
  console.log(`${frame.type}: ${frame.message}`);
};
```

## Trying it from the portal

Open the API's **Specification** page and switch on **Try it** to get a console that opens the
socket for you — useful for watching the broadcast behaviour without writing a client. Open it in
two browser tabs to see a message sent from one arrive in the other.

## Source

The backing service is published at
[wso2/api-platform-samples/chat-service-api](https://github.com/wso2/api-platform-samples/tree/main/chat-service-api).
