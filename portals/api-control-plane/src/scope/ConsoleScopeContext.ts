import { createContext, useContext } from 'react';

import type { ApiCapabilities } from '../features/apis/apiCapabilities';
import type { Api, Organization, Project } from '../types/domain';

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
  component?: Api;
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
