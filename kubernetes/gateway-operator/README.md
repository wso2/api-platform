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
helm install my-gateway-operator oci://ghcr.io/wso2/api-platform/helm-charts/gateway-operator --version 0.2.0
```

---

## 3. Apply Gateway (Bootstrap Gateway Components)

```sh
curl -X GET "https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/gateway-operator/config/samples/api_v1_gateway.yaml" \
  -o /tmp/api_v1_gateway.yaml

gatewayconfig_path="/tmp/api_v1_gateway.yaml"

kubectl apply -f $gatewayconfig_path
kubectl get gateway -n default -o json | jq '.items[0].status'
```

---

## 4. Apply RestApi (Configure APIs)

```sh
curl -X GET "https://raw.githubusercontent.com/wso2/api-platform/refs/heads/main/kubernetes/gateway-operator/config/samples/api_v1_restapi.yaml" \
  -o /tmp/api_v1_restapi.yaml

apiconfig_path="/tmp/api_v1_restapi.yaml"
kubectl create ns test
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
curl -X POST http://localhost:9090/certificates \
  -H "Content-Type: application/json" \
  -d "{\"certificate\":$(jq -Rs . < $cert_path),\"filename\":\"my-cert.pem\", \"name\":\"test\"}"
```

---

## 8. Test Secure Backend API Again

```sh
curl https://localhost:8444/ssa/info -vk
```


