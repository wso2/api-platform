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

// End-to-end for the Key Manager form: an admin creates a key manager (the form
// has no Handle field — the server generates a UUID handle since the UI sends no id),
// the creation is confirmed over the REST API, and a developer then sees key
// generation enabled on their application because an org key manager now exists.

describe('Settings — Key Managers', () => {
    // Not crypto.randomUUID(): Cypress runs specs against http://devportal:9543, an
    // insecure context where the WebCrypto API is unavailable. Date.now() + a random
    // suffix is unique across (serial) runs.
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
    const KM_NAME = `IT KM ${uid}`;
    const ENDPOINT = 'https://idp.example.invalid/oauth2/token';
    const APP_NAME = `IT KM App ${uid}`;

    const settingsUrl = () => `/${Cypress.env('ORG_HANDLE')}/settings`;

    after(() => {
        // Remove the developer-owned application (via its owner) and the key manager.
        cy.clearCookies();
        cy.login('developer', 'developer');
        cy.deleteApplication(APP_NAME);
        // The handle is a server-generated UUID, so discover it by display name and
        // delete via the admin management API key (independent of session).
        cy.apiRequest('GET', '/api/v0.9/key-managers').then((res) => {
            (res.body.list || [])
                .filter((km) => km.displayName === KM_NAME)
                .forEach((km) => cy.apiRequest('DELETE', `/api/v0.9/key-managers/${km.id}`, { failOnStatusCode: false }));
        });
    });

    it('creates a key manager and enables key generation for a developer application', () => {
        // 1. Admin creates the key manager from the Settings UI (no Handle field).
        cy.login();
        cy.visit(settingsUrl());
        cy.get('.cfg-nav-item[data-panel="cfg-keymanagers"]').click();
        cy.get('#cfg-add-km-btn').click();
        cy.get('#cfg-km-modal').should('be.visible');
        cy.get('#km-display').type(KM_NAME);
        cy.get('#km-token-endpoint').type(ENDPOINT);
        cy.get('#cfg-km-modal-save').click();
        cy.contains('.cfg-km-edit-btn', KM_NAME, { timeout: 15000 }).should('exist');

        // 2. Confirm it was created over the REST API, with a server-generated handle.
        cy.apiRequest('GET', '/api/v0.9/key-managers').then((res) => {
            const match = (res.body.list || []).find((km) => km.displayName === KM_NAME);
            expect(match, 'created key manager present in list').to.exist;
            expect(match.id, 'server generated a handle').to.be.a('string').and.not.be.empty;
        });

        // 3. Switch to a developer user and create an application.
        cy.clearCookies();
        cy.login('developer', 'developer');
        cy.createApplication(APP_NAME, 'App for the key-manager key-generation test');

        // 4. On the application, key generation is now enabled: the "unavailable"
        //    message is gone and the key-manager card (with its key controls) renders.
        cy.visitPortal('/applications');
        cy.contains('.app-card-name', APP_NAME).click();
        cy.url().should('include', '/applications/');
        cy.get('.mk-unavailable').should('not.exist');
        cy.get('.mk-km-card').should('exist');
        cy.get('[id^="addClientIdBtn-"]').should('exist');
    });
});
