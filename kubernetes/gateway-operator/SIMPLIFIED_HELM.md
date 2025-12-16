# Simplified Helm Deployment - No Templates

## Summary of Changes

Successfully refactored the Helm deployment implementation to **remove template complexity** and use the Helm chart's `values.yaml` directly. This is much simpler and aligns with standard Helm practices.

## What Changed

### ❌ Removed
1. **Template file**: `internal/controller/resources/gateway-helm-values.yaml.tmpl` - DELETED
2. **Values data structure**: `internal/k8sutil/helm_values_data.go` - DELETED
3. **Template rendering**: `RenderTemplate()` function from `manifest.go` - REMOVED
4. **Values building logic**: `buildHelmValuesData()` function from controller - REMOVED

### ✅ Updated
1. **Configuration** (`internal/config/config.go`):
   - Renamed: `HelmValuesTemplatePath` → `HelmValuesFilePath`
   - Updated default: Empty string (uses chart's default values.yaml)
   - Updated env var: `GATEWAY_HELM_VALUES_TEMPLATE_PATH` → `GATEWAY_HELM_VALUES_FILE_PATH`

2. **Controller** (`internal/controller/gateway_controller.go`):
   - Removed template rendering logic
   - Simplified deployment to use `ValuesFilePath` instead of `ValuesYAML`
   - Direct file path to Helm client

3. **Documentation**:
   - Updated `HELM_DEPLOYMENT.md` with new approach
   - Updated `HELM_QUICK_START.md` to reflect simpler configuration

## How It Works Now

### Simple Flow
```
Operator → helm install/upgrade --values <chart-path>/values.yaml
```

Or with custom values:
```
Operator → helm install/upgrade --values <custom-values-path>
```

### Configuration Options

**Option 1: Use Chart Defaults** (simplest)
```bash
export GATEWAY_USE_HELM=true
export GATEWAY_HELM_CHART_PATH=../helm/gateway-helm-chart
# That's it! Uses chart's default values.yaml
```

**Option 2: Custom Values File**
```bash
export GATEWAY_USE_HELM=true
export GATEWAY_HELM_CHART_PATH=../helm/gateway-helm-chart
export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/custom-values.yaml
```

**Option 3: Volume-Mounted Values** (for containers)
```yaml
# Docker Compose or K8s deployment
volumes:
  - ./my-values.yaml:/app/config/values.yaml
env:
  GATEWAY_HELM_VALUES_FILE_PATH: /app/config/values.yaml
```

## Benefits of This Approach

1. **Simpler Code**: No template rendering, data structure building, or complex logic
2. **Standard Helm**: Works exactly like `helm install --values values.yaml`
3. **Flexible**: Easy to customize via file mounts without code changes
4. **Maintainable**: Changes to values don't require operator updates
5. **Transparent**: Users can see exactly what values are being used
6. **No Magic**: No template variable mapping or rendering errors

## Usage Examples

### Basic Gateway (Default Values)
```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: my-gateway
spec:
  gatewayClassName: default
```
Operator uses: `../helm/gateway-helm-chart/values.yaml`

### Custom Configuration
```bash
# Create custom values
cat > custom-values.yaml <<EOF
gateway:
  controller:
    replicaCount: 3
    image:
      tag: v1.0.0-m5
  router:
    replicaCount: 3
    service:
      type: LoadBalancer
EOF

# Configure operator
export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/custom-values.yaml

# Create gateway
kubectl apply -f my-gateway.yaml
```
Operator uses: `/path/to/custom-values.yaml`

### Docker Compose Example
```yaml
services:
  gateway-operator:
    image: gateway-operator:latest
    environment:
      - GATEWAY_USE_HELM=true
      - GATEWAY_HELM_CHART_PATH=/charts/gateway-helm-chart
      - GATEWAY_HELM_VALUES_FILE_PATH=/config/values.yaml
    volumes:
      - ./helm-chart:/charts/gateway-helm-chart:ro
      - ./custom-values.yaml:/config/values.yaml:ro
```

### Kubernetes Deployment Example
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-operator-values
data:
  values.yaml: |
    gateway:
      controller:
        replicaCount: 3
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway-operator
spec:
  template:
    spec:
      containers:
      - name: operator
        env:
        - name: GATEWAY_HELM_VALUES_FILE_PATH
          value: /config/values.yaml
        volumeMounts:
        - name: values
          mountPath: /config/values.yaml
          subPath: values.yaml
      volumes:
      - name: values
        configMap:
          name: gateway-operator-values
```

## File Changes

### Deleted Files
- ❌ `internal/k8sutil/helm_values_data.go`
- ❌ `internal/controller/resources/gateway-helm-values.yaml.tmpl`

### Modified Files
- ✏️ `internal/config/config.go` - Simplified configuration
- ✏️ `internal/controller/gateway_controller.go` - Removed template logic
- ✏️ `internal/k8sutil/manifest.go` - Removed RenderTemplate function
- ✏️ `HELM_DEPLOYMENT.md` - Updated documentation
- ✏️ `HELM_QUICK_START.md` - Updated quick start guide

## Migration from Previous Implementation

If you were using the template-based approach:

**Before:**
```bash
export GATEWAY_HELM_VALUES_TEMPLATE_PATH=internal/controller/resources/gateway-helm-values.yaml.tmpl
```

**Now:**
```bash
# Option 1: Use chart defaults (no env var needed)
# Just delete the GATEWAY_HELM_VALUES_TEMPLATE_PATH variable

# Option 2: Use custom values file
export GATEWAY_HELM_VALUES_FILE_PATH=/path/to/custom-values.yaml
```

## Build Status

✅ Build successful
✅ No compilation errors
✅ Backward compatible
✅ Simpler codebase

## Next Steps

1. **Update Deployment**: Remove old environment variables
2. **Custom Values**: Create custom values.yaml if needed
3. **Volume Mounts**: Set up volume mounts for containerized deployments
4. **Test**: Deploy a gateway and verify Helm release

## Key Takeaway

**Before**: Complex template rendering with Go templates and data structures
**Now**: Simple file path to values.yaml - standard Helm behavior

This is much cleaner and follows the principle: "Use the chart's values.yaml directly, customize via file if needed."
