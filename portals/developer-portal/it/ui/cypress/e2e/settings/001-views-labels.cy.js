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

// Happy-path coverage for creating Views and Labels from Settings. The normal
// flow is: type a name, let the handle auto-generate, and save.

describe('Settings — Views & Labels', () => {
    // Not crypto.randomUUID(): Cypress runs specs against http://api-portal:9543, an
    // insecure context where the WebCrypto API is unavailable. Date.now() + a random
    // suffix is unique across (serial) runs and stays slug-safe ([0-9a-z-]).
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
    const VIEW_NAME = `IT View ${uid}`;
    const VIEW_HANDLE = `it-view-${uid}`; // slugify(VIEW_NAME)
    const LABEL_DISPLAY = `IT Label ${uid}`;
    const LABEL_HANDLE = `it-label-${uid}`; // slugify(LABEL_DISPLAY)
    // A view requires at least one label (ViewCreateRequest.labels has minItems: 1), so
    // the view test needs an existing label to pick. Seed one up front, distinct from the
    // label the label test creates.
    const VIEW_LABEL = `it-vlabel-${uid}`;

    const settingsUrl = () => `/${Cypress.env('ORG_HANDLE')}/settings`;

    before(() => {
        cy.login();
        cy.apiRequest('POST', '/api/v0.9/labels', { body: { id: VIEW_LABEL, displayName: 'IT View Label' } });
    });

    after(() => {
        // Robust API cleanup, idempotent (404 if the create step never persisted).
        cy.apiRequest('DELETE', `/api/v0.9/views/${VIEW_HANDLE}`, { failOnStatusCode: false });
        cy.apiRequest('DELETE', `/api/v0.9/labels/${LABEL_HANDLE}`, { failOnStatusCode: false });
        cy.apiRequest('DELETE', `/api/v0.9/labels/${VIEW_LABEL}`, { failOnStatusCode: false });
    });

    it('creates a view from a name', () => {
        cy.login();
        cy.visit(settingsUrl());

        cy.get('.cfg-nav-item[data-panel="cfg-views"]').click();
        cy.get('#cfg-add-view-btn').click();
        cy.get('#cfg-view-modal').should('be.visible');

        // Type the name; the handle auto-generates from it.
        cy.get('#view-display').type(VIEW_NAME);
        cy.get('#view-handle').should('have.value', VIEW_HANDLE);

        // A view must have at least one label — pick the seeded one.
        cy.get(`#view-labels .cfg-label-toggle[data-value="${VIEW_LABEL}"]`).click();

        cy.get('#cfg-view-modal-save').click();

        // The modal reloads the page on success; the new row is rendered server-side.
        cy.get(`#cfg-view-row-${VIEW_HANDLE}`, { timeout: 15000 })
            .should('exist')
            .and('contain', VIEW_HANDLE)
            .and('contain', VIEW_NAME);
    });

    it('creates a label from a display name', () => {
        cy.login();
        cy.visit(settingsUrl());

        cy.get('.cfg-nav-item[data-panel="cfg-labels"]').click();
        cy.get('#cfg-add-label-btn').click();
        cy.get('#cfg-label-modal').should('be.visible');

        // Type the display name; the identifier (name/handle) auto-generates from it.
        cy.get('#lbl-display').type(LABEL_DISPLAY);
        cy.get('#lbl-name').should('have.value', LABEL_HANDLE);

        cy.get('#cfg-label-modal-save').click();

        cy.get(`#cfg-label-row-${LABEL_HANDLE}`, { timeout: 15000 })
            .should('exist')
            .and('contain', LABEL_HANDLE)
            .and('contain', LABEL_DISPLAY);
    });
});
