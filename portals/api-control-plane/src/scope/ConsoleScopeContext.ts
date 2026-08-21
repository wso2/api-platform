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

import { createContext, useContext } from 'react';

import type { ApiCapabilities } from '../pages/appShell/appShellPages/apis/utils/apiCapabilities';
import { RestApi } from '../api/resources/restApis';
import { Organization } from '../api/resources/organizations';
import { Project } from '../api/resources/projects';

export type ConsoleRouteParams = {
  apiHandler?: string;
  deploymentId?: string;
  environmentId?: string;
  orgHandle?: string;
  projectHandler?: string;
};

/**
 * Token-ready scope identifiers the data hooks default to when called with no
 * args. `orgHandle` is only populated once the org-token exchange for the
 * active org has completed (it mirrors the provider's `queryOrgHandle`), so
 * context-aware queries never fire before their bearer token is ready.
 */
export type ActiveScope = {
  orgHandle?: string;
  projectHandler?: string;
  apiHandler?: string;
};

export type ConsoleScope = {
  activeScope: ActiveScope;
  capabilities: ApiCapabilities;
  component?: RestApi;
  isApiScope: boolean;
  isLoading: boolean;
  isOrganizationScope: boolean;
  isProjectScope: boolean;
  organization?: Organization;
  organizations: Organization[];
  params: ConsoleRouteParams;
  project?: Project;
  projects: Project[];
  projectsError?: Error;
};

// In its own module (not ConsoleScopeProvider) so the data hooks can read it
// without a circular import — the provider imports those hooks to build scope.
// Exported so tests can inject a stub scope via `ConsoleScopeContext.Provider`.
export const ConsoleScopeContext = createContext<ConsoleScope | null>(null);

export const useConsoleScope = () => {
  const context = useContext(ConsoleScopeContext);
  if (!context) {
    throw new Error('useConsoleScope must be used within ConsoleScopeProvider');
  }
  return context;
};
