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

import type { Api, ApiKind, ApiStatus } from '../../types/domain';

export const COMPONENT_KIND_LABEL: Record<ApiKind, string> = {
  API_PROXY: 'API Proxy',
  SERVICE: 'Service',
  WEB_APP: 'Web App',
};

export type ChipColor =
  | 'default'
  | 'primary'
  | 'success'
  | 'warning'
  | 'error'
  | 'info';

export const componentStatusColor = (status: ApiStatus): ChipColor => {
  switch (status) {
    case 'ACTIVE':
      return 'success';
    case 'PENDING':
      return 'warning';
    case 'FAILED':
      return 'error';
    default:
      return 'default';
  }
};

export type ApiGroups = {
  apiProxies: Api[];
  others: Api[];
};

/** Splits components into the "API Proxies" section and everything else. */
export const groupApisByKind = (components: Api[]): ApiGroups => ({
  apiProxies: components.filter((component) => component.kind === 'API_PROXY'),
  others: components.filter((component) => component.kind !== 'API_PROXY'),
});

export const filterApis = (components: Api[], search: string): Api[] => {
  const term = search.trim().toLowerCase();
  if (!term) return components;
  return components.filter((component) =>
    [component.displayName, component.name, component.description].some(
      (field) => field?.toLowerCase().includes(term)
    )
  );
};
