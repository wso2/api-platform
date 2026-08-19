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

import { getApiCapabilities } from '../pages/appShell/appShellPages/apis/apiCapabilities';
import { organizations, projects } from '../api/mocks/data';
import type { ConsoleScope } from '../scope/ConsoleScopeProvider';

/**
 * Builds a `ConsoleScope` for tests, seeded from the mock fixtures. Inject via
 * `ConsoleScopeContext.Provider` (see `renderWithProviders`) so pages that call
 * `useConsoleScope()` render without mounting the real provider.
 */
export function makeConsoleScope(
  overrides: Partial<ConsoleScope> = {}
): ConsoleScope {
  const organization = overrides.organization ?? organizations[0];
  const project = overrides.project ?? projects[0];
  const component = overrides.component;
  const params = {
    orgHandle: organization?.handle,
    projectHandler: project?.handler,
    ...overrides.params,
  };
  return {
    // Token-ready by default in tests (org handle present), so context-aware
    // hooks resolve their scope from here.
    activeScope: {
      orgHandle: params.orgHandle,
      projectHandler: params.projectHandler,
      apiHandler: params.apiHandler ?? component?.handler,
    },
    capabilities: getApiCapabilities(component),
    component,
    isApiScope: Boolean(component),
    isLoading: false,
    isOrganizationScope: Boolean(organization),
    isProjectScope: Boolean(project),
    organization,
    organizations: overrides.organizations ?? organizations,
    params,
    project,
    projects: overrides.projects ?? projects,
    projectsError: undefined,
    ...overrides,
  };
}
