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

import type { DevPortalAuthType, DevPortalWorkflowStatus } from '../../types/domain';

/** Shared across the list, create, and detail/edit pages so the three never drift. */
export const AUTH_TYPE_OPTIONS: { value: DevPortalAuthType; label: string }[] = [
  { value: 'local', label: 'Local' },
  { value: 'idp_client_credentials', label: 'IdP Client Credentials' },
];

export const AUTH_LABEL: Record<DevPortalAuthType, string> = {
  local: 'Local',
  idp_client_credentials: 'IdP Client Credentials',
};

export const STATUS_LABEL: Record<DevPortalWorkflowStatus, string> = {
  pending: 'Pending',
  active: 'Active',
  failed: 'Failed',
};

export const STATUS_COLOR: Record<DevPortalWorkflowStatus, string> = {
  pending: 'warning.main',
  active: 'success.main',
  failed: 'error.main',
};

export const STATUS_CHIP_COLOR: Record<
  DevPortalWorkflowStatus,
  'warning' | 'success' | 'error'
> = {
  pending: 'warning',
  active: 'success',
  failed: 'error',
};
