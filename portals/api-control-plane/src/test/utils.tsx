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

import { ReactElement, ReactNode, type ComponentProps } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { OxygenUIThemeProvider, WSO2Theme } from '@wso2/oxygen-ui';
import { render, type RenderOptions } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { IntlProvider, ReactIntlErrorCode } from 'react-intl';
import { MemoryRouter } from 'react-router-dom';

import {
  ApiClientProvider,
  realApiClient,
  type ApiClient,
} from '../api/ApiClientProvider';
import { NotificationProvider } from '../components/Notifications';
import { AuthStateContext } from '../contexts/auth/AuthStateContext';
import type { AuthState } from '../contexts/auth/authTypes';
import {
  ConsoleScopeContext,
  type ConsoleScope,
} from '../scope/ConsoleScopeProvider';
import { DISPLAY_TIME_ZONE, INTL_FORMATS } from '../i18n/formats';
import { makeAuthState } from './mockAuthState';

/*
 * Test-only Intl context: uses a bare `IntlProvider` with `messages={}` so
 * tests render `defaultMessage` directly without async catalog loading.
 */
const TEST_LOCALE = 'en';

const handleTestIntlError: NonNullable<
  ComponentProps<typeof IntlProvider>['onError']
> = (err) => {
  // Lookups intentionally miss; only invalid placeholders/config should fail.
  if (err.code === ReactIntlErrorCode.MISSING_TRANSLATION) return;
  console.error(err);
};

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
 * Renders `ui` with the app's provider stack, using injected auth/route context
 * and synchronous i18n. Returns the RTL result, `queryClient`, and `userEvent`.
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
    <IntlProvider
      locale={TEST_LOCALE}
      defaultLocale={TEST_LOCALE}
      messages={{}}
      formats={INTL_FORMATS}
      timeZone={DISPLAY_TIME_ZONE}
      onError={handleTestIntlError}
    >
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
    </IntlProvider>
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
