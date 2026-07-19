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

// Global support file — imported before every spec.
// Import custom commands so they are available in all specs.
import './commands';

// Resolve the default org UUID from the API at runtime so tests are not
// coupled to a hardcoded UUID that changes between database backends.
before(() => {
    const apiKey = Cypress.env('API_KEY');
    cy.request({
        method: 'GET',
        // Must include the API base path — the devportal API router (see
        // src/routes/api/devportalApiRouter.js) only recognizes requests whose
        // first path segment matches the OpenAPI spec's server basePath ('api').
        // A bare '/organizations' falls through to the page-rendering route
        // tree instead, which misinterprets "organizations" as an org handle.
        url: '/api/v0.9/organizations',
        headers: apiKey ? { 'x-wso2-api-key': apiKey } : {},
        failOnStatusCode: false,
    }).then((resp) => {
        if (resp.status === 200 && resp.body && resp.body.list && resp.body.list.length > 0) {
            Cypress.env('ORG_ID', resp.body.list[0].orgId);
        }
    });
});
