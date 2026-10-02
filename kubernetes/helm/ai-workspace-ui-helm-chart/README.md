# AI Workspace UI Helm Chart

This chart packages the **AI Workspace UI**: a React single-page app served by a small Go BFF
(backend for frontend). It is **stateless** — no database and no PVC — and talks only to the
Platform API.

It is an independently released **component chart** and is used in two ways:

| How | When |
| --- | --- |
| **As a subchart of the [`ai-workspace`](../ai-workspace-helm-chart/README.md) umbrella** (recommended) | The umbrella installs the Platform API next to the UI, creates the Secrets and the ServiceAccount, derives the Platform API URL, and checks that both sides use the same authorization mode. |
| **Standalone** | The Platform API is already running in another release or cluster. |

## Umbrella vs standalone

| | Under the umbrella | Standalone |
| --- | --- | --- |
| Values prefix | `ai-workspace-ui.<key>` (e.g. `ai-workspace-ui.config.auth.mode`) | `<key>` (e.g. `config.auth.mode`) |
| `enabled` | Umbrella dependency toggle | Ignored (the chart default is `false`, but the UI renders anyway). Use `deployment.enabled` to render nothing. |
| `global.*` | Set once by the umbrella. This chart's `global:` block is overridden. | This chart's own `global:` defaults apply |
| ServiceAccount | Created by the umbrella and shared | **None created**. Pods use the namespace `default` SA, or set `global.serviceAccount.name`. |
| Secret | Created by the umbrella's `generate-secrets.sh` (OIDC mode only) | You create it, only for OIDC mode |
| Platform API URL | Derived: `https://<release>-platform-api:<global.platformApi.port>` | **Set `config.controlPlane.url`**. The derived default assumes a Platform API in the *same* release. |
| Authorization mode check | Install fails if `config.auth.authorization.mode` ≠ `platform-api.config.auth.authorization.mode` | Not checked. Set it to match your Platform API manually. |

## Prerequisites

- Kubernetes 1.24+
- Helm 3.12+
- A reachable Platform API
- cert-manager (default certificate provider), **or** a pre-created TLS Secret. The BFF
  always serves HTTPS and doesn't generate a self-signed certificate.

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

Follow the [`ai-workspace` README](../ai-workspace-helm-chart/README.md). Override this
chart's values under the `ai-workspace-ui:` key:

```yaml
ai-workspace-ui:
  config:
    auth:
      mode: oidc
```

### Standalone

In the default `basic` auth mode (the Platform API's file-based login), **no Secret is
needed**. Run from `kubernetes/helm/`:

```bash
helm install ai-workspace-ui ./ai-workspace-ui-helm-chart -n ai-workspace --create-namespace \
  --set config.controlPlane.url=https://platform-api-platform-api.platform-api.svc:9243
```

From the published OCI chart:
```bash
helm install ai-workspace-ui oci://ghcr.io/wso2/api-platform/helm-charts/ai-workspace-ui \
  --version 1.0.0 -n ai-workspace --create-namespace \
  --set config.controlPlane.url=https://platform-api.example.com:9243
```

For developers using locally built / `latest` images (this also skips TLS verification to the
Platform API):
```bash
helm install ai-workspace-ui ./ai-workspace-ui-helm-chart -n ai-workspace --create-namespace \
  -f ai-workspace-ui-helm-chart/values-local.yaml \
  --set config.controlPlane.url=https://platform-api-platform-api.platform-api.svc:9243
```

#### OIDC mode

The BFF is a confidential OIDC client, so it needs a client secret:

```bash
kubectl create secret generic ai-workspace-ui-secrets -n ai-workspace \
  --from-literal=APIP_AIW_AUTH_OIDC_CLIENT_SECRET='<client-secret>'
```

```yaml
# oidc-values.yaml
secrets:
  existingSecret: ai-workspace-ui-secrets
config:
  auth:
    mode: oidc
    oidc:
      authority: https://idp.example.com
      clientId: ai-workspace
      # Both URLs must include the /ai-workspace prefix and match the IDP application.
      redirectUrl: https://workspace.example.com/ai-workspace/api/auth/callback
      postLogoutRedirectUrl: https://workspace.example.com/ai-workspace/login
      scope: "openid profile email"
```

## Accessing the UI

The Service defaults to `LoadBalancer` on port `9643`. The app is served under the
**`/ai-workspace`** path prefix, which is baked into the SPA bundle at image build time and
isn't a chart value:

```bash
kubectl port-forward svc/ai-workspace-ui-ai-workspace 9643:9643 -n ai-workspace
# then open https://localhost:9643/ai-workspace/
```

Behind an ingress, route a single `path: /ai-workspace, pathType: Prefix` rule to the Service,
with no rewriting. Several portals can then share one host. The probes use `/healthz` at the
origin root, so they don't depend on the ingress.

## Uninstalling the Chart

```bash
helm uninstall ai-workspace-ui -n ai-workspace
```

The UI keeps no persistent state, so there's no PVC to clean up.

## Upgrading the Chart

```bash
helm upgrade ai-workspace-ui ./ai-workspace-ui-helm-chart -n ai-workspace -f custom-values.yaml
```

## Verifying the Installation

```bash
helm status ai-workspace-ui -n ai-workspace
kubectl get all -l app.kubernetes.io/instance=ai-workspace-ui -n ai-workspace
kubectl logs -l app.kubernetes.io/component=ai-workspace -n ai-workspace
```

### Troubleshooting

- **`aiWorkspace.secrets.existingSecret is required ...`** — `config.auth.mode=oidc` needs the
  client-secret Secret (see [OIDC mode](#oidc-mode)). Under the umbrella, re-run
  `generate-secrets.sh` with `APIP_AIW_AUTH_OIDC_CLIENT_SECRET` set.
- **`tls.certificateProvider must be "cert-manager" or "secret"`** — the BFF has no
  self-signed fallback and no plain-HTTP-only mode. To add a plain-HTTP listener for a
  TLS-terminating proxy, set `config.server.http.enabled=true`. HTTPS keeps running on
  `containerPort`.
- **UI loads but API calls fail with a TLS / certificate error** — the BFF verifies the
  Platform API's certificate. Mount the CA and set `config.controlPlane.caFile`
  (`deployment.extraVolumes` / `extraVolumeMounts`). For development only, set
  `config.controlPlane.tlsSkipVerify=true`.
- **UI shows the wrong permissions** — `config.auth.authorization.mode` (and
  `roleToScopeMapping` in `role` mode) must mirror the Platform API's settings. The Platform
  API enforces authorization; the UI only reflects it.
- **OIDC callback errors** — `redirectUrl` / `postLogoutRedirectUrl` are missing the
  `/ai-workspace` prefix, or differ from what's registered on the IDP.

## Chart layout

```
ai-workspace-ui-helm-chart/
├── templates/
│   ├── configmap.yaml     # /etc/ai-workspace/config.toml ([ai_workspace.*])
│   ├── deployment.yaml
│   ├── service.yaml
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
every key below with `ai-workspace-ui.`.

- `image.*` / `imagePullSecrets` — container image and component-only pull secrets.
- `config.defaultOrgRegion` — region assigned to a new organization on first login.
- `config.server.domain` / `config.server.http.*` — host shown in the startup banner, and an
  optional plain-HTTP listener.
- `config.logging.*` — `level`, `format`, and `browserDebug` (verbose SPA console logging).
- `config.controlPlane.*` — `url` (derived under the umbrella), `tlsSkipVerify` (dev only),
  and `caFile`.
- `config.gateway.controlplaneHost` — the **public** host:port that deployed gateways dial to
  reach the Platform API. It's shown verbatim in the "add a gateway" instructions, so it must
  be externally reachable.
- `config.gateway.platformGatewayVersions` — JSON list of gateway versions offered in the
  create-gateway dropdown.
- `config.session.*` — session store (`memory` only), idle timeout, and absolute TTL.
- `config.auth.mode` — `basic` (Platform API file login) or `oidc` (`config.auth.oidc.*` plus
  the Secret).
- `config.auth.claimMappings.*` — must agree with the Platform API's claim mappings.
- `config.auth.authorization.*` — `scope` | `role`. Must match the Platform API.
- `configToml` — raw TOML appended to the generated config.
- `secrets.existingSecret` / `secrets.keys.oidcClientSecret` — OIDC client secret reference.
- `service.*` / `containerPort` — Service type (default `LoadBalancer`) and port `9643`.
- `tls.*` — `certificateProvider` is `cert-manager` (default) or `secret`.
- `deployment.*`, `podDisruptionBudget.*`, `configMap.*` — pod spec, PDB, and extra metadata.
- `hpa.*` — autoscaling. The UI is stateless, so unlike the Platform API and API Portal it
  can scale with no database requirement. Set CPU requests first.
