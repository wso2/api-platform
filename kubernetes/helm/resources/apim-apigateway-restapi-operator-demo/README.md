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

## Apply (order matters)

```bash
cd kubernetes/helm/resources/apim-apigateway-restapi-operator-demo

kubectl apply -f 00-namespace.yaml
kubectl apply -f 01-gateway-values-configmap.yaml
kubectl apply -f 02-apigateway.yaml
kubectl apply -f 03-backend.yaml
# Wait for the gateway Helm workloads to become Ready.
kubectl apply -f 04-restapi.yaml
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

4. Invoke via gateway-runtime Service (HTTPS may be enabled in your gateway values; use `-k` if needed):

```bash
curl --request GET \
  --url https://localhost:8443/hello-normal-apim \
  --header 'Accept: application/json' -k
```

Expected result:

```text
Request succeeds with an HTTP `200` response from `hello-backend`.
```

