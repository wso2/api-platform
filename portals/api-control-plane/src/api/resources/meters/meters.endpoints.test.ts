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

import { beforeEach, describe, expect, it } from 'vitest';

import {
  aMeter,
  accepts,
  collection,
  failure,
  noContent,
  recorder,
  resource,
  type Recorder,
} from '../../../test/msw';
import { server } from '../../../test/server';
import { ApiError } from '../../core/errors';
import { resetHttpClient } from '../../core/http';
import { createMeter, deleteMeter, getMeter, listMeters, updateMeter } from './meters.endpoints';

/**
 * Contract tests for `/meters`: URL and method pinning, and that the server's
 * reason for refusing a change or a delete reaches the caller as it is.
 */

const RESOURCE = '/meters/successful-calls';

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('meters', () => {
  it('GETs the collection, scoped to the organization', async () => {
    server.use(collection('/meters', [aMeter()], { record: requests }));

    await listMeters({ orgId: 'acme-org', query: { limit: 100 } });

    expect(requests.last()?.url.pathname).toBe('/api/v0.9/meters');
    expect(requests.last()?.headers.get('X-Org-Id')).toBe('acme-org');
    expect(requests.last()?.params.get('limit')).toBe('100');
  });

  it('GETs, PUTs and DELETEs one meter by id', async () => {
    server.use(
      resource(RESOURCE, aMeter(), { record: requests }),
      accepts('put', RESOURCE, aMeter(), { record: requests }),
      noContent('delete', RESOURCE, { record: requests }),
    );

    await getMeter('successful-calls');
    await updateMeter('successful-calls', { displayName: 'Calls', type: 'REQUEST_COUNT' });
    await expect(deleteMeter('successful-calls')).resolves.toBeUndefined();

    expect(requests.calls.map((r) => [r.method, r.url.pathname])).toEqual([
      ['GET', '/api/v0.9/meters/successful-calls'],
      ['PUT', '/api/v0.9/meters/successful-calls'],
      ['DELETE', '/api/v0.9/meters/successful-calls'],
    ]);
  });

  it('POSTs a new meter with its type, event key and filter properties with their keys, and no conditions', async () => {
    server.use(accepts('post', '/meters', aMeter(), { record: requests }));
    const body = {
      displayName: 'LLM input tokens',
      filterFields: [
        { eventKey: 'metadata.apiType', field: 'API type' },
        { eventKey: 'metadata.region', field: 'Region' },
      ],
      type: 'TOKENS' as const,
      typeKey: 'metadata.aiTokenUsage.promptTokens',
    };

    await createMeter(body);

    expect(JSON.parse(requests.last()!.body)).toEqual(body);
  });

  it('surfaces the reason for refusing a delete', async () => {
    server.use(
      failure('delete', RESOURCE, 409, 'METER_IN_USE', {
        message: "The meter is in use, so it can't be deleted.",
      }),
    );

    const error = (await deleteMeter('successful-calls').catch((e: unknown) => e)) as ApiError;

    expect(error.code).toBe('METER_IN_USE');
    expect(error.message).toBe("The meter is in use, so it can't be deleted.");
    expect(error.operation).toBe('DeleteMeter');
  });

  it('surfaces the reason for refusing to change a meter a plan prices', async () => {
    const message = "The meter is in use, so it can't be updated.";
    server.use(failure('put', RESOURCE, 409, 'METER_IN_USE', { message }));

    const error = (await updateMeter('successful-calls', {
      displayName: 'Calls',
      filterFields: [],
      type: 'REQUEST_COUNT',
    }).catch((e: unknown) => e)) as ApiError;

    expect(error.message).toBe(message);
    expect(error.operation).toBe('UpdateMeter');
  });
});
