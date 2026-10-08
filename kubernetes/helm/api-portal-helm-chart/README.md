# API Portal Helm Chart

This is the **API Portal & MCP Hub product package** — an umbrella chart that installs the
shared Platform API control plane together with the API Portal. It ships no workloads of its
own: each component is a separately released chart, pulled in as a dependency.

| Component | Chart | Enabled by default | Purpose |
| --- | --- | --- | --- |
| Platform API | [`platform-api`](../platform-api-helm-chart/README.md) | yes | Shared control plane (Go backend); issues the RS256 tokens the portal verifies |
| API Portal | [`api-portal-ui`](../api-portal-ui-helm-chart/README.md) | yes | Node.js developer portal & MCP Hub with its **own** database |

The umbrella itself renders only a shared ServiceAccount, render-time validation, and the
post-install notes.

## How the umbrella works

- **Values are namespaced by component.** Settings for a component go under its chart name
  in kebab-case — `platform-api.*` or `api-portal-ui.*`. Every key documented in a
  component's own `values.yaml` can be overridden that way. For example, the component key
  `config.server.baseUrl` becomes `api-portal-ui.config.server.baseUrl` here.
- **`global.*` is shared.** Naming overrides, common labels/annotations, image pull secrets,
  the ServiceAccount, the WSO2 subscription registry, and the in-cluster Platform API address
  (`global.platformApi.port` / `tlsEnabled`) are set once and every component reads them. A
  component's own `global:` block is only its standalone default; the umbrella's value wins.
- **`<component>.enabled` turns a component on or off.** It maps to the dependency
  `condition` in `Chart.yaml`, so a disabled component isn't rendered at all.
- **Wiring is automatic.** The portal derives the in-cluster Platform API URL
  (`https://<release>-platform-api:9243`) from `global.platformApi`. It verifies Platform
  API-issued tokens with the RS256 **public key**, which `generate-secrets.sh` copies from the
  Platform API Secret into the portal's Secret. There's no shared HMAC secret.
- **The umbrella validates cross-component settings.** The install fails early if the portal
  is disabled, or if the Platform API is disabled without an external URL
  (`api-portal-ui.config.platformApi.baseUrl`).
- **One ServiceAccount per release.** It's created by the umbrella (`global.serviceAccount`)
  and shared by all pods.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+
- cert-manager. By default each component's HTTPS certificate is issued by cert-manager.
  Without it, give every component a TLS Secret (`tls.certificateProvider=secret`). The
  portal can also run plain HTTP (`tls.certificateProvider=none`) behind a TLS-terminating
  proxy.
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

The commands assume the release **and** namespace are both named `api-portal`. Every resource is
prefixed with the release name: `<release>-platform-api`, `<release>-platform-api-secrets`,
`<release>-platform-api-data`, and so on. If you choose different names, substitute them in
every command below, including access, uninstall, and cleanup.

### Step 1: Generate the Secrets

The chart never creates or embeds secret values. `generate-secrets.sh` creates the
Kubernetes Secrets and writes `values-secrets.yaml`. That file holds only the Secret
**names** and `has*` flags, so it's safe to commit.

```bash
./api-portal-helm-chart/generate-secrets.sh api-portal              # <namespace> [release-name]
```

The script:
- creates the namespace if it doesn't exist.
- creates `<release>-platform-api-secrets`, holding the AES-256 encryption key, the RS256
  JWT keypair, and the file-mode admin credential (a generated username and bcrypt hash).
- prints the **generated admin password once**. Store it right away — there's no
  `admin/admin` default.
- creates `<release>-api-portal-ui-secrets`, holding the portal's encryption key, session
  secret, and a copy of the Platform API public key (`jwt_public.pem`).

It is **cumulative and idempotent**: a re-run never rotates or deletes an existing Secret.
Optional inputs are passed as environment variables. Read secret values with `read -rs`
so they never land in your shell history:

```bash
read -rsp 'API Portal DB password: ' APIP_AP_DATABASE_PASSWORD; echo
read -rsp 'OIDC client secret: ' APIP_AP_AUTH_IDP_CLIENT_SECRET; echo
export APIP_AP_DATABASE_PASSWORD APIP_AP_AUTH_IDP_CLIENT_SECRET
./api-portal-helm-chart/generate-secrets.sh api-portal
unset APIP_AP_DATABASE_PASSWORD APIP_AP_AUTH_IDP_CLIENT_SECRET
```

| Variable | Used for |
| --- | --- |
| `APIP_CP_ADMIN_USERNAME` | File-mode admin username (default `admin`) |
| `APIP_CP_DATABASE_PASSWORD` | Platform API database password (`postgres` / `sqlserver`) |
| `APIP_CP_WEBHOOK_SECRET` | Platform API webhook receiver HMAC secret (must match the portal's webhook subscriber) |
| `APIP_AP_DATABASE_PASSWORD` | API Portal database password (`postgres` / `mssql`) |
| `APIP_AP_AUTH_IDP_CLIENT_SECRET` | API Portal OIDC client secret. Also sets `hasIdpClientSecret: true` in `values-secrets.yaml` |
| `API_PORTAL=false` | Skip the API Portal Secret |

Optional inputs only take effect when the Secret is **first** created. To add one to an
existing Secret, patch it with `kubectl patch secret ...`.

If you pass a release name as the second argument, use the **same** release name in
`helm install`.

### Step 2: Fetch the component charts

The dependencies point at the OCI registry `oci://ghcr.io/wso2/api-platform/helm-charts`:

```bash
helm dependency update ./api-portal-helm-chart
```

> To develop against your local component charts without publishing them, temporarily
> change each dependency's `repository` in `Chart.yaml` to `file://../<name>-helm-chart`, then
> re-run `helm dependency update`.

### Step 3: Install the chart

Install with default values:
```bash
helm upgrade --install api-portal ./api-portal-helm-chart -n api-portal \
  -f values-secrets.yaml
```

Install from the published OCI chart (no checkout needed, only `values-secrets.yaml`):
```bash
helm upgrade --install api-portal oci://ghcr.io/wso2/api-platform/helm-charts/api-portal \
  --version 1.0.0 -n api-portal -f values-secrets.yaml
```

Install with your own overrides (layered last, so it wins):
```bash
helm upgrade --install api-portal ./api-portal-helm-chart -n api-portal \
  -f values-secrets.yaml -f my_values.yaml
```

For developers using locally built / `latest` images:
```bash
helm upgrade --install api-portal ./api-portal-helm-chart -n api-portal \
  -f values-secrets.yaml -f api-portal-helm-chart/values-local.yaml
```

`values-local.yaml` also sets `api-portal-ui.config.platformApi.insecure=true` so the portal
accepts the Platform API's self-signed development certificate. Don't use it in production.

## Accessing the API Portal

The portal Service defaults to `LoadBalancer` on port `9543`. Pages are served under the
organization handle (`config.organization.handle`, default `default`):

```bash
kubectl get svc api-portal-api-portal -n api-portal -w     # wait for EXTERNAL-IP
# then open https://<EXTERNAL-IP>:9543/default/views/default

# or, without a load balancer:
kubectl port-forward svc/api-portal-api-portal 9543:9543 -n api-portal
# then open https://localhost:9543/default/views/default
```

Log in with the admin credential that `generate-secrets.sh` printed, unless you configured
OIDC. When a real hostname fronts the portal, set `api-portal-ui.config.server.baseUrl` so
login redirects and asset URLs are correct.

## Uninstalling the Chart

```bash
helm uninstall api-portal -n api-portal
```

Both data PVCs (`<release>-platform-api-data` and `<release>-api-portal-data`) carry
`helm.sh/resource-policy: keep`, so **uninstall doesn't delete them**. A later install with
the same release name reuses them. Uninstall also leaves the Secrets from Step 1. To wipe
everything:

```bash
kubectl delete pvc api-portal-platform-api-data api-portal-api-portal-data -n api-portal
kubectl delete secret api-portal-platform-api-secrets api-portal-api-portal-ui-secrets -n api-portal
```

Deleting a Secret discards its encryption key. Any data encrypted with it in a retained
database becomes unreadable.

## Upgrading the Chart

```bash
helm dependency update ./api-portal-helm-chart
helm upgrade api-portal ./api-portal-helm-chart -n api-portal \
  -f values-secrets.yaml -f my_values.yaml
```

Always pass `values-secrets.yaml` again. Without it, the render fails because the
`secrets.existingSecret` references are unset.

## Verifying the Installation

```bash
helm status api-portal -n api-portal
kubectl get all -l app.kubernetes.io/instance=api-portal -n api-portal

# Platform API logs
kubectl logs -l app.kubernetes.io/component=platform-api -n api-portal
# API Portal logs
kubectl logs -l app.kubernetes.io/component=api-portal -n api-portal
```

### Troubleshooting

- **`platformApi.secrets.existingSecret is required` / `apiPortal.secrets.existingSecret is
  required`** — `values-secrets.yaml` wasn't passed, or Step 1 wasn't run.
- **`found in Chart.yaml, but missing in charts/ directory`** — run Step 2
  (`helm dependency update`).
- **Login or API calls fail with a TLS / certificate error between the portal and the
  Platform API** — the default cert-manager Issuer is self-signed, and the portal verifies
  the Platform API's certificate by default. For development, set
  `api-portal-ui.config.platformApi.insecure=true` (`values-local.yaml` does this). For
  production, issue the Platform API certificate from a trusted issuer
  (`platform-api.tls.certManager.issuerRef`). Include the in-cluster Service name
  (`<release>-platform-api`, `<release>-platform-api.<namespace>.svc`) in
  `platform-api.tls.certManager.dnsNames`.
- **Portal rejects valid logins / token signature errors** — the portal's `jwt_public.pem`
  no longer matches the Platform API's private key. This happens if the Platform API Secret
  was recreated after the portal Secret. Copy the current public key over and restart the
  portal:
  ```bash
  PUB=$(kubectl get secret api-portal-platform-api-secrets -n api-portal -o jsonpath='{.data.jwt_public\.pem}')
  kubectl patch secret api-portal-api-portal-ui-secrets -n api-portal -p "{\"data\":{\"jwt_public.pem\":\"$PUB\"}}"
  kubectl rollout restart deployment/api-portal-api-portal -n api-portal
  ```
- **`config.database.sslmode must be "disable" or "verify-full"`** — the portal supports no
  partial TLS mode for its database. Use one of those two values.
- **`deployment.replicaCount > 1 requires an external database` / HPA render failure** —
  both components default to SQLite, which is single-replica. Switch
  `platform-api.config.database.driver` to `postgres`/`sqlserver`, and
  `api-portal-ui.config.database.driver` to `postgres`/`mssql`, before scaling.
- **Lost the admin password** — the script never shows it again. Patch a new bcrypt hash
  into the Platform API Secret, then restart the Platform API:
  ```bash
  read -rsp 'New admin password: ' NEW_PASSWORD; echo
  PATCH=$(umask 077; mktemp)
  printf '{"stringData":{"APIP_CP_ADMIN_PASSWORD_HASH":"%s"}}' \
    "$(printf '%s' "$NEW_PASSWORD" | htpasswd -niB -C 10 "" | cut -d: -f2 | tr -d '\r\n')" > "$PATCH"
  kubectl patch secret api-portal-platform-api-secrets -n api-portal --patch-file "$PATCH"
  rm -f "$PATCH"; unset NEW_PASSWORD
  kubectl rollout restart deployment/api-portal-platform-api -n api-portal
  ```

## Chart layout

```
api-portal-helm-chart/
├── Chart.yaml              # dependencies: platform-api, api-portal-ui (OCI)
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
  (`file` / `internal_token` / `idp`), role-to-scope mapping, webhook receiver, TLS,
  persistence, HPA.
- [`api-portal-ui`](../api-portal-ui-helm-chart/README.md) — the portal's own database, local
  vs OIDC login, authorization and page-role gating, organization, artifact types, uploads,
  the "Try It" console, TLS, persistence, HPA.

Common umbrella-level settings:

| Key | Description |
| --- | --- |
| `global.fullnameOverride` / `nameOverride` | Fixed resource-name prefix instead of the release name |
| `global.commonLabels` / `commonAnnotations` | Applied to every rendered object |
| `global.imagePullSecrets` | Pull secrets attached to every pod |
| `global.serviceAccount.*` | The shared ServiceAccount (`create`, `name`, `annotations` such as an IRSA role) |
| `global.wso2.subscription.imagePullSecret` | WSO2 Subscription: adds the pull secret and rewrites images to `registry.wso2.com/wso2-api-platform/*` |
| `global.platformApi.port` / `tlsEnabled` | How the portal reaches the Platform API. **Must match** `platform-api.service.port` and `platform-api.config.server.https.enabled` |
| `platform-api.enabled` / `api-portal-ui.enabled` | Component toggles |

Example `my_values.yaml` — public hostname and external PostgreSQL for both components:

```yaml
platform-api:
  config:
    database:
      driver: postgres
      postgres:
        host: postgres.db.svc
        name: platform_api
        user: platform_api        # password: APIP_CP_DATABASE_PASSWORD in the Platform API Secret
api-portal-ui:
  config:
    server:
      baseUrl: https://devportal.example.com
    database:
      driver: postgres
      host: postgres.db.svc
      database: devportal
      user: devportal             # password: APIP_AP_DATABASE_PASSWORD in the portal Secret
```

### Using an external Platform API

To point the portal at a Platform API that this release doesn't manage:

```yaml
platform-api:
  enabled: false
api-portal-ui:
  config:
    platformApi:
      baseUrl: https://platform-api.example.com:9243
```

The portal Secret's `jwt_public.pem` must then hold **that** Platform API's public key.
