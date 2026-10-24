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

import { defaultSchema } from 'rehype-sanitize';
import { describe, expect, it } from 'vitest';

import { buildSanitizeSchema } from './sanitizeSchema';

describe('buildSanitizeSchema', () => {
  it('returns a schema containing every default tag when no additions are given', () => {
    const schema = buildSanitizeSchema([], {});
    for (const defaultTag of defaultSchema.tagNames ?? []) {
      expect(schema.tagNames).toContain(defaultTag);
    }
    // And still excludes the dangerous ones.
    expect(schema.tagNames).not.toContain('script');
    expect(schema.tagNames).not.toContain('iframe');
  });

  it('appends operator-added tags to the default tagNames', () => {
    const schema = buildSanitizeSchema(['figure', 'figcaption', 'details'], {});
    expect(schema.tagNames).toContain('figure');
    expect(schema.tagNames).toContain('figcaption');
    expect(schema.tagNames).toContain('details');
    // Defaults still present.
    expect(schema.tagNames).toContain('a');
  });

  it('does not duplicate a tag that is already in the default allowlist', () => {
    const schema = buildSanitizeSchema(['a', 'p'], {});
    const occurrences = (tag: string) => schema.tagNames?.filter((t) => t === tag).length ?? 0;
    expect(occurrences('a')).toBe(1);
    expect(occurrences('p')).toBe(1);
  });

  it('appends operator-added attributes to a tag, preserving existing entries', () => {
    const schema = buildSanitizeSchema([], { a: ['download'] });
    const anchorAttrs = schema.attributes?.a ?? [];
    const names = anchorAttrs.map((entry) => (Array.isArray(entry) ? entry[0] : entry));
    expect(names).toContain('href'); // from default
    expect(names).toContain('download'); // from config
  });

  it('appends a wildcard attribute so it applies to every tag', () => {
    const schema = buildSanitizeSchema([], { '*': ['data-doc-id'] });
    const wildcardAttrs = schema.attributes?.['*'] ?? [];
    expect(wildcardAttrs).toContain('data-doc-id');
  });

  it('does not duplicate an attribute that is already allowed', () => {
    const schema = buildSanitizeSchema([], { a: ['href'] });
    const anchorAttrs = schema.attributes?.a ?? [];
    const hrefOccurrences = anchorAttrs.filter((entry) => entry === 'href').length;
    expect(hrefOccurrences).toBe(1);
  });

  it('does not mutate rehype-sanitize defaultSchema', () => {
    const beforeTagsLen = defaultSchema.tagNames?.length ?? 0;
    const beforeAnchorAttrs = JSON.stringify(defaultSchema.attributes?.a);
    buildSanitizeSchema(['figure'], { a: ['download'] });
    expect(defaultSchema.tagNames?.length).toBe(beforeTagsLen);
    expect(JSON.stringify(defaultSchema.attributes?.a)).toBe(beforeAnchorAttrs);
  });
});
