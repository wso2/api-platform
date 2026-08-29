/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { ReactNode } from 'react';

import { getCloudExtensions, type CloudPluginFeature } from '../plugin';

/**
 * Mirrors `ApiControlPlaneExtension` from api-platform's
 * `portals/api-control-plane/src/extensions.tsx`. Duplicated, not imported —
 * api-platform and apim-saas are separate git repos; this file is only ever
 * glued to that source at Docker-build time (this package's `src/` is copied
 * into the cloned portal tree as `src/cloud`, see
 * `services/apip-api-control-plane/Dockerfile`), never at typecheck time
 * here. Keep this in sync by hand if that type's shape changes upstream.
 */
export type ApiControlPlaneExtension = {
  id: string;
  routePath: string;
  element: ReactNode;
  label: string;
  icon?: ReactNode;
  level: 'organization' | 'project' | 'api';
  group?: string;
  order: number;
  isVisible?: (scope: unknown) => boolean;
};

/**
 * No cloud feature has been built for the console yet — this starts empty on
 * purpose. Add a `defineCloudPlugin<ApiControlPlaneExtension>({...})` entry
 * here per feature once one exists; `services/apip-api-control-plane/VERSION`
 * gets a matching `CLOUD_UI_<FEATURE>_VERSION` key at that point.
 */
export const cloudPluginFeatures: CloudPluginFeature<ApiControlPlaneExtension>[] =
  [];

export const cloudExtensions = getCloudExtensions(cloudPluginFeatures);
