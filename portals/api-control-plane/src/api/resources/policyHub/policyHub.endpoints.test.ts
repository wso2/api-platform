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

import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { server } from '@/test/server';
import { buildPolicyQuery } from './policyHub.endpoints';

const HUB = 'http://hub.test';

/**
 * `policyHubBaseUrl` is read from `runtimeConfig` at module load, so pointing
 * the endpoints at a test hub means re-importing the module with the env var
 * stubbed. `buildPolicyQuery` is pure and can be imported statically.
 */
async function loadEndpoints() {
  vi.stubEnv('VITE_POLICY_HUB_BASE_URL', HUB);
  vi.resetModules();
  return import('./policyHub.endpoints');
}

describe('buildPolicyQuery', () => {
  it('converts 1-based page to offset/limit', () => {
    expect(buildPolicyQuery(1, 20)).toBe('offset=0&limit=20');
    expect(buildPolicyQuery(3, 20)).toBe('offset=40&limit=20');
  });

  it('appends categories when present', () => {
    expect(buildPolicyQuery(1, 10, ['security', 'transform'])).toBe(
      'offset=0&limit=10&categories=security%2Ctransform',
    );
  });

  it('omits categories when empty', () => {
    expect(buildPolicyQuery(2, 10, [])).toBe('offset=10&limit=10');
  });
});

describe('isPolicyHubConfigured', () => {
  afterEach(() => vi.unstubAllEnvs());

  it('is true once a base URL is configured', async () => {
    const { isPolicyHubConfigured } = await loadEndpoints();
    expect(isPolicyHubConfigured()).toBe(true);
  });

  it('is false when no base URL is configured', async () => {
    vi.stubEnv('VITE_POLICY_HUB_BASE_URL', '');
    vi.resetModules();
    const { isPolicyHubConfigured } = await import('./policyHub.endpoints');
    expect(isPolicyHubConfigured()).toBe(false);
  });
});

describe('policyHub endpoints (MSW fetch)', () => {
  afterEach(() => vi.unstubAllEnvs());

  it('listPolicies maps the catalog and prefers pagination.total', async () => {
    const { listPolicies } = await loadEndpoints();
    server.use(
      http.get(`${HUB}/policies`, () =>
        HttpResponse.json({
          data: [{ name: 'p1', version: '1.0.0', categories: ['transform'] }],
          count: 1,
          pagination: { total: 42 },
        }),
      ),
    );
    const result = await listPolicies(1, 20);
    expect(result.total).toBe(42);
    expect(result.policies[0]).toMatchObject({
      name: 'p1',
      displayName: 'p1',
      categories: ['transform'],
    });
  });

  it('listPolicyCategories returns the data array', async () => {
    const { listPolicyCategories } = await loadEndpoints();
    server.use(
      http.get(`${HUB}/policies/categories`, () =>
        HttpResponse.json({ data: ['security', 'transform'] }),
      ),
    );
    await expect(listPolicyCategories()).resolves.toEqual(['security', 'transform']);
  });

  it('listPolicyVersions accepts a bare array response', async () => {
    const { listPolicyVersions } = await loadEndpoints();
    server.use(
      http.get(`${HUB}/policies/p1/versions`, () =>
        HttpResponse.json([{ name: 'p1', version: '2.0.0' }]),
      ),
    );
    const versions = await listPolicyVersions('p1');
    expect(versions).toHaveLength(1);
    expect(versions[0].version).toBe('2.0.0');
  });

  it('getPolicyDefinition parses the YAML definition into a schema', async () => {
    const { getPolicyDefinition } = await loadEndpoints();
    const yaml = [
      'name: rewrite',
      'version: 2.0.0',
      'description: Rewrite the path',
      'parameters:',
      '  type: object',
      '  properties:',
      '    New Path:',
      '      type: string',
    ].join('\n');
    server.use(
      http.get(`${HUB}/policies/rewrite/versions/2.0.0/definition`, () => HttpResponse.text(yaml)),
    );
    const definition = await getPolicyDefinition('rewrite', '2.0.0');
    expect(definition).toMatchObject({ name: 'rewrite', version: '2.0.0' });
    expect(definition.schema.type).toBe('object');
    expect(definition.schema.properties?.['New Path']).toMatchObject({ type: 'string' });
  });

  it('rejects with an http-kind ApiError carrying the status on a 500', async () => {
    const { listPolicies } = await loadEndpoints();
    server.use(http.get(`${HUB}/policies`, () => new HttpResponse(null, { status: 500 })));
    await expect(listPolicies(1, 20)).rejects.toMatchObject({
      name: 'ApiError',
      kind: 'http',
      status: 500,
    });
  });

  it('does not leak the hub response body into the error message', async () => {
    const { listPolicies } = await loadEndpoints();
    server.use(
      http.get(`${HUB}/policies`, () =>
        HttpResponse.json({ error: 'upstream policy-hub-internal.svc refused' }, { status: 502 }),
      ),
    );
    // `.claude/rules/error-handling.md`: a rejection must not carry the
    // upstream's own wording, which can name internal hosts.
    await expect(listPolicies(1, 20)).rejects.toMatchObject({
      message: 'The Policy Hub request could not be completed.',
    });
  });

  it('classifies an aborted request as `aborted`, not a network failure', async () => {
    const { listPolicies } = await loadEndpoints();
    server.use(http.get(`${HUB}/policies`, () => HttpResponse.json({ data: [] })));

    const controller = new AbortController();
    controller.abort();

    // React Query cancels superseded queries this way; it must not surface as
    // an error the user could see.
    await expect(listPolicies(1, 20, [], { signal: controller.signal })).rejects.toMatchObject({
      kind: 'aborted',
      code: 'CLIENT_REQUEST_ABORTED',
    });
  });
});
