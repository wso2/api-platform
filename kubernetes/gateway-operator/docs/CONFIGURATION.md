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
        # Gateway storage (and all other gateway config) is set through the gateway Helm
        # values (gateway.config.controller.storage.*), which the operator renders into the
        # gateway's config.toml — not via operator env.
        - name: GATEWAY_DEFAULT_IMAGE
          value: "wso2/gateway-controller:latest"
        - name: GATEWAY_ROUTER_IMAGE
          value: "envoyproxy/envoy:v1.38-latest"
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
| `GATEWAY_DEFAULT_IMAGE` | - | `wso2/gateway-controller:latest` | Default gateway controller image |
| `GATEWAY_ROUTER_IMAGE` | - | `envoyproxy/envoy:v1.38-latest` | Default router/proxy image |

> **Gateway-internal configuration** (storage, control-plane sync, logging, encryption,
> policies, etc.) is **not** set via operator environment variables. It is configured through
> the gateway Helm values (`gateway.config.*`), which the operator renders into the gateway's
> `config.toml`. Secret/runtime values (control-plane token, DB passwords) are injected into
> the gateway container as env and read back by `config.toml` `{{ env "NAME" "default" }}`
> interpolation tokens at startup. The legacy `APIP_GW_*` koanf override layer has been removed,
> so plain gateway-container env vars no longer override config.
>
> At-rest encryption is **mandatory**: every gateway requires an AES-256 key secret
> (`gateway.controller.encryptionKeys`) in its namespace, and the controller fails to start
> without it.

### At-rest Encryption Key (Required)

Every operator-managed gateway needs an AES-256 key secret in the **Gateway's own namespace**
(the operator deploys the gateway release into that namespace). Provision it before the Gateway
is reconciled, then enable `gateway.controller.encryptionKeys` in the gateway Helm values
(directly in `gateway_values.yaml`, or via the per-Gateway values ConfigMap):

```bash
openssl rand 32 > default-aesgcm256-v1.bin
kubectl create secret generic gateway-encryption-keys \
  --from-file=default-aesgcm256-v1.bin=default-aesgcm256-v1.bin \
  -n <gateway-namespace>
```

```yaml
gateway:
  controller:
    encryptionKeys:
      enabled: true
      secretName: gateway-encryption-keys
```

The chart fails to render if `encryptionKeys` is not enabled.

### Gateway Deletion Safety

| Config key | Environment Variable | Default | Description |
|------------|---------------------|---------|-------------|
| `gateway.deletion_grace_period` | `GATEWAY_DELETION_GRACE_PERIOD` | `0s` | How long a deleted APIGateway's gateway release keeps running before it is uninstalled. While waiting, the APIGateway reports a `DeletionPending` condition with the uninstall time. `0s` uninstalls at once. |
| `gateway.retain_volume_on_delete` | `GATEWAY_RETAIN_VOLUME_ON_DELETE` | `false` | Annotate each gateway controller's PersistentVolumeClaim with `helm.sh/resource-policy: keep` so it survives the uninstall. Applies from the next install or upgrade of each gateway. |

The annotation `gateway.api-platform.wso2.com/delete-immediately: "true"` on an APIGateway skips
the grace period for that gateway. In the operator Helm chart these are `gateway.deletionGracePeriod`
and `gateway.retainVolumeOnDelete`; the chart README's "Gateway Deletion Safety" section describes
how to recover a gateway deleted by mistake.

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
| `gateway_api.gateway_class_names` | Only **Gateway** objects whose `spec.gatewayClassName` is in this list are managed. An **HTTPRoute** is deployed to each of its parent **Gateways** that uses one of these classes. |

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
| `gateway.api-platform.wso2.com/api-handle` | REST API handle for `POST`/`PUT`/`DELETE` `/api/management/v0.9/rest-apis/{handle}` (default: `{namespace}-{name}`). The gateway-controller rejects a second API with the same display name and version, so changing only the handle fails with `DeploymentFailed` while the old API keeps serving, as in earlier releases. Change `display-name` or `api-version` with it: on a route with several parent Gateways the old handle is then removed from each Gateway; on a route with one parent Gateway the old API stays deployed, as in earlier releases. |

The operator tracks where a route is deployed with `last-deployed-parent-gateway` / `last-deployed-config-hash` (route on one Gateway, unchanged from earlier releases) or `deployed-gateways` (route on several Gateways), all under `gateway.api-platform.wso2.com/`. Don't set or edit these; they are excluded from the deployed payload.

### HTTPRoutes with several parent Gateways

An HTTPRoute whose `spec.parentRefs` list several Gateways of a managed class is deployed to each of them, as Gateway API specifies. Each parent reports its own `Accepted` condition in `status.parents`, so one unreachable gateway-controller shows `Retrying` on its parent while the others keep serving. Parents whose Gateway uses another class are left to that class's controller.

Limitations:

- `ApiKey`, `Subscription`, `SubscriptionPlan`, `Certificate` and `ManagedSecret` resources select their own gateway and exist on that one only. If the route relies on them, requests through its other Gateways fail those checks.
- The default handle `{namespace}-{name}` can collide (`a-b/c` and `a/b-c`). On a shared Gateway, set `api-handle` explicitly.

**Upgrading from an operator without this support:** earlier releases rejected these routes (`ResolvedRefs=False`, reason `Invalid`) and served them nowhere. After the upgrade they start serving on every managed Gateway they list. To review them first, list the routes the current operator rejected:

```sh
kubectl get httproutes -A -o json | jq -r '.items[]
  | select([.status.parents[]?
      | select(.controllerName == "gateway.api-platform.wso2.com/gateway-operator")
      | .conditions[]?
      | select(.reason == "Invalid"
          and (.message | test("multiple (distinct )?Gateway parentRefs")))] | length > 0)
  | "\(.metadata.namespace)/\(.metadata.name)"'
```

**Downgrading:** an older operator rejects any route whose `parentRefs` list more than one Gateway, of any class, and can't find where a multi-Gateway route is deployed: its copies keep serving with nothing managing them, and deleting the route later removes at most one of them. Before downgrading, reduce these routes to one parent Gateway (or delete them) while this operator is still running, and wait for it to reconcile them. To list them:

```sh
kubectl get httproutes -A -o json | jq -r '.items[] | .metadata.namespace as $ns
  | select([.spec.parentRefs[]? | select((.kind // "Gateway") == "Gateway") | "\(.namespace // $ns)/\(.name)"] | unique | length > 1)
  | select([.status.parents[]? | select(.controllerName == "gateway.api-platform.wso2.com/gateway-operator")] | length > 0)
  | "\(.metadata.namespace)/\(.metadata.name)"'
```

Routes on one Gateway need nothing: their annotations are the same as in earlier releases, so the older operator neither redeploys nor loses them. Upgrading again picks up the copies of multi-Gateway routes that still exist. A multi-Gateway route deleted while the older operator ran leaves its other copies with no record, so delete those from each Gateway it targeted.

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
