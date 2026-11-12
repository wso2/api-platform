/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { PageContent } from '@wso2/oxygen-ui';
import { BarChart3, PanelsTopLeft } from '@wso2/oxygen-ui-icons-react';

import { DeployFeature } from '@wso2-enterprise/apip-cloud-ui-deploy';
import { GATEWAY_OPERATIONS, GatewaysFeature } from '@wso2-enterprise/apip-cloud-ui-gateways';
import type { GatewayType } from '@wso2-enterprise/apip-cloud-ui-gateways';
import { InsightsFeature } from '@wso2-enterprise/apip-cloud-ui-insights';
import {
  ManagedPortalsPage,
  PORTAL_OPERATIONS,
} from '@wso2-enterprise/apip-cloud-ui-managed-portals';
import { MarketingConsentGate } from '@wso2-enterprise/apip-cloud-ui-marketing-consent';
import { AppSwitcherButton } from '@wso2-enterprise/apip-cloud-ui-app-switcher';
import { ManageProjectButton, SettingsButton } from '@wso2-enterprise/apip-cloud-ui-settings';
import { nameFromPort } from '@wso2-enterprise/apip-cloud-ui-user-profile';
import {
  EntitlementsGate,
  TrialStatusFeature,
} from '@wso2-enterprise/apip-cloud-ui-entitlements';
import type { BrandLogo } from '../../../../api-control-plane/src/branding/BrandLogoProvider';
import { AppPage } from '../../../../api-control-plane/src/components/AppPage';
import {
  APP_GATE_SLOT,
  HEADER_ACTIONS_SLOT,
  HEADER_START_SLOT,
  HOME_GREETING_SLOT,
  PAGE_API_DEPLOY_SLOT,
  PAGE_GRAPHQL_API_DEPLOY_SLOT,
  PAGE_GATEWAYS_SLOT,
  PROJECT_HEADER_ACTIONS_SLOT,
  type ApiControlPlaneCloudEntry,
  type ApiControlPlaneExtension,
} from '../../../../api-control-plane/src/extensions';
import type { CloudHostPort } from '../../../../api-control-plane/src/hostPort';
import { isFeatureDisabled } from '../../../../api-control-plane/src/config/features';
import { runtimeConfig } from '../../../../api-control-plane/src/config/runtime';
import { routes } from '../../../../api-control-plane/src/routes/paths';
import { useConsoleScope } from '../../../../api-control-plane/src/scope/ConsoleScopeContext';
import { ScopeGate } from '../../../../api-control-plane/src/scope/ScopeGate';
import { defineCloudPlugin, getCloudExtensions, type CloudPluginFeature } from '../plugin';
import { filterExtensionsForRuntime } from '../runtimeFlags';
import cloudLogoDark from '../assets/logos/apiplatform_white.svg';
import cloudLogoLight from '../assets/logos/apiplatform_black.svg';

/**
 * Scopes the cloud Deploy page deploys under (apip-cloud-ui-deploy
 * API_DEPLOYMENT_OPERATIONS.DeployApi): one of them is needed to deploy.
 */
const CLOUD_DEPLOY_SCOPES = [
  'ap:deployment:deploy',
  'ap:deployment:manage',
  'ap:rest_api:deployment:create',
  'ap:rest_api:deployment:manage',
  'ap:rest_api:manage',
  'ap:graphql_api:deployment:create',
  'ap:graphql_api:deployment:manage',
  'ap:graphql_api:manage',
] as const;

/**
 * Scopes that change a project (the accepted lists of UpdateProject and
 * DeleteProject): one of them is needed for the Manage project button, as the
 * WSO2 Cloud console shows its own Manage button only to someone who may update
 * or delete the project.
 */
const MANAGE_PROJECT_SCOPES = [
  'ap:project:update',
  'ap:project:delete',
  'ap:project:manage',
] as const;

/** Cloud console branding used by the host header and login page. */
export const cloudBrandLogo: BrandLogo = {
  dark: cloudLogoDark,
  light: cloudLogoLight,
};

/**
 * The kinds of gateway this host manages. Module scope, not a literal at the
 * use site: the gateways feature asks the server for exactly these kinds, and a
 * fresh array on every render would make that request repeat.
 */
const API_GATEWAY_TYPES: GatewayType[] = ['regular', 'event'];

/**
 * `GatewaysFeature`'s port requires `gatewayVersions`, which this host's own
 * `CloudHostPort` (core, `api-control-plane/src/hostPort.tsx`) does not carry.
 * API Control Plane has no hardcoded catalog the way AI Workspace does — its
 * release versions come from the BFF's own `config.toml` (`[[gateway_versions]]`,
 * see `bff/internal/config/config.go`), surfaced through this SPA's
 * `runtimeConfig.gatewayVersions` (`src/config/runtime.ts`), which already
 * falls back to `[]` for a deployment whose config predates this field.
 */
const API_GATEWAY_VERSIONS = runtimeConfig.gatewayVersions;

/**
 * The API Deploy override. The built-in Deploy page sits inside `AppPage`, which
 * supplies the page padding and breadcrumbs, so the scope picker shown before an
 * API is in scope gets that same container here; without it the picker card
 * renders flush against the shell. Once the API is in scope `DeployFeature`
 * renders bare, since it brings its own `PageContent` and would be padded twice.
 */
function CloudDeployPage({ port }: { port: CloudHostPort }) {
  const { isApiScope } = useConsoleScope();
  const gate = (
    <ScopeGate
      prompt="Deployments are made for a single API."
      requires="api"
      to={routes.apiDeploy}
    >
      <DeployFeature port={port} />
    </ScopeGate>
  );
  return isApiScope ? gate : <AppPage>{gate}</AppPage>;
}

/**
 * Cloud features registered for the api-control-plane host. All live in this
 * same repo now (this package is copied into the cloned portal tree at
 * Docker-build time — see `services/apip-api-control-plane/Dockerfile` in
 * apim-saas — never patched into it), so `ApiControlPlaneExtension` is
 * imported directly from core rather than hand-mirrored.
 *
 * `gateways` overrides the built-in Gateways page via the `page.gateways` slot
 * (consumed by `AppRoutes`' `gateways/*` route wrapper) rather than adding a
 * sidebar item — the native nav entry and route stay in place; only what renders
 * there changes. Data flows through `apiFetch` to `/managed-gateways`. This
 * console manages API and event gateways; AI gateways belong to the AI
 * workspace host, which registers the same feature with only that type.
 *
 * `deploy` overrides the built-in API Deploy page via the `page.apiDeploy` slot,
 * and the GraphQL API Deploy page via `page.graphqlApiDeploy` with the same feature,
 * the same way `gateways` does: the nav entry and route stay native, and only
 * what renders there changes. It is the one API-scoped feature here, so it needs
 * the API in scope, which the Port carries as `apiHandle`. Because the override
 * replaces the whole page, it also replaces the `ScopeGate` the built-in page
 * wraps itself in - so it is re-applied here. Without it, reaching Deploy from an
 * organization- or project-level page (which the sidebar allows, and is a normal
 * thing to do) left a dead end instead of the project/API picker that navigates
 * to the scoped URL.
 *
 * `managed-api-portals` is an organization-level sidebar item, one WSO2-managed
 * developer portal per entry in the org. Talks to apip-platform-api's cloud-only
 * `/managed-api-portals` resource via the host port; SaaS-only, distinct from
 * the OSS `/api-portals` registry (SaaS lifecycle vs plain registry).
 *
 * `insights` registers org/project sidebar Moesif embeds. Both claim the
 * built-in Insights item, so it gives way wherever one of them is visible (org
 * and project scope) and comes back inside an API. Gated on
 * `cloudProxyEnabled` via `filterExtensionsForRuntime`.
 */

export const cloudPluginFeatures: CloudPluginFeature<ApiControlPlaneCloudEntry>[] = [
  defineCloudPlugin({
    // Entitlements is console-only. The shared `apip-cloud-ui-trial-status`
    // package still serves AI Workspace unchanged; this console renders the
    // newer badge from its own package instead, so neither portal constrains
    // the other. Keeps the slot id `trial-status` so nothing downstream that
    // selects on it has to change.
    id: 'trial-status',
    version: '0.1.0',
    extensions: [
      {
        id: 'trial-status',
        slot: HEADER_ACTIONS_SLOT,
        order: 10,
        render: (port) => <TrialStatusFeature port={port} />,
      },
      {
        // Headless: supplies the organization's creation limits and read-only
        // verdict through the Port. Mounted on every in-shell route, and the
        // only thing that ever calls `resourceLimits.set` — with no cloud
        // plugin registered the host supplies nothing and stays unrestricted.
        id: 'entitlements-gate',
        slot: APP_GATE_SLOT,
        order: 10,
        render: (port) => <EntitlementsGate port={port} />,
      },
    ],
  }),
  defineCloudPlugin({
    // The WSO2 Cloud platform switcher (Oxygen AppSwitcher), fed by the
    // deployment's `cloudShell` / `cloudConsoleUrl` runtime config. Console-only.
    // Takes over the header's sidebar-collapse toggle spot at the far left —
    // the sidebar's own `Sidebar.CollapseToggle` still collapses it.
    id: 'app-switcher',
    version: '0.1.0',
    extensions: [
      {
        id: 'app-switcher',
        slot: HEADER_START_SLOT,
        order: 0,
        render: () => (
          <AppSwitcherButton
            cloudShell={runtimeConfig.cloudShell}
            cloudConsoleUrl={runtimeConfig.cloudConsoleUrl}
          />
        ),
      },
    ],
  }),
  defineCloudPlugin({
    // Asks a signed-in person once whether they want WSO2 marketing email, and
    // records the answer with Pardot through platform-api's internal consent
    // endpoint (the BFF's `marketing-consent` upstream, carrying the person's own
    // token). Headless `app.gate` entry: renders nothing unless they never answered.
    // Console-only; the AI Workspace host registers nothing.
    id: 'marketing-consent',
    version: '0.1.0',
    extensions: [
      {
        id: 'marketing-consent-gate',
        slot: APP_GATE_SLOT,
        order: 20,
        render: (port) => (
          <MarketingConsentGate port={port} proxyBase={runtimeConfig.platformApiBaseUrl} />
        ),
      },
    ],
  }),
  defineCloudPlugin({
    // The header's settings icon, which opens the WSO2 Cloud console's organization
    // pages. Console-only: it is registered in this host and not in the AI Workspace
    // one, so that portal's header is unchanged. Ordered after the trial badge, and
    // the header lays its action slot out ahead of the theme switcher, so the icon
    // sits between them.
    //
    // The project overview's Manage project button, which opens the WSO2 Cloud
    // console's manage page for the project, where it is edited and deleted. Shown
    // with a project in scope and to someone who may change it (MANAGE_PROJECT_SCOPES).
    id: 'settings',
    version: '0.1.0',
    extensions: [
      {
        id: 'settings-button',
        slot: HEADER_ACTIONS_SLOT,
        order: 20,
        render: () => <SettingsButton />,
      },
      {
        id: 'manage-project-button',
        slot: PROJECT_HEADER_ACTIONS_SLOT,
        order: 10,
        render: (port) =>
          port.projectHandle && port.permissions.canAny(MANAGE_PROJECT_SCOPES) ? (
            <ManageProjectButton
              consoleUrl={runtimeConfig.cloudConsoleUrl}
              projectHandle={port.projectHandle}
            />
          ) : null,
      },
    ],
  }),
  defineCloudPlugin({
    // Greets the signed-in user by their own name on the Organization home page,
    // read from the session the host already holds (no request of its own). With no
    // name the page keeps greeting by the organization's name.
    id: 'user-profile',
    version: '0.1.0',
    extensions: [
      {
        id: 'home-greeting-name',
        slot: HOME_GREETING_SLOT,
        order: 10,
        resolveName: (port) => nameFromPort(port),
      },
    ],
  }),
  defineCloudPlugin({
    id: 'gateways',
    version: '0.1.0',
    extensions: [
      {
        id: 'gateways',
        slot: PAGE_GATEWAYS_SLOT,
        // Cloud-only nav placement for the built-in Gateways item (matched by
        // `id`): between Environments (40) and Pipelines (50). The open-source
        // console registers no override, so its Gateways item keeps its own
        // built-in placement. `routePath`/`level` are inert (the page override
        // is consumed by the `gateways/*` route wrapper in `AppRoutes`, not the
        // nav/Settings-tab pipeline) but required by the shared extension type.
        //
        // `group` must be set explicitly, and to the same cluster Portals falls
        // into. That is a sidebar extension that declares no group, so it
        // shares the unnamed divider cluster (`item.group ?? ''` in
        // `useNavigationClusters`), while the built-in Gateways item lives in
        // the "place" cluster higher up. Omitting it here would keep that
        // built-in cluster and leave Gateways above Portals.
        //
        // Order 110 puts this cluster last, after the Insights / Observability /
        // Manage cluster (`operate`, up to order 100).
        group: '',
        order: 110,
        routePath: 'gateways',
        render: (port) => (
          <GatewaysFeature
            gatewayTypes={API_GATEWAY_TYPES}
            // Map the environment-specific gateway host expected by the plugin.
            port={{
              ...port,
              gatewayVersions: API_GATEWAY_VERSIONS,
              controlPlaneHost: port.gatewayControlPlaneHost,
            }}
          />
        ),
        label: 'Gateways',
        level: 'organization',
        // The scopes the gateways list accepts (see `GATEWAY_OPERATIONS` for
        // why `ap:gateway:read` is among them). Gates the nav placement this
        // override carries — `useNavigationItems` applies it to the built-in
        // Gateways item it repositions — and the page gates itself the same way.
        requiredScopes: GATEWAY_OPERATIONS.ListManagedGateways,
      },
    ],
  }),
  defineCloudPlugin({
    id: 'deploy',
    version: '0.1.0',
    extensions: [
      {
        id: 'api-deploy',
        slot: PAGE_API_DEPLOY_SLOT,
        // Inert here, as for the gateways override: the page override is consumed
        // by the `apiDeploy` route wrapper in `AppRoutes`, not by the nav or
        // Settings-tab pipeline, which only match `sidebar.*` / `settings.*.tabs`.
        order: 0,
        routePath: 'deploy',
        render: (port) => <CloudDeployPage port={port} />,
        label: 'Deploy',
        level: 'api',
        // The cloud deploys through the project's pipeline, under the scopes
        // its Deploy page checks (CLOUD_DEPLOY_SCOPES).
        actionScopes: CLOUD_DEPLOY_SCOPES,
      },
      {
        // The same Deploy feature for a GraphQL API, so both kinds deploy through
        // one page. Its route is reached only with the API in its URL, which is
        // where the handle comes from — the Port's `apiHandle` is REST-only — so
        // there is no scope to gate on. `routePath`/`level` are inert, as above.
        id: 'graphql-api-deploy',
        slot: PAGE_GRAPHQL_API_DEPLOY_SLOT,
        order: 0,
        actionScopes: CLOUD_DEPLOY_SCOPES,
        routePath: 'deploy',
        render: (port, artifactHandle) => (
          <DeployFeature
            artifactHandle={artifactHandle}
            kind="GraphQLApi"
            port={port}
          />
        ),
        label: 'Deploy',
        level: 'api',
      },
    ],
  }),
  defineCloudPlugin({
    id: 'managed-api-portals',
    version: '0.1.0',
    extensions: [
      {
        id: 'managed-api-portals',
        slot: 'sidebar.organization',
        // Last in the sidebar, after the Gateways override (110).
        order: 120,
        routePath: 'managed-api-portals',
        render: (port) => <ManagedPortalsPage port={port} />,
        requiredScopes: PORTAL_OPERATIONS.ListManagedApiPortals,
        label: 'Portals',
        icon: <PanelsTopLeft size={20} />,
        level: 'organization',
      },
    ],
  }),
  defineCloudPlugin({
    id: 'insights',
    version: '0.1.0',
    extensions: [
      {
        id: 'organization-insights',
        slot: 'sidebar.organization',
        claims: 'insights',
        // First of the Insights / Observability / Manage cluster.
        order: 70,
        routePath: 'insights',
        label: 'Insights',
        group: 'operate',
        level: 'organization',
        icon: <BarChart3 size={20} />,
        isVisible: (scope) => {
          const typed = scope as {
            isOrganizationScope?: boolean;
            isProjectScope?: boolean;
            isApiScope?: boolean;
          };
          return (
            Boolean(typed.isOrganizationScope) &&
            !typed.isProjectScope &&
            !typed.isApiScope &&
            !isFeatureDisabled('insights')
          );
        },
        // `InsightsFeature` brings its own `PageContent` on AI Workspace (whose
        // shell doesn't wrap the outlet either) but not here, so this slot's
        // route supplies it — ACP's shell adds none, same as the `deploy`
        // override above.
        render: (port) => (
          <PageContent fullWidth>
            <InsightsFeature
              port={port}
              forcedScopeLevel="organization"
              embedProfile="api-control-plane"
            />
          </PageContent>
        ),
      },
      {
        id: 'project-insights',
        slot: 'sidebar.project',
        claims: 'insights',
        order: 60,
        routePath: 'insights',
        label: 'Insights',
        group: 'operate',
        level: 'project',
        icon: <BarChart3 size={20} />,
        isVisible: (scope) => {
          const typed = scope as {
            isProjectScope?: boolean;
            isApiScope?: boolean;
          };
          return (
            Boolean(typed.isProjectScope) && !typed.isApiScope && !isFeatureDisabled('insights')
          );
        },
        render: (port) => (
          <PageContent fullWidth>
            <InsightsFeature
              port={port}
              forcedScopeLevel="project"
              embedProfile="api-control-plane"
            />
          </PageContent>
        ),
      },
    ],
  }),
];

export const cloudExtensions = filterExtensionsForRuntime(
  getCloudExtensions(cloudPluginFeatures)
);
export type { ApiControlPlaneCloudEntry, ApiControlPlaneExtension };
