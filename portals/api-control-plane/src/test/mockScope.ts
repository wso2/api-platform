import { getApiCapabilities } from '../features/apis/apiCapabilities';
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
