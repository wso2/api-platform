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

// The /mcps listing page: a card per published MCP server (in the default view)
// with an MCP badge, and the same server-side search as /apis. The counterpart
// of 002-apis/002-api-listing.cy.js — loadAPIs filters type === MCP for this
// page, so REST/other APIs must NOT appear here. The listing is public.

describe('MCP server listing', () => {
    const MCP_ONE = 'IT Listing MCP Alpha';
    const MCP_TWO = 'IT Listing MCP Beta';
    const REST_API = 'IT Listing MCP-page REST API';
    let mcpOneHandle;
    let mcpTwoHandle;
    let restHandle;

    before(() => {
        cy.login();
        cy.seedMcp({ name: MCP_ONE }).then((h) => { mcpOneHandle = h; });
        cy.seedMcp({ name: MCP_TWO }).then((h) => { mcpTwoHandle = h; });
        // A REST API — must NOT appear on the /mcps listing (it belongs on /apis).
        cy.seedApi({ name: REST_API }).then((h) => { restHandle = h; });
    });

    after(() => {
        cy.login();
        cy.deleteMcp(mcpOneHandle);
        cy.deleteMcp(mcpTwoHandle);
        cy.deleteApi(restHandle);
    });

    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
    });

    it('lists MCP server cards with the MCP badge', () => {
        cy.visitPortal('/mcps');

        cy.get('.apilist-results-heading').should('contain', 'MCP Servers');
        cy.get('.api-card').should('have.length.at.least', 2);

        // Seeded MCP servers appear, with the MCP badge.
        cy.contains('.api-card', MCP_ONE).within(() => {
            cy.get('.dp-badge--mcp').should('exist').and('contain', 'MCP');
        });
        cy.contains('.api-card', MCP_TWO).should('exist');

        // REST APIs belong on /apis and must not appear here.
        cy.contains('.api-card', REST_API).should('not.exist');
    });

    it('narrows the listing with a search query', () => {
        cy.visitPortal('/mcps');

        // Search is server-side: Enter navigates to ?query=... and the server filters.
        cy.get('#query').type(`${MCP_ONE}{enter}`);
        cy.url().should('include', 'query=');

        cy.contains('.api-card', MCP_ONE).should('be.visible');
        cy.contains('.api-card', MCP_TWO).should('not.exist');
    });

    it('does not surface REST APIs via MCP search', () => {
        cy.visitPortal('/mcps');

        cy.get('#query').type(`${REST_API}{enter}`);
        cy.url().should('include', 'query=');
        cy.contains('.api-card', REST_API).should('not.exist');
    });
});
