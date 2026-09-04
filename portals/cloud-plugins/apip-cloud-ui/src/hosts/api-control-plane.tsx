/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { BarChart3, Layers, Workflow } from '@wso2/oxygen-ui-icons-react';

import { EnvironmentsFeature } from '@wso2-enterprise/apip-cloud-ui-environments-new';
import { InsightsFeature } from '@wso2-enterprise/apip-cloud-ui-insights';
import {
  PipelinesFeature,
  ProjectPipelinesFeature,
} from '@wso2-enterprise/apip-cloud-ui-pipelines';
import {
  settingsTabSlot,
  type ApiControlPlaneExtension,
} from '../../../../api-control-plane/src/extensions';
import { defineCloudPlugin, getCloudExtensions, type CloudPluginFeature } from '../plugin';
import { filterExtensionsForRuntime } from '../runtimeFlags';

/**
 * Cloud features registered for the api-control-plane host. All live in this
 * same repo now (this package is copied into the cloned portal tree at
 * Docker-build time — see `services/apip-api-control-plane/Dockerfile` in
 * apim-saas — never patched into it), so `ApiControlPlaneExtension` is
 * imported directly from core rather than hand-mirrored.
 *
 * - `environments` is nested under Settings via the `settings.project.tabs`
 *   slot (`apip-cloud-ui-environments-new`, shared with ai-workspace).
 * - `pipelines` ships org + project sidebar views from the same feature
 *   package; `isVisible` keeps exactly one "Pipelines" item at a time.
 * - `insights` registers org/project sidebar Moesif embeds. Gated on
 *   `cloudProxyEnabled` via `filterExtensionsForRuntime`.
 */
export const cloudPluginFeatures: CloudPluginFeature<ApiControlPlaneExtension>[] =
  [
    defineCloudPlugin({
      id: 'environments',
      version: '0.1.0',
      extensions: [
        {
          id: 'environments',
          routePath: 'settings/environments',
          render: (port) => <EnvironmentsFeature port={port} />,
          label: 'Environments',
          icon: <Layers />,
          level: 'project',
          slot: settingsTabSlot('project'),
          order: 10,
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
