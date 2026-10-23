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

import { parseSpecText } from '../../apis/create/utils/specText';
import type { DraftFormValues } from './publicationForm';
import type { StoredDefinition } from './storedDefinition';

/**
 * The Markdown a landing page starts from. It carries over the Endpoints and Resources the
 * portal's default section would show, and is not kept in step with them afterwards.
 */

/** The methods the portal's own Resources section lists. */
const RESOURCE_METHODS = ['get', 'post', 'put', 'delete', 'patch'] as const;

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;

/** One bullet per operation of a REST definition: `- **GET** `/loans` — List loans`. */
const resourceLines = (definition: StoredDefinition): string[] => {
  if (definition.format === 'graphql') return [];
  const parsed = parseSpecText(definition.text, definition.format);
  if (parsed.status !== 'parsed') return [];
  const paths = asRecord(parsed.spec.paths) ?? {};
  return Object.entries(paths).flatMap(([path, pathItem]) => {
    const item = asRecord(pathItem);
    if (!item || !path.startsWith('/')) return [];
    return RESOURCE_METHODS.flatMap((method) => {
      const operation = asRecord(item[method]);
      if (!operation) return [];
      const summary = typeof operation.summary === 'string' ? operation.summary.trim().replace(/\s+/g, ' ') : '';
      return [`- **${method.toUpperCase()}** \`${path}\`${summary ? ` — ${summary}` : ''}`];
    });
  });
};

/** The Overview, then the Endpoints and Resources that have something to list. */
export const buildLandingPageStarter = (
  values: Pick<DraftFormValues, 'description' | 'productionUrl' | 'sandboxUrl'>,
  definition: StoredDefinition | undefined,
): string => {
  const endpoints = [
    values.productionUrl && `- **Production:** ${values.productionUrl}`,
    values.sandboxUrl && `- **Sandbox:** ${values.sandboxUrl}`,
  ].filter(Boolean);
  const resources = definition ? resourceLines(definition) : [];
  return [
    `## Overview\n\n${values.description.trim()}`,
    endpoints.length > 0 && `## Endpoints\n\n${endpoints.join('\n')}`,
    resources.length > 0 && `## Resources\n\n${resources.join('\n')}`,
  ]
    .filter(Boolean)
    .join('\n\n')
    .concat('\n');
};
