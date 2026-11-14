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

import { describe, expect, it, vi } from 'vitest';

import { parseConsoleSpec } from './parseSpec';

const SPEC = {
  openapi: '3.0.1',
  paths: {
    '/books': {
      get: {
        parameters: [
          { in: 'query', name: 'status', schema: { enum: ['to_read', 'read'], type: 'string' } },
          { in: 'query', name: 'limit', schema: { maximum: 100, minimum: 1, type: 'integer' } },
          { in: 'query', name: 'tags', schema: { type: 'array', items: { type: 'string' } } },
          { in: 'cookie', name: 'session', schema: { type: 'string' } },
        ],
        responses: {
          '200': {
            content: { 'application/json': { schema: { $ref: '#/components/schemas/BookList' } } },
            description: 'ok',
          },
        },
        summary: 'List books',
        tags: ['Books'],
      },
      post: {
        requestBody: {
          content: {
            'application/json': { schema: { $ref: '#/components/schemas/NewBook' } },
            'application/x-www-form-urlencoded': {
              schema: {
                properties: { title: { type: 'string' } },
                required: ['title'],
                type: 'object',
              },
            },
          },
          required: true,
        },
        responses: { '201': { description: 'created' } },
        summary: 'Add a book',
      },
    },
    '/books/{id}': {
      parameters: [{ in: 'path', name: 'id', schema: { type: 'string' } }],
      get: { responses: { '404': { description: 'missing' } } },
      put: {
        parameters: [{ in: 'path', name: 'id', required: true, schema: { type: 'integer' } }],
        responses: {},
      },
    },
    '/upload': {
      post: {
        requestBody: {
          content: {
            'multipart/form-data': {
              schema: {
                properties: {
                  file: { format: 'binary', type: 'string' },
                  note: { type: 'string' },
                },
                required: ['file'],
                type: 'object',
              },
            },
          },
        },
        responses: {},
      },
    },
  },
  components: {
    schemas: {
      Book: {
        description: 'A book.',
        properties: {
          id: { format: 'uuid', type: 'string' },
          title: { example: 'Dune', type: 'string' },
        },
        required: ['id'],
        type: 'object',
      },
      BookList: {
        properties: { books: { items: { $ref: '#/components/schemas/Book' }, type: 'array' } },
        type: 'object',
      },
      NewBook: {
        properties: { title: { example: 'Dune', type: 'string' } },
        required: ['title'],
        type: 'object',
      },
    },
  },
};

describe('parseConsoleSpec', () => {
  const model = parseConsoleSpec(SPEC);
  const op = (key: string) => model.operations.find((o) => o.key === key);

  it('lists every operation with its summary and group', () => {
    expect(model.operations.map((o) => o.key)).toEqual([
      'GET /books',
      'POST /books',
      'GET /books/{id}',
      'PUT /books/{id}',
      'POST /upload',
    ]);
    expect(op('GET /books')).toMatchObject({ group: 'Books', summary: 'List books' });
    expect(op('GET /books/{id}')?.group).toBe('/books');
  });

  it('reads parameters, and leaves out cookies the browser cannot set', () => {
    const params = op('GET /books')?.parameters ?? [];
    expect(params.map((p) => p.name)).toEqual(['status', 'limit', 'tags']);
    expect(params[0].enum).toEqual(['to_read', 'read']);
    expect(params[1]).toMatchObject({ maximum: 100, minimum: 1, type: 'integer' });
    expect(params[2].type).toBe('array');
  });

  it('treats a path parameter as required and lets the operation override the path level', () => {
    expect(op('GET /books/{id}')?.parameters[0]).toMatchObject({
      in: 'path',
      required: true,
      type: 'string',
    });
    expect(op('PUT /books/{id}')?.parameters).toHaveLength(1);
    expect(op('PUT /books/{id}')?.parameters[0].type).toBe('integer');
  });

  it('offers each request media type, with an example built from the schema', () => {
    const body = op('POST /books')?.body;
    expect(body?.required).toBe(true);
    expect(body?.options.map((o) => o.kind)).toEqual(['json', 'urlencoded']);
    expect(JSON.parse(body?.options[0].example ?? '')).toEqual({ title: 'Dune' });
    expect(body?.options[0].schemaName).toBe('NewBook');
    expect(body?.options[1].formFields[0]).toMatchObject({ name: 'title', required: true });
  });

  it('marks binary form fields as file inputs', () => {
    const fields = op('POST /upload')?.body?.options[0].formFields ?? [];
    expect(fields.find((f) => f.name === 'file')).toMatchObject({ binary: true, required: true });
    expect(fields.find((f) => f.name === 'note')?.binary).toBe(false);
  });

  it('documents the responses and the schemas', () => {
    expect(op('GET /books')?.responses[0]).toMatchObject({ schemaName: 'BookList', status: '200' });
    expect(model.schemas.map((s) => s.name)).toEqual(['Book', 'BookList', 'NewBook']);
    expect(model.schemas[0].fields[0]).toMatchObject({
      name: 'id',
      required: true,
      type: 'string (uuid)',
    });
  });

  it('does not follow a remote $ref', () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal('fetch', fetchSpy);
    const remote = parseConsoleSpec({
      paths: {
        '/x': {
          post: {
            requestBody: {
              content: {
                'application/json': { schema: { $ref: 'https://evil.example.com/schema.json' } },
              },
            },
            responses: {},
          },
        },
      },
    });
    // Read as an unknown schema (a placeholder); the URL in the document is never contacted.
    expect(remote.operations[0].body?.options[0].example).not.toContain('evil');
    expect(fetchSpy).not.toHaveBeenCalled();
    vi.unstubAllGlobals();
  });

  it('survives a self-referencing $ref', () => {
    const loop = parseConsoleSpec({
      paths: {
        '/x': { get: { parameters: [{ $ref: '#/components/parameters/Loop' }], responses: {} } },
      },
      components: { parameters: { Loop: { $ref: '#/components/parameters/Loop' } } },
    });
    expect(loop.operations[0].parameters).toEqual([]);
  });

  it('reads a Swagger 2.0 body parameter', () => {
    const v2 = parseConsoleSpec({
      swagger: '2.0',
      paths: {
        '/pets': {
          post: {
            consumes: ['application/json'],
            parameters: [
              {
                in: 'body',
                name: 'pet',
                required: true,
                schema: { properties: { name: { type: 'string' } }, type: 'object' },
              },
            ],
            responses: {},
          },
        },
      },
    });
    expect(v2.operations[0].body?.options[0]).toMatchObject({
      kind: 'json',
      mediaType: 'application/json',
    });
  });

  it('returns nothing for a document with no paths, rather than throwing', () => {
    expect(parseConsoleSpec({})).toEqual({ operations: [], schemas: [] });
  });
});
