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
    // Labels are optional on a view, but this test exercises attaching one from the
    // picker, so it needs an existing label to click. Seed one up front, distinct from
    // the label the label test creates.
    const VIEW_LABEL = `it-vlabel-${uid}`;
    // The rename test's own view, seeded through the API so it doesn't depend on the
    // create test having run.
    const RENAME_FROM = `it-rename-from-${uid}`;
    const RENAME_TO = `it-rename-to-${uid}`;

    const settingsUrl = () => `${Cypress.env('BASE_PATH')}/${Cypress.env('ORG_HANDLE')}/settings`;

    before(() => {
        cy.login();
        cy.apiRequest('POST', '/api/v0.9/labels', { body: { id: VIEW_LABEL, displayName: 'IT View Label' } });
    });

    after(() => {
        // Robust API cleanup, idempotent (404 if the create step never persisted).
        cy.login();
        cy.apiRequest('DELETE', `/api/v0.9/views/${VIEW_HANDLE}`, { failOnStatusCode: false });
        // Whichever handle the rename test left behind.
        cy.apiRequest('DELETE', `/api/v0.9/views/${RENAME_TO}`, { failOnStatusCode: false });
        cy.apiRequest('DELETE', `/api/v0.9/views/${RENAME_FROM}`, { failOnStatusCode: false });
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

    it('renames a view handle from the edit modal', () => {
        // A handle used to be read-only after creation. It is editable now: a rename
        // keeps the view's identity (labels, assets and workflows are keyed on its uuid)
        // and only changes its URL, which the modal warns about.
        cy.login();
        cy.apiRequest('POST', '/api/v0.9/views', {
            body: { id: RENAME_FROM, displayName: 'IT Rename View', labels: [VIEW_LABEL] },
        });
        cy.visit(settingsUrl());

        cy.get('.cfg-nav-item[data-panel="cfg-views"]').click();
        cy.get(`#cfg-view-row-${RENAME_FROM} .cfg-view-edit-btn`).first().click();
        cy.get('#cfg-view-modal').should('be.visible');

        // Editable, pre-filled with the current handle, and the rename warning names the
        // URL that is about to stop working.
        // Two statements, not a chain: `should('not.have.attr', …)` yields the attribute
        // value (undefined), so a chained `.and('have.value', …)` would assert against
        // that instead of the element.
        cy.get('#view-handle').should('not.have.attr', 'readonly');
        cy.get('#view-handle').should('have.value', RENAME_FROM);
        cy.get('#view-handle-rename-warning').should('be.visible');
        cy.get('#view-handle-rename-old').should('contain', `/views/${RENAME_FROM}`);

        cy.get('#view-handle').clear().type(RENAME_TO);
        cy.get('#cfg-view-modal-save').click();

        // The row is keyed on the handle, so the new one appearing (and the old one
        // gone) is the rename landing.
        cy.get(`#cfg-view-row-${RENAME_TO}`, { timeout: 15000 }).should('exist').and('contain', RENAME_TO);
        cy.get(`#cfg-view-row-${RENAME_FROM}`).should('not.exist');

        // Same view, so the label attached before the rename is still attached. The row
        // renders label DISPLAY names (viewConfigureController maps handle → name), not
        // the handles.
        cy.get(`#cfg-view-row-${RENAME_TO}`).should('contain', 'IT View Label');

        // And the view is served at its new URL (visitPortal is pinned to the fixture's
        // own view, so this navigates explicitly).
        cy.visit(`${Cypress.env('BASE_PATH')}/${Cypress.env('ORG_HANDLE')}/views/${RENAME_TO}`);
        cy.url().should('include', `/views/${RENAME_TO}`);
    });

    it('shows a delete control for the seeded default view', () => {
        // The default view used to render no delete button at all. Any view is deletable
        // now — only the last one is held back, enforced server-side with a 400 — so the
        // control must be present for 'default' too.
        cy.login();
        cy.visit(settingsUrl());
        cy.get('.cfg-nav-item[data-panel="cfg-views"]').click();

        // More than one view exists at this point (the seeded 'default' plus this
        // spec's), so the control is present and enabled.
        cy.get('#cfg-view-row-default .cfg-view-delete-btn')
            .should('exist')
            .and('not.be.disabled');
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
