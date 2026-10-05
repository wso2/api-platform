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

import { useEffect, useMemo } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import type { ApiError } from '../../core/errors';
import { useApiScope } from '../../core/scope';
import {
  deleteApiThumbnail,
  getApiThumbnail,
  upsertApiThumbnail,
} from './apiThumbnail.endpoints';
import { apiThumbnailKeys, apiThumbnailParentId, apiThumbnailQueries } from './apiThumbnail.queries';

/**
 * Public hook surface for API thumbnails.
 *
 * The GET hook returns a browser `objectURL` for an `<img src>` — the raw Blob
 * is cached by React Query so multiple `<img>`s targeting the same API share
 * one fetch. The URL is revoked on unmount / blob swap to avoid leaking the
 * `blob:` handle. "No thumbnail set" is resolved at the endpoint to a `null`
 * successful value (not an error), so the UI can distinguish it from a real
 * failure and the delete mutation can push the empty state into the cache
 * directly for an instant UI flip.
 */

type Overrides = { orgId?: string };

type UseApiThumbnailResult = {
  /** `blob:` URL for an `<img>` tag. `undefined` while loading, when no thumbnail exists, or on a real error. */
  url: string | undefined;
  /** True on first fetch (branch here, not on `isLoading`, since the query is scope-gated). */
  isPending: boolean;
  /** A real error. "No thumbnail" is a successful `null` value, not an error. */
  error: ApiError | null;
};

/** Fetch and expose the artifact's thumbnail as a `blob:` URL ready for `<img src>`. */
export const useApiThumbnail = (
  apiType: string,
  apiId: string | undefined,
  overrides: Overrides = {}
): UseApiThumbnailResult => {
  const { org } = useApiScope(overrides);

  const query = useQuery({
    ...apiThumbnailQueries.blob(org!, apiType, apiId!),
    enabled: Boolean(org && apiId),
  });

  // Build the object URL from the fetched Blob and release it when the Blob
  // changes or the component unmounts. A stale URL would point at freed bytes.
  const url = useMemo(
    () => (query.data?.blob ? URL.createObjectURL(query.data.blob) : undefined),
    [query.data?.blob]
  );
  useEffect(() => {
    if (!url) return;
    return () => URL.revokeObjectURL(url);
  }, [url]);

  return { url, isPending: query.isPending, error: query.error as ApiError | null };
};

/**
 * Invalidates one API's thumbnail cache so every live `<img>` on the page
 * refetches on next render.
 */
const useInvalidateApiThumbnail = (overrides: Overrides) => {
  const queryClient = useQueryClient();
  const { org } = useApiScope(overrides);

  return (apiType: string, apiId: string) => {
    if (!org) return;
    void queryClient.invalidateQueries({
      queryKey: apiThumbnailKeys.detail(org, apiThumbnailParentId(apiType, apiId)),
    });
  };
};

type ParentArgs = { apiType: string; apiId: string };

export const useUpsertApiThumbnail = (overrides: Overrides = {}) => {
  const { orgId } = useApiScope(overrides);
  const invalidate = useInvalidateApiThumbnail(overrides);

  return useMutation<void, ApiError, ParentArgs & { file: Blob }>({
    mutationFn: ({ apiType, apiId, file }) =>
      upsertApiThumbnail(apiType, apiId, file, { orgId }),
    onSuccess: (_data, { apiType, apiId }) => invalidate(apiType, apiId),
  });
};

export const useDeleteApiThumbnail = (overrides: Overrides = {}) => {
  const queryClient = useQueryClient();
  const { org, orgId } = useApiScope(overrides);

  return useMutation<void, ApiError, ParentArgs>({
    mutationFn: ({ apiType, apiId }) => deleteApiThumbnail(apiType, apiId, { orgId }),
    onSuccess: (_data, { apiType, apiId }) => {
      // Push the "no thumbnail" state into the cache directly instead of
      // invalidating-and-refetching. Invalidation would trigger a GET that
      // returns 404 — React Query treats that as an error and *keeps the
      // previous data on screen*, so the old thumbnail would linger until a
      // full page refresh. Setting the data to `null` (which the endpoint
      // also returns for 404) flips every live `<img>` to the initials
      // fallback on the next render with zero extra network calls.
      if (!org) return;
      queryClient.setQueryData(
        apiThumbnailQueries.blob(org, apiType, apiId).queryKey,
        null
      );
    },
  });
};

// Re-exports kept for parity with the file structure other resources use.
export { getApiThumbnail };
