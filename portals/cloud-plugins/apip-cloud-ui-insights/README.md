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
```

## Embed modes (`embedProfile`)

| Profile | Org iframe | Project iframe |
|---------|------------|----------------|
| `api-control-plane` (default) | `/wrap/basic#auth=post` | Org URL while `ENABLE_ACP_PROJECT_FILTER` is `false`; `/wrap/basic?project_id=…` when `true` (resolve failure still falls back to org) |
| `ai-workspace` | `/wrap/basic/ai-overview?embedded_ui=true&isolated_section=true#auth=post` | **same URL** (no project_id filter) |

Flip `ENABLE_ACP_PROJECT_FILTER` in `src/config/acpProjectFilter.ts` to re-enable ACP project filtering.

Do **not** put `/wrap/basic` or `/ai-overview` in env — only the Moesif origin.

## Host registration

- **API Control Plane** — org/project sidebar via `apip-cloud-ui/src/hosts/api-control-plane.tsx`
- **AI Workspace** — `page.insights` override (like gateways) via `hosts/ai-workspace.tsx`,
  registered only when `isInsightsMoesifConfigured()` is true. Otherwise
  `InsightsRoute` keeps the built-in Insights page (no override registered).
