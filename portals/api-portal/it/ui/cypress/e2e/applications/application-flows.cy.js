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

// Applications require login. This spec covers the application lifecycle and the
// key association sections on the detail page.
//
// Key generation itself is no longer tested here. It used to live on this page as
// a per-key-manager "Manage Keys" card; that flow was removed when OAuth2 key
// generation moved to its own page, and a key manager is no longer bound to an
// application. The replacement is covered by the oauth2-keys specs.

describe('Applications', () => {
    const DETAIL_APP = 'IT Detail App';
    let detailHandle;

    before(() => {
        // A persistent, admin-owned application reused by the detail tests.
        cy.login();
        cy.createApplication(DETAIL_APP, 'Used by the application detail tests')
            .then((handle) => { detailHandle = handle; });
    });

    after(() => {
        // Delete via the management API using the logged-in admin session — apps
        // are created_by-scoped, so this must run as admin (not the api-key path),
        // and there is no CSRF on the applications endpoints. Robust cleanup that
        // doesn't depend on the delete-modal UI.
        cy.login();
        cy.request({
            method: 'DELETE',
            url: `${Cypress.env('BASE_PATH')}/api/v0.9/applications/${detailHandle}`,
            failOnStatusCode: false,
        }).its('status').should('be.oneOf', [200, 404]); // 200 = deleted; 404 = already gone (idempotent). Any other status is a real failure.
    });

    it('creates, edits, and deletes an application', () => {
        const NAME = 'IT CRUD App';
        const RENAMED = 'IT CRUD App Renamed';
        cy.login();

        // Create.
        cy.createApplication(NAME, 'Created by the CRUD test');
        cy.contains('.app-card-name', NAME).should('be.visible');

        // Open the detail page and rename it (inline contenteditable edit).
        cy.contains('.app-card', NAME).click();
        cy.url().should('include', '/applications/');
        cy.get('#applicationName').should('contain', NAME);
        cy.get('#editNameBtn').click();
        cy.get('#applicationName')
            .should('have.attr', 'contenteditable', 'true')
            .type('{selectall}' + RENAMED);
        cy.get('#saveNameBtn').click();
        cy.get('#applicationName').should('contain', RENAMED);

        // Edit the description (also an inline contenteditable edit).
        const NEW_DESC = 'Updated by the CRUD test';
        cy.get('#editDescriptionBtn').click();
        cy.get('#applicationDescription')
            .should('have.attr', 'contenteditable', 'true')
            .type('{selectall}' + NEW_DESC);
        cy.get('#saveDescriptionBtn').click();
        cy.get('#applicationDescription').should('contain', NEW_DESC);

        // Delete (the card now carries the renamed name).
        cy.deleteApplication(RENAMED);
        cy.contains('.app-card-name', RENAMED).should('not.exist');
    });

    it('shows the key association sections on the application detail page', () => {
        cy.login();
        cy.visitPortal(`/applications/${detailHandle}`);

        // Both sections are association surfaces: a key is created elsewhere (the
        // API Keys and OAuth2 Keys pages) and attached to an application here.
        cy.get('.ak-title').should('exist');
        cy.get('#btn-open-associate-key').should('exist');
    });

});
