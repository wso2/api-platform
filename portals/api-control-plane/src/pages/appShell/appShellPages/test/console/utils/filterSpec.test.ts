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

import { filterSpecResources, hasResourceOperations } from './filterSpec';

const spec = {
  openapi: '3.0.1',
  paths: {
    '/books': {
      parameters: [{ in: 'query', name: 'tenant' }],
      get: { operationId: 'listBooks', summary: 'List all the reading list books' },
      post: { operationId: 'createBook', summary: 'Add a new book to the reading list' },
    },
    '/books/{id}': {
      get: { operationId: 'getBookById', summary: 'Get reading list book by id' },
      delete: {
        description: 'Removes the record permanently',
        operationId: 'deleteBookById',
        summary: 'Delete a reading list book by id',
      },
    },
    '/authors': {
      get: { operationId: 'listAuthors', summary: 'List contributors' },
    },
  },
} as Record<string, unknown>;

const pathsOf = (result: Record<string, unknown>) =>
  Object.keys((result.paths ?? {}) as Record<string, unknown>);

const methodsOf = (result: Record<string, unknown>, path: string) => {
  const item = ((result.paths ?? {}) as Record<string, Record<string, unknown>>)[path] ?? {};
  return Object.keys(item).filter((key) => key !== 'parameters');
};

describe('filterSpecResources — identity', () => {
  it('returns the very same object when nothing is being filtered', () => {
    // Not merely an equal one. swagger-ui re-parses the whole document when
    // this prop's *reference* changes, discarding expanded operations and
    // everything typed into a try-out form.
    expect(filterSpecResources(spec, '', 'all')).toBe(spec);
    expect(filterSpecResources(spec, '   ', 'all')).toBe(spec);
  });

  it('returns the same object for a document with no paths', () => {
    const bare = { openapi: '3.0.1' };

    expect(filterSpecResources(bare, 'books', 'get')).toBe(bare);
  });

  it('does not mutate the input document', () => {
    const before = JSON.stringify(spec);

    filterSpecResources(spec, 'authors', 'get');

    // The definition is cached by React Query and shared with other consumers.
    expect(JSON.stringify(spec)).toBe(before);
  });
});

describe('filterSpecResources — search', () => {
  it('matches on the path', () => {
    expect(pathsOf(filterSpecResources(spec, '/authors', 'all'))).toEqual(['/authors']);
  });

  it('keeps every operation of a path whose text matches', () => {
    // Searching a resource shows the whole resource, rather than only the
    // operations that happen to repeat the word in their own summary.
    expect(methodsOf(filterSpecResources(spec, '/books/{id}', 'all'), '/books/{id}')).toEqual([
      'get',
      'delete',
    ]);
  });

  it('matches on an operation summary', () => {
    const result = filterSpecResources(spec, 'Add a new book', 'all');

    expect(pathsOf(result)).toEqual(['/books']);
    expect(methodsOf(result, '/books')).toEqual(['post']);
  });

  it('does not let a short query match the middle of an ordinary word', () => {
    // The reported bug: typing "id" kept every operation, because "id" is a
    // substring of "valid", "provide" and "identifier". It read as the filter
    // doing nothing at all.
    const doc = {
      paths: {
        '/books': { get: { summary: 'Returns all valid books' } },
        '/books/{id}': { get: { summary: 'Get a book' } },
        '/authors': { get: { summary: 'Provide a page number' } },
      },
    };

    expect(pathsOf(filterSpecResources(doc, 'id', 'all'))).toEqual(['/books/{id}']);
  });

  it('still matches a word by its start, so plurals and stems are found', () => {
    const doc = {
      paths: { '/books': { get: { summary: 'List paginated books' } } },
    };

    expect(pathsOf(filterSpecResources(doc, 'book', 'all'))).toEqual(['/books']);
    expect(pathsOf(filterSpecResources(doc, 'pagin', 'all'))).toEqual(['/books']);
  });

  it('requires every term of a multi-word query to match', () => {
    const doc = {
      paths: {
        '/books': { get: { summary: 'List books' } },
        '/authors': { get: { summary: 'List authors' } },
      },
    };

    expect(pathsOf(filterSpecResources(doc, 'list books', 'all'))).toEqual(['/books']);
  });

  it('does not match on a description, which the collapsed row does not show', () => {
    // Matching hidden text leaves rows in the list for a reason the user
    // cannot see, which reads as the filter keeping the wrong things.
    expect(pathsOf(filterSpecResources(spec, 'permanently', 'all'))).toEqual([]);
  });

  it('is case-insensitive and ignores surrounding whitespace', () => {
    expect(pathsOf(filterSpecResources(spec, '  CONTRIBUTORS  ', 'all'))).toEqual(['/authors']);
  });

  it('yields no paths when nothing matches', () => {
    expect(pathsOf(filterSpecResources(spec, 'nothing-matches-this', 'all'))).toEqual([]);
  });

  it('keeps the path item’s own configuration alongside the surviving operations', () => {
    const result = filterSpecResources(spec, 'Add a new book', 'all');
    const item = ((result.paths ?? {}) as Record<string, Record<string, unknown>>)['/books'];

    // Dropping `parameters` would strip path-level inputs the surviving
    // operation still declares.
    expect(item).toHaveProperty('parameters');
  });
});

describe('filterSpecResources — method', () => {
  it('keeps only the selected verb', () => {
    const result = filterSpecResources(spec, '', 'post');

    expect(pathsOf(result)).toEqual(['/books']);
    expect(methodsOf(result, '/books')).toEqual(['post']);
  });

  it('drops paths that have no operation of that verb', () => {
    expect(pathsOf(filterSpecResources(spec, '', 'delete'))).toEqual(['/books/{id}']);
  });

  it('combines with the search, narrowing by both', () => {
    // `/books` and `/books/{id}` both match the text; only one has a DELETE.
    const result = filterSpecResources(spec, 'book', 'delete');

    expect(pathsOf(result)).toEqual(['/books/{id}']);
    expect(methodsOf(result, '/books/{id}')).toEqual(['delete']);
  });

  it('yields no paths when the verb exists nowhere', () => {
    expect(pathsOf(filterSpecResources(spec, '', 'patch'))).toEqual([]);
  });
});

describe('filterSpecResources — malformed input', () => {
  it('skips path items that are not objects', () => {
    const malformed = { paths: { '/a': null, '/b': 'nope', '/c': { get: {} } } };

    expect(pathsOf(filterSpecResources(malformed, '', 'get'))).toEqual(['/c']);
  });

  it('ignores non-object operations', () => {
    const malformed = { paths: { '/a': { get: 'nope', post: { summary: 'real' } } } };

    expect(methodsOf(filterSpecResources(malformed, 'real', 'all'), '/a')).toEqual(['post']);
  });
});

describe('hasResourceOperations', () => {
  it('is true when any path has an operation', () => {
    expect(hasResourceOperations(spec)).toBe(true);
  });

  it('is false for a filtered-to-nothing document', () => {
    expect(hasResourceOperations(filterSpecResources(spec, 'no-such-thing', 'all'))).toBe(false);
  });

  it('is false for a document with no paths at all', () => {
    expect(hasResourceOperations({ openapi: '3.0.1' })).toBe(false);
    expect(hasResourceOperations({ paths: {} })).toBe(false);
  });

  it('is false for a path item carrying only non-method keys', () => {
    // A path with `parameters` but no verb renders nothing, so the empty
    // message should still show.
    expect(hasResourceOperations({ paths: { '/a': { parameters: [] } } })).toBe(false);
  });
});
