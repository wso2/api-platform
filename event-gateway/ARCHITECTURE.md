# Event Gateway Architecture: WebSubApi & WebBrokerApi

This document provides a comprehensive architectural overview of how **WebSubApi** and **WebBrokerApi** are implemented and fit into the Event Gateway.

## Architecture Overview

Both API types follow the same **Receiver → Policy Engine → Broker Driver** architecture with protocol-specific implementations.

---

## 1. Spec Submission → Storage (Controller)

### Entry Point: REST API
```
POST /websub-apis    → CreateWebSubAPI()
POST /webbroker-apis → CreateWebBrokerApi()
```

### Flow
1. **Handler** receives YAML/JSON spec via HTTP
2. **DeploymentService.DeployAPIConfiguration()** processes it:
   - Parses spec into typed structs (`WebSubAPI` or `WebBrokerApi`)
   - Validates structure
   - Stores in **SQLite** as `StoredConfig`
   - Returns UUID and deployment status

### Storage Schema
```go
StoredConfig {
    UUID: string              // Unique ID
    Kind: "WebSubApi" | "WebBrokerApi"
    DisplayName: string
    Configuration: interface{} // Typed as WebSubAPI or WebBrokerApi
    DesiredState: "deployed" | "undeployed"
    SourceConfiguration: []byte
}
```

**Implementation Files:**
- `gateway/gateway-controller/pkg/api/handlers/websub_api_handler.go`
- `gateway/gateway-controller/pkg/api/handlers/webbroker_api_handler.go`
- `gateway/gateway-controller/pkg/api/management/generated.go` (WebSubAPI types)
- `gateway/gateway-controller/pkg/api/management/webbroker_types.go` (WebBrokerApi types)

---

## 2. xDS Translation (Controller)

### Translator Service
**File:** `gateway/gateway-controller/pkg/policyxds/event_channel_translator.go`

### Functions
- `TranslateWebSubApisToEventChannelConfigs()` 
- `TranslateWebBrokerApisToEventChannelConfigs()`

### Process
1. Fetches all `StoredConfig` entries by Kind
2. Filters for `DesiredState == "deployed"`
3. Converts each into **EventChannelConfig** xDS resource:
   ```json
   {
     "uuid": "...",
     "name": "my-api",
     "kind": "WebSubApi" | "WebBrokerApi",
     "context": "/api/v1",
     "version": "1.0.0",
     "channels": [...],      // Structure differs by Kind
     "policies": {...},      // Structure differs by Kind
     "receiver": {...},      // Only for WebBrokerApi
     "broker-driver": {...}  // Only for WebBrokerApi
   }
   ```

### WebSubApi xDS Structure
```json
{
  "kind": "WebSubApi",
  "channels": [
    {
      "name": "issues",
      "policies": {
        "subscribe": [],
        "inbound": [],
        "outbound": []
      }
    }
  ],
  "policies": {
    "subscribe": [],  // Hub-level auth
    "inbound": [],    // Webhook receiver validation
    "outbound": []    // Delivery transformation/signing
  },
  "receiver": {
    "type": "websub"
  }
}
```

### WebBrokerApi xDS Structure
```json
{
  "kind": "WebBrokerApi",
  "channels": {
    "/issues": {
      "policies": {
        "onConnectionInit": {
          "request": [],
          "response": []
        },
        "on_produce": [],
        "on_consume": []
      }
    }
  },
  "policies": {
    "on_connection_init": {
      "request": [],
      "response": []
    },
    "on_produce": [],
    "on_consume": []
  },
  "receiver": {
    "name": "ws-receiver",
    "type": "websocket"
  },
  "broker-driver": {
    "name": "kafka-driver",
    "type": "kafka",
    "properties": {
      "bootstrap.servers": "kafka:9092"
    }
  }
}
```

4. xDS Server pushes to runtime via gRPC stream

---

## 3. Runtime Processing (Event Gateway)

### xDS Reception
**File:** `event-gateway/gateway-runtime/internal/xdsclient/handler.go`

### Handler.HandleResources()
1. Receives EventChannelConfig from xDS stream
2. Deserializes JSON payload into `EventChannelResource`
3. Diffs against previous state (add/remove/update)
4. Calls `addBinding()` or `removeBinding()`

### Binding Conversion

#### WebSubApi
```go
// In handler.go: toWebSubApiBinding()
WebSubApiBinding {
    Kind: "WebSubApi"
    Name: string
    Context: string
    Version: string
    Channels: map[string]string  // channel-name → Kafka topic
    BrokerDriver: BrokerDriverSpec{Type: "kafka", Config: {...}}
    Receiver: ReceiverSpec{Type: "websub"}
    Policies: {Subscribe: [], Inbound: [], Outbound: []}
    ChannelPolicies: map[string]{Subscribe: []}  // Per-channel policies
}
```

**Implementation File:** `event-gateway/gateway-runtime/internal/binding/types.go`

#### WebBrokerApi
```go
// In handler.go: toWebBrokerApiBinding()
WebBrokerApiBinding {
    Kind: "WebBrokerApi"
    Name: string
    Context: string  // WebSocket path
    Receiver: ReceiverSpec{Name: "ws-receiver", Type: "websocket"}
    BrokerDriver: BrokerDriverSpec{Name: "kafka-driver", Type: "kafka", ...}
    Policies: {
        OnConnectionInit: {Request: [], Response: []},
        OnProduce: [],
        OnConsume: []
    }
    Channels: map[string]WebBrokerChannelDef{
        "/issues": {
            OnConnectionInit: {...},
            OnProduce: [],
            OnConsume: []
        }
    }
}
```

**Implementation File:** `event-gateway/gateway-runtime/internal/binding/types.go`

---

## 4. Component Creation (Runtime)

### Runtime.AddWebSubApiBinding()

**File:** `event-gateway/gateway-runtime/internal/runtime/runtime.go`

```
┌─────────────────────────────────────────────┐
│ 1. Build Policy Chains                      │
│    - Subscribe chain (hub-level + per-chan) │
│    - Inbound chain (webhook validation)     │
│    - Outbound chain (delivery transform)    │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│ 2. Create Broker Driver (Kafka)             │
│    - Topics: one per channel + sync topic   │
│    - Producer for webhook ingress           │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│ 3. Create WebSub Receiver                   │
│    - HTTP handlers: /hub, /webhook-receiver │
│    - Subscription store (in-memory)         │
│    - Consumer manager (per-callback)        │
│    - Deliverer (async retry logic)          │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│ 4. Register with Hub                        │
│    - ChannelBinding with chain keys         │
│    - Channel → Kafka topic mapping          │
└─────────────────────────────────────────────┘
```

**Implementation:** `event-gateway/gateway-runtime/internal/connectors/receiver/websub/connector.go`

### Runtime.AddWebBrokerApiBinding()

**File:** `event-gateway/gateway-runtime/internal/runtime/runtime.go`

```
┌─────────────────────────────────────────────┐
│ 1. Build Policy Chains                      │
│    API-level:                               │
│    - onConnectionInit (request + response)  │
│    - onProduce                              │
│    - onConsume                              │
│    Per-channel:                             │
│    - /issues: onProduce + onConsume chains  │
│    - /commits: onProduce + onConsume chains │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│ 2. Extract Topics & Create Broker Driver    │
│    - Parse map-topic policies               │
│    - Extract produceTo + consumeFrom topics │
│    - Create Kafka driver with all topics    │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│ 3. Create WebSocket Receiver                │
│    - HTTP handler on context path           │
│    - Metadata: channelNames, chainKeys      │
│    - Upgrade logic with X-channel validation  │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│ 4. Register with Hub                        │
│    - ChannelChainKeys per channel           │
│    - Enables ProcessByChainKey()            │
└─────────────────────────────────────────────┘
```

**Implementation:** `event-gateway/gateway-runtime/internal/connectors/receiver/websocket/broker_api_connector.go`

---

## 5. Data Flow Architecture

### WebSubApi Flow (Publisher → Subscribers)

```
┌──────────────┐
│   Publisher  │ (HTTP POST webhook)
└──────┬───────┘
       │
       ▼
┌──────────────────────────────────────────┐
│ WebSubReceiver                           │
│ /api/v1/webhook-receiver                 │
└──────┬───────────────────────────────────┘
       │ connectors.Message
       ▼
┌──────────────────────────────────────────┐
│ Hub.ProcessInbound()                     │
│ - Executes inbound policy chain          │
│ - Validates/transforms webhook payload   │
└──────┬───────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────┐
│ KafkaBrokerDriver.Publish()              │
│ - Writes to topic: api-v1_issues         │
└──────┬───────────────────────────────────┘
       │
       ▼
┌──────────────────────────────────────────┐
│ ConsumerManager (per-callback consumer)  │
│ - One consumer per active subscription   │
└──────┬───────────────────────────────────┘
       │ connectors.Message
       ▼
┌──────────────────────────────────────────┐
│ Hub.ProcessOutbound()                    │
│ - Executes outbound policy chain         │
│ - Signs/transforms delivery payload      │
└──────┬───────────────────────────────────┘
       │
       ▼
┌──────────────┐
│ Subscriber   │ (HTTP POST to callback URL)
└──────────────┘
```

**Key Components:**
- **WebSubReceiver:** HTTP endpoint for data ingress and subscription management
- **ConsumerManager:** Creates dedicated Kafka consumer for each active subscription
- **Deliverer:** Handles async delivery with exponential backoff retry

### WebBrokerApi Flow (Bidirectional WebSocket ↔ Kafka)

**Note:** This diagram shows WebSocket + Kafka as an example. The architecture supports any receiver type (eg: SSE) with any broker driver (eg: MQTT, RabbitMQ) through the plugin system (see [Extensibility](#12-extensibility--plugin-architecture)).

```
┌───────────────┐
│ WebSocket     │
│ Client        │
└───────┬───────┘
        │ ws://gateway/api?X-channel=/issues
        ▼
┌──────────────────────────────────────────┐
│ WebBrokerApiReceiver.handleUpgrade()     │
│ 1. Validate X-channel header               │
│ 2. Apply onConnectionInit.request        │
│ 3. Upgrade to WebSocket                  │
│ 4. Apply onConnectionInit.response       │
└──────┬───────────────────────────────────┘
        │
        ├─────────── Inbound (client → broker) ──────────┐
        │                                                 │
        ▼                                                 │
┌──────────────────────────────────────────┐             │
│ brokerApiConnection.inboundLoop()        │             │
│ - Read WebSocket frames                  │             │
└──────┬───────────────────────────────────┘             │
        │ connectors.Message                             │
        ▼                                                 │
┌──────────────────────────────────────────┐             │
│ Hub.ProcessByChainKey(produceChainKey)   │             │
│ - API-level on_produce                   │             │
│ - Channel-level on_produce (/issues)     │             │
│ - map-topic policy sets kafka.topic      │             │
└──────┬───────────────────────────────────┘             │
        │                                                 │
        ▼                                                 │
┌──────────────────────────────────────────┐             │
│ KafkaBrokerDriver.Publish()              │             │
│ - Writes to: produce_issues              │             │
└──────────────────────────────────────────┘             │
                                                          │
        ├─────────── Outbound (broker → client) ─────────┘
        │
        ▼
┌──────────────────────────────────────────┐
│ Kafka Consumer (unique group per conn)   │
│ - Reads from: consume_issues             │
└──────┬───────────────────────────────────┘
        │ connectors.Message
        ▼
┌──────────────────────────────────────────┐
│ Hub.ProcessByChainKey(consumeChainKey)   │
│ - API-level on_consume                   │
│ - Channel-level on_consume (/issues)     │
└──────┬───────────────────────────────────┘
        │
        ▼
┌──────────────────────────────────────────┐
│ brokerApiConnection.outboundLoop()       │
│ - Write WebSocket frames                 │
└──────────────────────────────────────────┘
```

**Key Components:**
- **brokerApiConnection:** Per-connection state with dedicated Kafka consumer/producer
- **Consumer Group:** `{prefix}-ws-{uuid}` ensures per-connection isolation
- **Bidirectional Channels:** Inbound (client→Kafka) and Outbound (Kafka→client) Go channels

---

## 6. Policy Engine Integration

### Policy Chain Structure
Both API types use the same policy engine with different enforcement points.

**Key Insight: Policies are fully interchangeable between WebSubApi and WebBrokerApi.** The same policy (e.g., `api-key-auth`, `transform-payload-case`, `map-topic`) can be used in either API type. The policy engine treats all policies identically regardless of which API type invokes them.

#### What Makes Policies Interchangeable?

1. **Unified Policy Engine:** Both API types use the same `engine.Engine` instance
2. **Same Execution Method:** Both call `engine.ExecuteRequestHeaderPolicies()` and `engine.ExecuteRequestBodyPolicies()`
3. **Common Message Format:** Both work with `connectors.Message` structure
4. **No API-Type Restrictions:** Policy definitions have no `applyTo` or API-type constraints
5. **Same Results Processing:** Both process `RequestHeaderResult` and `RequestBodyResult` identically

#### The Only Differences:

| Aspect | WebSubApi | WebBrokerApi |
|--------|-----------|--------------|
| **Enforcement Point Names** | subscribe, inbound, outbound | onConnectionInit, onProduce, onConsume |
| **Chain Key Format** | `{api}-subscribe`, `{api}-inbound`, `{api}-outbound` | `{api}-on_connection_init_req`, `{api}-on_produce`, etc. |
| **When Applied** | Subscription requests, webhook ingress, callback delivery | WebSocket upgrade, client→broker, broker→client |
| **Hub Method** | `ProcessSubscribe()`, `ProcessInbound()`, `ProcessOutbound()` | `ProcessByChainKey()` with specific chain keys |

**Example: The Same Policy in Both API Types**

```yaml
# In WebSubApi - validating inbound webhook data
spec:
  receiver:
    policies:
      - name: json-schema-validator
        version: v1
        params:
          schema: {...}

# In WebBrokerApi - validating inbound client messages  
spec:
  policies:
    on_produce:
      - name: json-schema-validator
        version: v1
        params:
          schema: {...}
```

Both use the **exact same policy** (`json-schema-validator`), just at different enforcement points!

#### WebSubApi Policy Enforcement Points

| Phase | Purpose | When Applied |
|-------|---------|--------------|
| **Subscribe** | Authentication/authorization | During subscription request (POST /hub) |
| **Inbound** | Validate/transform incoming data | When publisher sends webhook data |
| **Outbound** | Sign/transform outgoing delivery | Before delivering to subscriber callback |

**Implementation:** `event-gateway/gateway-runtime/internal/hub/hub.go`
- `Hub.ProcessSubscribe()` - Subscribe phase
- `Hub.ProcessInbound()` - Inbound phase  
- `Hub.ProcessOutbound()` - Outbound phase

#### WebBrokerApi Policy Enforcement Points

| Phase | Purpose | When Applied |
|-------|---------|--------------|
| **onConnectionInit (request)** | Authenticate upgrade request | Before WebSocket upgrade |
| **onConnectionInit (response)** | Modify upgrade response | After successful upgrade |
| **onProduce** | Validate/route client messages | Client → Kafka direction |
| **onConsume** | Transform broker messages | Kafka → Client direction |

**Implementation:** `event-gateway/gateway-runtime/internal/hub/hub.go`
- `Hub.ProcessByChainKey()` - All phases using specific chain keys

### Hub Orchestration

The Hub acts as the central message router and policy orchestrator:

```go
// WebSubApi
msg → hub.engine.ExecuteRequestPolicies(inboundChain) → result

// WebBrokerApi
msg → hub.engine.ExecuteRequestPolicies(chainKey) → result
```

**Chain Building:** Policies are compiled into chains during binding registration:
- API-level policies apply to all channels
- Channel-level policies apply only to specific channels
- Chains are stored with unique keys in the policy engine

---

## 7. Key Differences

| Aspect | WebSubApi | WebBrokerApi |
|--------|-----------|--------------|
| **Protocol** | HTTP Webhooks (WebSub standard) | Pluggable (WebSocket, SSE, etc.) |
| **Broker** | Pluggable (Kafka default) | Pluggable (Kafka, MQTT, RabbitMQ, etc.) |
| **Direction** | Unidirectional (pub → sub) | Bidirectional (client ↔ broker) |
| **Connection Model** | Request/response per webhook | Persistent connection |
| **Subscription Model** | Hub manages subscriptions | Per-connection isolation |
| **Kafka Consumer** | One per callback URL | One per connection (for Kafka) |
| **Consumer Group** | `{api}-{callback-hash}` | `{prefix}-{protocol}-{uuid}` |
| **Topics** | One per channel + sync topic | Separate produce/consume topics per channel |
| **Receiver Paths** | `/hub`, `/webhook-receiver` | Configurable via `context` field |
| **Policy Points** | 3 (subscribe, inbound, outbound) | 3 (onConnectionInit, onProduce, onConsume) |
| **State Management** | Subscription store + Kafka sync topic | Per-connection state only |
| **Delivery** | Async with retry (exponential backoff) | Synchronous over connection |
| **Use Case** | Event distribution to webhooks | Protocol mediation / real-time streaming |
| **Extensibility** | Fixed to WebSub protocol | Any streaming protocol via receiver plugins |

**Note:** While WebSubApi is specialized for the WebSub protocol, WebBrokerApi is designed for **protocol mediation** between any streaming protocol and any message broker through its plugin architecture.

---

## 8. Component Lifecycle

### Deployment Flow
```
Controller:
  1. Spec submitted via REST API
  2. Stored in SQLite (StoredConfig)
  3. Translated to EventChannelConfig xDS
  4. Pushed to runtime via gRPC stream

Runtime:
  1. xDS received and deserialized
  2. Binding created (WebSubApiBinding or WebBrokerApiBinding)
  3. Components instantiated:
     - Policy chains built
     - Broker driver created
     - Receiver created
  4. Registered with Hub
  5. HTTP handlers activated
```

### Teardown Flow
```
Controller:
  1. API deleted or undeployed
  2. xDS deletion marker sent to runtime

Runtime:
  1. xDS delete received
  2. Hub deregisters binding
  3. Receiver stops (closes connections)
  4. Broker driver closes
  5. Kafka consumers cleanup
  6. HTTP handlers removed
```

### Dynamic Updates
The xDS-based architecture enables zero-downtime updates:
- New configuration → new binding created
- Old binding continues serving existing connections
- New connections use new binding
- Old binding removed when connections drain

---

## 9. Configuration Examples

### WebSubApi Configuration

```yaml
apiVersion: wso2.com/v1
kind: WebSubApi
metadata:
  name: github-events
spec:
  context: /github/webhooks
  version: v1
  hub:
    policies:
      - name: api-key-auth
        version: v1
        params:
          in: header
          name: X-API-Key
    channels:
      - name: issues
        policies:
          - name: json-schema-validator
            version: v1
            params:
              schema: {...}
      - name: pull_requests
  receiver:
    policies:
      - name: hmac-signature-validator
        version: v1
        params:
          algorithm: sha256
          header: X-Hub-Signature-256
  delivery:
    policies:
      - name: hmac-signature-signer
        version: v1
        params:
          algorithm: sha256
          header: X-Hub-Signature
```

**Resulting Topics:**
- `github-webhooks-v1_issues` - Issue events
- `github-webhooks-v1_pull_requests` - PR events
- `_sync_github-webhooks-v1` - Subscription state sync

### WebBrokerApi Configuration

```yaml
apiVersion: wso2.com/v1
kind: WebBrokerApi
metadata:
  name: realtime-events
spec:
  context: /ws/events
  version: v1
  receiver:
    name: ws-receiver
    type: websocket
  broker-driver:
    name: kafka-driver
    type: kafka
    properties:
      bootstrap.servers: kafka:9092
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
              topic: produce_issues
        on_consume:
          - name: map-topic
            version: v1
            params:
              mode: consumeFrom
              topic: consume_issues
```

**Resulting Topics:**
- `produce_issues` - Client writes
- `consume_issues` - Client reads

**WebSocket Connection:**
```bash
websocat --header "X-API-Key: secret" --header "X-channel: /issues" ws://gateway:8081/ws/events/v1
```

---

## 10. Implementation File Reference

### Controller (Gateway Controller)

| Component | File Path |
|-----------|-----------|
| WebSubApi Types | `gateway/gateway-controller/pkg/api/management/generated.go` |
| WebBrokerApi Types | `gateway/gateway-controller/pkg/api/management/webbroker_types.go` |
| WebSubApi Handler | `gateway/gateway-controller/pkg/api/handlers/websub_api_handler.go` |
| WebBrokerApi Handler | `gateway/gateway-controller/pkg/api/handlers/webbroker_api_handler.go` |
| xDS Translator | `gateway/gateway-controller/pkg/policyxds/event_channel_translator.go` |
| Storage | `gateway/gateway-controller/pkg/storage/` |

### Runtime (Event Gateway)

| Component | File Path |
|-----------|-----------|
| Binding Types | `event-gateway/gateway-runtime/internal/binding/types.go` |
| xDS Handler | `event-gateway/gateway-runtime/internal/xdsclient/handler.go` |
| Runtime Core | `event-gateway/gateway-runtime/internal/runtime/runtime.go` |
| Hub | `event-gateway/gateway-runtime/internal/hub/hub.go` |
| WebSub Receiver | `event-gateway/gateway-runtime/internal/connectors/receiver/websub/connector.go` |
| WebBroker Receiver | `event-gateway/gateway-runtime/internal/connectors/receiver/websocket/broker_api_connector.go` |
| Kafka Driver | `event-gateway/gateway-runtime/internal/connectors/brokerdriver/kafka/endpoint.go` |
| Policy Engine | `gateway/gateway-runtime/policy-engine/` |

---

---

## 11. Policy Interchangeability

### Are Policies the Same Between WebSubApi and WebBrokerApi?

**Yes! Policies are fully interchangeable.** A policy that works in WebSubApi will work in WebBrokerApi and vice versa. The policy engine treats all policies identically regardless of which API type invokes them.

### How Policy Execution Works (Unified for Both API Types)

Both WebSubApi and WebBrokerApi use the **exact same policy engine** (`engine.Engine`) and follow the same execution flow:

```go
// In Hub.go - Same for both API types
msg → engine.ExecuteRequestHeaderPolicies(chainKey) → RequestHeaderResult
    → engine.ExecuteRequestBodyPolicies(chainKey)   → RequestBodyResult
    → Apply results to msg
```

**Implementation:**
- **WebSubApi:** Calls `Hub.ProcessInbound()`, `Hub.ProcessOutbound()`, `Hub.ProcessSubscribe()`
- **WebBrokerApi:** Calls `Hub.ProcessByChainKey()` with specific chain keys

Both ultimately call the same engine methods:
- `engine.ExecuteRequestHeaderPolicies()`
- `engine.ExecuteRequestBodyPolicies()`

### What Makes Them Interchangeable?

| Component | WebSubApi | WebBrokerApi | Same? |
|-----------|-----------|--------------|-------|
| **Policy Engine** | `engine.Engine` | `engine.Engine` | ✅ Same instance |
| **Message Format** | `connectors.Message` | `connectors.Message` | ✅ Same struct |
| **Policy Registry** | `registry.PolicyRegistry` | `registry.PolicyRegistry` | ✅ Same registry |
| **Execution Methods** | `ExecuteRequest*Policies()` | `ExecuteRequest*Policies()` | ✅ Same methods |
| **Result Types** | `RequestHeaderResult`, `RequestBodyResult` | `RequestHeaderResult`, `RequestBodyResult` | ✅ Same types |
| **Policy Definitions** | `policy-definition.yaml` | `policy-definition.yaml` | ✅ Same format |
| **Policy Factories** | `policy.PolicyFactory` | `policy.PolicyFactory` | ✅ Same interface |

### The Only Difference: Enforcement Point Naming

The **only** difference is the naming convention for enforcement points:

| WebSubApi Enforcement Point | WebBrokerApi Enforcement Point | Purpose |
|----------------------------|-------------------------------|---------|
| `on_subscription` | `on_connection_init.request` | Authenticate/authorize initial request |
| N/A | `on_connection_init.response` | Modify initial response |
| `on_message_received` (inbound) | `on_produce` | Validate/transform messages going TO broker |
| `on_message_delivery` (outbound) | `on_consume` | Validate/transform messages FROM broker |
| `on_unsubscription` | N/A | Handle unsubscription (WebSub-specific) |

### Example: Using the Same Policies in Both API Types

#### Authentication Policy Example

**WebSubApi:**
```yaml
apiVersion: wso2.com/v1
kind: WebSubApi
metadata:
  name: secure-webhooks
spec:
  hub:
    policies:
      - name: api-key-auth        # ← Same policy
        version: v1
        params:
          in: header
          name: X-API-Key
```

**WebBrokerApi:**
```yaml
apiVersion: wso2.com/v1
kind: WebBrokerApi
metadata:
  name: secure-websocket
spec:
  policies:
    on_connection_init:
      request:
        - name: api-key-auth      # ← Same policy
          version: v1
          params:
            in: header
            name: X-API-Key
```

#### Transformation Policy Example

**WebSubApi:**
```yaml
spec:
  receiver:
    policies:
      - name: transform-payload-case    # ← Same policy
        version: v1
        params:
          targetCase: lowercase
```

**WebBrokerApi:**
```yaml
spec:
  policies:
    on_produce:
      - name: transform-payload-case    # ← Same policy
        version: v1
        params:
          targetCase: lowercase
```

#### Validation Policy Example

**WebSubApi:**
```yaml
spec:
  hub:
    channels:
      - name: issues
        policies:
          - name: json-schema-validator   # ← Same policy
            version: v1
            params:
              schema:
                type: object
                required: [title, body]
```

**WebBrokerApi:**
```yaml
spec:
  channels:
    "/issues":
      policies:
        on_produce:
          - name: json-schema-validator   # ← Same policy
            version: v1
            params:
              schema:
                type: object
                required: [title, body]
```

### Policy Definition Structure (Agnostic to API Type)

A policy definition has **no API-type restrictions**:

```yaml
# gateway/policies/my-policy/policy-definition.yaml
name: my-custom-policy
version: v1.0.0
displayName: My Custom Policy
description: This policy works in both WebSubApi and WebBrokerApi

parameters:
  type: object
  properties:
    someParam:
      type: string
      description: A parameter that works everywhere

systemParameters:
  type: object
  properties: {}
```

**No `applyTo` or `apiType` field exists!** Policies are universal.

### When Would a Policy NOT Work?

A policy might not be semantically appropriate (but will still technically execute) if:

1. **Protocol-specific logic:** A policy that expects WebSocket-specific headers in a WebSub HTTP POST
2. **Broker-specific features:** A policy that uses Kafka-specific features (like partition keys) with an MQTT broker
3. **Timing mismatch:** Applying a subscription validation policy at the message delivery phase

**But these are semantic/logical issues, not technical restrictions.** The policy engine will still execute them.

### Benefits of This Design

✅ **Write Once, Use Everywhere:** Develop a policy once, use it in any API type  
✅ **Consistent Behavior:** Same policy behaves identically across API types  
✅ **Simplified Testing:** Test policies independently of API types  
✅ **Reusable Libraries:** Build policy libraries that work universally  
✅ **Easier Migration:** Move policies between API types without modification  
✅ **Future-Proof:** New API types automatically support existing policies  

---

## 12. Extensibility & Plugin Architecture

### Pluggable Receivers and Broker Drivers

The WebBrokerApi implementation uses a **plugin/registry pattern** that allows adding new receivers and broker drivers without modifying the core runtime. While WebSocket and Kafka are the default implementations, the architecture is designed to support any streaming protocol and message broker.

### Receiver Interface

**File:** `event-gateway/gateway-runtime/internal/connectors/types.go`

Any protocol can be a receiver by implementing:

```go
type Receiver interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

**Currently Implemented:**
- `websub` - HTTP WebSub (WebHooks)
- `websocket` - WebSocket receiver for legacy single-channel protocol mediation
  - Simple 1:1 passthrough between WebSocket client and broker
  - Single topic per API
  - Uses `ProcessInbound()` policy enforcement only
- `websocket-broker-api` - WebSocket receiver for multi-channel WebBrokerApi
  - Supports multiple channels per API via `X-channel` header routing
  - Per-channel policy chains (onConnectionInit, onProduce, onConsume)
  - Separate produce/consume topics per channel
  - Designed for the WebBrokerApi specification

**Future Possibilities:**
- `sse` - Server-Sent Events
- `grpc-stream` - gRPC bidirectional streaming
- `http-long-poll` - HTTP Long Polling
- `amqp` - AMQP protocol
- `mqtt-ws` - MQTT over WebSocket

### BrokerDriver Interface

**File:** `event-gateway/gateway-runtime/internal/connectors/types.go`

Any message broker can be a broker driver by implementing:

```go
type BrokerDriver interface {
    Publish(ctx context.Context, topic string, msg *Message) error
    Subscribe(groupID string, topics []string, handler MessageHandler) (Receiver, error)
    TopicExists(ctx context.Context, topic string) (bool, error)
    EnsureTopics(ctx context.Context, topics []string) error
    DeleteTopics(ctx context.Context, topics []string) error
    Close() error
}
```

**Currently Implemented:**
- `kafka` - Apache Kafka via franz-go

**Future Possibilities:**
- `mqtt` - MQTT broker
- `rabbitmq` - RabbitMQ
- `pulsar` - Apache Pulsar
- `redis` - Redis Streams
- `nats` - NATS JetStream
- `aws-sqs` - AWS SQS
- `azure-servicebus` - Azure Service Bus

### Plugin Registration

**File:** `event-gateway/gateway-runtime/cmd/event-gateway/plugins.go`

New types are registered at startup:

```go
func registerConnectors(registry *connectors.Registry, cfg *config.Config) {
    // Register broker drivers
    registry.RegisterBrokerDriver("kafka", func(cfg map[string]interface{}) (BrokerDriver, error) {
        return kafka.NewBrokerDriver(brokers)
    })
    
    registry.RegisterBrokerDriver("mqtt", func(cfg map[string]interface{}) (BrokerDriver, error) {
        return mqtt.NewBrokerDriver(cfg)  // Future implementation
    })
    
    // Register receivers
    registry.RegisterReceiver("websocket-broker-api", func(cfg ReceiverConfig) (Receiver, error) {
        return websocket.NewBrokerApiReceiver(cfg, opts)
    })
    
    registry.RegisterReceiver("sse", func(cfg ReceiverConfig) (Receiver, error) {
        return sse.NewBrokerApiReceiver(cfg, opts)  // Future implementation
    })
}
```

### Adding a New Receiver (e.g., SSE)

**1. Implement the Receiver interface:**

```go
// event-gateway/gateway-runtime/internal/connectors/receiver/sse/broker_api_connector.go
package sse

type SSEBrokerApiReceiver struct {
    channel      connectors.ChannelInfo
    processor    connectors.MessageProcessor
    brokerDriver connectors.BrokerDriver
    connections  map[string]*sseConnection
}

func NewBrokerApiReceiver(cfg connectors.ReceiverConfig, opts Options) (connectors.Receiver, error) {
    receiver := &SSEBrokerApiReceiver{
        channel:      cfg.Channel,
        processor:    cfg.Processor,
        brokerDriver: cfg.BrokerDriver,
        connections:  make(map[string]*sseConnection),
    }
    
    // Register HTTP handler for SSE endpoint
    cfg.Mux.HandleFunc(cfg.Channel.Context, receiver.handleSSE)
    
    return receiver, nil
}

func (r *SSEBrokerApiReceiver) Start(ctx context.Context) error {
    // Initialize SSE receiver
    return nil
}

func (r *SSEBrokerApiReceiver) Stop(ctx context.Context) error {
    // Close all SSE connections
    return nil
}
```

**2. Register in plugins.go:**

```go
registry.RegisterReceiver("sse", func(cfg connectors.ReceiverConfig) (connectors.Receiver, error) {
    return sse.NewBrokerApiReceiver(cfg, sse.BrokerApiOptions{
        Port: config.Server.HTTPPort,
        ConsumerGroupPrefix: config.Kafka.ConsumerGroupPrefix,
        Topics: cfg.Channel.Topics,
    })
})
```

**3. Use in API spec:**

```yaml
apiVersion: wso2.com/v1
kind: WebBrokerApi
metadata:
  name: sse-events
spec:
  context: /sse/events
  version: v1
  receiver:
    name: sse-receiver
    type: sse  # ← New receiver type
    properties:
      retry: 3000
  broker-driver:
    name: kafka-driver
    type: kafka
    properties:
      bootstrap.servers: kafka:9092
```

### Adding a New Broker Driver (e.g., MQTT)

**1. Implement the BrokerDriver interface:**

```go
// event-gateway/gateway-runtime/internal/connectors/brokerdriver/mqtt/endpoint.go
package mqtt

type MQTTBrokerDriver struct {
    client mqtt.Client
    brokers []string
}

func NewBrokerDriver(brokers []string) (connectors.BrokerDriver, error) {
    // Initialize MQTT client
    return &MQTTBrokerDriver{brokers: brokers}, nil
}

func (d *MQTTBrokerDriver) Publish(ctx context.Context, topic string, msg *connectors.Message) error {
    // Publish to MQTT broker
    return d.client.Publish(topic, 0, false, msg.Value)
}

func (d *MQTTBrokerDriver) Subscribe(groupID string, topics []string, handler MessageHandler) (Receiver, error) {
    // Create MQTT subscriber
    return newMQTTSubscriber(d.client, topics, handler)
}

// Implement other interface methods...
```

**2. Register in plugins.go:**

```go
registry.RegisterBrokerDriver("mqtt", func(cfg map[string]interface{}) (BrokerDriver, error) {
    brokers := extractBrokers(cfg)
    return mqtt.NewBrokerDriver(brokers)
})
```

**3. Use in API spec:**

```yaml
apiVersion: wso2.com/v1
kind: WebBrokerApi
metadata:
  name: mqtt-events
spec:
  context: /ws/mqtt
  version: v1
  receiver:
    name: ws-receiver
    type: websocket
  broker-driver:
    name: mqtt-driver
    type: mqtt  # ← New broker driver type
    properties:
      brokers:
        - tcp://mqtt-broker:1883
      client_id: event-gateway
```

### Example: SSE ↔ MQTT

Combining new receiver and broker driver types:

```yaml
apiVersion: wso2.com/v1
kind: WebBrokerApi
metadata:
  name: iot-events
spec:
  context: /sse/iot
  version: v1
  receiver:
    name: sse-receiver
    type: sse  # ← Server-Sent Events
  broker-driver:
    name: mqtt-driver
    type: mqtt  # ← MQTT broker
    properties:
      brokers:
        - tcp://mqtt-broker:1883
  channels:
    "/sensors":
      policies:
        on_produce:
          - name: map-topic
            version: v1
            params:
              mode: produceTo
              topic: iot/sensors/data
```

This enables: **SSE Client → Policy Engine → MQTT Broker**

### Benefits of This Architecture

- ✅ **Zero core changes** - Add new types without modifying runtime or controller
- ✅ **Type safety** - Compile-time checks via Go interfaces
- ✅ **Dynamic loading** - Runtime selects implementation based on spec
- ✅ **Configuration flexibility** - Each type gets custom properties
- ✅ **Testability** - Mock receivers/drivers for testing
- ✅ **Community extensibility** - Third parties can add new connectors

---

## Summary

Both **WebSubApi** and **WebBrokerApi** leverage the same foundational architecture:

1. **Controller** manages API lifecycle via REST API and SQLite storage
2. **xDS Protocol** distributes configurations to runtime instances
3. **Runtime** instantiates protocol-specific receivers and broker drivers via **plugin registry**
4. **Hub** orchestrates policy execution and message routing
5. **Policy Engine** enforces security and transformation at defined points

The architecture ensures:
- ✅ **Consistent patterns** across both API types
- ✅ **Dynamic configuration** via xDS for zero-downtime updates
- ✅ **Protocol flexibility** through pluggable receivers (WebSocket, SSE, etc.)
- ✅ **Broker flexibility** through pluggable drivers (Kafka, MQTT, RabbitMQ, etc.)
- ✅ **Policy enforcement** at appropriate protocol lifecycle points
- ✅ **Extensibility** without modifying core runtime code
