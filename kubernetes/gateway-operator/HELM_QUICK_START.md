# Quick Start: Helm-Based Gateway Deployment

## TL;DR

The Gateway Operator now uses Helm to deploy gateways instead of raw Kubernetes manifests.

## Configuration

Set these environment variables before starting the operator:

```bash
# Enable Helm deployment (default: true)
export GATEWAY_USE_HELM=true

# Helm chart location (default: ../helm/gateway-helm-chart)
export GATEWAY_HELM_CHART_PATH=../helm/gateway-helm-chart

# Chart version (default: 0.1.0)
export GATEWAY_HELM_CHART_VERSION=0.1.0

# Optional: Custom values file to override chart defaults
export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/custom-values.yaml
```

## How It Works

### Before (Template-based)
```
Gateway → Render YAML Template → Apply K8s Manifests
```

### Now (Helm-based)
```
Gateway → Use Chart Values → helm install/upgrade
```

## What Changed

### Deployment
- **Old**: Applied raw Kubernetes manifests via controller
- **New**: Installs/upgrades Helm releases

### Resource Management
- **Old**: Owner references for garbage collection
- **New**: Helm manages release lifecycle

### Values Configuration
- **Default**: Uses Helm chart's default `values.yaml`
- **Custom**: Optionally provide custom values file via `GATEWAY_HELM_VALUES_FILE_PATH`
- **No Templates**: Values are loaded directly from files, not rendered from templates

### Release Naming
| File | Purpose |
|------|---------|
| `internal/helm/client.go` | Helm client wrapper |
| `internal/config/config.go` | Configuration (line 38-48) |
| `internal/controller/gateway_controller.go` | Controller logic (line 256+) |
| `../helm/gateway-helm-chart/values.yaml` | Default Helm values |

## Usage

### Create Gateway (Default Values)
```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: my-gateway
spec:
  infrastructure:
    replicas: 2
```

### Create Gateway (Custom Values)
```bash
# Create custom values file
cat > my-values.yaml <<EOF
gateway:
  controller:
    replicaCount: 3
  router:
    replicaCount: 3
EOF

# Configure operator
export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/my-values.yaml

# Create gateway (uses custom values)
kubectl apply -f gateway.yaml
```adata:
  name: my-gateway
spec:
  infrastructure:
    replicas: 2
```

### Check Helm Release
```bash
helm list -n default
helm get values my-gateway-gateway -n default
```

### Delete Gateway
```bash
kubectl delete gateway my-gateway
# Helm release automatically uninstalled
```

## Troubleshooting

### Chart Not Found
```bash
# Check chart path
ls -la $GATEWAY_HELM_CHART_PATH/Chart.yaml
```

### Release Failed
```bash
# Check Helm status
helm status <release-name> -n <namespace>

# View operator logs
kubectl logs -n gateway-operator-system deployment/gateway-operator
```

### Disable Helm (Rollback)
```bash
export GATEWAY_USE_HELM=false
# Restart operator
```

## Development

### Build
```bash
cd kubernetes/gateway-operator
go mod tidy
make build
```

### Run Locally
```bash
make run
```

### Test
```bash
# Create gateway
kubectl apply -f config/samples/

# Verify Helm release
helm list -A

# Check gateway status
kubectl get gateway -A
```

## Dependencies

- `helm.sh/helm/v3@v3.16.3` - Helm Go SDK
- `k8s.io/cli-runtime@v0.29.2` - Kubernetes CLI runtime

## More Information

- Full documentation: [HELM_DEPLOYMENT.md](HELM_DEPLOYMENT.md)
- Implementation details: [IMPLEMENTATION_SUMMARY_HELM.md](IMPLEMENTATION_SUMMARY_HELM.md)
- Helm chart: [../helm/gateway-helm-chart/](../helm/gateway-helm-chart/)
