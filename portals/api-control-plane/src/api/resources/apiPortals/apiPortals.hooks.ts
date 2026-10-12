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
import type { ListApiPortalsQuery } from './apiPortals.endpoints';
import { apiPortalQueries } from './apiPortals.queries';

/** Everything a caller may vary on the list request. */
export type ApiPortalListFilters = ListApiPortalsQuery;

/**
 * Org-scoped list of API Portals for the caller's organization.
 *
 * Returns the server's default first page unless `filters` narrows or pages it.
 * The org-overview count reads `data.pagination.total`, which the server
 * reports for the full result set and is therefore independent of the page
 * size chosen here.
 *
 * `keepPreviousData` matches the other list hooks; org-switch callers must
 * treat `isPlaceholderData` as unavailable so the prior org's data does not
 * leak into the new view.
 */
export const useApiPortals = (
  filters: ApiPortalListFilters = {},
  overrides: { orgId?: string } = {},
) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...apiPortalQueries.list(org!, filters),
    enabled: Boolean(org),
    placeholderData: keepPreviousData,
  });
};
