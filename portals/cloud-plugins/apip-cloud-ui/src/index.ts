/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

export { defineCloudPlugin, getCloudExtensions } from './plugin';
export type { CloudPluginFeature } from './plugin';

export {
  cloudExtensions as apiControlPlaneCloudExtensions,
  cloudPluginFeatures as apiControlPlaneCloudPluginFeatures,
} from './hosts/api-control-plane';
export type { ApiControlPlaneExtension } from './hosts/api-control-plane';

export {
  cloudExtensions as aiWorkspaceCloudExtensions,
  cloudPluginFeatures as aiWorkspaceCloudPluginFeatures,
} from './hosts/ai-workspace';
export type { AIWorkspaceExtension } from './hosts/ai-workspace';
