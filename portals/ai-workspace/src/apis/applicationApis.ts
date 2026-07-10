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

import type {
  Application,
  ApplicationListResponse,
  ApplicationListQueryParams,
  CreateApplicationRequest,
  UpdateApplicationRequest,
  MappedAPIKeyListResponse,
  APIKeyMappingListQueryParams,
  RemoveApplicationAPIKeyOptions,
  AddApplicationAPIKeysRequest,
  ApplicationAssociationListResponse,
  AddApplicationAssociationsRequest,
  AssociationListQueryParams,
} from '../utils/types';

const buildQueryString = (
  params: Record<string, string | number | undefined>
): string => {
  const searchParams = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') {
      return;
    }
    searchParams.append(key, String(value));
  });
  const query = searchParams.toString();
  return query ? `?${query}` : '';
};

// ============================================================================
// Application API Functions
// ============================================================================

/**
 * Create a new Application
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param application - The application details
 * @param baseUrl - The APIM base URL
 * @returns Promise with the created application
 */
export async function createApplication(
  application: CreateApplicationRequest,
  baseUrl: string
): Promise<Application> {
  try {
    const response = await post<Application>(
      '/applications',
      application,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to create application:', error);
    throw error;
  }
}

/**
 * Get all Applications
 *
 * Organization is resolved from the JWT token on the server side.
 * `projectId` is required by the spec.
 *
 * @param projectId - The project ID (required)
 * @param baseUrl - The APIM base URL
 * @param options - Optional list filters
 * @returns Promise with the list of applications
 */
export async function getApplications(
  projectId: string,
  baseUrl: string,
  options?: Omit<ApplicationListQueryParams, 'projectId'>
): Promise<ApplicationListResponse> {
  try {
    const query = buildQueryString({
      projectId,
      limit: options?.limit,
      offset: options?.offset,
    });
    const response = await get<ApplicationListResponse>(
      `/applications${query}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to fetch applications:', error);
    throw error;
  }
}

/**
 * Get a single Application by ID
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param appId - The application ID
 * @param baseUrl - The APIM base URL
 * @returns Promise with the application details
 */
export async function getApplication(
  appId: string,
  baseUrl: string
): Promise<Application> {
  try {
    const response = await get<Application>(
      `/applications/${encodeURIComponent(appId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch application ${appId}:`, error);
    throw error;
  }
}

/**
 * Update an existing Application.
 *
 * The spec's PUT /applications/{applicationId} requestBody is the full
 * Application schema (id, displayName, projectId, type required). Since
 * callers often only have a partial update DTO, this fetches the current
 * application first and merges the partial `updates` on top before sending.
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param appId - The application ID
 * @param updates - The fields to update
 * @param baseUrl - The APIM base URL
 * @returns Promise with the updated application
 */
export async function updateApplication(
  appId: string,
  updates: UpdateApplicationRequest,
  baseUrl: string
): Promise<Application> {
  try {
    const current = await getApplication(appId, baseUrl);
    const fullApplication: Application = {
      ...current,
      ...updates,
    };
    const response = await put<Application>(
      `/applications/${encodeURIComponent(appId)}`,
      fullApplication,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to update application ${appId}:`, error);
    throw error;
  }
}

/**
 * Delete an Application
 *
 * Organization is resolved from the JWT token on the server side.
 *
 * @param appId - The application ID
 * @param baseUrl - The APIM base URL
 * @returns Promise that resolves when the application is deleted
 */
export async function deleteApplication(
  appId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/applications/${encodeURIComponent(appId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete application ${appId}:`, error);
    throw error;
  }
}

// ============================================================================
// Application API Key Mapping Functions
// ============================================================================

/**
 * List application API key mappings
 *
 * @param appId - The application ID
 * @param baseUrl - The APIM base URL
 * @param options - Optional list pagination filters
 * @returns Promise with the mapped API keys
 */
export async function getApplicationAPIKeys(
  appId: string,
  baseUrl: string,
  options?: APIKeyMappingListQueryParams
): Promise<MappedAPIKeyListResponse> {
  try {
    const query = buildQueryString({
      limit: options?.limit,
      offset: options?.offset,
    });
    const response = await get<MappedAPIKeyListResponse>(
      `/applications/${encodeURIComponent(appId)}/api-keys${query}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to fetch API keys for application ${appId}:`, error);
    throw error;
  }
}

/**
 * Add application API key mappings
 *
 * @param appId - The application ID
 * @param request - The add request
 * @param baseUrl - The APIM base URL
 * @returns Promise with the updated mapped API keys
 */
export async function addApplicationAPIKeys(
  appId: string,
  request: AddApplicationAPIKeysRequest,
  baseUrl: string
): Promise<MappedAPIKeyListResponse> {
  try {
    const response = await post<MappedAPIKeyListResponse>(
      `/applications/${encodeURIComponent(appId)}/api-keys`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to add API keys to application ${appId}:`, error);
    throw error;
  }
}

/**
 * Remove an API key mapping from an application
 *
 * @param appId - The application ID
 * @param mappedKeyId - The mapped API key ID to remove
 * @param baseUrl - The APIM base URL
 * @param options - Delete query options; `entityID` is required by the spec
 * @returns Promise that resolves when the mapping is removed
 */
export async function removeApplicationAPIKey(
  appId: string,
  mappedKeyId: string,
  baseUrl: string,
  options: RemoveApplicationAPIKeyOptions
): Promise<void> {
  try {
    const query = buildQueryString({
      entityID: options.entityID,
    });
    await del<void>(
      `/applications/${encodeURIComponent(appId)}/api-keys/${encodeURIComponent(mappedKeyId)}${query}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(
      `Failed to remove API key ${mappedKeyId} from application ${appId}:`,
      error
    );
    throw error;
  }
}

export async function listApplicationAssociations(
  appId: string,
  baseUrl: string,
  options?: AssociationListQueryParams
): Promise<ApplicationAssociationListResponse> {
  try {
    const query = buildQueryString({
      limit: options?.limit,
      offset: options?.offset,
    });
    const response = await get<ApplicationAssociationListResponse>(
      `/applications/${encodeURIComponent(appId)}/associations${query}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to list associations for application ${appId}:`, error);
    throw error;
  }
}

export async function addApplicationAssociations(
  appId: string,
  request: AddApplicationAssociationsRequest,
  baseUrl: string
): Promise<ApplicationAssociationListResponse> {
  try {
    const response = await post<ApplicationAssociationListResponse>(
      `/applications/${encodeURIComponent(appId)}/associations`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to add associations to application ${appId}:`, error);
    throw error;
  }
}

export async function removeApplicationAssociation(
  appId: string,
  associationId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/applications/${encodeURIComponent(appId)}/associations/${encodeURIComponent(associationId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(
      `Failed to remove association ${associationId} from application ${appId}:`,
      error
    );
    throw error;
  }
}

export async function listApplicationAssociationAPIKeys(
  appId: string,
  associationId: string,
  baseUrl: string,
  options?: AssociationListQueryParams
): Promise<MappedAPIKeyListResponse> {
  try {
    const query = buildQueryString({
      limit: options?.limit,
      offset: options?.offset,
    });
    const response = await get<MappedAPIKeyListResponse>(
      `/applications/${encodeURIComponent(appId)}/associations/${encodeURIComponent(associationId)}/api-keys${query}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(
      `Failed to list API keys for association ${associationId} on application ${appId}:`,
      error
    );
    throw error;
  }
}

export const applicationApis = {
  createApplication,
  getApplications,
  getApplication,
  updateApplication,
  deleteApplication,
  getApplicationAPIKeys,
  addApplicationAPIKeys,
  removeApplicationAPIKey,
  listApplicationAssociations,
  addApplicationAssociations,
  removeApplicationAssociation,
  listApplicationAssociationAPIKeys,
};
