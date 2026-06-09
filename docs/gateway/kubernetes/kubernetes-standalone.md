# API Platform Gateway - Kubernetes Standalone Mode

This guide explains how to run API Platform Gateway in **Standalone Mode** using the gateway Helm chart only (without the Gateway Operator).

Standalone mode is recommended when you want:

- Direct Helm-based lifecycle management of gateway components.
- A simpler footprint without operator-managed CRDs.
- Explicit control over values and release upgrades.

For mode selection and architecture context, see [`kubernetes-gateway-v2.md`](./kubernetes-gateway-v2.md).

## What Gets Deployed

The gateway chart deploys the runtime components used by API Platform Gateway (controller and gateway runtime workloads) from chart templates and values.

Chart reference:

- OCI chart: `oci://ghcr.io/wso2/api-platform/helm-charts/gateway`
- Local chart (repo): `kubernetes/helm/gateway-helm-chart`

## Prerequisites

- Kubernetes `1.24+`
- Helm `3.12+`
- `kubectl`
- `cert-manager` (required for cert-manager-backed TLS flows)

## Install cert-manager

```bash
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true
```

Verify:

```bash
kubectl get pods -n cert-manager
```

## Install Gateway Chart

Use one of the following patterns.

### Default install

```bash
helm install ap-gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway \
  --set gateway.developmentMode=true
```

### Install into a dedicated namespace

```bash
kubectl create namespace api-gateway

helm install ap-gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway \
  --namespace api-gateway \
  --set gateway.developmentMode=true
```

### Install with control-plane overrides

```bash
helm install ap-gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway \
  --set gateway.controller.controlPlane.host="platform.example.com:8443" \
  --set gateway.controller.controlPlane.token.value="your-token-here" \
  --set gateway.developmentMode=true
```

### Install with a values file

```bash
helm install ap-gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway \
  -f custom-values.yaml
```

## Verify Installation

```bash
helm status ap-gateway
kubectl get all -l app.kubernetes.io/instance=ap-gateway
```

Check logs:

```bash
# Controller logs
kubectl logs -l app.kubernetes.io/component=controller

# Gateway runtime logs
kubectl logs -l app.kubernetes.io/component=gateway-runtime
```

## Upgrade and Uninstall

Upgrade:

```bash
helm upgrade ap-gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway -f custom-values.yaml
```

Uninstall:

```bash
helm uninstall ap-gateway
```

Namespace-scoped uninstall:

```bash
helm uninstall ap-gateway --namespace api-gateway
```

## Core Configuration Areas

Most runtime configuration is controlled in `values.yaml`. Common sections:

- `gateway.controller.image`, `gateway.gatewayRuntime.image`
- `gateway.<component>.deployment.*`
- `gateway.<component>.service.*`
- `gateway.controller.controlPlane.*`
- `gateway.controller.logging.*`
- `gateway.controller.tls.*`
- `gateway.controller.upstreamCerts.*`
- `gateway.config.policy_engine.*`

Refer to inline comments in chart `values.yaml` for all supported fields.

## TLS Configuration

### Option 1: cert-manager (recommended)

```bash
helm install ap-gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway \
  --set gateway.controller.tls.enabled=true
```

Production-style example:

```yaml
gateway:
  controller:
    tls:
      enabled: true
      certificateProvider: cert-manager
      certManager:
        createIssuer: false
        issuerRef:
          name: letsencrypt-prod
          kind: Issuer
        commonName: api.example.com
        dnsNames:
          - api.example.com
          - "*.api.example.com"
```

### Option 2: Existing TLS secret

```bash
kubectl create secret tls gateway-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key

helm install ap-gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway \
  --set gateway.controller.tls.enabled=true \
  --set gateway.controller.tls.certificateProvider=secret \
  --set gateway.controller.tls.secret.name=gateway-tls
```

## Upstream Custom CAs

When calling upstream services that use private/self-signed CAs:

```bash
kubectl create secret generic upstream-ca-certs \
  --from-file=ca1.crt=path/to/ca1.crt \
  --from-file=ca2.crt=path/to/ca2.crt

helm install ap-gateway oci://ghcr.io/wso2/api-platform/helm-charts/gateway \
  --set gateway.controller.upstreamCerts.enabled=true \
  --set gateway.controller.upstreamCerts.secretName=upstream-ca-certs
```

## Create and Invoke API

### Port-forward Gateway Controller Service

```bash
kubectl port-forward svc/ap-gateway-controller 9090:9090
```

### Verify gateway controller admin endpoint is running
```bash
curl http://localhost:9094/api/admin/v0.9/health
```

### Deploy an API configuration

```bash
curl -X POST http://localhost:9090/api/management/v0.9/rest-apis \
  -u admin:admin \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: reading-list-api-v1.0
spec:
  displayName: Reading-List-API
  version: v1.0
  context: /reading-list/$version
  upstream:
    main:
      url: https://apis.bijira.dev/samples/reading-list-api-service/v1.0
  policies:
    - name: set-headers
      version: v1
      params:
        request:
          headers:
            - name: x-wso2-apip-gateway-version
              value: v1.0.0
        response:
          headers:
            - name: x-environment
              value: development
  operations:
    - method: GET
      path: /books
    - method: POST
      path: /books
    - method: GET
      path: /books/{id}
    - method: PUT
      path: /books/{id}
    - method: DELETE
      path: /books/{id}
EOF
```

### Test routing through the gateway
```bash
curl http://localhost:8080/reading-list/v1.0/books
curl -k https://localhost:8443/reading-list/v1.0/books
```

## Next Steps

- For operator-managed lifecycle and CRDs, continue with [`gateway-operator.md`](./gateway-operator.md).
- For mode comparison and migration context from APK 1.3.0, see [`kubernetes-gateway-v2.md`](./kubernetes-gateway-v2.md).
