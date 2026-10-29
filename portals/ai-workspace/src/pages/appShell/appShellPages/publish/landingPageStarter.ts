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

import { parseDefinitionText, type DraftFormValues } from './mcpPublishForm';

/**
 * The Markdown a landing page starts from. It carries over the Endpoints and the
 * tools, resources and prompts the portal's default section would show, and is not
 * kept in step with them afterwards.
 */

const CAPABILITY_HEADINGS = [
  ['tools', 'Tools'],
  ['resources', 'Resources'],
  ['prompts', 'Prompts'],
] as const;

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;

/** One bullet per named entry of a capability list: `- **search** — Find trips`. */
const capabilityLines = (entries: unknown): string[] =>
  (Array.isArray(entries) ? entries : []).flatMap((entry) => {
    const { name, description } = asRecord(entry) ?? {};
    if (typeof name !== 'string' || name.trim() === '') return [];
    const summary = typeof description === 'string' ? description.trim().replace(/\s+/g, ' ') : '';
    return [`- **${name.trim()}**${summary ? ` — ${summary}` : ''}`];
  });

/** The Overview, then the Endpoints and the capabilities, each only if it has something to say. */
export const buildLandingPageStarter = (
  values: Pick<DraftFormValues, 'description' | 'productionUrl' | 'sandboxUrl'>,
  definitionText: string
): string => {
  const endpoints = [
    values.productionUrl && `- **Production:** ${values.productionUrl}`,
    values.sandboxUrl && `- **Sandbox:** ${values.sandboxUrl}`,
  ].filter(Boolean);
  const definition = parseDefinitionText(definitionText);
  const capabilities = CAPABILITY_HEADINGS.map(([key, heading]) => [heading, capabilityLines(definition[key])] as const);
  return [
    values.description.trim() && `## Overview\n\n${values.description.trim()}`,
    endpoints.length > 0 && `## Endpoints\n\n${endpoints.join('\n')}`,
    ...capabilities.map(([heading, lines]) => lines.length > 0 && `## ${heading}\n\n${lines.join('\n')}`),
  ]
    .filter(Boolean)
    .join('\n\n')
    .concat('\n');
};
