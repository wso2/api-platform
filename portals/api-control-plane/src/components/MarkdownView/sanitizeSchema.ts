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

import type { Schema } from 'hast-util-sanitize';
import { defaultSchema } from 'rehype-sanitize';

type AttributeEntry = NonNullable<Schema['attributes']>[string][number];

const entryName = (entry: AttributeEntry) => (Array.isArray(entry) ? entry[0] : entry);

const toTagName = (tag: string) => tag.trim().toLowerCase();

/**
 * Build the sanitize schema the Markdown viewer passes to rehype-sanitize.
 *
 * The baseline is always rehype-sanitize's `defaultSchema` (GitHub's
 * allowlist). The two parameters only *add* to that baseline — a tag or
 * attribute that is not on the default allowlist AND not on the operator's
 * additions remains denied. Nothing here can relax the sanitizer below the
 * library default.
 *
 * Attribute names must be camelCase hast property names (`className`,
 * `dataDocId`, `srcSet`); an HTML spelling such as `data-doc-id` has no effect.
 * Naming an attribute for a tag lifts any default value restriction on it for
 * that tag (e.g. `className` on `h2`); a `"*"` entry does not.
 */
export function buildSanitizeSchema(
  additionalTags: readonly string[],
  additionalAttributes: Readonly<Record<string, readonly string[]>>,
): Schema {
  const baseTagNames = defaultSchema.tagNames ?? [];
  const extraTagNames = [...new Set(additionalTags.map(toTagName))].filter(
    (tag) => tag !== '' && !baseTagNames.includes(tag),
  );
  const tagNames = extraTagNames.length === 0 ? baseTagNames : [...baseTagNames, ...extraTagNames];

  const baseAttributes = defaultSchema.attributes ?? {};
  const attributes: Schema['attributes'] = { ...baseAttributes };
  for (const [rawTag, rawNames] of Object.entries(additionalAttributes)) {
    const tag = rawTag.trim() === '*' ? '*' : toTagName(rawTag);
    const names = [...new Set(rawNames.map((name) => name.trim()))].filter((name) => name !== '');
    if (tag === '' || names.length === 0) continue;
    const existing = attributes[tag] ?? [];
    // Replace a [name, ...allowedValues] restriction with the bare name.
    const kept = existing.filter((entry) => !(Array.isArray(entry) && names.includes(entryName(entry))));
    const keptNames = new Set(kept.map(entryName));
    const added = names.filter((name) => !keptNames.has(name));
    if (added.length === 0 && kept.length === existing.length) continue;
    attributes[tag] = [...kept, ...added];
  }

  return {
    ...defaultSchema,
    attributes,
    // `poster` is a URL but has no default scheme check; reuse `src`'s.
    protocols: { ...defaultSchema.protocols, poster: defaultSchema.protocols?.src ?? ['http', 'https'] },
    tagNames,
  };
}
