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

module.exports = defineConfig({
    e2e: {
        baseUrl: process.env.CYPRESS_BASE_URL || 'https://localhost:3000',
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
        setupNodeEvents(on) {
            // Pass required flags to Chrome/Chromium in Docker (no sandbox, no GPU)
            on('before:browser:launch', (browser, launchOptions) => {
                if (browser.family === 'chromium') {
                    launchOptions.args.push('--no-sandbox');
                    launchOptions.args.push('--disable-gpu');
                    launchOptions.args.push('--disable-dev-shm-usage');
                }
                return launchOptions;
            });
        },
    },
    env: {
        // Org/view used throughout tests — matches seed data in 02_seed_default.sql
        ORG_HANDLE: 'ACME',
        VIEW_NAME: 'default',
        ORG_ID: '1ba42a09-45c0-40f8-a1bf-e4aa7cde1575',
        ADMIN_USER: 'admin',
        ADMIN_PASSWORD: 'admin',
    },
});
