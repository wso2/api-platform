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

import { waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';

import {
  aSecret,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  type Recorder,
} from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import {
  isSecretInUse,
  useCreateSecret,
  useDeleteSecret,
  useRotateSecret,
  useSecrets,
} from './secrets.hooks';
import { secretKeys } from './secrets.queries';

/**
 * Hook-layer tests for the **no-cache-write** shape: mutations that invalidate
 * but deliberately never seed the cache with their response.
 *
 * Stands in for `secrets`, `organizations` and `gatewayCustomPolicies`. Secrets
 * is the representative because its reason is the strongest: `createSecret` and
 * `rotateSecret` return a value that must not linger in a store any component
 * can read. `organizations` and `gatewayCustomPolicies` share the mechanics and
 * differ only in that they *do* seed, which the restApis file already covers.
 *
 * The negative assertions here are the point. "Nothing was written to the
 * cache" is invisible from a component — the screen looks identical either way.
 */

const SECRET_ID = 'signing-key';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('useSecrets — scope gating', () => {
  it('fetches once the organization is known', async () => {
    server.use(collection('/secrets', [aSecret()], { record: requests }));

    const { result } = renderApiHook(() => useSecrets());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requests.count()).toBe(1);
  });

  it('issues no request while the organization is unknown', async () => {
    server.use(collection('/secrets', [], { record: requests }));

    renderApiHook(() => useSecrets(), { orgId: undefined });

    await settle();
    expect(requests.count()).toBe(0);
  });

  it('does not require a project, unlike project-scoped resources', async () => {
    // Secrets are organization-scoped. Gating them on a project would leave the
    // list permanently empty on any screen outside one.
    server.use(collection('/secrets', [aSecret()], { record: requests }));

    const { result } = renderApiHook(() => useSecrets(), { projectId: undefined });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
  });
});

describe('useCreateSecret — deliberately does not seed the cache', () => {
  it('writes nothing into the detail cache', async () => {
    // The create response describes a secret the user just supplied a value
    // for. Keeping it out of the store is the whole reason this shape exists.
    server.use(accepts('post', '/secrets', aSecret({ id: 'new-secret' })));

    const { result, queryClient, org } = renderApiHook(() => useCreateSecret());
    result.current.mutate({
      displayName: 'New Secret',
      value: 's3cret',
      type: 'GENERIC',
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(
      queryClient.getQueryData(secretKeys.detail(org, 'new-secret'))
    ).toBeUndefined();
  });

  it('still invalidates every list variant, so the new secret appears', async () => {
    server.use(accepts('post', '/secrets', aSecret({ id: 'new-secret' })));

    const { result, queryClient, org } = renderApiHook(() => useCreateSecret());
    const listKey = secretKeys.list(org, { limit: 10 });
    queryClient.setQueryData(listKey, { count: 0, list: [], pagination: {} });

    result.current.mutate({
      displayName: 'New Secret',
      value: 's3cret',
      type: 'GENERIC',
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() =>
      expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
    );
  });

  it('never leaves the submitted value anywhere in the query cache', async () => {
    // Broader than the assertion above: no entry anywhere should contain it.
    server.use(accepts('post', '/secrets', aSecret({ id: 'new-secret' })));

    const { result, queryClient } = renderApiHook(() => useCreateSecret());
    result.current.mutate({
      displayName: 'New Secret',
      value: 'super-secret-value',
      type: 'GENERIC',
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const everythingCached = JSON.stringify(
      queryClient.getQueryCache().getAll().map((query) => query.state.data)
    );
    expect(everythingCached).not.toContain('super-secret-value');
  });

  it('releases the submitted value from the mutation cache once the form unmounts', async () => {
    // React Query also keeps a settled mutation's `variables` (the plaintext).
    // `gcTime: 0` ensures those variables are released; this test preserves that.
    server.use(accepts('post', '/secrets', aSecret({ id: 'new-secret' })));

    const { result, queryClient, unmount } = renderApiHook(() => useCreateSecret());
    result.current.mutate({
      displayName: 'New Secret',
      value: 'super-secret-value',
      type: 'GENERIC',
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // While mounted, `useMutation` still exposes submitted `variables`.
    // Clearing them requires `reset()`, which also clears rendered success.
    // Forms can call `reset()` after showing the result.
    expect(JSON.stringify(result.current.variables)).toContain('super-secret-value');

    unmount();

    await waitFor(() => {
      const retained = JSON.stringify(
        queryClient.getMutationCache().getAll().map((mutation) => mutation.state.variables)
      );
      expect(retained).not.toContain('super-secret-value');
    });
  });
});

describe('useRotateSecret', () => {
  it('performs no optimistic write', async () => {
    // Rotation changes server-managed metadata (updatedAt, hash) that cannot be
    // predicted client-side, so guessing would show values that are simply wrong.
    server.use(accepts('put', `/secrets/${SECRET_ID}`, aSecret(), { status: 200 }));

    const { result, queryClient, org } = renderApiHook(() => useRotateSecret());
    const detailKey = secretKeys.detail(org, SECRET_ID);
    queryClient.setQueryData(detailKey, aSecret({ displayName: 'Signing Key' }));

    result.current.mutate({
      secretId: SECRET_ID,
      body: { displayName: 'Signing Key', value: 'rotated' },
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // Unchanged in place; the refresh comes from invalidation, not a local edit.
    expect(
      queryClient.getQueryData<{ displayName: string }>(detailKey)?.displayName
    ).toBe('Signing Key');
  });

  it('invalidates so the refreshed metadata is re-read', async () => {
    server.use(accepts('put', `/secrets/${SECRET_ID}`, aSecret(), { status: 200 }));

    const { result, queryClient, org } = renderApiHook(() => useRotateSecret());
    const listKey = secretKeys.list(org);
    queryClient.setQueryData(listKey, { count: 0, list: [], pagination: {} });

    result.current.mutate({
      secretId: SECRET_ID,
      body: { displayName: 'Signing Key', value: 'rotated' },
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() =>
      expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
    );
  });
});

describe('useDeleteSecret', () => {
  it('drops the detail entry', async () => {
    server.use(noContent('delete', `/secrets/${SECRET_ID}`));

    const { result, queryClient, org } = renderApiHook(() => useDeleteSecret());
    queryClient.setQueryData(secretKeys.detail(org, SECRET_ID), aSecret());

    result.current.mutate({ secretId: SECRET_ID });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(
      queryClient.getQueryData(secretKeys.detail(org, SECRET_ID))
    ).toBeUndefined();
  });

  it('keeps the cache intact when the server refuses the delete', async () => {
    // A blocked delete must not evict an entry the resource still has — the UI
    // would show the secret as gone while it is very much still there.
    server.use(
      failure('delete', `/secrets/${SECRET_ID}`, 409, 'SECRET_IN_USE', {
        details: { referencedBy: ['pizza-shack'] },
      })
    );

    const { result, queryClient, org } = renderApiHook(() => useDeleteSecret());
    queryClient.setQueryData(secretKeys.detail(org, SECRET_ID), aSecret());

    result.current.mutate({ secretId: SECRET_ID });
    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(queryClient.getQueryData(secretKeys.detail(org, SECRET_ID))).toBeDefined();
  });

  it('surfaces the in-use guard with the blocking resources attached', async () => {
    // "In use by 1 API" needs `details`; "delete failed" does not, and is what
    // the user gets if this is lost.
    server.use(
      failure('delete', `/secrets/${SECRET_ID}`, 409, 'SECRET_IN_USE', {
        details: { referencedBy: ['pizza-shack'] },
      })
    );

    const { result } = renderApiHook(() => useDeleteSecret());
    result.current.mutate({ secretId: SECRET_ID });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(isSecretInUse(result.current.error)).toBe(true);
    expect(result.current.error?.details).toEqual({
      referencedBy: ['pizza-shack'],
    });
  });

  it('isSecretInUse is false for any other failure', async () => {
    server.use(failure('delete', `/secrets/${SECRET_ID}`, 403, 'FORBIDDEN'));

    const { result } = renderApiHook(() => useDeleteSecret());
    result.current.mutate({ secretId: SECRET_ID });

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(isSecretInUse(result.current.error)).toBe(false);
  });
});
