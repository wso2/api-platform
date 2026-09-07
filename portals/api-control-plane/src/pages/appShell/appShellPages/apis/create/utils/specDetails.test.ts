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

import { extractApiDetails, extractOperations } from './specDetails';

/**
 * The draft these produce pre-fills the wizard's last step, so the assertions
 * are on which keys are present as much as on their values: a key the document
 * doesn't answer for must be *absent*, not empty, or it would blank out a
 * field the form had already filled.
 */
describe('extractApiDetails', () => {
  it('reads name, version, description and upstream off an OpenAPI 3 document', () => {
    expect(
      extractApiDetails({
        info: { title: 'Orders API', version: '2.1', description: 'Everything orders.' },
        servers: [{ url: 'https://api.example.com/v2/orders' }],
      }),
    ).toEqual({
      description: 'Everything orders.',
      displayName: 'Orders API',
      transports: ['https'],
      upstream: { main: { url: 'https://api.example.com/v2/orders' } },
      version: '2.1',
    });
  });

  it('never returns a context: the base path belongs to the platform, not the document', () => {
    // The form builds `/{project}/{id}/v{version}` and says so in its helper
    // text, so a server URL's path must not quietly become the base path.
    const draft = extractApiDetails({
      info: { title: 'Orders API', version: '1' },
      servers: [{ url: 'https://api.example.com/v2/orders' }],
    });

    expect(draft).not.toHaveProperty('context');
  });

  it('skips a templated server URL and falls through to the next one', () => {
    expect(
      extractApiDetails({
        info: { title: 'Orders API', version: '1' },
        servers: [{ url: 'https://{region}.example.com' }, { url: 'http://fixed.example.com' }],
      }),
    ).toMatchObject({
      transports: ['http'],
      upstream: { main: { url: 'http://fixed.example.com' } },
    });
  });

  it('reads a relative server URL as the upstream, with no transport claimed', () => {
    const draft = extractApiDetails({
      info: { title: 'Orders API', version: '1' },
      servers: [{ url: '/v2' }],
    });

    expect(draft.upstream).toEqual({ main: { url: '/v2' } });
    // Neither http nor https can be told from a relative URL, so nothing is
    // asserted about the transport rather than a wrong default being picked.
    expect(draft).not.toHaveProperty('transports');
  });

  it('assembles the upstream from Swagger 2.0 host/basePath/schemes', () => {
    expect(
      extractApiDetails({
        swagger: '2.0',
        info: { title: 'Orders API', version: '1' },
        host: 'api.example.com',
        basePath: '/v2',
        schemes: ['http', 'https'],
      }),
    ).toMatchObject({
      // https wins when the document offers both.
      transports: ['https'],
      upstream: { main: { url: 'https://api.example.com/v2' } },
    });
  });

  it('omits every key a document says nothing about', () => {
    expect(extractApiDetails({})).toEqual({});
    expect(extractApiDetails(undefined)).toEqual({});
  });
});

describe('extractOperations', () => {
  it('flattens the methods the form can hold and skips the ones it cannot', () => {
    expect(
      extractOperations({
        paths: {
          '/orders': {
            get: { operationId: 'listOrders' },
            post: { summary: 'Place an order' },
            head: { operationId: 'headOrders' },
            options: {},
          },
          // Not a path: a `paths` sibling key that isn't a route.
          'x-internal': { get: {} },
        },
      }),
    ).toEqual([
      { name: 'listOrders', request: { method: 'GET', path: '/orders' } },
      { name: 'Place an order', request: { method: 'POST', path: '/orders' } },
    ]);
  });

  it('names an operation after its method and path when it carries neither id nor summary', () => {
    expect(extractOperations({ paths: { '/orders': { delete: {} } } })).toEqual([
      { name: 'DELETE /orders', request: { method: 'DELETE', path: '/orders' } },
    ]);
  });
});
