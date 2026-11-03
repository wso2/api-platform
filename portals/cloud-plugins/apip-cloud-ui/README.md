# APIP Cloud UI

Shared cloud-plugin registration for API Platform portals. `src/plugin.ts` is
host-agnostic: it selects one version per plugin ID and flattens that release's
extensions. Each module under `src/hosts` registers features against a portal's
native extension contract.

## Layout

- `src/hosts/ai-workspace.tsx` registers the Pipelines and Deploy features for AI Workspace.
- `src/hosts/api-control-plane.tsx` is the API Control Plane registry; it is empty
  until a control-plane cloud feature is added.
- `../apip-cloud-ui-pipelines` contains the reusable Pipelines UI.
- `../apip-cloud-ui-deploy` contains the reusable Deploy UI.

AI Workspace consumes this package through a local file dependency and re-exports
its registry from `portals/ai-workspace/src/cloud/index.ts`. Both the web app and
plugins therefore build from one api-platform revision.

```ts
import { aiWorkspaceCloudExtensions } from '@wso2-enterprise/apip-cloud-ui';
```

To add another AI Workspace plugin, create a sibling package in
`portals/cloud-plugins`, add it as a dependency here, and register its extension in
`src/hosts/ai-workspace.tsx`.

## Scopes

The console's Port (`CloudHostPort`) carries `permissions.canAny(scopes)`,
answered by its own permission check (`usePermissions().hasScope`). A feature
passes it an operation's accepted-scope list — copied from the cloud plugin's
OpenAPI `security` block into the package's `permissions.ts`, `:manage` scopes
included, and exported as `*_OPERATIONS` — and disables the control, or shows a
no-permission notice in place of the page, when none is held. The console
registry declares `requiredScopes` on each nav entry from the same lists, so an
entry whose page the user may not read is left out of the sidebar; a `page.*`
override's `requiredScopes` gate the built-in item it repositions the same way.
The `permissions` field is optional on every package's Port: a host that
supplies none (AI Workspace today) gates nothing, and every control stays
enabled exactly as before. None of this is a security boundary: platform-api
still decides, and a refused call is still reported.

Two things the gating depends on live outside these packages:

- **The login scope list.** The console's BFF requests the Platform API's own
  scopes by default (`defaultOIDCScopes` in
  `api-control-plane/bff/internal/config/config.go`), which do not include the
  cloud extension's `ap:environment:*`, `ap:pipeline:*` and `ap:deployment:*`.
  A deployment that compiles these plugins in **must** set
  `APIP_ACP_AUTH_OIDC_SCOPE` to the full list (keeping `offline_access`), or no
  token carries those scopes and the Environments, Pipelines and Deploy pages
  are hidden or disabled for everyone. The IdP has to grant them too: a grant
  added to `platform-api/resources/role-to-scope-mapping.yaml` reaches only the
  tokens the Platform API issues itself (file-mode login) or expands (role
  mode); in scope mode an external IdP's token carries whatever that IdP was
  configured to grant.
- **The gateways list scope.** The cloud spec declares `ListManagedGateways`
  and `ListSelfHostedGateways` with `ap:gateway:create`/`:manage` only. The
  gateways package already accepts `ap:gateway:read` for the list as well (see
  its `permissions.ts`), which the spec is to be corrected to match; until it
  is, a read-only user sees the page and the refused list is reported on it.
