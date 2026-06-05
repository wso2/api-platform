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

import React from 'react';
import { createRoot } from 'react-dom/client';
import { IntlProvider } from 'react-intl';
import { AuthProvider } from 'react-oidc-context';
import { AcrylicOrangeTheme, AcrylicPurpleTheme, ClassicTheme, HighContrastTheme, OxygenUIThemeProvider } from '@wso2/oxygen-ui';
import App from './App.tsx';
import './styles.css';
import {
  OIDC_AUTHORITY,
  OIDC_CLIENT_ID,
  OIDC_REDIRECT_URI,
  OIDC_POST_LOGOUT_REDIRECT_URI,
  OIDC_SCOPE,
  OIDC_END_SESSION_ENDPOINT,
} from './config.env';
import { OIDCAppAuthProvider } from './contexts/OIDCAppAuthProvider';

const container = document.getElementById('root')!;
const root = createRoot(container);

const themeConfig = (
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
    <AuthProvider
      authority={OIDC_AUTHORITY}
      client_id={OIDC_CLIENT_ID}
      redirect_uri={OIDC_REDIRECT_URI}
      post_logout_redirect_uri={OIDC_POST_LOGOUT_REDIRECT_URI}
      scope={OIDC_SCOPE}
      extraTokenParams={{ scope: OIDC_SCOPE }}
      loadUserInfo={true}
      metadata={OIDC_END_SESSION_ENDPOINT ? { end_session_endpoint: OIDC_END_SESSION_ENDPOINT } : undefined}
      onSigninCallback={() => {
        window.history.replaceState({}, document.title, window.location.pathname);
      }}
    >
      <OIDCAppAuthProvider>
        <IntlProvider locale="en" defaultLocale="en">
          <App />
        </IntlProvider>
      </OIDCAppAuthProvider>
    </AuthProvider>
  </OxygenUIThemeProvider>
);

root.render(
  <React.StrictMode>
    {themeConfig}
  </React.StrictMode>
);
