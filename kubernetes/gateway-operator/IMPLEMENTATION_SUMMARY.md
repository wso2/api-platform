# Runtime Configuration Implementation Summary

## Overview

This implementation provides a flexible, production-ready runtime configuration system for the API Platform Gateway Operator. The manifest file path is now configurable, and there's a reusable utility to apply Kubernetes manifests dynamically.

## What Was Implemented

### 1. Configuration Package (`internal/config/config.go`)

A centralized configuration management system with:
- **Hierarchical configuration loading** (flags > env vars > defaults)
- **Structured configuration** for gateway, reconciliation, and logging settings
- **Automatic validation** on load
- **Environment variable support** for all key settings

Key features:
```go
cfg, err := config.LoadConfig()
manifestPath := cfg.Gateway.ManifestPath
```

### 2. Manifest Applier Utility (`internal/k8sutil/manifest.go`)

A reusable Kubernetes manifest applier that:
- **Reads YAML manifests** from configurable file paths
- **Applies resources** to the cluster (create or update)
- **Sets owner references** for proper lifecycle management
- **Handles multiple resources** in a single file
- **Provides deletion support** for cleanup

Key features:
```go
applier := k8sutil.NewManifestApplier(client, scheme)
err := applier.ApplyManifestFile(ctx, manifestPath, namespace, owner)
```

### 3. Controller Integration

Both controllers updated to:
- Accept configuration through the `Config` field
- Use the manifest applier to deploy resources
- Log configuration on startup
- Handle resources with proper ownership

**GatewayConfigurationReconciler** now applies the gateway manifest when a `GatewayConfiguration` resource is created.

### 4. Main Entry Point Updates (`cmd/main.go`)

- Loads configuration on startup
- Supports CLI flags for overrides
- Validates configuration before proceeding
- Passes configuration to all controllers
- Logs loaded configuration for visibility

### 5. Documentation & Examples

- **`docs/CONFIGURATION.md`**: Complete configuration guide
- **`config/samples/operator-config.yaml`**: Sample ConfigMap
- **`internal/k8sutil/manifest_example_test.go`**: Usage examples

## How to Use

### Method 1: Environment Variables (Recommended for Kubernetes)

```yaml
env:
- name: GATEWAY_MANIFEST_PATH
  value: "/path/to/manifest.yaml"
- name: GATEWAY_CONTROLPLANE_HOST
  value: "gateway:9243"
```

### Method 2: Command-Line Flags

```bash
./manager --gateway-manifest-path=/custom/manifests.yaml
```

### Method 3: ConfigMap

```bash
kubectl apply -f config/samples/operator-config.yaml
```

Then reference in deployment:
```yaml
envFrom:
- configMapRef:
    name: operator-config
```

## Key Benefits

✅ **No hardcoded paths** - manifest file location is fully configurable
✅ **Flexible deployment** - works in different environments (dev/staging/prod)
✅ **Reusable utility** - `ManifestApplier` can be used anywhere in the codebase
✅ **Proper ownership** - resources are owned by CRs for automatic cleanup
✅ **Production ready** - includes validation, error handling, and logging
✅ **Cloud-native** - follows Kubernetes operator best practices

## Configuration Precedence

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Default values** (lowest priority)

## Validation

The configuration is validated on startup:
- Manifest file must exist and be readable
- Log level must be valid (debug/info/warn/error)
- Paths are converted to absolute paths automatically

## Next Steps

To use this in production:

1. **Update the deployment** to include your environment variables:
   ```yaml
   env:
   - name: GATEWAY_MANIFEST_PATH
     value: "internal/controller/resources/api-platform-gateway-k8s-manifests.yaml"
   ```

2. **Create a GatewayConfiguration** resource:
   ```bash
   kubectl apply -f config/samples/api_v1_gatewayconfiguration.yaml
   ```

3. **The operator will automatically**:
   - Read the configured manifest path
   - Parse and apply all resources in that file
   - Set proper owner references
   - Log the operation

## Example Usage in Code

```go
// In any reconciler
func (r *YourReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Access configuration
    manifestPath := r.Config.Gateway.ManifestPath
    
    // Create applier
    applier := k8sutil.NewManifestApplier(r.Client, r.Scheme)
    
    // Apply your manifest
    err := applier.ApplyManifestFile(ctx, manifestPath, "default", owner)
    
    return ctrl.Result{}, err
}
```

## Files Created/Modified

**Created:**
- `internal/config/config.go` - Configuration package
- `internal/k8sutil/manifest.go` - Manifest applier utility
- `internal/k8sutil/manifest_example_test.go` - Usage examples
- `config/samples/operator-config.yaml` - Sample ConfigMap
- `docs/CONFIGURATION.md` - Complete documentation

**Modified:**
- `cmd/main.go` - Added config loading
- `internal/controller/gatewayconfiguration_controller.go` - Uses manifest applier
- `internal/controller/apiconfiguration_controller.go` - Added config field

All code compiles successfully with `go build ./...`
