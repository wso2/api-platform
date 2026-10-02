# AI Workspace Helm Chart

This is the **AI Workspace product package** — an umbrella chart that installs the shared
Platform API control plane together with the AI Workspace UI. It ships no workloads of its
own: each component is a separately released chart, pulled in as a dependency.

| Component | Chart | Enabled by default | Purpose |
| --- | --- | --- | --- |
| Platform API | [`platform-api`](../platform-api-helm-chart/README.md) | yes | Shared control plane (Go backend) |
| AI Workspace UI | [`ai-workspace-ui`](../ai-workspace-ui-helm-chart/README.md) | yes | React SPA + Go BFF, talks only to the Platform API |

The umbrella itself renders only a shared ServiceAccount, render-time validation, and the
post-install notes.

## How the umbrella works

- **Values are namespaced by component.** Settings for a component go under its chart name
  in kebab-case — `platform-api.*` or `ai-workspace-ui.*`. Every key documented in a
  component's own `values.yaml` can be overridden that way. For example, the component key
  `config.auth.mode` becomes `ai-workspace-ui.config.auth.mode` here.
- **`global.*` is shared.** Naming overrides, common labels/annotations, image pull secrets,
  the ServiceAccount, the WSO2 subscription registry, and the in-cluster Platform API address
  (`global.platformApi.port` / `tlsEnabled`) are set once and every component reads them. A
  component's own `global:` block is only its standalone default; the umbrella's value wins.
- **`<component>.enabled` turns a component on or off.** It maps to the dependency
  `condition` in `Chart.yaml`, so a disabled component isn't rendered at all.
- **Wiring is automatic.** The UI derives the in-cluster Platform API URL
  (`https://<release>-platform-api:9243`) from `global.platformApi`, so you don't need to set
  any URL when both components are in the same release.
- **The umbrella validates cross-component settings.** The install fails early if no UI is
  enabled, if the Platform API is disabled without an external URL, or if
  `platform-api.config.auth.authorization.mode` doesn't match
  `ai-workspace-ui.config.auth.authorization.mode`.
- **One ServiceAccount per release.** It's created by the umbrella (`global.serviceAccount`)
  and shared by all pods.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+
- cert-manager. By default each component's HTTPS certificate is issued by cert-manager.
  Without it, give every component a TLS Secret (`tls.certificateProvider=secret`).
- `kubectl` and `openssl`, plus `htpasswd` (apache2-utils / httpd-tools) or `docker`. The
  secret-generation script uses these.

## Installing cert-manager

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

Run every command below from `kubernetes/helm/`. The script writes `values-secrets.yaml` to
your current directory.

### Step 1: Generate the Secrets

The chart never creates or embeds secret values. `generate-secrets.sh` creates the
Kubernetes Secrets and writes `values-secrets.yaml`. That file holds only the Secret
**names**, so it's safe to commit.

```bash
./ai-workspace-helm-chart/generate-secrets.sh ai-workspace            # <namespace> [release-name]
```

The script:
- creates the namespace if it doesn't exist.
- creates `<release>-platform-api-secrets`, holding the AES-256 encryption key, the RS256
  JWT keypair, and the file-mode admin credential (a generated username and bcrypt hash).
- prints the **generated admin password once**. Store it right away — there's no
  `admin/admin` default.
- creates `<release>-ai-workspace-ui-secrets` only when `APIP_AIW_AUTH_OIDC_CLIENT_SECRET`
  is set. Basic (file-based) login doesn't need it.

It is **cumulative and idempotent**: a re-run never rotates or deletes an existing Secret.
Optional inputs are passed as environment variables:

```bash
APIP_CP_DATABASE_PASSWORD='<db-password>' \
APIP_AIW_AUTH_OIDC_CLIENT_SECRET='<oidc-client-secret>' \
  ./ai-workspace-helm-chart/generate-secrets.sh ai-workspace
```

| Variable | Used for |
| --- | --- |
| `APIP_CP_ADMIN_USERNAME` | File-mode admin username (default `admin`) |
| `APIP_CP_DATABASE_PASSWORD` | Platform API database password (`postgres` / `sqlserver`) |
| `APIP_CP_WEBHOOK_SECRET` | Platform API webhook receiver HMAC secret |
| `APIP_AIW_AUTH_OIDC_CLIENT_SECRET` | AI Workspace UI OIDC client secret (`auth.mode=oidc`) |

If you pass a release name as the second argument, use the **same** release name in
`helm install`.

### Step 2: Fetch the component charts

The dependencies point at the OCI registry `oci://ghcr.io/wso2/api-platform/helm-charts`:

```bash
helm dependency update ./ai-workspace-helm-chart
```

> To develop against your local component charts without publishing them, temporarily
> change each dependency's `repository` in `Chart.yaml` to `file://../<name>-helm-chart`, then
> re-run `helm dependency update`.

### Step 3: Install the chart

Install with default values:
```bash
helm upgrade --install ai-workspace ./ai-workspace-helm-chart -n ai-workspace \
  -f values-secrets.yaml
```

Install from the published OCI chart (no checkout needed, only `values-secrets.yaml`):
```bash
helm upgrade --install ai-workspace oci://ghcr.io/wso2/api-platform/helm-charts/ai-workspace \
  --version 1.0.0 -n ai-workspace -f values-secrets.yaml
```

Install with your own overrides (layered last, so it wins):
```bash
helm upgrade --install ai-workspace ./ai-workspace-helm-chart -n ai-workspace \
  -f values-secrets.yaml -f my_values.yaml
```

For developers using locally built / `latest` images:
```bash
helm upgrade --install ai-workspace ./ai-workspace-helm-chart -n ai-workspace \
  -f values-secrets.yaml -f ai-workspace-helm-chart/values-local.yaml
```

`values-local.yaml` also sets `ai-workspace-ui.config.controlPlane.tlsSkipVerify=true` so the
UI accepts the Platform API's self-signed development certificate. Don't use it in production.

## Accessing the AI Workspace

The UI Service defaults to `LoadBalancer` on port `9643`. The app is served under the
`/ai-workspace` path prefix:

```bash
kubectl get svc ai-workspace-ai-workspace -n ai-workspace -w     # wait for EXTERNAL-IP
# then open https://<EXTERNAL-IP>:9643/ai-workspace/

# or, without a load balancer:
kubectl port-forward svc/ai-workspace-ai-workspace 9643:9643 -n ai-workspace
# then open https://localhost:9643/ai-workspace/
```

In `basic` auth mode, log in with the admin credential that `generate-secrets.sh` printed.
`helm status ai-workspace -n ai-workspace` prints the same access instructions for your
configuration.

## Uninstalling the Chart

```bash
helm uninstall ai-workspace -n ai-workspace
```

The Platform API's data PVC (`<release>-platform-api-data`) carries
`helm.sh/resource-policy: keep`, so **uninstall doesn't delete it**. A later install with the
same release name reuses it. Uninstall also leaves the Secrets from Step 1. To wipe
everything:

```bash
kubectl delete pvc ai-workspace-platform-api-data -n ai-workspace
kubectl delete secret ai-workspace-platform-api-secrets ai-workspace-ai-workspace-ui-secrets -n ai-workspace --ignore-not-found
```

Deleting the Platform API Secret discards its encryption key. Any data encrypted with it in a
retained database becomes unreadable.

## Upgrading the Chart

```bash
helm dependency update ./ai-workspace-helm-chart
helm upgrade ai-workspace ./ai-workspace-helm-chart -n ai-workspace \
  -f values-secrets.yaml -f my_values.yaml
```

Always pass `values-secrets.yaml` again. Without it, the render fails because
`platform-api.secrets.existingSecret` is unset. If you turn on OIDC later, re-run
`generate-secrets.sh` with the new input. It adds the new Secret and leaves the existing ones
untouched.

## Verifying the Installation

```bash
helm status ai-workspace -n ai-workspace
kubectl get all -l app.kubernetes.io/instance=ai-workspace -n ai-workspace

# Platform API logs
kubectl logs -l app.kubernetes.io/component=platform-api -n ai-workspace
# AI Workspace UI (BFF) logs
kubectl logs -l app.kubernetes.io/component=ai-workspace -n ai-workspace
```

### Troubleshooting

- **`platformApi.secrets.existingSecret is required`** — `values-secrets.yaml` wasn't passed,
  or Step 1 wasn't run. Run the script and install with `-f values-secrets.yaml`.
- **`found in Chart.yaml, but missing in charts/ directory`** — run Step 2
  (`helm dependency update`).
- **`... authorization.mode is "role" but ai-workspace-ui ... is "scope"`** — the UI mirrors
  the Platform API's authorization mode. Set both
  `platform-api.config.auth.authorization.mode` and
  `ai-workspace-ui.config.auth.authorization.mode` to the same value.
- **The UI loads but every API call fails with a TLS / certificate error** — the default
  cert-manager Issuer is self-signed, and the UI verifies the Platform API's certificate by
  default. For development, set `ai-workspace-ui.config.controlPlane.tlsSkipVerify=true`
  (`values-local.yaml` does this). For production, issue the Platform API certificate from a
  trusted issuer (`platform-api.tls.certManager.issuerRef`). Include the in-cluster Service
  name (`<release>-platform-api`, `<release>-platform-api.<namespace>.svc`) in
  `platform-api.tls.certManager.dnsNames`. Then point
  `ai-workspace-ui.config.controlPlane.caFile` at the mounted CA.
- **`deployment.replicaCount > 1 requires an external database` / HPA render failure** —
  the Platform API defaults to SQLite, which is single-replica. Switch
  `platform-api.config.database.driver` to `postgres` or `sqlserver` first. The UI is
  stateless and can scale freely.
- **Lost the admin password** — the script never shows it again. Generate a new bcrypt hash
  and patch it into the Secret, then restart the Platform API:
  ```bash
  HASH=$(printf '%s' 'new-password' | htpasswd -niB -C 10 "" | cut -d: -f2 | tr -d '\r\n')
  kubectl patch secret ai-workspace-platform-api-secrets -n ai-workspace \
    -p "{\"stringData\":{\"APIP_CP_ADMIN_PASSWORD_HASH\":\"$HASH\"}}"
  kubectl rollout restart deployment/ai-workspace-platform-api -n ai-workspace
  ```
  Don't delete the Secret and re-run the script to reset the password. That also replaces
  the encryption key and JWT keys.

## Chart layout

```
ai-workspace-helm-chart/
├── Chart.yaml              # dependencies: platform-api, ai-workspace-ui (OCI)
├── generate-secrets.sh     # creates the Secrets + writes values-secrets.yaml
├── templates/
│   ├── serviceaccount.yaml # the single shared ServiceAccount
│   ├── validation.yaml     # cross-component render-time checks (renders nothing)
│   └── NOTES.txt
├── values.yaml             # global.* + per-component toggles
├── values-local.yaml       # local development overrides (not packaged)
└── README.md
```

## Configuration

`values.yaml` holds only the `global.*` block and the component toggles. The full option
list for each component is in that component's README and `values.yaml`:

- [`platform-api`](../platform-api-helm-chart/README.md) — database, auth modes
  (`file` / `internal_token` / `idp`), role-to-scope mapping, TLS, persistence, HPA.
- [`ai-workspace-ui`](../ai-workspace-ui-helm-chart/README.md) — auth mode (`basic` / `oidc`),
  Platform API hop, gateway onboarding hints, sessions, TLS, HPA.

Common umbrella-level settings:

| Key | Description |
| --- | --- |
| `global.fullnameOverride` / `nameOverride` | Fixed resource-name prefix instead of the release name |
| `global.commonLabels` / `commonAnnotations` | Applied to every rendered object |
| `global.imagePullSecrets` | Pull secrets attached to every pod |
| `global.serviceAccount.*` | The shared ServiceAccount (`create`, `name`, `annotations` such as an IRSA role) |
| `global.wso2.subscription.imagePullSecret` | WSO2 Subscription: adds the pull secret and rewrites images to `registry.wso2.com/wso2-api-platform/*` |
| `global.platformApi.port` / `tlsEnabled` | How the UI reaches the Platform API. **Must match** `platform-api.service.port` and `platform-api.config.server.https.enabled` |
| `platform-api.enabled` / `ai-workspace-ui.enabled` | Component toggles |

Example `my_values.yaml` — OIDC login and an external PostgreSQL:

```yaml
platform-api:
  config:
    database:
      driver: postgres
      postgres:
        host: postgres.db.svc
        name: platform_api
        user: platform_api        # password: APIP_CP_DATABASE_PASSWORD in the Secret
ai-workspace-ui:
  config:
    auth:
      mode: oidc
      oidc:
        authority: https://idp.example.com
        clientId: ai-workspace
        redirectUrl: https://workspace.example.com/ai-workspace/api/auth/callback
        postLogoutRedirectUrl: https://workspace.example.com/ai-workspace/login
        scope: "openid profile email"
```

### Using an external Platform API

To point the UI at a Platform API that this release doesn't manage, disable the bundled one
and set the URL explicitly:

```yaml
platform-api:
  enabled: false
ai-workspace-ui:
  config:
    controlPlane:
      url: https://platform-api.example.com:9243
    auth:
      authorization:
        mode: scope     # must match the external Platform API's mode — not checked in this case
```
