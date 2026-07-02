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

export default {
  e2e: {
    baseUrl: process.env.CYPRESS_BASE_URL || 'https://localhost:5380',
    specPattern: 'cypress/e2e/**/*.cy.js',
    supportFile: 'cypress/support/e2e.js',
    videosFolder: 'reports/videos',
    screenshotsFolder: 'reports/screenshots',
    video: true,
    screenshotOnRunFailure: true,
    defaultCommandTimeout: 10000,
    requestTimeout: 15000,
    responseTimeout: 15000,
    chromeWebSecurity: false,
    setupNodeEvents(on) {
      on('before:browser:launch', (browser, launchOptions) => {
        if (browser.family === 'chromium') {
          launchOptions.args.push('--ignore-certificate-errors');
          launchOptions.args.push('--allow-insecure-localhost');
          launchOptions.args.push('--no-sandbox');
          launchOptions.args.push('--disable-gpu');
          launchOptions.args.push('--disable-dev-shm-usage');
        }
        return launchOptions;
      });
    },
  },
  env: {
    ADMIN_USER: 'admin',
    ADMIN_PASSWORD: 'admin',
    ORG_HANDLE: 'default',
    QS_INTRO_STORAGE_KEY: 'qs_intro_seen_v1',
  },
};
