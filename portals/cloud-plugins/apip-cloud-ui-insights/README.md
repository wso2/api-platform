# APIP Cloud UI — Insights

Shared Moesif Insights embed for **API Control Plane** and **AI Workspace**.
The plugin is the same; each host registry picks an `embedProfile` so the
iframe *path* differs. The Moesif *origin* is runtime config; the path is not.

## Layout

```
src/
  InsightsFeature.tsx      # extension entry (scope resolution)
  InsightsEmbed.tsx        # Moesif wrap/basic iframe handshake
  hostPort.ts              # host port contract
  types.ts                 # shared types (includes InsightsEmbedProfile)
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

## Embed modes (`embedProfile`)

| Profile | Org iframe | Project iframe |
|---------|------------|----------------|
| `api-control-plane` (default) | `/wrap/basic#auth=post` | `/wrap/basic?project_id=…` (falls back to org if resolve fails) |
| `ai-workspace` | `/wrap/basic/ai-overview?embedded_ui=true&isolated_section=true#auth=post` | **same URL** (no project_id filter) |

Do **not** put `/wrap/basic` or `/ai-overview` in env — only the Moesif origin.

## Host registration

- **API Control Plane** — org/project sidebar via `apip-cloud-ui/src/hosts/api-control-plane.tsx`
- **AI Workspace** — `page.insights` override (like gateways) via `hosts/ai-workspace.tsx`.
  If Moesif origin is not in runtime config, `InsightsRoute` keeps the built-in
  Insights page instead of showing a configuration error.
