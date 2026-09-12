/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { BarChart3, Layers, Workflow } from '@wso2/oxygen-ui-icons-react';

import { DeployFeature } from '@wso2-enterprise/apip-cloud-ui-deploy';
import { EnvironmentsFeature } from '@wso2-enterprise/apip-cloud-ui-environments-new';
import { GatewaysFeature } from '@wso2-enterprise/apip-cloud-ui-gateways';
import { InsightsFeature } from '@wso2-enterprise/apip-cloud-ui-insights';
import {
  PipelinesFeature,
  ProjectPipelinesFeature,
} from '@wso2-enterprise/apip-cloud-ui-pipelines';
import {
  PAGE_API_DEPLOY_SLOT,
  PAGE_GATEWAYS_SLOT,
  type ApiControlPlaneExtension,
} from '../../../../api-control-plane/src/extensions';
import { routes } from '../../../../api-control-plane/src/routes/paths';
import { ScopeGate } from '../../../../api-control-plane/src/scope/ScopeGate';
import { defineCloudPlugin, getCloudExtensions, type CloudPluginFeature } from '../plugin';
import { filterExtensionsForRuntime } from '../runtimeFlags';

/**
 * Cloud features registered for the api-control-plane host. All live in this
 * same repo now (this package is copied into the cloned portal tree at
 * Docker-build time — see `services/apip-api-control-plane/Dockerfile` in
 * apim-saas — never patched into it), so `ApiControlPlaneExtension` is
 * imported directly from core rather than hand-mirrored.
 *
 * `environments` is an organization-level sidebar item (environments are
 * org-scoped resources), sharing the same `apip-cloud-ui-environments-new`
 * package the ai-workspace host uses (`hosts/ai-workspace.tsx`). Its data flows
 * through the host port's `apiFetch` to platform-api's `/environments` endpoints.
 *
 * `gateways` overrides the built-in Gateways page via the `page.gateways` slot
 * (consumed by `AppRoutes`' `gateways/*` route wrapper) rather than adding a
 * sidebar item — the native nav entry and route stay in place; only what renders
 * there changes. Data flows through `apiFetch` to `/managed-gateways`. This
 * console manages API and event gateways; AI gateways belong to the AI
 * workspace host, which registers the same feature with only that type.
 *
 * `pipelines` ships one plugin with two extensions from the same feature
 * package: an organization-level list/create/edit view, and a project-level
 * view that binds the project to a single pipeline. Both present as one
 * "Pipelines" nav item that adapts to scope — the sidebar shows items at every
 * scope, so each is gated by `isVisible` on whether a project is in scope, and
 * exactly one is shown at a time. Data flows through the host port's `apiFetch`
 * to the platform-api REST endpoints.
 *
 * `deploy` overrides the built-in API Deploy page via the `page.apiDeploy` slot,
 * the same way `gateways` does: the nav entry and route stay native, and only
 * what renders there changes. It is the one API-scoped feature here, so it needs
 * the API in scope, which the Port carries as `apiHandle`. Because the override
 * replaces the whole page, it also replaces the `ScopeGate` the built-in page
 * wraps itself in — so it is re-applied here. Without it, reaching Deploy from an
 * organization- or project-level page (which the sidebar allows, and is a normal
 * thing to do) left a dead end instead of the project/API picker that navigates
 * to the scoped URL.
 *
 * `insights` registers org/project sidebar Moesif embeds and hides the
 * built-in Insights parent outside API scope when loaded. Gated on
 * `cloudProxyEnabled` via `filterExtensionsForRuntime`.
 */
export const cloudPluginFeatures: CloudPluginFeature<ApiControlPlaneExtension>[] = [
  defineCloudPlugin({
    id: 'environments',
    version: '0.1.0',
    extensions: [
      {
        id: 'environments',
        slot: 'sidebar.organization',
        order: 40,
        routePath: 'environments',
        render: (port) => <EnvironmentsFeature port={port} />,
        label: 'Environments',
        icon: <Layers size={20} />,
        level: 'organization',
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
        // `group` must be set explicitly, and to the same cluster Environments
        // and Pipelines fall into. Those are sidebar extensions that declare no
        // group, so they share the unnamed divider cluster (`item.group ?? ''`
        // in `useNavigationClusters`), while the built-in Gateways item lives in
        // the "place" cluster higher up. Omitting it here would keep that
        // built-in cluster and leave Gateways above Environments.
        group: '',
        order: 45,
        routePath: 'gateways',
        render: (port) => <GatewaysFeature gatewayTypes={['regular', 'event']} port={port} />,
        label: 'Gateways',
        level: 'organization',
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
        render: (port) => (
          <ScopeGate
            prompt="Deployments are made for a single API."
            requires="api"
            to={routes.apiDeploy}
          >
            <DeployFeature port={port} />
          </ScopeGate>
        ),
        label: 'Deploy',
        level: 'api',
      },
    ],
  }),
  defineCloudPlugin({
    id: 'pipelines',
    version: '0.1.0',
    extensions: [
      {
        id: 'pipelines',
        slot: 'sidebar.organization',
        order: 50,
        routePath: 'pipelines',
        render: (port) => <PipelinesFeature port={port} />,
        label: 'Pipelines',
        icon: <Workflow size={20} />,
        level: 'organization',
        // Only when no project is in scope — otherwise the project view below
        // takes the same "Pipelines" slot, so exactly one is ever shown.
        isVisible: (scope) => !scope.isProjectScope,
      },
      {
        id: 'project-pipeline',
        slot: 'sidebar.project',
        order: 50,
        routePath: 'pipelines',
        render: (port) => <ProjectPipelinesFeature port={port} />,
        label: 'Pipelines',
        icon: <Workflow size={20} />,
        level: 'project',
        isVisible: (scope) => scope.isProjectScope,
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
        order: 60,
        routePath: 'insights',
        label: 'Insights',
        group: 'api',
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
            !typed.isApiScope
          );
        },
        render: (port) => (
          <InsightsFeature
            port={port}
            forcedScopeLevel="organization"
            embedProfile="api-control-plane"
          />
        ),
      },
      {
        id: 'project-insights',
        slot: 'sidebar.project',
        order: 60,
        routePath: 'insights',
        label: 'Insights',
        group: 'api',
        level: 'project',
        icon: <BarChart3 size={20} />,
        isVisible: (scope) => {
          const typed = scope as {
            isProjectScope?: boolean;
            isApiScope?: boolean;
          };
          return Boolean(typed.isProjectScope) && !typed.isApiScope;
        },
        render: (port) => (
          <InsightsFeature
            port={port}
            forcedScopeLevel="project"
            embedProfile="api-control-plane"
          />
        ),
      },
    ],
  }),
];

export const cloudExtensions = filterExtensionsForRuntime(
  getCloudExtensions(cloudPluginFeatures)
);
export type { ApiControlPlaneExtension };
