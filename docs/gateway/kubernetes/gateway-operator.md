# Gateway Operator

The WSO2 API Platform Gateway Operator enables native Kubernetes deployment using a GitOps-friendly, operator-based model. It manages the full lifecycle of API gateways and REST APIs. You can use **either** platform CRDs **or** the **Kubernetes Gateway API** on the same operator build.

## Overview

### Path A — Platform CRDs (`APIGateway` + `RestApi`)

| CRD | Purpose |
|-----|---------|
| `APIGateway` | Deploys and configures gateway infrastructure (controller, router, policy engine) |
| `RestApi` | Defines API routes, upstreams, and policies |

The operator watches these CRs, runs Helm for the gateway runtime, and deploys APIs through gateway-controller’s management REST API.

### Path B — Kubernetes Gateway API (`Gateway` + `HTTPRoute`)

| Resource | Purpose |
|----------|---------|
| `GatewayClass` | Cluster-scoped class your `Gateway` references (`spec.gatewayClassName` must match the operator allowlist). |
| `Gateway` (`gateway.networking.k8s.io`) | Triggers the same Helm-based gateway deployment as `APIGateway`; controller endpoint is registered for discovery by routes. |
| `HTTPRoute` | Parents attach to a `Gateway`; `backendRefs` target a Kubernetes `Service`. The operator maps the route to `APIConfigData` and calls gateway-controller **`/api/management/v0.9/rest-apis`** (same outcome as `RestApi`, different user surface). |
| `APIPolicy` (optional) | Rule or API-level policies for Gateway API flows; same CRD as HTTPRoute policy demos in-repo. |

**Hands-on walkthrough:** manifests are in **[Kubernetes Gateway API path](#kubernetes-gateway-api-path)** below.

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
helm install my-gateway-operator oci://ghcr.io/wso2/api-platform/helm-charts/gateway-operator --version 0.6.0
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
  context: /test
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

### Test API Endpoints

**`RestApi` / APIGateway-managed API** (example context `/test`, operation `GET /info`):

```sh
curl https://localhost:8443/test/info -vk
```

## Kubernetes Gateway API path

Use this when you prefer standard Gateway API resources instead of `APIGateway` / `RestApi`. The manifests below match the **`gateway-api-demo`** demo in this repository (`kubernetes/helm/resources/gateway-api-operator-demo/`). Apply them **in order**, or concatenate and `kubectl apply -f -`.

### What you need

- **Gateway Operator** Helm install with RBAC for `gateway.networking.k8s.io` (included in the operator chart).
- **Gateway API CRDs** in the cluster (cloud add-on, another controller, or `--set gatewayApi.installStandardCRDs=true` on a greenfield cluster where no conflicting CRD owner exists).
- **`GatewayClass`** whose `metadata.name` is listed in **`gatewayApi.managedGatewayClassNames`** (default includes `wso2-api-platform`).
- **`spec.controllerName`** on the `GatewayClass` should match the operator (`gateway.api-platform.wso2.com/gateway-operator`) so the operator can set **`Accepted`** status on the class.
- **cert-manager** if you add **Certificate** / **Issuer** via per-Gateway Helm values (not included in the minimal YAMLs below; extend with a `ConfigMap` and **`gateway.api-platform.wso2.com/helm-values-configmap`** on the `Gateway` when needed).
- A **`Service`** backend referenced from **`HTTPRoute.spec.rules[].backendRefs`**.

### 1. Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-operator-demo
```

### 2. GatewayClass

```yaml
# GatewayClass must use controllerName matching the operator so the operator can set status.conditions[Accepted].
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: wso2-api-platform
spec:
  controllerName: gateway.api-platform.wso2.com/gateway-operator
```

### 3. Gateway

```yaml
# Triggers the operator: Helm installs release named platform-gw-gateway, then registers the gateway-controller Service.
# Optional: set metadata.annotations["gateway.api-platform.wso2.com/helm-values-configmap"] to a ConfigMap name (key values.yaml)
# for per-Gateway Helm overrides (TLS, auth, developmentMode). See operator chart defaults in gateway_values.yaml.
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: platform-gw
  namespace: gateway-api-demo
  annotations:
    # Prevent this Gateway from matching RestApi CRs intended for APIGateway (CRD mode) in mixed demos.
    gateway.api-platform.wso2.com/api-selector: '{"scope":"LabelSelector","matchLabels":{"gateway.api-platform.wso2.com/restapi-target":"k8s"}}'
  labels:
    app.kubernetes.io/part-of: gateway-api-operator-demo
spec:
  gatewayClassName: wso2-api-platform
  infrastructure:
    labels:
      environment: dev
      team: platform
    annotations:
      prometheus.io/scrape: "true"
  listeners:
    - name: http
      port: 8080
      protocol: HTTP
      allowedRoutes:
        namespaces:
          from: Same
    - name: https
      port: 8443
      protocol: HTTPS
      allowedRoutes:
        namespaces:
          from: Same
```

### 4. Sample backend (Deployment + Service)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-backend
  namespace: gateway-api-demo
  labels:
    app: hello-backend
    app.kubernetes.io/part-of: gateway-api-operator-demo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hello-backend
  template:
    metadata:
      labels:
        app: hello-backend
    spec:
      containers:
        - name: sample-backend
          image: ghcr.io/wso2/api-platform/sample-service:latest
          args:
            - "-addr"
            - ":9080"
            - "-pretty"
          ports:
            - name: http
              containerPort: 9080
          resources:
            requests:
              cpu: 10m
              memory: 32Mi
---
apiVersion: v1
kind: Service
metadata:
  name: hello-backend
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-operator-demo
spec:
  type: ClusterIP
  selector:
    app: hello-backend
  ports:
    - name: http
      port: 9080
      targetPort: 9080
```

Wait until the **Gateway** is **Programmed** and gateway workloads are **Ready**, then apply the HTTPRoute(s).

### 5. HTTPRoute (`hello-api`)

```yaml
# Operator maps this route to APIConfigData and calls gateway-controller /api/management/v0.9/rest-apis.
# Default REST handle is namespace-name: gateway-api-demo-hello-api (override with gateway.api-platform.wso2.com/api-handle).
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: hello-api
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-operator-demo
  annotations:
    gateway.api-platform.wso2.com/api-version: "v1.0"
    gateway.api-platform.wso2.com/context: "/hello-context"
    gateway.api-platform.wso2.com/display-name: "Hello API"
spec:
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: platform-gw
      namespace: gateway-api-demo
  hostnames:
    - demo.gateway-api.local
  rules:
    - matches:
        # match.method is optional; if omitted, the operator emits GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS for this path.
        - path:
            type: PathPrefix
            value: /hello
          method: GET
      backendRefs:
        - group: ""
          kind: Service
          name: hello-backend
          port: 9080
          weight: 1
```

### 6. Optional: second HTTPRoute (`hello-api-2`)

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: hello-api-2
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-operator-demo
  annotations:
    gateway.api-platform.wso2.com/display-name: "Hello API 2"
spec:
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: platform-gw
      namespace: gateway-api-demo
  hostnames:
    - demo.gateway-api.local
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /hello
      backendRefs:
        - group: ""
          kind: Service
          name: hello-backend
          port: 9080
          weight: 1
```

Verify: `kubectl get gateway,httproute -n gateway-api-demo`, wait for parent conditions on the HTTPRoute, then exercise the API (port-forward or in-cluster curl to **gateway-runtime** HTTPS as in **Testing APIs** below).

### HTTPRoute annotations (payload metadata)

Common annotations on `HTTPRoute` are copied into the **`api.yaml`** payload (for example **`gateway.api-platform.wso2.com/context`**, **`api-version`**, **`api-handle`**, **`display-name`**, **`project-id`**). If **`context`** is omitted or only whitespace, it defaults to **`/`**. If a rule **`match`** omits **`method`**, the operator emits all RestApi-supported verbs for that path: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS.

### Mixed clusters (`RestApi` + `Gateway`)

If you run **both** `APIGateway`-selected **`RestApi`** resources and **Gateway API** routes, keep the **`gateway.api-platform.wso2.com/api-selector`** annotation on the **`Gateway`** (as in the YAML above) so this gateway does not select `RestApi` CRs meant for another `APIGateway`.

### Test API Endpoints

**Kubernetes Gateway API** — HTTPRoute **`hello-api`** from [above](#5-httproute-hello-api): API **`context`** `/hello-context`, route match path prefix **`/hello`** (hits Envoy HTTPS on the forwarded router port):

```sh
curl --request GET \
  --url 'https://localhost:8443/hello-context/hello' \
  --header 'Accept: application/json' \
  -k
```

Use **`NS=gateway-api-demo`** in the port-forward snippet when testing that demo. The sample backend may respond with a short plain-text body (e.g. `hello from gateway api demo`) depending on chart and image version.

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
curl -X POST http://localhost:9090/api/management/v0.9/certificates -u "admin:admin" \
  -H "Content-Type: application/json" \
  -d "{\"certificate\":$(jq -Rs . < $cert_path),\"filename\":\"my-cert.pem\", \"name\":\"test\"}"
```

## Custom Configuration

Per-gateway Helm values are supplied as a **ConfigMap** whose data includes **`values.yaml`** (partial YAML is fine; the operator **deep-merges** it onto the operator’s default gateway values file loaded from **`gateway.helm.valuesFilePath`**).

### `APIGateway` (`spec.configRef`)

Create the ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-custom-config
data:
  values.yaml: |
    ### IMPORTANT you need to provide entire values yaml of the gateway helm. 
    gateway:
      controller:
        logging:
          level: debug
      router:
        service:
          type: LoadBalancer
```

Reference it from the **APIGateway**:

```yaml
spec:
  configRef:
    name: gateway-custom-config
```

### Kubernetes Gateway API (`Gateway`)

Use the **same ConfigMap** shape (`data.values.yaml`). Put the ConfigMap in the **same namespace** as the **`Gateway`**, then point the **`Gateway`** at it with this annotation (not a field on **`spec`**):

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: platform-gw
  namespace: gateway-api-demo
  annotations:
    gateway.api-platform.wso2.com/helm-values-configmap: gateway-custom-config
    # ... other annotations (e.g. api-selector) as needed
spec:
  gatewayClassName: wso2-api-platform
  # listeners, infrastructure, ...
```

The operator reads **`metadata.annotations[gateway.api-platform.wso2.com/helm-values-configmap]`**, loads **`ConfigMap.data["values.yaml"]`**, and merges it into the Helm values used for **`{metadata.name}-gateway`**, same merge rules as **`APIGateway.spec.configRef`**.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Gateway Operator                              │
│  Watches: APIGateway, RestApi; Gateway, HTTPRoute (+ Service,      │
│  APIPolicy, Secret, ConfigMap for Gateway API path)                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Gateway Components                          │
│  ┌─────────────────┐  ┌────────┐  ┌──────────────────┐         │
│  │ Gateway         │  │ Router │  │ Policy Engine    │         │
│  │ Controller      │  │(Envoy) │  │                  │         │
│  │ (Control Plane) │  │        │  │                  │         │
│  └─────────────────┘  └────────┘  └──────────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

- **CRD path:** `APIGateway` drives Helm; `RestApi` drives management REST deploys.
- **Gateway API path:** `Gateway` drives the same Helm install pattern; `HTTPRoute` is translated to the same management REST payload shape as `RestApi`.

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
- [Policies](../README.md#policies)
- [Gateway REST API](../../rest-apis/gateway/)
