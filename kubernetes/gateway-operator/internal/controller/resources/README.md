# Gateway Manifest Templating

This document explains how to use the templated manifest system for deploying gateway infrastructure.

## Overview

The gateway manifest template allows you to deploy multiple gateway instances in the same namespace without naming conflicts. Each gateway gets uniquely named resources based on the gateway name.

## Template File

Location: `internal/controller/resources/api-platform-gateway-k8s-manifests.yaml.tmpl`

The template uses Go's `text/template` syntax to generate unique resource names and configure gateway settings dynamically.

## Resource Naming Convention

All resources are prefixed with the gateway name to ensure uniqueness:

| Resource Type | Template Name Format | Example (gateway: "prod-api") |
|--------------|---------------------|------------------------------|
| Service | `{{ .GatewayName }}-gateway-controller` | `prod-api-gateway-controller` |
| Service | `{{ .GatewayName }}-router` | `prod-api-router` |
| Service | `{{ .GatewayName }}-sample-backend` | `prod-api-sample-backend` |
| Deployment | `{{ .GatewayName }}-gateway-controller` | `prod-api-gateway-controller` |
| Deployment | `{{ .GatewayName }}-router` | `prod-api-router` |
| Deployment | `{{ .GatewayName }}-sample-backend` | `prod-api-sample-backend` |
| PVC | `{{ .GatewayName }}-controller-data` | `prod-api-controller-data` |
| ConfigMap | `{{ .GatewayName }}-gateway-controller-config` | `prod-api-gateway-controller-config` |

## Template Data Structure

The template expects a `GatewayManifestTemplateData` struct with the following fields:

```go
type GatewayManifestTemplateData struct {
    // Required
    GatewayName       string                    // Unique name for the gateway
    
    // Infrastructure
    Replicas          int32                     // Number of replicas (default: 1)
    GatewayImage      string                    // Gateway controller image
    RouterImage       string                    // Router (Envoy) image
    
    // Control Plane
    ControlPlaneHost  string                    // Control plane host:port
    ControlPlaneTokenSecret *SecretReference    // Token secret reference
    
    // Storage
    StorageType       string                    // "sqlite", "postgres", "memory"
    StorageSQLitePath string                    // SQLite database path
    
    // Logging
    LogLevel          string                    // "debug", "info", "warn", "error"
    
    // Resource Management
    Resources         *ResourceRequirements     // CPU/Memory requests & limits
    NodeSelector      map[string]string         // Node selector labels
    Tolerations       []corev1.Toleration       // Pod tolerations
    Affinity          *corev1.Affinity          // Pod affinity rules
}
```

## Usage in Controller

### Step 1: Create Template Data

```go
import "github.com/wso2/api-platform/kubernetes/gateway-operator/internal/k8sutil"

// Start with defaults
data := k8sutil.NewGatewayManifestTemplateData(gatewayConfig.Name)

// Override with spec values
if gatewayConfig.Spec.Infrastructure != nil {
    infra := gatewayConfig.Spec.Infrastructure
    
    if infra.Replicas != nil {
        data.Replicas = *infra.Replicas
    }
    
    if infra.Image != "" {
        data.GatewayImage = infra.Image
    }
    
    // ... etc
}
```

### Step 2: Apply Template

```go
import "github.com/wso2/api-platform/kubernetes/gateway-operator/internal/k8sutil"

err := k8sutil.ApplyManifestTemplate(
    ctx,
    r.Client,
    r.Scheme,
    templatePath,                    // Path to .yaml.tmpl file
    gatewayConfig.Namespace,         // Namespace to deploy in
    gatewayConfig,                   // Owner reference
    data,                            // Template data
)
```

### Complete Example

See `internal/k8sutil/examples/template_usage.go` for a full working example.

## Configuration

Configure the template path via:

1. **Environment Variable**: `GATEWAY_MANIFEST_TEMPLATE_PATH`
2. **Config struct**: `OperatorConfig.Gateway.ManifestTemplatePath`
3. **Default**: `internal/controller/resources/api-platform-gateway-k8s-manifests.yaml.tmpl`

## Multi-Gateway Deployment Example

Deploy three gateways in the same namespace:

### Gateway 1: Production API Gateway

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: prod-api
  namespace: api-gateway
spec:
  infrastructure:
    replicas: 3
    image: wso2/gateway-controller:v1.0.0
    routerImage: wso2/gateway-router:v1.0.0
    resources:
      requests:
        cpu: "1000m"
        memory: "2Gi"
      limits:
        cpu: "2000m"
        memory: "4Gi"
```

Resources created:
- `prod-api-gateway-controller` (Service, Deployment)
- `prod-api-router` (Service, Deployment)
- `prod-api-sample-backend` (Service, Deployment)
- `prod-api-controller-data` (PVC)
- `prod-api-gateway-controller-config` (ConfigMap)

### Gateway 2: Staging API Gateway

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: staging-api
  namespace: api-gateway
spec:
  infrastructure:
    replicas: 2
    image: wso2/gateway-controller:v1.0.0-rc1
```

Resources created:
- `staging-api-gateway-controller` (Service, Deployment)
- `staging-api-router` (Service, Deployment)
- `staging-api-sample-backend` (Service, Deployment)
- `staging-api-controller-data` (PVC)
- `staging-api-gateway-controller-config` (ConfigMap)

### Gateway 3: Dev API Gateway

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: Gateway
metadata:
  name: dev-api
  namespace: api-gateway
spec:
  infrastructure:
    replicas: 1
```

Resources created:
- `dev-api-gateway-controller` (Service, Deployment)
- `dev-api-router` (Service, Deployment)
- `dev-api-sample-backend` (Service, Deployment)
- `dev-api-controller-data` (PVC)
- `dev-api-gateway-controller-config` (ConfigMap)

## Key Features

✅ **Unique Resource Names**: Each gateway gets distinct resource names  
✅ **No Naming Conflicts**: Multiple gateways can coexist in the same namespace  
✅ **Dynamic Configuration**: Template renders based on Gateway spec  
✅ **Owner References**: All resources owned by the Gateway  
✅ **Consistent Labeling**: All resources labeled with `app.kubernetes.io/name: {{ .GatewayName }}`  
✅ **Service Discovery**: Router can find its controller via `{{ .GatewayName }}-gateway-controller`  

## Template Variables Reference

| Variable | Type | Example | Description |
|----------|------|---------|-------------|
| `.GatewayName` | string | `"prod-api"` | Gateway identifier |
| `.Replicas` | int32 | `3` | Number of replicas |
| `.GatewayImage` | string | `"wso2/gateway-controller:latest"` | Controller image |
| `.RouterImage` | string | `"wso2/gateway-router:latest"` | Router image |
| `.ControlPlaneHost` | string | `"cp.example.com:8443"` | Control plane endpoint |
| `.ControlPlaneTokenSecret` | object | See below | Token secret reference |
| `.LogLevel` | string | `"info"` | Logging level |
| `.StorageType` | string | `"sqlite"` | Storage backend type |
| `.StorageSQLitePath` | string | `"./data/gateway.db"` | SQLite DB path |
| `.Resources` | object | See below | Resource requests/limits |
| `.NodeSelector` | map | `{"tier": "frontend"}` | Node selector |
| `.Tolerations` | array | See below | Pod tolerations |
| `.Affinity` | object | See below | Pod affinity |

### ControlPlaneTokenSecret Structure

```go
{
    Name: "cp-token",
    Key:  "token"
}
```

Template usage:
```yaml
{{- if .ControlPlaneTokenSecret }}
valueFrom:
  secretKeyRef:
    name: {{ .ControlPlaneTokenSecret.Name }}
    key: {{ .ControlPlaneTokenSecret.Key }}
{{- else }}
value: ""
{{- end }}
```

### Resources Structure

```go
{
    Requests: {
        CPU:    "500m",
        Memory: "1Gi"
    },
    Limits: {
        CPU:    "1000m",
        Memory: "2Gi"
    }
}
```

Template usage:
```yaml
{{- if .Resources }}
resources:
  {{- if .Resources.Requests }}
  requests:
    cpu: {{ .Resources.Requests.CPU }}
    memory: {{ .Resources.Requests.Memory }}
  {{- end }}
  {{- if .Resources.Limits }}
  limits:
    cpu: {{ .Resources.Limits.CPU }}
    memory: {{ .Resources.Limits.Memory }}
  {{- end }}
{{- end }}
```

## Best Practices

1. **Gateway Naming**: Use descriptive names like `prod-api`, `staging-api`, `dev-api`
2. **Resource Limits**: Always set resource limits in production
3. **High Availability**: Use replicas >= 3 for production gateways
4. **Secrets**: Use `ControlPlaneTokenSecret` instead of hardcoded tokens
5. **Node Placement**: Use `NodeSelector` and `Affinity` for critical gateways
6. **Logging**: Use `"info"` in production, `"debug"` for troubleshooting

## Troubleshooting

### Problem: Resources Not Created

**Check**: Verify template path is correct
```bash
echo $GATEWAY_MANIFEST_TEMPLATE_PATH
```

**Solution**: Set the environment variable or use default path

### Problem: Naming Conflicts

**Check**: Verify each gateway has a unique name
```bash
kubectl get gateway -A
```

**Solution**: Ensure `metadata.name` is unique per namespace

### Problem: Template Rendering Errors

**Check**: Controller logs for template errors
```bash
kubectl logs -n gateway-operator-system deployment/gateway-operator-controller-manager
```

**Solution**: Verify template data structure matches expected format

## See Also

- [Gateway CRD Spec](../../docs/GATEWAY_API_SPEC.md)
- [Quick Reference](../../docs/QUICK_REFERENCE.md)
- [Example Usage](examples/template_usage.go)
