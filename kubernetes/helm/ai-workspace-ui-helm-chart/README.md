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
  serves HTTPS by default and doesn't generate a self-signed certificate. Neither is needed
  when [TLS is terminated in front of the BFF](#terminating-tls-in-front-of-the-bff).

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
needed**. Run from `kubernetes/helm/`. The commands assume release `ai-workspace-ui` in
namespace `ai-workspace`. Resource names are prefixed with the release name
(`<release>-ai-workspace`); substitute your own if they differ.

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

The BFF is a confidential OIDC client, so it needs a client secret. Create the Secret
**before** installing. The secret is read without echo and passed through a private
temporary file, so it never appears in shell history or in `kubectl`'s arguments:

```bash
kubectl create namespace ai-workspace       # skip if it already exists
SECRET_DIR=$(umask 077; mktemp -d)
read -rsp 'OIDC client secret: ' OIDC_SECRET; echo
printf '%s' "$OIDC_SECRET" > "$SECRET_DIR/APIP_AIW_AUTH_OIDC_CLIENT_SECRET"
kubectl create secret generic ai-workspace-ui-secrets -n ai-workspace --from-file="$SECRET_DIR"
rm -rf "$SECRET_DIR"; unset OIDC_SECRET
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

```bash
helm install ai-workspace-ui ./ai-workspace-ui-helm-chart -n ai-workspace \
  -f oidc-values.yaml \
  --set config.controlPlane.url=https://platform-api-platform-api.platform-api.svc:9243
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

### Terminating TLS in front of the BFF

The BFF has two listeners, and the Service exposes whichever are enabled:

| Listener | Pod port | Service port (name) | On when |
| --- | --- | --- | --- |
| HTTPS | `containerPort` (9643) | `service.port` (`https`) | `config.server.https.enabled=true` (default) |
| Plain HTTP | `config.server.http.port` (9080) | `service.httpPort` (`http`) | `config.server.https.enabled=false`, or `config.server.http.enabled=true` |

When an OpenShift edge Route, an Ingress controller or a service mesh terminates TLS, set
`config.server.https.enabled=false`. Only the plain-HTTP listener runs, and the probes and the
Route move to it. The chart no longer needs a certificate: `tls.*` is ignored and no
cert-manager objects are created. Point an Ingress at Service port `9080` (`http`).

```yaml
service:
  type: ClusterIP              # don't publish the plain-HTTP port directly
config:
  server:
    https:
      enabled: false
openshift:
  enabled: true
  routes:
    tls:
      termination: edge        # the default while HTTPS is off; targets port http
      insecureEdgeTerminationPolicy: Redirect
      # certificate / key / caCertificate: served by the router; empty uses its default
```

Browsers must still reach the UI over HTTPS, because the session cookies are always `Secure`.
The hop between the proxy and the pod is plain HTTP, so restrict who can reach the Service
(for example with a `NetworkPolicy`).

To keep HTTPS and add the plain-HTTP listener next to it (for example HTTPS for direct clients
and an edge Route for browsers), set `config.server.http.enabled=true` instead. Each Route
termination then goes to the matching listener: edge to `http`, passthrough and
reencrypt to `https`.

To move a running HTTPS install behind a TLS-terminating proxy without an outage, change one
thing at a time. Changing the Route and turning HTTPS off in one upgrade interrupts traffic
for a few seconds while the router and the pods switch over.

1. `config.server.http.enabled=true`: adds the plain-HTTP listener; the Route or Ingress is
   unchanged.
2. Move the proxy to the plain-HTTP listener: `openshift.routes.tls.termination=edge` for
   the chart's Route, or, for your own Ingress, Service port `9080` with a plain-HTTP backend
   (for ingress-nginx, drop `nginx.ingress.kubernetes.io/backend-protocol: HTTPS`).
3. `config.server.https.enabled=false`: removes the HTTPS listener and its certificate.

## Uninstalling the Chart

```bash
helm uninstall ai-workspace-ui -n ai-workspace
```

The UI keeps no persistent state, so there's no PVC to clean up.

## Upgrading the Chart

```bash
helm upgrade ai-workspace-ui ./ai-workspace-ui-helm-chart -n ai-workspace -f custom-values.yaml
```

### Port names changed

Earlier releases named the HTTPS port `http`. It is now `https`, and `http` names the
plain-HTTP port (9080), which exists only while that listener is on. The port numbers are
unchanged. Before upgrading, update anything you created yourself that refers to the port by
name:

- an Ingress with `backend.service.port.name: http`: use `https`, or `number: 9643`;
- a Route, `ServiceMonitor` or `NetworkPolicy` with `targetPort: http`: use `https`.

The chart's own Route and probes are updated for you. A probe override still written as
`port: http` with `scheme: HTTPS` is rendered against `https`.

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
- **`tls.certificateProvider must be "cert-manager" or "secret"`**: the BFF has no
  self-signed fallback. Supply a certificate, or, if a proxy terminates TLS, set
  `config.server.https.enabled=false` (see
  [Terminating TLS in front of the BFF](#terminating-tls-in-front-of-the-bff)).
- **`edge termination forwards plain HTTP ...` / `... termination needs the HTTPS listener`**:
  edge needs the plain-HTTP listener, and passthrough and reencrypt need HTTPS. Leave
  `openshift.routes.tls.termination` empty to pick the one that matches
  `config.server.https.enabled`.
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
│   ├── route.yaml         # OpenShift Route (openshift.enabled)
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
- `config.server.domain`: host shown in the startup banner.
- `config.server.https.enabled`: `true` (default) serves HTTPS on `containerPort`; `false`
  turns it off, leaving only the plain-HTTP listener behind a TLS-terminating proxy.
- `config.server.http.*`: the plain-HTTP listener on `port` (9080). Always on while HTTPS is
  off; `enabled=true` runs it next to HTTPS.
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
- `service.*` / `containerPort` — Service type (default `LoadBalancer`), the HTTPS port
  `9643` (`service.port`, name `https`) and the plain-HTTP port `9080` (`service.httpPort`,
  name `http`). Each is present only while its listener is on.
- `tls.*` — `certificateProvider` is `cert-manager` (default) or `secret`. Ignored while
  `config.server.https.enabled=false`.
- `deployment.*`, `podDisruptionBudget.*`, `configMap.*` — pod spec, PDB, and extra metadata.
- `hpa.*` — autoscaling. The UI is stateless, so unlike the Platform API and API Portal it
  can scale with no database requirement. Set CPU requests first.
- `openshift.*`: an OpenShift Route to the UI (`openshift.routes.aiWorkspace`), rendered only
  when `openshift.enabled=true`. `passthrough` (default) and `reencrypt` go to the HTTPS
  listener; `edge` (the default with `config.server.https.enabled=false`) goes to the
  plain-HTTP listener. Security contexts are set through `deployment.podSecurityContext` /
  `deployment.securityContext`, rendered as given.
