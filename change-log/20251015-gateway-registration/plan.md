# Implementation Plan: Gateway to Control Plane Registration

**Branch**: `003-gateway-registration` | **Date**: 2025-10-22 | **Spec**: [spec.md](./spec.md)

## Summary

Implement automatic gateway registration with control plane via WebSocket connection on startup. Gateway reads registration token and control plane URL from environment variables, establishes persistent WebSocket connection using gorilla/websocket (matching control plane implementation), and maintains eventual consistency through timestamp-based reconciliation mechanism.

## Technical Context

**Language/Version**: Go 1.25.1
**Primary Dependencies**:
- `github.com/gorilla/websocket v1.5.3` - WebSocket client
- `github.com/knadh/koanf/v2 v2.3.0` - Configuration (already in use)
- `github.com/mattn/go-sqlite3 v1.14.32` - Event timestamp persistence (already in use)
- `go.uber.org/zap v1.27.0` - Structured logging (already in use)
- Prometheus client library (NEEDS CLARIFICATION - version selection)

**Storage**: SQLite (already in use) - for persisting event timestamps per event type
**Testing**: Go testing framework, `github.com/stretchr/testify v1.11.1` (already in use)
**Target Platform**: Linux server (containerized via Docker)
**Project Type**: Single (gateway-controller within gateway directory)
**Performance Goals**: Connection within 5s, reconnection within 30s, 24+ hour stability
**Constraints**: Zero-downtime per Constitution I, degraded mode during outages
**Scale/Scope**: Multiple instances per token, 15min polling interval

## Constitution Check

*GATE: Must pass before Phase 0 research.*

### I. Zero-Downtime Operations PASS
WebSocket runs in background goroutine, doesn't block traffic (FR-010, SC-005).

### II. Observability by Design PASS
Structured JSON logging to stdout (FR-009), Prometheus metrics (FR-016, FR-017).

### Test Independence PASS
Clear unit/integration test separation.

### Code Organization PASS
Follows existing `pkg/` structure pattern.

### Documentation Standards PASS
Includes architecture, design, and quickstart documentation.

## Project Structure

### Documentation

```
specs/003-gateway-registration/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 (/speckit.tasks command)
```

### Source Code

```
gateway/gateway-controller/
├── cmd/gateway-controller/
│   └── main.go                              # MODIFY: Initialize controlplane client
├── pkg/
│   ├── controlplane/                        # NEW: Control plane integration
│   │   ├── client.go
│   │   ├── reconnection.go
│   │   ├── reconciliation.go
│   │   ├── metrics.go
│   │   └── events.go
│   ├── models/
│   │   └── event_timestamp.go               # NEW
│   ├── storage/
│   │   └── event_repository.go              # NEW
│   └── config/
│       └── config.go                        # MODIFY: Add control plane config
└── tests/integration/
    ├── controlplane_connection_test.go      # NEW
    └── reconciliation_test.go               # NEW
```


## Complexity Tracking

*No violations.*
