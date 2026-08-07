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

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { OxygenUIThemeProvider, WSO2Theme } from '@wso2/oxygen-ui';
import { BrowserRouter } from 'react-router-dom';

import { ApiClientProvider } from './api/ApiClientProvider';
import { ErrorBoundary } from './components/ErrorBoundary';
import { NotificationProvider } from './components/Notifications';
import { runtimeConfig } from './config/runtime';
import { AuthProvider } from './features/auth/AuthProvider';
import { ProductActivation } from './features/billing/ProductActivation';
import { AppRoutes } from './routes/AppRoutes';

const isProduction = import.meta.env.PROD;

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: isProduction ? 2 : false,
      refetchOnWindowFocus: isProduction,
    },
  },
});

export default function App() {
  return (
    <OxygenUIThemeProvider theme={WSO2Theme}>
      <ApiClientProvider>
        <QueryClientProvider client={queryClient}>
          <ErrorBoundary>
            <BrowserRouter basename={runtimeConfig.appBasePath || undefined}>
              <AuthProvider>
                <ProductActivation />
                <NotificationProvider>
                  <AppRoutes />
                </NotificationProvider>
              </AuthProvider>
            </BrowserRouter>
          </ErrorBoundary>
        </QueryClientProvider>
      </ApiClientProvider>
    </OxygenUIThemeProvider>
  );
}
