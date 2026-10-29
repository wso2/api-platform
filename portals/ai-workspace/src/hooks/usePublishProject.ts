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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useCallback, useMemo } from 'react';
import { useParams } from 'react-router-dom';

import { useAppShell } from '../contexts/AppShellContext';
import { buildOrgPath, buildProjectPath, getProjectSlug } from '../utils/projectRouting';

/**
 * Where the Publish pages are: the organization and project they act in, and
 * builders for their own URLs, so every link stays inside the current scope.
 */
export function usePublishProject() {
  const { projectSlug } = useParams<{ projectSlug: string }>();
  const { currentOrganization, currentProject, projectsForCurrentOrganization } = useAppShell();

  const effectiveProject = useMemo(
    () =>
      projectsForCurrentOrganization.find((project) => getProjectSlug(project) === projectSlug) ?? currentProject,
    [projectSlug, projectsForCurrentOrganization, currentProject]
  );

  const scopedPath = useCallback(
    (path: string) =>
      effectiveProject
        ? buildProjectPath(currentOrganization, effectiveProject, path)
        : buildOrgPath(currentOrganization, path),
    [currentOrganization, effectiveProject]
  );

  return {
    currentOrganization,
    organizationId: currentOrganization?.uuid ?? '',
    /** An MCP proxy's overview page. */
    overviewPath: useCallback(
      (mcpProxyId: string) => scopedPath(`/mcp-proxy/${encodeURIComponent(mcpProxyId)}`),
      [scopedPath]
    ),
    /** `/publish` and below, in the current scope. */
    publishPath: useCallback((suffix = '') => scopedPath(`/publish${suffix}`), [scopedPath]),
  };
}
