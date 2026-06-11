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

import React, { useCallback, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, useNavigate } from 'react-router-dom';
import { useAuth, AuthProvider } from 'react-oidc-context';
import type { User } from 'oidc-client-ts';
import { IntlProvider } from 'react-intl';
import {
  AcrylicOrangeTheme, AcrylicPurpleTheme, ClassicTheme,
  HighContrastTheme, OxygenUIThemeProvider,
  Box, LinearProgress, Stack, Typography,
} from '@wso2/oxygen-ui';

import App from './App.tsx';
import './styles.css';
import {
  OIDC_AUTHORITY,
  OIDC_CLIENT_ID,
  OIDC_SCOPE,
  OIDC_REDIRECT_URI,
  OIDC_POST_LOGOUT_REDIRECT_URI,
} from './config.env';
import { AUTH_MODE } from './config.env';
import { OIDCAppAuthProvider } from './contexts/OIDCAppAuthProvider';
import { BasicAuthProvider, isBasicAuthSession, clearBasicAuthSession } from './contexts/BasicAuthProvider';
import BasicAuthLoginPage from './pages/login/BasicAuthLoginPage';
import { setStoredToken } from './clients/choreoApiClient';

// ── Helpers ───────────────────────────────────────────────────────────────────

function sanitizeReturnUrl(url: string): string {
  if (typeof url !== 'string' || !url.startsWith('/') || url.startsWith('//')) return '/';
  return url.replace(/[\r\n]/g, '') || '/';
}

// ── OIDCWrapper — owns AuthProvider; lives inside BrowserRouter for useNavigate ─

function OIDCWrapper({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();

  const onSigninCallback = useCallback((user: User | void) => {
    if (user && 'access_token' in user && user.access_token) {
      setStoredToken(user.access_token);
    }
    const saved = sanitizeReturnUrl(sessionStorage.getItem('ai_workspace_return_url') || '/');
    sessionStorage.removeItem('ai_workspace_return_url');
    navigate(saved, { replace: true });
  }, [navigate]);

  const onSignoutCallback = useCallback(() => {
    navigate('/login', { replace: true });
  }, [navigate]);

  return (
    <AuthProvider
      authority={OIDC_AUTHORITY}
      client_id={OIDC_CLIENT_ID}
      redirect_uri={OIDC_REDIRECT_URI}
      post_logout_redirect_uri={OIDC_POST_LOGOUT_REDIRECT_URI}
      scope={OIDC_SCOPE}
      extraTokenParams={{ scope: OIDC_SCOPE }}
      loadUserInfo={false}
      onSigninCallback={onSigninCallback}
      onSignoutCallback={onSignoutCallback}
    >
      <OIDCAppAuthProvider>
        {children}
      </OIDCAppAuthProvider>
    </AuthProvider>
  );
}

// ── AppRoot ────────────────────────────────────────────────────────────────────

function AppRoot() {
  const [basicAuthActive, setBasicAuthActive] = useState<boolean>(isBasicAuthSession);

  const handleBasicAuthLogout = useCallback(() => {
    setBasicAuthActive(false);
    clearBasicAuthSession();
  }, []);

  // ── Basic auth path ────────────────────────────────────────────────────────
  if (AUTH_MODE === 'basic') {
    if (basicAuthActive) {
      return (
        <BasicAuthProvider onLogout={handleBasicAuthLogout}>
          <IntlProvider locale="en" defaultLocale="en">
            <App />
          </IntlProvider>
        </BasicAuthProvider>
      );
    }
    return (
      <BasicAuthLoginPage onSuccess={() => setBasicAuthActive(true)} />
    );
  }

  // ── OIDC not configured ────────────────────────────────────────────────────
  if (!OIDC_AUTHORITY || !OIDC_CLIENT_ID) {
    return (
      <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', p: 4 }}>
        <Stack spacing={2} alignItems="center" sx={{ maxWidth: 480, textAlign: 'center' }}>
          <Typography variant="h5" fontWeight="bold">OIDC not configured</Typography>
          <Typography color="text.secondary">
            Set <code>VITE_OIDC_AUTHORITY</code> and <code>VITE_OIDC_CLIENT_ID</code> to enable login.
          </Typography>
          <Box sx={{ width: 200 }}>
            <LinearProgress color="primary" variant="indeterminate" />
          </Box>
        </Stack>
      </Box>
    );
  }

  // ── OIDC path ──────────────────────────────────────────────────────────────
  return (
    <OIDCWrapper>
      <IntlProvider locale="en" defaultLocale="en">
        <App />
      </IntlProvider>
    </OIDCWrapper>
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
        <AppRoot />
      </BrowserRouter>
    </OxygenUIThemeProvider>
  </React.StrictMode>
);
