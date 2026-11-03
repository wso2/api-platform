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

import { parseSpecText, serializeSpec, type SpecFormat } from '../../apis/create/utils/specText';

/**
 * How a definition is written: a REST API's OpenAPI document as JSON or YAML,
 * or a GraphQL API's SDL, which has a single serialization.
 */
export type DefinitionFormat = SpecFormat | 'graphql';

/** A stored definition as the editor shows it. */
export type StoredDefinition = { format: DefinitionFormat; text: string };

/** Whether a stored definition's content type marks it as GraphQL SDL. */
const isGraphQLContentType = (contentType = ''): boolean => /graphql/i.test(contentType);

/** An SDL as the editor shows it: as stored, or nothing when there is no text to pre-fill from. */
export const readStoredSdl = (text: string): StoredDefinition | undefined =>
  text.trim() ? { format: 'graphql', text } : undefined;

/**
 * A stored definition's serialization: the content type the server labelled it
 * with, or, where there isn't one (the API's own spec), what the text looks like.
 */
const formatOf = (text: string, contentType = ''): SpecFormat => {
  if (/ya?ml/i.test(contentType)) return 'yaml';
  if (/json/i.test(contentType)) return 'json';
  return text.trimStart().startsWith('{') ? 'json' : 'yaml';
};

/**
 * Reads a definition delivered as text: the draft and publication definitions
 * come back in whichever serialization they were saved in, and
 * `GET /rest-apis/{id}/openapi` (`useRestApiOpenApi`) returns the raw spec. It
 * is shown in that same format — YAML as stored, JSON pretty-printed — so what
 * the user opens is what was saved. Text that doesn't read as an object is
 * treated as "nothing to pre-fill from".
 */
export const readStoredDefinition = (text: string, contentType?: string): StoredDefinition | undefined => {
  if (isGraphQLContentType(contentType)) return readStoredSdl(text);
  const format = formatOf(text, contentType);
  const parsed = parseSpecText(text, format);
  if (parsed.status !== 'parsed') return undefined;
  return { format, text: format === 'json' ? serializeSpec(parsed.spec, 'json') : text };
};

/** The definition re-printed in `format`; unchanged when it is already in it. */
export const reformatDefinition = (definition: StoredDefinition, format: DefinitionFormat): string => {
  // SDL has one serialization, and OpenAPI is never shown as SDL or back.
  if (definition.format === format || definition.format === 'graphql' || format === 'graphql') {
    return definition.text;
  }
  const parsed = parseSpecText(definition.text, definition.format);
  return parsed.status === 'parsed' ? serializeSpec(parsed.spec, format) : definition.text;
};
