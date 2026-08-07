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

// Manage Keys with MORE THAN ONE key manager in the organization.
//
// manage-keys.hbs renders one KM card per enabled key manager per key type, and
// keys-token.hbs one result modal per the same. Ids used to be keyed on the key type
// alone, so a second key manager produced duplicate ids and every
// document.getElementById in oauth2-key-generation.js resolved to the FIRST-rendered
// key manager. Generating a token from the second card then wrote the token into that
// card's modal but opened the first card's — which reads as "generate access token
// fails" — and spinners, errors and scope chips all landed on the wrong card. With
// only one key manager configured nothing collides, which is why the single-KM spec
// (application-flows.cy.js) never caught it.
//
// Everything here therefore asserts the KM the interaction *came from*, never just
// the key type.

describe('Applications — multiple key managers', () => {
    const APP = 'IT Multi KM App';
    // Alphabetical handles, because the cards render in whatever order the DAO
    // returns: 'alpha' being first makes "the second card" unambiguous below.
    const KM_A = { id: 'it-km-alpha', displayName: 'IT KM Alpha' };
    const KM_B = { id: 'it-km-beta', displayName: 'IT KM Beta' };
    const CLIENT_ID_A = 'it-client-id-alpha';
    const CLIENT_ID_B = 'it-client-id-beta';

    let appHandle;
    let mockToken;

    before(() => {
        cy.login();
        cy.createApplication(APP, 'Used by the multiple-key-manager tests')
            .then((handle) => { appHandle = handle; });
        // One mock token endpoint serves both key managers — the portal reaches it
        // server-side, and which KM a token came from is established by the card the
        // click started in, not by the endpoint.
        cy.task('startMockTokenServer').then((mock) => {
            mockToken = mock;
            cy.seedKeyManager({ ...KM_A, tokenEndpoint: mock.endpoint });
            cy.seedKeyManager({ ...KM_B, tokenEndpoint: mock.endpoint });
        });
    });

    after(() => {
        cy.login();
        cy.request({
            method: 'DELETE',
            url: `${Cypress.env('BASE_PATH')}/api/v0.9/applications/${appHandle}`,
            failOnStatusCode: false,
        }).its('status').should('be.oneOf', [200, 404]);
        cy.deleteKeyManager(KM_A.id);
        cy.deleteKeyManager(KM_B.id);
        cy.task('stopMockTokenServer');
    });

    it('renders one card per key manager, with no duplicate element ids', () => {
        cy.login();
        cy.visitPortal(`/applications/${appHandle}`);

        cy.get('#production').find('.mk-km-card').should('have.length', 2);
        cy.get('.mk-km-name').should('contain', KM_A.displayName);
        cy.get('.mk-km-name').should('contain', KM_B.displayName);

        // Each card owns its own generate button and result modal.
        cy.get(`#tokenKeyBtn-${KM_A.id}-PRODUCTION`).should('exist');
        cy.get(`#tokenKeyBtn-${KM_B.id}-PRODUCTION`).should('exist');
        cy.get(`#keysTokenModal-${KM_A.id}-PRODUCTION`).should('exist');
        cy.get(`#keysTokenModal-${KM_B.id}-PRODUCTION`).should('exist');

        // The regression guard proper: any id repeated on this page is a getElementById
        // that silently resolves to whichever key manager happens to render first.
        cy.document().then((doc) => {
            const ids = Array.from(doc.querySelectorAll('[id]')).map((el) => el.id);
            const duplicates = [...new Set(ids.filter((id, i) => ids.indexOf(id) !== i))];
            expect(duplicates, 'duplicate element ids on the application page').to.deep.equal([]);
        });
    });

    it('links a separate client ID to each key manager', () => {
        cy.login();
        cy.visitPortal(`/applications/${appHandle}`);

        cy.get(`#addClientIdInput-${KM_A.id}-PRODUCTION`).type(CLIENT_ID_A);
        cy.get(`#addClientIdBtn-${KM_A.id}-PRODUCTION`).click();
        cy.get(`#consumer-key-${KM_A.id}-PRODUCTION-view`, { timeout: 15000 })
            .should('have.value', CLIENT_ID_A);

        cy.get(`#addClientIdInput-${KM_B.id}-PRODUCTION`).type(CLIENT_ID_B);
        cy.get(`#addClientIdBtn-${KM_B.id}-PRODUCTION`).click();
        cy.get(`#consumer-key-${KM_B.id}-PRODUCTION-view`, { timeout: 15000 })
            .should('have.value', CLIENT_ID_B);

        // Each card keeps its own credentials — the first card's client id must not
        // have been overwritten by, or copied onto, the second.
        cy.get(`#consumer-key-${KM_A.id}-PRODUCTION-view`).should('have.value', CLIENT_ID_A);
        // appRefId used to be appKeyMappings[0].asClientId for every card.
        cy.get(`#app-ref-${KM_A.id}-PRODUCTION`).should('have.value', CLIENT_ID_A);
        cy.get(`#app-ref-${KM_B.id}-PRODUCTION`).should('have.value', CLIENT_ID_B);
        // Distinct mappings, so distinct mapping ids.
        cy.get(`#key-map-${KM_A.id}-PRODUCTION`).invoke('val').then((mappingA) => {
            cy.get(`#key-map-${KM_B.id}-PRODUCTION`).invoke('val').should('not.eq', mappingA);
        });
    });

    it('generates a token from the second key manager into that key manager\'s own modal', () => {
        cy.login();
        cy.visitPortal(`/applications/${appHandle}`);

        cy.get(`#tab-btn-token-${KM_B.id}-PRODUCTION`).click();
        cy.get(`#tokenKeyBtn-${KM_B.id}-PRODUCTION`).click();
        cy.get('#generateTokenPromptModal').should('be.visible');
        cy.get('#generateTokenPromptSecretInput').type(mockToken.secret);
        cy.get('#generateTokenPromptConfirmBtn').click();

        // The token lands in the second key manager's modal, and that modal is the one
        // shown. Before the fix the token was written here but the FIRST key manager's
        // (empty) modal was opened instead.
        cy.get(`#keysTokenModal-${KM_B.id}-PRODUCTION`, { timeout: 15000 })
            .should('be.visible');
        cy.get(`#token_${KM_B.id}_PRODUCTION`, { timeout: 15000 })
            .should('contain', mockToken.accessToken);

        // The other key manager's modal stays closed and empty.
        cy.get(`#keysTokenModal-${KM_A.id}-PRODUCTION`).should('not.be.visible');
        cy.get(`#token_${KM_A.id}_PRODUCTION`).should('not.contain', mockToken.accessToken);

        // The clicked button returns to its normal state (the spinner used to be
        // applied to, and left on, the first card's button).
        cy.get(`[data-cyid="keysTokenModal-${KM_B.id}-PRODUCTION-close"]`).click();
        cy.get(`#tokenKeyBtn-${KM_B.id}-PRODUCTION`)
            .should('not.be.disabled')
            .find('.button-normal-state').should('be.visible');
    });

    it('generates a token from the first key manager as well', () => {
        cy.login();
        cy.visitPortal(`/applications/${appHandle}`);

        cy.get(`#tab-btn-token-${KM_A.id}-PRODUCTION`).click();
        cy.get(`#tokenKeyBtn-${KM_A.id}-PRODUCTION`).click();
        cy.get('#generateTokenPromptModal').should('be.visible');
        cy.get('#generateTokenPromptSecretInput').type(mockToken.secret);
        cy.get('#generateTokenPromptConfirmBtn').click();

        cy.get(`#keysTokenModal-${KM_A.id}-PRODUCTION`, { timeout: 15000 })
            .should('be.visible');
        cy.get(`#token_${KM_A.id}_PRODUCTION`, { timeout: 15000 })
            .should('contain', mockToken.accessToken);
        cy.get(`#keysTokenModal-${KM_B.id}-PRODUCTION`).should('not.be.visible');
        cy.get(`[data-cyid="keysTokenModal-${KM_A.id}-PRODUCTION-close"]`).click();
    });

    it('shows a token failure in the card it came from, not the first one', () => {
        cy.login();
        cy.visitPortal(`/applications/${appHandle}`);

        // The mock rejects any secret but its own, so this is a genuine upstream 401
        // surfaced through the portal.
        cy.get(`#tab-btn-token-${KM_B.id}-PRODUCTION`).click();
        cy.get(`#tokenKeyBtn-${KM_B.id}-PRODUCTION`).click();
        cy.get('#generateTokenPromptSecretInput').type('it-wrong-secret');
        cy.get('#generateTokenPromptConfirmBtn').click();

        cy.get(`#keyGenerationErrorContainer-${KM_B.id}-PRODUCTION`, { timeout: 15000 })
            .should('be.visible')
            .and('contain', 'Failed to generate access token');
        cy.get(`#keyGenerationErrorContainer-${KM_A.id}-PRODUCTION`).should('not.be.visible');
        cy.get(`#keysTokenModal-${KM_B.id}-PRODUCTION`).should('not.be.visible');
    });

    it('revokes one key manager\'s keys without touching the other\'s', () => {
        cy.login();
        cy.visitPortal(`/applications/${appHandle}`);

        cy.get(`#tab-btn-creds-${KM_B.id}-PRODUCTION`).click();
        // Scope the revoke click to the second card — every card renders its own.
        cy.get(`#keyActionsContainer-${KM_B.id}-PRODUCTION`).find('.mk-btn-danger').click();
        cy.get('#deleteConfirmation').should('be.visible');
        cy.get('#deleteConfirmationBtn').click();

        // The second key manager is back to the empty state; the first still holds its
        // own client id (the unscoped fallback used to delete the first card's mapping).
        cy.get(`#addClientIdBtn-${KM_B.id}-PRODUCTION`, { timeout: 15000 }).should('exist');
        cy.get(`#consumer-key-${KM_B.id}-PRODUCTION-view`).should('have.value', '');
        cy.get(`#consumer-key-${KM_A.id}-PRODUCTION-view`).should('have.value', CLIENT_ID_A);
    });
});
