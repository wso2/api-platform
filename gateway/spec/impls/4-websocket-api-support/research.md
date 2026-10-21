# Research: Proxying WebSocket APIs through the API Platform Gateway's Envoy

**Date**: 2026-10-13
**Envoy**: v1.39.0 (the `gateway-runtime` image version, `gateway/gateway-runtime/Dockerfile`)
**Feature spec**: [spec.md](spec.md)

## Executive Summary

Envoy can proxy WebSocket APIs for the standalone gateway with configuration changes only: a per-route upgrade opt-in, plus idle and maximum-duration limits. The policy engine (ext_proc) works for policies that run on the handshake. With body modes `NONE`, Envoy closes the ext_proc stream right after the `101`, so the engine holds no state per connection.

Two findings shape the design:

1. **Asking ext_proc for a body on an upgrade deadlocks the handshake.** This held in every request body mode (`BUFFERED`, `STREAMED`, `FULL_DUPLEX_STREAMED`) and for a buffered response body. WebSocket policy chains must be header-only.
2. **Config changes don't reach live connections.** Undeploy, revocation and policy changes only affect new handshakes. A finite maximum connection duration is the bound on stale access.

## Current State of the Gateway (`upstream/main`)

| Area | Code | Effect on WebSocket |
|---|---|---|
| Upgrade support | `translator.go` `createListener`: no `upgrade_configs` | Every upgrade is rejected: `403 upgrade_failed` |
| HTTP filters | ext_proc → Lua → router | Lua rewrites headers only, so it is fine for handshakes |
| Upstream protocol | Clusters set no HTTP protocol options, so HTTP/1.1 | Correct for WebSocket upgrades |
| Downstream TLS | ALPN `h2, http/1.1` | HTTP/2 browsers fall back to an HTTP/1.1 WebSocket unless `allow_connect` is set |
| Route timeouts | `route_timeout_ms` 60000 (validator rejects ≤ 0), `route_idle_timeout_ms` 300000 | Route timeout does not end upgraded connections; idle timeout does |
| HCM timeouts | `stream_idle_timeout` 5m, connection `idle_timeout` 1h, `request_timeout` 0 | Stream idle applies to upgraded streams |
| ext_proc | fail-closed, `allow_mode_override`, request headers `SEND`, grpc/message timeouts 60s | Engine chooses body modes per request (see below) |
| Engine body mode | `execution_context.go` `getModeOverride`: `BUFFERED`/`FULL_DUPLEX_STREAMED` when a chain needs a body; `responseHasNoBody()` is true for 1xx | Request body: deadlock risk. Response side resolves to `NONE` on a `101` |
| Engine streams | `DefaultMaxConcurrentStreams` 10000 | Not consumed by WebSocket connections in `NONE` mode |
| Analytics | From the Envoy ALS / access log, emitted at stream end | One event per connection, at close |
| Kinds | `RestApi`, `GraphQLApi`, `Mcp`, `LlmProvider`, `LlmProxy`, agents, `WebSubApi` via hooks | No WebSocket kind |

## Spike Setup

- **Envoy:** v1.39.0 in Docker. Its HCM mirrors the gateway's (`createListener`/`createExtProcFilter`): codec AUTO, ext_proc → router, ext_proc fail-closed with `allow_mode_override: true`, request headers `SEND`, grpc and message timeouts 60s. Routes come from RDS (file-based, so they can be changed while connections are open). Defaults match the gateway: route timeout 60s, route idle 300s, HCM `stream_idle_timeout` 300s.
- **Mock ext_proc:** answers request headers with a `ModeOverride` and passes everything through. The body mode is selected per request; every stream open/close and every message is logged.
- **Backend:** a WebSocket echo server (gorilla/websocket).
- **Client:** opens N connections, sends text or ping frames at an interval, and reports how and when each one ended.

The real policy engine, policies and gateway-controller were not run. Their Envoy-facing contract (modes, timeouts, failure mode) was reproduced, and the real engine's mode logic was checked in code.

## Results

| # | Scenario | Result |
|---|---|---|
| 01 | No `upgrade_configs` (today's gateway) | `403`, `details=upgrade_failed` |
| 02 | HCM `websocket enabled:false` + route `enabled:true` | WebSocket route → `101`; plain route → `403 upgrade_failed`. Subprotocol negotiated end to end. `?access_token=…` reached ext_proc, the backend, and the access-log path |
| 03 | Route timeout 60s (gateway default), active client for 80s | Survived 80s; the route timeout does not apply after the upgrade |
| 04 | Route timeout 0, ext_proc grpc/message timeouts 60s, 80s | Survived 80s |
| 05 | Route idle 10s: idle, text every 4s, ping every 4s | Idle client closed at 10s (`SI`, `stream_idle_timeout`; client sees `1006`). Text and ping-only clients survived 30s |
| 06 | 20 concurrent connections, body mode NONE | `cluster.ext_proc.upstream_rq_active = 0` with 20 active; the ext_proc stream ends right after the `101` |
| 07 | Request body `BUFFERED` | Handshake deadlock: the backend never receives the handshake; client times out (`code=0`) |
| 08 | Request body `STREAMED` | Same deadlock |
| 08b | Request body `FULL_DUPLEX_STREAMED` | Same deadlock |
| 08c | Request `NONE` + response `BUFFERED` / `FULL_DUPLEX_STREAMED` | `BUFFERED`: backend sends `101`, Envoy holds it, client times out. `FULL_DUPLEX_STREAMED`: works; ext_proc receives every server→client frame as raw RFC 6455 bytes (e.g. `81 05 6d73672d30`, an unmasked FIN text frame) |
| 08d | Response `FULL_DUPLEX_STREAMED`, 80s vs 60s ext_proc timeouts | Survived; one ext_proc stream held open for the whole connection, not cut by the 60s timeouts |
| 09 | Route removed via RDS while connected; new client after removal | Existing connection kept working until the client closed it (22s later); new handshake got `404 NR route_not_found` |
| 10 | Route `max_stream_duration: 15s` | Closed at 15.0s (`details=max_duration_timeout`); client sees `1006` |
| 11 | Circuit breaker `max_connections: 3`, `max_pending_requests: 0`, 5 simultaneous | All 5 rejected `503 UO`; upgrades pass through the pending queue first |
| 11b | Same with `max_pending_requests: 1`, burst of 5 | 2 connected, 3 rejected |
| 11c | Same, arrivals 0.5s apart | 3 connected, 1 rejected, 1 queued ~8s until a slot freed |
| — | ext_proc unreachable at handshake | `500`, `ext_proc_error_gRPC_error_14` (fail closed) |

Access log: one line per connection, written at close, with `code=101`, `duration` = connection lifetime, and `rx/tx` bytes. A normal client close is flagged `DC` / `downstream_remote_disconnect`.

## Key Decisions

### 1. Opt in per route, disable at HCM level

- **Decision:** HCM `upgrade_configs: [{websocket, enabled: false}]`; route-level `enabled: true` only on `WebSocketApi` routes.
- **Rationale:** Verified in scenario 02. No other kind's route can be upgraded into a tunnel, and existing routes are unchanged.
- **Alternative rejected:** enabling `websocket` on the HCM for all routes. That would let a REST route be upgraded whenever the backend agrees.

### 2. Connection lifetime from route `idle_timeout` and `max_stream_duration`

- **Decision:** Per-API `idleTimeout` → route `idle_timeout`; `maxConnectionDuration` → route `max_stream_duration`. Both are bounded by gateway config, and the maximum duration is always finite.
- **Rationale:** Both enforced exactly (scenarios 05, 10). Pings keep a connection alive. The route `timeout` does not end upgraded connections (03), so it needs no special casing.

### 3. Handshake-only policy chains

- **Decision:** Reject body-processing policies for `WebSocketApi` at deploy time. The engine also forces `NONE` body modes for WebSocket routes as a fail-safe.
- **Rationale:** Any request-body mode, and buffered response mode, deadlocks the handshake (07, 08, 08b, 08c). With `NONE`, the engine holds no stream per connection (06), so WebSocket concurrency does not depend on `MaxConcurrentStreams`.

### 4. No per-message policies in this feature

- **Rationale:**
  - Client→server frames can't be processed: requesting the request body deadlocks the handshake.
  - Server→client frames can be processed in `FULL_DUPLEX_STREAMED` (08c, 08d), but that needs:
    - RFC 6455 frame parsing in the engine (fragmentation, control frames, `permessage-deflate` with decompression limits);
    - one ext_proc stream per connection for its whole life;
    - a gRPC round trip per frame.
- **Revisit:** as a separate feature if needed.

### 5. A dedicated connection limit instead of circuit breakers

- **Decision:** Implement a per-API concurrent connection limit that rejects immediately.
- **Rationale:** Cluster circuit breakers are per pod, sensitive to bursts (11b), and either reject everything (11) or leave handshakes queued (11c).

### 6. Bound stale authorization with maximum duration

- **Rationale:** Route removal does not affect live connections (09). Undeploy, revocation and policy changes all go through route/config updates, so the maximum connection duration is the effective upper bound on how long revoked access lasts.

### 7. The same circuit breaker rejected for downstream (decision 5) is the right tool for upstream (FR-016)

- **Decision:** Use the cluster's native `circuit_breakers.thresholds.max_connections` to cap concurrent connections to each upstream slot (`main`/`sandbox`), despite decision 5 rejecting the identical mechanism for the downstream per-API limit.
- **Rationale:** Decision 5 rejected circuit breakers for downstream because the *resource being protected* (how many clients may connect to the gateway) is exactly what a per-pod, best-effort mechanism can't bound precisely. For upstream, the resource being protected — outbound connections *this gateway instance* opens to the backend — genuinely is a per-instance quantity, so the same per-pod scope is the correct scope, not a limitation.
- **Re-reading spikes 11/11b/11c for this purpose:** these spikes already exercised the upstream cluster's circuit breaker against live WebSocket connections (not a theoretical reading of the Envoy docs). Spike 11c's result — 3 connected, 1 rejected, 1 queued ~8s **until a slot freed** — confirms a long-lived upgraded connection is held counted as active against the threshold for its *entire* open duration, not just during the initial handshake. That resolves the question of whether `max_connections` even applies to upgraded connections at all: it does.
- **Caveat carried over, not resolved by re-reading:** the same spikes show outcome depends on arrival timing, not just the configured number — identical `max_connections: 3` produced "reject all 5" (11), "admit 2 of 5" (11b), and "admit 3 of 5, queue 1" (11c). For upstream protection this is acceptable (erring toward rejecting more under contention is a safe failure mode for protecting a backend), unlike for downstream where SC-005 requires an exact, predictable admission count.
- **Still not covered:** all three spikes ran against a single-host echo backend (Spike Setup, above). Envoy's documented guarantee of *at least one connection per selected host* — the mechanism's actual precision floor — only produces overshoot above the configured `max_connections` when a cluster resolves to multiple hosts. That case was never exercised. See "Not Covered by the Spike."

## Implications for Analytics

- One event per connection at close; duration is connection lifetime, not latency.
- `DC` on a `101` is a normal client close and must not count as an error. Map the close reason from response flags and details: `DC`, `SI`, `max_duration_timeout`, `UC`/`UF`.
- Query-string credentials appear in the logged path and must be redacted before the ALS stream.

## Not Covered by the Spike

- HTTP/2 downstream (RFC 8441 `allow_connect`), TLS, and `wss` upstreams.
- Listener drain, hot restart and rolling updates.
- Load and performance. In scenario 06, 4 of 20 simultaneous handshakes took ~1s; this was most likely the local Docker port-forward and was not investigated.
- The real policy engine and policies (`api-key-auth`, `jwt-auth`) on handshakes.
- **Upstream circuit-breaker (`max_connections`) overshoot on a multi-host cluster (FR-016).** Spikes
  11/11b/11c ran against a single-host echo backend (Spike Setup, above), so they confirm the
  mechanism holds a connection counted for its full lifetime (decision 7), but never exercised
  Envoy's documented guarantee of *at least one connection per selected host* — the actual source
  of overshoot above the configured threshold, which only manifests when the upstream cluster
  resolves to multiple hosts (e.g. a `main` upstream backed by several DNS-resolved instances).
  Needs its own spike before FR-016 is considered fully verified.
