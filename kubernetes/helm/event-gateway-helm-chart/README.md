# Event Gateway Helm Chart

Helm chart for deploying the **Event Gateway** components on Kubernetes.

## Components

| Component | Description |
|-----------|-------------|
| **Gateway Controller** | Manages event channel configurations and distributes them via xDS (EventChannelConfig) to the Event Gateway Runtime |
| **Event Gateway Runtime** | WebSub hub + WebSocket server that handles event subscriptions, deliveries, and streaming |

## Prerequisites

- Kubernetes `>=1.24.0`
- Helm `>=3.0`
- [cert-manager](https://cert-manager.io/) (if using `certificateProvider: cert-manager` for TLS)

## Installation

```bash
# Install with default values
helm install event-gateway ./event-gateway-helm-chart

# Install for local development (development mode, latest images)
helm install event-gateway ./event-gateway-helm-chart -f values-local.yaml

# Install in a specific namespace
helm install event-gateway ./event-gateway-helm-chart --namespace event-gateway --create-namespace
```

## Configuration

Key values to configure:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `eventGateway.developmentMode` | Relaxes encryption key requirement (dev/test only) | `false` |
| `eventGateway.controller.image.repository` | Gateway Controller image repository | `ghcr.io/wso2/api-platform/event-gateway-controller` |
| `eventGateway.controller.image.tag` | Gateway Controller image tag | `1.0.0` |
| `eventGateway.controller.controlPlane.host` | Control plane host | `host.docker.internal` |
| `eventGateway.controller.controlPlane.token.value` | Gateway registration token | `""` |
| `eventGateway.gatewayRuntime.image.repository` | Event Gateway Runtime image repository | `ghcr.io/wso2/api-platform/event-gateway-runtime` |
| `eventGateway.gatewayRuntime.image.tag` | Event Gateway Runtime image tag | `1.0.0` |
| `eventGateway.config.runtime.kafka.brokers` | Kafka broker addresses | `["kafka:9092"]` |
| `eventGateway.gatewayRuntime.service.type` | Kubernetes service type for runtime | `ClusterIP` |

## TLS Configuration

By default, TLS is enabled using cert-manager with a self-signed Issuer. To use an existing secret:

```yaml
eventGateway:
  controller:
    tls:
      certificateProvider: secret
      secret:
        name: my-controller-tls-secret

  gatewayRuntime:
    tls:
      certificateProvider: secret
      secret:
        name: my-runtime-tls-secret
```

## Encryption Keys (Production)

In production (`developmentMode: false`), you must provide an AES-GCM encryption key:

```bash
# Create the key file
openssl rand -out default-aesgcm256-v1.bin 32

# Create the Kubernetes secret
kubectl create secret generic event-gateway-encryption-keys \
  --from-file=default-aesgcm256-v1.bin=./default-aesgcm256-v1.bin
```

Then configure:
```yaml
eventGateway:
  controller:
    encryptionKeys:
      enabled: true
      secretName: event-gateway-encryption-keys
```

## Port Reference

| Port | Component | Description |
|------|-----------|-------------|
| `9090` | Controller | REST Management API |
| `18001` | Controller | Policy xDS gRPC (EventChannelConfig) |
| `8080` | Runtime | WebSub HTTP |
| `8443` | Runtime | WebSub HTTPS |
| `8081` | Runtime | WebSocket |
| `9002` | Runtime | Admin API |
| `9003` | Runtime | Metrics |
