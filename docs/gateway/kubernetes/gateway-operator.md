# Gateway Operator

The WSO2 API Platform Gateway Operator enables native Kubernetes deployment using a GitOps-friendly, operator-based model. It manages the full lifecycle of API gateways and REST APIs through custom resources.

## Overview

The Gateway Operator watches for two custom resource types:

| CRD | Purpose |
|-----|---------|
| `APIGateway` | Deploys and configures gateway infrastructure (controller, router, policy engine) |
| `RestApi` | Defines API routes, upstreams, and policies |

## Prerequisites

- Kubernetes cluster (Docker Desktop, Kind, Minikube, OpenShift, etc.)
- `kubectl` installed
- `helm` v3+
- `jq` (for JSON output)

## Installation

### 1. Install Cert-Manager

The operator requires cert-manager for TLS certificate management:

```sh
helm upgrade --install \
  cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --version v1.19.1 \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true \
  --debug --wait --timeout 10m
```

### 2. Install Gateway Operator

```sh
helm install my-gateway-operator oci://ghcr.io/wso2/api-platform/helm-charts/gateway-operator --version 0.2.0
```

## Deploying an API Gateway

Create an `APIGateway` resource to bootstrap gateway components:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: cluster-gateway
spec:
  gatewayClassName: "production"
  
  apiSelector:
    scope: Cluster  # Accepts APIs from any namespace
  
  infrastructure:
    replicas: 1
    resources:
      requests:
        cpu: "500m"
        memory: "1Gi"
      limits:
        cpu: "2"
        memory: "4Gi"
  
  controlPlane:
    host: "gateway-control-plane.gateway-operator-system.svc.cluster.local:8443"
    tls:
      enabled: true
  
  storage:
    type: sqlite

  configRef:
    name: gateway-custom-config  # Optional: reference a ConfigMap with custom Helm values
```

Apply the sample APIGateway:

```sh
kubectl apply -f https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/gateway-operator/config/samples/api_v1_apigateway.yaml

kubectl get apigateway -n default -o json | jq '.items[0].status'
```

## Deploying REST APIs

Define APIs using the `RestApi` custom resource:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: my-api
  labels:
    environment: "production"
spec:
  displayName: My API
  version: v1.0
  context: /myapi
  upstream:
    main:
      url: https://httpbin.org/anything
  operations:
    - method: GET
      path: /info
    - method: POST
      path: /submit
```

Apply the sample RestApi:

```sh
kubectl apply -f https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/gateway-operator/config/samples/api_v1_restapi.yaml

kubectl get restapi -n default -o json | jq '.items[0].status'
```

## Testing APIs

### Port-Forward Gateway Components

```sh
# Kill existing port-forward sessions
pkill -f "kubectl.*port-forward"

# Forward controller and router ports
kubectl port-forward $(kubectl get pods -l app.kubernetes.io/component=controller -o jsonpath='{.items[0].metadata.name}') 9090:9090 &
kubectl port-forward $(kubectl get pods -l app.kubernetes.io/component=router -o jsonpath='{.items[0].metadata.name}') \
  8081:8080 8444:8443 9901:9901 &
```

### Test API Endpoints

```sh
# Test via HTTPS
curl https://localhost:8444/test/info -vk
```

## Adding Backend Certificates

For APIs connecting to backends with self-signed certificates:

### 1. Download the Certificate

```sh
curl -X GET "https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/gateway/resources/secure-backend/test-backend-certs/test-backend.crt" \
  -o /tmp/test-backend.crt
```

### 2. Add Certificate to Gateway

```sh
cert_path="/tmp/test-backend.crt"
curl -X POST http://localhost:9090/certificates \
  -H "Content-Type: application/json" \
  -d "{\"certificate\":$(jq -Rs . < $cert_path),\"filename\":\"my-cert.pem\", \"name\":\"test\"}"
```

## Custom Configuration

The `APIGateway` resource supports custom configuration via a ConfigMap reference. Create a ConfigMap with custom Helm values:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-custom-config
data:
  values.yaml: |
    gateway:
      controller:
        logging:
          level: debug
      router:
        service:
          type: LoadBalancer
```

Reference it in your APIGateway:

```yaml
spec:
  configRef:
    name: gateway-custom-config
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Gateway Operator                          │
│  Watches: APIGateway, RestApi CRDs                          │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    Gateway Components                        │
│  ┌─────────────────┐  ┌────────┐  ┌──────────────────┐      │
│  │ Gateway         │  │ Router │  │ Policy Engine    │      │
│  │ Controller      │  │(Envoy) │  │                  │      │
│  │ (Control Plane) │  │        │  │                  │      │
│  └─────────────────┘  └────────┘  └──────────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## Default Ports

| Port | Component | Description |
|------|-----------|-------------|
| 9090 | Controller | REST API for management |
| 18000 | Controller | xDS gRPC for Envoy |
| 18001 | Controller | Policy xDS |
| 8080 | Router | HTTP traffic |
| 8443 | Router | HTTPS traffic |
| 9901 | Router | Envoy admin |
| 9001 | Policy Engine | ext_proc gRPC |

## See Also

- [Gateway Quick Start (Docker Compose)](../quick-start-guide.md)
- [Policies](../policies/)
- [Gateway REST API](../gateway-rest-api/)
