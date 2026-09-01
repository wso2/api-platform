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

type ApiErrorPayload = {
  error?: string;
};

const cloudApiBase = () =>
  insightsRuntimeConfig.platformApiBaseUrl.replace(/\/$/, '');

const readJson = async <T>(response: Response): Promise<T> => {
  const payload = (await response.json().catch(() => ({}))) as T &
    ApiErrorPayload & { message?: string };
  if (!response.ok) {
    const detail =
      payload.error ||
      payload.message ||
      `Cloud analytics request failed (${response.status})`;
    if (response.status === 404) {
      throw new Error(
        `${detail}. No Moesif org mapping exists for the authenticated organization — provision it in wso2cloud core (moesif_org_mappings).`
      );
    }
    throw new Error(detail);
  }
  return payload;
};

const fetchJson = async <T>(
  url: string,
  init?: RequestInit
): Promise<T> => {
  const response = await fetch(url, {
    credentials: 'include',
    headers: {
      accept: 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  });
  return readJson<T>(response);
};

/** GET /cloud/analytics/id-token — Moesif dashboard-viewer token for the caller org. */
export async function fetchViewerToken(): Promise<string> {
  const payload = await fetchJson<ViewerTokenResponse>(
    `${cloudApiBase()}/cloud/analytics/id-token`
  );
  if (!payload.token?.trim()) {
    throw new Error('Viewer token missing from cloud analytics response');
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
  project.handler?.trim() ||
  project.handle?.trim() ||
  project.id?.trim() ||
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
  throw new Error(`Project "${trimmedHandle}" was not found`);
}
