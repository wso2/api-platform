import { http, HttpResponse } from 'msw';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { server } from '../../test/server';
import { buildPolicyQuery } from './policyHubClient';

const HUB = 'http://hub.test';

// policyHubBaseUrl is read from runtimeConfig at module load.
async function loadClient() {
  vi.stubEnv('VITE_POLICY_HUB_BASE_URL', HUB);
  vi.resetModules();
  return import('./policyHubClient');
}

describe('buildPolicyQuery', () => {
  it('converts 1-based page to offset/limit', () => {
    expect(buildPolicyQuery(1, 20)).toBe('offset=0&limit=20');
    expect(buildPolicyQuery(3, 20)).toBe('offset=40&limit=20');
  });

  it('appends categories when present', () => {
    expect(buildPolicyQuery(1, 10, ['security', 'transform'])).toBe(
      'offset=0&limit=10&categories=security%2Ctransform'
    );
  });

  it('omits categories when empty', () => {
    expect(buildPolicyQuery(2, 10, [])).toBe('offset=10&limit=10');
  });
});

describe('policyHubClient (MSW fetch)', () => {
  afterEach(() => vi.unstubAllEnvs());

  it('listPolicies maps the catalog and prefers pagination.total', async () => {
    const { listPolicies } = await loadClient();
    server.use(
      http.get(`${HUB}/policies`, () =>
        HttpResponse.json({
          data: [{ name: 'p1', version: '1.0.0', categories: ['transform'] }],
          count: 1,
          pagination: { total: 42 },
        })
      )
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
    const { listPolicyCategories } = await loadClient();
    server.use(
      http.get(`${HUB}/policies/categories`, () =>
        HttpResponse.json({ data: ['security', 'transform'] })
      )
    );
    await expect(listPolicyCategories()).resolves.toEqual([
      'security',
      'transform',
    ]);
  });

  it('listPolicyVersions accepts a bare array response', async () => {
    const { listPolicyVersions } = await loadClient();
    server.use(
      http.get(`${HUB}/policies/p1/versions`, () =>
        HttpResponse.json([{ name: 'p1', version: '2.0.0' }])
      )
    );
    const versions = await listPolicyVersions('p1');
    expect(versions).toHaveLength(1);
    expect(versions[0].version).toBe('2.0.0');
  });

  it('getPolicyDefinition parses the YAML definition into a schema', async () => {
    const { getPolicyDefinition } = await loadClient();
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
      http.get(`${HUB}/policies/rewrite/versions/2.0.0/definition`, () =>
        HttpResponse.text(yaml)
      )
    );
    const def = await getPolicyDefinition('rewrite', '2.0.0');
    expect(def).toMatchObject({ name: 'rewrite', version: '2.0.0' });
    expect(def.schema.type).toBe('object');
    expect(def.schema.properties?.['New Path']).toMatchObject({ type: 'string' });
  });

  it('throws a SERVER_ERROR ApiError on a 500', async () => {
    const { listPolicies } = await loadClient();
    server.use(
      http.get(`${HUB}/policies`, () => new HttpResponse(null, { status: 500 }))
    );
    await expect(listPolicies(1, 20)).rejects.toMatchObject({
      code: 'SERVER_ERROR',
      status: 500,
    });
  });
});
