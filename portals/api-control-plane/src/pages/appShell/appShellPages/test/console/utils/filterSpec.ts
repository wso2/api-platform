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

/** Filters a specification document by search text and HTTP method. */

/** OpenAPI path-item keys that denote operations. */
export const SPEC_HTTP_METHODS = ['get', 'post', 'put', 'delete', 'patch'] as const;

export type SpecHttpMethod = (typeof SPEC_HTTP_METHODS)[number];

/** The method filter's value — one verb, or every verb. */
export type ResourceMethod = 'all' | SpecHttpMethod;

type SpecDocument = Record<string, unknown>;

/** Word-ish runs in prose, so a query can be matched against word starts. */
const PROSE_WORD = /[a-z0-9]+/g;

/**
 * Splits prose into lowercase words for prefix matching.
 *
 * Substring matching is right for a path — people type URL fragments like
 * "{id" or "/books" — but wrong for prose, because a short query is a
 * substring of ordinary English: "id" is inside "valid" and "provide".
 */
const proseWords = (text: string): string[] => text.toLowerCase().match(PROSE_WORD) ?? [];

/**
 * Whether one search term matches an operation.
 *
 * The path is matched as a substring; the summary is matched on word starts,
 * so "book" still finds "books" while "id" no longer finds "valid".
 */
const termMatches = (term: string, path: string, words: string[]): boolean =>
  path.includes(term) || words.some((word) => word.startsWith(term));

const isMethodKey = (key: string): key is SpecHttpMethod =>
  (SPEC_HTTP_METHODS as readonly string[]).includes(key);

/**
 * Keeps operations matching `searchValue` and `selectedMethod`.
 *
 * Returns the original `spec` when unfiltered so swagger-ui preserves its
 * expanded operations and try-out state. A matching path keeps all its
 * operations, subject to the method filter.
 */
export const filterSpecResources = (
  spec: SpecDocument,
  searchValue: string,
  selectedMethod: ResourceMethod,
): SpecDocument => {
  const query = searchValue.trim().toLowerCase();
  // Every term must match, so "list books" narrows rather than widens.
  const terms = query === '' ? [] : query.split(/\s+/);
  if ((!query && selectedMethod === 'all') || !spec.paths || typeof spec.paths !== 'object') {
    return spec;
  }

  const filteredPaths: Record<string, unknown> = {};

  Object.entries(spec.paths as Record<string, unknown>).forEach(([path, pathValue]) => {
    if (!pathValue || typeof pathValue !== 'object') return;

    const pathItem = pathValue as Record<string, unknown>;
    const lowerPath = path.toLowerCase();

    const matchingOperations = SPEC_HTTP_METHODS.filter((method) => {
      const operation = pathItem[method];
      if (!operation || typeof operation !== 'object') return false;
      if (selectedMethod !== 'all' && method !== selectedMethod) return false;
      if (terms.length === 0) return true;

      // Only what the collapsed operation row shows: its path and its
      // summary. The description is behind the expand toggle, so matching it
      // would leave rows in the list for a reason the user cannot see; the
      // result reads as a filter that kept the wrong things.
      const { summary } = operation as Record<string, unknown>;
      const words = typeof summary === 'string' ? proseWords(summary) : [];

      return terms.every((term) => termMatches(term, lowerPath, words));
    });

    if (matchingOperations.length === 0) return;

    // Non-method keys (`parameters`, `servers`, `$ref`) are kept: they are the
    // path item's own configuration, and dropping `parameters` would strip
    // path-level inputs the surviving operations still declare.
    filteredPaths[path] = Object.fromEntries(
      Object.entries(pathItem).filter(
        ([key]) => !isMethodKey(key) || matchingOperations.includes(key),
      ),
    );
  });

  return { ...spec, paths: filteredPaths };
};

/** Whether a document has any operation left to render. */
export const hasResourceOperations = (spec: SpecDocument): boolean => {
  if (!spec.paths || typeof spec.paths !== 'object') return false;

  return Object.values(spec.paths as Record<string, unknown>).some(
    (pathValue) =>
      Boolean(pathValue) &&
      typeof pathValue === 'object' &&
      SPEC_HTTP_METHODS.some((method) => Boolean((pathValue as Record<string, unknown>)[method])),
  );
};
