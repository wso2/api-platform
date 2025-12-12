# Runtime Configuration Guide

This operator supports runtime configuration through multiple methods, in order of precedence:

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Default values** (lowest priority)

## Configuration Methods

### 1. Command-Line Flags

You can override configuration using flags when starting the operator:

```bash
./manager \
  --gateway-manifest-path=/path/to/manifests.yaml \
  --metrics-bind-address=:8080 \
  --health-probe-bind-address=:8081 \
  --leader-elect
```

Available flags:
- `--gateway-manifest-path`: Path to the Kubernetes manifest file for gateway resources
- `--metrics-bind-address`: Address for metrics endpoint (default: `:8080`)
- `--health-probe-bind-address`: Address for health probe endpoint (default: `:8081`)
- `--leader-elect`: Enable leader election for HA deployments
- `--metrics-secure`: Enable secure metrics serving
- `--enable-http2`: Enable HTTP/2 for metrics and webhook servers

### 2. Environment Variables

Set environment variables in the operator deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: GATEWAY_MANIFEST_PATH
          value: "internal/controller/resources/api-platform-gateway-k8s-manifests.yaml"
        - name: GATEWAY_CONTROLPLANE_HOST
          value: "gateway-control-plane:9243"
        - name: GATEWAY_CONTROLPLANE_TOKEN
          valueFrom:
            secretKeyRef:
              name: gateway-credentials
              key: token
        - name: GATEWAY_STORAGE_TYPE
          value: "sqlite"
        - name: GATEWAY_STORAGE_SQLITE_PATH
          value: "./data/gateway.db"
        - name: GATEWAY_DEFAULT_IMAGE
          value: "wso2/gateway-controller:latest"
        - name: GATEWAY_ROUTER_IMAGE
          value: "envoyproxy/envoy:v1.28-latest"
        - name: LOG_LEVEL
          value: "info"
```

### 3. Using ConfigMap (Recommended for Kubernetes)

Create a ConfigMap and reference it in the deployment:

```bash
kubectl apply -f config/samples/operator-config.yaml
```

Then update the deployment to use the ConfigMap:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        envFrom:
        - configMapRef:
            name: operator-config
```

## Configuration Options

### Gateway Configuration

| Environment Variable | Flag | Default | Description |
|---------------------|------|---------|-------------|
| `GATEWAY_MANIFEST_PATH` | `--gateway-manifest-path` | `internal/controller/resources/api-platform-gateway-k8s-manifests.yaml` | Path to gateway Kubernetes manifest file |
| `GATEWAY_CONTROLPLANE_HOST` | - | `host.docker.internal:9243` | Gateway control plane host address |
| `GATEWAY_CONTROLPLANE_TOKEN` | - | `""` | Authentication token for control plane |
| `GATEWAY_STORAGE_TYPE` | - | `sqlite` | Storage backend type |
| `GATEWAY_STORAGE_SQLITE_PATH` | - | `./data/gateway.db` | SQLite database file path |
| `GATEWAY_DEFAULT_IMAGE` | - | `wso2/gateway-controller:latest` | Default gateway controller image |
| `GATEWAY_ROUTER_IMAGE` | - | `envoyproxy/envoy:v1.28-latest` | Default router/proxy image |

### Logging Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

## Usage Example

### Apply Gateway Configuration

Once the operator is running with proper configuration, create a `GatewayConfiguration` resource:

```yaml
apiVersion: api.api-platform.wso2.com/v1
kind: GatewayConfiguration
metadata:
  name: my-gateway
  namespace: default
spec:
  foo: "example"
```

Apply it:

```bash
kubectl apply -f config/samples/api_v1_gatewayconfiguration.yaml
```

The operator will:
1. Read the configured manifest path from `GATEWAY_MANIFEST_PATH`
2. Parse the YAML file at that location
3. Apply all Kubernetes resources defined in the manifest
4. Set the `GatewayConfiguration` resource as the owner for proper lifecycle management

### Custom Manifest Path

To use a different manifest file:

```bash
# Using environment variable
export GATEWAY_MANIFEST_PATH=/custom/path/gateway-manifests.yaml
./manager

# Or using flag
./manager --gateway-manifest-path=/custom/path/gateway-manifests.yaml
```

### Storing Sensitive Configuration

For sensitive values like tokens, use Kubernetes Secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gateway-credentials
  namespace: gateway-operator-system
type: Opaque
stringData:
  token: "your-secret-token-here"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: GATEWAY_CONTROLPLANE_TOKEN
          valueFrom:
            secretKeyRef:
              name: gateway-credentials
              key: token
```

## Programmatic Usage

The manifest applier can be used programmatically in controllers:

```go
import (
    "github.com/wso2/api-platform/kubernetes/gateway-operator/internal/config"
    "github.com/wso2/api-platform/kubernetes/gateway-operator/internal/k8sutil"
)

// In your reconciler
func (r *YourReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Create manifest applier
    applier := k8sutil.NewManifestApplier(r.Client, r.Scheme)
    
    // Apply manifest from configured path
    manifestPath := r.Config.Gateway.ManifestPath
    namespace := "default"
    
    // owner can be any Kubernetes object (for setting owner references)
    err := applier.ApplyManifestFile(ctx, manifestPath, namespace, owner)
    if err != nil {
        return ctrl.Result{}, err
    }
    
    return ctrl.Result{}, nil
}
```

## Validation

The operator validates configuration on startup:
- Manifest file path must exist and be readable
- Log level must be one of: debug, info, warn, error
- Concurrent reconciles must be >= 1

If validation fails, the operator will log an error and exit with code 1.

## Best Practices

1. **Use ConfigMaps for non-sensitive configuration** - Easy to update without rebuilding
2. **Use Secrets for sensitive data** - Tokens, passwords, API keys
3. **Use absolute paths for manifest files** - Ensures consistency across environments
4. **Set appropriate log levels** - Use `debug` for development, `info` for production
5. **Enable leader election in production** - For high availability deployments

## Troubleshooting

### Manifest file not found

If you see:
```
ERROR unable to load configuration {"error": "manifest file not found at /path/to/file: ..."}
```

Solution:
- Verify the file exists at the specified path
- Use an absolute path or ensure the relative path is correct from the operator's working directory
- Check file permissions

### Configuration not taking effect

Check precedence order:
1. Command-line flags override everything
2. Environment variables override defaults
3. Defaults are used if nothing else is specified

View loaded configuration in logs:
```
INFO Loaded operator configuration {
    "manifestPath": "/path/to/manifest.yaml",
    "controlPlaneHost": "gateway:9243",
    "storageType": "sqlite"
}
```
