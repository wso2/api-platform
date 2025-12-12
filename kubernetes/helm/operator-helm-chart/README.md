# API Platform Gateway Operator Helm Chart

This Helm chart deploys the API Platform Gateway Operator, a Kubernetes operator that manages the lifecycle of API Gateway instances and API configurations.

## Overview

The Gateway Operator is responsible for:
- Managing Gateway custom resources
- Managing RestApi custom resources  
- Automatically deploying and configuring gateway instances
- Reconciling gateway state with desired configuration
- Integrating with the control plane API

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- cert-manager v1.0+ (optional, for TLS certificate management)

## Installation

### Basic Installation

```bash
helm install apip-operator ./operator-helm-chart
```

### Install with Custom Values

```bash
helm install apip-operator ./operator-helm-chart \
  --set image.tag=v1.0.0 \
  --set gateway.controlPlaneHost=http://my-control-plane:3001
```

### Install from OCI Registry

```bash
helm install apip-operator oci://registry-1.docker.io/yourorg/api-platform-operator \
  --version 0.0.1
```

## Configuration

### Key Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `1` |
| `image.repository` | Operator image repository | `tharsanan/api-platform/gateway-controller` |
| `image.tag` | Operator image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `Always` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `controller-manager` |
| `debug.enabled` | Enable debug mode (Delve) | `false` |
| `debug.port` | Debug port | `2345` |

### Gateway Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gateway.controlPlaneHost` | Control plane API endpoint | `http://platform-api:3001` |
| `gateway.helm.chartName` | Gateway Helm chart OCI reference | `oci://registry-1.docker.io/tharsanan/api-platform-gateway` |
| `gateway.helm.chartVersion` | Gateway chart version | `0.0.1` |
| `gateway.helm.valuesFilePath` | Path to gateway values file | `/config/gateway_values.yaml` |

### Gateway Default Values

The operator can deploy gateway instances with default values. These are defined under `gateway.values` and include:

#### Controller Configuration
- Image: `tharsanan/api-platform/gateway-controller:v1.0.0-m4`
- Service ports: REST (9090), xDS (18000), Policy (18001)
- TLS support with cert-manager integration
- Persistence with PVC support (100Mi default)

#### Router Configuration
- Image: `tharsanan/api-platform/gateway-router:v1.0.0-m4`
- Service ports: HTTP (8080), HTTPS (8443), Admin (9901)
- Envoy-based routing
- Health probes on admin port

#### Policy Engine Configuration
- Image: `tharsanan/api-platform/policy-engine:v1.0.0-m4`
- Service port: External processor (9001)
- xDS-based configuration
- Admin interface support

### Reconciliation Settings

| Parameter | Description | Default |
|-----------|-------------|---------|
| `reconciliation.enabled` | Enable periodic reconciliation | `true` |
| `reconciliation.interval` | Reconciliation interval in seconds | `300` |

### Logging Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `logging.level` | Log level (debug, info, warn, error) | `info` |
| `logging.format` | Log format (json, text) | `json` |

### Security Context

| Parameter | Description | Default |
|-----------|-------------|---------|
| `securityContext.runAsNonRoot` | Run as non-root user | `false` |
| `securityContext.runAsUser` | User ID to run as | `null` |

## Custom Resource Definitions (CRDs)

The chart installs two CRDs:

### 1. Gateway

Defines a gateway instance with all its components (controller, router, policy engine).

```yaml
apiVersion: api.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: my-gateway
  namespace: default
spec:
  # Gateway specification
```

### 2. RestApi

Defines an API that will be deployed to gateway instances.

```yaml
apiVersion: api.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: my-api
  namespace: default
spec:
  # API specification
```

## Usage

### Deploy the Operator

```bash
helm install apip-operator ./operator-helm-chart \
  --namespace api-platform \
  --create-namespace
```

### Verify Installation

```bash
# Check operator deployment
kubectl get deployment controller-manager -n api-platform

# Check operator pods
kubectl get pods -n api-platform -l control-plane=controller-manager

# View operator logs
kubectl logs -f deployment/controller-manager -n api-platform
```

### Create a Gateway Instance

```bash
kubectl apply -f - <<EOF
apiVersion: api.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: production-gateway
  namespace: default
spec:
  gateway:
    controller:
      replicaCount: 2
    router:
      replicaCount: 3
      service:
        type: LoadBalancer
EOF
```

### Deploy an API

```bash
kubectl apply -f - <<EOF
apiVersion: api.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: petstore-api
  namespace: default
spec:
  # API specification
EOF
```

### Monitor Resources

```bash
# List all gateway configurations
kubectl get gateway -A

# List all API configurations
kubectl get apiconfiguration -A

# Describe a specific gateway
kubectl describe gateway production-gateway
```

## Advanced Configuration

### Enable Debug Mode

Debug mode runs the operator under Delve debugger for remote debugging:

```yaml
debug:
  enabled: true
  port: 2345
  debugImage: "tharsanan/api-platform/gateway-controller:debug"
```

Connect your debugger to the debug port:

```bash
kubectl port-forward deployment/controller-manager 2345:2345 -n api-platform
```

### Custom Gateway Values

Override default gateway values during installation:

```yaml
gateway:
  values:
    gateway:
      controller:
        image:
          repository: myorg/custom-controller
          tag: v2.0.0
        replicaCount: 3
      router:
        replicaCount: 5
        service:
          type: LoadBalancer
```

### TLS Configuration

Enable cert-manager integration for automatic TLS certificates:

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
  --namespace api-platform \
  --reuse-values
```

### Upgrade with New Values

```bash
helm upgrade apip-operator ./operator-helm-chart \
  --namespace api-platform \
  --set image.tag=v2.0.0
```

## Uninstallation

### Uninstall the Release

```bash
helm uninstall apip-operator --namespace api-platform
```

**Note:** This will not delete CRDs. To delete CRDs manually:

```bash
kubectl delete crd gateway.gateway.api-platform.wso2.com
kubectl delete crd apiconfiguration.gateway.api-platform.wso2.com
```

## Troubleshooting

### Operator Not Starting

Check operator logs:
```bash
kubectl logs deployment/controller-manager -n api-platform
```

Check events:
```bash
kubectl get events -n api-platform --sort-by='.lastTimestamp'
```

### Gateway Not Deploying

Check operator logs for reconciliation errors:
```bash
kubectl logs deployment/controller-manager -n api-platform | grep -i error
```

Describe the gateway configuration:
```bash
kubectl describe gateway <name>
```

### CRDs Not Installing

Verify CRDs are installed:
```bash
kubectl get crd | grep api-platform.wso2.com
```

Manually install CRDs if needed:
```bash
kubectl apply -f crds/
```

### Leader Election Issues

If running multiple replicas, check leader election:
```bash
kubectl logs deployment/controller-manager -n api-platform | grep leader
```

## Architecture

The operator follows the Kubernetes operator pattern:

1. **Controller Manager**: Main operator process that watches CRDs
2. **Reconciliation Loop**: Continuously ensures actual state matches desired state
3. **Helm Integration**: Uses Helm to deploy gateway instances
4. **Control Plane Integration**: Syncs with platform API for centralized management

## Components

- **ClusterRole/ClusterRoleBinding**: Grants necessary RBAC permissions
- **Leader Election Role/RoleBinding**: Manages leader election for HA
- **ServiceAccount**: Identity for the operator
- **Deployment**: Runs the operator controller manager
- **ConfigMap**: Stores operator configuration and gateway values
- **Finalizer Job**: Cleanup job for operator deletion

## Development

### Local Testing

Run the operator locally:
```bash
cd kubernetes/gateway-operator
make run
```

### Building Custom Image

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
- Documentation: See `/kubernetes/gateway-operator/` directory

## License

See LICENSE file in the repository root.
