# WebSocket API Support

## Overview

Proxy WebSocket APIs (RFC 6455) through the Envoy-based gateway as a first-class `WebSocketApi` artifact kind, defined from AsyncAPI. Policies are enforced on the handshake, and connection lifetime and concurrency are bounded — client-facing bounds by gateway configuration, backend connection pools per upstream definition. The feature is managed gateway-first, from platform-api, from the `ap` CLI, the console and the API Portal, in the same way as GraphQL APIs.

## Requirements

### Routing and Protocol
- One `GET` route per AsyncAPI channel (with path parameters), proxied to `ws`/`wss` upstreams over HTTP/1.1
- WebSocket upgrades enabled only on `WebSocketApi` routes; every other route rejects upgrades
- Subprotocol negotiation, ping/pong and close frames passed through unchanged
- Main and sandbox upstreams following the RestApi model: an inline `ws`/`wss` url, or a `ref` to an `upstreamDefinitions` entry (weighted targets, `basePath`, connect timeout, TLS); upstream auth on the handshake
- Optional WebSocket over HTTP/2 (RFC 8441) behind a gateway config flag

### Policies and Security
- Policy chains are handshake-only; body-processing policies are rejected at deploy time
- Credentials accepted from a header, a query parameter or `Sec-WebSocket-Protocol`; stripped before the upstream and redacted from logs and analytics
- Channel-level scopes and an origin allowlist (cross-site WebSocket hijacking defence)

### Connection Lifecycle and Limits
- Client-facing idle timeout and finite maximum connection duration, set by gateway configuration (no per-API override)
- Per-upstream-definition backend idle timeout (`timeout.idleTimeout`, beside `connect`) and `connection` block (maximum connection duration and concurrent connections), with gateway defaults and maximums; beyond the limit and its bounded pending queue, handshakes get `503`
- Gateway-wide downstream connection limit

### Management and Experience
- Gateway management API `/websocket-apis` (CRUD and API keys)
- platform-api `/websocket-apis` (AsyncAPI import, deploy/undeploy/restore, builds, API keys, gateway version gating, bulk sync)
- `ap gateway websocket-api` CLI, console wizard and WebSocket test console, API Portal publishing
- One analytics event per connection with duration, bytes and close reason

## Success Criteria

- Upgrade attempts on non-`WebSocketApi` routes are rejected 100% of the time
- Invalid or missing credentials never establish a connection, and credentials never reach the backend, logs or analytics
- Idle and maximum-duration limits are enforced within 1 second
- Handshakes beyond an upstream definition's `maxConnections` and its bounded pending queue are rejected with `503`
- 10,000 concurrent idle connections per gateway instance without consuming policy-engine streams

## Implementation

[WebSocket API Support](../impls/4-websocket-api-support/spec.md) · [Research](../impls/4-websocket-api-support/research.md)
