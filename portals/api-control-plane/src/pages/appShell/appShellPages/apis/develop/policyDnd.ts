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

import type { PolicySummary } from '../../../../../api/policyHub/policyHubClient';

/**
 * Native HTML5 drag-and-drop for the policy workspace. The catalog (right panel)
 * is the drag source; the Policies panel zones (left) are drop targets. We carry
 * a lightweight policy ref both on `dataTransfer` (for native semantics) and in a
 * module-level holder (because `dataTransfer.getData` is empty during dragover,
 * where we need the payload to validate/highlight).
 */
export const POLICY_DND_MIME = 'application/x-policy-ref';

export type DraggedPolicy = Pick<
  PolicySummary,
  'name' | 'version' | 'displayName'
>;

let dragged: DraggedPolicy | null = null;

export const setDraggedPolicy = (policy: DraggedPolicy | null) => {
  dragged = policy;
};

export const getDraggedPolicy = (): DraggedPolicy | null => dragged;

/** Scope a dropped policy targets. */
export type PolicyScope = { kind: 'api' } | { kind: 'operation'; index: number };

export const scopeId = (scope: PolicyScope): string =>
  scope.kind === 'api' ? 'api' : `op-${scope.index}`;
