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

import type {
  CreateApiPortalInput,
  ApiPortal,
  UpdateApiPortalInput,
} from '../../types/domain';
import { toApiPortal } from '../adapters';
import { apiPortals } from '../mocks/data';
import { ApiError } from '../types/errors';

/**
 * API Portal management has no platform-api backend yet (console-only feature
 * for now), so this always operates on the in-memory mock store — unlike
 * gatewayClient, there is no real REST endpoint to call. Swap this for a real
 * client (platformGet/platformPost against ApiPortalResponse) once
 * platform-api adds one.
 */
export async function listApiPortals(): Promise<ApiPortal[]> {
  return apiPortals.map(toApiPortal);
}

export async function getApiPortal(id: string): Promise<ApiPortal | undefined> {
  const found = apiPortals.find((item) => item.id === id);
  return found ? toApiPortal(found) : undefined;
}

export async function createApiPortal(
  input: CreateApiPortalInput
): Promise<ApiPortal> {
  // Picked explicitly (not `...input`) so `clientSecret` — the one genuinely
  // write-only field — never ends up on the stored/returned record.
  // stsTokenUrl/clientId are not secret and are stored/returned normally.
  const apiPortal: ApiPortal = {
    id: input.handle,
    name: input.name,
    handle: input.handle,
    description: input.description,
    url: input.url,
    authType: input.authType,
    stsTokenUrl: input.stsTokenUrl,
    clientId: input.clientId,
    workflowStatus: 'pending',
    createdAt: new Date().toISOString(),
  };
  apiPortals.push(apiPortal);
  return toApiPortal(apiPortal);
}

export async function updateApiPortal(
  id: string,
  input: UpdateApiPortalInput
): Promise<ApiPortal> {
  const index = apiPortals.findIndex((item) => item.id === id);
  if (index < 0) {
    throw new ApiError('API Portal not found', 'NOT_FOUND', 404);
  }
  // Same reasoning as createApiPortal: only clientSecret is excluded.
  const updated: ApiPortal = {
    ...apiPortals[index],
    name: input.name,
    description: input.description,
    url: input.url,
    authType: input.authType,
    stsTokenUrl: input.stsTokenUrl,
    clientId: input.clientId,
  };
  apiPortals[index] = updated;
  return toApiPortal(updated);
}

export async function deleteApiPortal(id: string): Promise<void> {
  const index = apiPortals.findIndex((item) => item.id === id);
  if (index < 0) {
    throw new ApiError('API Portal not found', 'NOT_FOUND', 404);
  }
  apiPortals.splice(index, 1);
}
