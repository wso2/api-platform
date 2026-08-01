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

describe('API Portal — Portal Access', () => {
    // Prereq: the default org + view are auto-seeded by the portal on startup
    // (seederService.seedDefaultOrg), but the API/MCP listings are empty until
    // something is deployed — so we seed one PUBLISHED REST API and one MCP
    // server into the default view via the management API, and tear them down
    // afterward. The REST API's definition declares an apiKey security scheme so
    // its detail page renders the auth-gated "API Keys" action used below.
    let apiHandle;
    let mcpHandle;

    before(() => {
        // Seeding goes through the REST API as a logged-in admin — the portal's
        // static service API key is gone.
        cy.login();
        cy.seedApi().then((handle) => { apiHandle = handle; });
        cy.seedMcp().then((handle) => { mcpHandle = handle; });
    });

    after(() => {
        cy.login();
        cy.deleteApi(apiHandle);
        cy.deleteMcp(mcpHandle);
    });

    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
    });

    it('portal home loads and shows the hero section', () => {
        cy.visitPortal();
        cy.get('.hero').should('be.visible');
    });

    // -----------------------------------------------------------------------
    // Basic browsing as an anonymous (logged-out) visitor. The APIs / MCP
    // listings and the API detail page are public — no login required.
    // -----------------------------------------------------------------------
    context('basic browsing (logged out)', () => {
        it('browses the APIs listing and opens an API detail page', () => {
            cy.visitPortal('/apis');
            // At least the seeded API is present in the default view.
            cy.get('.api-card').should('have.length.at.least', 1);

            // Click into an API — the card's [data-href] region drives navigation.
            cy.get('.api-card').first().find('.api-card-top').click();

            // The API detail page loaded.
            cy.url().should('include', '/api/');
            cy.get('.aov-hero-name').should('be.visible');
        });

        it('browses the MCP servers listing', () => {
            cy.visitPortal('/mcps');
            cy.get('.apilist-results-heading').should('contain', 'MCP Servers');
            cy.get('.api-card').should('have.length.at.least', 1);
        });

        it('opens the API Workflows page', () => {
            cy.visitPortal('/api-workflows');
            cy.get('body').should('be.visible');
            cy.get('body').should('not.contain.text', 'Cannot GET');
            cy.get('body').should('not.contain.text', '500');
        });
    });

    // -----------------------------------------------------------------------
    // Auth-gated pages redirect an anonymous visitor to the login page and,
    // after a successful login, send them back to the page they asked for
    // (server-side `returnTo`).
    // -----------------------------------------------------------------------
    context('auth redirection (logged out)', () => {
        it('redirects Applications to login and returns there after signing in', () => {
            cy.visitPortal();

            // Applications is auth-gated → clicking it bounces to the login page.
            cy.get('#sidebar #applications').click();
            cy.url().should('include', '/login');
            cy.get('#local-login-form').should('be.visible');

            // After logging in, land back on the originally requested page.
            cy.completeLoginForm();
            cy.url().should('include', '/applications');
            cy.get('#local-login-form').should('not.exist');
            cy.contains('.page-title', 'Applications').should('be.visible');
        });

        it('redirects an API\'s API Keys page to login and returns there after signing in', () => {
            // Open the seeded API's detail page (public).
            cy.visitPortal(`/api/${apiHandle}`);
            cy.get('.aov-hero-name').should('be.visible');

            // The API Keys action is auth-gated → clicking it bounces to login.
            cy.get('.aov-hero-actions a[href$="/api-keys"]').click();
            cy.url().should('include', '/login');
            cy.get('#local-login-form').should('be.visible');

            // After logging in, land back on that API's API Keys page.
            cy.completeLoginForm();
            cy.url().should('include', `/api/${apiHandle}/api-keys`);
            cy.get('#local-login-form').should('not.exist');
        });
    });
});
