# APIGateway + RestApi demo (CRD mode)

This walkthrough validates the **APIGateway** + **`RestApi`** path (distinct from the Kubernetes Gateway API mode):

1. Deploy a platform gateway via **`APIGateway`** (`gateway.api-platform.wso2.com`).
2. Push an API definition via **`RestApi`**, routing to a Kubernetes **`Service`** backend.

## Binding `RestApi` to a specific gateway

The operator picks a target with `registry.FindMatchingGateways(namespace, restApi.metadata.labels)` and then `findTargetGateway` uses the **first** match. If more than one gateway is registered with **`apiSelector.scope: Cluster`**, every `RestApi` matches all of them and the winner is **nondeterministic**—so APIs can land on a Kubernetes **`Gateway`** deployment instead of your **`APIGateway`**.

To pin APIs to this **`APIGateway`**:

- Set **`APIGateway.spec.apiSelector`** to **`scope: LabelSelector`** with **`matchLabels`** (see `02-apigateway.yaml`).
- Set the **same labels** on **`RestApi.metadata.labels`** (see `04-restapi.yaml`).
- Optional annotations on `RestApi.metadata.annotations` (for example `gateway.api-platform.wso2.com/project-id`) are copied verbatim into the gateway-controller `api.yaml` payload under `metadata.annotations` (same keys as on the CR).

If you also run the Gateway API demo, give that **`Gateway`** a different API selection (annotation **`gateway.api-platform.wso2.com/api-selector`**) so it does not use the same **`restapi-target`** label value as this **`APIGateway`**, or `RestApi` objects could match both selectors.

## Prerequisites

- Gateway Operator installed (with `APIGateway` / `RestApi` CRDs).
- Operator `config.yaml` points to a valid gateway Helm chart and has a mounted default `gateway_values.yaml`.

## Apply (order matters)

```bash
cd kubernetes/helm/resources/apigateway-restapi-operator-demo

kubectl apply -f 00-namespace.yaml
kubectl apply -f 01-gateway-values-configmap.yaml
kubectl apply -f 02-apigateway.yaml
kubectl apply -f 03-backend.yaml
# Wait for the gateway Helm workloads to become Ready.
kubectl apply -f 04-restapi.yaml
kubectl apply -f 04-restapi-policy.yaml

```

To execute management flows separately, apply split manifests instead:

```bash
# Shared prerequisites (Secret + ManagedSecret)
kubectl apply -f 05a-management-prerequisites.yaml

# Prism + nginx HTTPS mock for LLM upstream (parity with gateway IT docker-compose — see section below)
kubectl apply -f 05b0-mock-openapi-https.yaml

# LLM flow
kubectl apply -f 05b-llm-resources.yaml

# MCP backend service (required by MCP flow; IT parity URL http://mcp-server-backend:3001)
kubectl apply -f 05c0-mcp-server-backend.yaml

# MCP flow
kubectl apply -f 05c-mcp-resources.yaml

# Certificate flow
kubectl apply -f 05e-certificate-resources.yaml

# ApiKey flow
kubectl apply -f 04-03-restapi.yaml
kubectl apply -f 05b-02-llm-resources-apikey.yaml
kubectl apply -f 05f0-apikey-resources.yaml

# SubscriptionPlan + Subscription flow
kubectl apply -f 04-02-restapi.yaml
kubectl apply -f 05f-subscription-resources.yaml
```

## Verification

1. Check the `APIGateway` status:

```bash
kubectl get apigateway restapi-gw -n apigateway-demo -o yaml
```

2. Confirm Helm release and workloads (release name `restapi-gw-gateway`):

```bash
helm list -n apigateway-demo
kubectl get deploy,svc,pods -n apigateway-demo -l 'app.kubernetes.io/instance=restapi-gw-gateway'
```

3. Check `RestApi` status:

```bash
kubectl get restapi hello-normal-api -n apigateway-demo -o yaml
```

4. Check management-resource CR status:

```bash
kubectl get llmprovidertemplate,llmprovider,llmproxy,mcp,managedsecret,certificate,apikey,subscriptionplan,subscription -n apigateway-demo
```

### LLM upstream mock (integration-test parity)

Gateway IT wires the URL **`http://mock-openapi:4010/openai/v1`** in `gateway/it/docker-compose.test.yaml` as two services on a shared Docker network:

- **`mock-openapi`**: `stoplight/prism:5.14.3` runs `mock /api/openapi.yaml` with `gateway/it/mock-api` mounted read-only (HTTP **4010**).
- **`mock-openapi-https`**: `nginx:1.25` terminates TLS on **8443** (host **9443**), proxies to **`http://mock-openapi:4010`**, using **`gateway/it/mock-api/nginx.conf`** and **`gateway/gateway-controller/listener-certs`**.

The demo manifest **`05b0-mock-openapi-https.yaml`** recreates that pattern in-cluster (same namespace as the gateway): Prism with a **bundled** OpenAPI containing **`/health`** and **`/openai/v1/chat/completions`** (aligned with `gateway/it/mock-api/paths/openai/chat.yaml`), plus nginx and a small demo **TLS Secret** whose SANs cover **`mock-openapi-https`**, **`mock-openapi-https.apigateway-demo`**, **`mock-openapi-https.apigateway-demo-apim`**, and cluster FQDNs. The **Service** listens on **9449** (targeting nginx **8443**) so **`LlmProvider`** uses **`https://mock-openapi-https:9449/...`**, avoiding port clashes with **Rancher** which often binds **9443** (IT docker-compose still maps **9443** on the host). Apply **`05b0`** **before** **`05b-llm-resources.yaml`**.

For the **full** multi-path OpenAPI tree used in IT, build the optional image from **`llm-mock-openapi-it/Dockerfile`** (see comments in that file) and swap the Prism `Deployment` image.

Quick check from another pod in the same namespace:

```bash
kubectl run -n apigateway-demo curl-mock --rm -it --restart=Never --image=curlimages/curl:8.5.0 -- \
  curl -sk https://mock-openapi-https:9449/health
```

`05b-llm-resources.yaml` uses **`LlmProviderTemplate` `openai-test`**, **`LlmProvider`** **`template: openai-test`**, **`http://mock-openapi:4010/openai/v1`**, **`Bearer sk-test-key`**, and **`LlmProxy`** naming consistent with **`llm-provider.feature`** / **`llm-proxies.feature`** (port **9449** in-cluster vs **9443** in compose IT). If the gateway validates upstream TLS with a custom trust store, add the demo CA or relax verification for this internal mock. The operator **retries** gateway errors when a template or provider is not visible yet, and **re-queues** providers when a template changes and proxies when a provider changes, so applying the single file is fine even if reconcilers run in parallel.

### Sample LLM invocations (from integration-test flows)

These mirror the IT paths used in `llm-provider.feature` and `llm-proxies.feature`.

```bash
# LlmProvider context path from 05b
curl -sS -k \
  -H 'Content-Type: application/json' \
  --request POST \
  --url 'https://localhost:8443/llm-invoke-context/chat/completions' \
  --data '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"Hello from provider test"}]
  }'

# LlmProxy context path from 05b
curl -sS -k \
  -H 'Content-Type: application/json' \
  --request POST \
  --url 'https://localhost:8443/proxy-invoke-test/chat/completions' \
  --data '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"Hello from proxy test"}]
  }'
```

Expected result for both: HTTP `200` and a JSON body containing `"object":"chat.completion"` and `"choices"`.

### Sample LLM API-key invocations (from `05b-02-llm-resources-apikey.yaml`)

```bash
# LlmProvider API-key protected context from 05b-02 (wrong key -> reject)
curl -sS -k -i \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: wrong-api-key-not-valid' \
  --request POST \
  --url 'https://localhost:8443/llm-invoke-context-apikey/chat/completions' \
  --data '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"Hello from provider apikey test"}]
  }'

# LlmProvider API-key protected context from 05b-02 (valid key -> 200)
curl -sS -k \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: demo-llmprovider-apikey-value-1234567890-abcdef' \
  --request POST \
  --url 'https://localhost:8443/llm-invoke-context-apikey/chat/completions' \
  --data '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"Hello from provider apikey test"}]
  }'

# LlmProxy API-key protected context from 05b-02 (valid key -> 200)
curl -sS -k \
  -H 'Content-Type: application/json' \
  -H 'X-API-Key: demo-llmproxy-apikey-value-1234567890-abcdef' \
  --request POST \
  --url 'https://localhost:8443/proxy-invoke-test-apikey/chat/completions' \
  --data '{
    "model": "gpt-4",
    "messages": [{"role":"user","content":"Hello from proxy apikey test"}]
  }'
```

### Sample MCP client invocations (from integration-test flows)

These mirror `mcp_deploy.feature` and `steps_mcp.go` (`/everything/mcp`).

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

Expected result: initialize returns success JSON-RPC response; `tools/call` returns `result.content[0].text` containing `The sum of 40 and 60 is 100.`.  
If `SESSION_ID` is empty, inspect `/tmp/mcp-init-headers.txt` and copy the `mcp-session-id` value manually.

5. Invoke via gateway-runtime Service (HTTPS may be enabled in your default gateway values; use `-k` if needed):

```bash
curl --request GET \
  --url https://localhost:8443/hello-normal \
  --header 'Accept: application/json' -k
```

```bash
curl --request GET \
  --url https://localhost:8443/hello-normal-policy/test-policy \
  --header 'Accept: application/json' -k
```

```bash
curl --request GET \
  --url https://localhost:8443/hello-normal-policy/test-policy-resource \
  --header 'Accept: application/json' -k
```

```bash
curl --request GET \
  --url https://localhost:8443/hello-sub/new \
  --header 'Accept: application/json' -k
```

```bash
curl --request GET \
  --url https://localhost:8443/hello-sub/new \
  --header 'Accept: application/json' \
  --header 'My-Key: demo-subscription-token-1234567890-abcdef' -k
```

```bash
curl --request GET \
  --url https://localhost:8443/hello-apikey/test\
  --header 'Accept: application/json' \
  --header 'X-API-Key: demo-restapi-apikey-value-1234567890-abcdef' -k
```

Expected response body:

```text
hello from apigateway + restapi demo
```

