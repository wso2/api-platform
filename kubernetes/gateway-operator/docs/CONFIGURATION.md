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
          value: "gateway-control-plane:8443"
        - name: GATEWAY_CONTROLPLANE_TOKEN
          valueFrom:
            secretKeyRef:
              name: gateway-credentials
              key: token
        - name: APIP_GW_GATEWAY__CONTROLLER_STORAGE_TYPE
          value: "sqlite"
        - name: APIP_GW_GATEWAY__CONTROLLER_STORAGE_SQLITE_PATH
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
| `GATEWAY_CONTROLPLANE_HOST` | - | `host.docker.internal:8443` | Gateway control plane host address |
| `GATEWAY_CONTROLPLANE_TOKEN` | - | `""` | Authentication token for control plane |
| `APIP_GW_GATEWAY__CONTROLLER_STORAGE_TYPE` | - | `sqlite` | Storage backend type |
| `APIP_GW_GATEWAY__CONTROLLER_STORAGE_SQLITE_PATH` | - | `./data/gateway.db` | SQLite database file path |
| `GATEWAY_DEFAULT_IMAGE` | - | `wso2/gateway-controller:latest` | Default gateway controller image |
| `GATEWAY_ROUTER_IMAGE` | - | `envoyproxy/envoy:v1.28-latest` | Default router/proxy image |

### Logging Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

## Usage Example

### Apply Gateway Configuration

Once the operator is running with proper configuration, create an `APIGateway` resource:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: my-gateway
  namespace: default
spec:
  foo: "example"
```

Apply it:

```bash
kubectl apply -f config/samples/api_v1_apigateway.yaml
```

The operator will:
1. Read the configured manifest path from `GATEWAY_MANIFEST_PATH`
2. Parse the YAML file at that location
3. Apply all Kubernetes resources defined in the manifest
4. Set the `Gateway` resource as the owner for proper lifecycle management

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

## Kubernetes Gateway API

The operator reconciles standard **Gateway** and **HTTPRoute** resources (`gateway.networking.k8s.io`) as well as the custom `APIGateway` and `RestApi` CRDs.

### Prerequisites

- **Gateway API CRDs:** The operator Helm chart can install Gateway API **v1.3.0** standard-channel CRDs when **`gatewayApi.installStandardCRDs=true`**. The default is **`false`** so installs do not conflict with Gateway API already on the cluster (duplicate CRD apply often fails with server-side apply conflicts). WSO2 CRDs in **`crds/`** always install with the chart. For non-Helm installs, apply the official [Gateway API](https://github.com/kubernetes-sigs/gateway-api) manifests if needed.
- **cert-manager:** If Helm values use **`gateway.controller.tls.certificateProvider: cert-manager`** (default in bundled gateway chart values and the [Gateway API demo](../../helm/resources/gateway-api-operator-demo/README.md)), install cert-manager so **Issuers** and **Certificates** reconcile. Without it, Helm fails with unknown kinds `Issuer` / `Certificate`.
- Create a **GatewayClass** whose name matches the operator configuration (default managed class: `wso2-api-platform`).

### `gateway_api` configuration

| Setting | Description |
| -------- | ------------- |
| `gateway_api.gateway_class_names` | Only **Gateway** objects whose `spec.gatewayClassName` is in this list are managed. **HTTPRoute** objects are processed when their parent **Gateway** uses one of these classes. |

In `config.yaml`:

```yaml
gateway_api:
  gateway_class_names:
    - wso2-api-platform
```

Environment variable (comma-separated list, overrides the config file list):

`GATEWAY_API_GATEWAY_CLASS_NAMES` — example: `wso2-api-platform,my-class`

### Annotations on `Gateway`

| Annotation | Purpose |
| ---------- | ------- |
| `gateway.api-platform.wso2.com/helm-values-configmap` | Name of a ConfigMap in the Gateway namespace with a `values.yaml` key (Helm values), same idea as `APIGateway.spec.configRef`. |
| `gateway.api-platform.wso2.com/api-selector` | Optional JSON for `APISelector` (same structure as on `APIGateway`) controlling which `RestApi` CRs are associated with this deployment. |
| `gateway.api-platform.wso2.com/control-plane-host` | Optional; stored on the gateway registry entry for control plane connectivity. |

### Annotations on `HTTPRoute`

| Annotation | Purpose |
| ---------- | ------- |
| `gateway.api-platform.wso2.com/api-version` | Version field in the generated REST payload (default `v1.0`). |
| `gateway.api-platform.wso2.com/context` | API base context; default **`/`** when omitted or whitespace-only. |
| `gateway.api-platform.wso2.com/display-name` | Overrides API display name. |
| `gateway.api-platform.wso2.com/api-handle` | REST API handle for `POST`/`PUT`/`DELETE` `/api/management/v0.9/rest-apis/{handle}` (default: `{namespace}-{name}`). |

**`APIPolicy` CR (`gateway.api-platform.wso2.com/v1alpha1`, plural `apipolicies`):** Recommended for Gateway API flows. **`spec.policies`** is a non-empty list of policy instances. **API-level** attachment: set **`spec.targetRef`** to the **`HTTPRoute`** (`group: gateway.networking.k8s.io`, `kind: HTTPRoute`); all entries are merged into **`APIConfigData.policies`**. **Rule / resource-level** attachment: omit **`spec.targetRef`** and reference the object from **`spec.rules[].filters`** with `type: ExtensionRef`, `group: gateway.api-platform.wso2.com`, `kind: APIPolicy`, `name: <metadata.name>`. If **`targetRef`** is set and the policy is also referenced from a rule, **`targetRef`** must still match that HTTPRoute for the ExtensionRef path. Policy **`params`** may embed **`valueFrom`** using the same shape as PodSpec `env[].valueFrom` — **exactly one** of **`secretKeyRef`** or **`configMapKeyRef`** with `{name, key, namespace?}`; the operator resolves these from **Secrets** (via `Secret.data[key]`) or **ConfigMaps** (via `data[key]` / `binaryData[key]`) before calling gateway-controller and watches both **`Secret`** and **`ConfigMap`** in addition to **`APIPolicy`** to re-reconcile the route when referenced values change. **`APIPolicy` does not apply to `RestApi` / `APIGateway` reconciliation.**

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
    "controlPlaneHost": "gateway:8443",
    "storageType": "sqlite"
}
```
