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

// A real, BFF-backed EnvironmentPort over platform-api's /environments
// endpoints, built from the host-injected apiFetch. Satisfies the same
// EnvironmentPort interface the mock did, so EnvironmentsList/EnvironmentForm
// need no change.

import type { ApiFetch } from './hostPort';
import type { CreateEnvironmentInput, Environment, EnvironmentPort } from './types';

/** The `/environments` record shape as returned by platform-api. */
type EnvironmentDTO = { id: string; name: string; isProduction?: boolean; createdAt?: string };

const mapEnvironment = (dto: EnvironmentDTO): Environment => ({
  id: dto.id,
  name: dto.name,
  critical: dto.isProduction ?? false,
  createdAt: dto.createdAt ?? '',
});

export function createApiEnvironmentPort(apiFetch: ApiFetch): EnvironmentPort {
  return {
    async list(): Promise<Environment[]> {
      const response = await apiFetch<{ list?: EnvironmentDTO[] }>('GET', '/environments');
      return (response?.list ?? []).map(mapEnvironment);
    },
    async create(input: CreateEnvironmentInput): Promise<Environment> {
      const created = await apiFetch<EnvironmentDTO>('POST', '/environments', {
        name: input.name,
        isProduction: input.critical,
      });
      // A successful create always returns the new record; fall back to the
      // request data if the body is empty so the caller still gets an Environment.
      return created
        ? mapEnvironment(created)
        : { id: input.name, name: input.name, critical: input.critical, createdAt: '' };
    },
    async remove(id: string): Promise<void> {
      await apiFetch('DELETE', `/environments/${encodeURIComponent(id)}`);
    },
  };
}
