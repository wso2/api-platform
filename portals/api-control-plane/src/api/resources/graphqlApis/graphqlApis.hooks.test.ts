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

import { aGraphQLApi, collection, recorder, type Recorder } from '../../../test/msw';
import { renderApiHook, settle } from '../../../test/renderApiHook';
import { server } from '../../../test/server';
import { resetHttpClient } from '../../core/http';
import { useAllGraphQLApis } from './graphqlApis.hooks';

/**
 * `useAllGraphQLApis` mirrors `useAllRestApis`, which `ApiList` merges it with;
 * the two must window identically or the merged page is wrong. The REST
 * counterpart is covered in `restApis.hooks.test.ts`.
 */

const manyGraphQLApis = (count: number) =>
  Array.from({ length: count }, (_unused, index) =>
    aGraphQLApi({ id: `gql-${index + 1}`, displayName: `GQL ${index + 1}` })
  );

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('useAllGraphQLApis — windowing', () => {
  const offsets = () => requests.calls.map((call) => call.params.get('offset')).sort();

  it('fetches every page when no window is asked for', async () => {
    server.use(collection('/graphql-apis', manyGraphQLApis(250), { record: requests }));

    const { result } = renderApiHook(() => useAllGraphQLApis());

    await waitFor(() => expect(result.current.data?.list).toHaveLength(250));
    expect(offsets()).toEqual(['0', '100', '200']);
  });

  it('fetches only the first `max` items in one request, and still reports the total', async () => {
    server.use(collection('/graphql-apis', manyGraphQLApis(250), { record: requests }));

    const { result } = renderApiHook(() => useAllGraphQLApis({}, {}, { max: 10 }));

    await waitFor(() => expect(result.current.data?.list).toHaveLength(10));
    await settle();
    expect(requests.count()).toBe(1);
    expect(requests.last()?.params.get('limit')).toBe('10');
    expect(result.current.data?.pagination.total).toBe(250);
  });

  it('spans several requests when `max` exceeds the server page size', async () => {
    server.use(collection('/graphql-apis', manyGraphQLApis(250), { record: requests }));

    const { result } = renderApiHook(() => useAllGraphQLApis({}, {}, { max: 150 }));

    await waitFor(() => expect(result.current.data?.list).toHaveLength(150));
    await settle();
    expect(offsets()).toEqual(['0', '100']);
  });

  it('treats `max: 0` as nothing wanted, not as no limit', async () => {
    server.use(collection('/graphql-apis', manyGraphQLApis(5), { record: requests }));

    const { result } = renderApiHook(() => useAllGraphQLApis({}, {}, { max: 0 }));

    await waitFor(() => expect(result.current.data?.list).toEqual([]));
    await settle();
    // One cheap request, so the total still arrives.
    expect(requests.count()).toBe(1);
    expect(requests.last()?.params.get('limit')).toBe('1');
    expect(result.current.data?.pagination.total).toBe(5);
  });

  it('sends no request while disabled', async () => {
    server.use(collection('/graphql-apis', manyGraphQLApis(5), { record: requests }));

    renderApiHook(() => useAllGraphQLApis({}, {}, { enabled: false }));

    await settle();
    expect(requests.count()).toBe(0);
  });
});
