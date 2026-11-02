/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * The accepted-scope list of each Cloud Extension API operation this feature
 * calls, as the operation's `security` block declares it in the cloud plugin's
 * OpenAPI spec ("API Platform Cloud Extension API", `/environments`). Copied
 * rather than generated: that spec lives with the cloud plugin, outside this
 * repo's codegen, so this is the one place the feature names a scope.
 *
 * Each list is passed whole to the host's `permissions.canAny`, which is how
 * platform-api's `ScopeEnforcer` admits a call — one held scope matching one
 * accepted entry — so the broader `:manage` scope is listed beside the action
 * and the feature never reasons about inheritance itself.
 */
export const ENVIRONMENT_OPERATIONS = {
  ListEnvironments: ['ap:environment:read', 'ap:environment:manage'],
  CreateEnvironment: ['ap:environment:create', 'ap:environment:manage'],
  DeleteEnvironment: ['ap:environment:delete', 'ap:environment:manage'],
} as const satisfies Record<string, readonly string[]>;

/**
 * Tooltip for a control disabled because the signed-in user lacks the scope.
 * Deliberately does not name the missing scope: that would tell a probing
 * caller what to look for, and means nothing to the person reading it.
 */
export const NO_PERMISSION_TOOLTIP = "You don't have permission to perform this action.";
