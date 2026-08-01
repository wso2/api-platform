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

// Anonymous (logged-out) browse of a REST API: listing → overview → docs →
// specification/try-it → other docs. The API is seeded via the management API
// with a multi-operation OpenAPI definition, a linked subscription plan, and a
// markdown document so every overview/docs section has something to render.

describe('REST API — overview, documentation & try-out', () => {
    const API_NAME = 'IT REST Detail API';
    let apiHandle;

    before(() => {
        cy.login();
        cy.seedApi({
            name: API_NAME,
            version: 'v1.0',
            // "Bronze" is one of the org's auto-created default plans (seederService).
            subscriptionPlans: [{ id: 'Bronze' }],
            definition: {
                openapi: '3.0.3',
                info: { title: API_NAME, version: '1.0.0' },
                servers: [{ url: 'https://backend.example.invalid' }],
                components: {
                    securitySchemes: {
                        OAuth2: {
                            type: 'oauth2',
                            flows: {
                                clientCredentials: {
                                    tokenUrl: 'https://idp.example.invalid/token',
                                    scopes: { 'read:items': 'Read items', 'write:items': 'Write items' },
                                },
                            },
                        },
                    },
                },
                security: [{ OAuth2: ['read:items'] }],
                paths: {
                    '/items': {
                        get: { summary: 'List items', responses: { 200: { description: 'ok' } } },
                        post: { summary: 'Create an item', responses: { 201: { description: 'created' } } },
                    },
                    '/items/{id}': {
                        get: { summary: 'Get an item', responses: { 200: { description: 'ok' } } },
                    },
                },
            },
            docs: [{
                name: 'getting-started.md',
                content: '# Getting Started\n\nGuide for the IT REST Detail API.\n',
            }],
        }).then((handle) => { apiHandle = handle; });
    });

    after(() => {
        cy.login();
        cy.deleteApi(apiHandle);
    });

    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
    });

    it('opens the API from the listing and shows the overview details', () => {
        cy.visitPortal('/apis');
        // Open the seeded API from its card (the [data-href] region navigates;
        // the click auto-scrolls the card into view).
        cy.contains('.api-card', API_NAME).find('.api-card-top').click();
        cy.url().should('include', `/api/${apiHandle}`);

        // Banner (top of page): name, version, type.
        cy.get('.aov-hero-name').should('be.visible').and('contain', API_NAME);
        cy.get('.dp-badge--version').should('contain', 'v1.0');
        cy.get('.dp-badge--rest').should('be.visible'); // REST type badge

        // The sections below live in a scroll container, so assert on their
        // presence/content rather than viewport visibility.

        // Endpoints: production + sandbox URLs.
        cy.contains('.aov-section-title', 'Endpoints').should('exist');
        cy.get('#token_api_prod_url').should('contain', 'backend.example.invalid');
        cy.get('#token_api_dev_url').should('contain', 'sandbox.example.invalid');

        // Resources: the operations from the OpenAPI definition.
        cy.contains('.aov-section-title', 'Resources').should('exist');
        cy.get('.aov-resource-row').should('have.length.at.least', 2);
        cy.get('.aov-resource-path').should('contain', '/items');
        cy.get('.aov-method-badge--GET').should('exist');
        cy.get('.aov-method-badge--POST').should('exist');

        // Subscription plans: the linked "Bronze" plan.
        cy.get('#subscriptionPlans').should('exist');
        cy.contains('.aov-plans-title', 'Subscription plans').should('exist');
        cy.contains('.aov-plan-card', 'Bronze').should('exist');

        // Scopes section renders. NOTE: the DB-backed overview passes scopes: []
        // unconditionally (apiContentController.loadAPIContent), so this section
        // shows the empty state here; the actual OAuth2 scopes surface in the
        // specification view (Stoplight Elements), asserted separately below.
        cy.contains('.aov-section-title', 'Scopes').should('exist');
    });

    it('opens the documentation and renders the OpenAPI specification', () => {
        cy.visitPortal(`/api/${apiHandle}`);
        cy.get('a.dp-btn[href$="/docs/specification"]').click();

        cy.contains('.page-title', 'Documentation').should('be.visible');
        // The OpenAPI spec is embedded in the Stoplight Elements web component,
        // carrying the API document (server-modified but still containing the
        // seeded paths and the OAuth2 scopes from its security scheme).
        cy.get('.adoc-file-badge--spec').should('contain', 'openapi');
        cy.get('elements-api').should('exist');
        cy.get('elements-api')
            .invoke('attr', 'apiDescriptionDocument')
            .should('include', '/items')
            .and('include', 'read:items')
            .and('include', 'write:items');
    });

    it('lists the specification and the other document in the docs sidebar', () => {
        cy.visitPortal(`/api/${apiHandle}/docs/specification`);

        cy.get('.adoc-nav').should('be.visible');
        cy.get('.adoc-nav').contains('.adoc-nav-item', 'API Definition').should('exist');

        // The seeded markdown doc surfaces under the "Other" group.
        cy.get('.adoc-nav .adoc-nav-group-title').should('contain', 'Other');
        cy.get('.adoc-nav a.doc-link[href*="/docs/Other/"]')
            .should('contain', 'getting-started')
            .click();

        // The document body renders.
        cy.get('.api-markdown-content').should('contain', 'Getting Started');
    });

    it('exposes the Try It console on the specification page', () => {
        cy.visitPortal(`/api/${apiHandle}/docs/specification`);
        // For a REST API the "try out" console is Stoplight Elements' built-in
        // Try-It panel inside <elements-api>. The internal CORS proxy is deliberately
        // NOT wired in (no tryItCorsProxy attribute) so Elements' sample curl can't leak
        // the internal proxy URL — so we assert the console is present but unproxied.
        cy.get('elements-api').should('exist').and('not.have.attr', 'tryItCorsProxy');
    });
});
