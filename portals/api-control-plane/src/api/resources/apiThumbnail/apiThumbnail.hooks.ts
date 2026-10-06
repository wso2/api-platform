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

import { useEffect, useState } from 'react';
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

/**
 * Shared, ref-counted `blob:` URLs keyed by Blob.
 *
 * Every avatar showing the same cached thumbnail reuses one URL, and the URL
 * is revoked only once no mounted component holds it. The revoke is deferred
 * so an immediate remount (StrictMode's mount → cleanup → mount, or a quick
 * route change between pages that both show the avatar) re-acquires the same
 * URL instead of rendering a revoked one, which made the `<img>` fail and fall
 * back to initials until a full refresh.
 */
const objectUrls = new Map<Blob, { url: string; refs: number }>();

const objectUrlFor = (blob: Blob): string => {
  let entry = objectUrls.get(blob);
  if (!entry) {
    entry = { url: URL.createObjectURL(blob), refs: 0 };
    objectUrls.set(blob, entry);
  }
  return entry.url;
};

const retainObjectUrl = (blob: Blob): (() => void) => {
  objectUrlFor(blob);
  objectUrls.get(blob)!.refs += 1;
  return () => {
    const entry = objectUrls.get(blob);
    if (!entry) return;
    entry.refs -= 1;
    setTimeout(() => {
      const current = objectUrls.get(blob);
      if (current && current.refs <= 0) {
        URL.revokeObjectURL(current.url);
        objectUrls.delete(blob);
      }
    }, 0);
  };
};

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

  const blob = query.data?.blob;
  // Create the URL inside the effect, not during render. Calling
  // `objectUrlFor(blob)` in render would store a shared-cache entry with
  // `refs: 0`; if the render is discarded before commit (concurrent mode,
  // Suspense / error interruption, blob swap), the effect never increments
  // the ref count, no revoke timer is scheduled, and the `blob:` URL leaks
  // its backing bytes. Keeping URL creation + retain / release inside the
  // same commit-phase effect ties the URL's lifetime to a mounted consumer.
  const [url, setUrl] = useState<string | undefined>(undefined);
  useEffect(() => {
    if (!blob) {
      setUrl(undefined);
      return;
    }
    setUrl(objectUrlFor(blob));
    return retainObjectUrl(blob);
  }, [blob]);

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
      // returns 204
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
