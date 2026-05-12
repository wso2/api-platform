# WebBrokerApi: Protocol Mediation

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
  - [Key Components](#key-components)
  - [Per-Connection Model](#per-connection-model)
  - [Message Flows](#message-flows)
- [Policy Enforcement Points](#policy-enforcement-points)
- [Specification Format](#specification-format)
- [Example Use Cases](#example-use-cases)
- [Building and Running](#building-and-running)
  - [Option 1: Using Docker Compose (Recommended)](#option-1-using-docker-compose-recommended)
  - [Option 2: Building from Source](#option-2-building-from-source)
  - [Option 3: Development Mode with Live Reload](#option-3-development-mode-with-live-reload)
- [Testing with Policies](#testing-with-policies)
- [End-to-End Testing with Kafka](#end-to-end-testing-with-kafka)
- [Implementation Details](#implementation-details)
- [Comparison: WebSubApi vs WebBrokerApi](#comparison-websubapi-vs-webbrokerapi)
- [Consumer Group Strategy](#consumer-group-strategy)
- [Topic Subscription](#topic-subscription)
- [Troubleshooting](#troubleshooting)
- [Quick Reference](#quick-reference)
- [Future Enhancements](#future-enhancements)
- [Next Steps](#next-steps)

## Overview

**WebBrokerApi** is a new binding type in the Event Gateway that enables **protocol mediation** between web-friendly protocols (WebSocket, SSE) and message brokers (Kafka, MQTT, AMQP). It provides bidirectional streaming with per-connection isolation.

## Architecture

### Key Components

1. **Receiver**: Protocol adapter for web-friendly clients (WebSocket, SSE)
2. **Broker Driver**: Message broker adapter (Kafka, MQTT, AMQP)
3. **Policy Engine**: Message processing with three enforcement points

### Per-Connection Model

Each WebSocket connection gets:
- **Inbound Go Channel**: Handles messages from client → broker (produce path)
- **Outbound Go Channel**: Handles messages from broker → client (consume path)
- **Dedicated Kafka Consumer**: Unique consumer group per connection
- **Shared Kafka Producer**: Can publish to any topic dynamically

### Message Flows

**Produce Path** (Client → Broker):
```
WebSocket Client → Receiver → Inbound Channel → on_produce policies → Broker Driver → Kafka
```

**Consume Path** (Broker → Client):
```
Kafka → Broker Driver → Outbound Channel → on_consume policies → Receiver → WebSocket Client
```

## Policy Enforcement Points

Unlike WebSub's `subscribe/inbound/outbound`, WebBrokerApi has:

| Policy Point | When Applied | Purpose |
|--------------|-------------|---------|
| `on_connection_init.request` | WebSocket handshake (before upgrade) | Authentication, authorization |
| `on_connection_init.response` | WebSocket handshake (after upgrade) | Response customization |
| `on_produce` | Client sends message to broker | Topic mapping, validation, transformation |
| `on_consume` | Broker message delivered to client | Filtering, transformation |

## Specification Format

```yaml
kind: WebBrokerApi
apiId: unique-api-identifier
name: api-name
version: v1.0
context: /base-path

receiver:
  name: receiver-instance-name  # Instance identifier
  type: websocket  # or "sse" in the future
  properties: {}

brokerDriver:
  name: broker-instance-name  # Instance identifier
  type: kafka  # or "mqtt", "amqp" in the future
  properties:
    bootstrap.servers: localhost:9092
    security.protocol: PLAINTEXT

policies:  # API-level policies applied to all channels
  on_connection_init:
    request:
      - name: policy-name
        version: v1
        params: {}
    response: []
  on_produce:
    - name: policy-name
      version: v1
      params: {}
  on_consume: []

channels:  # Channel-specific configurations with per-channel policies
  "/channel-name":
    policies:
      onConnectionInit:
        request: []
        response: []
      on_produce:
        - name: map-topic
          version: v1
          params:
            mode: produceTo
            topic: kafka-topic-name
      on_consume:
        - name: map-topic
          version: v1
          params:
            mode: consumeFrom
            topic: kafka-topic-name
```

## Example Use Cases

### 1. WebSocket to Kafka with Channel Routing

```yaml
channels:
  - kind: WebBrokerApi
    name: websocket-kafka-api
    version: v1.0
    context: /ws-kafka
    receiver:
      name: ws-receiver
      type: websocket
    brokerDriver:
      name: kafka-driver
      type: kafka
      properties:
        bootstrap.servers: localhost:9092
    policies:  # API-level policies
      on_connection_init:
        request:
          - name: api-key-auth
            version: v1
            params:
              in: header
              name: X-API-Key
      on_produce: []
      on_consume: []
    channels:  # Per-channel configurations
      "/issues":
        policies:
          onConnectionInit:
            request: []
            response: []
          on_produce:
            - name: map-topic
              version: v1
              params:
                mode: produceTo
                topic: kafka-repo-issues
          on_consume:
            - name: map-topic
              version: v1
              params:
                mode: consumeFrom
                topic: kafka-repo-issues
      "/commits":
        policies:
          onConnectionInit:
            request: []
            response: []
          on_produce:
            - name: map-topic
              version: v1
              params:
              mode: produceTo
              topic: kafka-repo-commits
        on_consume:
          - name: map-topic
            version: v1
            params:
              mode: consumeFrom
              topic: kafka-repo-commits
```

**Client Usage:**
```javascript
// Connect with channel specified via X-topic header
const ws = new WebSocket('ws://localhost:8080/ws-kafka', {
  headers: {
    'X-API-Key': 'your-api-key',
    'X-topic': '/issues'  // Select channel
  }
});

// Send message (will produce to kafka-repo-issues)
ws.send(JSON.stringify({
  headers: {
    'X-Client-Topic': 'client-issues'
  },
  body: { issue: 'Bug report' }
}));

// Consume from all subscribed topics
ws.onmessage = (event) => {
  console.log('Received:', event.data);
};
```

## Implementation Details

### Files Modified/Created

1. **`internal/binding/types.go`**
   - Added `WebBrokerApiBinding` struct
   - Added `ProtocolMediationPolicies` struct
   - Added `ConnectionInitPolicies` struct

2. **`internal/binding/loader.go`**
   - Updated `ParseResult` to include `WebBrokerApiBindings`
   - Added `WebBrokerApi` case in parser

3. **`internal/hub/hub.go`**
   - Added `ProcessConnectionInitRequest()`
   - Added `ProcessConnectionInitResponse()`
   - Added `ProcessProduce()`
   - Added `ProcessConsume()`

4. **`internal/connectors/types.go`**
   - Updated `MessageProcessor` interface with new methods
   - Added `Topics` field to `ChannelInfo`

5. **`internal/connectors/receiver/websocket/broker_api_connector.go`** (NEW)
   - Implemented `WebBrokerApiReceiver`
   - Per-connection bidirectional streaming
   - Dedicated Kafka consumer/producer per connection

6. **`internal/runtime/runtime.go`**
   - Added WebBrokerApi processing in `LoadChannels()`
   - Added `buildWebBrokerApiPolicyChains()`

7. **`cmd/event-gateway/plugins.go`**
   - Registered `websocket-broker-api` receiver factory

### Connection Lifecycle

1. **WebSocket Upgrade**:
   - Client sends upgrade request
   - `on_connection_init.request` policies applied
   - If short-circuited, reject with policy-defined response
   - Upgrade to WebSocket
   - `on_connection_init.response` policies applied

2. **Resource Creation**:
   - Unique connection ID generated
   - Inbound/outbound Go channels created
   - Unique Kafka consumer group: `{prefix}-ws-{connID}`
   - Kafka consumer subscribes to all relevant topics
   - Kafka producer created for publishing

3. **Message Processing**:
   - **Read loop**: WebSocket → Inbound channel
   - **Inbound loop**: Inbound channel → on_produce policies → Kafka
   - **Outbound loop**: Outbound channel → on_consume policies → WebSocket
   - **Consumer callback**: Kafka → Outbound channel

4. **Connection Close**:
   - Stop Kafka consumer
   - Close inbound/outbound channels
   - Close WebSocket connection
   - Clean up connection from registry

## Comparison: WebSubApi vs WebBrokerApi

| Aspect | WebSubApi | WebBrokerApi |
|--------|-----------|--------------|
| **Use Case** | Async pub/sub with HTTP callbacks | Bidirectional streaming |
| **Protocol** | HTTP (POST to callbacks) | WebSocket, SSE |
| **Connection** | Stateless HTTP | Persistent streaming |
| **Isolation** | Per-callback consumer group | Per-connection consumer group |
| **Topics** | Multiple channels per API | Dynamic via policies |
| **Policy Points** | subscribe/inbound/outbound | connection_init/produce/consume |
| **Direction** | Unidirectional (gateway → callback) | Bidirectional (client ↔ broker) |

## Consumer Group Strategy

Each WebSocket connection gets a **unique consumer group** to ensure:
- Independent consumption (not load-balanced)
- Each client receives all messages
- No message loss on connection drop (offset tracked per connection)

Consumer group ID format: `{prefix}-ws-{uuid}`

## Channel Routing and Topic Subscription

For WebBrokerApi:
- **Channel Selection**: Client specifies channel via `X-topic` header during WebSocket handshake (e.g., `X-topic: /issues`)
- **Per-Channel Topics**: Each channel defines its own topics via `map-topic` policies with `mode: produceTo` and `mode: consumeFrom`
- **Consumer Subscription**: Each WebSocket connection subscribes only to topics configured for the selected channel
- **Producer Publishing**: Messages are published to the topic specified in the channel's `on_produce` policies
- **Policy Cascade**: API-level policies execute first, followed by channel-specific policies

## Future Enhancements

1. **SSE Support**: Add Server-Sent Events receiver
2. **More Brokers**: Add MQTT, AMQP, NATS broker drivers
3. **Topic Discovery**: Dynamic topic subscription based on client requests
4. **Connection Pooling**: Shared consumer groups for load balancing (optional mode)
5. **Backpressure Control**: Configurable buffer sizes and overflow strategies
6. **Metrics**: Per-connection throughput, latency, error rates

## Building and Running

The event gateway supports two configuration modes:

1. **Control Plane Mode (Recommended)**: Configure APIs through the gateway-controller REST API. The controller distributes configurations to the event-gateway via xDS protocol. This is the default mode in Docker Compose.

2. **Static File Mode**: Configure APIs by editing the `channels.yaml` file directly. Useful for development and testing without the controller.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [Go 1.24+](https://go.dev/dl/) (for building from source)
- [Kafka](https://kafka.apache.org/) (provided via Docker Compose)

### Option 1: Using Docker Compose (Recommended)

This is the easiest way to test protocol mediation with all dependencies.

#### 1. Start All Services

From the `event-gateway/` directory:

```bash
# Copy environment template
cp .env.example .env

# Start all services (Kafka, Event Gateway, Controller)
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f event-gateway
```

This starts:
- **Kafka** on `localhost:9092` (external) and `kafka:29092` (internal)
- **Event Gateway** on `localhost:8081` (WebSocket), `localhost:8080` (HTTP), `localhost:9002` (Admin API)
- **Gateway Controller** on `localhost:9090` (Management API), `localhost:18001` (xDS)

**Note:** TLS is currently disabled for local development. To enable HTTPS:
1. Generate certificates in `listener-certs/` directory
2. Set `websub_tls_enabled = false` in `gateway-runtime/configs/config.toml`
3. Restart services with `docker compose restart event-gateway`

#### 2. Create WebBrokerApi via Control Plane (Recommended)

The gateway runs in control plane mode by default, which means you configure APIs through the gateway-controller REST API:

```bash
# Create a WebBrokerApi via the gateway-controller
curl --location 'http://localhost:9090/api/management/v0.9/webbroker-apis' \
--header 'Content-Type: application/json' \
--header 'Authorization: Basic YWRtaW46YWRtaW4=' \
--data '{
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "WebBrokerApi",
    "metadata": { "name": "websocket-kafka-api-v1-0" },
    "spec": {
      "displayName": "websocket-kafka-api",
      "version": "v1.0",
      "context": "/ws-kafka",
      "receiver": {
        "name": "ws-receiver",
        "type": "websocket",
        "properties": {}
      },
      "brokerDriver": {
        "name": "kafka-driver",
        "type": "kafka",
        "properties": {
          "bootstrap.servers": "kafka:29092"
        }
      },
      "policies": {
        "onConnectionInit": {
          "request": [],
          "response": []
        },
        "onProduce": [],
        "onConsume": []
      },
      "channels": {
        "/issues": {
          "policies": {
            "onConnectionInit": {
              "request": [],
              "response": []
            },
            "on_produce": [
              {
                "name": "map-topic",
                "version": "v1",
                "params": {
                  "mode": "produceTo",
                  "topic": "repo-events"
                }
              }
            ],
            "on_consume": [
              {
                "name": "map-topic",
                "version": "v1",
                "params": {
                  "mode": "consumeFrom",
                  "topic": "repo-events"
                }
              }
            ]
          }
        }
      },
      "deploymentState": "deployed"
    }
  }'
```

Verify it was created:

```bash
# List all WebBrokerApis
curl -X GET http://localhost:9090/api/management/v0.9/webbroker-apis \
  -u admin:admin

# Get specific WebBrokerApi
curl -X GET http://localhost:9090/api/management/v0.9/webbroker-apis/websocket-kafka-api-v1-0 \
  -u admin:admin
```

The controller automatically distributes the configuration to the event-gateway via xDS.

**Alternative: Static File Mode**

If you prefer to use static configuration files instead of the control plane:

1. Disable control plane in `docker-compose.yaml`:
   ```yaml
   environment:
     - APIP_EGW_CONTROLPLANE_ENABLED=false
   ```

2. Edit `gateway-runtime/configs/channels.yaml`:
   ```yaml
   channels:
     - kind: WebBrokerApi
       apiId: websocket-kafka-api-v1-0
       name: websocket-kafka-api
       version: v1.0
       context: /ws-kafka
       receiver:
         type: websocket
         properties: {}
       broker-driver:
         type: kafka
         properties:
           topic: repo-events
           bootstrap.servers: kafka:29092
       allChannelPolicies:
         on_connection_init:
           request: []
           response: []
         on_produce: []
         on_consume: []
   ```

3. Restart services:
   ```bash
   docker compose restart event-gateway
   ```

#### 3. Verify Event Gateway is Running

```bash
# Health check
curl http://localhost:9002/health
# → {"status":"UP"}

# Check that xDS distributed the config (should show the WebBrokerApi)
docker compose logs event-gateway | grep "WebBrokerApi"

# Check WebSocket endpoint
curl -I http://localhost:8081/ws-kafka
# → HTTP/1.1 426 Upgrade Required (means WebSocket endpoint is ready)
```

#### 4. Test with WebSocket Client

**Using wscat (CLI tool):**

```bash
# Install wscat
npm install -g wscat

# Connect to the WebSocket endpoint
wscat -c ws://localhost:8081/ws-kafka

# Once connected, type messages and press Enter to send to Kafka
# You should see messages echoed back as they're consumed from Kafka
```

**Using websocat (alternative CLI tool):**

```bash
# Install websocat
brew install websocat  # macOS
# or download from https://github.com/vi/websocat

# Connect and send/receive messages
websocat ws://localhost:8081/ws-kafka
```

**Using Node.js:**

```javascript
// test-websocket.js
const WebSocket = require('ws');

const ws = new WebSocket('ws://localhost:8081/ws-kafka', {
  headers: {
    'X-topic': '/issues'
  }
});

ws.on('open', () => {
  console.log('Connected to Event Gateway');
  
  // Send a message to Kafka
  ws.send(JSON.stringify({
    message: 'Hello Kafka from WebSocket!',
    timestamp: new Date().toISOString()
  }));
});

ws.on('message', (data) => {
  console.log('Received from Kafka:', data.toString());
});

ws.on('close', () => {
  console.log('Disconnected');
});

ws.on('error', (error) => {
  console.error('Error:', error);
});
```

Run it:
```bash
npm install ws
node test-websocket.js
```

**Using Browser Console:**

```javascript
// Note: Browser WebSocket API doesn't support custom headers in constructor
// Use Sec-WebSocket-Protocol or URL query parameters as alternatives
const ws = new WebSocket('ws://localhost:8081/ws-kafka?channel=/issues');

ws.onopen = () => {
  console.log('Connected!');
  ws.send('Hello from browser!');
};

ws.onmessage = (event) => {
  console.log('Received:', event.data);
};
```

#### 5. Monitor Kafka Topics

```bash
# View messages in Kafka UI
open http://localhost:7080

# Or use Kafka CLI
docker exec -it event-gateway-kafka-1 kafka-console-consumer \
  --bootstrap-server localhost:29092 \
  --topic repo-events \
  --from-beginning
```

#### 6. Stop Services

```bash
# Stop all services
docker compose down

# Stop and remove volumes (clean slate)
docker compose down -v
```

### Option 2: Building from Source

For development or when you need to modify the code.

#### 1. Build the Event Gateway Runtime

```bash
cd event-gateway/gateway-runtime

# Build the binary
go build -o event-gateway ./cmd/event-gateway

# Or use the Makefile (builds Docker image)
cd ..
make build-gateway-runtime
```

#### 2. Start Dependencies (Kafka only)

```bash
# Start just Kafka from docker-compose
docker compose up kafka -d

# Wait for Kafka to be ready
docker compose logs -f kafka
```

#### 3. Configure Channels

Create or edit `gateway-runtime/configs/channels.yaml`:

```yaml
channels:
  - kind: WebBrokerApi
    name: websocket-kafka-api
    version: v1.0
    context: /ws-kafka
    receiver:
      name: ws-receiver
      type: websocket
    broker-driver:
      name: kafka-driver
      type: kafka
      properties:
        bootstrap.servers: localhost:9092
    policies:
      on_connection_init:
        request: []
        response: []
      on_produce: []
      on_consume: []
    channels:
      "/issues":
        policies:
          onConnectionInit:
            request: []
            response: []
          on_produce:
            - name: map-topic
              version: v1
              params:
                mode: produceTo
                topic: repo-events
          on_consume:
            - name: map-topic
              version: v1
              params:
                mode: consumeFrom
                topic: repo-events
```

#### 4. Configure Gateway

Edit `gateway-runtime/configs/config.toml`:

```toml
[kafka]
brokers = ["localhost:9092"]
consumer_group_prefix = "egw"

[server]
websocket_port = 8081
websub_enabled = false
websub_tls_enabled = false  # TLS disabled for local dev
admin_port = 9002

[controlplane]
enabled = false  # Run in static mode (set to true to use gateway-controller)

[logging]
level = "info"
```

**Note:** When `controlplane.enabled = false`, the gateway reads configuration from the local `channels.yaml` file. When `enabled = true`, it connects to the gateway-controller at `xds_address` and receives configuration via xDS.

#### 5. Run the Event Gateway

```bash
cd gateway-runtime

# Run with config and channels files
./event-gateway \
  -config configs/config.toml \
  -channels configs/channels.yaml

# Or if you didn't build, run directly with Go
go run ./cmd/event-gateway \
  -config configs/config.toml \
  -channels configs/channels.yaml
```

You should see:
```
INFO Event gateway is ready runtime_id=...
INFO Registered WebBrokerApi binding name=websocket-kafka-api context=/ws-kafka channels=1 topics=[repo-events]
```

#### 6. Test the Connection

Open another terminal and test with websocat:

```bash
websocat --header "X-topic: /issues" ws://localhost:8081/ws-kafka
```

Type messages and press Enter to send them to Kafka.

#### 7. Verify Messages in Kafka

```bash
# View messages being published
docker exec -it event-gateway-kafka-1 kafka-console-consumer \
  --bootstrap-server localhost:29092 \
  --topic repo-events \
  --from-beginning
```

### Option 3: Development Mode with Live Reload

For rapid iteration during development.

#### 1. Install Air (Live Reload Tool)

```bash
go install github.com/cosmtrek/air@latest
```

#### 2. Configure Air

Create `.air.toml` in `gateway-runtime/`:

```toml
root = "."
tmp_dir = "tmp"

[build]
cmd = "go build -o ./tmp/event-gateway ./cmd/event-gateway"
bin = "./tmp/event-gateway -config configs/config.toml -channels configs/channels.yaml"
include_ext = ["go", "toml", "yaml"]
exclude_dir = ["tmp", "vendor"]
delay = 1000
```

#### 3. Run with Live Reload

```bash
cd gateway-runtime
air
```

Now any changes to Go files will automatically rebuild and restart the gateway.

## Testing with Policies

### Example: API Key Authentication

Update your `channels.yaml`:

```yaml
channels:
  - kind: WebBrokerApi
    name: secure-websocket-api
    version: v1.0
    context: /secure-ws
    receiver:
      name: ws-receiver
      type: websocket
    broker-driver:
      name: kafka-driver
      type: kafka
      properties:
        bootstrap.servers: kafka:29092
    policies:
      on_connection_init:
        request:
          - name: api-key-auth
            version: v1
            params:
              in: header
              name: X-API-Key
      on_produce: []
      on_consume: []
    channels:
      "/secure-channel":
        policies:
          onConnectionInit:
            request: []
            response: []
          on_produce:
            - name: map-topic
              version: v1
              params:
                mode: produceTo
                topic: secure-events
          on_consume:
            - name: map-topic
              version: v1
              params:
                mode: consumeFrom
                topic: secure-events
```

Test with authentication:

```bash
# Without API key (should fail)
websocat --header "X-topic: /secure-channel" ws://localhost:8081/secure-ws
# → Connection rejected

# With API key (should succeed)
websocat --header "X-API-Key: your-api-key" --header "X-topic: /secure-channel" ws://localhost:8081/secure-ws
```

### Example: Multiple Channels with Different Topics

```yaml
channels:
  "/issues":
    policies:
      onConnectionInit:
        request: []
        response: []
      on_produce:
        - name: map-topic
          version: v1
          params:
            mode: produceTo
            topic: kafka-repo-issues
      on_consume:
        - name: map-topic
          version: v1
          params:
            mode: consumeFrom
            topic: kafka-repo-issues
  "/commits":
    policies:
      onConnectionInit:
        request: []
        response: []
      on_produce:
        - name: map-topic
          version: v1
          params:
            mode: produceTo
            topic: kafka-repo-commits
      on_consume:
        - name: map-topic
          version: v1
          params:
            mode: consumeFrom
            topic: kafka-repo-commits
```

Connect to different channels:

```javascript
// Connect to /issues channel
const ws1 = new WebSocket('ws://localhost:8081/ws-kafka', {
  headers: { 'X-topic': '/issues' }
});

// Connect to /commits channel
const ws2 = new WebSocket('ws://localhost:8081/ws-kafka', {
  headers: { 'X-topic': '/commits' }
});
```

## End-to-End Testing with Kafka

This section demonstrates how to verify the complete WebSocket ↔ Kafka flow in both directions.

### Prerequisites

- Docker Compose services running (`docker compose up -d`)
- WebBrokerApi created with separate produce and consume topics (recommended for testing)
- `wscat` installed (`npm install -g wscat`)

### Example API with Topic Separation

For clear testing, configure a WebBrokerApi with **separate topics** for produce and consume:

```bash
curl --location 'http://localhost:9090/api/management/v0.9/webbroker-apis' \
--header 'Content-Type: application/json' \
--header 'Authorization: Basic YWRtaW46YWRtaW4=' \
--data '{
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "WebBrokerApi",
    "metadata": { "name": "websocket-kafka-api-v1-0" },
    "spec": {
      "displayName": "websocket-kafka-api",
      "version": "v1.0",
      "context": "/ws-kafka",
      "receiver": {
        "name": "ws-receiver",
        "type": "websocket",
        "properties": {}
      },
      "brokerDriver": {
        "name": "kafka-driver",
        "type": "kafka",
        "properties": {
          "bootstrap.servers": "kafka:29092"
        }
      },
      "policies": {
        "onConnectionInit": { "request": [], "response": [] },
        "onProduce": [],
        "onConsume": []
      },
      "channels": {
        "/issues": {
          "policies": {
            "onConnectionInit": {
              "request": [],
              "response": []
            },
            "on_produce": [
              {
                "name": "map-topic",
                "version": "v1",
                "params": {
                  "mode": "produceTo",
                  "topic": "produce_issues"
                }
              }
            ],
            "on_consume": [
              {
                "name": "map-topic",
                "version": "v1",
                "params": {
                  "mode": "consumeFrom",
                  "topic": "consume_issues"
                }
              }
            ]
          }
        }
      },
      "deploymentState": "deployed"
    }
  }'
```

**Key Points:**
- `/issues` channel publishes to `produce_issues` topic (one-way)
- `/issues` channel consumes from `consume_issues` topic (one-way)
- **No echo/loopback**: Messages sent via WebSocket don't come back to the sender

### Test Setup: 3 Terminals

#### Terminal 1: Kafka Consumer (Monitor WebSocket → Kafka)

This terminal monitors the `produce_issues` topic to verify messages from WebSocket clients arrive in Kafka:

```bash
docker exec -it event-gateway-kafka-1 /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic produce_issues \
  --from-beginning
```

**Expected Output:** All messages sent via WebSocket will appear here.

#### Terminal 2: WebSocket Client

Connect to the `/issues` channel:

```bash
wscat -c ws://localhost:8081/ws-kafka --header "X-topic: /issues"
```

**Expected Output:**
```
Connected (press CTRL+C to quit)
> 
```

#### Terminal 3: Kafka Producer (Send to WebSocket Clients)

This terminal publishes to the `consume_issues` topic, which will be delivered to WebSocket clients:

```bash
docker exec -it event-gateway-kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server localhost:9092 \
  --topic consume_issues
```

**Expected Output:**
```
>
```

### Test 1: WebSocket → Kafka (Produce Path)

**Objective:** Verify messages sent from WebSocket client arrive in Kafka.

**Steps:**

1. In **Terminal 2** (WebSocket client), type a message and press Enter:
   ```
   > {"message": "Hello from WebSocket", "timestamp": "2026-05-11T12:00:00Z"}
   ```

2. In **Terminal 1** (Kafka consumer), verify the message appears:
   ```
   {"message": "Hello from WebSocket", "timestamp": "2026-05-11T12:00:00Z"}
   ```

3. Send multiple messages to test throughput:
   ```
   > {"event": "issue_created", "id": 1}
   > {"event": "issue_updated", "id": 1, "status": "in-progress"}
   > {"event": "issue_closed", "id": 1}
   ```

4. Verify all messages appear in **Terminal 1** in order.

**✅ Success Criteria:**
- All messages from WebSocket appear in the `produce_issues` Kafka topic
- Messages maintain order
- No messages echo back to WebSocket client (Terminal 2 shows only `>` prompt)

### Test 2: Kafka → WebSocket (Consume Path)

**Objective:** Verify messages published to Kafka are delivered to WebSocket clients.

**Steps:**

1. In **Terminal 3** (Kafka producer), type a message and press Enter:
   ```
   > {"message": "Hello from Kafka", "source": "external-service"}
   ```

2. In **Terminal 2** (WebSocket client), verify the message is received:
   ```
   < {"message": "Hello from Kafka", "source": "external-service"}
   ```

3. Send multiple messages from Kafka:
   ```
   > {"event": "notification", "type": "email", "status": "sent"}
   > {"event": "notification", "type": "sms", "status": "delivered"}
   ```

4. Verify all messages appear in **Terminal 2** immediately.

**✅ Success Criteria:**
- All messages from `consume_issues` Kafka topic appear in WebSocket client
- Messages are delivered in real-time (< 1 second delay)
- Messages maintain order

### Test 3: Multiple WebSocket Connections

**Objective:** Verify each connection receives messages independently.

**Steps:**

1. Open **Terminal 4** with a second WebSocket client:
   ```bash
   wscat -c ws://localhost:8081/ws-kafka --header "X-topic: /issues"
   ```

2. In **Terminal 3** (Kafka producer), send a message:
   ```
   > {"broadcast": "message to all clients"}
   ```

3. Verify the message appears in **both Terminal 2 and Terminal 4**.

4. In **Terminal 2**, send a message:
   ```
   > {"from": "client1"}
   ```

5. Verify in **Terminal 1** (Kafka consumer) that the message appears.

6. Verify in **Terminal 2 and Terminal 4** that the message does NOT echo back.

**✅ Success Criteria:**
- Each WebSocket connection has its own unique consumer group
- All connections receive broadcast messages from Kafka
- Messages sent by one client don't echo to any WebSocket client
- Each connection can produce independently

### Test 4: Verify Topic Isolation

**Objective:** Confirm produce and consume topics are completely isolated.

**Steps:**

1. List Kafka topics to verify both exist:
   ```bash
   docker exec -it event-gateway-kafka-1 /opt/kafka/bin/kafka-topics.sh \
     --bootstrap-server localhost:9092 \
     --list | grep issues
   ```
   
   **Expected Output:**
   ```
   consume_issues
   produce_issues
   ```

2. Check message count in `produce_issues`:
   ```bash
   docker exec -it event-gateway-kafka-1 /opt/kafka/bin/kafka-run-class.sh \
     kafka.tools.GetOffsetShell \
     --broker-list localhost:9092 \
     --topic produce_issues
   ```

3. Check message count in `consume_issues`:
   ```bash
   docker exec -it event-gateway-kafka-1 /opt/kafka/bin/kafka-run-class.sh \
     kafka.tools.GetOffsetShell \
     --broker-list localhost:9092 \
     --topic consume_issues
   ```

4. Verify counts match your test messages (produce_issues has WebSocket messages, consume_issues has Kafka messages).

**✅ Success Criteria:**
- Topics are independent with different message counts
- No cross-contamination between produce and consume paths

### Debugging Tips

**Check Event Gateway Logs:**
```bash
docker compose logs -f event-gateway | grep -E "Map Topic|Publishing|received from WebSocket"
```

**Verify Channel Configuration:**
```bash
docker compose logs event-gateway | grep "Built policy chains for WebBrokerApi channel"
```

**Expected Log Output:**
```
level=INFO msg="Built policy chains for WebBrokerApi channel" api=websocket-kafka-api-v1-0 channel=/issues topics="[produce_issues consume_issues]"
```

**Check WebSocket Connection:**
```bash
docker compose logs event-gateway | grep "WebSocket handshake completed"
```

**Expected Log Output:**
```
level=INFO msg="[4] WebSocket handshake completed" connID=xxx api=websocket-kafka-api-v1-0 channel=/issues topics=[consume_issues]
```

Note the `topics=[consume_issues]` - this confirms the consumer only subscribes to the consume topic, not the produce topic.

## Troubleshooting

**WebSocket endpoint not available (Empty reply from server):**
- In control plane mode, verify the WebBrokerApi was created via the controller:
  ```bash
  curl -X GET http://localhost:9090/api/management/v0.9/webbroker-apis -u admin:admin
  ```
- Check event-gateway logs for xDS configuration:
  ```bash
  docker compose logs event-gateway | grep -E "EventChannelConfig|WebBrokerApi"
  ```
- Verify control plane is connected:
  ```bash
  docker compose logs event-gateway | grep "Connected to xDS"
  ```
- In static file mode, verify `channels.yaml` has the WebBrokerApi entry and `controlplane.enabled = false`

**Connection rejected during handshake:**
- **Missing X-topic header**: Verify the `X-topic` header is included with a valid channel name (e.g., `X-topic: /issues`)
- Check `on_connection_init.request` policies (e.g., API key validation)
- Verify headers are correctly set in upgrade request
- Check logs for: "Missing X-topic header" or "Unknown channel in X-topic header"

**Messages not reaching Kafka:**
- Check `on_produce` policies for short-circuit conditions
- Verify topic mapping in policies
- Check Kafka broker connectivity

**Messages not reaching client:**
- Check `on_consume` policies for short-circuit conditions
- Verify WebSocket connection is still open
- Check outbound channel buffer (may be full)

**High memory usage:**
- Too many concurrent connections
- Adjust buffer sizes in receiver config
- Consider implementing connection limits

**Connection timeout during Kafka operations:**
- Verify Kafka is running: `docker compose ps kafka`
- Check Kafka logs: `docker compose logs kafka`
- Verify bootstrap servers address (use `kafka:29092` in Docker, `localhost:9092` on host)

**WebSocket connection closes immediately:**
- Check event gateway logs: `docker compose logs event-gateway`
- Verify the context path matches your channels.yaml config
- Check for policy errors during connection init

**Gateway container exits with "config.toml is a directory" error:**
- Ensure `configs/gateway-controller/config.toml` is a file, not a directory
- If you accidentally created it as a directory: `rm -rf configs/gateway-controller/config.toml` and recreate as a file
- Restart services: `docker compose up -d`

**Event gateway exits with "TLS certificate file does not exist":**
- TLS is enabled but certificates are missing in `listener-certs/` directory
- **Solution 1 (Recommended for local dev):** Disable TLS by setting `websub_tls_enabled = false` in `gateway-runtime/configs/config.toml`
- **Solution 2:** Generate self-signed certificates:
  ```bash
  cd listener-certs/
  openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
    -keyout default-listener.key \
    -out default-listener.crt \
    -subj "/CN=localhost"
  ```
- After fixing, restart: `docker compose restart event-gateway`

## Quick Reference

### Common Commands

```bash
# Start services
docker compose up -d

# Stop services
docker compose down

# View logs
docker compose logs -f event-gateway

# Restart event gateway only
docker compose restart event-gateway

# Rebuild after code changes
make build-gateway-runtime
docker compose up -d --build event-gateway

# WebBrokerApi Management (Control Plane Mode)
# Create WebBrokerApi
curl -X POST http://localhost:9090/api/management/v0.9/webbroker-apis \
  -u admin:admin \
  -H "Content-Type: application/json" \
  -d @webbroker-config.json

# List WebBrokerApis
curl -X GET http://localhost:9090/api/management/v0.9/webbroker-apis \
  -u admin:admin

# Get WebBrokerApi by ID
curl -X GET http://localhost:9090/api/management/v0.9/webbroker-apis/websocket-kafka-api-v1-0 \
  -u admin:admin

# Delete WebBrokerApi
curl -X DELETE http://localhost:9090/api/management/v0.9/webbroker-apis/websocket-kafka-api-v1-0 \
  -u admin:admin

# Kafka Management
# List Kafka topics
docker exec event-gateway-kafka-1 kafka-topics \
  --bootstrap-server localhost:29092 --list

# Create a new Kafka topic
docker exec event-gateway-kafka-1 kafka-topics \
  --bootstrap-server localhost:29092 \
  --create --topic my-topic \
  --partitions 3 --replication-factor 1

# Consume from topic
docker exec event-gateway-kafka-1 kafka-console-consumer \
  --bootstrap-server localhost:29092 \
  --topic repo-events \
  --from-beginning

# Produce to topic
docker exec -it event-gateway-kafka-1 kafka-console-producer \
  --bootstrap-server localhost:29092 \
  --topic repo-events
```

### Configuration Reference

**Event Gateway Environment Variables:**

| Variable | Description | Default |
|----------|-------------|---------|
| `APIP_EGW_KAFKA_BROKERS` | Kafka broker addresses | `kafka:29092` |
| `APIP_EGW_SERVER_WEBSOCKET_PORT` | WebSocket server port | `8081` |
| `APIP_EGW_SERVER_ADMIN_PORT` | Admin API port | `9002` |
| `APIP_EGW_LOGGING_LEVEL` | Log level (debug/info/warn/error) | `info` |
| `APIP_EGW_CONTROLPLANE_ENABLED` | Enable xDS control plane | `true` |
| `APIP_EGW_CONTROLPLANE_XDS_ADDRESS` | xDS server address | `gateway-controller:18001` |

**Config File Locations:**

- Gateway config: `gateway-runtime/configs/config.toml`
- Channels config: `gateway-runtime/configs/channels.yaml`
- Controller config: `configs/gateway-controller/config.toml`

### Sample WebBrokerApi Configurations

**Minimal Config (No Policies):**

```yaml
channels:
  - kind: WebBrokerApi
    name: simple-ws
    version: v1.0
    context: /simple
    receiver:
      name: ws-receiver
      type: websocket
    broker-driver:
      name: kafka-driver
      type: kafka
      properties:
        bootstrap.servers: kafka:29092
    policies:
      on_connection_init:
        request: []
        response: []
      on_produce: []
      on_consume: []
    channels:
      "/default":
        on_produce:
          - name: map-topic
            version: v1
            params:
              mode: produceTo
              topic: simple-events
        on_consume:
          - name: map-topic
            version: v1
            params:
              mode: consumeFrom
              topic: simple-events
```

**With Authentication:**

```yaml
channels:
  - kind: WebBrokerApi
    name: secure-ws
    version: v1.0
    context: /secure
    receiver:
      name: ws-receiver
      type: websocket
    broker-driver:
      name: kafka-driver
      type: kafka
      properties:
        bootstrap.servers: kafka:29092
    policies:
      on_connection_init:
        request:
          - name: api-key-auth
            version: v1
            params:
              in: header
              name: X-API-Key
      on_produce: []
      on_consume: []
    channels:
      "/secure-channel":
        on_produce:
          - name: map-topic
            version: v1
            params:
              mode: produceTo
              topic: secure-events
        on_consume:
          - name: map-topic
            version: v1
            params:
              mode: consumeFrom
              topic: secure-events
```

**With Multiple Channels:**

```yaml
channels:
  - kind: WebBrokerApi
    name: smart-ws
    version: v1.0
    context: /smart
    receiver:
      name: ws-receiver
      type: websocket
    broker-driver:
      name: kafka-driver
      type: kafka
      properties:
        bootstrap.servers: kafka:29092
    policies:
      on_connection_init:
        request:
          - name: api-key-auth
            version: v1
      on_produce: []
      on_consume:
        - name: set-headers
          version: v1
          params:
            headers:
              X-Gateway: event-gateway
              X-Timestamp: "${timestamp}"
    channels:
      "/users":
        on_produce:
          - name: map-topic
            version: v1
            params:
              mode: produceTo
              topic: users-topic
        on_consume:
          - name: map-topic
            version: v1
            params:
              mode: consumeFrom
              topic: users-topic
      "/orders":
        on_produce:
          - name: map-topic
            version: v1
            params:
              mode: produceTo
              topic: orders-topic
        on_consume:
          - name: map-topic
            version: v1
            params:
              mode: consumeFrom
              topic: orders-topic
```

### Useful Tools

**websocat** - WebSocket CLI client:
```bash
# Install
brew install websocat  # macOS
cargo install websocat  # Linux/Rust

# Basic usage
websocat --header "X-topic: /channel-name" ws://localhost:8081/ws-kafka

# With headers
websocat --header "X-API-Key: test" --header "X-topic: /secure-channel" ws://localhost:8081/secure

# With text protocol
websocat -t --header "X-topic: /channel-name" ws://localhost:8081/ws-kafka
```

**wscat** - Alternative WebSocket CLI:
```bash
# Install
npm install -g wscat

# Connect
wscat -c ws://localhost:8081/ws-kafka -H "X-topic: /channel-name"

# With headers
wscat -c ws://localhost:8081/secure -H "X-API-Key: test" -H "X-topic: /secure-channel"
```

**kcat (kafkacat)** - Kafka CLI tool:
```bash
# Install
brew install kcat  # macOS
apt install kafkacat  # Linux

# Consume
kcat -b localhost:9092 -t repo-events -C

# Produce
echo "test message" | kcat -b localhost:9092 -t repo-events -P
```

## Current Spec for WebBroker APIs

```json
{
    "apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
    "kind": "WebBrokerApi",
    "metadata": {
        "name": "websocket-kafka-api-v1-0"
    },
    "spec": {
        "displayName": "websocket-kafka-api",
        "version": "v1.0",
        "context": "/websocket-kafka",

        "receiver": {
            "name": "my-websocket-receiver",
            "type": "websocket",
            "properties": {}
        },

        "brokerDriver": {
            "name": "my-kafka-broker",
            "type": "kafka",
            "properties": {
                "bootstrap.servers": "localhost:9092",
                "security.protocol": "PLAINTEXT"
            }
        },

        "policies": {
            "onConnectionInit": {
                "request": [
                    {
                        "name": "api-key-auth",
                        "version": "v1",
                        "params": {
                            "in": "header",
                            "name": "X-API-Key"
                        }
                    }
                ],
                "response": []
            },
            "onProduce": [],
            "onConsume": []
        },

        "channels": {
            "/issues": {
                "on_connection_init": {
                    "request": [],
                    "response": []
                },
                "on_produce": [
                    {
                        "name": "map-topic",
                        "version": "v1",
                        "params": {
                            "mode": "produceTo",
                            "topic": "kafka-repo-issues"
                        }
                    }
                ],
                "on_consume": [
                    {
                        "name": "map-topic",
                        "version": "v1",
                        "params": {
                            "mode": "consumeFrom",
                            "topic": "kafka-repo-issues"
                        }
                    }
                ]
            },
            "/commits": {
                "on_produce": [
                    {
                        "name": "map-topic",
                        "version": "v1",
                        "params": {
                            "mode": "produceTo",
                            "topic": "kafka-repo-commits"
                        }
                    }
                ],
                "on_consume": [
                    {
                        "name": "map-topic",
                        "version": "v1",
                        "params": {
                            "mode": "consumeFrom",
                            "topic": "kafka-repo-commits"
                        }
                    }
                ]
            }
        },

        "deploymentState": "deployed"
    }
}
```

### Key Changes in New Spec

1. **Receiver and BrokerDriver Names**: Added `name` field to both `receiver` and `brokerDriver` for instance identification
2. **API-Level Policies**: Renamed `allChannelPolicies` → `policies` for API-level policy enforcement
3. **Channel-Specific Configs**: New `channels` map where each key is a channel name (e.g., `/issues`, `/commits`)
4. **Per-Channel Policies**: Each channel has its own `on_connection_init`, `on_produce`, and `on_consume` policies
5. **Topic Extraction**: Topics no longer in `brokerDriver.properties.topic`; instead extracted from `map-topic` policies with `mode: produceTo` and `mode: consumeFrom`
6. **Channel Routing**: Client selects channel via `X-topic` header during WebSocket handshake
7. **Policy Cascade**: API-level policies execute first, then channel-specific policies

## Next Steps

- Review [WebSub documentation](README.md) for comparison
- Explore [policy development](../gateway/README.md) for custom policies
- Check [performance tuning guide](docs/performance.md) for optimization
- Learn about [monitoring and observability](docs/observability.md)
