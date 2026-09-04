/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { Boxes, Workflow } from '@wso2/oxygen-ui-icons-react';

import { EnvironmentsFeature } from '@wso2-enterprise/apip-cloud-ui-environments-new';
import { GatewaysFeature } from '@wso2-enterprise/apip-cloud-ui-gateways';
import { PipelinesFeature, ProjectPipelinesFeature } from '@wso2-enterprise/apip-cloud-ui-pipelines';
import {
  AI_WORKSPACE_GATEWAYS_SLOT,
  type AIWorkspaceCloudEntry,
  type AIWorkspaceExtension,
} from '../../../../ai-workspace/src/extensions';
import { defineCloudPlugin, getCloudExtensions, type CloudPluginFeature } from '../plugin';

/**
 * Cloud features registered for the AI Workspace host. The host owns routing,
 * organization/project scope, navigation and notifications; each plugin only
 * renders against the small host port passed to it.
 *
 * Most entries are `sidebar.main` items (new nav entry + route). `gateways`
 * is different: it registers against `AI_WORKSPACE_GATEWAYS_SLOT` to replace
 * what renders at the host's existing, built-in `gateways` route/sidebar item
 * — see `GatewaysRoute` in `ai-workspace/src/App.tsx` — rather than adding a
 * new one. Nothing under `ai-workspace/src/pages/appShell/appShellPages/gateways`
 * is touched by this.
 *
 * `deploy` is NOT registered here. It deploys one API through its project's
 * pipeline, so it needs an API in scope; this host has no API-scoped placement
 * and its Port carries no `apiHandle`, so the page could only ever tell the user
 * to open an API. The feature package is shared and unchanged — registering it
 * here is a matter of adding that placement, not of changing the feature.
 */
export const cloudPluginFeatures: CloudPluginFeature<AIWorkspaceCloudEntry>[] = [
  defineCloudPlugin({
    id: 'environments',
    version: '0.1.0',
    extensions: [
      {
        id: 'environments',
        slot: 'sidebar.main',
        order: 50,
        path: 'environments',
        label: 'Environments',
        icon: <Boxes size={20} />,
        render: (port) => <EnvironmentsFeature port={port} />,
      },
    ],
  }),
  defineCloudPlugin({
    id: 'pipelines',
    version: '0.1.0',
    extensions: [
      {
        id: 'pipelines',
        slot: 'sidebar.main',
        order: 60,
        path: 'pipelines',
        label: 'Pipelines',
        icon: <Workflow size={20} />,
        // One scope-adaptive "Pipelines" item: the project binding view when a
        // project is selected, the organization list/create/edit view otherwise.
        render: (port) =>
          port.projectHandle ? (
            <ProjectPipelinesFeature port={port} />
          ) : (
            <PipelinesFeature port={port} />
          ),
      },
    ],
  }),
  defineCloudPlugin({
    id: 'gateways',
    version: '0.1.0',
    extensions: [
      {
        id: 'gateways',
        slot: AI_WORKSPACE_GATEWAYS_SLOT,
        order: 0,
        render: (port) => <GatewaysFeature port={port} />,
      },
    ],
  }),
];

export const cloudExtensions = getCloudExtensions(cloudPluginFeatures);
export type { AIWorkspaceExtension };
