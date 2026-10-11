# API Portal UI Helm Chart

This chart packages the **API Portal & MCP Hub**, the Node.js developer portal of the WSO2
API Platform. The portal keeps its **own database**, separate from the Platform API's. It
authenticates users against the Platform API and verifies Platform API-issued RS256 tokens
with its public key.

It is an independently released **component chart** and is used in two ways:

| How | When |
| --- | --- |
| **As a subchart of the [`api-portal`](../api-portal-helm-chart/README.md) umbrella** (recommended) | The umbrella installs the Platform API next to the portal, creates both Secrets (copying the Platform API public key into the portal's Secret), creates the ServiceAccount, and derives the Platform API URL. |
| **Standalone** | The Platform API is already running in another release or cluster. |

## Umbrella vs standalone

| | Under the umbrella | Standalone |
| --- | --- | --- |
| Values prefix | `api-portal-ui.<key>` (e.g. `api-portal-ui.config.server.baseUrl`) | `<key>` (e.g. `config.server.baseUrl`) |
| `enabled` | Umbrella dependency toggle | Ignored (the chart default is `false`, but the portal renders anyway). Use `deployment.enabled` to render nothing. |
| `global.*` | Set once by the umbrella. This chart's `global:` block is overridden. | This chart's own `global:` defaults apply |
| ServiceAccount | Created by the umbrella and shared | **None created**. Pods use the namespace `default` SA, or set `global.serviceAccount.name`. |
| Secret | Created by the umbrella's `generate-secrets.sh` | You create it ([below](#standalone-step-1-create-the-secret)) |
| Platform API URL | Derived: `https://<release>-platform-api:<global.platformApi.port>` | **Set `config.platformApi.baseUrl`**. The derived default assumes a Platform API in the *same* release. |

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+
- A reachable Platform API, and its RS256 **public key** (`jwt_public.pem`)
- cert-manager (default certificate provider), **or** a pre-created TLS Secret, **or**
  `tls.certificateProvider=none` for plain HTTP behind a TLS-terminating proxy. The image
  doesn't generate a self-signed certificate.

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

## Installing the Chart

### Via the umbrella

Follow the [`api-portal` README](../api-portal-helm-chart/README.md). Override this chart's
values under the `api-portal-ui:` key:

```yaml
api-portal-ui:
  config:
    server:
      baseUrl: https://devportal.example.com
```

### Standalone, Step 1: Create the Secret

The render **fails** unless `secrets.existingSecret` is set.

| Key | Required when |
| --- | --- |
| `APIP_AP_SECURITY_ENCRYPTION_KEY` | Always (64-char hex) |
| `APIP_AP_SECURITY_SESSION_SECRET` | Always (64-char hex) |
| `jwt_public.pem` | `secrets.hasPublicKey=true` (default) — the Platform API's public key |
| `APIP_AP_DATABASE_PASSWORD` | `config.database.driver` is `postgres` / `mssql` |
| `APIP_AP_AUTH_IDP_CLIENT_SECRET` | OIDC login, together with `secrets.hasIdpClientSecret=true` |

Each value is written to its own file in a private temporary directory. The directory is
passed with `--from-file`, which turns each file name into a Secret key. Secret values never
appear in shell history or in `kubectl`'s command-line arguments.

```bash
kubectl create namespace api-portal
SECRET_DIR=$(umask 077; mktemp -d)
# jwt_public.pem = the public half of the Platform API's RS256 keypair, e.g.:
kubectl get secret <platform-api-secret> -n <pa-namespace> -o jsonpath='{.data.jwt_public\.pem}' \
  | base64 --decode > "$SECRET_DIR/jwt_public.pem"
openssl rand -hex 32 | tr -d '\n' > "$SECRET_DIR/APIP_AP_SECURITY_ENCRYPTION_KEY"
openssl rand -hex 32 | tr -d '\n' > "$SECRET_DIR/APIP_AP_SECURITY_SESSION_SECRET"
kubectl create secret generic api-portal-secrets -n api-portal --from-file="$SECRET_DIR"
rm -rf "$SECRET_DIR"
```

For a database password or OIDC client secret, read it with `read -rsp` and write it to
`$SECRET_DIR/APIP_AP_DATABASE_PASSWORD` or `$SECRET_DIR/APIP_AP_AUTH_IDP_CLIENT_SECRET`
before running `kubectl create secret`. For the OIDC secret, also set
`secrets.hasIdpClientSecret=true`.

### Standalone, Step 2: Install the chart

Run from `kubernetes/helm/`. The commands assume release `api-portal-ui` in namespace
`api-portal`. Resource names are prefixed with the release name (`<release>-api-portal`,
`<release>-api-portal-data`); substitute your own if they differ.

```bash
helm install api-portal-ui ./api-portal-ui-helm-chart -n api-portal \
  --set secrets.existingSecret=api-portal-secrets \
  --set config.platformApi.baseUrl=https://platform-api-platform-api.platform-api.svc:9243
```

From the published OCI chart:
```bash
helm install api-portal-ui oci://ghcr.io/wso2/api-platform/helm-charts/api-portal-ui \
  --version 1.0.0 -n api-portal \
  --set secrets.existingSecret=api-portal-secrets \
  --set config.platformApi.baseUrl=https://platform-api.example.com:9243
```

For developers using locally built / `latest` images (this also skips TLS verification to the
Platform API):
```bash
helm install api-portal-ui ./api-portal-ui-helm-chart -n api-portal \
  -f api-portal-ui-helm-chart/values-local.yaml \
  --set secrets.existingSecret=api-portal-secrets \
  --set config.platformApi.baseUrl=https://platform-api-platform-api.platform-api.svc:9243
```

## Accessing the Portal

The Service defaults to `LoadBalancer` on port `9543`. Pages live under
`/<organization.handle>/views/default`:

```bash
kubectl port-forward svc/api-portal-ui-api-portal 9543:9543 -n api-portal
# then open https://localhost:9543/default/views/default
```

## Uninstalling the Chart

```bash
helm uninstall api-portal-ui -n api-portal
```

The SQLite PVC (`<release>-api-portal-data`) has `helm.sh/resource-policy: keep` and
**survives uninstall**. Delete it manually to wipe the portal's data, or set
`persistence.annotations: {}` to have Helm remove it.

## Upgrading the Chart

```bash
helm upgrade api-portal-ui ./api-portal-ui-helm-chart -n api-portal -f custom-values.yaml
```

## Verifying the Installation

```bash
helm status api-portal-ui -n api-portal
kubectl get all -l app.kubernetes.io/instance=api-portal-ui -n api-portal
kubectl logs -l app.kubernetes.io/component=api-portal -n api-portal
```

### Troubleshooting

- **`apiPortal.secrets.existingSecret is required`** — set `secrets.existingSecret` (under
  the umbrella: pass `-f values-secrets.yaml`).
- **`tls.certificateProvider must be "cert-manager", "secret", or "none"`** — `selfSigned` isn't
  supported. Pick one of those three values.
- **Pod fails to start with a missing-key error** — a `has*` flag is `true` but the Secret
  doesn't contain that key, for example `hasPublicKey=true` without `jwt_public.pem`. Add the
  key or set the flag to `false`.
- **Login / token verification fails** — `jwt_public.pem` must be the public half of the
  **current** Platform API keypair. Also check that `config.platformApi.baseUrl` is reachable
  from the pod. For a self-signed Platform API certificate in development, set
  `config.platformApi.insecure=true`.
- **`config.database.sslmode must be "disable" or "verify-full"`** — those are the only
  supported values. On SQL Server, anything else would connect unencrypted.
- **`deployment.replicaCount > 1 requires an external database` / HPA render failure** —
  switch `config.database.driver` to `postgres` or `mssql`.
- **Pages return 403 for valid users** — `config.auth.authorization.pageRoleValidation=true`
  gates pages on `portalRoles.admin` / `portalRoles.subscriber`. Those role names must
  appear in the token's `roles` claim.

## Chart layout

```
api-portal-ui-helm-chart/
├── templates/
│   ├── configmap.yaml     # /app/configs/config.toml ([api_portal.*])
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── pvc.yaml           # SQLite data volume
│   ├── certificate.yaml   # cert-manager Certificate
│   ├── issuer.yaml        # optional self-signed Issuer
│   ├── hpa.yaml
│   ├── pdb.yaml
│   └── _helpers.tpl       # shared apip.* helpers
├── values.yaml
├── values-local.yaml      # local development overrides
└── README.md
```

## Configuration

All configurable values are documented inline in `values.yaml`. Under the umbrella, prefix
every key below with `api-portal-ui.`.

- `image.*` / `imagePullSecrets` — container image and component-only pull secrets.
- `config.server.baseUrl` — public URL browsers use. Set it when a real hostname fronts the
  portal so login redirects and asset URLs are correct.
- `config.database.*` — the portal's own store. `driver` is `sqlite` (default, file on the
  PVC), `postgres`, or `mssql`. Aliases such as `sqlite3`, `postgresql`, and `sqlserver` are
  normalized. `host`/`port`/`database`/`user`, `sslmode` (`disable` | `verify-full`), and
  pool tuning also live here.
- `config.platformApi.baseUrl` / `insecure` — the Platform API endpoint (derived under the
  umbrella) and dev-only TLS skip.
- `config.auth.*` — `publicKeyPath` for the mounted Platform API key, `claimMappings`, and
  `idp.*` for OIDC login. OIDC is active when `idp.clientId` is set; otherwise login goes
  through the Platform API.
- `config.auth.authorization.*` — REST scope enforcement (`enabled`), `mode` (`role` default
  via `roleToScopeMapping`, or `scope`), and page role gating (`pageRoleValidation`,
  `portalRoles`).
- `config.organization.*` — org `handle` (URL slug, required), `displayName`, and
  auto-created subscription plans.
- `config.artifacts.enabledTypes` — allowlist of served types (`apis`, `mcp-servers`,
  `api-workflows`).
- `config.webhooksDelivery.*`, `config.uploads.*`, `config.tryout.*` — webhook delivery
  tuning, upload/archive limits, and the SSRF-sensitive "Try It" console. Private endpoints
  are denied by default.
- `configToml` — raw TOML appended to the generated config.
- `secrets.existingSecret` / `secrets.keys.*` / `secrets.hasPublicKey` /
  `secrets.hasIdpClientSecret` — the Secret, its key names, and which optional keys to wire.
- `service.*` / `containerPort` — Service type (default `LoadBalancer`) and port `9543`.
- `tls.*` — `certificateProvider` is `cert-manager` (default), `secret`, or `none`.
- `persistence.*` — SQLite PVC (1Gi, kept on uninstall).
- `deployment.*`, `hpa.*`, `podDisruptionBudget.*`, `configMap.*` — pod spec, autoscaling
  (requires `postgres`/`mssql`), PDB, and extra metadata.
