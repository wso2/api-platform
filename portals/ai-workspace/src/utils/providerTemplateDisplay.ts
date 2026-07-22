/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

const PROVIDER_TEMPLATE_NAME_MAP: Record<string, string> = {
  openai: 'OpenAI',
  mistralai: 'Mistral',
  gemini: 'Gemini',
  'azure-openai': 'Azure OpenAI',
  'azureai-foundry': 'Azure AI Foundry',
  awsbedrock: 'AWS Bedrock',
  'aws-bedrock': 'AWS Bedrock',
  anthropic: 'Anthropic',
  'google-vertex': 'Google Vertex AI',
};

/**
 * IDs of the built-in (predefined) provider templates that the backend seeds.
 * These appear in the "Add Provider" catalog but are NOT user-created custom
 * templates, so screens that manage custom templates should exclude them.
 */
const BUILTIN_PROVIDER_TEMPLATE_IDS = new Set(
  Object.keys(PROVIDER_TEMPLATE_NAME_MAP)
);

/**
 * Strip a trailing version suffix (`-v<major>-<minor>`) from a template handle to
 * get the family handle, e.g. `mistralai-v2-0` -> `mistralai`. Handles without a
 * suffix are returned unchanged (covers any legacy built-in handles). Use this
 * for family-level checks (built-in detection, provider-specific behavior) that
 * must not depend on the specific version.
 */
export function familyHandle(id?: string | null): string {
  return (id ?? '').trim().replace(/-v\d+-\d+$/i, '');
}

/**
 * Returns true if the given template id (any version) belongs to one of the
 * predefined built-in families.
 */
export function isBuiltInProviderTemplate(id?: string | null): boolean {
  const normalized = familyHandle(id).toLowerCase();
  if (!normalized) return false;
  return BUILTIN_PROVIDER_TEMPLATE_IDS.has(normalized);
}

export function getProviderTemplateDisplayName(template?: string | null): string {
  const normalizedTemplate = template?.trim().toLowerCase();
  if (!normalizedTemplate) {
    return '';
  }

  return PROVIDER_TEMPLATE_NAME_MAP[normalizedTemplate] ?? template ?? '';
}

/**
 * Resolve a provider's template handle to a human-friendly display name.
 */
export function resolveTemplateDisplayName(
  handle: string | null | undefined,
  templates: Array<{ id?: string; groupId?: string; displayName?: string }>
): string {
  const h = (handle ?? '').trim();
  if (!h) return '';

  const exact = templates.find((t) => t.id === h);
  if (exact?.displayName) return exact.displayName;

  const fam = familyHandle(h).toLowerCase();
  const familyMatch = templates.find(
    (t) =>
      familyHandle(t.id).toLowerCase() === fam ||
      (t.groupId ?? '').toLowerCase() === fam
  );
  if (familyMatch?.displayName) return familyMatch.displayName;

  return getProviderTemplateDisplayName(h);
}

export function truncateProviderDisplayName(
  name?: string | null,
  maxLength = 30
): string {
  const normalizedName = name?.trim() ?? '';
  if (normalizedName.length <= maxLength) {
    return normalizedName;
  }

  return `${normalizedName.slice(0, maxLength).trim()}…`;
}
