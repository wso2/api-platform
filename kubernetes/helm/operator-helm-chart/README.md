# API Platform Gateway Operator Helm Chart

This Helm chart deploys the API Platform Gateway Operator, a Kubernetes operator that manages the lifecycle of API Gateway instances and the APIs deployed onto them.

## Overview

The Gateway Operator is responsible for:
- Managing `APIGateway` custom resources (gateway instances)
- Managing `RestApi` and the other API-management custom resources (Agent, ApiKey, APIPolicy, Certificate, LlmProvider, LlmProviderTemplate, LlmProxy, ManagedSecret, Mcp, Subscription, SubscriptionPlan)
- Automatically deploying and configuring gateway instances (via the gateway Helm chart)
- Reconciling gateway state with desired configuration
- Optionally reconciling Kubernetes Gateway API resources (Gateway, HTTPRoute) for managed gateway classes

All operator CRDs live in the API group **`gateway.api-platform.wso2.com`**, served at `v1alpha1` and `v1` (v1 is the storage version; the two schemas are identical and bridged with conversion `strategy: None`).

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- cert-manager v1.0+ (optional, for gateway TLS certificate management)
- An AES-256 at-rest encryption key Secret in every namespace a gateway will be deployed into.

## Installation

### Basic Installation

```bash
helm install apip-operator ./operator-helm-chart --namespace gateway-operator-system --create-namespace
```

### Install with Custom Values

```bash
helm install apip-operator ./operator-helm-chart \
  --namespace gateway-operator-system --create-namespace \
  --set image.tag=0.10.1 \
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
| `image.tag` | Operator image tag | `0.10.1` |
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
| `gateway.helm.chartVersion` | Gateway chart version (for remote pulls; also used in upgrade signatures) | `2026.11.19-r1` |
| `gateway.helm.chartPath` | Local chart dir or `.tgz` path **inside the operator pod**; when non-empty, remote chart lookup (`chartName`/`chartVersion`) and registry auth are ignored | `""` |
| `gateway.helm.valuesFilePath` | Path to gateway values file | `/config/gateway_values.yaml` |
| `gateway.helm.insecureRegistry` | Skip TLS verification for OCI registries (still HTTPS) | `false` |
| `gateway.helm.plainHTTP` | Use plain HTTP for OCI registries | `false` |
| `gateway.helm.registryCredentialsSecret.name` | Secret holding private-registry credentials for the gateway chart pull (empty = anonymous) | `""` |
| `gateway.deletionGracePeriod` | How long (Go duration) a deleted APIGateway's gateway release keeps running before the operator uninstalls it; `0s` uninstalls at once. See [Gateway Deletion Safety](#gateway-deletion-safety) | `0s` |
| `gateway.retainVolumeOnDelete` | Keep each gateway controller's PersistentVolumeClaim (and its data) when the gateway release is uninstalled. See [Gateway Deletion Safety](#gateway-deletion-safety) | `false` |

### Gateway API (Kubernetes Gateway API) Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gatewayApi.installStandardCRDs` | Install the standard-channel Gateway API CRDs. Set `false` on OpenShift 4.19+, where the Ingress Operator manages these CRDs | `true` |
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

Both blocks are rendered as given, so any valid `PodSecurityContext` / `SecurityContext` field can be set. They are ignored when `debug.enabled=true` (Delve needs root and `SYS_PTRACE`).

> [!NOTE]
> **Upgrading from earlier chart versions:** `securityContext` is now the container-level security context. Earlier versions treated it as pod-level but rendered only `runAsUser` and `runAsNonRoot`, which work at both levels, so existing values keep their effect. Move any pod-only fields (`fsGroup`, `supplementalGroups`, `sysctls`) to `podSecurityContext`.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podSecurityContext` | Pod-level security context for the operator | `runAsNonRoot: true`, `seccompProfile.type: RuntimeDefault` |
| `securityContext` | Container-level security context for the operator | `runAsUser: 10001`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, `readOnlyRootFilesystem: true` |
| `securityContext.runAsUser` | UID for the operator container. Set to `null` to leave it unset (the image default `65532` is used, or OpenShift assigns one from the project range) | `10001` |
| `cleanupJob.podSecurityContext` | Pod-level security context for the pre-delete cleanup Job | `{}` |
| `cleanupJob.securityContext` | Container-level security context for the pre-delete cleanup Job | `{}` |

### OpenShift

| Parameter | Description | Default |
|-----------|-------------|---------|
| `openshift.enabled` | `true`, `false` or `auto`. `auto` turns OpenShift support on when the cluster serves `route.openshift.io/v1`; `helm template`, dry runs and GitOps renderers such as Argo CD resolve it to `false`, so use `true` there | `false` |

With OpenShift support on, the chart:

- grants the operator permission to manage Routes (`routes` and `routes/custom-host`);
- sets `openshift.enabled: true` in the rendered gateway values, so the gateway chart's optional management API Route (`gateway.values.openshift.routes.managementApi`, off by default) can be turned on. A value set in `gateway.values.openshift.enabled` takes precedence. Gateway chart `1.2.0` has no OpenShift support and ignores it.

The chart does not create Routes for API traffic. The hostnames a gateway serves come from its virtual hosts (`gateway.values.gateway.config.router.vhosts`) and from each API definition, so create a Route to the gateway's `gateway-runtime` Service for each hostname you use.

`openshift.enabled` does not change the security contexts. On OpenShift, set `securityContext.runAsUser: null` so the `restricted-v2` SCC assigns a UID from the project's range, or set a UID your SCC allows.

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

### Gateway Deletion Safety

Deleting an APIGateway makes the operator uninstall that gateway's Helm release, including the
gateway controller's PersistentVolumeClaim. By default this happens as soon as the deletion is
seen, so an APIGateway deleted by mistake (by hand, or by automation that manages APIGateways)
takes its gateway and the gateway's stored data with it. Two opt-in settings guard against that:

```yaml
gateway:
  deletionGracePeriod: "1h"    # keep a deleted gateway running for an hour before uninstalling
  retainVolumeOnDelete: true   # keep the controller's volume when the release is uninstalled
```

**Grace period.** With `gateway.deletionGracePeriod` set, a deleted APIGateway stays in the
`Deleting` phase until its `metadata.deletionTimestamp` plus the grace period has passed. Its
release keeps running and serving traffic in the meantime, and a `DeletionPending` condition
records when the uninstall will happen:

```bash
kubectl get apigateway my-gateway -o jsonpath='{.status.conditions[?(@.type=="DeletionPending")].message}'
```

To skip the wait for a deletion you do intend, annotate the APIGateway before or after deleting
it; the operator then uninstalls the release straight away:

```bash
kubectl annotate apigateway my-gateway gateway.api-platform.wso2.com/delete-immediately=true
```

**Recovering a deleted gateway.** Kubernetes cannot cancel a deletion once it has been requested,
so the deleted APIGateway object cannot be kept. The gateway itself can, as long as the grace
period has not run out:

1. Stop whatever deleted the APIGateway, so it does not delete the replacement as well.
2. Release the deleted object without uninstalling the gateway by removing the operator's
   finalizer. The APIGateway disappears and its release is left running, unowned:
   ```bash
   kubectl patch apigateway my-gateway --type=json \
     -p='[{"op":"remove","path":"/metadata/finalizers"}]'
   ```
3. Re-create the APIGateway with the same name and namespace (for example, re-apply the manifest
   it was created from; recreate its `spec.configRef` ConfigMap first if that was deleted too).
   Release names are derived from the APIGateway name, so the operator upgrades the release that
   is still running instead of installing a new one, keeping its pods and its volume.

**Keeping the volume.** With `gateway.retainVolumeOnDelete: true` the operator adds
`helm.sh/resource-policy: keep` to each gateway controller's PersistentVolumeClaim, so the claim
survives the uninstall. Re-creating an APIGateway with the same name later installs a release
with the same name, which adopts the retained claim and its data. Retained claims are not
cleaned up automatically; delete them by hand when they are no longer needed. The setting
reaches a gateway the next time the operator installs or upgrades it, so gateways deployed
before it was enabled are not covered until then. It has no effect on gateways that use
`gateway.controller.persistence.existingClaim` (the operator never owned that claim) or an
external database.

### Enable Debug Mode

Debug mode runs the operator under the Delve debugger for remote debugging:

```yaml
debug:
  enabled: true
  port: 2345
  debugImage: "ghcr.io/wso2/api-platform/gateway-operator:0.10.1-debug"
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
  --set image.tag=<image-tag>
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
