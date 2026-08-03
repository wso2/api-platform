import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { OxygenUIThemeProvider, WSO2Theme } from '@wso2/oxygen-ui';
import { BrowserRouter } from 'react-router-dom';

import { ApiClientProvider } from './api/ApiClientProvider';
import { ErrorBoundary } from './components/ErrorBoundary';
import { NotificationProvider } from './components/Notifications';
import { runtimeConfig } from './config/runtime';
import { ApiPlatformAsgardeoProvider } from './features/auth/AsgardeoProvider';
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
              <ApiPlatformAsgardeoProvider>
                <AuthProvider>
                  <ProductActivation />
                  <NotificationProvider>
                    <AppRoutes />
                  </NotificationProvider>
                </AuthProvider>
              </ApiPlatformAsgardeoProvider>
            </BrowserRouter>
          </ErrorBoundary>
        </QueryClientProvider>
      </ApiClientProvider>
    </OxygenUIThemeProvider>
  );
}
