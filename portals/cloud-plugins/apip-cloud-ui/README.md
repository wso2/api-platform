# APIP Cloud UI

Shared cloud-plugin registration for API Platform portals. `src/plugin.ts` is
host-agnostic: `defineCloudPlugin` declares a plugin and its versions, and
`getCloudExtensions` selects one version per plugin ID and flattens that
release's extensions. Each module under `src/hosts` registers features against a
portal's native extension contract.

## Layout

- `src/hosts/api-control-plane.tsx` is the API Control Plane (console) registry.
  It registers the trial-status header action, the settings header icon and
  the entitlements gate, the
  Gateways page override (`page.gateways`), the API and GraphQL API Deploy page
  overrides, Observability (an organization-level Logs/Metrics disclosure, a
  project-level notice, and the per-API Logs page), Portals (managed API
  portals) and Insights.
- `src/hosts/ai-workspace.tsx` is the AI Workspace registry. It registers the
  trial-status header action, Environments, Pipelines, AI Gateways (replacing
  what renders at the built-in `gateways` route), Insights, and the Deploy
  panels for MCP, LLM proxy, LLM provider and agent proxy.
- `src/runtimeFlags.ts` drops the Insights entries unless the `cloudProxyEnabled`
  runtime flag is set.
- The feature packages are siblings under `portals/cloud-plugins`, each a
  `file:` dependency of this package: `apip-cloud-ui-deploy`,
  `apip-cloud-ui-entitlements`, `apip-cloud-ui-environments-new`,
  `apip-cloud-ui-gateways`, `apip-cloud-ui-insights`,
  `apip-cloud-ui-managed-portals`, `apip-cloud-ui-marketing-consent`,
  `apip-cloud-ui-pipelines`,
  `apip-cloud-ui-settings` and `apip-cloud-ui-trial-status`.

Each portal has an injection seam at `src/cloud/index.ts` that exports an empty
`cloudExtensions` and an undefined `cloudBrandLogo`; its `main.tsx` imports
that seam before the first render. The open-source build ships the seam as is.
The cloud build copies this package into the cloned portal tree and overlays
the seam with the matching host module, so the portal renders what that module
exports. Both the web app and the plugins therefore build from one api-platform
revision. `src/index.ts` also re-exports each host's registry under a prefixed
name:

```ts
import {
  aiWorkspaceCloudExtensions,
  apiControlPlaneCloudExtensions,
} from '@wso2-enterprise/apip-cloud-ui';
```

To add a feature, create a sibling package in `portals/cloud-plugins`, add it as
a dependency here, and register its extension in the host module(s) it belongs
to. If it calls a scoped operation, give it a `permissions.ts` as described
below.

The feature packages have no `node_modules` of their own: a package's
`tsconfig.json` resolves against the AI Workspace's and its
`tsconfig.console.json` against the console's. `tsconfig.json` excludes the
package's tests, so `tsconfig.console.json` is the configuration that typechecks
and runs them. The `typecheck`/`build` scripts of `deploy`, `environments-new`,
`gateways`, `insights`, `logs`, `managed-portals` and `pipelines` run `tsc` with
both; `entitlements` has no console configuration and `trial-status` runs only
`tsconfig.json`.

## Scopes

Scope gating runs in the console only. Its Port (`CloudHostPort`, in
`portals/api-control-plane/src/hostPort.tsx`) carries
`permissions.canAny(scopes)`, answered by the console's own permission check
(`usePermissions().hasScope`), so a cloud control disables under the same
conditions a built-in one does: allowed while the session loads, and in
permissive mode when the token carries no scope claim. The AI Workspace host
supplies no `permissions`, and the field is optional on every feature package's
Port: `permissionsOf(port)` (exported by each package's `permissions.ts`)
returns an allow-all check when it is absent, so a host without a permission
layer leaves every control enabled exactly as before.

Each feature package that calls a scoped operation keeps `src/permissions.ts`
with:

- `*_OPERATIONS` tables, one entry per operation the feature calls, holding the
  operation's accepted-scope list exactly as its `security` block declares it,
  `:manage` scopes included. They are copied rather than generated: the "API
  Platform Cloud Extension API" spec lives with the cloud plugin, outside this
  repo's codegen, so the table is the one place a feature names a scope. The
  whole list is passed to `canAny`, which mirrors how platform-api's
  `ScopeEnforcer` admits a call (one held scope matching one accepted entry), so
  no feature reasons about scope inheritance itself. The tables are
  `ENVIRONMENT_OPERATIONS`, `GATEWAY_OPERATIONS`, `PIPELINE_OPERATIONS`,
  `PORTAL_OPERATIONS`, and in the deploy package
  `API_DEPLOYMENT_OPERATIONS`, `PROVIDER_DEPLOYMENT_OPERATIONS` and
  `PROVIDER_BUILD_OPERATIONS`. The deploy package also gates on two Platform API
  scopes from `platform-api/resources/openapi.yaml`: `ARTIFACT_READ_OPERATIONS`
  (the artifact's own record, read for its name and, for a REST API, whether a
  gateway owns it) and `SECRET_OPERATIONS` (the credential field in the provider
  deploy drawer, since a typed key is stored via `POST /secrets` before the
  deploy is sent).
- `NO_PERMISSION_TOOLTIP`, the one tooltip shown on a control disabled for lack
  of scope. It deliberately does not name the missing scope.
- `permissionsOf(port)`, described above, and the `PermissionsPort` type
  mirrored from the console's.

A feature uses the answer to disable a control with that tooltip, to show a
no-permission notice in place of a page it may not read, and to skip a call it
knows would be refused (a list, a poll, an environment catalogue). A
confirmation dialog or drawer re-checks the scope when confirmed and closes if
the scope was lost while it was open. Where a page is readable but
`/environments` is not, the pipeline pages fall back to the environment names
the pipelines already carry, and gateway creation is offered only when the
environment list can be read, since the form cannot be submitted without one.

The console registry declares `requiredScopes` on each nav entry whose page
reads a scoped operation, from the same tables: the Gateways page override, the
Observability entries and Portals. `useNavigationItems` leaves out an entry the
user may not read, hands a claimed built-in item back when its claimant is
hidden this way, and applies a `page.*` override's `requiredScopes` to the
built-in item it repositions (while the override is registered, its page is
what the route renders for everyone). The Deploy overrides and Insights carry
none: the built-in Deploy item they replace keeps the console's own capability
gate (`canDeploy`) and the page gates each action itself, and Insights, a Moesif
embed whose only Platform API calls resolve the project it embeds for, is gated
on the `cloudProxyEnabled` runtime flag instead.

None of this is a security boundary. platform-api decides whether a call
happens, and a refused call is still reported on the page.

Three things the gating depends on live outside these packages:

- **The scopes the token carries.** The console gates on the scopes its
  session reports, and the cloud extension's `ap:environment:*`,
  `ap:pipeline:*` and `ap:deployment:*` are not part of the Platform API's own
  list. What has to request them depends on the console's auth mode
  (`portals/api-control-plane/README.md`):
  - `external` (no BFF; the API Platform gateway's `oauth2-session` policies,
    as in WSO2 Cloud, or another proxy in front): the component in front
    requests the scopes from the IdP and reports the granted set through its
    session endpoint (the scope claim named by `authClaimMappings.scope` in
    `claims` format, the `scope` field in `sdk` format). If it reports no
    scopes at all, the console offers every control and the Platform API
    refuses what the user may not do, unless the `ui-permissions` feature flag
    is set, in which case it offers nothing.
  - `basic` / `oidc` (the BFF): the BFF requests the Platform API's own scopes
    by default (`defaultOIDCScopes` in
    `portals/api-control-plane/bff/internal/config/config.go`). A deployment
    that compiles these plugins in **must** set `APIP_ACP_AUTH_OIDC_SCOPE` to
    the full list (keeping `offline_access`), or no token carries the cloud
    scopes and the pages that need them are hidden or disabled for everyone.
- **The grant.** `platform-api/resources/role-to-scope-mapping.yaml` is read
  for file-mode users, whose roles are expanded into the scope claim of the
  token the login endpoint issues, and in role authorization mode
  (`auth.authorization.mode = "role"`), where a token's roles claim is expanded
  on every request. In the default scope mode (`"scope"`), and always in the
  console's `external` auth mode, a token's scope claim is used as the IdP
  issued it, so the IdP has to grant the cloud scopes itself.
- **The gateway read scope.** The cloud spec declares `ListManagedGateways`,
  `GetManagedGateway`, `ListSelfHostedGateways` and `GetSelfHostedGateway` with
  `ap:gateway:create`/`:manage` only, while every other gateway read on the
  platform accepts `ap:gateway:read`. `GATEWAY_OPERATIONS.ListManagedGateways`
  lists `ap:gateway:read` already, which the cloud spec is to be corrected to
  match; until it is, a read-only user sees the Gateways page and the refused
  list is reported on it.

One known limitation follows from the deploy package's artifact read: a user who
holds the deployment scopes but none of the artifact's own read scopes keeps a
REST API's Deploy and Promote actions locked, because only the API's record says
whether a gateway owns it and the feature leaves that unverified rather than
assumed. Lifting it needs an ownership source on the deployment routes.
