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

import { type ReactNode, useState } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';

import { ApiClientProvider } from './api/ApiClientProvider';
import { createQueryClient } from './api/core/queryClient';
import { ErrorBoundary } from './components/ErrorBoundary';
import { NotificationProvider, useNotifications } from './components/Notifications';
import { runtimeConfig } from './config/runtime';
import { AuthProvider } from './contexts/auth/AuthProvider';
import { ProductActivation } from './hooks/ProductActivation';
import { AppRoutes } from './routes/AppRoutes';
import {
  ExtensionsProvider,
  type ApiControlPlaneExtension,
} from './extensions';
import { I18nProvider } from './i18n';
import { AppThemeProvider } from './theme/AppThemeProvider';

/**
 * Builds the app's QueryClient with the notification handler already attached,
 * which is why it lives below `NotificationProvider` rather than at module
 * scope.
 *
 * The handler is what stops a failed mutation from failing silently: it fires
 * for every mutation error, including ones whose own `onError` performs an
 * optimistic rollback, so a component that forgets to surface an error still
 * cannot swallow it.
 *
 * `useState` with an initializer creates the client exactly once per mount —
 * building it inline on every render would discard the whole cache each time.
 */
function AppQueryProvider({ children }: { children: ReactNode }) {
  const { notify } = useNotifications();
  const [queryClient] = useState(() =>
    createQueryClient({
      onMutationError: (error) => notify(error.message, 'error'),
    })
  );

  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

export type AppProps = {
  extensions?: readonly ApiControlPlaneExtension[];
};

export default function App({ extensions = [] }: AppProps) {
  return (

    <I18nProvider>
      <AppThemeProvider>
        <NotificationProvider>
          <AppQueryProvider>
            <ApiClientProvider>
              <ErrorBoundary>
                <BrowserRouter basename={runtimeConfig.appBasePath || undefined}>
                  <AuthProvider>
                    <ProductActivation />
                    <ExtensionsProvider extensions={extensions}>
                      <AppRoutes extensions={extensions} />
                    </ExtensionsProvider>
                  </AuthProvider>
                </BrowserRouter>
              </ErrorBoundary>
            </ApiClientProvider>
          </AppQueryProvider>
        </NotificationProvider>
      </AppThemeProvider>
    </I18nProvider>
  );
}
