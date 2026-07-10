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

import React, { useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { IntlProvider } from 'react-intl';
import {
  AcrylicOrangeTheme, AcrylicPurpleTheme, ClassicTheme,
  HighContrastTheme, OxygenUIThemeProvider,
  Box, LinearProgress, Stack, Typography,
} from '@wso2/oxygen-ui';

import App from './App.tsx';
import './styles.css';
import { AUTH_MODE } from './config.env';
import { BFFAuthProvider } from './contexts/BFFAuthProvider';
import { useAppAuth } from './contexts/AppAuthContext';
import BasicAuthLoginPage from './pages/login/BasicAuthLoginPage';

// ── Loading screen ──────────────────────────────────────────────────────────

function LoadingScreen({ message }: { message?: string }) {
  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', p: 4 }}>
      <Stack spacing={2} alignItems="center" sx={{ maxWidth: 480, textAlign: 'center' }}>
        {message && <Typography color="text.secondary">{message}</Typography>}
        <Box sx={{ width: 200 }}>
          <LinearProgress color="primary" variant="indeterminate" />
        </Box>
      </Stack>
    </Box>
  );
}

// ── OIDC redirect — the BFF owns the handshake; we just navigate to it ─────────

function OIDCRedirect() {
  const { login } = useAppAuth();
  useEffect(() => { void login(); }, [login]);
  return <LoadingScreen message="Redirecting to sign in…" />;
}

// ── AppGate — decides login UX vs. the authenticated app ──────────────────────

function AppGate() {
  const { isAuthenticated, isLoading } = useAppAuth();

  if (isLoading) {
    return <LoadingScreen />;
  }

  if (!isAuthenticated) {
    if (AUTH_MODE === 'basic') {
      // On success the BFF has set the session cookie; reload to re-hydrate while
      // preserving the path the user originally requested (matching OIDC return
      // behaviour). Only avoid pinning to the login route itself.
      return <BasicAuthLoginPage onSuccess={() => {
        const { pathname, search } = window.location;
        const target = pathname === '/login' || pathname === '/signin' ? '/' : pathname + search;
        window.location.replace(target);
      }} />;
    }
    return <OIDCRedirect />;
  }

  return (
    <IntlProvider locale="en" defaultLocale="en">
      <App />
    </IntlProvider>
  );
}

// ── Entry point ───────────────────────────────────────────────────────────────

const container = document.getElementById('root')!;
const root = createRoot(container);

root.render(
  <React.StrictMode>
    <OxygenUIThemeProvider
      themes={[
        { key: 'acrylicOrange', label: 'Acrylic Orange Theme', theme: AcrylicOrangeTheme },
        { key: 'acrylicPurple', label: 'Acrylic Purple Theme', theme: AcrylicPurpleTheme },
        { key: 'highContrast', label: 'High Contrast Theme', theme: HighContrastTheme },
        { key: 'classic', label: 'Classic Theme', theme: ClassicTheme },
      ]}
      initialTheme="acrylicOrange"
    >
      <BrowserRouter>
        <BFFAuthProvider>
          <AppGate />
        </BFFAuthProvider>
      </BrowserRouter>
    </OxygenUIThemeProvider>
  </React.StrictMode>
);
