# Gateway Operator — Management CRDs

This document covers the optional management-API-backed custom resources supported by the Gateway Operator: `LlmProviderTemplate`, `LlmProvider`, `LlmProxy`, `Mcp`, `ApiKey`, `SubscriptionPlan`, `Subscription`, and `Certificate`.

For the main operator modes (Platform CRDs and Kubernetes Gateway API), see [gateway-operator.md](./gateway-operator.md).

## Overview

In addition to `RestApi`, the operator supports management-API-backed CRDs such as `LlmProviderTemplate`, `LlmProvider`, `LlmProxy`, `Mcp`, `ApiKey`, `SubscriptionPlan`, `Subscription`, and `Certificate`.

These resources use the same gateway selection model as `RestApi` (labels + `APIGateway.spec.apiSelector`).

## Shared Prerequisites (Kubernetes Secret)

Create a Kubernetes Secret containing credentials and tokens used by policy parameters and resource configurations:

```sh
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Secret
metadata:
  name: httproute-demo-policy-credentials
type: Opaque
stringData:
  subscriptionKey: My-Key
  subscriptionKeyResource: My-Key-Resource
  apikey-restapi-value: demo-apikey-value-1234567890-abcdef
  apikey-header-name: X-API-Key
  apikey-llmprovider-value: demo-llmprovider-apikey-value
  apikey-llmproxy-value: demo-llmproxy-apikey-value
  llm-upstream-authorization: Bearer sk-test-key
  subscription-token: demo-subscription-token-1234567890-abcdef
EOF

kubectl get secret httproute-demo-policy-credentials
```

**Note:** The operator uses Kubernetes-native `valueFrom.secretKeyRef` references in policy parameters and resource configurations, allowing dynamic secret resolution at reconciliation time. This approach provides better security and follows Kubernetes best practices for secret management.

## Deploy LLM Backend Mock (Prerequisites for LLM Flow)

The LLM resources require a mock OpenAI-compatible backend (Prism + nginx HTTPS). The manifest deploys to `apigateway-demo` by default; override to `default` namespace with:

```sh
curl -sL https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/helm/resources/apigateway-restapi-operator-demo/05b0-mock-openapi-https.yaml \
  | sed 's/namespace: apigateway-demo/namespace: default/g' \
  | kubectl apply -f -

kubectl get deploy,svc -n default -l 'app.kubernetes.io/name in (mock-openapi,mock-openapi-https)'
```

Wait for both Deployments to be `Ready` before proceeding.

## Deploy ApiKey Resources (Prerequisites)

Apply `ApiKey` CRs up front. The LLM-parent ApiKeys (`demo-llmprovider-apikey`, `demo-llmproxy-apikey`) will retry reconciliation automatically once their parent `LlmProvider`/`LlmProxy` are created.

```sh
kubectl apply -f - <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: ApiKey
metadata:
  name: demo-llmprovider-apikey
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  parentRef:
    kind: LlmProvider
    name: demo-llm-provider-apikey
  displayName: demo key for llm provider parent
  apiKey:
    valueFrom:
      name: httproute-demo-policy-credentials
      key: apikey-llmprovider-value
  expiresIn:
    duration: 30
    unit: days
---
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: ApiKey
metadata:
  name: demo-llmproxy-apikey
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  parentRef:
    kind: LlmProxy
    name: demo-llm-proxy-apikey
  displayName: demo key for llm proxy parent
  apiKey:
    valueFrom:
      name: httproute-demo-policy-credentials
      key: apikey-llmproxy-value
  expiresIn:
    duration: 30
    unit: days
EOF

kubectl get apikey
```

## Deploy LLMProviderTemplate, LLMProvider, LLMProxy with valueFrom

```sh
kubectl apply -f - <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProviderTemplate
metadata:
  name: openai-test
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  displayName: OpenAI
  promptTokens:
    location: payload
    identifier: $.usage.prompt_tokens
  completionTokens:
    location: payload
    identifier: $.usage.completion_tokens
  totalTokens:
    location: payload
    identifier: $.usage.total_tokens
  remainingTokens:
    location: header
    identifier: x-ratelimit-remaining-tokens
  requestModel:
    location: payload
    identifier: $.model
  responseModel:
    location: payload
    identifier: $.model
---
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: demo-llm-provider-apikey
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  displayName: OpenAI Provider with APIKey
  version: v1.0
  template: openai-test
  context: /llm-invoke-context-apikey
  accessControl:
    mode: allow_all
  policies:
    - name: api-key-auth
      version: v1
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            key:
              valueFrom:
                secretKeyRef:
                  name: httproute-demo-policy-credentials
                  key: apikey-header-name
            in: header
  upstream:
    url: http://mock-openapi:4010/openai/v1
    auth:
      type: api-key
      header: Authorization
      value:
        valueFrom:
            name: httproute-demo-policy-credentials
            key: llm-upstream-authorization
---
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProxy
metadata:
  name: demo-llm-proxy-apikey
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  displayName: Lifecycle LLM Proxy with APIKey
  version: v1.0
  context: /proxy-invoke-test-apikey
  policies:
    - name: api-key-auth
      version: v1
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            key:
              valueFrom:
                secretKeyRef:
                  name: httproute-demo-policy-credentials
                  key: apikey-header-name
            in: header
  provider:
    id: demo-llm-provider-apikey
    auth:
      type: api-key
      header: X-API-Key
      value:
        valueFrom:
            name: httproute-demo-policy-credentials
            key: apikey-llmprovider-value
EOF

kubectl get llmprovidertemplate,llmprovider,llmproxy
```

**valueFrom pattern for LLM resources:**
- Policy parameters in `policies[].paths[].params` support `valueFrom.secretKeyRef` for dynamic secret resolution.
- Upstream authentication credentials can be sourced from Kubernetes Secrets via `valueFrom`.
- The operator resolves all `valueFrom` references before sending the configuration to the gateway.

### Sample LLM API-key Protected Invocations

```bash
# LlmProvider API-key protected (wrong key -> reject)
curl -sS -k -i \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: wrong-api-key-not-valid' \
  --request POST \
  --url 'https://localhost:8443/llm-invoke-context-apikey/chat/completions' \
  --data '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"Hello from provider apikey test"}]
  }'

# LlmProvider API-key protected (valid key -> 200)
curl -sS -k \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: demo-llmprovider-apikey-value' \
  --request POST \
  --url 'https://localhost:8443/llm-invoke-context-apikey/chat/completions' \
  --data '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"Hello from provider apikey test"}]
  }'

# LlmProxy API-key protected (valid key -> 200)
curl -sS -k \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: demo-llmproxy-apikey-value' \
  --request POST \
  --url 'https://localhost:8443/proxy-invoke-test-apikey/chat/completions' \
  --data '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"Hello from proxy apikey test"}]
  }'
```

## Deploy MCP Backend (Prerequisites for MCP Flow)

The MCP resources require the `mcp-server-backend` service. The manifest deploys to `apigateway-demo` by default; override to `default` namespace with:

```sh
curl -sL https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/helm/resources/apigateway-restapi-operator-demo/05c0-mcp-server-backend.yaml \
  | sed 's/namespace: apigateway-demo/namespace: default/g' \
  | kubectl apply -f -

kubectl get deploy,svc -n default -l app.kubernetes.io/name=mcp-server-backend
```

Wait for the Deployment to be `Ready` before proceeding.

## Deploy MCP

```sh
kubectl apply -f - <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Mcp
metadata:
  name: everything-mcp-v1.0
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  displayName: Everything
  version: v1.0
  context: /everything
  specVersion: "2025-06-18"
  upstream:
    url: http://mcp-server-backend:3001
  tools: []
  resources: []
  prompts: []
EOF

kubectl get mcp
```

### Sample MCP Client Invocations

```bash
# 1) MCP initialize (capture headers to extract mcp-session-id)
curl -sS -k -D /tmp/mcp-init-headers.txt \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  --request POST \
  --url 'https://localhost:8443/everything/mcp' \
  --data '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"initialize",
    "params":{
      "protocolVersion":"2025-06-18",
      "capabilities":{"roots":{"listChanged":true}},
      "clientInfo":{"name":"gateway-it-client","version":"1.0.0"}
    }
  }'

# 2) Extract session id from initialize response headers
SESSION_ID="$(awk 'BEGIN{IGNORECASE=1} /^mcp-session-id:/ {print $2}' /tmp/mcp-init-headers.txt | tr -d '\r')"
echo "SESSION_ID=$SESSION_ID"

# 3) MCP tools/call ("add") using same session
curl -sS -k \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H "mcp-session-id: ${SESSION_ID}" \
  --request POST \
  --url 'https://localhost:8443/everything/mcp' \
  --data '{
    "jsonrpc":"2.0",
    "id":2,
    "method":"tools/call",
    "params":{
      "name":"add",
      "arguments":{"a":40,"b":60}
    }
  }'
```

Expected result: initialize returns success JSON-RPC response; `tools/call` returns `result.content[0].text` containing `The sum of 40 and 60 is 100.`

## Deploy RestAPI with Policy Parameter from Secret

```sh
kubectl apply -f - <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: hello-normal-api-policy
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  displayName: hello-normal-api-policy
  version: v1.0
  context: /hello-normal-policy
  upstream:
    main:
      url: http://hello-backend:9080
  policies:
    - name: subscription-validation
      version: v1
      params:
        subscriptionKeyHeader:
          valueFrom:
            secretKeyRef:
              name: httproute-demo-policy-credentials
              key: subscriptionKey
    - name: set-headers
      version: v1
      params:
        request:
          headers:
            - name: X-Client-Version
              value: "1.2.3"
  operations:
    - method: GET
      path: /test-policy
    - method: GET
      path: /test-policy-resource
      policies:
        - name: subscription-validation
          version: v1
          params:
            subscriptionKeyHeader:
              valueFrom:
                secretKeyRef:
                  name: httproute-demo-policy-credentials
                  key: subscriptionKeyResource
EOF

kubectl get restapi
```

Sample verification curls:

```bash
curl --request GET \
  --url https://localhost:8443/hello-normal-policy/test-policy \
  --header 'Accept: application/json' -k

curl --request GET \
  --url https://localhost:8443/hello-normal-policy/test-policy-resource \
  --header 'Accept: application/json' -k
```

**Pattern explanation:**
- Policy parameters support both inline values and `valueFrom` references.
- `valueFrom.secretKeyRef` resolves the value from a Kubernetes Secret at reconciliation time.
- Nested fields within policy parameters are automatically traversed and resolved.
- The resolved string value replaces the `valueFrom` object before the API configuration is sent to the gateway.

## Deploy Hello Backend (Prerequisites for RestAPI Flows)

The RestAPI resources route to the `hello-backend` service. The manifest deploys to `apigateway-demo` by default; override to `default` namespace with:

```sh
curl -sL https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/helm/resources/apigateway-restapi-operator-demo/03-backend.yaml \
  | sed 's/namespace: apigateway-demo/namespace: default/g' \
  | kubectl apply -f -

kubectl get deploy,svc -n default
```

Wait for the Deployment to be `Ready` before proceeding.

## Deploy RESTAPI with API Key

```sh
kubectl apply -f - <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: hello-apikey-api
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  displayName: hello-apikey-api
  version: v1.0
  context: /hello-apikey
  upstream:
    main:
      url: http://hello-backend:9080
  policies:
    - name: api-key-auth
      version: v1
      params:
        key: X-API-Key
        in: header
  operations:
    - method: GET
      path: /test
---
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: ApiKey
metadata:
  name: demo-restapi-apikey
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  parentRef:
    kind: RestApi
    name: hello-apikey-api
  displayName: demo key for rest api parent
  apiKey:
    valueFrom:
      name: httproute-demo-policy-credentials
      key: apikey-restapi-value
  expiresIn:
    duration: 29
    unit: days
EOF

kubectl get restapi,apikey
```

Sample verification curls:

```sh
# Wrong key should be rejected (typically 401)
curl --request GET \
  --url https://localhost:8443/hello-apikey/test \
  --header 'Accept: application/json' \
  --header 'X-API-Key: wrong-api-key-not-valid' \
  -k -i

# Correct key should pass (200)
curl --request GET \
  --url https://localhost:8443/hello-apikey/test \
  --header 'Accept: application/json' \
  --header 'X-API-Key: demo-apikey-value-1234567890-abcdef' \
  -k
```

## Deploy REST API with Subscriptions

```sh
kubectl apply -f - <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: hello-sub-api
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  displayName: hello-sub-api
  version: v1.0
  context: /hello-sub
  upstream:
    main:
      url: http://hello-backend:9080
  policies:
    - name: subscription-validation
      version: v1
      params:
        subscriptionKeyHeader:
          valueFrom:
            secretKeyRef:
              name: httproute-demo-policy-credentials
              key: subscriptionKey
  operations:
    - method: GET
      path: /new
---
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: SubscriptionPlan
metadata:
  name: demo-plan
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  planName: demo-plan
  status: ACTIVE
  stopOnQuotaReach: true
  throttleLimitCount: 1000
  throttleLimitUnit: Min
---
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Subscription
metadata:
  name: demo-subscription
  labels:
    gateway.api-platform.wso2.com/restapi-target: wso2-crd
spec:
  apiId: hello-sub-api
  subscriptionPlanId: demo-plan
  status: ACTIVE
  subscriptionToken:
    valueFrom:
      name: httproute-demo-policy-credentials
      key: subscription-token
EOF

kubectl get restapi,subscriptionplan,subscription
```

Sample verification curls:

# No key should be rejected (403 depending on gateway response mapping)
```sh
curl --request GET \
  --url https://localhost:8443/hello-sub/new \
  --header 'Accept: application/json' \
  -k -i

# Wrong key should be rejected (403)
curl --request GET \
  --url https://localhost:8443/hello-sub/new \
  --header 'Accept: application/json' \
  --header 'My-Key: wrong-subscription-token-not-valid' \
  -k -i

# Correct subscription token should pass (200)
curl --request GET \
  --url https://localhost:8443/hello-sub/new \
  --header 'Accept: application/json' \
  --header 'My-Key: demo-subscription-token-1234567890-abcdef' \
  -k
```

## Validation Notes

- `Subscription.spec.subscriptionPlanId` can be a literal ID or a `SubscriptionPlan` CR name.
- `ApiKey.spec.parentRef.kind` supports `RestApi`, `LlmProvider`, and `LlmProxy`.
- `ApiKey.spec.expiresAt` and `ApiKey.spec.expiresIn` are mutually exclusive (CEL validation rejects both together).

## valueFrom Pattern — Kubernetes-Native Secret Resolution

The operator supports Kubernetes-native secret management through the `valueFrom` pattern, allowing policy parameters, credentials, and sensitive values to be resolved from Kubernetes Secrets at reconciliation time.

### Supported Use Cases

The `valueFrom` pattern is supported in:

1. **Policy parameters** in RestApi, LlmProvider, and LlmProxy CRs
   - Any policy param field can reference a Kubernetes Secret
   - Nested policy parameter objects are recursively resolved

2. **Upstream authentication** in LlmProvider and LlmProxy
   - Bearer tokens and API keys sourced from Secrets

3. **API Key values** in ApiKey CRs
   - API key material resolved from Secrets

4. **Subscription tokens** in Subscription CRs
   - Authentication tokens resolved from Secrets

### Pattern Syntax

The `valueFrom` pattern supports two variants depending on the resource type:

**Variant 1: Policy Parameters** (RestApi, LlmProvider, LlmProxy)

Policy parameters use `valueFrom.secretKeyRef` for nested secret references:
```yaml
params:
  subscriptionKeyHeader:
    valueFrom:
      secretKeyRef:
        name: my-secret
        key: subscription-key
```

**Variant 2: Direct Fields** (ApiKey, Subscription, Upstream Auth)

Resource-level secrets use `valueFrom` directly with `name` and `key`:
```yaml
apiKey:
  valueFrom:
    name: my-secret
    key: api-key-value
```

**Fields:**
- `name`: Required. Name of the Kubernetes Secret.
- `key`: Required. The data key within the Secret.
- `namespace`: Optional. Defaults to the CR's namespace.

### How It Works

1. Operator watches all Kubernetes `Secret` resources in the namespace.
2. At reconciliation, operator fetches referenced Secret and extracts the value.
3. The `valueFrom` object is replaced with the resolved string value.
4. Configuration is sent to gateway-controller with resolved values only.
5. If Secret is missing, reconciliation retries automatically.

### Change Detection

The operator computes a fingerprint of all referenced Secret `resourceVersion` fields. When a Secret changes, affected CRs are automatically re-reconciled, ensuring credentials stay synchronized without manual intervention.

### Example: RestApi with Policy Parameter from Secret

Create the Secret:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-credentials
  namespace: default
type: Opaque
stringData:
  subscription-key: "My-Subscription-Key-123"
```

Reference in RestApi:
```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: protected-api
spec:
  context: /protected
  upstream:
    main:
      url: https://backend.example.com
  policies:
    - name: subscription-validation
      version: v1
      params:
        subscriptionKeyHeader:
          valueFrom:
            secretKeyRef:
              name: api-credentials
              key: subscription-key
```

The gateway receives the resolved policy parameter:
```yaml
policies:
  - name: subscription-validation
    version: v1
    params:
      subscriptionKeyHeader: "My-Subscription-Key-123"
```

Update the Secret anytime; the operator automatically redeploys with the new value.

### Example 2: ApiKey with Direct valueFrom

Reference in ApiKey (using `name` and `key` directly):
```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: ApiKey
metadata:
  name: my-api-key
spec:
  parentRef:
    kind: RestApi
    name: protected-api
  apiKey:
    valueFrom:
      name: api-credentials
      key: api-key-material
  expiresIn:
    duration: 30
    unit: days
```

**Key Difference:**
- RestApi policy params use nested `secretKeyRef` structure
- ApiKey, Subscription, and upstream auth use flat `name`/`key` under `valueFrom`
