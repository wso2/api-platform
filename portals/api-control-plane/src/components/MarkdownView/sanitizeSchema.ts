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
import type { Schema } from 'hast-util-sanitize';

/**
 * Build the sanitize schema the Markdown viewer passes to rehype-sanitize.
 *
 * The baseline is always rehype-sanitize's `defaultSchema` (GitHub's
 * allowlist). The two parameters only *add* to that baseline — a tag or
 * attribute that is not on the default allowlist AND not on the operator's
 * additions remains denied. Nothing here can relax the sanitizer below the
 * library default.
 */
export function buildSanitizeSchema(
  additionalTags: readonly string[],
  additionalAttributes: Readonly<Record<string, readonly string[]>>,
): Schema {
  const baseTagNames = defaultSchema.tagNames ?? [];
  const extraTagNames = additionalTags.filter((tag) => !baseTagNames.includes(tag));
  const tagNames = extraTagNames.length === 0 ? baseTagNames : [...baseTagNames, ...extraTagNames];

  const baseAttributes = defaultSchema.attributes ?? {};
  const attributes: Schema['attributes'] = { ...baseAttributes };
  for (const [tag, extras] of Object.entries(additionalAttributes)) {
    if (extras.length === 0) continue;
    const existing = baseAttributes[tag] ?? [];
    // defaultSchema attribute entries are either a bare name string or a
    // [name, ...allowed-values] tuple. We only add bare names — operators
    // allowing an attribute everywhere don't need the value-restriction
    // syntax, and keeping it uniform avoids a type-widening escape hatch.
    const existingNames = new Set(existing.map((entry) => (Array.isArray(entry) ? entry[0] : entry)));
    const newNames = extras.filter((name) => !existingNames.has(name));
    if (newNames.length === 0) continue;
    attributes[tag] = [...existing, ...newNames];
  }

  return { ...defaultSchema, tagNames, attributes };
}
