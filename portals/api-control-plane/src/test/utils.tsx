import { ReactElement, ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { OxygenUIThemeProvider, WSO2Theme } from '@wso2/oxygen-ui';
import { render, type RenderOptions } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';

import {
  ApiClientProvider,
  realApiClient,
  type ApiClient,
} from '../api/ApiClientProvider';
import { NotificationProvider } from '../components/Notifications';
import { AuthStateContext } from '../features/auth/AuthStateContext';
import type { AuthState } from '../features/auth/authTypes';
import {
  ConsoleScopeContext,
  type ConsoleScope,
} from '../scope/ConsoleScopeProvider';
import { makeAuthState } from './mockAuthState';

/** Test QueryClient: no retries/refetch so failures surface immediately. */
export function makeTestQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity, refetchOnWindowFocus: false },
      mutations: { retry: false },
    },
  });
}

export type RenderWithProvidersOptions = Omit<RenderOptions, 'wrapper'> & {
  /** Initial router entry (default `/`). */
  route?: string;
  /** All router entries (overrides `route` when provided). */
  routerEntries?: string[];
  /** Injected auth state (default: authenticated via `makeAuthState()`). */
  authState?: AuthState;
  /** Injected console scope. Omit for tests that don't read `useConsoleScope`. */
  scope?: ConsoleScope;
  /** Reuse a specific QueryClient (default: fresh per render). */
  queryClient?: QueryClient;
  /**
   * Override API client functions (merged over the real client) so a test can
   * stub specific calls without `vi.mock`. Omit to use the real client.
   */
  apiClient?: Partial<ApiClient>;
};

/**
 * Renders `ui` inside the app's provider stack (theme → query → router → auth
 * → scope → notifications), mirroring `App.tsx` but with the real Asgardeo SDK
 * and BrowserRouter replaced by injected context + MemoryRouter. Returns the
 * RTL result plus the `queryClient` and a ready `userEvent` instance.
 */
export function renderWithProviders(
  ui: ReactElement,
  {
    route = '/',
    routerEntries,
    authState = makeAuthState(),
    scope,
    queryClient = makeTestQueryClient(),
    apiClient,
    ...renderOptions
  }: RenderWithProvidersOptions = {}
) {
  const mergedApiClient: ApiClient = apiClient
    ? { ...realApiClient, ...apiClient }
    : realApiClient;
  const Wrapper = ({ children }: { children: ReactNode }) => (
    <OxygenUIThemeProvider theme={WSO2Theme}>
      <ApiClientProvider value={mergedApiClient}>
        <QueryClientProvider client={queryClient}>
          <MemoryRouter initialEntries={routerEntries ?? [route]}>
            <AuthStateContext.Provider value={authState}>
              <ScopeWrapper scope={scope}>
                <NotificationProvider>{children}</NotificationProvider>
              </ScopeWrapper>
            </AuthStateContext.Provider>
          </MemoryRouter>
        </QueryClientProvider>
      </ApiClientProvider>
    </OxygenUIThemeProvider>
  );

  return {
    user: userEvent.setup(),
    queryClient,
    ...render(ui, { wrapper: Wrapper, ...renderOptions }),
  };
}

function ScopeWrapper({
  scope,
  children,
}: {
  scope?: ConsoleScope;
  children: ReactNode;
}) {
  if (!scope) return <>{children}</>;
  return (
    <ConsoleScopeContext.Provider value={scope}>
      {children}
    </ConsoleScopeContext.Provider>
  );
}

// Re-export RTL so tests import everything from one place.
export * from '@testing-library/react';
export { default as userEvent } from '@testing-library/user-event';
