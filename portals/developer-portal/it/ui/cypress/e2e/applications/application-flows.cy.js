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

// Applications require login. The "Manage Keys" section depends on an org-level
// key manager: with none configured (the default) it shows an unavailable
// message; once one is seeded, the key-manager card and its key controls render.

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
            url: `/api/v0.9/applications/${detailHandle}`,
            failOnStatusCode: false,
        }).its('status').should('be.oneOf', [200, 404]); // 200 = deleted; 404 = already gone (idempotent). Any other status is a real failure.
    });

    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
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

    it('shows the "no key manager" message and the API keys association section', () => {
        cy.login();
        cy.visitPortal(`/applications/${detailHandle}`);

        // Manage Keys section — no key manager configured yet.
        cy.contains('.mk-title', 'Manage Keys').should('exist');
        cy.get('.mk-unavailable').should('exist').and('contain', 'Key generation is unavailable');
        cy.get('.mk-km-card').should('not.exist');

        // The API keys association section is independent of key managers.
        cy.get('.ak-title').should('exist');
        cy.get('#btn-open-associate-key').should('exist');
    });

    context('with a key manager configured', () => {
        // The Manage Keys card labels each key manager by its handle (kmName =
        // km.handle), so seed a known id and assert on that.
        const KM_ID = 'it-key-manager';
        // Populated from the on-demand mock token server (started in before()).
        let mockToken;

        before(() => {
            // Start the mock OAuth2 token endpoint only for this context, and point
            // the key manager at it so the token round-trip can actually resolve.
            cy.task('startMockTokenServer').then((mock) => {
                mockToken = mock;
                cy.seedKeyManager({
                    id: KM_ID,
                    displayName: 'IT Key Manager',
                    tokenEndpoint: mock.endpoint,
                });
            });
        });

        after(() => {
            cy.deleteKeyManager(KM_ID);
            cy.task('stopMockTokenServer');
        });

        it('loads the key manager section and its key controls', () => {
            cy.login();
            cy.visitPortal(`/applications/${detailHandle}`);

            // The unavailable message is gone; a key-manager card renders instead.
            cy.get('.mk-unavailable').should('not.exist');
            cy.get('.mk-km-card').should('exist');
            cy.get('.mk-km-name').should('contain', KM_ID);
            // With no credentials yet, the card exposes the "add client ID" control
            // (OAuth apps are created in the key manager itself, then linked here).
            cy.get('[id^="addClientIdBtn-"]').should('exist');
        });

        it('adds a client ID, generates a token via the key manager, then revokes it', () => {
            cy.login();
            cy.visitPortal(`/applications/${detailHandle}`);

            // 1. Add a client ID (PRODUCTION) — links the consumer key; no external call.
            cy.get(`#addClientIdInput-${KM_ID}-PRODUCTION`).type('it-client-id');
            cy.get(`#addClientIdBtn-${KM_ID}-PRODUCTION`).click();
            // The page reloads showing the linked credentials.
            cy.get(`#consumer-key-${KM_ID}-PRODUCTION-view`, { timeout: 15000 })
                .should('have.value', 'it-client-id');

            // 2. Generate a token with the consumer secret (devportal → mock key manager).
            cy.get(`#tab-btn-token-${KM_ID}-PRODUCTION`).click();
            cy.get('#tokenKeyBtn-PRODUCTION').click();
            cy.get('#generateTokenPromptModal').should('be.visible');
            cy.get('#generateTokenPromptSecretInput').type(mockToken.secret);
            cy.get('#generateTokenPromptConfirmBtn').click();
            cy.get(`#token_${KM_ID}_PRODUCTION`, { timeout: 15000 })
                .should('contain', mockToken.accessToken);
            cy.get('[data-cyid="keysTokenModal-PRODUCTION-close"]').click();

            // 3. Revoke the keys — confirm in the shared delete-confirmation modal.
            //    Scope the revoke button to the Production pane — the Sandbox card
            //    renders its own revoke button too.
            cy.get(`#tab-btn-creds-${KM_ID}-PRODUCTION`).click();
            cy.get('#production').find('.mk-btn-danger').click();
            cy.get('#deleteConfirmation').should('be.visible');
            cy.get('#deleteConfirmationBtn').click();
            // After the reload, the card is back to the empty "add client ID" state.
            // (Both the credentials and empty-state blocks are always in the DOM —
            // toggled by whether a consumer key exists — so assert the consumer key
            // value is cleared rather than that its input is gone.)
            cy.get(`#addClientIdBtn-${KM_ID}-PRODUCTION`, { timeout: 15000 }).should('exist');
            cy.get(`#consumer-key-${KM_ID}-PRODUCTION-view`).should('have.value', '');
        });
    });
});
