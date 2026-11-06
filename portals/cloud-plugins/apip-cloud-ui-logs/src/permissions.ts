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

import type { PermissionsPort } from './hostPort';

/**
 * The accepted-scope list of each Cloud Extension API operation this feature
 * calls, as the operation's `security` block declares it in the cloud plugin's
 * OpenAPI spec ("API Platform Cloud Extension API": `/logs` and
 * `/environments`). Copied rather than generated: that spec lives with the
 * cloud plugin, outside this repo's codegen, so this is the one place the
 * feature names a scope.
 *
 * Each list is passed whole to the host's `permissions.canAny`, which is how
 * platform-api's `ScopeEnforcer` admits a call — one held scope matching one
 * accepted entry — so the broader `:manage` scope is listed beside the action
 * and the feature never reasons about inheritance itself. Logs are read under
 * the gateway scopes: every line is a gateway's, which is what the query is
 * scoped by.
 */
export const LOG_OPERATIONS = {
  QueryGatewayLogs: ['ap:gateway:read', 'ap:gateway:manage'],
  ListEnvironments: ['ap:environment:read', 'ap:environment:manage'],
} as const satisfies Record<string, readonly string[]>;


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
