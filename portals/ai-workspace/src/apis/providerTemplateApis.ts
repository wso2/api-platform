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

import { get, post, put, del, patch } from '../clients/choreoApiClient';
import { logger } from '../utils/logger';

// ============================================================================
// Type Definitions (from `src/utils/types.ts`)
// ============================================================================

import type {
  ProviderTemplate,
  ProviderTemplatesResponse,
  CreateProviderTemplateRequest,
  UpdateProviderTemplateRequest,
} from '../utils/types';

// ============================================================================
// Provider Template API Functions
// ============================================================================

/**
 * Build the URL-encoded `query` value used for family-scoped collection
 * operations — e.g. `groupId:openai` → `groupId%3Aopenai`. The value is
 * percent-encoded so its `:` is not misread during query parsing.
 */
function buildTemplateQuery(groupId: string): string {
  return encodeURIComponent(`groupId:${groupId}`);
}

/**
 * Create a new Provider Template
 * 
 * @param template - The provider template details
 * @param organizationId - The organization ID
 * @returns Promise with the created template
 * 
 * @example
 * ```ts
 * const template = await createProviderTemplate({
 *   name: "OpenAI Template",
 *   description: "Default OpenAI template"
 * }, 'org-uuid');
 * console.log(template); // { id: '...', name: 'OpenAI Template', ... }
 * ```
 */
export async function createProviderTemplate(
  template: CreateProviderTemplateRequest,
  organizationId: string,
  baseUrl: string
): Promise<ProviderTemplate> {
  try {
    const response = await post<ProviderTemplate>(
      `/llm-provider-templates`,
      template,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to create provider template:', error);
    throw error;
  }
}

/**
 * Get Provider Templates for the catalog listing.
 *
 * Requests `latest=true` so the backend returns only the latest version
 * of each family (one entry per family) rather than the full version history.
 *
 * @param organizationId - The organization ID
 * @returns Promise with the list of provider templates (latest per family)
 * 
 * @example
 * ```ts
 * const response = await getProviderTemplates('org-uuid');
 * console.log(response); // { count: 1, list: [...], pagination: {...} }
 * ```
 */
export async function getProviderTemplates(organizationId: string, baseUrl: string): Promise<ProviderTemplatesResponse> {
  try {
    const response = await get<ProviderTemplatesResponse>(
      `/llm-provider-templates?latest=true&organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch provider templates:', error);
    throw error;
  }
}

/**
 * Get a single Provider Template by ID
 * 
 * @param templateId - The provider template ID
 * @param organizationId - The organization ID
 * @returns Promise with the template details
 * 
 * @example
 * ```ts
 * const template = await getProviderTemplate('openai', 'org-uuid');
 * console.log(template); // { id: 'openai', name: '...', ... }
 * ```
 */
export async function getProviderTemplate(
  templateId: string,
  organizationId: string,
  baseUrl: string
): Promise<ProviderTemplate> {
  try {
    const response = await get<ProviderTemplate>(
      `/llm-provider-templates/${encodeURIComponent(templateId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch provider template ${templateId}:`, error);
    throw error;
  }
}

/**
 * List all versions of a Provider Template (newest first).
 *
 * Templates are immutable per version — each edit creates a new version. This
 * powers the version switcher on the overview page.
 *
 * @param groupId - The template family group id (version routes are keyed by group id, not the per-version handle)
 * @param organizationId - The organization ID
 * @returns Promise with the list of versions (most recent first)
 */
export async function getProviderTemplateVersions(
  groupId: string,
  organizationId: string,
  baseUrl: string
): Promise<ProviderTemplate[]> {
  try {
    const response = await get<ProviderTemplate[] | ProviderTemplatesResponse>(
      `/llm-provider-templates?query=${buildTemplateQuery(groupId)}&organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
    return Array.isArray(response) ? response : response.list ?? [];
  } catch (error) {
    logger.error(`Failed to fetch versions for provider template ${groupId}:`, error);
    throw error;
  }
}

/**
 * Create a new version of an existing Provider Template by copying an existing
 * version and applying overrides.
 *
 * The server clones `fromTemplateId`'s config into the new `toVersion` (in the
 * same family) and overrides any fields present in `overrides`. The new version
 * becomes the latest. Versions start at v1.0 and only go higher.
 *
 * @param fromTemplateId - Handle (id) of the source version to copy from
 * @param toTemplateId - Expected handle of the new version (derived from family + version)
 * @param toVersion - New version identifier, e.g. "v2.0" (must be >= v1.0 and unique)
 * @param overrides - Fields to override on top of the copied config
 * @param organizationId - The organization ID
 * @returns Promise with the newly created version
 */
export async function createProviderTemplateVersion(
  fromTemplateId: string,
  toTemplateId: string,
  toVersion: string,
  overrides: Partial<ProviderTemplate>,
  organizationId: string,
  baseUrl: string
): Promise<ProviderTemplate> {
  try {
    const params = new URLSearchParams({
      fromTemplateId,
      toTemplateId,
      toVersion,
      organizationId,
    });
    const response = await post<ProviderTemplate>(
      `/llm-provider-templates/copy?${params.toString()}`,
      overrides,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to create new version from ${fromTemplateId}:`, error);
    throw error;
  }
}

/**
 * Update an existing Provider Template
 *
 * @param templateId - The provider template ID
 * @param updates - The fields to update
 * @param organizationId - The organization ID
 * @returns Promise with the updated template
 * 
 * @example
 * ```ts
 * const template = await updateProviderTemplate('openai', {
 *   name: "OpenAI Template Updated",
 *   description: "Updated template"
 * }, 'org-uuid');
 * console.log(template); // { id: '...', name: 'OpenAI Template Updated', ... }
 * ```
 */
/**
 * Enable or disable a specific version of a Provider Template. Disabled
 * versions stay in the catalog but are hidden from the provider picker.
 *
 * @param templateId - The version's unique handle (its `id`)
 * @param enabled - Whether the version should be enabled
 * @param organizationId - The organization ID
 * @returns Promise with the updated version
 */
export async function setProviderTemplateVersionEnabled(
  templateId: string,
  enabled: boolean,
  organizationId: string,
  baseUrl: string
): Promise<ProviderTemplate> {
  try {
    const response = await patch<ProviderTemplate>(
      `/llm-provider-templates/${encodeURIComponent(templateId)}?organizationId=${encodeURIComponent(organizationId)}`,
      { enabled },
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to set enabled=${enabled} for provider template ${templateId}:`, error);
    throw error;
  }
}

/**
 * Delete a single version of a Provider Template. If it was the only version
 * the template is removed; otherwise the newest remaining version becomes the
 * latest.
 *
 * @param templateId - The version's unique handle (its `id`)
 * @param organizationId - The organization ID
 */
export async function deleteProviderTemplateVersion(
  templateId: string,
  organizationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/llm-provider-templates/${encodeURIComponent(templateId)}?organizationId=${encodeURIComponent(organizationId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete provider template version ${templateId}:`, error);
    throw error;
  }
}

export async function updateProviderTemplate(
  templateId: string,
  updates: UpdateProviderTemplateRequest,
  organizationId: string,
  baseUrl: string
): Promise<ProviderTemplate> {
  try {
    const response = await put<ProviderTemplate>(
      `/llm-provider-templates/${encodeURIComponent(templateId)}`,
      updates,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to update provider template ${templateId}:`, error);
    throw error;
  }
}

