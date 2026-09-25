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
 * Defines the capabilities that may be presented to the authenticated user.
 *
 * This module provides the public permissions API for the application. Components
 * should import from `'../permissions'` rather than accessing internal files.
 *
 * This layer controls user-interface affordances only. The Platform API remains
 * responsible for enforcing scopes and determining whether an operation succeeds.
 * Consequently, any operation that bypasses these checks must still fail safely.
 * Uncertain cases therefore remain permissive, and 403 handling must continue to
 * function independently of the decisions made here.
 *
 * In this module, a "permission" denotes an `ap:*` scope. By contrast, "scope"
 * refers to the organization, project, or API tier represented in the URL and
 * used by `ConsoleScope`, `ScopeGate`, and `useApiScope`. `PermissionGate` and
 * `ScopeGate` may be used on the same page while evaluating different concerns.
 */

export {
  PERMISSION_ENFORCEMENT_FLAG,
  PermissionProvider,
  type PermissionProviderProps,
} from './PermissionProvider';

export { PermissionContext, usePermissions, type PermissionState } from './PermissionContext';

export { Can, type CanProps, type DeniedBehaviour } from './Can';

export { permissionMessages } from './messages';

export {
  useActionPermission,
  useCan,
  useCanAny,
  useHasScope,
  type ActionPermission,
} from './useCan';

export {
  canOperation,
  reportForbiddenDrift,
  decideAnyOperation,
  decideOperation,
  decideScope,
  requiredScopesFor,
  resetPermissionWarnings,
  toGrantedSet,
  type Decision,
  type DecisionReason,
  type PermissionInput,
  type PermissionMode,
  type PermissionOperation,
} from './evaluate';
