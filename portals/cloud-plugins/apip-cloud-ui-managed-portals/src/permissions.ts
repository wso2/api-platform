/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import type { PermissionsPort } from './hostPort';

/**
 * The accepted-scope list of each Cloud Extension API operation this feature
 * calls, as the operation's `security` block declares it in the cloud plugin's
 * OpenAPI spec ("API Platform Cloud Extension API", `/managed-api-portals`).
 * Copied rather than generated: that spec lives with the cloud plugin, outside
 * this repo's codegen, so this is the one place the feature names a scope.
 *
 * Each list is passed whole to the host's `permissions.canAny`, which is how
 * platform-api's `ScopeEnforcer` admits a call — one held scope matching one
 * accepted entry — so the broader `:manage` scope is listed beside the action
 * and the feature never reasons about inheritance itself.
 */
export const PORTAL_OPERATIONS = {
  ListManagedApiPortals: ['ap:api_portal:read', 'ap:api_portal:manage'],
  GetManagedApiPortal: ['ap:api_portal:read', 'ap:api_portal:manage'],
  CreateManagedApiPortal: ['ap:api_portal:create', 'ap:api_portal:manage'],
  UpdateManagedApiPortal: ['ap:api_portal:update', 'ap:api_portal:manage'],
  DeleteManagedApiPortal: ['ap:api_portal:delete', 'ap:api_portal:manage'],
} as const satisfies Record<string, readonly string[]>;

/**
 * Tooltip for a control disabled because the signed-in user lacks the scope.
 * Deliberately does not name the missing scope: that would tell a probing
 * caller what to look for, and means nothing to the person reading it.
 */
export const NO_PERMISSION_TOOLTIP = "You don't have permission to perform this action.";


/** The permission check with no host Port supplied: nothing is gated. */
const ALLOW_ALL: PermissionsPort = { canAny: () => true };

/**
 * The host's permission check, or one that gates nothing when the Port carries
 * none. A host without a permission layer leaves every control enabled exactly
 * as before — the posture `resourceLimits` takes when absent — and platform-api
 * still refuses what the user may not do.
 */
export const permissionsOf = (port: { permissions?: PermissionsPort }): PermissionsPort =>
  port.permissions ?? ALLOW_ALL;
