/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

/**
 * The accepted-scope list of each Cloud Extension API operation this feature
 * calls, as the operation's `security` block declares it in the cloud plugin's
 * OpenAPI spec ("API Platform Cloud Extension API": `/pipelines`,
 * `/environments` and `/projects/{projectId}/deployment-pipeline`). Copied
 * rather than generated: that spec lives with the cloud plugin, outside this
 * repo's codegen, so this is the one place the feature names a scope.
 *
 * Each list is passed whole to the host's `permissions.canAny`, which is how
 * platform-api's `ScopeEnforcer` admits a call — one held scope matching one
 * accepted entry — so the broader `:manage` scope is listed beside the action
 * and the feature never reasons about inheritance itself. The project binding
 * is a property of the project, which is why its two operations take project
 * scopes rather than pipeline ones.
 */
export const PIPELINE_OPERATIONS = {
  ListEnvironments: ['ap:environment:read', 'ap:environment:manage'],
  ListPipelines: ['ap:pipeline:read', 'ap:pipeline:manage'],
  CreatePipeline: ['ap:pipeline:create', 'ap:pipeline:manage'],
  UpdatePipeline: ['ap:pipeline:update', 'ap:pipeline:manage'],
  DeletePipeline: ['ap:pipeline:delete', 'ap:pipeline:manage'],
  GetProjectDeploymentPipeline: ['ap:project:read', 'ap:project:manage'],
  SetProjectDeploymentPipeline: ['ap:project:update', 'ap:project:manage'],
} as const satisfies Record<string, readonly string[]>;

/**
 * Tooltip for a control disabled because the signed-in user lacks the scope.
 * Deliberately does not name the missing scope: that would tell a probing
 * caller what to look for, and means nothing to the person reading it.
 */
export const NO_PERMISSION_TOOLTIP = "You don't have permission to perform this action.";
