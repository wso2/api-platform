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

// Applications are user-scoped (created_by), so they must be created through the
// UI as the logged-in user — a resource seeded via the service API key would be
// owned by a different actor and never appear on the user's Applications page.
// Call these only after cy.login().

// ---------------------------------------------------------------------------
// cy.createApplication(name, description)
//   Create an application via the Applications page create modal. Waits for the
//   new card and yields the created application's handle (the card's data-id).
// ---------------------------------------------------------------------------
Cypress.Commands.add('createApplication', (name, description = '') => {
    cy.visitPortal('/applications');
    // Header button (apps exist) or empty-state button (zero apps) — only one renders.
    cy.get('#apps-create-btn, #apps-create-btn-empty').first().click();
    cy.get('#app-create-modal').should('be.visible');
    cy.get('#app-name-input').clear().type(name);
    if (description) {
        cy.get('#app-desc-input').clear().type(description);
    }
    cy.get('#app-create-confirm').should('not.be.disabled').click();
    // The create request reloads the page; wait for the new card, then yield its
    // handle. Match the name exactly against .app-card-name (not a substring of the
    // whole .app-card) so e.g. "IT CRUD App" doesn't also match "IT CRUD App Renamed".
    return cy
        .contains('.app-card-name', new RegExp(`^${Cypress._.escapeRegExp(name)}$`), { timeout: 15000 })
        .should('be.visible')
        .closest('.app-card')
        .invoke('attr', 'data-id');
});

// ---------------------------------------------------------------------------
// cy.deleteApplication(name)
//   Best-effort delete via the Applications page — a no-op if the app is already
//   gone, so it is safe to call from an after() hook.
// ---------------------------------------------------------------------------
Cypress.Commands.add('deleteApplication', (name) => {
    cy.visitPortal('/applications');
    cy.get('body').then(($body) => {
        // Exact-match the card name, not a substring of the whole card, so a
        // similarly-named app (e.g. "IT CRUD App" vs "IT CRUD App Renamed") is
        // never picked by mistake.
        const nameCard = $body
            .find('.app-card-name')
            .filter((_, el) => el.textContent.trim() === name);
        if (!nameCard.length) {
            return; // Already deleted.
        }
        cy.wrap(nameCard).first().closest('.app-card').find('.app-delete-btn').click();
        cy.get('#app-delete-modal').should('be.visible');
        cy.get('#app-delete-confirm').click();
        cy.contains('.app-card-name', name).should('not.exist');
    });
});
