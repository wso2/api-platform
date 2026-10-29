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

import {
  keepPreviousData,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from '@tanstack/react-query';
import { useEffect } from 'react';

import type { ApiError } from '../../core/errors';
import { useApiScope } from '../../core/scope';
import {
  createMeter,
  deleteMeter,
  updateMeter,
  type ListMetersQuery,
  type Meter,
  type MeterBody,
} from './meters.endpoints';
import { meterKeys, meterQueries } from './meters.queries';

/**
 * The public hook surface for meters. `query.error` is always an `ApiError`.
 * A meter that any plan prices can't change or be deleted; the refusal (409
 * `METER_IN_USE`) arrives with a message that counts the plans.
 */

export type MeterListFilters = ListMetersQuery;

/**
 * The organization's meters, a page at a time; `query` searches their names and ids. Each listed
 * meter is also kept as a single meter, so one picked from the list is known at once.
 */
export const useMeters = (filters: MeterListFilters = {}, overrides: { orgId?: string } = {}) => {
  const { org } = useApiScope(overrides);
  const queryClient = useQueryClient();

  const query = useQuery({
    ...meterQueries.list(org!, filters),
    enabled: Boolean(org),
    placeholderData: keepPreviousData,
  });
  useEffect(() => {
    if (!org) return;
    for (const meter of query.data?.list ?? []) {
      if (meter.id) queryClient.setQueryData(meterKeys.detail(org, meter.id), meter);
    }
  }, [org, query.data, queryClient]);
  return query;
};

const combineMeters = (results: UseQueryResult<Meter>[]) => ({
  isPending: results.some((result) => result.isPending),
  meters: results.flatMap((result) => (result.data ? [result.data] : [])),
});

/**
 * The meters with the given ids, each read on its own and kept, for a screen that needs only the
 * meters its rows use. Repeated and empty ids are read once or not at all; a meter that can't be
 * read is left out. `isPending` is true while any is still loading.
 */
export const useMetersById = (meterIds: readonly string[], overrides: { orgId?: string } = {}) => {
  const { org } = useApiScope(overrides);
  const ids = [...new Set(meterIds.filter(Boolean))];

  return useQueries({
    combine: combineMeters,
    queries: ids.map((meterId) => ({ ...meterQueries.detail(org!, meterId), enabled: Boolean(org) })),
  });
};

/** A single meter. */
export const useMeter = (meterId: string | undefined, overrides: { orgId?: string } = {}) => {
  const { org } = useApiScope(overrides);

  return useQuery({
    ...meterQueries.detail(org!, meterId!),
    enabled: Boolean(org && meterId),
  });
};

/**
 * Invalidation shared by every meter mutation: the meter queries are read again
 * after a write, so the list and each meter's `planCount` come from the server.
 */
const useInvalidateMeters = (orgId?: string) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope({ orgId });

  return () => {
    if (!org) return;
    void queryClient.invalidateQueries({ queryKey: meterKeys.all(org) });
  };
};

export const useCreateMeter = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateMeters(orgId);

  return useMutation<Meter, ApiError, MeterBody>({
    mutationFn: (body) => createMeter(body, { orgId }),
    onSuccess: () => invalidate(),
  });
};

export const useUpdateMeter = (overrides: { orgId?: string } = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateMeters(orgId);

  return useMutation<Meter, ApiError, { meterId: string; body: MeterBody }>({
    mutationFn: ({ meterId, body }) => updateMeter(meterId, body, { orgId }),
    onSuccess: () => invalidate(),
  });
};

export const useDeleteMeter = (overrides: { orgId?: string } = {}) => {
  const { org, orgId } = useApiScope(overrides);
  const queryClient = useQueryClient();
  const invalidate = useInvalidateMeters(orgId);

  return useMutation<void, ApiError, { meterId: string }>({
    mutationFn: ({ meterId }) => deleteMeter(meterId, { orgId }),
    onSuccess: (_result, { meterId }) => {
      if (org) queryClient.removeQueries({ queryKey: meterKeys.detail(org, meterId) });
      invalidate();
    },
  });
};
