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

/** Handle of the built-in cost tracker policy attached automatically at creation. */
export const COST_POLICY_NAME = 'llm-cost';

/**
 * Families that do NOT get `llm-cost` attached automatically at provider
 * creation. For these the operator has to add it by hand, so an `llm-cost`
 * policy on such a provider is a user-added guardrail rather than a default.
 */
const NO_AUTO_COST_POLICY_FAMILIES = new Set(['azure-openai', 'azureai-foundry']);

/**
 * Returns true if a provider created from this template gets the `llm-cost`
 * policy attached automatically. An unknown or empty handle is treated as
 * auto-attaching, which is the conservative default for legacy providers.
 */
export function autoAttachesCostPolicy(templateId?: string | null): boolean {
  return !NO_AUTO_COST_POLICY_FAMILIES.has(familyHandle(templateId).toLowerCase());
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

/**
 * Resolve a provider's template handle to the template's own logo URL
 * (`metadata.logoUrl`, falling back to `logoUrl`), matching the same template
 * `resolveTemplateDisplayName` would. Returns undefined when the template can't
 * be found or has no logo, so callers can fall back to a built-in vendor map.
 * Used only for the "Template: …" label logo / template chip (NOT the provider
 * avatar), so a provider created from a CUSTOM template shows that template's
 * uploaded logo beside the template name.
 */
export function resolveTemplateLogo(
  handle: string | null | undefined,
  templates: Array<{
    id?: string;
    groupId?: string;
    logoUrl?: string;
    metadata?: { logoUrl?: string };
  }>
): string | undefined {
  const h = (handle ?? '').trim();
  if (!h) return undefined;

  const exact = templates.find((t) => t.id === h);
  const fam = familyHandle(h).toLowerCase();
  const match =
    exact ??
    templates.find(
      (t) =>
        familyHandle(t.id).toLowerCase() === fam ||
        (t.groupId ?? '').toLowerCase() === fam
    );

  const logo = match?.metadata?.logoUrl?.trim() || match?.logoUrl?.trim();
  return logo || undefined;
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
