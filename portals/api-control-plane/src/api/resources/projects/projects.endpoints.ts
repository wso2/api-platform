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

import { http, type RequestOptions } from '../../core/http';
import type {
  BodyOf,
  PathOf,
  QueryOf,
  ResponseOf,
  Schema,
} from '../../core/spec';

/**
 * Transport layer for `/projects`. One thin function per spec operation:
 * no branching, no adapters, no cache awareness — just "call this endpoint
 * with these arguments and get the spec's response type back".
 *
 * The organization is never a parameter here: the collection is scoped by the
 * `X-Org-Id` header the transport attaches from `options.orgId`, which is why
 * no list filter names an organization.
 */

export type Project = Schema<'Project'>;
export type ProjectListResponse = ResponseOf<'ListProjects'>;
export type ListProjectsQuery = QueryOf<'ListProjects'>;
export type CreateProjectBody = BodyOf<'CreateProject'>;
export type UpdateProjectBody = BodyOf<'UpdateProject'>;

const BASE = '/projects';

/** URL-encoded path for one project. Handles are user-supplied — always encode. */
const resourcePath = (projectId: PathOf<'GetProject'>['projectId']): string =>
  `${BASE}/${encodeURIComponent(projectId)}`;

export const listProjects = async (
  options?: RequestOptions
): Promise<ProjectListResponse> => {
  return http.get<ProjectListResponse>(BASE, {
    ...options,
    operationName: 'ListProjects',
  });
};

export const getProject = async (
  projectId: string,
  options?: RequestOptions
): Promise<Project> => {
  return http.get<Project>(resourcePath(projectId), {
    ...options,
    operationName: 'GetProject',
  });
};

export const createProject = async (
  body: CreateProjectBody,
  options?: RequestOptions
): Promise<Project> => {
  return http.post<Project>(BASE, body, {
    ...options,
    operationName: 'CreateProject',
  });
};

export const updateProject = async (
  projectId: string,
  body: UpdateProjectBody,
  options?: RequestOptions
): Promise<Project> => {
  return http.put<Project>(resourcePath(projectId), body, {
    ...options,
    operationName: 'UpdateProject',
  });
};

/**
 * Deletes a project. The backend enforces the last-project and
 * associated-resource guards and answers 400 with a descriptive `code`, which
 * reaches the caller as `ApiError.code`.
 */
export const deleteProject = async (
  projectId: string,
  options?: RequestOptions
): Promise<void> => {
  await http.delete<void>(resourcePath(projectId), {
    ...options,
    operationName: 'DeleteProject',
  });
};
