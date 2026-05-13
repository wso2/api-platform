# Gateway Operator

The WSO2 API Platform Gateway Operator enables native Kubernetes deployment using a GitOps-friendly, operator-based model. It manages the full lifecycle of API gateways and REST APIs. You can use **either** platform CRDs **or** the **Kubernetes Gateway API** on the same operator build.

Use this mode when you need:

- Operator-driven reconciliation and drift correction.
- GitOps-friendly CRD workflows.
- A unified control surface for both API Platform CRDs and Kubernetes Gateway API resources.

For mode comparison and overall context, see [`kubernetes-gateway-v2.md`](./kubernetes-gateway-v2.md).

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
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true
```

### 2. Install Gateway Operator

```sh
helm install my-gateway-operator oci://ghcr.io/wso2/api-platform/helm-charts/gateway-operator --version 0.8.0 --set image.tag=0.8.1
```

## Deploying an API Gateway

Create an `APIGateway` resource to bootstrap gateway components:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: cluster-gw
spec:
  apiSelector:
    scope: Cluster  # Accepts APIs from any namespace
  infrastructure:
    labels:
      environment: dev
      team: platform
    annotations:
      prometheus.io/scrape: "true"
  configRef:
    name: custom-gateway-values  # Optional: reference a ConfigMap with custom Helm values
```

Apply the sample APIGateway using the YAML above:



```sh
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: custom-gateway-values
data:
  # The operator expects this key to be named exactly `values.yaml`.
  # These are *overrides* deep-merged into the operator's default gateway_values.yaml.
  values.yaml: |
    gateway:
      gatewayRuntime:
        service:
          type: ClusterIP
---
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: cluster-gw
spec:
  apiSelector:
    scope: Cluster  # Accepts APIs from any namespace 
  infrastructure:
    labels:
      environment: dev
      team: platform
    annotations:
      prometheus.io/scrape: "true"
  configRef:
    name: custom-gateway-values  # Optional: reference a ConfigMap with custom Helm values
EOF

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
    environment: "dev"
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

Apply the sample RestApi using the YAML above:

```sh
kubectl apply -f - <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: my-api
  labels:
    environment: "dev"
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
EOF

kubectl get restapi -n default -o json | jq '.items[0].status'
```

Port-forward the gateway service 

```sh
kubectl port-forward svc/cluster-gw-gateway-gateway-runtime 8443:8443
```

Invoke the deployed API

```sh
curl --request GET \
  --url https://localhost:8443/test/info \
  --header 'Accept: application/json' -k
```

## Management CRDs (LLM, MCP, API Key, Subscription)

For complete documentation on optional management-API-backed CRDs (`LlmProviderTemplate`, `LlmProvider`, `LlmProxy`, `Mcp`, `ApiKey`, `SubscriptionPlan`, `Subscription`, `Certificate`), see [gateway-operator-management-crds.md](./gateway-operator-management-crds.md).

That guide covers:
- Kubernetes Secret prerequisites for credentials and tokens
- LlmProviderTemplate, LlmProvider, and LlmProxy deployments
- MCP (Model Context Protocol) resources
- RestAPI with policy parameters from secrets
- API Key management across RestApi, LlmProvider, and LlmProxy parents
- Subscription and SubscriptionPlan resources
- valueFrom pattern for Kubernetes-native secret resolution
- Complete sample invocations and test endpoints

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

```sh
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Namespace
metadata:
  name: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-operator-demo
EOF

kubectl get namespace gateway-api-demo
```

### 2. GatewayClass

```sh
kubectl apply -f - <<'EOF'
# GatewayClass must use controllerName matching the operator so the operator can set status.conditions[Accepted].
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: wso2-api-platform
spec:
  controllerName: gateway.api-platform.wso2.com/gateway-operator
EOF

kubectl get gatewayclass wso2-api-platform
```

### 3. Gateway

```sh
kubectl apply -f - <<'EOF'
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
EOF

kubectl get gateway -n gateway-api-demo
```

### 4. Sample backend (Deployment + Service)

```sh
kubectl apply -f - <<'EOF'
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
EOF

kubectl get deploy,svc -n gateway-api-demo
```

Wait until the **Gateway** is **Programmed** and gateway workloads are **Ready**, then apply the HTTPRoute(s).

### 5. HTTPRoute (`hello-api`)

```sh
kubectl apply -f - <<'EOF'
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
EOF

kubectl get httproute hello-api -n gateway-api-demo
```

Test Curl: 

```sh
curl --request GET \
  --url 'https://localhost:8443/hello-context/hello' \
  --header 'Accept: application/json' \
  -k
```

### 6. Optional: second HTTPRoute (`hello-api-2`)

```sh
kubectl apply -f - <<'EOF'
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
EOF

kubectl get httproute -n gateway-api-demo
```

Test Curl: 

```sh
curl --request GET \
  --url 'https://localhost:8443/hello' \
  --header 'Accept: application/json' \
  -k
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

### 7. HTTPRoute with Policies (APIPolicy CR)

Attach policies to HTTPRoutes using the `APIPolicy` CR (`gateway.api-platform.wso2.com/v1alpha1`). Two attachment modes:

- **API-level** — set `spec.targetRef` to the HTTPRoute; entries in `spec.policies` are merged into `APIConfigData.policies`.
- **Rule-scoped** — omit `spec.targetRef`; reference the `APIPolicy` from an HTTPRoute rule via `filters[].type: ExtensionRef`.

#### 7.1 APIPolicy CRs (API-level + rule-scoped)

```sh
kubectl apply -f - <<'EOF'
# API-level policy: targetRef → hello-apipolicy-demo HTTPRoute
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIPolicy
metadata:
  name: httproute-demo-api-level
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-httproute-policies-demo
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: hello-apipolicy-demo
  policies:
    - name: set-headers
      version: v1
      params:
        request:
          headers:
            - name: X-Client-Version
              value: "1.2.3"
---
# Rule-scoped policy: no targetRef; referenced from HTTPRoute rule via ExtensionRef
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIPolicy
metadata:
  name: httproute-demo-rule-ratelimit
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-httproute-policies-demo
spec:
  policies:
    - name: basic-ratelimit
      version: v1
      params:
        limits:
          - requests: 3
            duration: 1m
EOF

kubectl get apipolicy -n gateway-api-demo
```

#### 7.2 HTTPRoute with ExtensionRef

```sh
kubectl apply -f - <<'EOF'
# HTTPRoute: API-level policies from APIPolicy with targetRef; one rule with rule-level ExtensionRef → APIPolicy,
# and one rule with no filters (API-level only, no resource/rule-scoped policies).
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: hello-apipolicy-demo
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-httproute-policies-demo
  annotations:
    gateway.api-platform.wso2.com/api-handle: gateway-api-demo-hello-apipolicy
    gateway.api-platform.wso2.com/api-version: "v1.0"
    gateway.api-platform.wso2.com/context: "/hello-policies-context"
    gateway.api-platform.wso2.com/display-name: "Hello API policies via APIPolicy CR"
    gateway.api-platform.wso2.com/project-id: "1234567890"
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
            value: /hello-policies
          method: GET
      filters:
        - type: ExtensionRef
          extensionRef:
            group: gateway.api-platform.wso2.com
            kind: APIPolicy
            name: httproute-demo-rule-ratelimit
      backendRefs:
        - group: ""
          kind: Service
          name: hello-backend
          port: 9080
          weight: 1
    - matches:
        - path:
            type: PathPrefix
            value: /hello-policies-plain
          method: GET
      backendRefs:
        - group: ""
          kind: Service
          name: hello-backend
          port: 9080
          weight: 1
EOF

kubectl get httproute hello-apipolicy-demo -n gateway-api-demo
```

Test the rule-scoped policy path (rate-limited) and the API-level-only path:

```sh
curl --request GET \
  --url 'https://localhost:8443/hello-policies-context/hello-policies' \
  --header 'Accept: application/json' -k

curl --request GET \
  --url 'https://localhost:8443/hello-policies-context/hello-policies-plain' \
  --header 'Accept: application/json' -k
```

#### 7.3 Secret-backed APIPolicy with `valueFrom`

The operator resolves `params.valueFrom.secretKeyRef` (or `configMapKeyRef`) to a plain string before calling gateway-controller, and re-reconciles the HTTPRoute whenever the referenced Secret/ConfigMap changes.

```sh
kubectl apply -f - <<'EOF'
# Secret + APIPolicy: subscription param resolved from Kubernetes Secret via valueFrom.secretKeyRef
apiVersion: v1
kind: Secret
metadata:
  name: httproute-demo-policy-credentials
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-httproute-policies-demo
type: Opaque
stringData:
  subscriptionKey: My-Key
---
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIPolicy
metadata:
  name: httproute-demo-rule-secret-params
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-httproute-policies-demo
spec:
  policies:
    - name: subscription-validation
      version: v1
      params:
        subscriptionKeyHeader:
          valueFrom:
            secretKeyRef:
              name: httproute-demo-policy-credentials
              key: subscriptionKey
              # namespace: <optional; defaults to the APIPolicy namespace>
EOF

kubectl get secret,apipolicy -n gateway-api-demo
```

#### 7.4 HTTPRoute referencing the secret-backed APIPolicy

```sh
kubectl apply -f - <<'EOF'
# Second HTTPRoute: rule policy pulls sensitive param from Secret via APIPolicy params.valueFrom.secretKeyRef
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: hello-apipolicy-secrets-demo
  namespace: gateway-api-demo
  labels:
    app.kubernetes.io/part-of: gateway-api-httproute-policies-demo
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
            value: /hello-secrets
          method: GET
      filters:
        - type: ExtensionRef
          extensionRef:
            group: gateway.api-platform.wso2.com
            kind: APIPolicy
            name: httproute-demo-rule-secret-params
      backendRefs:
        - group: ""
          kind: Service
          name: hello-backend
          port: 9080
          weight: 1
EOF

kubectl get httproute hello-apipolicy-secrets-demo -n gateway-api-demo
```

Test:

```sh
curl --request GET \
  --url 'https://localhost:8443/hello-secrets' \
  --header 'Accept: application/json' -k
```

**Validate Secret watch:** patch the Secret and confirm the operator re-reconciles `hello-apipolicy-secrets-demo` without editing the HTTPRoute:

```sh
kubectl patch secret httproute-demo-policy-credentials -n gateway-api-demo \
  --type merge -p '{"stringData":{"subscriptionKey":"Updated-Key"}}'
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
