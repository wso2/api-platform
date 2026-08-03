import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';

import { server } from '../../../test/server';
import {
  contractCandidates,
  discoverBackendResources,
  parseContractResources,
} from './backendDiscovery';

describe('contractCandidates', () => {
  it('derives the conventional contract URLs from a base URL', () => {
    expect(contractCandidates('https://api.example.com/')).toEqual([
      'https://api.example.com/openapi.json',
      'https://api.example.com/openapi.yaml',
      'https://api.example.com/swagger.json',
      'https://api.example.com/v3/api-docs',
      'https://api.example.com/api-docs',
    ]);
  });

  it('uses the URL directly when it already points at a contract document', () => {
    expect(contractCandidates('https://api.example.com/spec.yaml')).toEqual([
      'https://api.example.com/spec.yaml',
    ]);
    expect(contractCandidates('https://api.example.com/openapi.json')).toEqual([
      'https://api.example.com/openapi.json',
    ]);
  });

  it('also probes the host root when the base has a sub-path', () => {
    expect(contractCandidates('https://api.example.com/v2')).toContain(
      'https://api.example.com/openapi.json'
    );
  });

  it('returns nothing for an empty URL', () => {
    expect(contractCandidates('   ')).toEqual([]);
  });
});

describe('parseContractResources', () => {
  it('reads OpenAPI v3 JSON paths × methods', () => {
    const doc = JSON.stringify({
      openapi: '3.0.0',
      paths: {
        '/orders': { get: {}, post: {} },
        '/orders/{id}': { get: {}, delete: {} },
      },
    });
    expect(parseContractResources(doc)).toEqual([
      { method: 'GET', path: '/orders' },
      { method: 'POST', path: '/orders' },
      { method: 'GET', path: '/orders/{id}' },
      { method: 'DELETE', path: '/orders/{id}' },
    ]);
  });

  it('reads Swagger v2 and YAML documents', () => {
    const yamlDoc = [
      'swagger: "2.0"',
      'paths:',
      '  /items:',
      '    get: {}',
      '    put: {}',
    ].join('\n');
    expect(parseContractResources(yamlDoc)).toEqual([
      { method: 'GET', path: '/items' },
      { method: 'PUT', path: '/items' },
    ]);
  });

  it('ignores non-method keys on a path item (parameters, $ref)', () => {
    const doc = JSON.stringify({
      paths: { '/x': { get: {}, parameters: [], $ref: '#/foo' } },
    });
    expect(parseContractResources(doc)).toEqual([{ method: 'GET', path: '/x' }]);
  });

  it('returns [] for documents without a paths object or garbage', () => {
    expect(parseContractResources('{}')).toEqual([]);
    expect(parseContractResources('not json or yaml: [unclosed')).toEqual([]);
  });
});

describe('discoverBackendResources', () => {
  const BASE = 'http://backend.test';

  it('returns the first candidate that resolves to resources', async () => {
    server.use(
      http.get(`${BASE}/openapi.json`, () =>
        HttpResponse.json({ message: 'nope' }, { status: 404 })
      ),
      http.get(`${BASE}/openapi.yaml`, () =>
        HttpResponse.text('paths:\n  /orders:\n    get: {}')
      )
    );
    expect(await discoverBackendResources(BASE)).toEqual([
      { method: 'GET', path: '/orders' },
    ]);
  });

  it('returns [] when a contract is reachable but declares no resources', async () => {
    server.use(
      http.get(`${BASE}/openapi.json`, () =>
        HttpResponse.json({ openapi: '3.0.0', paths: {} })
      ),
      http.get(`${BASE}/openapi.yaml`, () => new HttpResponse(null, { status: 404 })),
      http.get(`${BASE}/swagger.json`, () => new HttpResponse(null, { status: 404 })),
      http.get(`${BASE}/v3/api-docs`, () => new HttpResponse(null, { status: 404 })),
      http.get(`${BASE}/api-docs`, () => new HttpResponse(null, { status: 404 }))
    );
    expect(await discoverBackendResources(BASE)).toEqual([]);
  });

  it('throws when every candidate fails to fetch (CORS / network)', async () => {
    for (const path of [
      'openapi.json',
      'openapi.yaml',
      'swagger.json',
      'v3/api-docs',
      'api-docs',
    ]) {
      server.use(http.get(`${BASE}/${path}`, () => HttpResponse.error()));
    }
    await expect(discoverBackendResources(BASE)).rejects.toThrow();
  });
});
