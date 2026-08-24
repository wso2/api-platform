# Gateway Operator
The WSO2 API Platform is designed to run natively on Kubernetes, providing a fully GitOps- and operator-friendly deployment model.


# API Platform – Gateway Operator Quick Start Guide

This document explains how to install Cert-Manager, configure Docker Hub credentials, deploy the Gateway Operator, apply Gateway/API configurations, and test APIs locally.

---

## Prerequisites

* Kubernetes cluster (Docker Desktop, Kind, Minikube, OpenShift, etc.)
* `kubectl` installed
* `helm` installed (v3+)
* `jq` installed (for JSON output)

---

## 1. Install Cert-Manager (with CRDs)

```sh
helm upgrade --install \
  cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --version v1.19.1 \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true \
  --debug --wait --timeout 10m
```


---

## 2. Install Gateway Operator

```sh
helm install my-gateway-operator oci://ghcr.io/wso2/api-platform/helm-charts/gateway-operator --version 0.11.0
```

---

## 3. Apply APIGateway (Bootstrap Gateway Components)

```sh
curl -X GET "https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/gateway-operator/config/samples/api_v1_apigateway.yaml" \
  -o /tmp/api_v1_apigateway.yaml

apigatewayconfig_path="/tmp/api_v1_apigateway.yaml"

kubectl apply -f $apigatewayconfig_path
kubectl get apigateway -n default -o json | jq '.items[0].status'
```

---

## 4. Apply RestApi (Configure APIs)

```sh
curl -X GET "https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/gateway-operator/config/samples/api_v1_restapi.yaml" \
  -o /tmp/api_v1_restapi.yaml

apiconfig_path="/tmp/api_v1_restapi.yaml"
kubectl apply -f $apiconfig_path

kubectl get restapi -n default -o json | jq '.items[0].status'
kubectl get restapi -n default -o json | jq '.items[1].status'
```

---

## 5. Port-Forward Gateway Components

Kill existing port-forward sessions:

```sh
pkill -f "kubectl.*port-forward"
```

Start port-forwarding:

```sh
kubectl port-forward $(kubectl get pods -l app.kubernetes.io/component=controller -o jsonpath='{.items[0].metadata.name}') 9090:9090 &
kubectl port-forward $(kubectl get pods -l app.kubernetes.io/component=router -o jsonpath='{.items[0].metadata.name}') \
  8081:8080 8444:8443 9901:9901 &
```

---

## 6. Test APIs

### HTTPS Test API

Create sample secure backend

```sh
git clone https://github.com/wso2/api-platform.git
cd api-platform/kubernetes/helm/resources/secure-backend-k8s/k8s
kubectl apply -f .
kubectl wait --for=condition=ready pod -l app=secure-backend --timeout=120s
```

```sh
curl https://localhost:8444/test/info -vk
```

### Secure Backend API (expected to fail before adding certificate)

```sh
curl https://localhost:8444/ssa/info -vk
```

---

## 7. Add Certificate for Secure Backend API

Download certificate:

```sh
curl -X GET "https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/gateway/resources/secure-backend/test-backend-certs/test-backend.crt" \
  -o /tmp/test-backend.crt
```

Add certificate to Gateway:

```sh
cert_path="/tmp/test-backend.crt"
curl -X POST http://localhost:9090/api/management/v0.9/certificates \
  -H "Content-Type: application/json" \
  -d "{\"certificate\":$(jq -Rs . < $cert_path),\"filename\":\"my-cert.pem\", \"name\":\"test\"}"
```

---

## 8. Test Secure Backend API Again

```sh
curl https://localhost:8444/ssa/info -vk
```

---

## Per-gateway gateway version (image tag)

One operator can run multiple API Gateways, each on a **different gateway version**. The version is not a typed CR field — set it via the gateway's own `spec.configRef` ConfigMap, which the operator deep-merges as the highest-priority overlay on top of its default gateway values:

```
configRef ConfigMap  >  spec.infrastructure labels/annotations  >  operator default (/config/gateway_values.yaml)  >  chart built-in values
```

A configRef ConfigMap that contains only the image blocks overrides just the version and inherits everything else:

```yaml
# values.yaml key inside the configRef ConfigMap
gateway:
  controller:
    image: { repository: ghcr.io/wso2/api-platform/gateway-controller, tag: "1.2.0", pullPolicy: IfNotPresent }
  gatewayRuntime:
    image: { repository: ghcr.io/wso2/api-platform/gateway-runtime,   tag: "1.2.0", pullPolicy: IfNotPresent }
```

A full example — one gateway (`gw-default`) that tracks the operator default version and one (`gw-121`) pinned to 1.2.1, each with its own configRef ConfigMap — is in [`config/samples/api_v1_apigateway_multiversion.yaml`](config/samples/api_v1_apigateway_multiversion.yaml):

```sh
kubectl apply -f config/samples/api_v1_apigateway_multiversion.yaml

# confirm the versions running
kubectl get pods -n gw-default -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[*].image}{"\n"}{end}'
kubectl get pods -n gw-121     -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[*].image}{"\n"}{end}'
```

**Notes:**

- **Create the ConfigMap before the APIGateway.** The operator ignores ConfigMap *create* events, so an APIGateway created before its configRef ConfigMap won't be re-triggered by the later ConfigMap creation. (The sample orders Namespaces + ConfigMaps ahead of the APIGateways.)
- **`spec.infrastructure.image` / `routerImage` are not wired** — use the configRef `gateway.controller.image.*` / `gateway.gatewayRuntime.image.*` keys instead.
- **Updating an existing gateway:** edit its configRef ConfigMap. If the rollout doesn't trigger, restart the operator or bump the CR generation — the annotation must go under **`spec.infrastructure.annotations`** (a `.spec` change bumps `metadata.generation`); a `metadata.annotations` change does **not** bump generation and won't force a redeploy:
  ```sh
  kubectl patch apigateway gw-default -n gw-default --type merge \
    -p '{"spec":{"infrastructure":{"annotations":{"redeploy":"'"$(date +%s)"'"}}}}'
  ```
- **Changing the operator default (`GATEWAY_HELM_VALUES_FILE_PATH`) only auto-applies to *new* gateways.** It isn't watched or hashed, so restarting the operator won't roll it onto existing gateways — bump each existing CR's generation (above) to pick up a default change.
- Keep `gateway-controller` and `gateway-runtime` on the **same** version — they share the xDS/policy-definition contract.


