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
// manager offered to them on the OAuth2 Keys page, because an org key manager now
// exists. Key generation used to be asserted on the application detail page; that
// flow was removed when a key manager stopped being bound to an application.

describe('Settings — Key Managers', () => {
    // Not crypto.randomUUID(): Cypress runs specs against http://api-portal:9543, an
    // insecure context where the WebCrypto API is unavailable. Date.now() + a random
    // suffix is unique across (serial) runs.
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
    const KM_NAME = `IT KM ${uid}`;
    const ENDPOINT = 'https://idp.example.invalid/oauth2/token';
    const APP_NAME = `IT KM App ${uid}`;

    const settingsUrl = () => `${Cypress.env('BASE_PATH')}/${Cypress.env('ORG_HANDLE')}/settings`;

    after(() => {
        // Remove the developer-owned application (via its owner) first.
        cy.clearCookies();
        cy.login('developer', 'developer');
        cy.deleteApplication(APP_NAME);
        // Then the key manager. Swap the developer session for an admin one — the
        // REST call authorizes as whoever is logged in, and `developer` lacks
        // dp:key_manager:read, so it would 403.
        // The handle is a server-generated UUID, so discover it by display name.
        cy.clearCookies();
        cy.login();
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
        // "In the identity server": this key manager only proxies token requests for
        // applications registered at the identity server, so it needs no
        // registration endpoint or provisioning credential. The form opens on
        // "In the portal", which now refuses a blank registration block rather
        // than silently saving an importing key manager — so the mode has to be
        // chosen explicitly for a token-proxy-only key manager like this one.
        cy.get('#km-mode-import').check();
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

        // A developer can list the enabled key managers (public, credential-free view) —
        // dp:application_key_mapping:read grants read access; the request is authorized by the
        // developer session (which takes precedence over the API-key header).
        cy.apiRequest('GET', '/api/v0.9/key-managers').then((res) => {
            expect(res.status, 'developer can read key managers').to.eq(200);
            expect((res.body.list || []).some((km) => km.displayName === KM_NAME),
                'developer sees the enabled key manager').to.be.true;
        });

        cy.createApplication(APP_NAME, 'App for the key-manager key-generation test');

        // 4. Key generation is now available to the developer. It no longer lives on
        //    the application page — a key manager is not bound to an application —
        //    so the check is that the OAuth2 Keys page offers this key manager as a
        //    choice, which is what "enabled for a developer" now means.
        cy.visitPortal('/oauth2-keys');
        cy.get('#ok-add-btn, #ok-add-btn-empty').first().click();
        cy.get('#ok-add-modal').should('be.visible');
        cy.get('#ok-km-select').should('contain', KM_NAME);
    });
});
