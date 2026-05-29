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
import { AuthProvider, AuthReactConfig } from '@asgardeo/auth-react';
import { AcrylicOrangeTheme, AcrylicPurpleTheme, ClassicTheme, HighContrastTheme, OxygenUIThemeProvider } from '@wso2/oxygen-ui';
import App from './App.tsx';
import './styles.css';
import {
  asgardeoSdkConfig,
  ASGARDEO_SDK_SCOPES,
  ASGARDEO_SDK_RESOURCE_URLS,
} from './config.env';

// Configure Asgardeo SDK
const sdkConfig = {
  ...asgardeoSdkConfig,
  // ASGARDEO_SDK_RESOURCE_URLS is a pipe-separated string, need to split into array
  resourceServerURLs: ASGARDEO_SDK_RESOURCE_URLS.split('|'),
  scope: ASGARDEO_SDK_SCOPES.split('|'),
} as AuthReactConfig;

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
      <AuthProvider config={sdkConfig}>
        <IntlProvider locale="en" defaultLocale="en">
          <App />
        </IntlProvider>
      </AuthProvider>
    </OxygenUIThemeProvider>
  </React.StrictMode>
);
