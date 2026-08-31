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

import type { ApiDetail, CreateApiInput } from '../../types/domain';
import {
  createRestApiBody,
  detailToRestApiBody,
  restApiToApi,
  restApiToDetail,
} from './platformAdapters';

const base: CreateApiInput = {
  name: 'pizza-shack',
  displayName: 'Pizza Shack API',
  kind: 'API_PROXY',
  version: '1.0.0',
};

describe('createRestApiBody', () => {
  it('maps the required fields with id=handle and displayName=display', () => {
    const body = createRestApiBody(
      { ...base, apiContext: 'pizza', prodUrl: 'https://backend:5000' },
      'proj-1',
    );
    expect(body).toMatchObject({
      id: 'pizza-shack',
      displayName: 'Pizza Shack API',
      context: 'pizza',
      version: '1.0.0',
      projectId: 'proj-1',
      transport: ['http', 'https'],
      upstream: { main: { url: 'https://backend:5000' } },
    });
  });

  it('falls back to the slug for an empty context and omits upstream when no URL', () => {
    const body = createRestApiBody(base, 'proj-1');
    expect(body.context).toBe('pizza-shack');
    expect(body.upstream).toBeUndefined();
  });

  it('includes the sandbox upstream only when provided', () => {
    const body = createRestApiBody(
      { ...base, prodUrl: 'https://p', sandboxUrl: 'https://s' },
      'proj-1',
    );
    expect(body.upstream).toEqual({
      main: { url: 'https://p' },
      sandbox: { url: 'https://s' },
    });
  });

  it('honors a custom transport list', () => {
    const body = createRestApiBody({ ...base, transport: ['https'] }, 'proj-1');
    expect(body.transport).toEqual(['https']);
  });

  it('attaches backend auth to main (and sandbox) upstreams', () => {
    const body = createRestApiBody(
      {
        ...base,
        prodUrl: 'https://p',
        sandboxUrl: 'https://s',
        upstreamAuth: { type: 'api-key', header: 'X-API-Key', value: 'secret' },
      },
      'proj-1',
    );
    const auth = { type: 'api-key', header: 'X-API-Key', value: 'secret' };
    expect(body.upstream).toEqual({
      main: { url: 'https://p', auth },
      sandbox: { url: 'https://s', auth },
    });
  });

  it('omits auth when no backend URL is set', () => {
    const body = createRestApiBody(
      { ...base, upstreamAuth: { type: 'bearer', value: 't' } },
      'proj-1',
    );
    expect(body.upstream).toBeUndefined();
  });

  it('includes imported operations under request', () => {
    const body = createRestApiBody(
      {
        ...base,
        prodUrl: 'https://p',
        operations: [
          {
            name: 'listOrders',
            description: 'List',
            method: 'GET',
            path: '/orders',
          },
        ],
      },
      'proj-1',
    );
    expect(body.operations).toEqual([
      {
        name: 'listOrders',
        description: 'List',
        request: { method: 'GET', path: '/orders' },
      },
    ]);
  });
});

describe('restApiToApi', () => {
  it('uses id as handle, name fallback to id, kind API_PROXY, httpBased true', () => {
    expect(restApiToApi({ id: 'orders-api', projectId: 'p1' })).toMatchObject({
      id: 'orders-api',
      name: 'orders-api',
      displayName: 'orders-api',
      handler: 'orders-api',
      kind: 'API_PROXY',
      httpBased: true,
    });
  });

  it('uses name as the display name when present', () => {
    expect(restApiToApi({ id: 'orders-api', name: 'Orders' })).toMatchObject({
      name: 'Orders',
      displayName: 'Orders',
      handler: 'orders-api',
    });
  });

  it('maps lifeCycleStatus to ApiStatus', () => {
    const status = (s: string) => restApiToApi({ id: 'x', lifeCycleStatus: s }).status;
    expect(status('PUBLISHED')).toBe('ACTIVE');
    expect(status('STAGED')).toBe('PENDING');
    expect(status('CREATED')).toBe('DRAFT');
    expect(status('DEPRECATED')).toBe('FAILED');
    expect(status('RETIRED')).toBe('FAILED');
    expect(status('WAT')).toBe('DRAFT');
  });
});

describe('restApiToDetail', () => {
  it('extracts operations, policies, upstream endpoints and preserves raw', () => {
    const raw = {
      id: 'orders-api',
      context: '/orders',
      transport: ['https'],
      operations: [
        {
          name: 'list',
          request: {
            method: 'get',
            path: '/items',
            policies: [{ name: 'p1', version: '2.0.0' }],
          },
        },
      ],
      policies: [{ name: 'cors' }],
      upstream: { main: { url: 'https://p' }, sandbox: { url: 'https://s' } },
    };
    const detail = restApiToDetail(raw);
    expect(detail.context).toBe('/orders');
    expect(detail.transport).toEqual(['https']);
    expect(detail.operations[0]).toMatchObject({
      name: 'list',
      method: 'GET',
      path: '/items',
      policies: [{ name: 'p1', version: '2.0.0' }],
    });
    expect(detail.policies[0]).toMatchObject({
      name: 'cors',
      version: '1.0.0',
    });
    expect(detail.endpoints).toEqual({
      prodUrl: 'https://p',
      sandboxUrl: 'https://s',
    });
    expect(detail.raw).toBe(raw);
  });

  it('defaults an invalid method to GET and missing path to /', () => {
    const detail = restApiToDetail({
      id: 'x',
      operations: [{ request: { method: 'FOO' } }],
    });
    expect(detail.operations[0]).toMatchObject({ method: 'GET', path: '/' });
  });
});

describe('detailToRestApiBody round-trip', () => {
  it('preserves raw fields and replaces operations/policies/upstream', () => {
    const raw = { id: 'orders-api', name: 'Orders', extraServerField: 'keep' };
    const detail: ApiDetail = {
      ...restApiToApi(raw),
      context: '/orders',
      transport: ['https'],
      operations: [
        {
          name: 'list',
          method: 'GET',
          path: '/items',
          policies: [{ name: 'rewrite', version: '2.0.0', params: { New_Path: '/v2' } }],
        },
      ],
      policies: [{ name: 'cors', version: '1.0.0', executionCondition: 'always' }],
      endpoints: { prodUrl: 'https://p', sandboxUrl: 'https://s' },
      raw,
    };
    const body = detailToRestApiBody(detail);
    expect(body.extraServerField).toBe('keep');
    expect(body.operations).toEqual([
      {
        name: 'list',
        description: undefined,
        request: {
          method: 'GET',
          path: '/items',
          policies: [{ name: 'rewrite', version: '2.0.0', params: { New_Path: '/v2' } }],
        },
      },
    ]);
    expect(body.policies).toEqual([
      { name: 'cors', version: '1.0.0', executionCondition: 'always' },
    ]);
    expect(body.upstream).toEqual({
      main: { url: 'https://p' },
      sandbox: { url: 'https://s' },
    });
  });

  it('writes the edited description over the one raw still carries', () => {
    const raw = { id: 'orders-api', description: 'the old one' };
    const detail: ApiDetail = {
      ...restApiToApi(raw),
      description: 'the edited one',
      context: '/orders',
      transport: [],
      operations: [],
      policies: [],
      endpoints: {},
      raw,
    };
    expect(detailToRestApiBody(detail).description).toBe('the edited one');
  });

  it('clears the description with an empty string rather than dropping the key', () => {
    const raw = { id: 'orders-api', description: 'the old one' };
    const detail: ApiDetail = {
      ...restApiToApi(raw),
      description: undefined,
      context: '/orders',
      transport: [],
      operations: [],
      policies: [],
      endpoints: {},
      raw,
    };
    const body = detailToRestApiBody(detail);
    // Omitting the key would leave the server's old value in place, so a
    // cleared description has to be sent explicitly.
    expect(body).toHaveProperty('description', '');
  });

  it('omits the per-operation policies key when there are none', () => {
    const detail: ApiDetail = {
      ...restApiToApi({ id: 'x' }),
      context: '/x',
      transport: [],
      operations: [{ name: 'a', method: 'GET', path: '/', policies: [] }],
      policies: [],
      endpoints: { prodUrl: 'https://p' },
      raw: { id: 'x' },
    };
    const op = (detailToRestApiBody(detail).operations as Array<{ request: object }>)[0];
    expect(op.request).not.toHaveProperty('policies');
  });
});
