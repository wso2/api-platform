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

import type { CreateDevPortalInput, DevPortal } from '../../types/domain';
import { toDevPortal } from '../adapters';
import { devPortals } from '../mocks/data';
import { ApiError } from '../types/errors';

/**
 * Devportal management has no platform-api backend yet (console-only feature
 * for now), so this always operates on the in-memory mock store — unlike
 * gatewayClient, there is no real REST endpoint to call. Swap this for a real
 * client (platformGet/platformPost against DevPortalResponse) once
 * platform-api adds one.
 */
export async function listDevPortals(): Promise<DevPortal[]> {
  return devPortals.map(toDevPortal);
}

export async function createDevPortal(
  input: CreateDevPortalInput
): Promise<DevPortal> {
  const devPortal: DevPortal = {
    ...input,
    id: input.handle,
    workflowStatus: 'pending',
    createdAt: new Date().toISOString(),
  };
  devPortals.push(devPortal);
  return toDevPortal(devPortal);
}

export async function deleteDevPortal(id: string): Promise<void> {
  const index = devPortals.findIndex((item) => item.id === id);
  if (index < 0) {
    throw new ApiError('Devportal not found', 'NOT_FOUND', 404);
  }
  devPortals.splice(index, 1);
}
