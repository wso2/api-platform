# API Platform Gateway Operator Helm Chart

This Helm chart deploys the API Platform Gateway Operator, a Kubernetes operator that manages the lifecycle of API Gateway instances and the APIs deployed onto them.

## Overview

The Gateway Operator is responsible for:
- Managing `APIGateway` custom resources (gateway instances)
- Managing `RestApi` and the other API-management custom resources (ApiKey, APIPolicy, Certificate, LlmProvider, LlmProviderTemplate, LlmProxy, ManagedSecret, Mcp, Subscription, SubscriptionPlan)
- Automatically deploying and configuring gateway instances (via the gateway Helm chart)
- Reconciling gateway state with desired configuration
- Optionally reconciling Kubernetes Gateway API resources (Gateway, HTTPRoute) for managed gateway classes

All operator CRDs live in the API group **`gateway.api-platform.wso2.com`**, served at `v1alpha1` and `v1` (v1 is the storage version; the two schemas are identical and bridged with conversion `strategy: None`).

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- cert-manager v1.0+ (optional, for gateway TLS certificate management)

## Installation

### Basic Installation

```bash
helm install apip-operator ./operator-helm-chart --namespace gateway-operator-system --create-namespace
```

### Install with Custom Values

```bash
helm install apip-operator ./operator-helm-chart \
  --namespace gateway-operator-system --create-namespace \
  --set image.tag=0.10.0 \
  --set gateway.controlPlaneHost=http://my-control-plane:3001
```

### Install from OCI Registry

```bash
helm install apip-operator oci://ghcr.io/wso2/api-platform/helm-charts/gateway-operator \
  --version <chart-version>
```

## Configuration

### Key Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `1` |
| `watchNamespaces` | Namespaces to watch (cluster-wide if empty) | `[]` |
| `image.repository` | Operator image repository | `ghcr.io/wso2/api-platform/gateway-operator` |
| `image.tag` | Operator image tag | `0.10.0` |
| `image.pullPolicy` | Image pull policy | `Always` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `controller-manager` |
| `debug.enabled` | Enable debug mode (Delve) | `false` |
| `debug.port` | Debug port | `2345` |

### Gateway Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gateway.controlPlaneHost` | Control plane API endpoint | `http://platform-api:3001` |
| `gateway.helm.chartName` | Gateway Helm chart OCI or repo reference (ignored if `chartPath` is set) | `oci://ghcr.io/wso2/api-platform/helm-charts/gateway` |
| `gateway.helm.chartVersion` | Gateway chart version (for remote pulls; also used in upgrade signatures) | `1.1.0` |
| `gateway.helm.chartPath` | Local chart dir or `.tgz` path **inside the operator pod**; when non-empty, remote chart lookup (`chartName`/`chartVersion`) and registry auth are ignored | `""` |
| `gateway.helm.valuesFilePath` | Path to gateway values file | `/config/gateway_values.yaml` |
| `gateway.helm.insecureRegistry` | Skip TLS verification for OCI registries (still HTTPS) | `false` |
| `gateway.helm.plainHTTP` | Use plain HTTP for OCI registries | `false` |
| `gateway.helm.registryCredentialsSecret.name` | Secret holding private-registry credentials for the gateway chart pull (empty = anonymous) | `""` |

### Gateway API (Kubernetes Gateway API) Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gatewayApi.installStandardCRDs` | Install the standard-channel Gateway API CRDs | `true` |
| `gatewayApi.managedGatewayClassNames` | `gatewayClassName` values this operator reconciles | `[wso2-api-platform]` |
| `gatewayApi.clusterDomain` | Cluster DNS suffix for in-cluster Service URLs | `cluster.local` |

### Gateway Default Values (`gateway.values`)

The operator deploys gateway instances with the values under `gateway.values`, which mirror the gateway Helm chart's own `values.yaml` — see `kubernetes/helm/gateway-helm-chart/values.yaml` for the authoritative, fully-documented set. The two main components are:

- **`gateway.values.gateway.controller`** — control plane of the gateway. Service type `ClusterIP` by default; ports `rest: 9090`, `xds: 18000`, `policy: 18001`, `admin: 9092`, `metrics: 9091`. Supports TLS (see below), storage, persistence, and `deployment.replicaCount`.
- **`gateway.values.gateway.gatewayRuntime`** — the data plane (Envoy-based router + policy engine). Service type `LoadBalancer` by default; ports `http: 8080`, `https: 8443`, plus admin/metrics ports. Has its own `deployment.replicaCount`.

Component container image tags come from the gateway chart; do not hardcode them here.

### Reconciliation Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `reconciliation.syncPeriod` | Minimum frequency at which watched resources are re-reconciled (Go duration) | `10m` |
| `reconciliation.maxConcurrentReconciles` | Maximum concurrent reconciles (must be ≥ 1) | `1` |
| `reconciliation.maxRetryAttempts` | Maximum retry attempts for gateway operations | `10` |
| `reconciliation.initialBackoff` | Initial retry backoff (Go duration) | `1s` |
| `reconciliation.maxBackoffDuration` | Maximum exponential backoff (Go duration) | `60s` |

### Logging Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `logging.level` | Log level (debug, info, warn, error) | `info` |
| `logging.format` | Log format (json, text) | `json` |

### Security Context

| Parameter | Description | Default |
|-----------|-------------|---------|
| `securityContext.runAsNonRoot` | Run as non-root user | `true` |
| `securityContext.runAsUser` | User ID to run as | `10001` |

## Custom Resource Definitions (CRDs)

The chart ships all operator CRDs (group `gateway.api-platform.wso2.com`, served at `v1alpha1` and `v1`, with `v1` as the storage version) in the Helm-native `crds/` directory. Helm installs everything under `crds/` automatically on `helm install`.

> **Important — CRDs are install-only.** Helm applies `crds/` only when a CRD does not already exist, and **never updates or deletes** CRDs on `helm upgrade`, `helm uninstall`, or a re-`helm install` on a cluster where they already exist. This operator **requires the `v1` version to be served**, so:
> - **Fresh clusters** (no pre-existing `gateway.api-platform.wso2.com` CRDs) get the correct v1+v1alpha1 CRDs and work out of the box.
> - **Clusters that already have older (v1alpha1-only) CRDs** will NOT be upgraded by Helm. To move them to v1, delete the old CRDs first (`kubectl get crd -o name | grep gateway.api-platform.wso2.com | xargs kubectl delete`) then reinstall, or apply the updated CRDs manually with `kubectl apply -f crds/`.

The two most commonly used kinds:

### 1. APIGateway

Defines a gateway instance. `spec.apiSelector` is required; the gateway topology (replicas, service types, images, TLS, …) is supplied through a referenced ConfigMap via `spec.configRef`.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: APIGateway
metadata:
  name: my-gateway
  namespace: default
spec:
  apiSelector:
    scope: Cluster          # accept APIs from any namespace
  infrastructure:
    replicas: 1
  storage:
    type: sqlite
  # controlPlane: { host: "...", tls: { enabled: true } }
  # configRef: { name: my-gateway-values }   # ConfigMap with gateway Helm values
```

### 2. RestApi

Defines an API deployed onto matching gateways.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: my-api
  namespace: default
spec:
  displayName: my-api
  version: v1.0
  context: /my-api
  upstream:
    main:
      url: https://httpbin.org/anything
  operations:                 # required
    - method: GET
      path: /
```

## Usage

### Deploy the Operator

```bash
helm install apip-operator ./operator-helm-chart \
  --namespace gateway-operator-system \
  --create-namespace
```

### Verify Installation

```bash
# Check operator deployment
kubectl get deployment -n gateway-operator-system -l app.kubernetes.io/name=gateway-operator

# Check operator pods
kubectl get pods -n gateway-operator-system -l app.kubernetes.io/name=gateway-operator

# View operator logs
kubectl logs -f deployment/apip-operator-gateway-operator -n gateway-operator-system
```

### Create a Gateway Instance

To control gateway topology (separate controller / data-plane replica counts, a LoadBalancer data plane, etc.), supply gateway Helm values through a ConfigMap referenced by `spec.configRef`:

```bash
kubectl apply -f - <<EOF
apiVersion: gateway.api-platform.wso2.com/v1
kind: APIGateway
metadata:
  name: production-gateway
  namespace: default
spec:
  apiSelector:
    scope: Cluster
  storage:
    type: sqlite
  configRef:
    name: production-gateway-values
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: production-gateway-values
  namespace: default
data:
  values.yaml: |
    gateway:
      controller:
        deployment:
          replicaCount: 2
        service:
          type: ClusterIP
      gatewayRuntime:        # data-plane ("router")
        deployment:
          replicaCount: 3
        service:
          type: LoadBalancer
EOF
```

### Deploy an API

```bash
kubectl apply -f - <<EOF
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: petstore-api
  namespace: default
spec:
  displayName: petstore
  version: v1.0
  context: /petstore
  upstream:
    main:
      url: https://petstore.swagger.io
  operations:
    - method: GET
      path: /pet/findByStatus
    - method: POST
      path: /pet
EOF
```

### Monitor Resources

```bash
# List all gateway instances
kubectl get apigateways -A

# List all APIs
kubectl get restapis -A

# Describe a specific gateway
kubectl describe apigateway production-gateway
```

## Advanced Configuration

### Enable Debug Mode

Debug mode runs the operator under the Delve debugger for remote debugging:

```yaml
debug:
  enabled: true
  port: 2345
  debugImage: "ghcr.io/wso2/api-platform/gateway-operator:0.10.0-debug"
```

Connect your debugger to the debug port:

```bash
kubectl port-forward deployment/apip-operator-gateway-operator 2345:2345 -n gateway-operator-system
```

### Custom Gateway Values

Override default gateway values during installation (these feed the gateway Helm chart):

```yaml
gateway:
  values:
    gateway:
      controller:
        image:
          repository: myorg/custom-controller
          tag: v2.0.0
        deployment:
          replicaCount: 3
      gatewayRuntime:
        deployment:
          replicaCount: 5
        service:
          type: LoadBalancer
```

### TLS Configuration

Enable cert-manager integration for automatic gateway TLS certificates:

```yaml
gateway:
  values:
    gateway:
      controller:
        tls:
          enabled: true
          certificateProvider: cert-manager
          certManager:
            create: true
            createIssuer: true
            issuerRef:
              name: selfsigned-issuer
              kind: Issuer
            commonName: gateway.example.com
            dnsNames:
              - gateway.example.com
              - "*.gateway.example.com"
```

Or use an existing secret:

```yaml
gateway:
  values:
    gateway:
      controller:
        tls:
          enabled: true
          certificateProvider: secret
          secret:
            name: my-tls-secret
```

### Resource Limits

Configure resource limits for the operator:

```yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 250m
    memory: 256Mi
```

## Upgrading

### Upgrade the Chart

```bash
helm upgrade apip-operator ./operator-helm-chart \
  --namespace gateway-operator-system \
  --reuse-values
```

> **Note:** CRDs in `crds/` are **not** re-applied on `helm upgrade` (a Helm limitation). If a chart upgrade includes CRD schema changes, apply the updated CRDs manually with `kubectl apply -f crds/`. Because the operator requires the `v1` version to be served, do this **before** rolling out an operator image that expects it.

### Upgrade with New Values

```bash
helm upgrade apip-operator ./operator-helm-chart \
  --namespace gateway-operator-system \
  --set image.tag=0.11.0
```

## Uninstallation

### Uninstall the Release

```bash
helm uninstall apip-operator --namespace gateway-operator-system
```

**Note:** This does not delete the CRDs (Helm never removes CRDs installed from `crds/`). To delete them manually:

```bash
kubectl get crd -o name | grep gateway.api-platform.wso2.com | xargs -r kubectl delete
```

## Troubleshooting

### Operator Not Starting

Check operator logs and events:
```bash
kubectl logs deployment/apip-operator-gateway-operator -n gateway-operator-system
kubectl get events -n gateway-operator-system --sort-by='.lastTimestamp'
```

### Gateway Not Deploying

Check operator logs for reconciliation errors, then describe the resource:
```bash
kubectl logs deployment/apip-operator-gateway-operator -n gateway-operator-system | grep -i error
kubectl describe apigateway <name>
```

### CRDs Not Installing

CRDs are installed from the chart's `crds/` directory on `helm install`. Verify:
```bash
kubectl get crd | grep gateway.api-platform.wso2.com
```
If they are missing on a fresh install, ensure you did not pass `--skip-crds`. If they are present but **stale** (e.g. only `v1alpha1`), Helm will not update them on upgrade or reinstall — delete and reinstall, or run `kubectl apply -f crds/` (the operator requires the `v1` version to be served).

### Leader Election Issues

If running multiple replicas, check leader election:
```bash
kubectl logs deployment/apip-operator-gateway-operator -n gateway-operator-system | grep leader
```

## Architecture

The operator follows the Kubernetes operator pattern:

1. **Controller Manager**: Main operator process that watches the CRDs
2. **Reconciliation Loop**: Continuously ensures actual state matches desired state
3. **Helm Integration**: Uses Helm to deploy gateway instances from the gateway chart
4. **Control Plane Integration**: Syncs with the platform API for centralized management

## Components

- **`crds/` directory**: operator CRDs, installed by Helm on fresh install
- **ClusterRole/ClusterRoleBinding & Role/RoleBinding**: operator runtime RBAC
- **Leader Election Role/RoleBinding**: manages leader election for HA
- **ServiceAccount**: runtime identity for the operator
- **Deployment**: runs the operator controller manager
- **ConfigMap**: stores operator configuration and gateway values
- **Finalizer Job**: cleanup job run on operator deletion

## Development

### Local Testing

```bash
cd kubernetes/gateway-operator
make run
```

### Building a Custom Image

```bash
cd kubernetes/gateway-operator
make docker-build IMG=myorg/gateway-operator:test
```

### Running Tests

```bash
cd kubernetes/gateway-operator
make test
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/wso2/api-platform
- Documentation: See the `/kubernetes/gateway-operator/` directory

## License

See LICENSE file in the repository root.
