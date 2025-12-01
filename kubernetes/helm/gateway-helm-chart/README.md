# Open Choreo Gateway Helm Chart

This chart packages the sample API Platform Gateway deployment (controller and Envoy router) that is provided as raw manifests in `internal/controller/resources/api-platform-gateway-k8s-manifests.yaml`.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+

## Installing the Chart

Install with default values:
```bash
helm install gateway ./gateway-helm-chart
```

Install with custom control plane configuration:
```bash
helm install gateway ./gateway-helm-chart \
  --set gateway.controller.controlPlane.host="host.docker.internal" \
  --set gateway.controller.controlPlane.port=8443 \
  --set gateway.controller.controlPlane.token.value="your-token-here"
```

Install in a specific namespace:
```bash
kubectl create namespace api-gateway
helm install gateway ./gateway-helm-chart \
  --namespace api-gateway \
  --set gateway.controller.controlPlane.host="platform.example.com" \
  --set gateway.controller.controlPlane.port=8443
```

Install with custom values file:
```bash
helm install gateway ./gateway-helm-chart -f custom-values.yaml
```

## Uninstalling the Chart

```bash
helm uninstall gateway
```

Or with namespace:
```bash
helm uninstall gateway --namespace api-gateway
```

## Upgrading the Chart

```bash
helm upgrade gateway ./gateway-helm-chart
```

Upgrade with new values:
```bash
helm upgrade gateway ./gateway-helm-chart -f custom-values.yaml
```

## Verifying the Installation

Check the status of the release:
```bash
helm status gateway
```

List all resources:
```bash
kubectl get all -l app.kubernetes.io/instance=gateway
```

Check pod logs:
```bash
# Controller logs
kubectl logs -l app.kubernetes.io/component=controller

# Router logs
kubectl logs -l app.kubernetes.io/component=router

# Policy Engine logs
kubectl logs -l app.kubernetes.io/component=policy-engine
```

## Chart layout

```
helm-chart/
├── templates/
│   ├── gateway/
│   │   ├── controller/     # Deployment, Service, ConfigMap, PVC
│   │   ├── router/         # Deployment and Service
│   │   └── policy-engine/  # Deployment, Service, ConfigMap
│   ├── serviceaccount.yaml
│   └── _helpers.tpl
├── values.yaml
└── README.md
```

Each major workload (controller, router, policy-engine) lives in its own nested template folder so their manifests remain isolated and easier to reason about.

## Configuration

All configurable values are documented in `values.yaml`. Component blocks are fully namespaced so overrides are intuitive:

- `gateway.controller.image` / `gateway.router.image` / `gateway.policyEngine.image` – container image metadata and pull policies.
- `gateway.<component>.deployment.*` – pod-level knobs (replicas, probes, affinities, env overrides, extra volumes) and enable/disable switches.
- `gateway.<component>.service.*` – service type/ports plus optional annotations and labels.
- `gateway.controller.persistence` / `gateway.controller.configMap` / `gateway.policyEngine.configMap` – PVC sizing/claims and component configuration payloads.
- `gateway.controller.controlPlane` and `gateway.controller.logging` – control-plane connectivity plus controller logging level.
- `gateway.controller.tls.*` – TLS certificate configuration for HTTPS listener using cert-manager or existing secrets.
- `gateway.controller.upstreamCerts.*` – Custom CA certificates for upstream backend TLS verification.
- `gateway.policyEngine.config.*` – policy engine configuration including xDS client settings and admin API.

Refer to the inline comments inside `values.yaml` for a complete matrix of options and the expected data types for each block.

## TLS Certificate Configuration

The gateway controller supports HTTPS with automatic certificate management via cert-manager or manual certificate provisioning.

### cert-manager Subchart

This chart includes cert-manager as an optional subchart. By default, it will be installed automatically:

```bash
# Install with bundled cert-manager
helm install gateway ./gateway-helm-chart
```

If you already have cert-manager installed in your cluster:

```bash
# Skip cert-manager installation
helm install gateway ./gateway-helm-chart \
  --set cert-manager.enabled=false
```

### Using cert-manager (Recommended)

With the default configuration, the chart will:
1. Install cert-manager (if enabled)
2. Create a self-signed Issuer for development
3. Generate a TLS certificate automatically

```bash
# Simple installation with automatic TLS
helm install gateway ./gateway-helm-chart \
  --set gateway.controller.tls.enabled=true
```

For production, configure a proper issuer (Let's Encrypt, corporate CA):

```yaml
# Disable cert-manager subchart if already installed
cert-manager:
  enabled: false

gateway:
  controller:
    tls:
      enabled: true
      certificateProvider: cert-manager
      certManager:
        createIssuer: false  # Don't create default self-signed issuer
        issuerRef:
          name: letsencrypt-prod
          kind: Issuer  # or ClusterIssuer
        commonName: api.example.com
        dnsNames:
          - api.example.com
          - "*.api.example.com"
```

### Using Existing Secret

```bash
# Create a secret with your TLS certificate
kubectl create secret tls gateway-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key

# Install with existing secret
helm install gateway ./gateway-helm-chart \
  --set gateway.controller.tls.enabled=true \
  --set gateway.controller.tls.certificateProvider=secret \
  --set gateway.controller.tls.secret.name=gateway-tls
```

### Custom Upstream Certificates

For backends with self-signed or custom CA certificates:

```bash
# Create a secret or configmap with CA certificates
kubectl create secret generic upstream-ca-certs \
  --from-file=ca1.crt=path/to/ca1.crt \
  --from-file=ca2.crt=path/to/ca2.crt

# Install with upstream certs
helm install gateway ./gateway-helm-chart \
  --set gateway.controller.upstreamCerts.enabled=true \
  --set gateway.controller.upstreamCerts.secretName=upstream-ca-certs
```

Refer to the inline comments inside `values.yaml` for a complete matrix of options and the expected data types for each block.
