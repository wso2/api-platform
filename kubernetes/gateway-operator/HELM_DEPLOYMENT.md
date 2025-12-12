# Helm-Based Gateway Deployment

This document describes the Helm-based deployment feature for the Gateway Operator.

## Overview

The Gateway Operator now supports deploying gateways using Helm charts instead of raw Kubernetes manifest templates. This provides several advantages:

- **Package Management**: Helm charts provide better versioning and dependency management
- **Values Customization**: More flexible configuration through values.yaml
- **Release Management**: Built-in support for upgrades, rollbacks, and release history
- **Ecosystem Integration**: Leverage the Helm ecosystem and tooling

## Architecture

### Components

1. **Helm Client** (`internal/helm/client.go`)
   - Provides functions to install, upgrade, and uninstall Helm charts
   - Handles release management and values merging

2. **Configuration** (`internal/config/config.go`)
   - New configuration options for Helm deployment:
     - `UseHelm`: Enable/disable Helm-based deployment
     - `HelmChartPath`: Path to the Helm chart
     - `HelmChartName`: Name of the Helm chart
     - `HelmChartVersion`: Version of the Helm chart
     - `HelmValuesFilePath`: Path to custom values.yaml (optional)

3. **Controller Updates** (`internal/controller/gateway_controller.go`)
   - Modified to support both Helm and template-based deployment
   - Automatic detection based on configuration
   - Helm release lifecycle management (install, upgrade, uninstall)

## Configuration

### Environment Variables

Configure Helm-based deployment using these environment variables:

```bash
# Enable Helm-based deployment (default: true)
export GATEWAY_USE_HELM=true

# Path to Helm chart (default: ../helm/gateway-helm-chart)
export GATEWAY_HELM_CHART_PATH=/path/to/helm/chart

# Helm chart name (default: api-platform-gateway)
export GATEWAY_HELM_CHART_NAME=api-platform-gateway

# Helm chart version (default: 0.1.0)
export GATEWAY_HELM_CHART_VERSION=0.1.0

# Optional: Path to custom values file to override chart defaults
export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/custom/values.yaml
```

### Default Configuration

By default, the operator uses:
- Helm chart: `../helm/gateway-helm-chart` (relative to operator)
- Chart version: `0.1.0`
- Values: Chart's default `values.yaml` (can be overridden with custom file)

### Custom Values

You can provide a custom values.yaml file in two ways:

1. **Environment Variable**:
   ```bash
   export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/custom-values.yaml
   ```

2. **Volume Mount** (in Docker/Kubernetes):
   ```yaml
   volumes:
     - /path/to/custom-values.yaml:/app/config/values.yaml
   ```
   ```bash
   export GATEWAY_HELM_VALUES_FILE_PATH=/app/config/values.yaml
   ```

## Usage

### Creating a Gateway

Create a `Gateway` resource as usual:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: my-gateway
  namespace: default
spec:
  gatewayClassName: default
  apiSelector:
    matchLabels:
      gateway: my-gateway
  infrastructure:
    replicas: 2
    image: wso2/api-platform/gateway-controller:v1.0.0-m4
    routerImage: wso2/api-platform/gateway-router:v1.0.0-m4
    resources:
      requests:
        cpu: 250m
        memory: 256Mi
      limits:
        cpu: "1"
        memory: 512Mi
  controlPlane:
    host: api-platform-control-plane:8443
  storage:
    type: sqlite
    sqlitePath: /app/data/gateway.db
```

The operator will:
1. Use the Helm chart's default values.yaml (or custom values if configured)
2. Install/upgrade the Helm release named `my-gateway-gateway`
3. Monitor the deployment status
4. Update the Gateway status

### Customizing Deployment

To customize the gateway deployment, you can:

1. **Modify the chart's values.yaml** directly in `../helm/gateway-helm-chart/values.yaml`

2. **Provide a custom values.yaml** via environment variable:
   ```bash
   export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/my-values.yaml
   ```

3. **Use Helm override values** (for manual installation):
   ```bash
   helm upgrade my-gateway-gateway ../helm/gateway-helm-chart \
     --install \
     --set gateway.controller.replicaCount=3 \
     --set gateway.router.replicaCount=3
   ```

### Helm Release Management

**Release Naming**: Helm releases are named `{gateway-name}-gateway`

For a gateway named `my-gateway`, the Helm release will be `my-gateway-gateway`.

**Viewing Releases**:
```bash
helm list -n default
```

**Manual Operations**:
kubectl delete gateway my-gateway
```

## Configuration Methods

The operator supports multiple ways to configure gateway deployments:

### 1. Chart Default Values
Simply create a Gateway and let the operator use the chart's default values:
```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: my-gateway
spec:
  gatewayClassName: default
```

### 2. Custom Values File
Provide a custom values.yaml file:
```bash
# Create custom values
cat > /path/to/my-values.yaml <<EOF
gateway:
  controller:
    replicaCount: 3
    image:
      tag: v1.0.0-m5
  router:
    replicaCount: 3
EOF

# Configure operator to use it
export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/my-values.yaml
```

### 3. Volume-Mounted Values (Docker/K8s)
```yaml
# In operator deployment
volumes:
  - name: custom-values
    configMap:
      name: gateway-operator-values
mountPath: /app/config/values.yaml
env:
  - name: GATEWAY_HELM_VALUES_FILE_PATH
    value: /app/config/values.yaml
```

## Values Configuration

When using custom values files, you can override any value from the chart's default values.yaml:

```yaml
# Example custom-values.yaml
gateway:
  controller:
    image:
      repository: wso2/api-platform/gateway-controller
      tag: v1.0.0-m5
    replicaCount: 3
    resources:
      limits:
        cpu: "2"
        memory: 1Gi
      requests:
        cpu: 500m
        memory: 512Mi
  
  router:
    image:
      repository: wso2/api-platform/gateway-router
      tag: v1.0.0-m5
    replicaCount: 3
    service:
      type: LoadBalancer
```

## Values Template

**Note**: This implementation does NOT use templates for values. The operator directly uses:
1. The Helm chart's default `values.yaml`
2. OR a custom `values.yaml` file specified via `GATEWAY_HELM_VALUES_FILE_PATH`

This approach is simpler and more flexible because:
- You can modify values directly in the chart or custom file
- No need to update operator code for new values
- Easy to volume-mount custom values in containerized environments
- Full control over all Helm chart values

## Migration from Template-Based Deployment

The values template (`gateway-helm-values.yaml.tmpl`) supports these fields:

- `.GatewayName`: Name of the gateway
- `.Replicas`: Number of replicas
2. **Values Rendering Error**
   - Not applicable - no template rendering is used
   - Custom values files must be valid YAML

3. **Release Installation Fails**ane hostname
- `.ControlPlanePort`: Control plane port
- `.ControlPlaneToken`: Control plane token
- `.StorageType`: Storage type (sqlite, postgres)
- `.StorageSQLitePath`: SQLite database path
- `.Resources`: Controller resource requirements
- `.RouterResources`: Router resource requirements

## Migration from Template-Based Deployment

To migrate from template-based to Helm-based deployment:

1. **Update Configuration**:
   ```bash
   export GATEWAY_USE_HELM=true
   ```

2. **Restart Operator**:
   ```bash
   kubectl rollout restart deployment gateway-operator -n gateway-operator-system
   ```

3. **Existing Gateways**: 
   - Existing gateways deployed with templates will continue to work
   - On the next reconciliation, they will be migrated to Helm
   - The operator will create a Helm release and adopt the existing resources

4. **Rollback Option**:
   - Set `GATEWAY_USE_HELM=false` to revert to template-based deployment
   - Useful for debugging or compatibility issues

## Troubleshooting

### Check Helm Release Status

```bash
# List releases
helm list -n <namespace>

# Get release details
helm get all <release-name> -n <namespace>

# View release history
helm history <release-name> -n <namespace>
```

### Check Operator Logs

```bash
kubectl logs -n gateway-operator-system deployment/gateway-operator -f
```

Look for messages like:
- `"Deploying gateway using Helm"`
- `"Successfully deployed gateway with Helm"`
- `"Uninstalling Helm release"`

### Common Issues

1. **Chart Not Found**
   - Verify `GATEWAY_HELM_CHART_PATH` points to a valid Helm chart directory
   - Check that Chart.yaml exists in the chart directory

2. **Values Rendering Error**
   - Check `GATEWAY_HELM_VALUES_TEMPLATE_PATH` is correct
   - Verify template syntax is valid Go template syntax
   - Check operator logs for template parsing errors

3. **Release Installation Fails**
   - Check Helm release status: `helm status <release-name> -n <namespace>`
   - View Helm logs for installation errors
   - Verify chart values are valid

4. **Upgrade Conflicts**
   - If a release is in a failed state, you may need to manually intervene:
     ```bash
     helm rollback <release-name> -n <namespace>
     # or
     helm uninstall <release-name> -n <namespace>
     ```

## Development

### Adding Helm Dependencies

The operator requires these Go modules:
- `helm.sh/helm/v3` - Helm Go SDK
- `k8s.io/cli-runtime` - Kubernetes CLI runtime for Helm

Update dependencies:
```bash
cd kubernetes/gateway-operator
go mod tidy
```

### Testing

Test Helm deployment:

```bash
# Build and run the operator
make run

# In another terminal, create a gateway
kubectl apply -f config/samples/gateway_v1_gateway.yaml

# Check Helm releases
helm list -A

# Check gateway status
kubectl get gateway -A
```

### Custom Values Template
### Custom Values Template

Create a custom values file:

1. Copy the chart's default values:
   ```bash
   cp ../helm/gateway-helm-chart/values.yaml my-values.yaml
   ```

2. Modify as needed

3. Configure the operator:
   ```bash
   export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/my-values.yaml
   ```

4. Restart operator

## Future Enhancements
Potential future improvements:

1. **Remote Chart Repositories**: Support installing charts from Helm repositories
2. **Chart Hooks**: Leverage Helm hooks for lifecycle management
3. **Values Override**: Allow users to provide custom values in Gateway
4. **Multi-Chart Support**: Support installing multiple charts for complex deployments
5. **Chart Testing**: Integrate Helm chart testing into CI/CD pipeline

## References

- [Helm Documentation](https://helm.sh/docs/)
- [Helm Go SDK](https://pkg.go.dev/helm.sh/helm/v3)
- [Gateway Helm Chart](../helm/gateway-helm-chart/README.md)
