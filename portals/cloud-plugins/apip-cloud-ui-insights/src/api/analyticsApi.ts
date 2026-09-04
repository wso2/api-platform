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

/**
 * Client for WSO2 Cloud Moesif viewer-token endpoint (platform-api-service).
 * See wso2cloud/backend/core/internal/moesifmapping/handler/handler.go.
 *
 * Calls go through the portal BFF same-origin proxy so session cookies and
 * bearer injection stay server-side.
 */

import { insightsRuntimeConfig, platformApiRoot } from '../config/runtimeConfig';

type ViewerTokenResponse = {
  token: string;
};

const cloudApiBase = () =>
  insightsRuntimeConfig.platformApiBaseUrl.replace(/\/$/, '');

/** User-facing copy only. */
const userFacingRequestError = (status: number): string => {
  if (status === 401 || status === 403) {
    return 'You do not have permission to view Insights for this organization.';
  }
  if (status === 404) {
    return 'Insights are not available for this organization. Contact your administrator if you believe this is an error.';
  }
  if (status === 408 || status === 504) {
    return 'Insights took too long to respond. Please try again.';
  }
  if (status >= 500) {
    return 'Insights are temporarily unavailable. Please try again in a few minutes.';
  }
  return 'Unable to load Insights right now. Please try again.';
};

const readJson = async <T>(response: Response): Promise<T> => {
  if (!response.ok) {
    throw new Error(userFacingRequestError(response.status));
  }
  return (await response.json().catch(() => ({}))) as T;
};

const fetchJson = async <T>(
  url: string,
  init?: RequestInit
): Promise<T> => {
  const response = await fetch(url, {
    ...init,
    credentials: 'include',
    headers: {
      accept: 'application/json',
      ...(init?.headers ?? {}),
    },
  });
  return readJson<T>(response);
};

/** GET /cloud/analytics/id-token — Moesif dashboard-viewer token for the caller org. */
export async function fetchViewerToken(): Promise<string> {
  const response = await fetch(`${cloudApiBase()}/cloud/analytics/id-token`, {
    credentials: 'include',
    headers: { accept: 'application/json' },
  });
  if (!response.ok) {
    throw new Error(userFacingRequestError(response.status));
  }
  const payload = (await response.json().catch(() => ({}))) as ViewerTokenResponse;
  if (!payload.token?.trim()) {
    throw new Error(
      'Unable to load Insights right now. Please try again.'
    );
  }
  return payload.token;
}

type ProjectRecord = {
  uuid?: string;
  id?: string;
  handler?: string;
  handle?: string;
  name?: string;
  displayName?: string;
};

const asRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : {};

const pickProjectHandle = (project: ProjectRecord) =>
  project.id?.trim() ||
  project.handler?.trim() ||
  project.handle?.trim() ||
  '';

const pickProjectId = (project: ProjectRecord) =>
  project.uuid?.trim() || pickProjectHandle(project);

const pickProjectName = (project: ProjectRecord, fallback: string) =>
  project.displayName?.trim() || project.name?.trim() || fallback;

/** Resolve a project id for Moesif `project_id` filtering. */
export async function resolveProjectScope(
  orgHandle: string,
  projectHandle: string
): Promise<{ projectId: string; projectName: string }> {
  const trimmedHandle = projectHandle.trim();
  if (!trimmedHandle) {
    throw new Error('Project scope is required for project insights.');
  }

  const headers = { 'X-Org-Id': orgHandle };

  try {
    const project = await fetchJson<ProjectRecord>(
      `${platformApiRoot()}/projects/${encodeURIComponent(trimmedHandle)}`,
      { headers }
    );
    if (pickProjectHandle(project) === trimmedHandle) {
      const projectId = pickProjectId(project);
      if (projectId) {
        return {
          projectId,
          projectName: pickProjectName(project, trimmedHandle),
        };
      }
    }
  } catch {
    // Fall back to list lookup below.
  }

  try {
    const response = await fetchJson<{ list?: unknown[] }>(
      `${platformApiRoot()}/projects`,
      { headers }
    );
    for (const item of response.list ?? []) {
      const project = asRecord(item) as ProjectRecord;
      if (pickProjectHandle(project) === trimmedHandle) {
        const projectId = pickProjectId(project);
        if (!projectId) break;
        return {
          projectId,
          projectName: pickProjectName(project, trimmedHandle),
        };
      }
    }
  } catch {
    // Treat failed project resolution as not found — do not surface proxy/status noise.
  }
  throw new Error(`Project "${trimmedHandle}" was not found`);
}
