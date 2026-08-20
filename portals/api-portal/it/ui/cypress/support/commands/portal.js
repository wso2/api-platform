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

// Generic portal navigation and management-API helpers shared across specs.

// ---------------------------------------------------------------------------
// cy.portalUrl(path)
//   Build a URL under the ACME/default view without hardcoding the base path.
// ---------------------------------------------------------------------------
Cypress.Commands.add('portalUrl', (path = '') => {
    const basePath = Cypress.env('BASE_PATH');
    const orgHandle = Cypress.env('ORG_HANDLE');
    const viewName = Cypress.env('VIEW_NAME');
    return `${basePath}/${orgHandle}/views/${viewName}${path}`;
});

// ---------------------------------------------------------------------------
// cy.apiRequest(method, path, options)
//   Call a portal REST endpoint as whichever user is currently logged in.
//
//   The portal's static service API key (x-wso2-api-key) was removed, so these
//   calls authenticate with the ordinary session cookie — cy.request shares the
//   browser's cookie jar — plus the X-CSRF-Token header that csrfProtection
//   requires on mutating cookie-authenticated requests (double-submit of the
//   XSRF-TOKEN cookie the server sets on every response).
//
//   Callers must have established a session first: testIsolation clears cookies
//   between tests, so before()/after() hooks need their own cy.login().
// ---------------------------------------------------------------------------
Cypress.Commands.add('apiRequest', (method, path, options = {}) => {
    // Callers pass a bare REST path (e.g. "/api/v0.9/apis"); the portal is mounted under
    // BASE_PATH, so prepend it here — mirroring the server's route mount.
    const basePath = Cypress.env('BASE_PATH');
    return cy.getCookie('XSRF-TOKEN').then((csrf) => cy.request({
        method,
        url: `${basePath}${path}`,
        failOnStatusCode: options.failOnStatusCode !== false,
        ...options,
        headers: {
            ...(csrf && csrf.value ? { 'X-CSRF-Token': decodeURIComponent(csrf.value) } : {}),
            ...(options.headers || {}),
        },
    }));
});

// ---------------------------------------------------------------------------
// cy.visitPortal(path)
//   Navigate to a path inside the ACME/default portal view.
// ---------------------------------------------------------------------------
Cypress.Commands.add('visitPortal', (path = '') => {
    const basePath = Cypress.env('BASE_PATH');
    const orgHandle = Cypress.env('ORG_HANDLE');
    const viewName = Cypress.env('VIEW_NAME');
    cy.visit(`${basePath}/${orgHandle}/views/${viewName}${path}`);
});
