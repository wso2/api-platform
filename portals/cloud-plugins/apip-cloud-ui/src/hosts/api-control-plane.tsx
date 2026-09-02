/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { Layers } from '@wso2/oxygen-ui-icons-react';

import { EnvironmentsFeature } from '@wso2-enterprise/apip-cloud-ui-environments-new';
import {
  settingsTabSlot,
  type ApiControlPlaneExtension,
} from '../../../../api-control-plane/src/extensions';
import { defineCloudPlugin, getCloudExtensions, type CloudPluginFeature } from '../plugin';

/**
 * Cloud features registered for the api-control-plane host. Both live in this
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
 */
export const cloudPluginFeatures: CloudPluginFeature<ApiControlPlaneExtension>[] = [
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
];

export const cloudExtensions = getCloudExtensions(cloudPluginFeatures);
export type { ApiControlPlaneExtension };
