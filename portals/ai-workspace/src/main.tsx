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

import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { IntlProvider } from 'react-intl';
import { AuthProvider } from 'react-oidc-context';
import { AcrylicOrangeTheme, AcrylicPurpleTheme, ClassicTheme, HighContrastTheme, OxygenUIThemeProvider } from '@wso2/oxygen-ui';
import App from './App.tsx';
import './styles.css';
import {
  ORG_HANDLE,
  OIDC_SCOPE,
  OIDC_REDIRECT_URI,
  OIDC_POST_LOGOUT_REDIRECT_URI,
} from './config.env';
import { OIDCAppAuthProvider } from './contexts/OIDCAppAuthProvider';
import { fetchOrgAuthConfig, type OrgAuthConfig } from './apis/authApi';

function OIDCBootstrap({ children }: { children: React.ReactNode }) {
  const [authConfig, setAuthConfig] = useState<OrgAuthConfig | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchOrgAuthConfig(ORG_HANDLE)
      .then(setAuthConfig)
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : 'Failed to load auth configuration')
      );
  }, []);

  if (error) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
        <p style={{ color: 'red' }}>Authentication configuration error: {error}</p>
      </div>
    );
  }

  if (!authConfig) {
    return null;
  }

  return (
    <AuthProvider
      authority={authConfig.issuer}
      client_id={authConfig.client_id}
      redirect_uri={OIDC_REDIRECT_URI}
      post_logout_redirect_uri={OIDC_POST_LOGOUT_REDIRECT_URI}
      scope={OIDC_SCOPE}
      extraTokenParams={{ scope: OIDC_SCOPE }}
      loadUserInfo={true}
      metadata={authConfig.logout_url ? { end_session_endpoint: authConfig.logout_url } : undefined}
      onSigninCallback={() => {
        window.history.replaceState({}, document.title, window.location.pathname);
      }}
    >
      <OIDCAppAuthProvider>
        <IntlProvider locale="en" defaultLocale="en">
          {children}
        </IntlProvider>
      </OIDCAppAuthProvider>
    </AuthProvider>
  );
}

const container = document.getElementById('root')!;
const root = createRoot(container);

root.render(
  <React.StrictMode>
    <OxygenUIThemeProvider
      themes={[
        {
          key: 'acrylicOrange',
          label: 'Acrylic Orange Theme',
          theme: AcrylicOrangeTheme,
        },
        {
          key: 'acrylicPurple',
          label: 'Acrylic Purple Theme',
          theme: AcrylicPurpleTheme,
        },
        {
          key: 'highContrast',
          label: 'High Contrast Theme',
          theme: HighContrastTheme,
        },
        { key: 'classic', label: 'Classic Theme', theme: ClassicTheme },
      ]}
      initialTheme="acrylicOrange"
    >
      <OIDCBootstrap>
        <App />
      </OIDCBootstrap>
    </OxygenUIThemeProvider>
  </React.StrictMode>
);
