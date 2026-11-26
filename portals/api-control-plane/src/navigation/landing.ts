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

import { readHeaderBreadcrumbsState } from '@wso2/oxygen-ui';

/**
 * Where a signed-in user first lands: a project's home, not the organization's.
 *
 * The console header remembers the last organization and project the user was
 * in, in the browser (`oxygen-ui:header-breadcrumbs`, shared by every WSO2 app
 * that uses it). That is the first choice; without it, the most recently
 * changed project of the organization.
 */

export type StoredScope = { organization: string; project: string };

const nonEmpty = (value: unknown): value is string =>
  typeof value === 'string' && value.trim() !== '';

/**
 * The organization and project the header last remembered, or `null` when
 * there are none or storage cannot be read. Read through Oxygen's own reader,
 * which owns the key and the stored shape.
 */
export function readStoredScope(): StoredScope | null {
  const value = readHeaderBreadcrumbsState()?.value;
  if (!nonEmpty(value?.organization) || !nonEmpty(value?.project)) return null;
  return { organization: value.organization, project: value.project };
}

type DatedProject = { id: string; createdAt?: string; updatedAt?: string };

const changedAt = (project: DatedProject): number =>
  new Date(project.updatedAt || project.createdAt || 0).getTime() || 0;

/**
 * The project changed most recently — the order the organization overview
 * lists projects in — or `undefined` for none.
 */
export function latestProjectId(projects: readonly DatedProject[]): string | undefined {
  return [...projects].sort((left, right) => changedAt(right) - changedAt(left))[0]?.id;
}
