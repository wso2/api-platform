# Event Gateway

The Event Gateway is a lightweight, extensible runtime for managing event-driven APIs. It supports **WebSub** (webhook-based pub/sub) and **WebSocket** protocol mediation, backed by **Apache Kafka** as the broker.

## Architecture

```
┌──────────────────┐       xDS (gRPC)       ┌──────────────────────┐
│ Gateway Controller├──────────────────────►│   Event Gateway       │
│  (Control Plane)  │                        │     (Runtime)         │
│  :9090 mgmt API   │                        │  :8080 WebSub         │
│  :18001 xDS       │                        │  :8081 WebSocket      │
└────────┬─────────┘                        │  :9002 Admin          │
         │                                   │  :9003 Metrics        │
         │                                   └────┬────────┬────────┘
         │                                        │        │
         │                                        │ pub/sub│
         │                                   ┌────▼────────▼────┐
         └──────────────────────────────────►│     Kafka          │
                                             │  :9092 / :29092    │
                                             └───────────────────┘
```

**Key concepts:**

- **WebSubApi** — Multi-channel pub/sub API. Publishers send events via a webhook receiver; subscribers register callbacks at a hub endpoint. Each channel maps to a Kafka topic.
- **Protocol Mediation** — Bridges WebSocket clients to Kafka topics (1:1 passthrough).
- **Policies** — Pluggable enforcement at three points: `subscribe` (hub requests), `inbound` (event ingress), `outbound` (event delivery).

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [Postman](https://www.postman.com/downloads/) (optional, for manual testing)
- [Go 1.24+](https://go.dev/dl/) (only if building from source)

## Quick Start

### 1. Start All Services

From the `event-gateway/` directory:

```bash
cp .env.example .env
# edit .env and set GATEWAY_REGISTRATION_TOKEN

docker compose up -d
```

This starts five services:

| Service | Port(s) | Description |
|---------|---------|-------------|
| `gateway-controller` | `9090`, `18001` | Control plane — Management REST API and xDS server |
| `event-gateway` | `8080`, `8081`, `9002`, `9003` | Event gateway runtime |
| `kafka` | `9092`, `29092` | Apache Kafka broker |
| `wh-listener` | `8090` | Test webhook receiver (Go) |
| `kafka-ui` | `7080` | Kafka web UI |

### 2. Verify Services Are Running

```bash
# Check all containers are up
docker compose ps

# Health check the event gateway
curl http://localhost:9002/health
# → {"status":"UP"}

# Readiness check
curl http://localhost:9002/ready
# → {"status":"READY"}
```

### 3. Stop Services

```bash
docker compose down
```

To also remove volumes (Kafka data, controller DB):

```bash
docker compose down -v
```

## Testing with Postman

Two Postman collections are provided in [`spec/postman/`](spec/postman/):

| Collection | Description |
|------------|-------------|
| **Control Plane** | CRUD operations for WebSub APIs via the gateway controller |
| **WebSub** | Subscribe to topics and publish events through the event gateway |

### Setup

1. Import both collections into Postman from `spec/postman/`.
2. Set the `host` collection variable:
   - **Control Plane**: `localhost:9090`
   - **WebSub**: `localhost:8080`

### End-to-End Walkthrough

#### Step 1: Create a WebSub API (Control Plane collection)

Use the **"Create Repo Watcher"** request. This registers a WebSub API with three channels (`issues`, `pull-requests`, `commits`) via the gateway controller, which pushes the configuration to the event gateway over xDS.

```
POST http://localhost:9090/api/management/v0.9/websub-apis
Authorization: Basic admin:admin
Content-Type: application/json

{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": {
    "name": "repo-watcher-v1-0"
  },
  "spec": {
    "displayName": "repo-watcher",
    "version": "v1.0",
    "context": "/repos",
    "channels": [
      { "name": "issues", "method": "SUB" },
      { "name": "pull-requests", "method": "SUB" },
      { "name": "commits", "method": "SUB" }
    ],
    "policies": [
      {
        "name": "basic-auth",
        "version": "v1",
        "params": {
          "username": "admin",
          "password": "admin"
        }
      }
    ],
    "deploymentState": "deployed"
  }
}
```

#### Step 2: Subscribe to a Topic (WebSub collection)

Use the **"Subscribe"** request. This registers a callback URL to receive events for the `issues` topic. The event gateway verifies the subscription by sending a `GET` challenge to the callback URL.

```
POST http://localhost:8080/repos/v1.0/hub
Authorization: Basic admin:admin
Content-Type: application/x-www-form-urlencoded

hub.mode=subscribe
hub.topic=issues
hub.callback=http://wh-listener:8090/
hub.secret=mysecret
hub.lease_seconds=3600
```

#### Step 3: Publish an Event (WebSub collection)

Use the **"Ingress"** request. This publishes an event to the `issues` channel. The event gateway writes it to Kafka and delivers it to all active subscribers.

```
POST http://localhost:8080/repos/v1.0/webhook-receiver?topic=issues
Content-Type: text/plain

issue0
```

#### Step 4: Verify Delivery

Check the webhook listener logs to confirm the event was delivered:

```bash
docker compose logs wh-listener
```

You should see the event body and headers printed by the listener.

### Other Control Plane Operations

| Request | Method | URL |
|---------|--------|-----|
| List WebSub APIs | `GET` | `http://localhost:9090/api/management/v0.9/websub-apis` |
| Get WebSub API | `GET` | `http://localhost:9090/api/management/v0.9/websub-apis/repo-watcher-v1-0` |
| Update WebSub API | `PUT` | `http://localhost:9090/api/management/v0.9/websub-apis/repo-watcher-v1-0` |
| Delete WebSub API | `DELETE` | `http://localhost:9090/api/management/v0.9/websub-apis/repo-watcher-v1-0` |

All control plane requests require Basic Auth (`admin`/`admin`).

## Testing with cURL

You can run the full flow without Postman:

```bash
# 1. Create a WebSub API
curl -X POST http://localhost:9090/api/management/v0.9/websub-apis \
  -u admin:admin \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "WebSubApi",
    "metadata": { "name": "repo-watcher-v1-0" },
    "spec": {
      "displayName": "repo-watcher",
      "version": "v1.0",
      "context": "/repos",
      "channels": [
        { "name": "issues", "method": "SUB" },
        { "name": "pull-requests", "method": "SUB" },
        { "name": "commits", "method": "SUB" }
      ],
      "policies": [{
        "name": "basic-auth", "version": "v1",
        "params": { "username": "admin", "password": "admin" }
      }],
      "deploymentState": "deployed"
    }
  }'

# 2. Subscribe to the "issues" topic
curl -X POST http://localhost:8080/repos/v1.0/hub \
  -u admin:admin \
  -d "hub.mode=subscribe&hub.topic=issues&hub.callback=http://wh-listener:8090/&hub.secret=mysecret&hub.lease_seconds=3600"

# 3. Publish an event
curl -X POST "http://localhost:8080/repos/v1.0/webhook-receiver?topic=issues" \
  -H "Content-Type: text/plain" \
  -d "issue0"

# 4. Check delivery
docker compose logs wh-listener
```

## Configuration

### Runtime Configuration (`config.toml`)

The event gateway is configured via [`gateway-runtime/configs/config.toml`](gateway-runtime/configs/config.toml):

| Section | Key | Default | Description |
|---------|-----|---------|-------------|
| `server` | `websub_port` | `8080` | WebSub listener port |
| `server` | `websub_tls_enabled` | `false` | Serve the WebSub listener with HTTPS |
| `server` | `websub_tls_cert_file` | `""` | PEM certificate path for the WebSub HTTPS listener |
| `server` | `websub_tls_key_file` | `""` | PEM private key path for the WebSub HTTPS listener |
| `server` | `websocket_port` | `8081` | WebSocket listener port |
| `server` | `admin_port` | `9002` | Admin/health endpoint port |
| `server` | `metrics_port` | `9003` | Metrics endpoint port |
| `kafka` | `brokers` | `["localhost:9092"]` | Kafka bootstrap servers |
| `kafka` | `consumer_group_prefix` | `event-gateway` | Kafka consumer group prefix |
| `websub` | `delivery_max_retries` | `5` | Max delivery retry attempts |
| `websub` | `delivery_concurrency` | `64` | Concurrent delivery workers |
| `websub` | `verification_timeout_seconds` | `10` | Subscription verification timeout |
| `controlplane` | `enabled` | `true` | Enable xDS control plane integration |
| `controlplane` | `xds_address` | `localhost:18001` | xDS server address |

All settings can be overridden via environment variables with the prefix `APIP_EGW_`:

```bash
APIP_EGW_SERVER_WEBSUB_PORT=8080
APIP_EGW_SERVER_WEBSUB_TLS_ENABLED=true
APIP_EGW_SERVER_WEBSUB_TLS_CERT_FILE=/etc/event-gateway/tls/tls.crt
APIP_EGW_SERVER_WEBSUB_TLS_KEY_FILE=/etc/event-gateway/tls/tls.key
APIP_EGW_KAFKA_BROKERS=broker1:9092,broker2:9092
APIP_EGW_CONTROLPLANE_ENABLED=true
```

When `websub_tls_enabled=true`, the event gateway serves `https://` on `websub_port`. If the gateway controller or router points at the event gateway directly, update `router.event_gateway.websub_hub_url` to use an `https://` URL.

### Channel Bindings (`channels.yaml`)

When the control plane is disabled, channels are loaded statically from [`gateway-runtime/configs/channels.yaml`](gateway-runtime/configs/channels.yaml). Two binding kinds are supported:

**WebSubApi** — Multi-channel API:

```yaml
channels:
  - kind: WebSubApi
    name: repo-watcher
    version: v1
    context: /repos
    channels:
      - name: issues
      - name: pull-requests
    receiver:
      type: websub
    broker-driver:
      type: kafka
      config:
        brokers: ["kafka:29092"]
    policies:
      subscribe:
        - name: basic-auth
          version: v1
          params: { username: "admin", password: "admin" }
      inbound: []
      outbound: []
```

**Flat binding** — Protocol mediation (WebSocket → Kafka):

```yaml
channels:
  - name: live-prices
    mode: protocol-mediation
    context: /prices
    version: v1
    receiver:
      type: websocket
      path: /stream
    broker-driver:
      type: kafka
      topic: price-updates
      config:
        brokers: ["kafka:29092"]
    policies:
      subscribe: []
      inbound: []
      outbound: []
```

## Building from Source

```bash
cd event-gateway/gateway-runtime

# Build the binary
make build

# Run unit tests
make test

# Build the Docker image
docker compose build event-gateway
```

## Useful Endpoints

| Endpoint | Port | Description |
|----------|------|-------------|
| `GET /health` | 9002 | Liveness probe — always returns `{"status":"UP"}` |
| `GET /ready` | 9002 | Readiness probe — `{"status":"READY"}` or 503 |
| `POST /{context}/{version}/hub` | 8080 | WebSub subscribe/unsubscribe over HTTP or HTTPS |
| `POST /{context}/{version}/webhook-receiver?topic=X` | 8080 | WebSub event ingress over HTTP or HTTPS |
| `ws://localhost:8081/{path}` | 8081 | WebSocket connection (protocol mediation) |
| Kafka UI | 7080 | Kafka topic browser at `http://localhost:7080` |

## Webhook Listener (Test Tool)

The `wh-listener` service is a small Go HTTP server for testing event delivery. It runs on port `8090` and:

- Responds to `GET` with WebSub verification challenges (`hub.challenge` echo).
- Logs `POST` webhook payloads (headers + body) to stdout.
- Starts quickly as a static binary and shuts down gracefully on `SIGTERM`/`SIGINT`.

View its output:

```bash
docker compose logs -f wh-listener
```

## Project Structure

```
event-gateway/
├── docker-compose.yaml          # Full stack: controller, gateway, kafka, listener, kafka-ui
├── gateway-runtime/
│   ├── Dockerfile               # Multi-stage Go build
│   ├── Makefile                 # build, test, clean targets
│   ├── VERSION                  # Current version (0.1.0-SNAPSHOT)
│   ├── cmd/event-gateway/       # Entry point and plugin registration
│   ├── configs/
│   │   ├── config.toml          # Runtime configuration
│   │   └── channels.yaml        # Static channel bindings
│   └── internal/
│       ├── admin/               # Health/readiness endpoints
│       ├── binding/             # Channel binding parser
│       ├── config/              # Configuration loader (TOML + env vars)
│       ├── connectors/          # Receiver and broker-driver interfaces + registry
│       │   ├── brokerdriver/kafka/  # Kafka connector
│       │   └── receiver/
│       │       ├── websub/      # WebSub protocol (hub, verification, delivery)
│       │       └── websocket/   # WebSocket protocol mediation
│       ├── hub/                 # Central message router and policy adapter
│       ├── runtime/             # Component orchestrator
│       ├── subscription/        # Subscription store and reconciler
│       └── xdsclient/           # xDS client for control plane integration
├── spec/postman/                # Postman collections for manual testing
└── webhook-listener/
    ├── Dockerfile               # Multi-stage Go build for the test listener
    ├── go.mod                   # Standalone Go module
    └── main.go                  # Test webhook callback receiver
```
