# API Platform Gateway Helm Chart

This chart packages the API Platform Gateway deployment (controller and gateway runtime) that is provided as raw manifests in `internal/controller/resources/api-platform-gateway-k8s-manifests.yaml`.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+
- cert-manager (for TLS certificate management)
- `openssl` (to generate the required AES-256 at-rest encryption key)

## Installing cert-manager

The gateway requires cert-manager for TLS certificate management. Install it before deploying the gateway:

```bash
helm upgrade --install \
  cert-manager oci://quay.io/jetstack/charts/cert-manager \
  --version v1.19.1 \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true \
  --debug --wait --timeout 10m
```

Verify the installation:
```bash
kubectl get pods -n cert-manager
```

## Installing the Chart

### Step 1: Create the encryption key Secret

Generate a 32-byte AES-256 key and store it in a Secret **in the namespace you install into**:

```bash
openssl rand 32 > default-aesgcm256-v1.bin
kubectl create secret generic gateway-encryption-keys \
  --from-file=default-aesgcm256-v1.bin=default-aesgcm256-v1.bin
rm default-aesgcm256-v1.bin   # don't leave the plaintext key on disk
# add -n <namespace> to both this command and `helm install` for a non-default namespace
```

### Step 2: Install the chart
For developers using SNAPSHOT images:
```bash
helm install ap-gateway . -f values-local.yaml
```

Install with default values:
```bash
helm install ap-gateway . \
  --set gateway.controller.encryptionKeys.enabled=true \
  --set gateway.controller.encryptionKeys.secretName=gateway-encryption-keys
```

Install with custom control plane configuration:
```bash
helm install ap-gateway . \
  --set gateway.controller.encryptionKeys.enabled=true \
  --set gateway.controller.encryptionKeys.secretName=gateway-encryption-keys \
  --set gateway.controller.controlPlane.host="host.docker.internal:8443" \
  --set gateway.controller.controlPlane.token.value="your-token-here"
```

Install in a specific namespace:
```bash
kubectl create namespace api-gateway
openssl rand 32 > default-aesgcm256-v1.bin
kubectl create secret generic gateway-encryption-keys -n api-gateway \
  --from-file=default-aesgcm256-v1.bin=default-aesgcm256-v1.bin
rm default-aesgcm256-v1.bin   # don't leave the plaintext key on disk
helm install ap-gateway . --namespace api-gateway \
  --set gateway.controller.encryptionKeys.enabled=true \
  --set gateway.controller.encryptionKeys.secretName=gateway-encryption-keys \
  --set gateway.controller.controlPlane.host="platform.example.com"
```

Install with custom values file:
The file must set `gateway.controller.encryptionKeys.enabled=true` and `secretName`:
```bash
helm install ap-gateway . -f custom-values.yaml
```

## Uninstalling the Chart

```bash
helm uninstall ap-gateway
```

Or with namespace:
```bash
helm uninstall ap-gateway --namespace api-gateway
```

## Upgrading the Chart

```bash
helm upgrade ap-gateway .
```

Upgrade with new values:
```bash
helm upgrade ap-gateway . -f custom-values.yaml
```

## Verifying the Installation

Check the status of the release:
```bash
helm status ap-gateway
```

List all resources:
```bash
kubectl get all -l app.kubernetes.io/instance=ap-gateway
```

Check pod logs:
```bash
# Controller logs
kubectl logs -l app.kubernetes.io/component=controller

# Gateway Runtime logs (Router + Policy Engine)
kubectl logs -l app.kubernetes.io/component=gateway-runtime
```

### Troubleshooting

- **`helm install` fails with `gateway.controller.encryptionKeys must be enabled ...`** — the
  chart is fail-closed on at-rest encryption. Complete
  [Step 1](#step-1-create-the-encryption-key-secret) (create the key Secret) and pass the
  `encryptionKeys` flags in [Step 2](#step-2-install-the-chart).
- **Controller pod is `CrashLoopBackOff` / never becomes Ready** — usually a missing or
  wrong-namespace encryption key Secret. Confirm it exists in the release namespace and that
  `default-aesgcm256-v1.bin` appears under `Data` with `32 bytes`:
  ```bash
  kubectl describe secret gateway-encryption-keys -n <namespace>
  ```
  If that entry is missing or misnamed (it must be `default-aesgcm256-v1.bin`), recreate the
  Secret. Also check the controller logs for the encryption-key error.

## Chart layout

```
helm-chart/
├── templates/
│   ├── gateway/
│   │   ├── controller/       # Deployment, Service, ConfigMap, PVC
│   │   └── gateway-runtime/  # Deployment and Service
│   ├── serviceaccount.yaml
│   └── _helpers.tpl
├── values.yaml
└── README.md
```

Each major workload (controller, gateway-runtime) lives in its own nested template folder so their manifests remain isolated and easier to reason about.

## Configuration

All configurable values are documented in `values.yaml`. Component blocks are fully namespaced so overrides are intuitive:

- `gateway.controller.image` / `gateway.gatewayRuntime.image` – container image metadata and pull policies.
- `gateway.<component>.deployment.*` – pod-level knobs (replicas, probes incl. optional `startupProbe`, scheduling via `nodeSelector`/`tolerations`/`affinity`/`topologySpreadConstraints`, update `strategy`, `terminationGracePeriodSeconds`, `hostAliases`, `dnsPolicy`/`dnsConfig`, `automountServiceAccountToken`, env overrides, extra volumes) and enable/disable switches.
- `gateway.<component>.service.*` – service type/ports plus optional annotations and labels, and network tuning (`clusterIP`, `externalTrafficPolicy`, `loadBalancerClass`, `loadBalancerSourceRanges`, `ipFamilyPolicy`/`ipFamilies`, static `nodePorts.*`).
- `gateway.<component>.service.expose.*` – per-port toggles for publishing admin/debug ports on the Service. **All default to `false`** so admin surfaces stay pod-internal (reach them with `kubectl port-forward`). Available toggles: `controller.service.expose.admin` (controller admin, 9092), `gatewayRuntime.service.expose.routerAdmin` (Router/Envoy admin, 9901 — includes mutating endpoints, leave off unless trusted), `gatewayRuntime.service.expose.policyEngineAdmin` (policy-engine admin, 9002). Probes are unaffected; they target container ports directly. Note: the controller admin port is no longer exposed by default — set `expose.admin=true` to restore prior behavior. The runtime port key was renamed `envoyAdmin` → `routerAdmin`.
- The sensitive `/config_dump` route on the controller and policy-engine admin servers is **off by default** (returns 404), independent of the `expose.*` Service toggles above — set `gateway.config.controller.admin_server.config_dump.enabled=true` / `gateway.config.policy_engine.admin.config_dump.enabled=true` to turn it on. `/health` and other admin routes are unaffected, so liveness/readiness probes keep working either way. Envoy's admin interface (port 9901) is likewise off by default at the process level — nothing is even listening on it unless `gateway.gatewayRuntime.deployment.env.routerAdminEnabled=true`, and even then it binds loopback-only inside the pod. Enabling it also restores the graceful-drain-on-SIGTERM behavior; `gateway-runtime`'s health probe falls back to a plain TCP check on the HTTP listener when it's off.
- `gateway.controller.persistence` / `gateway.configMap` – PVC sizing/claims (plus PVC `labels`/`annotations`, e.g. `helm.sh/resource-policy: keep`) and component configuration payloads.
- `commonLabels` / `commonAnnotations` – applied to every resource the chart renders; per-resource labels/annotations win on key conflicts.
- `gateway.controller.controlPlane` – control-plane connectivity. The host is non-secret and rendered directly into `config.toml`; the token is injected from a Secret. The controller log level is `gateway.config.controller.logging.level`.
- `gateway.controller.tls.*` – TLS certificate configuration for HTTPS listener using cert-manager or existing secrets.
- `gateway.controller.upstreamCerts.*` – Custom CA certificates for upstream backend TLS verification.
- `gateway.config.policy_engine.*` – policy engine configuration including xDS client settings and admin API.
- `gateway.config.api_key` / `gateway.config.subscriptions` – API-key policy tuning (length/algorithm/issuer) and opt-in application-subscription validation.

Refer to the inline comments inside `values.yaml` for a complete matrix of options and the expected data types for each block.

### How configuration is delivered

The chart renders a full `config.toml` into a ConfigMap and mounts it into both the controller
and the policy engine. Non-secret settings are written directly from `gateway.config.*`. There
is **no `APIP_GW_*` environment-variable override** — environment variables reach the config only
through explicit `{{ env "NAME" "default" }}` interpolation tokens that the gateway resolves at
container startup. The chart uses this only for the runtime secrets that must not appear in the
ConfigMap:

- **Control-plane token** — set `gateway.controller.controlPlane.token.{value,secretName,key}`.
  It is injected into the container as `APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN` and read back by an
  interpolation token in `config.toml`. The control-plane host is non-secret and rendered directly
  from `gateway.controller.controlPlane.host`.
- **Database password** — create the Secret and point a `passwordSecretRef` at it:
  ```bash
  kubectl create secret generic gateway-db --from-literal=password='<db-password>'
  # then: gateway.controller.postgres.passwordSecretRef.name=gateway-db   (postgres)
  #   or: gateway.controller.sqlserver.passwordSecretRef.name=gateway-db  (sqlserver)
  ```
  It is injected as `APIP_GW_CONTROLLER_STORAGE_POSTGRES_PASSWORD` /
  `APIP_GW_CONTROLLER_STORAGE_DATABASE_PASSWORD` and read back by an interpolation token in
  `config.toml`.

A plain `APIP_GW_*` env var with no matching token in `config.toml` is ignored.

### Raw config passthrough (`gateway.config_toml`)

For configuration the chart doesn't model as structured `gateway.config.*` values, use
`gateway.config_toml` — a raw TOML string rendered into the generated `config.toml`. It is
injected at the **top of the file, above every `[table]` section**, so it can carry two shapes:

- **Bare root keys** — e.g. policy "system parameters" that policies read as `${config.<key>}`
  (the Azure Content Safety, AWS Bedrock Guardrail, embedding-provider, Redis, and similar
  policies). TOML requires bare keys to precede all `[table]` headers, which is why the block
  leads the file.
- **Whole `[table]` sections** — a custom section the chart doesn't render. Tables are
  order-independent, so these work from the top too.

Do **not** redefine a table the chart already emits (`[controller.server]`, `[router]`, etc.) —
TOML rejects a duplicate table.

`config.toml` is rendered into a **ConfigMap, not a Secret**, so it must not hold plaintext
secrets. For any secret value, supply a `{{ env "NAME" }}` or `{{ file "/path" }}` interpolation
token (resolved by the gateway at startup, same mechanism as the control-plane token and DB
password above) instead of a literal — each token reads from a different source, so back it with
the matching mechanism:

- `{{ env "NAME" }}` reads an environment variable — inject it via
  `gateway.gatewayRuntime.deployment.extraEnv`.
- `{{ file "/path" }}` reads a mounted file — mount the Secret via
  `gateway.gatewayRuntime.deployment.extraVolumes` / `extraVolumeMounts` under one of the allowed
  source dirs (`/etc/gateway-runtime`, `/secrets/gateway-runtime`) and point the token at that
  mount path.

Example — the Azure Content Safety content-moderation policy (endpoint is non-secret, the
subscription key is a secret):

```yaml
gateway:
  config_toml: |
    azurecontentsafety_endpoint = "https://your-resource.cognitiveservices.azure.com"
    azurecontentsafety_key = '{{ env "APIP_GW_AZURE_CONTENT_SAFETY_KEY" }}'
  gatewayRuntime:
    deployment:
      extraEnv:
        - name: APIP_GW_AZURE_CONTENT_SAFETY_KEY
          valueFrom:
            secretKeyRef:
              name: azure-content-safety   # kubectl create secret generic azure-content-safety --from-literal=subscription-key='<key>'
              key: subscription-key
```

### Storage backends and scaling

`gateway.config.controller.storage.type` selects the controller's database:

| Type | Config block | Replicas |
| --- | --- | --- |
| `sqlite` (default) | `storage.sqlite` | Single replica only — the DB is a file on a PersistentVolume |
| `postgres` | `storage.postgres` | Multi-replica |
| `sqlserver` | `storage.database` | Multi-replica |

Notes:

- `gateway.controller.hpa.enabled=true` requires `postgres` or `sqlserver`; the chart fails to
  render with `sqlite`, which cannot serve multiple replicas.
- The PersistentVolumeClaim backs the SQLite file only. With `postgres`/`sqlserver` no claim is
  created or mounted regardless of `gateway.controller.persistence.enabled`, so replicas are not
  pinned to a single node by a `ReadWriteOnce` volume. If you are switching an existing release
  from `sqlite` to an external database and want to retain the old claim, annotate it with
  `helm.sh/resource-policy: keep` before upgrading.
- SQL Server connection encryption is on by default
  (`storage.database.options.encrypt="true"`, `trust_server_certificate="false"`), matching the
  gateway-controller binary. Relax these only for a local/dev SQL Server without a trusted
  certificate.
- Prefer the individual `host`/`port`/`database`/`user` fields over `dsn`: a DSN is rendered
  verbatim into the ConfigMap, so an embedded password would be stored in plaintext instead of
  being injected from the Secret above.

## At-rest Encryption (Required)

At-rest encryption of stored secrets is **mandatory and fail-closed**: the gateway-controller
will not start without its AES-256 key, and the chart **refuses to render** unless
`gateway.controller.encryptionKeys.enabled=true` with a `secretName`. There is no
development/demo bypass, and nothing is auto-generated — you provision the key.

Create the Secret and enable it as shown in
[Step 1](#step-1-create-the-encryption-key-secret) and
[Step 2](#step-2-install-the-chart) of *Installing the Chart*.

Details:
- The Secret must live in the **same namespace** as the release.
- The Secret's key entry must be named `default-aesgcm256-v1.bin` — it must match the filename in
  `gateway.config.controller.encryption.providers[].keys[].file` (default
  `/app/data/aesgcm-keys/default-aesgcm256-v1.bin`; the mount directory is
  `gateway.controller.encryptionKeys.mountPath`).
- Rotating the key makes previously-encrypted data unreadable.

## TLS Certificate Configuration

The gateway controller supports HTTPS with automatic certificate management via cert-manager or manual certificate provisioning.

### Using cert-manager (Recommended)

If you followed the installation steps above, cert-manager is already installed in your cluster. The chart will automatically:
1. Create a self-signed Issuer for development
2. Generate a TLS certificate automatically

```bash
# Simple installation with automatic TLS
helm install ap-gateway . \
  --set gateway.controller.tls.enabled=true
```

For production, configure a proper issuer (Let's Encrypt, corporate CA):

```yaml
gateway:
  controller:
    tls:
      enabled: true
      certificateProvider: cert-manager
      certManager:
        createIssuer: false  # Don't create default self-signed issuer
        issuerRef:
          name: letsencrypt-prod
          kind: Issuer  # or ClusterIssuer
        commonName: api.example.com
        dnsNames:
          - api.example.com
          - "*.api.example.com"
```

### Using Existing Secret

```bash
# Create a secret with your TLS certificate
kubectl create secret tls gateway-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key

# Install with existing secret
helm install ap-gateway . \
  --set gateway.controller.tls.enabled=true \
  --set gateway.controller.tls.certificateProvider=secret \
  --set gateway.controller.tls.secret.name=gateway-tls
```

### Custom Upstream Certificates

For backends with self-signed or custom CA certificates:

```bash
# Create a secret or configmap with CA certificates
kubectl create secret generic upstream-ca-certs \
  --from-file=ca1.crt=path/to/ca1.crt \
  --from-file=ca2.crt=path/to/ca2.crt

# Install with upstream certs
helm install ap-gateway . \
  --set gateway.controller.upstreamCerts.enabled=true \
  --set gateway.controller.upstreamCerts.secretName=upstream-ca-certs
```

Refer to the inline comments inside `values.yaml` for a complete matrix of options and the expected data types for each block.
