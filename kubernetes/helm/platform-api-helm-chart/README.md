# Platform API Helm Chart

This chart packages the **Platform API** — the shared Go control plane of the WSO2 API
Platform. Gateways register with it, and both portals (AI Workspace and API Portal)
authenticate against it.

It is an independently released **component chart** and is used in two ways:

| How | When |
| --- | --- |
| **As a subchart of a product umbrella** (recommended) | [`ai-workspace`](../ai-workspace-helm-chart/README.md) or [`api-portal`](../api-portal-helm-chart/README.md). The umbrella creates the Secrets (`generate-secrets.sh`) and the ServiceAccount, and wires the portal to this service automatically. |
| **Standalone** | You only need the control plane, or you run the portals in a different release/cluster. |

## Umbrella vs standalone

Everything in this README applies in both modes. Only **where you write the keys** and
**who provides the shared pieces** differ.

| | Under an umbrella | Standalone |
| --- | --- | --- |
| Values prefix | `platform-api.<key>` (e.g. `platform-api.config.database.driver`) | `<key>` (e.g. `config.database.driver`) |
| `enabled` | Umbrella dependency toggle | Ignored. Use `deployment.enabled` to render nothing. |
| `global.*` | Set once by the umbrella. This chart's `global:` block is overridden. | This chart's own `global:` defaults apply |
| ServiceAccount | Created by the umbrella and shared | **None created** (`global.serviceAccount.create=false`). Pods use the namespace `default` SA, or set `global.serviceAccount.name` to an existing SA. |
| Secret | Created by the umbrella's `generate-secrets.sh` | You create it ([below](#standalone-step-1-create-the-secret)) |
| Portal → Platform API URL | Derived automatically: `https://<release>-platform-api:<global.platformApi.port>` | Portals in another release must set their URL explicitly |

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+
- cert-manager (default certificate provider), **or** a pre-created TLS Secret. The Platform
  API has **no self-signed fallback**, so HTTPS needs a real certificate source.
- `openssl` and `htpasswd` (or `docker`) to generate the Secret material when installing
  standalone.

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

### Via a product umbrella

Follow the umbrella README. Override this chart's values under the `platform-api:` key in
your umbrella values file:

```yaml
platform-api:
  config:
    logging:
      level: debug
```

### Standalone, Step 1: Create the Secret

The chart never creates or embeds secret values. The render **fails** unless
`secrets.existingSecret` names a Secret in the release namespace. For the default
`auth.mode=file`, that Secret needs:

| Key | Required when |
| --- | --- |
| `APIP_CP_ENCRYPTION_KEY` | Always (64-char hex AES-256 key) |
| `jwt_public.pem` | `auth.mode` `internal_token` or `file` (not with `internalToken.skipValidation=true`); any mode when [service accounts](#service-accounts) are on without their own pair |
| `jwt_private.pem` | `auth.mode=file`; any mode when service accounts are on without their own pair |
| `APIP_CP_ADMIN_USERNAME` / `APIP_CP_ADMIN_PASSWORD_HASH` | `auth.mode=file` (bcrypt hash; there is no `admin/admin` default) |
| `APIP_CP_DATABASE_PASSWORD` | `config.database.driver` is `postgres` / `sqlserver` |
| `APIP_CP_WEBHOOK_SECRET` | `config.webhook.enabled=true` |
| `sa_jwt_public.pem` / `sa_jwt_private.pem` | `config.auth.serviceAccount.jwt` is set (see [Service accounts](#service-accounts)) |

Each value is written to its own file in a private temporary directory. The directory is
passed with `--from-file`, which turns each file name into a Secret key. Secret values never
appear in shell history or in `kubectl`'s command-line arguments.

```bash
kubectl create namespace platform-api
SECRET_DIR=$(umask 077; mktemp -d)
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$SECRET_DIR/jwt_private.pem"
openssl rsa -in "$SECRET_DIR/jwt_private.pem" -pubout -out "$SECRET_DIR/jwt_public.pem"
openssl rand -hex 32 | tr -d '\n' > "$SECRET_DIR/APIP_CP_ENCRYPTION_KEY"
printf '%s' admin > "$SECRET_DIR/APIP_CP_ADMIN_USERNAME"
ADMIN_PASSWORD="$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9' | cut -c1-20)"
printf '%s' "$ADMIN_PASSWORD" | htpasswd -niB -C 10 "" | cut -d: -f2 | tr -d '\r\n' \
  > "$SECRET_DIR/APIP_CP_ADMIN_PASSWORD_HASH"
kubectl create secret generic platform-api-secrets -n platform-api --from-file="$SECRET_DIR"
echo "admin password: $ADMIN_PASSWORD"   # store it now
cp "$SECRET_DIR/jwt_public.pem" .        # only if a portal must verify tokens
rm -rf "$SECRET_DIR"; unset ADMIN_PASSWORD
```

For a database password or webhook secret, read it with `read -rsp` and write it to
`$SECRET_DIR/APIP_CP_DATABASE_PASSWORD` or `$SECRET_DIR/APIP_CP_WEBHOOK_SECRET` before
running `kubectl create secret`.

The key names can be changed through `secrets.keys.*`.

For [service accounts](#service-accounts) with their own key pair, add one more pair to
`$SECRET_DIR` before `kubectl create secret`:

```bash
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$SECRET_DIR/sa_jwt_private.pem"
openssl rsa -in "$SECRET_DIR/sa_jwt_private.pem" -pubout -out "$SECRET_DIR/sa_jwt_public.pem"
```

### Standalone, Step 2: Install the chart

Run from `kubernetes/helm/`. The commands assume the release **and** namespace are both
named `platform-api`. Resource names are prefixed with the release name
(`<release>-platform-api`, `<release>-platform-api-data`); substitute your own if they differ.

Install with default values:
```bash
helm install platform-api ./platform-api-helm-chart -n platform-api \
  --set secrets.existingSecret=platform-api-secrets
```

Install from the published OCI chart:
```bash
helm install platform-api oci://ghcr.io/wso2/api-platform/helm-charts/platform-api \
  --version 0.16.0 -n platform-api --set secrets.existingSecret=platform-api-secrets
```

For developers using locally built / `latest` images:
```bash
helm install platform-api ./platform-api-helm-chart -n platform-api \
  -f platform-api-helm-chart/values-local.yaml \
  --set secrets.existingSecret=platform-api-secrets
```

Install with a custom values file (it must set `secrets.existingSecret`):
```bash
helm install platform-api ./platform-api-helm-chart -n platform-api -f custom-values.yaml
```

## Uninstalling the Chart

```bash
helm uninstall platform-api -n platform-api
```

The data PVC (`<release>-platform-api-data`) has `helm.sh/resource-policy: keep` and
**survives uninstall**. A reinstall with the same release name reuses it. Delete it with
`kubectl delete pvc platform-api-platform-api-data -n platform-api` to wipe the data. To have
Helm delete the PVC on uninstall, set `persistence.annotations: {}`.

## Upgrading the Chart

```bash
helm upgrade platform-api ./platform-api-helm-chart -n platform-api \
  --set secrets.existingSecret=platform-api-secrets -f custom-values.yaml
```

## Verifying the Installation

```bash
helm status platform-api -n platform-api
kubectl get all -l app.kubernetes.io/instance=platform-api -n platform-api
kubectl logs -l app.kubernetes.io/component=platform-api -n platform-api

kubectl port-forward svc/platform-api-platform-api 9243:9243 -n platform-api
curl -k https://localhost:9243/health
```

### Troubleshooting

- **`platformApi.secrets.existingSecret is required`** — set `secrets.existingSecret` (under
  an umbrella: pass `-f values-secrets.yaml`).
- **`config.server.https.enabled=true requires tls.certificateProvider to be "cert-manager"
  or "secret"`** — there's no self-signed fallback. Install cert-manager or provide a TLS
  Secret. To serve plain HTTP behind a TLS-terminating proxy, set
  `config.server.http.enabled=true` and `config.server.https.enabled=false`. Under an
  umbrella, also set `global.platformApi.tlsEnabled=false` and `global.platformApi.port` to
  match.
- **`deployment.replicaCount > 1 requires an external database` / HPA render failure** —
  SQLite is a per-pod file. Switch `config.database.driver` to `postgres` or `sqlserver`
  before scaling.
- **Pod `CrashLoopBackOff` right after install** — usually a Secret key that the selected
  auth mode needs is missing (see the table in Step 1). It can also be an ap: scope in
  `config.auth.authorization.roles` that the OpenAPI spec doesn't declare. Check
  `kubectl logs`.
- **Service-account errors** — see [Service accounts](#service-accounts), *Troubleshooting*.
- **A portal can't call the Platform API (TLS error)** — the default Issuer is self-signed and
  its certificate covers only `platform-api.localhost`. For production, use a trusted issuer
  (`tls.certManager.createIssuer=false`, `tls.certManager.issuerRef`). List the in-cluster
  Service names (`<release>-platform-api`, `<release>-platform-api.<namespace>.svc`) in
  `tls.certManager.dnsNames`.

## Chart layout

```
platform-api-helm-chart/
├── templates/
│   ├── configmap.yaml     # config-platform-api.toml + role-to-scope mapping
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── pvc.yaml           # SQLite data volume
│   ├── certificate.yaml   # cert-manager Certificate
│   ├── issuer.yaml        # optional self-signed Issuer
│   ├── hpa.yaml
│   ├── pdb.yaml
│   └── _helpers.tpl       # shared apip.* helpers (naming, labels, images, secrets)
├── values.yaml
├── values-local.yaml      # local development overrides
└── README.md
```

## Configuration

All configurable values are documented inline in `values.yaml`. Under an umbrella, prefix
every key below with `platform-api.`.

- `image.*` / `imagePullSecrets` — container image and component-only pull secrets (merged
  with `global.imagePullSecrets`). `global.wso2.subscription.imagePullSecret` switches to the
  `registry.wso2.com` images.
- `config.*` — rendered into `/etc/platform-api/config-platform-api.toml` under
  `[platform_api.*]`:
  - `config.database.*` — `driver` is `sqlite3` (default, file on the PVC), `postgres`, or
    `sqlserver`. Aliases such as `postgresql`, `pgx`, and `mssql` are normalized. Connection
    settings for both server drivers live under `config.database.postgres.*` (set `port: 1433`
    for SQL Server). The password always comes from the Secret.
  - `config.auth.mode` — `file` (default: local admin login plus RS256 tokens),
    `internal_token` (verify RS256 tokens minted elsewhere), or `idp` (external JWKS; no local
    PEMs mounted unless service accounts are on).
  - `config.auth.authorization.*` — scope enforcement, `scope` | `role` mode, and the
    `roles` → scopes table rendered into the mounted role-to-scope mapping (`ap_admin`
    and the service-account role `ap_sa_reader` ship by default). Under the `ai-workspace` umbrella, this `mode` must equal
    `ai-workspace-ui.config.auth.authorization.mode`.
  - `config.auth.file.*` — the default organization and the file-mode admin's roles. The
    username and password hash come from the Secret.
  - `config.auth.claimMappings.*` / `config.auth.jwt.*` / `config.auth.idp.*` — claim
    names, token issuer/TTL and PEM mount paths, and the IDP JWKS/issuer/audience.
  - `config.auth.serviceAccount.*` — service accounts: off by default. See
    [Service accounts](#service-accounts).
  - `config.server.*` — HTTPS/HTTP listeners, timeouts, CORS `allowedOrigins` (explicit
    origins only, never `*`), and WebSocket limits.
  - `config.gateway.*`, `config.deployments.*`, `config.eventHub.*`, `config.webhook.*` —
    gateway registration checks, deployment limits/timeouts, EventHub polling, and the signed
    webhook receiver for API Portal events.
- `configToml` — raw TOML appended to the generated config, for keys not modeled above. It's
  a ConfigMap, so use `{{ env "NAME" }}` tokens plus `deployment.extraEnv` for secret values.
- `secrets.existingSecret` / `secrets.keys.*` — the required Secret and the key names inside
  it.
- `service.*` / `containerPort` — Service type (default `ClusterIP`), port `9243`, and network
  tuning. Under an umbrella, keep `service.port` equal to `global.platformApi.port`.
- `tls.*` — `certificateProvider` is `cert-manager` (default, self-signed Issuer unless
  `createIssuer=false` with `issuerRef`) or `secret` (existing `tls.crt`/`tls.key`).
- `persistence.*` — PVC for the SQLite file (1Gi, `ReadWriteOnce`, kept on uninstall). It can
  be disabled with an external database.
- `deployment.*` — replicas, probes (`/health` over HTTPS), resources, security contexts,
  scheduling, `extraEnv`/`extraEnvFrom`/`extraVolumes`/`extraVolumeMounts`.
- `hpa.*` / `podDisruptionBudget.*` — autoscaling and the PDB. The HPA requires `postgres`
  or `sqlserver`, plus CPU requests.
- `configMap.*`, `global.commonLabels`, `global.commonAnnotations` — extra metadata.

### How secrets reach the config

Secret values never appear in the ConfigMap. Each Secret key is named **exactly** like the
environment variable it becomes and the `{{ env "..." }}` token in the config that reads it.
For example, `APIP_CP_DATABASE_PASSWORD` is the Secret key, the env var, and the config token.
The RS256 keys are the exception: they're mounted as files at `config.auth.jwt.publicKeyFile` /
`privateKeyFile`, and only for the auth modes that read them.

### Storage backends and scaling

| `config.database.driver` | Replicas | HPA |
| --- | --- | --- |
| `sqlite3` (default) | 1 only — the DB file lives on the PVC | Refused at render |
| `postgres` | Multi-replica | Supported |
| `sqlserver` | Multi-replica | Supported |

## Service accounts

A service account is a machine identity that belongs to no person. An administrator
creates one and gets a client ID and secret. The workload swaps them for a short-lived
token at `POST /api/v0.9/service-accounts/token` and calls the API with it. The endpoints
are described in
[`docs/rest-apis/platform-api/service-accounts.md`](../../../docs/rest-apis/platform-api/service-accounts.md).

The feature is **off by default**. Turn it on with
`config.auth.serviceAccount.enabled: true`.

### What each auth mode needs

The Platform API signs service-account tokens itself, so it needs a private key. By
default it reuses the `auth.jwt` pair (`jwt_public.pem` / `jwt_private.pem`). When
service accounts are on, the chart mounts that pair in **every** mode, including `idp`, so
the Secret must hold both PEMs. `generate-secrets.sh` always creates them.

An own pair is better in production:

- The shared `auth.jwt` key can't be changed without breaking every live token at once; an
  own pair can, using retired keys.
- In `internal_token` mode, `auth.jwt`'s private key belongs to whatever mints your tokens.
  Without an own pair, the Platform API must hold that key too.

```yaml
config:
  auth:
    serviceAccount:
      enabled: true
      jwt:
        issuer: platform-api-sa          # must differ from auth.jwt.issuer and auth.idp.issuer
        publicKeyFile: /etc/platform-api/keys/sa_jwt_public.pem
        privateKeyFile: /etc/platform-api/keys/sa_jwt_private.pem
```

The two PEMs come from the Secret, under `secrets.keys.saJwtPublicKey` /
`saJwtPrivateKey` (`sa_jwt_public.pem` / `sa_jwt_private.pem`). They are mounted into the
same directory as the `auth.jwt` pair, so keep both pairs in one directory with distinct
file names.

### Databases

On `sqlite3` the Platform API creates the tables itself. On `postgres` and `sqlserver` it
never runs DDL, so create the tables **before** turning the feature on:

1. Apply the `service_accounts` and `service_account_revocations` tables, and their
   indexes, from `schema.postgres.sql` or `schema.sqlserver.sql`. Both are in the image at
   `/app/internal/database/`.
2. Set `config.auth.serviceAccount.enabled: true` and run `helm upgrade`.

Doing step 2 without step 1 leaves every pod in `CrashLoopBackOff`. Set `enabled: false`
again to recover.

### Changing the key

Only an own pair can be changed without an outage. The mount paths stay the same; only
the Secret keys behind them change.

1. Add three keys to the Secret: the new pair (say `sa_jwt_public_v2.pem`,
   `sa_jwt_private_v2.pem`), and a copy of the current public key (`sa_jwt_public_v1.pem`).
2. Point the chart at them. Each retired file is read from the Secret key named like the
   file:

   ```yaml
   secrets:
     keys:
       saJwtPublicKey: sa_jwt_public_v2.pem
       saJwtPrivateKey: sa_jwt_private_v2.pem
   config:
     auth:
       serviceAccount:
         retiredPublicKeyFiles:
           - /etc/platform-api/keys/sa_jwt_public_v1.pem
   ```

3. `helm upgrade`. New tokens are signed with the new key. The retired key still verifies
   tokens and stays in the JWKS, but never signs.
4. After one `tokenTtl`, plus the refresh interval of anything that caches the JWKS, drop
   the retired entry and its Secret key.

### Troubleshooting

Look for these in `kubectl logs`:

- **`failed to load service-account revocations`** — the tables are missing. See
  [Databases](#databases).
- **`existing identities use the reserved "sa:" subject prefix`** — a user whose name
  starts with `sa:` already exists. Rename it before turning service accounts on.
- **`service-account role ... which a service account may never hold`** — an `ap_sa_*`
  role in `config.auth.authorization.roles` grants `ap:service_account:manage`,
  `ap:service_account:read` or `ap:api_key:all:manage`. Remove the scope.
- **`auth.service_account.jwt.issuer must differ from ...`** — the own pair's `issuer`
  equals `config.auth.jwt.issuer` (default `platform-api`) or one of
  `config.auth.idp.issuer`. Pick another, e.g. `platform-api-sa`.
- **`retired_public_key_files requires [auth.service_account.jwt]`** — retired keys work
  only with an own pair.
- **`is a service-account role`** — `config.auth.file.admin.roles` names an `ap_sa_*` role.
  Those are for service accounts only; use another role.
- **`no ap_sa_* role; no service account can be created`** (a warning) — the role mapping
  has no `ap_sa_*` role. Keep `ap_sa_reader`, or add your own.
