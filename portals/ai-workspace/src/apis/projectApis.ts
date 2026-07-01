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
import { PLATFORM_API_BASE_URL } from '../config.env';
import type { ProjectBase } from '../utils/types';

// ============================================================================
// Type Definitions
// ============================================================================

export interface CreateProjectRequest {
  displayName: string;
  description?: string;
}

export interface UpdateProjectRequest {
  id: string;
  displayName: string;
  organizationId: string;
  description?: string;
}

export interface ProjectListResponse {
  count: number;
  list: ProjectBase[];
}

// ============================================================================
// Project API Functions
// ============================================================================
export const DEFAULT_PROJECT_NAME = 'Default';

/**
 * Create the starter "Default Project" for a newly provisioned organization.
 */
export async function createDefaultProject(): Promise<void> {
  try {
    await createProject({ name: DEFAULT_PROJECT_NAME }, PLATFORM_API_BASE_URL);
  } catch (error) {
    logger.error('Failed to create default project for new organization:', error);
  }
}

/**
 * Create a new project.
 * Organization is resolved from the JWT token on the server side.
 */
export async function createProject(
  request: CreateProjectRequest,
  baseUrl: string
): Promise<ProjectBase> {
  try {
    const response = await post<ProjectBase>('/projects', request, baseUrl);
    return response;
  } catch (error) {
    logger.error('Failed to create project:', error);
    throw error;
  }
}

/**
 * Get all projects for the current user's organization.
 * Uses PLATFORM_API_BASE_URL — no arguments needed.
 * Called by AppShellContext on startup.
 */
export async function getProjects(): Promise<ProjectBase[]> {
  try {
    const response = await get<ProjectListResponse>(
      '/projects',
      undefined,
      PLATFORM_API_BASE_URL
    );
    return response.list ?? [];
  } catch (error) {
    logger.error('Failed to get projects:', error);
    throw error;
  }
}

/**
 * List all projects for the current user's organization.
 * Organization is resolved from the JWT token on the server side.
 */
export async function listProjects(
  baseUrl: string
): Promise<ProjectListResponse> {
  try {
    const response = await get<ProjectListResponse>(
      '/projects',
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error('Failed to list projects:', error);
    throw error;
  }
}

/**
 * Get a single project by its UUID.
 */
export async function getProject(
  projectId: string,
  baseUrl: string
): Promise<ProjectBase> {
  try {
    const response = await get<ProjectBase>(
      `/projects/${encodeURIComponent(projectId)}`,
      undefined,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to get project ${projectId}:`, error);
    throw error;
  }
}

/**
 * Update an existing project.
 * The spec's PUT /projects/{projectId} requestBody is the full Project object,
 * so `request` must include `id` and `organizationId` alongside the editable fields.
 */
export async function updateProject(
  projectId: string,
  request: UpdateProjectRequest,
  baseUrl: string
): Promise<ProjectBase> {
  try {
    const response = await put<ProjectBase>(
      `/projects/${encodeURIComponent(projectId)}`,
      request,
      baseUrl
    );
    return response;
  } catch (error) {
    logger.error(`Failed to update project ${projectId}:`, error);
    throw error;
  }
}

/**
 * Delete a project by its UUID.
 */
export async function deleteProject(
  projectId: string,
  baseUrl: string
): Promise<void> {
  try {
    await del<void>(
      `/projects/${encodeURIComponent(projectId)}`,
      undefined,
      baseUrl
    );
  } catch (error) {
    logger.error(`Failed to delete project ${projectId}:`, error);
    throw error;
  }
}
