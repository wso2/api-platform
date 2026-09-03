/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { Layers, Workflow } from '@wso2/oxygen-ui-icons-react';

import { DeployFeature } from '@wso2-enterprise/apip-cloud-ui-deploy';
import { EnvironmentsFeature } from '@wso2-enterprise/apip-cloud-ui-environments-new';
import {
  PipelinesFeature,
  ProjectPipelinesFeature,
} from '@wso2-enterprise/apip-cloud-ui-pipelines';
import {
  API_DEPLOY_SLOT,
  settingsTabSlot,
  type ApiControlPlaneCloudEntry,
  type ApiControlPlaneExtension,
} from '../../../../api-control-plane/src/extensions';
import { defineCloudPlugin, getCloudExtensions, type CloudPluginFeature } from '../plugin';

/**
 * Cloud features registered for the api-control-plane host. All live in this
 * same repo now (this package is copied into the cloned portal tree at
 * Docker-build time — see `services/apip-api-control-plane/Dockerfile` in
 * apim-saas — never patched into it), so `ApiControlPlaneExtension` is
 * imported directly from core rather than hand-mirrored.
 *
 * `environments` is nested under Settings via the `settings.project.tabs`
 * slot. It shares the same `apip-cloud-ui-environments-new` package the
 * ai-workspace host uses (`hosts/ai-workspace.tsx`) rather than a
 * control-plane-specific package, but ships in the same branch/release as the
 * rest of this repo — no independent version tag, no separate fetch stage.
 * Backed by an in-memory mock port (`createMockEnvironmentPort`, constructed
 * internally by `EnvironmentsFeature`) until a real backend exists — swap
 * that in without touching `EnvironmentsList`/`EnvironmentForm`, which only
 * ever see the `EnvironmentPort` interface.
 *
 * `pipelines` ships one plugin with two extensions from the same feature
 * package: an organization-level list/create/edit view, and a project-level
 * view that binds the project to a single pipeline. Both present as one
 * "Pipelines" nav item that adapts to scope — the sidebar shows items at every
 * scope, so each is gated by `isVisible` on whether a project is in scope, and
 * exactly one is shown at a time. Data flows through the host port's `apiFetch`
 * to the platform-api REST endpoints.
 *
 * `deploy` is different again: it registers against `API_DEPLOY_SLOT` to replace
 * what renders at the host's existing, built-in API Deploy route — see
 * `DeployRoute` in `api-control-plane/src/routes/AppRoutes.tsx` — rather than
 * adding a new nav item. The built-in page keeps its route, sidebar entry and
 * capability gate; nothing under
 * `api-control-plane/src/pages/appShell/appShellPages/deploy` is touched.
 */
export const cloudPluginFeatures: CloudPluginFeature<ApiControlPlaneCloudEntry>[] = [
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
    id: 'deploy',
    version: '0.1.0',
    extensions: [
      {
        id: 'deploy',
        slot: API_DEPLOY_SLOT,
        order: 0,
        render: (port) => <DeployFeature port={port} />,
      },
    ],
  }),
];

export const cloudExtensions = getCloudExtensions(cloudPluginFeatures);
export type { ApiControlPlaneCloudEntry, ApiControlPlaneExtension };
