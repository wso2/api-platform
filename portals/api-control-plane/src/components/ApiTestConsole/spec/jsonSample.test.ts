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

import { requestBodySample, resolveRef, sampleFromSchema } from './jsonSample';
import readingList from './readingListApi.fixture.json';

const spec = readingList as unknown as Record<string, unknown>;

describe('resolveRef', () => {
  it('follows a local component reference', () => {
    expect(resolveRef(spec, '#/components/schemas/models.Book')).toMatchObject({ type: 'object' });
  });

  it('unescapes JSON Pointer sequences', () => {
    const pointered = { components: { 'a/b': { 'c~d': { type: 'string' } } } };

    expect(resolveRef(pointered, '#/components/a~1b/c~0d')).toEqual({ type: 'string' });
  });

  it('refuses a remote reference', () => {
    // Following one would mean fetching a URL out of an untrusted definition —
    // the request-forgery shape the portal's SSRF rule forbids.
    expect(resolveRef(spec, 'https://evil.example.com/schema.json')).toBeUndefined();
    expect(resolveRef(spec, 'other.yaml#/components/schemas/Book')).toBeUndefined();
  });

  it('is undefined for a reference that goes nowhere', () => {
    expect(resolveRef(spec, '#/components/schemas/Nope')).toBeUndefined();
  });
});

describe('sampleFromSchema', () => {
  it('prefers the author’s own example over anything inferred', () => {
    expect(sampleFromSchema(spec, { type: 'string', example: 'The Hobbit' })).toBe('The Hobbit');
  });

  it('falls back to default, then to the first enum member', () => {
    expect(sampleFromSchema(spec, { type: 'string', default: 'd' })).toBe('d');
    expect(sampleFromSchema(spec, { type: 'string', enum: ['read', 'to_read'] })).toBe('read');
  });

  it('builds an object from its properties', () => {
    const sample = sampleFromSchema(spec, {
      type: 'object',
      properties: { a: { type: 'string', example: 'x' }, b: { type: 'integer', example: 7 } },
    });

    expect(sample).toEqual({ a: 'x', b: 7 });
  });

  // `readOnly` means "may come back in a response, should not be sent in a
  // request". This sample is a request body, so those properties have no place
  // in it — and a body-validating gateway can reject one that carries them.
  describe('response-only properties', () => {
    it('omits a readOnly property written inline', () => {
      const sample = sampleFromSchema(spec, {
        type: 'object',
        properties: {
          id: { type: 'string', readOnly: true, example: 'srv-generated' },
          title: { type: 'string', example: 'The Hobbit' },
        },
      });

      expect(sample).toEqual({ title: 'The Hobbit' });
    });

    it('omits one whose readOnly sits behind a $ref', () => {
      const refSpec = {
        components: { schemas: { Id: { type: 'string', readOnly: true } } },
      };
      const sample = sampleFromSchema(refSpec, {
        type: 'object',
        properties: { id: { $ref: '#/components/schemas/Id' }, title: { type: 'string' } },
      });

      expect(sample).toEqual({ title: 'string' });
    });

    it('omits one composed in through allOf', () => {
      const sample = sampleFromSchema(spec, {
        type: 'object',
        properties: {
          id: { allOf: [{ type: 'string' }, { readOnly: true }] },
          title: { type: 'string' },
        },
      });

      expect(sample).toEqual({ title: 'string' });
    });

    it('omits one nested inside another object', () => {
      const sample = sampleFromSchema(spec, {
        type: 'object',
        properties: {
          book: {
            type: 'object',
            properties: {
              id: { type: 'string', readOnly: true },
              title: { type: 'string', example: 'x' },
            },
          },
        },
      });

      expect(sample).toEqual({ book: { title: 'x' } });
    });

    it('omits one inside an array’s items', () => {
      const sample = sampleFromSchema(spec, {
        type: 'array',
        items: {
          type: 'object',
          properties: { id: { type: 'string', readOnly: true }, title: { type: 'string' } },
        },
      });

      expect(sample).toEqual([{ title: 'string' }]);
    });

    it('keeps writeOnly and an explicit readOnly: false', () => {
      // writeOnly is the opposite flag — request-only — so filtering it out
      // would strip exactly the properties this sample exists to show.
      const sample = sampleFromSchema(spec, {
        type: 'object',
        properties: {
          password: { type: 'string', writeOnly: true, example: 'hunter2' },
          title: { type: 'string', readOnly: false, example: 'x' },
        },
      });

      expect(sample).toEqual({ password: 'hunter2', title: 'x' });
    });

    it('yields an empty object when every property is readOnly', () => {
      const sample = sampleFromSchema(spec, {
        type: 'object',
        properties: { id: { type: 'string', readOnly: true } },
      });

      expect(sample).toEqual({});
    });
  });

  it('infers object-ness from properties alone, with no type field', () => {
    expect(sampleFromSchema(spec, { properties: { a: { type: 'boolean' } } })).toEqual({ a: true });
  });

  it('emits one array element rather than an empty array', () => {
    // An empty array tells the user nothing about the shape to fill in.
    expect(
      sampleFromSchema(spec, { type: 'array', items: { type: 'string', example: 'x' } }),
    ).toEqual(['x']);
  });

  it('shapes a placeholder by type and format', () => {
    expect(sampleFromSchema(spec, { type: 'integer' })).toBe(0);
    expect(sampleFromSchema(spec, { type: 'boolean' })).toBe(true);
    expect(sampleFromSchema(spec, { type: 'string' })).toBe('string');
    expect(sampleFromSchema(spec, { type: 'string', format: 'date' })).toBe('1970-01-01');
    expect(sampleFromSchema(spec, { type: 'string', format: 'date-time' })).toContain(
      '1970-01-01T',
    );
  });

  it('follows a $ref', () => {
    const sample = sampleFromSchema(spec, { $ref: '#/components/schemas/models.BookUpdate' });

    expect(sample).toEqual({ status: 'reading' });
  });

  it('merges an allOf chain into one object', () => {
    const sample = sampleFromSchema(spec, {
      allOf: [
        { type: 'object', properties: { a: { type: 'string', example: 'x' } } },
        { type: 'object', properties: { b: { type: 'string', example: 'y' } } },
      ],
    });

    expect(sample).toEqual({ a: 'x', b: 'y' });
  });

  // An `allOf` chain that composes a scalar carries no properties, and saying
  // so matters: an empty `properties` object still reads as "this is an object"
  // to the branch below, which would answer `{}` for a schema whose whole point
  // is a formatted string.
  describe('an allOf chain that composes a scalar', () => {
    it('keeps the format the chain contributes', () => {
      const sample = sampleFromSchema(spec, {
        allOf: [{ type: 'string' }, { format: 'date' }],
      });

      expect(sample).toBe('1970-01-01');
    });

    it('keeps a date-time the same way', () => {
      const sample = sampleFromSchema(spec, {
        allOf: [{ type: 'string' }, { format: 'date-time' }],
      });

      expect(sample).toBe(new Date(0).toISOString());
    });

    it('keeps a non-string scalar', () => {
      // The member adding a constraint contributes no properties either.
      expect(sampleFromSchema(spec, { allOf: [{ type: 'integer' }, { minimum: 1 }] })).toBe(0);
    });

    it('still merges properties when the chain composes an object', () => {
      // The empty-properties case must not cost the case this reducer is for.
      const sample = sampleFromSchema(spec, {
        allOf: [
          { type: 'object', properties: { a: { type: 'string', example: 'x' } } },
          { type: 'object', properties: { b: { type: 'integer', example: 7 } } },
        ],
      });

      expect(sample).toEqual({ a: 'x', b: 7 });
    });
  });

  it('takes the first branch of a oneOf rather than emitting nothing', () => {
    const sample = sampleFromSchema(spec, {
      oneOf: [{ type: 'string', example: 'first' }, { type: 'integer' }],
    });

    expect(sample).toBe('first');
  });

  it('terminates on a self-referencing schema', () => {
    const recursive = {
      components: {
        schemas: {
          Node: { type: 'object', properties: { next: { $ref: '#/components/schemas/Node' } } },
        },
      },
    };

    // Without the depth guard this recurses until the stack gives out.
    expect(() => sampleFromSchema(recursive, { $ref: '#/components/schemas/Node' })).not.toThrow();
  });

  it('returns null for a non-schema', () => {
    expect(sampleFromSchema(spec, undefined)).toBeNull();
    expect(sampleFromSchema(spec, 'nonsense')).toBeNull();
  });
});

describe('requestBodySample', () => {
  it('builds a sendable body for the reading list’s create operation', () => {
    const sample = requestBodySample(spec, '/books', 'post');

    // Realistic rather than a skeleton: the sample spec carries `example`
    // values, which is the whole reason "Insert sample" is worth offering.
    expect(JSON.parse(sample!)).toEqual({
      author: 'J. R. R. Tolkien',
      status: 'to_read',
      title: 'The Lord of the Rings',
    });
  });

  it('builds a body for the update operation', () => {
    expect(JSON.parse(requestBodySample(spec, '/books/{id}', 'put')!)).toEqual({
      status: 'reading',
    });
  });

  it('pretty-prints, so the editor shows readable lines', () => {
    expect(requestBodySample(spec, '/books', 'post')).toContain('\n  ');
  });

  it('is undefined for an operation that takes no body', () => {
    // Lets the editor hide the button instead of inserting `null`.
    expect(requestBodySample(spec, '/books', 'get')).toBeUndefined();
    expect(requestBodySample(spec, '/books/{id}', 'delete')).toBeUndefined();
  });

  it('is undefined for an unknown path or method', () => {
    expect(requestBodySample(spec, '/nope', 'post')).toBeUndefined();
    expect(requestBodySample(spec, '/books', 'patch')).toBeUndefined();
  });

  it('prefers a media-type example over one assembled from the schema', () => {
    const withExample = {
      paths: {
        '/x': {
          post: {
            requestBody: {
              content: {
                'application/json': {
                  example: { whole: 'body' },
                  schema: { type: 'object', properties: { other: { type: 'string' } } },
                },
              },
            },
          },
        },
      },
    };

    expect(JSON.parse(requestBodySample(withExample, '/x', 'post')!)).toEqual({ whole: 'body' });
  });

  // Media types are case-insensitive and may carry parameters, and both
  // spellings are common in real documents. Missing them shows up only as
  // "Insert sample" quietly not being offered.
  describe('media-type key matching', () => {
    const bodyWith = (mediaType: string) => ({
      paths: {
        '/x': {
          post: {
            requestBody: {
              content: {
                [mediaType]: {
                  schema: { type: 'object', properties: { a: { type: 'string', example: 'x' } } },
                },
              },
            },
          },
        },
      },
    });

    it('ignores a charset parameter on the key', () => {
      expect(
        JSON.parse(requestBodySample(bodyWith('application/json; charset=utf-8'), '/x', 'post')!),
      ).toEqual({ a: 'x' });
    });

    it('matches regardless of case', () => {
      expect(JSON.parse(requestBodySample(bodyWith('Application/JSON'), '/x', 'post')!)).toEqual({
        a: 'x',
      });
    });

    it('tolerates whitespace around the parameter separator', () => {
      expect(
        JSON.parse(requestBodySample(bodyWith('application/json ;charset=utf-8'), '/x', 'post')!),
      ).toEqual({ a: 'x' });
    });

    it('prefers the canonical spelling when a document lists both', () => {
      const both = {
        paths: {
          '/x': {
            post: {
              requestBody: {
                content: {
                  'application/json; charset=utf-8': { example: { from: 'parameterised' } },
                  'application/json': { example: { from: 'canonical' } },
                },
              },
            },
          },
        },
      };

      expect(JSON.parse(requestBodySample(both, '/x', 'post')!)).toEqual({ from: 'canonical' });
    });

    it('does not match a type that merely starts with the JSON one', () => {
      // `application/jsonl` is a different format, not a parameterised spelling.
      const jsonl = {
        paths: {
          '/x': {
            post: {
              requestBody: { content: { 'application/jsonl': { schema: { type: 'string' } } } },
            },
          },
        },
      };

      expect(requestBodySample(jsonl, '/x', 'post')).toBeUndefined();
    });
  });

  it('is undefined when the body is not JSON', () => {
    const xmlOnly = {
      paths: { '/x': { post: { requestBody: { content: { 'application/xml': {} } } } } },
    };

    expect(requestBodySample(xmlOnly, '/x', 'post')).toBeUndefined();
  });
});
