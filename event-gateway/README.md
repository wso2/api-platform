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
- **Policies** — Pluggable enforcement at four points per channel:

| Policy point | YAML key | Triggered when |
|---|---|---|
| `on_subscription` | `subscribe` | A client subscribes at the hub |
| `on_unsubscription` | `unsubscribe` | A client unsubscribes at the hub |
| `on_message_received` | `inbound` | An event is published via the webhook receiver |
| `on_message_delivery` | `outbound` | An event is delivered to a subscriber callback |

Policies can be applied at two scopes:
- **`policies`** — applied uniformly to every channel in the API (e.g., authentication)
- **`channels.<name>`** — applied only to a specific named channel (e.g., RBAC per topic)

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

Use the **"Create Repo Watcher"** request. This registers a WebSub API with two channels (`issues`, `pull-requests`) via the gateway controller, which pushes the configuration to the event gateway over xDS.

The spec uses two policy scopes:

- **`policies`** — API-wide policies applied to every channel (e.g., authentication on every subscribe/unsubscribe)
- **`channels`** — Per-channel policies applied only to the named channel (e.g., RBAC per topic)

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

    "policies": {
      "on_subscription": [],
      "on_unsubscription": [],
      "on_message_received": [],
      "on_message_delivery": []
    },

    "channels": {
      "issues": {
        "policies": {
          "on_subscription": [
            {
              "name": "rbac",
              "version": "v1",
              "params": { "allowedRoles": ["admin", "issue-manager"] }
            }
          ]
        }
      },
      "pull-requests": {
        "policies": {
          "on_subscription": [
            {
              "name": "rbac",
              "version": "v1",
              "params": { "allowedRoles": ["admin", "developer"] }
            }
          ]
        }
      }
    },

    "deploymentState": "deployed"
  }
}
```

#### Step 2: Subscribe to a Topic (WebSub collection)

Use the **"Subscribe"** request. This registers a callback URL to receive events for the `issues` topic. The event gateway verifies the subscription by sending a `GET` challenge to the callback URL.

```
POST http://localhost:8080/repos/v1.0/hub
X-API-Key: <your-api-key>
Content-Type: application/x-www-form-urlencoded

hub.mode=subscribe&hub.topic=issues&hub.callback=http://wh-listener:8090/&hub.secret=mysecret&hub.lease_seconds=3600
```

#### Step 3: Publish an Event (WebSub collection)

Use the **"Ingress"** request. This publishes an event to the `issues` channel. The event gateway writes it to Kafka and delivers it to all active subscribers.

```
POST http://localhost:8080/repos/v1.0/webhook-receiver?topic=issues
Content-Type: text/plain
X-Hub-Signature-256: sha256=<hmac-of-body>

issue0
```

#### Step 4: Unsubscribe from a Topic (WebSub collection)

```
POST http://localhost:8080/repos/v1.0/hub
X-API-Key: <your-api-key>
Content-Type: application/x-www-form-urlencoded

hub.mode=unsubscribe&hub.topic=issues&hub.callback=http://wh-listener:8090/
```

#### Step 5: Verify Delivery

Check the webhook listener logs to confirm the event was delivered:

```bash
docker compose logs wh-listener
```

You should see the event body and headers printed by the listener.

### WebBrokerApi Walkthrough

The WebBrokerApi enables bidirectional WebSocket ↔ Kafka protocol mediation. This walkthrough demonstrates creating a stock trading API where clients can produce messages to Kafka and consume messages in real-time over WebSocket.

#### Step 1: Create a WebBroker API

Use the following curl command to create a WebBrokerApi with a `prices` channel that maps to Kafka topics:

```bash
curl --location 'http://localhost:9090/api/management/v0.9/webbroker-apis' \
--header 'Content-Type: application/json' \
--header 'Accept: application/json' \
--header 'Authorization: Basic YWRtaW46YWRtaW4=' \
--data '{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebBrokerApi",
  "metadata": {
    "name": "stock-trading-v1.0"
  },
  "spec": {
    "displayName": "Stock Trading WebBroker API",
    "version": "v1.0",
    "context": "/stock-trading/v1.0",
    "receiver": {
      "name": "websocket-receiver",
      "type": "websocket"
    },
    "broker": {
      "name": "kafka-driver",
      "type": "kafka",
      "properties": {
        "brokers": [
          "kafka:29092"
        ]
      }
    },
    "allChannels": {
      "on_connection_init": {
        "policies": []
      },
      "on_produce": {
        "policies": []
      },
      "on_consume": {
        "policies": []
      }
    },
    "channels": {
      "prices": {
        "produceTo": {
          "topic": "stock.prices"
        },
        "consumeFrom": {
          "topic": "dummy.prices"
        },
        "on_connection_init": {
          "policies": []
        },
        "on_produce": {
          "policies": []
        },
        "on_consume": {
          "policies": []
        }
      }
    }
  }
}'
```

This creates a WebBrokerApi where:
- Client messages are published to the `stock.prices` Kafka topic
- Messages from the `dummy.prices` Kafka topic are delivered to the WebSocket client

#### Step 2: Connect via WebSocket

Install `wscat` if you haven't already:

```bash
npm install -g wscat
```

Connect to the WebBroker API and select the `prices` channel using the `X-channel` header:

```bash
wscat -c ws://localhost:8081/stock-trading/v1.0 -H "X-channel: prices"
```

Once connected, you'll see:
```
Connected (press CTRL+C to quit)
> 
```

#### Step 3: Monitor Messages Published to Kafka

In a new terminal, start a Kafka consumer to monitor messages that clients send via WebSocket:

```bash
docker exec -it event-gateway-kafka-1 /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic stock.prices \
  --from-beginning
```

Now, type a message in your WebSocket terminal (Step 2) and press Enter:

```
> {"symbol": "AAPL", "price": 150.25, "timestamp": "2026-05-13T10:30:00Z"}
```

The message should appear in the Kafka consumer terminal immediately.

#### Step 4: Publish Messages from Kafka to WebSocket

In another terminal, start a Kafka producer to send messages that will be delivered to WebSocket clients:

```bash
docker exec -it event-gateway-kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server localhost:9092 \
  --topic dummy.prices
```

Type a message in the Kafka producer terminal and press Enter:

```
> {"symbol": "GOOGL", "price": 2750.50, "timestamp": "2026-05-13T10:31:00Z"}
```

The message should appear in your WebSocket terminal (Step 2):

```
< {"symbol": "GOOGL", "price": 2750.50, "timestamp": "2026-05-13T10:31:00Z"}
```

**Key Points:**
- WebSocket → Kafka: Messages typed in wscat are published to `stock.prices`
- Kafka → WebSocket: Messages published to `dummy.prices` are delivered to the WebSocket client
- Bidirectional: Both directions work simultaneously over the same WebSocket connection
- Per-Connection Isolation: Each WebSocket connection gets its own Kafka consumer group

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
      "policies": {
        "on_subscription": [
          { "name": "api-key-auth", "version": "v1", "params": { "in": "header", "name": "X-API-Key" } }
        ],
        "on_unsubscription": [
          { "name": "api-key-auth", "version": "v1", "params": { "in": "header", "name": "X-API-Key" } }
        ],
        "on_message_received": [],
        "on_message_delivery": []
      },
      "channels": {
        "issues": {},
        "pull-requests": {},
        "commits": {}
      },
      "deploymentState": "deployed"
    }
  }'

# 2. Subscribe to the "issues" topic
curl -X POST http://localhost:8080/repos/v1.0/hub \
  -H "X-API-Key: <your-api-key>" \
  -d "hub.mode=subscribe&hub.topic=issues&hub.callback=http://wh-listener:8090/&hub.secret=mysecret&hub.lease_seconds=3600"

# 3. Publish an event
curl -X POST "http://localhost:8080/repos/v1.0/webhook-receiver?topic=issues" \
  -H "Content-Type: text/plain" \
  -d "issue0"

# 4. Unsubscribe
curl -X POST http://localhost:8080/repos/v1.0/hub \
  -H "X-API-Key: <your-api-key>" \
  -d "hub.mode=unsubscribe&hub.topic=issues&hub.callback=http://wh-listener:8090/"

# 5. Check delivery
docker compose logs wh-listener
```

## WebSubApi Spec Reference

The `WebSubApi` kind is configured via `policies` and `channels`. Both are optional; omitting a policy point leaves it open (no enforcement).

```json
{
  "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
  "kind": "WebSubApi",
  "metadata": { "name": "my-api" },
  "spec": {
    "displayName": "My API",
    "version": "v1.0",
    "context": "/my-api",

    "policies": {
      "on_subscription":    [ /* policies applied to every subscribe request   */ ],
      "on_unsubscription":  [ /* policies applied to every unsubscribe request */ ],
      "on_message_received":[ /* policies applied to every inbound event       */ ],
      "on_message_delivery":[ /* policies applied to every outbound delivery   */ ]
    },

    "channels": {
      "<channel-name>": {
        "policies": {
          "on_subscription":    [ /* channel-specific subscribe policies    */ ],
          "on_unsubscription":  [ /* channel-specific unsubscribe policies  */ ],
          "on_message_received":[ /* channel-specific inbound policies      */ ],
          "on_message_delivery":[ /* channel-specific outbound policies     */ ]
        }
      }
    },

    "deploymentState": "deployed"
  }
}
```

**Policy execution order:** `policies` policies run first, followed by the matching `channels` entry for that channel. Each policy object requires `name` and `version`; `params` is policy-specific.

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

**WebSubApi** — Multi-channel API. The `policies` block at root level maps to `policies`; each channel's `policies` block maps to its `channels` entry:

```yaml
channels:
  - kind: WebSubApi
    name: repo-watcher
    version: v1
    context: /repos
    channels:
      - name: issues
        policies:
          subscribe:            # → on_subscription (channel-level)
            - name: rbac
              version: v1
              params:
                allowedRoles: ["admin", "issue-manager"]
          unsubscribe: []       # → on_unsubscription (channel-level)
          inbound: []           # → on_message_received (channel-level)
          outbound: []          # → on_message_delivery (channel-level)
      - name: pull-requests
        policies:
          subscribe:
            - name: rbac
              version: v1
              params:
                allowedRoles: ["admin", "developer"]
          unsubscribe: []
          inbound: []
          outbound: []
    receiver:
      type: websub
    broker-driver:
      type: kafka
      config:
        brokers: ["kafka:29092"]
    policies:                   # → policies
      subscribe:                # → on_subscription
        - name: api-key-auth
          version: v1
          params: { in: header, name: X-API-Key }
      unsubscribe:              # → on_unsubscription
        - name: api-key-auth
          version: v1
          params: { in: header, name: X-API-Key }
      inbound: []               # → on_message_received
      outbound: []              # → on_message_delivery
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
      unsubscribe: []
      inbound: []
      outbound: []
```

The four `policies` keys map directly to the control-plane spec fields:

| `channels.yaml` key | Control plane field | Triggered when |
|---|---|---|
| `subscribe` | `on_subscription` | Client subscribes at the hub |
| `unsubscribe` | `on_unsubscription` | Client unsubscribes at the hub |
| `inbound` | `on_message_received` | Event published via webhook receiver |
| `outbound` | `on_message_delivery` | Event delivered to subscriber callback |

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

## Running Integration Tests

The integration tests exercise the full stack (controller → event gateway → Kafka → delivery).

### Prerequisites

Make sure all services are running:

```bash
cd event-gateway && docker compose up -d
```

### Run the Tests

```bash
cd event-gateway/gateway-runtime
make test
```

The integration test suite:

1. Creates a `WebSubApi` with `policies` and `channels` via the control plane REST API.
2. Verifies the xDS snapshot is pushed to the event gateway.
3. Subscribes a callback URL to each channel — `on_subscription` policies are enforced.
4. Publishes events and asserts delivery to the callback — `on_message_received` and `on_message_delivery` policies are enforced.
5. Unsubscribes and asserts `on_unsubscription` policies are enforced and no further delivery occurs.
6. Deletes the API and asserts cleanup.

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
│   │   └── channels.yaml        # Static channel bindings (used when control plane is disabled)
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
