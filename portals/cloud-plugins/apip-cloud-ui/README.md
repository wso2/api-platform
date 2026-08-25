# APIP Cloud UI

Shared cloud-plugin registration for API Platform portals. `src/plugin.ts` is
host-agnostic: it selects one version per plugin ID and flattens that release's
extensions. Each module under `src/hosts` registers features against a portal's
native extension contract.

## Layout

- `src/hosts/ai-workspace.tsx` registers the Pipelines feature for AI Workspace.
- `src/hosts/api-control-plane.tsx` is the API Control Plane registry; it is empty
  until a control-plane cloud feature is added.
- `../apip-cloud-ui-pipelines` contains the reusable Pipelines UI.

AI Workspace consumes this package through a local file dependency and re-exports
its registry from `portals/ai-workspace/src/cloud/index.ts`. Both the web app and
plugins therefore build from one api-platform revision.

```ts
import { aiWorkspaceCloudExtensions } from '@wso2-enterprise/apip-cloud-ui';
```

To add another AI Workspace plugin, create a sibling package in
`portals/cloud-plugins`, add it as a dependency here, and register its extension in
`src/hosts/ai-workspace.tsx`.
