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

export const DEFAULT_GATEWAY_NAME = 'Default Cloud Gateway';
const DEFAULT_GATEWAY_DESCRIPTION = 'Default API Platform Cloud Gateway';

/** A managed gateway as far as the Deploy page needs: which one, and how far along. */
export type ManagedGatewayState = {
  id: string;
  environment: string;
  displayName: string;
  status: 'provisioning' | 'active' | 'failed' | 'other';
  statusReason?: string;
};

type ManagedGatewayDTO = {
  id: string;
  displayName?: string;
  environment?: string;
  status?: string;
  statusReason?: string;
  isActive?: boolean;
};

// Same reading as the Gateways page: `status` answers the whole question, with
// `isActive` as the fallback for a server that does not report it.
const toState = (dto: ManagedGatewayDTO): ManagedGatewayState => {
  let status: ManagedGatewayState['status'] = 'other';
  if (dto.status === 'PROVISIONING') status = 'provisioning';
  else if (dto.status === 'FAILED') status = 'failed';
  else if (dto.status === 'ACTIVE' || (!dto.status && dto.isActive)) status = 'active';
  return {
    id: dto.id,
    environment: dto.environment ?? '',
    displayName: dto.displayName ?? dto.id,
    status,
    statusReason: dto.statusReason,
  };
};

/**
 * Creates and watches the WSO2 managed gateway the empty Deploy page offers. The
 * calls are the Gateways page's own (`POST /managed-gateways`, `GET
 * /managed-gateways`), so the server rules are the same.
 */
export function createManagedGatewaysClient(apiFetch: ApiFetch) {
  return {
    /** Creates the default managed gateway in `environment`. */
    async createDefaultGateway(environment: string): Promise<{ id?: string }> {
      const created = await apiFetch<{ id?: string }>('POST', '/managed-gateways', {
        displayName: DEFAULT_GATEWAY_NAME,
        description: DEFAULT_GATEWAY_DESCRIPTION,
        environment,
        functionalityType: 'regular',
        isCritical: false,
        isDefault: true,
      });
      return { id: created?.id };
    },

    async listGateways(): Promise<ManagedGatewayState[]> {
      const response = await apiFetch<{ list?: ManagedGatewayDTO[] }>(
        'GET',
        '/managed-gateways?functionalityType=regular'
      );
      return (response?.list ?? []).map(toState);
    },
  };
}
