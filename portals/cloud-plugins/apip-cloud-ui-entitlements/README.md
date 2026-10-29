# `apip-cloud-ui-entitlements`

Cloud entitlement UI for the **API Control Plane console only**.

## Why this is separate from `apip-cloud-ui-trial-status`

`apip-cloud-ui-trial-status` is registered in **both** host portals
(`apip-cloud-ui/src/hosts/api-control-plane.tsx` and `.../ai-workspace.tsx`), so
any change to it alters AI Workspace's UI too. AI Workspace's cloud version is
not being released, and its UI must not change — so this work lives in its own
package, registered only in the API Control Plane host.

That keeps the separation structural rather than conventional: a plugin absent
from `hosts/ai-workspace.tsx` cannot reach AI Workspace, whatever it contains.

`apip-cloud-ui-trial-status` stays exactly as it is and continues to serve both
portals. It keeps its own (older) trial badge, including the hardcoded
`TOTAL_TRIAL_DAYS = 14` against a 30-day product — a known-wrong number
deliberately left in place, because fixing it would change AI Workspace.

## What lives here

| File | Role |
| --- | --- |
| `TrialStatusFeature.tsx` | The header badge, rendered with oxygen-ui's `TrialBadge` |
| `trialDays.ts` | Trial arithmetic, kept out of the component so it unit-tests without pulling in the UI library |
| `hostPort.ts` | The slice of the host Port this plugin needs, hand-mirrored rather than imported (separate repos) |
| `types.ts` | The billing shapes this plugin reads |

Still to come (plan steps 4-7): the `app.gate` entitlements gate, the shared
memo store, `ReadOnlyNotice`, and the usage panel feeding `TrialBadge`'s
`usage` prop.

## Dependency note

This package is on `@wso2/oxygen-ui` **0.18.0** — the version that introduced
`TrialBadge` — while the other cloud plugins remain on 0.5.0.

Its `tsconfig.json` therefore resolves `@wso2/oxygen-ui` through
`../../api-control-plane/node_modules`, **not** `../../ai-workspace/node_modules`
like the other plugins do. That matters: the plugin tsconfigs hardcode a portal's
dependency tree, so a plugin pointed at AI Workspace compiles against AI
Workspace's oxygen version regardless of its own `package.json`.
