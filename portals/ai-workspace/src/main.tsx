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

import React, { useCallback, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useAuth, AuthProvider } from 'react-oidc-context';
import type { User } from 'oidc-client-ts';
import { IntlProvider } from 'react-intl';
import {
  AcrylicOrangeTheme, AcrylicPurpleTheme, ClassicTheme,
  HighContrastTheme, OxygenUIThemeProvider,
  Box, CircularProgress, Stack, Typography,
} from '@wso2/oxygen-ui';

import App from './App.tsx';
import './styles.css';
import {
  ORG_HANDLE,
  OIDC_SCOPE,
  OIDC_REDIRECT_URI,
  OIDC_POST_LOGOUT_REDIRECT_URI,
} from './config.env';
import { OIDCAppAuthProvider } from './contexts/OIDCAppAuthProvider';
import { SuperAdminAuthProvider, isSuperAdminSession, clearSuperAdminSession } from './contexts/SuperAdminAuthProvider';
import { setStoredToken } from './clients/choreoApiClient';
import { fetchOrgAuthConfig, IDPNotConfiguredError, type OrgAuthConfig } from './apis/authApi';
import OrgHandleEntryPage from './pages/select/OrgHandleEntryPage';
import IDPNotConfiguredPage from './pages/idp-not-configured/IDPNotConfiguredPage';

// ── Storage keys ──────────────────────────────────────────────────────────────

export const ORG_HANDLE_STORAGE_KEY = 'ai_workspace_org_handle';
const AUTH_CONFIG_SESSION_KEY = 'ai_workspace_auth_config';

function resolveStoredHandle(): string {
  if (ORG_HANDLE) return ORG_HANDLE;
  return localStorage.getItem(ORG_HANDLE_STORAGE_KEY) ?? '';
}

function storeAuthConfig(cfg: OrgAuthConfig) {
  sessionStorage.setItem(AUTH_CONFIG_SESSION_KEY, JSON.stringify(cfg));
}

function loadStoredAuthConfig(): OrgAuthConfig | null {
  try {
    const raw = sessionStorage.getItem(AUTH_CONFIG_SESSION_KEY);
    return raw ? (JSON.parse(raw) as OrgAuthConfig) : null;
  } catch {
    return null;
  }
}

function clearStoredAuthConfig() {
  sessionStorage.removeItem(AUTH_CONFIG_SESSION_KEY);
}

function sanitizeReturnUrl(url: string): string {
  if (typeof url !== 'string' || !url.startsWith('/') || url.startsWith('//')) return '/';
  return url.replace(/[\r\n]/g, '') || '/';
}

// ── OIDCWrapper — owns AuthProvider inside the router so onSigninCallback ─────
// can use useNavigate(), and explicitly stores the token before navigating.

interface OIDCWrapperProps {
  authConfig: OrgAuthConfig;
  pendingSignIn: boolean;
}

function OIDCWrapper({ authConfig, pendingSignIn }: OIDCWrapperProps) {
  const navigate = useNavigate();

  const onSigninCallback = useCallback((user: User | void) => {
    if (user && 'access_token' in user && user.access_token) {
      setStoredToken(user.access_token);
    }
    const saved = sanitizeReturnUrl(sessionStorage.getItem('ai_workspace_return_url') || '/');
    sessionStorage.removeItem('ai_workspace_return_url');
    const handle = resolveStoredHandle();
    const dest = saved !== '/' ? saved : (handle ? `/organizations/${handle}/home` : '/');
    navigate(dest, { replace: true });
  }, [navigate]);

  return (
    <AuthProvider
      authority={authConfig.issuer}
      client_id={authConfig.client_id}
      redirect_uri={OIDC_REDIRECT_URI}
      post_logout_redirect_uri={OIDC_POST_LOGOUT_REDIRECT_URI}
      scope={OIDC_SCOPE}
      extraTokenParams={{ scope: OIDC_SCOPE }}
      loadUserInfo={false}
      metadata={{
        issuer: authConfig.issuer,
        authorization_endpoint: authConfig.authorization_endpoint,
        token_endpoint: authConfig.token_endpoint,
        ...(authConfig.logout_url && { end_session_endpoint: authConfig.logout_url }),
      }}
      onSigninCallback={onSigninCallback}
    >
      <OIDCAppAuthProvider>
        <IntlProvider locale="en" defaultLocale="en">
          {pendingSignIn ? <AutoSignIn /> : <App />}
        </IntlProvider>
      </OIDCAppAuthProvider>
    </AuthProvider>
  );
}

// ── AutoSignIn — triggers OIDC redirect as soon as AuthProvider is ready ──────

function AutoSignIn() {
  const auth = useAuth();

  useEffect(() => {
    if (!auth.isLoading && !auth.isAuthenticated) {
      auth.signinRedirect();
    }
  }, [auth.isLoading, auth.isAuthenticated]);

  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <Stack spacing={2} alignItems="center">
        <CircularProgress />
        <Typography variant="body2" color="text.secondary">
          Redirecting to sign in…
        </Typography>
      </Stack>
    </Box>
  );
}

// ── AppRoot — lives inside BrowserRouter, owns all pre-auth routing logic ─────

function AppRoot() {
  const location = useLocation();

  const [superAdminActive, setSuperAdminActive] = useState<boolean>(isSuperAdminSession);
  const [orgHandle, setOrgHandle] = useState<string>(resolveStoredHandle);
  // Load auth config from sessionStorage so the /signin callback survives a page reload.
  const [authConfig, setAuthConfig] = useState<OrgAuthConfig | null>(loadStoredAuthConfig);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [idpNotConfigured, setIdpNotConfigured] = useState(false);
  const [isFetching, setIsFetching] = useState(false);
  // True immediately after the user confirms their org — drives AutoSignIn.
  const [pendingSignIn, setPendingSignIn] = useState(false);

  const loadAuthConfig = useCallback((handle: string) => {
    setIsFetching(true);
    setFetchError(null);
    setIdpNotConfigured(false);
    fetchOrgAuthConfig(handle)
      .then((cfg) => {
        localStorage.setItem(ORG_HANDLE_STORAGE_KEY, handle);
        setOrgHandle(handle);
        storeAuthConfig(cfg);
        setAuthConfig(cfg);
        setPendingSignIn(true);
      })
      .catch((err: unknown) => {
        if (err instanceof IDPNotConfiguredError) {
          localStorage.setItem(ORG_HANDLE_STORAGE_KEY, handle);
          setOrgHandle(handle);
          setIdpNotConfigured(true);
        } else {
          setFetchError(err instanceof Error ? err.message : 'Failed to load auth configuration');
        }
      })
      .finally(() => setIsFetching(false));
  }, []);

  const handleSuperAdminLogin = useCallback(() => setSuperAdminActive(true), []);

  const handleSuperAdminLogout = useCallback(() => {
    setSuperAdminActive(false);
    clearSuperAdminSession();
  }, []);

  // ── Super admin path — no OIDC needed ──────────────────────────────────────
  if (superAdminActive) {
    return (
      <SuperAdminAuthProvider onLogout={handleSuperAdminLogout}>
        <IntlProvider locale="en" defaultLocale="en">
          <App />
        </IntlProvider>
      </SuperAdminAuthProvider>
    );
  }

  // ── IDP not configured ─────────────────────────────────────────────────────
  if (idpNotConfigured) {
    return (
      <IDPNotConfiguredPage
        orgHandle={orgHandle}
        onBack={() => {
          setIdpNotConfigured(false);
          setOrgHandle('');
          setAuthConfig(null);
          localStorage.removeItem(ORG_HANDLE_STORAGE_KEY);
          clearStoredAuthConfig();
        }}
      />
    );
  }

  // ── /getting-started — always show the selection page unless mid-sign-in ───
  // pendingSignIn=true means the user just confirmed their org and AutoSignIn
  // is about to fire — don't interrupt that with the selection page.
  if (location.pathname === '/getting-started' && !pendingSignIn) {
    return (
      <OrgHandleEntryPage
        initialHandle={orgHandle}
        onConfirm={loadAuthConfig}
        onSuperAdminLogin={handleSuperAdminLogin}
        externalError={fetchError}
        isFetching={isFetching}
      />
    );
  }

  // ── No auth config and not at /getting-started → redirect there ────────────
  if (!authConfig) {
    return <Navigate to="/getting-started" replace />;
  }

  // ── OIDC path ──────────────────────────────────────────────────────────────
  return <OIDCWrapper authConfig={authConfig} pendingSignIn={pendingSignIn} />;
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
