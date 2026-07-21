# APIGateway + RestApi demo (APIM control plane)

This walkthrough validates the **APIGateway** + **`RestApi`** path with an APIM control plane (distinct from the Kubernetes Gateway API mode):

1. Deploy a platform gateway via **`APIGateway`** (`gateway.api-platform.wso2.com`).
2. Push an API definition via **`RestApi`**, routing to a Kubernetes **`Service`** backend.

## Binding `RestApi` to a specific gateway

The operator picks a target with `registry.FindMatchingGateways(namespace, restApi.metadata.labels)` and then `findTargetGateway` uses the **first** match. If more than one gateway is registered with **`apiSelector.scope: Cluster`**, every `RestApi` matches all of them and the winner is **nondeterministic**—so APIs can land on a Kubernetes **`Gateway`** deployment instead of your **`APIGateway`**.

To pin APIs to this **`APIGateway`**:

- Set **`APIGateway.spec.apiSelector`** to **`scope: LabelSelector`** with **`matchLabels`** (see `02-apigateway.yaml`).
- Set the **same labels** on **`RestApi.metadata.labels`** (see `04-restapi.yaml`).
- Provide a values `ConfigMap` and reference it from the `APIGateway` to point the gateway controller to your APIM host/token (`01-gateway-values-configmap.yaml` and `02-apigateway.yaml`).
- Optional annotations on `RestApi.metadata.annotations` (for example `gateway.api-platform.wso2.com/project-id`) are copied verbatim into the gateway-controller `api.yaml` payload under `metadata.annotations` (same keys as on the CR).

If you also run the Gateway API demo, give that **`Gateway`** a different API selection (annotation **`gateway.api-platform.wso2.com/api-selector`**) so it does not use the same **`restapi-target`** label value as this **`APIGateway`**, or `RestApi` objects could match both selectors.

## Prerequisites

- Gateway Operator installed (with `APIGateway` / `RestApi` CRDs).
- Operator `config.yaml` points to a valid gateway Helm chart and has a mounted default `gateway_values.yaml`.
- **At-rest AES-256 encryption key Secret** pre-created in the Gateway's namespace (`apigateway-demo-apim`). Encryption is mandatory and fail-closed — nothing auto-generates the key, and the gateway chart will not render without it. Create it before applying the `APIGateway` (see [Create the encryption key Secret](#create-the-encryption-key-secret)).

## Create the encryption key Secret

At-rest AES-256 encryption is **mandatory and fail-closed**: the gateway chart will not render unless `gateway.controller.encryptionKeys.enabled=true` points at a pre-created Kubernetes Secret, and nothing auto-generates the key. The operator deploys the gateway pods into the Gateway's **own namespace**, so the Secret must live in **`apigateway-demo-apim`**.

`01-gateway-values-configmap.yaml` already sets:

```yaml
gateway:
  controller:
    encryptionKeys:
      enabled: true
      secretName: gateway-encryption-keys
```

Create the matching Secret (once, before applying the `APIGateway`):

```bash
openssl rand 32 > default-aesgcm256-v1.bin
kubectl create secret generic gateway-encryption-keys \
  --from-file=default-aesgcm256-v1.bin=default-aesgcm256-v1.bin \
  -n apigateway-demo-apim
rm -f default-aesgcm256-v1.bin   # don't leave the plaintext key on disk
```

## Apply (order matters)

```bash
cd kubernetes/helm/resources/apim-apigateway-restapi-operator-demo

kubectl apply -f 00-namespace.yaml
# Pre-create the AES-256 encryption key Secret in apigateway-demo-apim
# (see "Create the encryption key Secret" above) before applying the APIGateway.
kubectl apply -f 01-gateway-values-configmap.yaml
kubectl apply -f 02-apigateway.yaml
kubectl apply -f 03-backend.yaml
# Wait for the gateway Helm workloads to become Ready.
kubectl apply -f 04-restapi.yaml
```

To execute management flows separately, apply split manifests instead:

```bash
# Shared prerequisites (Secret + ManagedSecret)
kubectl apply -f 05a-management-prerequisites.yaml

# Prism + nginx HTTPS mock for LLM upstream (see "LLM upstream mock" in Verification)
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
kubectl apply -f 05f0-apikey-resources.yaml

# SubscriptionPlan + Subscription flow
kubectl apply -f 05f-subscription-resources.yaml
```

## Verification

1. Check the `APIGateway` status:

```bash
kubectl get apigateway restapi-gw-apim -n apigateway-demo-apim -o yaml
```

2. Confirm Helm release and workloads:

```bash
helm list -n apigateway-demo-apim
kubectl get deploy,svc,pods -n apigateway-demo-apim
```

3. Check `RestApi` status:

```bash
kubectl get restapi hello-normal-api-apim -n apigateway-demo-apim -o yaml
```

4. Check management-resource CR status:

```bash
kubectl get llmprovidertemplate,llmprovider,llmproxy,mcp,managedsecret,certificate,apikey,subscriptionplan,subscription -n apigateway-demo-apim
```

### LLM upstream mock (integration-test parity)

Integration tests define **`http://mock-openapi:4010/openai/v1`** in **`gateway/it/docker-compose.test.yaml`** as **`mock-openapi`** (Prism + `gateway/it/mock-api` on **4010**) fronted by **`mock-openapi-https`** (nginx TLS on **9443**→**8443**, see `gateway/it/mock-api/nginx.conf`).

Apply **`05b0-mock-openapi-https.yaml`** in **`apigateway-demo-apim`** before **`05b-llm-resources.yaml`**. The mock **Service** uses port **9449** (nginx still listens on container **8443**) to avoid clashes with Rancher on **9443**. The companion standard demo uses the same manifest shape under **`apigateway-demo`**; optional full OpenAPI tree image: **`kubernetes/helm/resources/apigateway-restapi-operator-demo/llm-mock-openapi-it/Dockerfile`**.

```bash
kubectl run -n apigateway-demo-apim curl-mock --rm -it --restart=Never --image=curlimages/curl:8.5.0 -- \
  curl -sk https://mock-openapi-https:9449/health
```

**`05b-llm-resources.yaml`** uses **`openai-test`**, **`http://mock-openapi:4010/openai/v1`**, and **`Bearer sk-test-key`** aligned with **`llm-provider.feature`** / **`llm-proxies.feature`** (port differs from compose IT **9443**). The operator retries when the template or provider is not yet on the gateway and re-queues dependents when templates or providers change, so one **`kubectl apply -f 05b-llm-resources.yaml`** is enough.

5. Invoke via gateway-runtime Service (HTTPS may be enabled in your gateway values; use `-k` if needed):

```bash
curl --request GET \
  --url https://localhost:8443/hello-normal-apim \
  --header 'Accept: application/json' -k
```

Expected result:

```text
Request succeeds with an HTTP `200` response from `hello-backend`.
```

