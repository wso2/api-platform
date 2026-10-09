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

import { describe, expect, it } from 'vitest';

import { withServerUrl } from './specServers';

const GATEWAY = 'https://gw.example.com/default/payments-api/v1.0';

describe('withServerUrl', () => {
  it('replaces the document’s own server with the gateway', () => {
    const spec = {
      openapi: '3.0.1',
      servers: [{ url: 'https://apis.bijira.dev/samples/reading-list-api-service/v1.0' }],
    };

    expect(withServerUrl(spec, GATEWAY).servers).toEqual([{ url: GATEWAY }]);
  });

  it('strips a trailing slash from the gateway url', () => {
    expect(withServerUrl({ openapi: '3.0.1' }, `${GATEWAY}/`).servers).toEqual([{ url: GATEWAY }]);
  });

  it('removes a path-level server override', () => {
    const spec = {
      openapi: '3.0.1',
      paths: { '/books': { servers: [{ url: 'https://elsewhere.example.com' }], get: {} } },
    };

    // Left standing, this override wins over the top-level array and the
    // request quietly goes to the definition's origin, not the user's gateway.
    const paths = withServerUrl(spec, GATEWAY).paths as Record<string, Record<string, unknown>>;
    expect(paths['/books']).not.toHaveProperty('servers');
  });

  it('removes an operation-level server override', () => {
    const spec = {
      openapi: '3.0.1',
      paths: { '/books': { get: { servers: [{ url: 'https://elsewhere.example.com' }] } } },
    };

    const paths = withServerUrl(spec, GATEWAY).paths as Record<
      string,
      Record<string, Record<string, unknown>>
    >;
    expect(paths['/books'].get).not.toHaveProperty('servers');
  });

  it('keeps everything else on the operation intact', () => {
    const spec = {
      openapi: '3.0.1',
      paths: {
        '/books': {
          get: { operationId: 'listBooks', servers: [{ url: 'https://x.example' }], responses: {} },
        },
      },
    };

    const paths = withServerUrl(spec, GATEWAY).paths as Record<
      string,
      Record<string, Record<string, unknown>>
    >;
    expect(paths['/books'].get).toMatchObject({ operationId: 'listBooks', responses: {} });
  });

  it('does not mutate the input document', () => {
    const spec = {
      openapi: '3.0.1',
      servers: [{ url: 'https://original.example' }],
      paths: { '/books': { get: { servers: [{ url: 'https://x.example' }] } } },
    };

    withServerUrl(spec, GATEWAY);

    // The definition is cached by React Query and shared with other consumers;
    // rewriting it in place would corrupt that entry.
    expect(spec.servers).toEqual([{ url: 'https://original.example' }]);
    expect(spec.paths['/books'].get.servers).toEqual([{ url: 'https://x.example' }]);
  });

  it('composes Swagger 2’s split address fields', () => {
    const spec = { swagger: '2.0', host: 'old.example.com', basePath: '/old', schemes: ['http'] };

    expect(withServerUrl(spec, GATEWAY)).toMatchObject({
      schemes: ['https'],
      host: 'gw.example.com',
      basePath: '/default/payments-api/v1.0',
    });
  });

  it('uses / as the Swagger 2 basePath when the gateway has no path', () => {
    expect(withServerUrl({ swagger: '2.0' }, 'https://gw.example.com')).toMatchObject({
      basePath: '/',
    });
  });

  it('removes a Swagger 2 operation-level schemes override', () => {
    const spec = { swagger: '2.0', paths: { '/books': { get: { schemes: ['http'] } } } };

    const paths = withServerUrl(spec, GATEWAY).paths as Record<
      string,
      Record<string, Record<string, unknown>>
    >;
    expect(paths['/books'].get).not.toHaveProperty('schemes');
  });

  it('leaves the document alone when no gateway is resolved yet', () => {
    const spec = { openapi: '3.0.1', servers: [{ url: 'https://original.example' }] };

    // Rendering the document as written beats rendering a broken one while the
    // gateway list is still loading.
    expect(withServerUrl(spec, '')).toBe(spec);
    expect(withServerUrl(spec, '   ')).toBe(spec);
  });

  it('leaves the document alone when the gateway url is not a url', () => {
    const spec = { openapi: '3.0.1' };

    expect(withServerUrl(spec, 'gw.example.com')).toBe(spec);
  });

  it('tolerates a document with no paths', () => {
    expect(withServerUrl({ openapi: '3.0.1' }, GATEWAY).servers).toEqual([{ url: GATEWAY }]);
  });
});
