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

// The /apis listing page: renders a card per published API (in the default view)
// with a type badge, and supports server-side search (Enter navigates to
// ?query=... and the server filters). The listing is public — no login needed.

describe('API listing', () => {
    const REST_API = 'IT Listing REST API';
    const GQL_API = 'IT Listing GraphQL API';
    const MCP_NAME = 'IT Listing MCP Server';
    let restHandle;
    let gqlHandle;
    let mcpHandle;

    before(() => {
        // Seed two APIs of different types so the listing shows distinct badges,
        // plus an MCP server — which must NOT appear on the /apis listing (it
        // belongs on /mcps). loadAPIs filters type !== MCP for the APIs page.
        cy.seedApi({ name: REST_API }).then((h) => { restHandle = h; });
        cy.seedApi({
            name: GQL_API,
            type: 'GRAPHQL',
            definition: 'type Query { hello: String }\n',
            definitionFileName: 'schema.graphql',
            definitionContentType: 'application/graphql',
        }).then((h) => { gqlHandle = h; });
        cy.seedMcp({ name: MCP_NAME }).then((h) => { mcpHandle = h; });
    });

    after(() => {
        cy.deleteApi(restHandle);
        cy.deleteApi(gqlHandle);
        cy.deleteMcp(mcpHandle);
    });

    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
    });

    it('lists API cards, including different API types', () => {
        cy.visitPortal('/apis');

        cy.get('.apilist-results-heading').should('contain', 'APIs');
        cy.get('.api-card').should('have.length.at.least', 2);

        // Both seeded APIs appear, each with its own type badge.
        cy.contains('.api-card', REST_API).within(() => {
            cy.get('.dp-badge--rest').should('exist').and('contain', 'REST');
        });
        cy.contains('.api-card', GQL_API).within(() => {
            cy.get('.dp-badge--graphql').should('exist').and('contain', 'GraphQL');
        });

        // MCP servers belong on /mcps and must not appear in the API listing.
        cy.contains('.api-card', MCP_NAME).should('not.exist');
    });

    it('narrows the listing with a search query', () => {
        cy.visitPortal('/apis');

        // Search is server-side: Enter navigates to ?query=... and the server filters.
        cy.get('#query').type(`${GQL_API}{enter}`);
        cy.url().should('include', 'query=');

        // Only the matching API remains; the non-matching one is filtered out.
        cy.contains('.api-card', GQL_API).should('be.visible');
        cy.contains('.api-card', REST_API).should('not.exist');
    });

    it('does not surface MCP servers via API search', () => {
        cy.visitPortal('/apis');

        // Searching an MCP server's name from the APIs page returns no card —
        // MCP-typed results are filtered out of the /apis listing and search.
        cy.get('#query').type(`${MCP_NAME}{enter}`);
        cy.url().should('include', 'query=');
        cy.contains('.api-card', MCP_NAME).should('not.exist');
    });
});
