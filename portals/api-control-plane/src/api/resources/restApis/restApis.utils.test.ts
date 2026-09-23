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

import {
  parseSpecContent,
  serializeSpecContent,
  serverUrlOf,
  specVersionOf,
  toRestApiDefinition,
  type OpenApiDocument,
} from './restApis.utils';

/**
 * What an API's stored OpenApi text means.
 *
 * Pure throughout, no MSW, no transport. These are the derivations every
 * consumer relies on, and the ones that survive unchanged when the definition
 * endpoint stops being mock-backed.
 */

describe('parseSpecContent', () => {
  it('parses YAML, the format the endpoint stores', () => {
    expect(parseSpecContent('openapi: 3.0.1\npaths: {}\n')).toEqual({
      openapi: '3.0.1',
      paths: {},
    });
  });

  it('parses a JSON document too, since YAML is a superset of it', () => {
    // No JSON branch is needed, and this is why: a spec that happens to be JSON
    // still comes back correctly through the single YAML call.
    expect(parseSpecContent('{"openapi":"3.0.1","paths":{}}')).toEqual({
      openapi: '3.0.1',
      paths: {},
    });
  });

  it('tolerates surrounding whitespace', () => {
    expect(parseSpecContent('\n  openapi: 3.0.1\n  ')).toEqual({ openapi: '3.0.1' });
  });

  it('throws on an empty document', () => {
    expect(() => parseSpecContent('')).toThrow(/empty/i);
    expect(() => parseSpecContent('   \n ')).toThrow(/empty/i);
  });

  it('throws on malformed YAML rather than returning a partial result', () => {
    // A caller cannot render half a document; the query surfaces the failure.
    expect(() => parseSpecContent('openapi: 3.0.1\n  bad: [indent')).toThrow();
  });

  it('throws when the document is not an object', () => {
    expect(() => parseSpecContent('[1, 2, 3]')).toThrow(/not an OpenAPI document/i);
    expect(() => parseSpecContent('42')).toThrow(/not an OpenAPI document/i);
    expect(() => parseSpecContent('"just a string"')).toThrow(/not an OpenAPI document/i);
    expect(() => parseSpecContent('null')).toThrow(/not an OpenAPI document/i);
  });
});

describe('serializeSpecContent', () => {
  it('round-trips a document through YAML unchanged', () => {
    const spec: OpenApiDocument = {
      openapi: '3.0.1',
      paths: { '/books': { get: { operationId: 'listBooks', responses: { '200': {} } } } },
      servers: [{ url: 'https://api.example.com/v1' }],
    };

    // The property the sample relies on: what it serialises is what the parse
    // path gets back, so the mock cannot drift from the real endpoint.
    expect(parseSpecContent(serializeSpecContent(spec))).toEqual(spec);
  });

  it('keeps a long scalar on one line', () => {
    // Folding parses back identically but reads as damage in a source view.
    const description = 'a '.repeat(120).trim();

    expect(serializeSpecContent({ info: { description } })).toContain(description);
  });
});

describe('serverUrlOf', () => {
  it('reads the first OpenAPI 3 server', () => {
    const spec: OpenApiDocument = {
      servers: [{ url: 'https://api.example.com/v1' }, { url: 'https://staging.example.com' }],
    };

    expect(serverUrlOf(spec)).toBe('https://api.example.com/v1');
  });

  it('assembles a Swagger 2 base URL from scheme, host and basePath', () => {
    expect(
      serverUrlOf({ swagger: '2.0', schemes: ['https'], host: 'api.example.com', basePath: '/v1' }),
    ).toBe('https://api.example.com/v1');
  });

  it('defaults a Swagger 2 document with no schemes to https', () => {
    // `schemes` is optional and frequently omitted; guessing http would send a
    // try-out request in the clear.
    expect(serverUrlOf({ host: 'api.example.com', basePath: '/v1' })).toBe(
      'https://api.example.com/v1',
    );
  });

  it('does not leave a trailing slash for a root basePath', () => {
    // "/" means "no prefix"; keeping it would double the slash once a path is
    // joined on, and some gateways route "//books" differently.
    expect(serverUrlOf({ host: 'api.example.com', basePath: '/' })).toBe('https://api.example.com');
  });

  it('returns a relative OpenAPI 3 server unchanged', () => {
    // Resolving it against an origin is the caller's decision, not this layer's.
    expect(serverUrlOf({ servers: [{ url: '/api/v3' }] })).toBe('/api/v3');
  });

  it('is undefined when the document declares no usable server', () => {
    expect(serverUrlOf({ openapi: '3.0.1', paths: {} })).toBeUndefined();
    expect(serverUrlOf({ servers: [] })).toBeUndefined();
    expect(serverUrlOf({ servers: [{ description: 'no url here' }] })).toBeUndefined();
  });
});

describe('specVersionOf', () => {
  it('reads the OpenAPI 3 version field', () => {
    expect(specVersionOf({ openapi: '3.0.1' })).toBe('3.0.1');
  });

  it('falls back to the Swagger 2 version field', () => {
    expect(specVersionOf({ swagger: '2.0' })).toBe('2.0');
  });

  it('reports unknown rather than undefined when neither field is present', () => {
    // Consumers render this; `undefined` would print as "undefined".
    expect(specVersionOf({})).toBe('unknown');
    expect(specVersionOf({ openapi: '   ' })).toBe('unknown');
  });
});

describe('toRestApiDefinition', () => {
  it('derives the version and server url once, so consumers do not re-derive them', () => {
    const spec: OpenApiDocument = {
      openapi: '3.0.1',
      servers: [{ url: 'https://api.example.com/v1' }],
    };

    expect(toRestApiDefinition(spec)).toEqual({
      spec,
      specVersion: '3.0.1',
      serverUrl: 'https://api.example.com/v1',
    });
  });

  it('passes the document through by reference, unmodified', () => {
    // The spec viewer receives this object; rewriting it here would make the
    // console render something the platform never returned.
    const spec: OpenApiDocument = { openapi: '3.0.1' };

    expect(toRestApiDefinition(spec).spec).toBe(spec);
  });
});
