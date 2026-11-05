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

// An MCP server's landing text (web/apiContent.md) shows on its page, like an API's.

describe('MCP server landing text on its page', () => {
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    let handle;

    before(() => {
        cy.login();
        cy.seedMcp({ name: `IT Landing MCP ${uid}` }).then((h) => {
            handle = h;
            // A docs/ entry too: on main a ZIP holding only web/ is taken for a wrapper folder and rejected.
            cy.uploadMcpContent(h, [
                { name: 'web/apiContent.md', content: '# Weather MCP landing page' },
                { name: 'docs/Guide.md', content: '# Guide' },
            ]);
        });
    });

    after(() => {
        cy.login();
        cy.deleteMcp(handle);
    });

    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
    });

    it('shows the text from apiContent.md', () => {
        cy.login();
        cy.visitPortal(`/mcp/${handle}`);
        cy.get('.api-md-content').should('contain', 'Weather MCP landing page');
    });
});
