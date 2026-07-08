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

// ---------------------------------------------------------------------------
// cy.portalUrl(path)
//   Build a URL under the ACME/default view without hardcoding the base path.
// ---------------------------------------------------------------------------
Cypress.Commands.add('portalUrl', (path = '') => {
    const orgHandle = Cypress.env('ORG_HANDLE');
    const viewName = Cypress.env('VIEW_NAME');
    return `/${orgHandle}/views/${viewName}${path}`;
});

// ---------------------------------------------------------------------------
// cy.apiRequest(method, path, options)
//   Thin wrapper around cy.request that includes the API key header for
//   accessing admin-protected devportal endpoints in the IT environment.
// ---------------------------------------------------------------------------
Cypress.Commands.add('apiRequest', (method, path, options = {}) => {
    const apiKey = Cypress.env('API_KEY');
    const headers = apiKey
        ? { 'x-wso2-api-key': apiKey, ...(options.headers || {}) }
        : (options.headers || {});
    return cy.request({
        method,
        url: path,
        failOnStatusCode: options.failOnStatusCode !== false,
        ...options,
        headers,
    });
});

// ---------------------------------------------------------------------------
// cy.visitPortal(path)
//   Navigate to a path inside the ACME/default portal view.
// ---------------------------------------------------------------------------
Cypress.Commands.add('visitPortal', (path = '') => {
    const orgHandle = Cypress.env('ORG_HANDLE');
    const viewName = Cypress.env('VIEW_NAME');
    cy.visit(`/${orgHandle}/views/${viewName}${path}`);
});
