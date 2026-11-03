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
 * OpenAPI spec ("API Platform Cloud Extension API": `/managed-gateways` and
 * `/environments`). Copied rather than generated: that spec lives with the
 * cloud plugin, outside this repo's codegen, so this is the one place the
 * feature names a scope.
 *
 * Each list is passed whole to the host's `permissions.canAny`, which is how
 * platform-api's `ScopeEnforcer` admits a call — one held scope matching one
 * accepted entry — so the broader `:manage` scope is listed beside the action
 * and the feature never reasons about inheritance itself.
 *
 * One departure from the cloud spec as it stands: it declares the gateway
 * lists (managed and self-hosted alike) with `ap:gateway:create`/`:manage`
 * only, while every other gateway read on the platform — the main spec's
 * `GET /gateways`, a managed gateway's configuration below — accepts
 * `ap:gateway:read`. That is the scope the list is meant to accept too, and the
 * cloud spec is to be corrected to match, so it is listed here already: a
 * read-only user is shown the page, and until the spec catches up a refused
 * list is reported on it rather than the page being hidden from a user it is
 * meant for.
 */
export const GATEWAY_OPERATIONS = {
  ListEnvironments: ['ap:environment:read', 'ap:environment:manage'],
  ListManagedGateways: ['ap:gateway:read', 'ap:gateway:create', 'ap:gateway:manage'],
  CreateManagedGateway: ['ap:gateway:create', 'ap:gateway:manage'],
  UpdateManagedGateway: ['ap:gateway:manage'],
  DeleteManagedGateway: ['ap:gateway:delete', 'ap:gateway:manage'],
  GetManagedGatewayConfiguration: ['ap:gateway:read', 'ap:gateway:manage'],
  UpdateManagedGatewayConfiguration: ['ap:gateway:update', 'ap:gateway:manage'],
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
