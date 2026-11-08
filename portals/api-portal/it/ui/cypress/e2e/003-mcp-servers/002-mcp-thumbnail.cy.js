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

// An MCP server's thumbnail shows on the MCP Hub listing and the server's page, signed in
// and signed out, like an API's thumbnail on the API pages.

describe('MCP server thumbnail on the MCP Hub pages', () => {
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const MCP_NAME = `IT Thumbnail MCP ${uid}`;
    const PNG = Cypress.Buffer.from('89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c63606060f80f00010401005fe5c34b0000000049454e44ae426082', 'hex');
    let handle;

    const loaded = ($img) => expect($img[0].naturalWidth, 'image loaded').to.be.greaterThan(0);
    const expectThumbnails = () => {
        cy.visitPortal('/mcps');
        cy.contains('.api-card', MCP_NAME).find('img.api-card-img')
            .should('have.attr', 'src').and('contain', `/mcp-servers/${handle}/assets`);
        cy.contains('.api-card', MCP_NAME).find('img.api-card-img').should(loaded);

        cy.visitPortal(`/mcp/${handle}`);
        cy.get('img.aov-api-img').should(loaded);
    };

    before(() => {
        cy.login();
        cy.seedMcp({ name: MCP_NAME }).then((h) => {
            handle = h;
            cy.uploadMcpContent(h, [{ name: 'web/api-icon.png', content: PNG }]);
        });
    });

    after(() => {
        cy.login();
        cy.deleteMcp(handle);
    });

    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
    });

    it('shows the thumbnail to a signed-in user', () => {
        cy.login();
        expectThumbnails();
    });

    it('shows the thumbnail to a visitor who is not signed in', () => {
        cy.clearCookies();
        expectThumbnails();
    });
});
