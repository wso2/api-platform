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

import { get } from '../clients/choreoApiClient';
import { API_BASE_URLS, TOS_SERVICE_NAME } from '../config.env';
import { logger } from '../utils/logger';
import type { ValidateUserResponse, Organization } from '../utils/types';

// ============================================================================
// Organization API Functions
// ============================================================================

/**
 * Validate user and get their organizations
 * 
 * This is called after FIRST login to get the user's organizations.
 * Uses the same API endpoint as choreo-console.
 * 
 * @returns Promise with user's organizations and idpId
 */
export async function validateUser(): Promise<ValidateUserResponse> {
  const response = await get<ValidateUserResponse>(
    `/validate/user?origin_cloud=${TOS_SERVICE_NAME}`,
    undefined,
    API_BASE_URLS.userManagement
  );
  
  if (!response.organizations || response.organizations.length === 0) {
    throw new Error('No organizations found for the user');
  }
  
  return response;
}

/**
 * Get user's organizations
 * 
 * This is called on page reload when user is already authenticated.
 * Unlike validateUser, this endpoint works on page reload.
 * Uses the same API endpoint as choreo-console's getOrganizations.
 * 
 * @returns Promise with user's organizations
 */
export async function getOrganizations(): Promise<Organization[]> {
  const response = await get<Organization[]>(
    `/orgs`,
    undefined,
    API_BASE_URLS.organizationApi
  );
  
  if (!response || response.length === 0) {
    throw new Error('No organizations found for the user');
  }
  
  return response;
}

/**
 * Check if the current user is an admin for the specified organization.
 * 
 * This API is called after token exchange to determine the user's role
 * (admin or developer) in the organization context.
 * 
 * @param orgHandle - The organization handle
 * @returns Promise with boolean indicating if user is admin
 */
export async function getIsOrgAdmin(orgHandle: string): Promise<boolean> {
  try {
    const response = await get<{ isAdmin?: boolean }>(
      `/orgs/${orgHandle}`,
      undefined,
      API_BASE_URLS.organizationApi
    );
    return response?.isAdmin || false;
  } catch (error) {
    logger.error('Failed to check org admin status:', error);
    return false;
  }
}
