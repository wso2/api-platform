# APIP Cloud UI — Insights

Shared Moesif Insights embed for **API Control Plane** and **AI Workspace**.

## Layout

```
src/
  InsightsFeature.tsx      # extension entry (scope resolution)
  InsightsEmbed.tsx        # Moesif wrap/basic iframe handshake
  hostPort.ts              # host port contract
  types.ts                 # shared types
  api/
    analyticsApi.ts        # WSO2 Cloud analytics endpoints
  config/
    runtimeConfig.ts       # BFF proxy + Moesif URL resolution
  utils/
    moesifEmbed.ts         # iframe URL builders + postMessage types
    routeParams.ts         # pathname → org/project handles
  components/
    StateViews.tsx         # loading / error states
overlays/
  api-control-plane/
    InsightsPage.tsx       # cloud Docker overlay (API scope Coming soon)
```

## Cloud backend integration

Production deployments must **not** mint Moesif viewer tokens in the browser. This plugin calls WSO2 Cloud platform-api-service through the portal BFF same-origin proxy:

| Endpoint | Purpose |
|----------|---------|
| `GET /proxy/cloud/analytics/id-token` | Dashboard-viewer token for the authenticated org |

These map to `wso2cloud/backend/core/internal/moesifmapping/handler/handler.go`.

Org context for the viewer token is resolved server-side from the BFF session.

## Runtime configuration

Inject via BFF runtime config (`window.__RUNTIME_CONFIG__`) or Vite env:

| Key | Description |
|-----|-------------|
| `moesifAppUrl` | Moesif wrap iframe origin |
| `platformApiBaseUrl` | BFF proxy prefix (default `/proxy`) |
| `platformApiVersion` | Platform API version segment (default `v0.9`) |

## Host registration

- **API Control Plane** — org/project sidebar extensions via `apip-cloud-ui/src/hosts/api-control-plane.tsx`; API Insights route shows Coming soon (same as Compliance)
- **AI Workspace** — sidebar extension via `apip-cloud-ui/src/hosts/ai-workspace.tsx`

## Embed modes

| Scope | Iframe |
|-------|--------|
| Organization | `{moesifAppUrl}/wrap/basic#auth=post` |
| Project | `{moesifAppUrl}/wrap/basic?project_id=...#auth=post` |
| API | Coming soon (built-in ACP route; not embedded by this plugin) |
