// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

const { defineConfig } = require('cypress');
const { startMockTokenServer, stopMockTokenServer } = require('./mock-token-server');

module.exports = defineConfig({
    e2e: {
        baseUrl: process.env.CYPRESS_BASE_URL || 'https://localhost:9543',
        specPattern: 'cypress/e2e/**/*.cy.js',
        supportFile: 'cypress/support/e2e.js',
        fixturesFolder: 'cypress/fixtures',
        videosFolder: 'reports/videos',
        screenshotsFolder: 'reports/screenshots',
        video: true,
        screenshotOnRunFailure: true,
        defaultCommandTimeout: 10000,
        requestTimeout: 15000,
        responseTimeout: 15000,
        // Accept self-signed certs from the devportal
        chromeWebSecurity: false,
        setupNodeEvents(on, config) {
            // Pass required flags to Chrome/Chromium in Docker (no sandbox, no GPU)
            on('before:browser:launch', (browser, launchOptions) => {
                if (browser.family === 'chromium') {
                    launchOptions.args.push('--no-sandbox');
                    launchOptions.args.push('--disable-gpu');
                    launchOptions.args.push('--disable-dev-shm-usage');
                }
                return launchOptions;
            });

            // On-demand mock OAuth2 token endpoint for the application key/token
            // round-trip test. Registered as tasks (not started here) so it only
            // listens while a test that needs it is running — the test starts it in
            // its before() and stops it in after(); it returns { endpoint, secret,
            // accessToken } for the seeded key manager to point at.
            on('task', {
                startMockTokenServer: () => startMockTokenServer(),
                stopMockTokenServer: () => stopMockTokenServer(),
            });
            return config;
        },
    },
    env: {
        // Org/view used throughout tests. ORG_ID is resolved dynamically at runtime
        // via the before() hook in cypress/support/e2e.js.
        ORG_HANDLE: 'default',
        VIEW_NAME: 'default',
        ADMIN_USER: 'admin',
        ADMIN_PASSWORD: 'admin',
    },
});
