/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { Workflow } from '@wso2/oxygen-ui-icons-react';

import { PipelinesFeature } from '@wso2-enterprise/apip-cloud-ui-pipelines';
import type { AIWorkspaceExtension } from '../../../../ai-workspace/src/extensions';
import { defineCloudPlugin, getCloudExtensions, type CloudPluginFeature } from '../plugin';

/**
 * Cloud features registered for the AI Workspace host. The host owns routing,
 * organization/project scope, navigation and notifications; each plugin only
 * renders against the small host port passed to it.
 */
export const cloudPluginFeatures: CloudPluginFeature<AIWorkspaceExtension>[] = [
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
        render: (port) => <PipelinesFeature port={port} />,
      },
    ],
  }),
];

export const cloudExtensions = getCloudExtensions(cloudPluginFeatures);
export type { AIWorkspaceExtension };
