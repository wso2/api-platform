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
  accepts,
  failure,
  listEnvelope,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import {
  useCreateApiKey,
  useMyApiKeys,
  useRevokeApiKey,
  useUpdateApiKey,
} from './apiKeys.hooks';
import { apiKeyKeys } from './apiKeys.queries';

/**
 * Hook-layer tests for the **asymmetric-scoping** shape, which only this
 * resource has.
 *
 * Writes are scoped to one REST API (`/rest-apis/{id}/api-keys`), but the only
 * read the spec offers is the caller-scoped `/me/api-keys`. So a mutation on
 * one API invalidates a list that spans every artifact — which reads like a bug
 * until you know why, and is exactly the kind of thing a test should state
 * rather than leave to a comment.
 *
 * Nothing here seeds the cache: `createApiKey` returns a plaintext key that
 * exists once, and the same reasoning as gateway tokens applies.
 */

const API_ID = 'pizza-shack';
const KEYS = `/rest-apis/${API_ID}/api-keys`;

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('useMyApiKeys', () => {
  it('reads the caller-scoped collection, not a per-API one', async () => {
    // There is no per-API listing in the spec. A screen implying otherwise
    // would be showing the current user's keys and calling them the API's.
    server.use(resource('/me/api-keys', listEnvelope([]), { record: requests }));

    const { result } = renderApiHook(() => useMyApiKeys());

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requests.last()?.url.pathname).toBe('/api/v0.9/me/api-keys');
  });

  it('issues no request while the organization is unknown', async () => {
    server.use(resource('/me/api-keys', listEnvelope([]), { record: requests }));

    renderApiHook(() => useMyApiKeys(), { orgId: undefined });

    await settle();
    expect(requests.count()).toBe(0);
  });

  it('passes the artifact-type filter through to the server', async () => {
    server.use(resource('/me/api-keys', listEnvelope([]), { record: requests }));

    const { result } = renderApiHook(() => useMyApiKeys({ type: ['RestApi'] }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requests.last()?.params.getAll('type')).toEqual(['RestApi']);
  });

  it('keeps differently-filtered lists in separate cache entries', async () => {
    server.use(resource('/me/api-keys', listEnvelope([])));

    const { result, queryClient, org } = renderApiHook(() =>
      useMyApiKeys({ type: ['RestApi'] })
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(
      queryClient.getQueryData(apiKeyKeys.list(org, { type: ['RestApi'] }))
    ).toBeDefined();
    expect(
      queryClient.getQueryData(apiKeyKeys.list(org, { type: ['LlmProxy'] }))
    ).toBeUndefined();
  });
});

describe('useCreateApiKey', () => {
  it('writes the plaintext key nowhere in the cache', async () => {
    // The key is returned once and never again. Putting it in the query store
    // would make a one-shot secret readable by any component thereafter.
    server.use(
      accepts('post', KEYS, {
        status: 'success',
        message: 'created',
        keyId: 'key-1',
        apiKey: 'plaintext-once',
      })
    );

    const { result, queryClient } = renderApiHook(() => useCreateApiKey());
    result.current.mutate({ restApiId: API_ID, body: { name: 'ci-key' } as never });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const everythingCached = JSON.stringify(
      queryClient.getQueryCache().getAll().map((query) => query.state.data)
    );
    expect(everythingCached).not.toContain('plaintext-once');
  });

  it('returns the key to the caller, which is the only place it exists', async () => {
    server.use(
      accepts('post', KEYS, {
        status: 'success',
        message: 'created',
        apiKey: 'plaintext-once',
      })
    );

    const { result } = renderApiHook(() => useCreateApiKey());
    result.current.mutate({ restApiId: API_ID, body: { name: 'ci-key' } as never });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toMatchObject({ apiKey: 'plaintext-once' });
  });

  it('invalidates the caller-scoped list, because no per-API list exists', async () => {
    // The surprising bit, stated as a test: a mutation on one API refreshes a
    // list spanning every artifact, since that is the only list there is.
    server.use(accepts('post', KEYS, { status: 'success', message: 'created' }));

    const { result, queryClient, org } = renderApiHook(() => useCreateApiKey());
    const listKey = apiKeyKeys.list(org, { type: ['RestApi'] });
    queryClient.setQueryData(listKey, listEnvelope([]));

    result.current.mutate({ restApiId: API_ID, body: { name: 'ci-key' } as never });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() =>
      expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
    );
  });
});

describe('useUpdateApiKey', () => {
  it('makes no optimistic write, since the cached list may not contain the key', async () => {
    // The list is caller-scoped: another user could have created this key, so
    // patching an entry by id would be guesswork.
    server.use(accepts('put', `${KEYS}/key-1`, { status: 'success' }, { status: 200 }));

    const { result, queryClient, org } = renderApiHook(() => useUpdateApiKey());
    const listKey = apiKeyKeys.list(org);
    queryClient.setQueryData(listKey, listEnvelope([]));

    result.current.mutate({
      restApiId: API_ID,
      apiKeyId: 'key-1',
      body: { name: 'renamed' } as never,
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData(listKey)).toEqual(listEnvelope([]));
  });
});

describe('useRevokeApiKey', () => {
  it('invalidates the list so the revoked key disappears', async () => {
    server.use(noContent('delete', `${KEYS}/key-1`));

    const { result, queryClient, org } = renderApiHook(() => useRevokeApiKey());
    const listKey = apiKeyKeys.list(org);
    queryClient.setQueryData(listKey, listEnvelope([]));

    result.current.mutate({ restApiId: API_ID, apiKeyId: 'key-1' });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await waitFor(() =>
      expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(true)
    );
  });

  it('does not invalidate when revocation fails', async () => {
    // A failed revoke that refreshed the list would suggest something changed.
    // Worse, a UI that removed the row would be showing a live credential as
    // gone — the worst outcome for this operation.
    server.use(failure('delete', `${KEYS}/key-1`, 403, 'FORBIDDEN'));

    const { result, queryClient, org } = renderApiHook(() => useRevokeApiKey());
    const listKey = apiKeyKeys.list(org);
    queryClient.setQueryData(listKey, listEnvelope([]));

    result.current.mutate({ restApiId: API_ID, apiKeyId: 'key-1' });
    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(queryClient.getQueryState(listKey)?.isInvalidated).toBe(false);
  });
});
