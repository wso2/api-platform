# Feature Overview: Gateway Event Notification System

## Overview

This feature implements a real-time event notification system that enables the platform API to communicate with gateway instances through persistent WebSocket connections. Gateways establish authenticated connections to the platform and receive immediate notifications when API deployments, undeployments, or configuration changes occur, eliminating the need for polling and ensuring gateways stay synchronized with the platform state in real-time.

## Capabilities

### [✓] Capability 01: Gateway Connection Management

- [✓] **User Story 01** — Gateway Establishes Persistent Connection
  - **As a** gateway instance
  - **I want to** establish a persistent connection to the platform and have it maintained reliably
  - **So that** I can receive real-time events when they occur

- **Functional Requirements:**
  - [✓] **FR-001** Platform provides WebSocket connection endpoint at `wss://localhost:8443/api/internal/v1/ws/gateways/connect`
  - [✓] **FR-002** Platform authenticates gateway connections and rejects unauthorized attempts
  - [✓] **FR-003** Platform maintains in-memory registry of connected gateways with support for multiple connections per gateway ID
  - [✓] **FR-004** Platform stores connection handle associated with gateway identifier upon successful connection
  - [✓] **FR-005** Platform removes gateway from registry and logs disconnection events
  - [✓] **FR-006** Platform detects ungraceful disconnections within 30 seconds using heartbeat mechanisms
  - [✓] **FR-024** Platform sends connection acknowledgment message confirming successful registration

- **Key Implementation Highlights:**
  - `platform-api/src/internal/websocket/` - WebSocket connection management and transport abstraction
  - `platform-api/src/internal/handler/websocket.go` - WebSocket upgrade handler with authentication
  - `platform-api/src/config/config.go` - WebSocket configuration (max connections, timeout, rate limits)

**Notes:**
> Supports gateway clustering through multiple connections per gateway ID. Uses gorilla/websocket library with heartbeat ping/pong every 20 seconds and 30-second timeout detection.

---

### [~] Capability 02: Real-Time Event Broadcasting

- [ ] **User Story 02** — Platform Sends API Deployment Notification
  - **As a** platform
  - **I want to** send API deployment notifications to target gateways via established connections
  - **So that** gateways can immediately apply new configurations

- **Functional Requirements:**
  - [ ] **FR-007** Platform looks up target gateway connections and broadcasts deployment events
  - [ ] **FR-008** Deployment events include API UUID, revision details, and vhost (excluding API YAML)
  - [ ] **FR-009** Platform logs delivery failures when gateway is not connected
  - [ ] **FR-014** Platform maintains event delivery order per gateway

- **Key Implementation Highlights:**
  - `platform-api/src/internal/service/gateway_events.go` - Event broadcasting service
  - `platform-api/src/internal/service/api.go` - Integration with API deployment lifecycle
  - `platform-api/src/internal/model/gateway_event.go` - Event data models

**Notes:**
> Events are lightweight notifications containing references rather than full API YAML payloads. Gateways fetch complete API details separately via internal API runtime artifacts endpoint.

---

### [ ] Capability 03: Multi-Event Type Support

- [ ] **User Story 04** — Multiple Event Types Support
  - **As a** platform operator
  - **I want to** send different types of events to gateways
  - **So that** the system can support various operational scenarios beyond API deployment

- **Functional Requirements:**
  - [ ] **FR-010** Platform supports multiple event types (api.deployed, api.undeployed, gateway.config.updated)
  - [ ] **FR-011** System provides abstraction mechanism for registering new event types without modifying core code
  - [ ] **FR-012** Connection and event delivery uses abstraction layer isolating transport logic
  - [ ] **FR-013** Transport abstraction supports pluggable implementations

- **Key Implementation Highlights:**
  - `platform-api/src/internal/websocket/transport.go` - Transport abstraction interface
  - `platform-api/src/internal/websocket/websocket_transport.go` - WebSocket protocol implementation
  - `platform-api/src/internal/dto/gateway_event.go` - Event payload DTOs

**Notes:**
> Transport abstraction allows future protocol changes (e.g., replacing WebSocket with Server-Sent Events or gRPC) without rewriting business logic.

---

### [ ] Capability 04: Operational Visibility and Monitoring

- [ ] **User Story 03** — Platform Tracks Connected Gateways
  - **As a** platform administrator
  - **I want to** see which gateways are currently connected
  - **So that** I can verify event delivery is possible and troubleshoot connectivity issues

- [ ] **User Story 05** — Failed Event Delivery Handling
  - **As a** platform operator
  - **I want to** know about failed event deliveries
  - **So that** I can take corrective action

- **Functional Requirements:**
  - [ ] **FR-015** Platform logs all connection events (connections, disconnections, auth failures)
  - [ ] **FR-016** Platform logs all event delivery attempts with success/failure outcomes
  - [ ] **FR-017** Platform provides administrative API to query connected gateways
  - [ ] **FR-018** Connection status API returns gateway ID, connection timestamp, heartbeat time, health status
  - [ ] **FR-019** Platform provides API to query failed event deliveries within time window

- **Key Implementation Highlights:**
  - `platform-api/src/internal/handler/stats.go` - Statistics API endpoint
  - `platform-api/src/internal/websocket/stats.go` - In-memory delivery statistics tracking
  - `platform-api/src/resources/openapi.yaml` - Stats endpoint OpenAPI specification

**Notes:**
> Statistics are tracked in-memory using atomic counters. Counters reset on server restart. Future iterations may add persistent event history if needed.

---

### [✓] Capability 05: Security and Access Control

- [✓] **User Story 06** — Gateway Authentication and Authorization
  - **As a** security administrator
  - **I want to** ensure gateways authenticate when establishing connections
  - **So that** only authorized gateways can receive events

- **Functional Requirements:**
  - [✓] **FR-002** Platform authenticates gateway connections and rejects unauthorized attempts
  - [✓] **FR-020** Platform enforces maximum concurrent connection limit (default 1000)
  - [✓] **FR-021** Platform enforces maximum event payload size (default 1MB)
  - [✓] **FR-022** Gateway connections use encrypted transport (TLS/SSL required)
  - [✓] **FR-023** Platform implements rate limiting on connection attempts (default 10/minute/IP)

- **Key Implementation Highlights:**
  - `platform-api/src/internal/handler/websocket.go` - Authentication middleware with API key validation
  - `platform-api/src/internal/websocket/manager.go` - Rate limiting and connection limit enforcement
  - `platform-api/config/` - TLS/SSL configuration via HTTPS server

**Notes:**
> Uses API key-based authentication. Gateways provide `api-key` header during WebSocket upgrade. Credentials are validated using existing gateway service before connection is established.

---
