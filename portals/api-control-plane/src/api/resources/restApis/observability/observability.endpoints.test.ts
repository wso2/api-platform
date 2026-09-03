/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { beforeEach, describe, expect, it } from 'vitest';

import { resetHttpClient } from '../../../core/http';
import { recorder, resource, type Recorder } from '../../../../test/msw';
import { server } from '../../../../test/server';
import {
  getRestApiObservabilityTrace,
  listObservabilityLogs,
  listRestApiObservabilityLogs,
  listRestApiObservabilityTraces,
} from './observability.endpoints';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('REST API observability traces', () => {
  it('queries trace summaries and spans within the selected environment', async () => {
    server.use(
      resource(
        '/rest-apis/:restApiId/observability/traces',
        { items: [{ traceId: 'trace-1' }], pagination: { limit: 100 } },
        { record: requests }
      ),
      resource(
        '/rest-apis/:restApiId/observability/traces/:traceId',
        { spans: [{ spanId: 'span-1' }], total: 1 },
        { record: requests }
      )
    );

    await listRestApiObservabilityTraces('orders/v1', {
      startTime: '2026-08-22T09:00:00Z',
      endTime: '2026-08-22T10:00:00Z',
      environment: 'stage',
    });
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/orders%2Fv1/observability/traces'
    );
    expect(requests.last()?.params.get('environment')).toBe('stage');

    await getRestApiObservabilityTrace(
      'orders/v1',
      'trace/1',
      {
        startTime: '2026-08-22T09:00:00Z',
        endTime: '2026-08-22T10:00:00Z',
        environment: 'stage',
      }
    );
    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/rest-apis/orders%2Fv1/observability/traces/trace%2F1'
    );
  });
});

describe('listObservabilityLogs', () => {
  it('queries project logs with component and environment filters', async () => {
    server.use(
      resource(
        '/projects/:projectId/observability/logs',
        { items: [], pagination: { limit: 100, nextCursor: null } },
        { record: requests }
      )
    );

    await listObservabilityLogs(
      { projectId: 'retail/eu' },
      {
        startTime: '2026-08-22T09:00:00Z',
        endTime: '2026-08-22T10:00:00Z',
        component: 'orders',
        environment: 'stage',
        project: 'retail/eu',
      },
      { orgId: 'acme' }
    );

    const request = requests.last();
    expect(request?.url.pathname).toBe(
      '/api/v0.9/projects/retail%2Feu/observability/logs'
    );
    expect(request?.params.get('component')).toBe('orders');
    expect(request?.params.get('environment')).toBe('stage');
    expect(request?.params.get('project')).toBe('retail/eu');
  });

  it('uses the organization endpoint when no deeper scope is selected', async () => {
    server.use(
      resource(
        '/observability/logs',
        { items: [], pagination: { limit: 100, nextCursor: null } },
        { record: requests }
      )
    );

    await listObservabilityLogs(
      {},
      {
        startTime: '2026-08-22T09:00:00Z',
        endTime: '2026-08-22T10:00:00Z',
        environment: 'development',
      },
      { orgId: 'acme' }
    );

    expect(requests.last()?.url.pathname).toBe(
      '/api/v0.9/observability/logs'
    );
  });
});

describe('listRestApiObservabilityLogs', () => {
  it('queries one API with bounded filters and organization scope', async () => {
    server.use(
      resource(
        '/rest-apis/:restApiId/observability/logs',
        {
          items: [{ timestamp: '2026-08-22T10:00:00Z', level: 'INFO' }],
          pagination: { limit: 50, nextCursor: null },
        },
        { record: requests }
      )
    );

    const result = await listRestApiObservabilityLogs(
      'orders/v1',
      {
        startTime: '2026-08-22T09:00:00Z',
        endTime: '2026-08-22T10:00:00Z',
        limit: 50,
        query: ' checkout ',
        logLevels: ['ERROR', 'WARN'],
      },
      { orgId: 'acme' }
    );

    const request = requests.last();
    expect(request?.url.pathname).toBe(
      '/api/v0.9/rest-apis/orders%2Fv1/observability/logs'
    );
    expect(request?.headers.get('X-Org-Id')).toBe('acme');
    expect(request?.params.get('startTime')).toBe('2026-08-22T09:00:00Z');
    expect(request?.params.get('endTime')).toBe('2026-08-22T10:00:00Z');
    expect(request?.params.get('limit')).toBe('50');
    expect(request?.params.get('query')).toBe('checkout');
    expect(request?.params.getAll('logLevel')).toEqual(['ERROR', 'WARN']);
    expect(result.items).toHaveLength(1);
  });

  it('uses the server default limit and omits an empty search filter', async () => {
    server.use(
      resource('/rest-apis/:restApiId/observability/logs', {
        items: [],
        pagination: { limit: 100, nextCursor: null },
      }, { record: requests })
    );

    await listRestApiObservabilityLogs('orders', {
      startTime: '2026-08-22T09:00:00Z',
      endTime: '2026-08-22T10:00:00Z',
      query: '   ',
    });

    expect(requests.last()?.params.get('limit')).toBe('100');
    expect(requests.last()?.params.has('query')).toBe(false);
  });
});
