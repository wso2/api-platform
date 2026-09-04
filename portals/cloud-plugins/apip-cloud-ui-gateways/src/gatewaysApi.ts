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

import type { ApiFetch } from './hostPort';
import type { Environment, Gateway, GatewayInput, GatewayType } from './types';

/** Reference-data shapes returned by the platform-api list endpoints. */
type EnvironmentDTO = { id?: string; name: string; isProduction?: boolean };

/**
 * A `/managed-gateways` record: the whole platform-api gateway record plus the
 * cloud-only `environment` and `host`, so the list call alone carries everything
 * the detail view shows.
 */
type ManagedGatewayDTO = {
  id: string;
  displayName?: string;
  description?: string;
  functionalityType?: string;
  version?: string;
  isCritical?: boolean;
  isActive?: boolean;
  createdAt?: string;
  updatedAt?: string;
  environment?: string;
  host?: string;
};

const normalizeType = (functionalityType?: string): GatewayType =>
  functionalityType === 'ai' || functionalityType === 'event' ? functionalityType : 'regular';

/**
 * Projects a `/managed-gateways` record into the view model. The host is
 * server-assigned (shown as the gateway's URL). Status comes from `isActive` —
 * whether the gateway's controller has dialled in to the control plane — so a
 * gateway still being provisioned reads as inactive until it is really up.
 */
const mapGateway = (dto: ManagedGatewayDTO): Gateway => ({
  id: dto.id,
  name: dto.displayName || dto.id,
  description: dto.description,
  type: normalizeType(dto.functionalityType),
  environmentId: dto.environment ?? '',
  url: dto.host ?? '',
  status: dto.isActive ? 'active' : 'inactive',
  isCritical: dto.isCritical ?? false,
  version: dto.version,
  createdAt: dto.createdAt ?? '',
  updatedAt: dto.updatedAt ?? '',
});

/**
 * The managed-gateways data client, built from the host-injected `apiFetch`.
 * Environments are keyed by name (the value a gateway's `environment` field and
 * the create request both use), so the picker's selection round-trips as-is.
 */
export function createGatewaysClient(apiFetch: ApiFetch) {
  return {
    async listGateways(): Promise<Gateway[]> {
      const response = await apiFetch<{ list?: ManagedGatewayDTO[] }>('GET', '/managed-gateways');
      return (response?.list ?? []).map(mapGateway);
    },
    async listEnvironments(): Promise<Environment[]> {
      const response = await apiFetch<{ list?: EnvironmentDTO[] }>('GET', '/environments');
      return (response?.list ?? []).map((dto) => ({ id: dto.name, name: dto.name }));
    },
    async createGateway(input: GatewayInput): Promise<void> {
      await apiFetch('POST', '/managed-gateways', {
        displayName: input.name,
        environment: input.environmentId,
        functionalityType: input.type,
        description: input.description,
        isCritical: false,
      });
    },
    async updateGateway(id: string, input: GatewayInput): Promise<void> {
      // Only display name and description are mutable; environment, type, host
      // and version are fixed at creation and rejected by the update endpoint.
      await apiFetch('PUT', `/managed-gateways/${encodeURIComponent(id)}`, {
        displayName: input.name,
        description: input.description,
      });
    },
    async deleteGateway(id: string): Promise<void> {
      await apiFetch('DELETE', `/managed-gateways/${encodeURIComponent(id)}`);
    },
  };
}
