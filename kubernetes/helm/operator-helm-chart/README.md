# API Platform Gateway Operator Helm Chart

This Helm chart deploys the API Platform Gateway Operator, a Kubernetes operator that manages the lifecycle of API Gateway instances and the APIs deployed onto them.

## Overview

The Gateway Operator is responsible for:
- Managing `APIGateway` custom resources (gateway instances)
- Managing `RestApi` and the other API-management custom resources (Agent, ApiKey, APIPolicy, Certificate, LlmProvider, LlmProviderTemplate, LlmProxy, ManagedSecret, Mcp, Subscription, SubscriptionPlan)
- Automatically deploying and configuring gateway instances (via the gateway Helm chart)
- Reconciling gateway state with desired configuration
- Optionally reconciling Kubernetes Gateway API resources (Gateway, HTTPRoute) for managed gateway classes

All operator CRDs live in the API group **`gateway.api-platform.wso2.com`**, served at `v1alpha1` and `v1` (v1 is the storage version; the two schemas are identical and bridged with conversion `strategy: None`). `APIGateway` is also served at `v2alpha1`, which adds `spec.adminCredentials`; the operator converts between `APIGateway` versions with a conversion webhook (see [APIGateway versions and the conversion webhook](#apigateway-versions-and-the-conversion-webhook)).

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
  --set image.tag=0.12.0 \
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
| `replicaCount` | Number of operator replicas. APIGateway requests through `v1alpha1` or `v2alpha1` need a ready operator pod (conversion webhook), so consider `2` or more where those versions are used | `1` |
| `conversionWebhook.port` | Container port of the APIGateway conversion webhook (the `<fullname>-webhook` Service exposes it on 443) | `9443` |
| `conversionWebhook.maxRequestBytes` | Largest conversion request body accepted, in bytes (a list through `v1alpha1`/`v2alpha1` converts every object in one request) | `67108864` |
| `conversionWebhook.pqcKeyExchange` | Prefer the hybrid post-quantum `X25519MLKEM768` TLS key exchange on the webhook; classical groups stay as fallback | `true` |
| `watchNamespaces` | Namespaces to watch (cluster-wide if empty) | `[]` |
| `image.repository` | Operator image repository | `ghcr.io/wso2/api-platform/gateway-operator` |
| `image.tag` | Operator image tag | `0.12.0` |
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

### Gateway API (Kubernetes Gateway API) Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `gatewayApi.enabled` | Reconcile Kubernetes Gateway API resources (GatewayClass, Gateway, HTTPRoute). Set `false` where gateways are deployed only through `APIGateway`: the operator then watches no Gateway API resources, gets no Gateway API RBAC, runs without the Gateway API CRDs, and caches only the gateway chart's own Services and Deployments (by their `app.kubernetes.io/component` label; with gateway chart 2026.11.26-r1 or earlier, do not override it in the gateway's `deployment.labels`). The upgrade fails while a Gateway of a managed class, or an HTTPRoute with this operator's finalizer, still exists. Independent of `installStandardCRDs` | `true` |
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
| `cache.syncTimeout` | How long each controller waits for the informer cache's initial list at startup before the operator fails and restarts (Go duration). Raise it on clusters with many gateways if the leader logs `timed out waiting for cache to be synced` | `2m` |
| `cache.excludeHelmReleaseSecrets` | Keep Secrets labeled `owner=helm` (Helm release storage) out of the operator's cache. A reference to such a Secret resolves as not found | `true` |
| `cache.excludeGatewayChartConfigMaps` | Keep ConfigMaps labeled `app.kubernetes.io/name=gateway` (rendered by the gateway chart) out of the operator's cache. A reference to such a ConfigMap resolves as not found | `true` |

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

The chart ships all operator CRDs (group `gateway.api-platform.wso2.com`, served at `v1alpha1` and `v1`, with `v1` as the storage version; `APIGateway` is also served at `v2alpha1`) in the Helm-native `crds/` directory. Helm installs everything under `crds/` automatically on `helm install`.

> **Important — CRDs are install-only.** Helm applies `crds/` only when a CRD does not already exist, and **never updates or deletes** CRDs on `helm upgrade`, `helm uninstall`, or a re-`helm install` on a cluster where they already exist. This operator **requires the `v1` version to be served**, so:
> - **Fresh clusters** (no pre-existing `gateway.api-platform.wso2.com` CRDs) get the correct v1+v1alpha1 CRDs and work out of the box.
> - **Clusters that already have older (v1alpha1-only) CRDs** will NOT be upgraded by Helm. To move them to v1, delete the old CRDs first (`kubectl get crd -o name | grep gateway.api-platform.wso2.com | xargs kubectl delete`) then reinstall, or apply the updated CRDs manually with `kubectl apply -f crds/`.

### APIGateway versions and the conversion webhook

`APIGateway` is served at three versions:

| Version | Schema |
|---|---|
| `v1` (storage) | The released schema. |
| `v1alpha1` | Identical to `v1`. |
| `v2alpha1` | `v1` plus `spec.adminCredentials`. |

Because the versions differ, the API server converts `APIGateway` objects through the operator's conversion webhook (all other operator CRDs keep `strategy: None`):

- **What a `v1` read shows.** A gateway is always stored as `v1`. On the stored object, `spec.adminCredentials` lives in the annotation `gateway.api-platform.wso2.com/conversion-data`. A `v2alpha1` read turns it back into `spec.adminCredentials` and hides the annotation. `kubectl get apigateway` reads `v1`, so it shows the annotation. Use `kubectl get apigateways.v2alpha1.gateway.api-platform.wso2.com` to see the field. Set or change `adminCredentials` through `v2alpha1`, not by editing the annotation.
- **Older versions keep the field.** Reading and updating a gateway through `v1` or `v1alpha1` keeps its `adminCredentials`: those clients carry the annotation along. The operator re-validates the annotation and refuses to deploy a gateway whose value is invalid.
- **The webhook needs a ready operator.** Requests through `v1` never call the webhook. Requests through `v1alpha1` or `v2alpha1` fail while no operator pod is ready, so run 2 or more replicas (`replicaCount`) where clients use those versions.
- **The operator manages the webhook.** It creates and renews the webhook's TLS certificate itself (Secret `<fullname>-webhook-cert` in the release namespace: a 10-year self-signed CA and a 1-year serving certificate, renewed 90 days before expiry). No cert-manager is needed.
- **A deleted certificate Secret is recreated.** The operator notices within about a minute and recreates it, with a new CA. The pods keep serving the old certificate in the meantime. When the CA changes, the CRD trusts the old and the new CA together for 15 minutes. A pod serves a certificate from the new CA only once the CRD trusts it, so no request is rejected. It points the `APIGateway` CRD's `spec.conversion` at the `<fullname>-webhook` Service with that CA.
- **Rolling upgrades.** Every operator pod does this itself, without waiting for the leader lease. A rolling upgrade from a release without the webhook therefore completes even while an old pod still holds the lease. The Service selects only pods labelled `gateway.api-platform.wso2.com/conversion-webhook-server: "true"`, so conversion requests never reach an old pod that doesn't serve the webhook.
- **The shipped CRD carries a placeholder.** The CRD in `crds/` has a placeholder Service, because Helm does not template `crds/`. The operator replaces it within seconds of starting, and again whenever `kubectl apply`, Helm or a GitOps tool puts the placeholder back. For this it has `get`/`list`/`watch`/`patch` on that one CRD only.
- **TLS.** The webhook prefers the hybrid `X25519MLKEM768` key exchange, with classical groups as fallback (`conversionWebhook.pqcKeyExchange`). The certificates are ECDSA P-256, because the API server cannot verify post-quantum certificate signatures.

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
  # adminCredentials:                         # apiVersion v2alpha1 only; admin password from a Secret, see "Admin Credentials from a Secret"
  #   username: gw-admin
  #   passwordSecretRef: { name: gw-admin, key: password }
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

### Admin Credentials from a Secret

Every gateway's management API admin user comes from one of these, in this order:

| Source | Admin user |
|---|---|
| `spec.adminCredentials` | The username and the password in the Secret it references (below). |
| Basic-auth users in the gateway's `configRef` values, or in `gateway.values` of this chart | The first listed user. |
| Neither, on the gateway's first deployment | A password the operator generates (below). |
| Neither, for a gateway an earlier operator version deployed | The `admin`/`admin` user it was deployed with, kept so its credential does not change. The operator logs a warning and emits a `DefaultAdminCredentials` Event on each deployment; set `spec.adminCredentials` to replace it. |

The operator chart no longer ships a default `admin`/`admin` user in `gateway.values`.

To keep the admin password out of ConfigMaps and Helm values, reference a Secret with `spec.adminCredentials`. It is served by `apiVersion: gateway.api-platform.wso2.com/v2alpha1`:

```bash
kubectl create namespace gw1

kubectl create secret generic gateway-1-admin -n gw1 \
  --from-literal=password="$(openssl rand -base64 24)"

kubectl apply -f - <<EOF
apiVersion: gateway.api-platform.wso2.com/v2alpha1
kind: APIGateway
metadata:
  name: gateway-1
  namespace: gw1
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      gateway.api-platform.wso2.com/target: gateway-1
  adminCredentials:
    username: gw1-admin          # default "admin"
    passwordSecretRef:
      name: gateway-1-admin      # Secret in the APIGateway's namespace
      key: password
EOF
```

| Field | Description |
|---|---|
| `username` | Admin username. Defaults to `admin`. |
| `passwordSecretRef` | `name` and `key` of the plain-text password in a Secret in the APIGateway's namespace. |

- The operator passes only the Secret reference to the gateway chart (`gateway.config.controller.auth.basic.users[].passwordSecretRef`). The password never appears in Helm values, the gateway ConfigMap, or the Helm release.
- The gateway-controller reads the password from the Secret key mounted read-only as a file, so it never enters the pod environment. There is no option to deliver it as an environment variable.
- **Several users:** the `adminCredentials` user is listed first. Any other basic-auth users in the `configRef` values are kept after it, each with its own `password` or `passwordSecretRef`. A `configRef` user with the same username is replaced, and the operator's default `admin`/`admin` user is not kept. IDP settings in the `configRef` still apply.
- **Without `adminCredentials`:** the operator signs in as the first `configRef` user, so list a user with the `admin` role first. Admin-only resources such as LLM providers and LLM provider templates fail with 403 for a `developer` user.
- The operator fails closed: if the Secret or key is missing or empty, the gateway is not deployed and no request is sent to it with `admin`/`admin`. Creating or fixing the Secret triggers a redeploy.
- Changing the Secret rolls the gateway-controller pod, which reads the password only at startup. The same applies to a Secret referenced by a `passwordSecretRef` in the `configRef` values, with or without `adminCredentials`. (A Kubernetes Gateway API `Gateway` is not rolled: restart its controller after rotating such a Secret.)
- The API server rejects an empty `passwordSecretRef.name` or `key`, a `key` containing `..`, and a `username` containing `:` or control characters.
- Trailing spaces, tabs and newlines are trimmed from the Secret value, as the gateway trims the mounted file, so a Secret created with `--from-file` works.
- `adminCredentials` is declared by `v2alpha1` only; `v1` and `v1alpha1` keep their released schemas, so `kubectl` rejects a `v1` or `v1alpha1` manifest that sets it (`unknown field "spec.adminCredentials"`). Reading or updating the gateway through `v1` or `v1alpha1` keeps it (see [APIGateway versions and the conversion webhook](#apigateway-versions-and-the-conversion-webhook)). Changing `adminCredentials` through `v2alpha1` redeploys the gateway like any other spec change. Editing the `conversion-data` annotation directly through `v1` or `v1alpha1` does not change `metadata.generation`, but the operator still redeploys the gateway, because the annotation is part of its configuration hash.
- **Needs a gateway chart that renders `passwordSecretRef`.** The operator passes the admin password only as a Secret reference, which gateway charts from wso2/api-platform#4423 on render; they list `basicAuthPasswordSecretRef` in the `Chart.yaml` annotation `gateway.api-platform.wso2.com/features`. Before installing or upgrading, the operator checks the configured chart for it whenever a basic-auth user reads its password from a Secret: the `adminCredentials` user, the generated admin user (below), or a `configRef` user with `passwordSecretRef`. With an older chart (published charts up to and including `2026.11.19-r1`) it leaves the Helm release unchanged and reports `Programmed=False` with an error naming the chart version. A gateway an earlier operator deployed with `admin`/`admin` keeps it and does not need the newer chart.

#### Generated admin credential

An `APIGateway` (or a Kubernetes Gateway API `Gateway`) with no admin user of its own gets one generated by the operator on its first deployment:

- The operator creates the Secret `<gateway>-admin-credentials` in the gateway's namespace, with keys `username` (`admin`) and `password` (32 random letters and digits from `crypto/rand`). It is labelled `gateway.api-platform.wso2.com/admin-credentials: generated` and owned by the gateway, so deleting the gateway deletes it.
- The gateway-controller reads the password from the Secret mounted as a file, never an environment variable or the Helm values. The operator also turns off the gateway chart's own credential generation (`gateway.controller.adminCredentials.generate: false`), so the chart never generates a password the operator does not know.
- The operator annotates the gateway with `gateway.api-platform.wso2.com/admin-credentials-secret: <gateway>-admin-credentials`, so the gateway object shows where its password is. The annotation is informational: it is not part of the gateway's spec (GitOps tools that apply the spec leave it alone), and the operator removes it when the gateway stops using a generated credential, for example once `spec.adminCredentials` is set.
- The operator emits an `AdminCredentialsGenerated` Event on the gateway. Read the credential with:

  ```bash
  kubectl get secret <gateway>-admin-credentials -n <namespace> -o jsonpath='{.data.password}' | base64 -d; echo
  ```

- Upgrades and redeployments reuse the Secret. Changing its `password` rolls the gateway-controller pod of an `APIGateway`; restart a Gateway API gateway's controller yourself.
- If the Secret is deleted after the gateway was deployed with it, the operator does not generate a different credential: it reports `Programmed=False` (and an `AdminCredentialsUnavailable` Event) until the Secret is restored or `spec.adminCredentials` is set.
- A Secret with that name that the operator did not create (no label) is neither used nor overwritten; the gateway is not deployed until it is renamed or referenced from `spec.adminCredentials`.
- The generated credential needs a gateway chart from wso2/api-platform#4423 on (see "Needs a gateway chart that renders `passwordSecretRef`" above); with an older chart the gateway is not deployed.

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

### Enable Debug Mode

Debug mode runs the operator under the Delve debugger for remote debugging:

```yaml
debug:
  enabled: true
  port: 2345
  debugImage: "ghcr.io/wso2/api-platform/gateway-operator:0.12.0-debug"
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
>
> **Upgrading to the release that adds `APIGateway` `v2alpha1`:** upgrade the chart first, then apply the CRDs with `kubectl apply --server-side --force-conflicts -f crds/`. `--force-conflicts` is needed because the operator owns the live `APIGateway` CRD's `spec.conversion` webhook Service and `caBundle`: without it the apply fails with field-ownership conflicts. The apply puts the shipped placeholder back, and the operator restores its Service and CA within seconds. The new `APIGateway` CRD converts through the operator's webhook, which only the new operator serves. Applying it before the upgrade makes `v1alpha1` requests fail until the new operator is ready. Requests through `v1` are not affected. The operator replaces the CRD's placeholder webhook Service within seconds.

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

**Note:** This does not delete the CRDs (Helm never removes CRDs installed from `crds/`). The `APIGateway` CRD then still points at the removed conversion webhook: requests through `v1` keep working (including the chart's finalizer cleanup job), while `v1alpha1` and `v2alpha1` requests fail until the operator is reinstalled or the CRDs are deleted. To delete them manually:

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
