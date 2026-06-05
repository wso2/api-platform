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

import { get, post, put, del } from '../clients/choreoApiClient';
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
      `/llm-provider-templates?organizationId=${encodeURIComponent(organizationId)}`,
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
 * Get all Provider Templates
 * 
 * @param organizationId - The organization ID
 * @returns Promise with the list of provider templates
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
      `/llm-provider-templates?organizationId=${encodeURIComponent(organizationId)}`,
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
      `/llm-provider-templates/${encodeURIComponent(templateId)}?organizationId=${encodeURIComponent(organizationId)}`,
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
export async function updateProviderTemplate(
  templateId: string,
  updates: UpdateProviderTemplateRequest,
  organizationId: string,
  baseUrl: string
): Promise<ProviderTemplate> {
  try {
    const response = await put<ProviderTemplate>(
      `/llm-provider-templates/${encodeURIComponent(templateId)}?organizationId=${encodeURIComponent(organizationId)}`,
      updates,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to update provider template ${templateId}:`, error);
    throw error;
  }
}

/**
 * Delete a Provider Template
 * 
 * @param templateId - The provider template ID
 * @param organizationId - The organization ID
 * @returns Promise that resolves when the template is deleted
 * 
 * @example
 * ```ts
 * await deleteProviderTemplate('openai', 'org-uuid');
 * console.log('Template deleted successfully');
 * ```
 */
export async function deleteProviderTemplate(
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
    logger.error(`Failed to delete provider template ${templateId}:`, error);
    throw error;
  }
}
