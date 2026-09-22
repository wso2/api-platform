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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { keepPreviousData, useQuery } from '@tanstack/react-query';

import { useApiScope } from '../../core/scope';
import { apiPortalQueries } from './apiPortals.queries';

/**
 * Org-scoped list of every API Portal registered in the caller's organization.
 * Powers the portal-count metric on the org overview card; also fine for any
 * caller that needs the raw list (`data.list`).
 */
export const useApiPortals = (overrides: { orgId?: string } = {}) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPortalQueries.list(org!),
    enabled: Boolean(org),
    placeholderData: keepPreviousData,
  });
};
