# Getting Started

Connect to the Navigation API using any WebSocket client.

## Connect

```javascript
const ws = new WebSocket('wss://navigation.example.com/ws/route', {
  headers: { Authorization: 'Bearer <your-token>' }
});
```

## Request a route

```json
{
  "requestId": "req-001",
  "origin":      { "lat": 51.5074, "lng": -0.1278 },
  "destination": { "lat": 51.5033, "lng": -0.1195 },
  "mode": "walking"
}
```

## Receive updates

The server streams `RouteUpdate` messages as the route progresses:

```json
{
  "requestId": "req-001",
  "step": 1,
  "instruction": "Head south on Whitehall",
  "distanceRemainingMeters": 1240,
  "estimatedArrival": "2025-06-11T14:35:00Z"
}
```

If the user goes off-route, a `RerouteEvent` is emitted and the coordinates array is recalculated automatically.
