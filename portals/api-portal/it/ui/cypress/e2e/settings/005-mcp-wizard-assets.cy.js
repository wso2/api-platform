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

// The MCP server wizard has the same Assets step as the API wizard, saving through
// /mcp-servers and /mcp-servers/{id}/assets.

const { contentPackage } = require('../../support/contentPackages');

describe('Settings — MCP server wizard Assets step', () => {
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    const handles = [];
    const settingsUrl = () => `${Cypress.env('BASE_PATH')}/${Cypress.env('ORG_HANDLE')}/settings`;
    const TOOLS = JSON.stringify([{ type: 'TOOL', name: 'ping', description: 'Ping.', inputSchema: { type: 'object', properties: {} } }]);
    const PNG = '89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000d49444154789c63606060f80f00010401005fe5c34b0000000049454e44ae426082';

    const asset = (handle, type, fileName) => cy.apiRequest('GET',
        `/api/v0.9/mcp-servers/${handle}/assets?type=${type}&fileName=${encodeURIComponent(fileName)}`,
        { failOnStatusCode: false });
    const save = () => cy.get('#cfg-wizard-next').click();
    const saved = () => cy.get('#cfg-apis-wizard', { timeout: 20000 }).should('not.be.visible');
    const openSettings = () => {
        cy.login();
        cy.visit(settingsUrl());
        cy.get('.cfg-nav-item[data-panel="cfg-mcps"]').click();
    };

    after(() => {
        cy.login();
        handles.forEach((handle) => cy.deleteMcp(handle));
    });

    it('creates an MCP server with a thumbnail and documents in three steps', () => {
        const handle = `it-mcp-wizard-docs-${uid}`;
        handles.push(handle);
        openSettings();
        cy.get('#cfg-add-mcp-btn').click();
        cy.get('#wz-name').type(`MCP Wizard ${handle}`);
        cy.get('#wz-version').type('v1.0');
        cy.get('#wz-handle').clear().type(handle);
        cy.get('#wz-prod').type('https://mcp.example.invalid/ping');
        cy.get('#wz-card-general').should('contain', 'How this MCP server is named and shown in the portal.');
        cy.get('#wz-thumb-input').selectFile({ contents: Cypress.Buffer.from(PNG, 'hex'), fileName: 'logo.png', mimeType: 'image/png' }, { force: true });
        cy.get('#wz-thumb-tag').should('have.text', 'Replaced when you save');
        cy.get('#cfg-wizard-next').click();
        // The tools schema step keeps its own wording and has no API formats table.
        cy.get('#wz-spec-desc').should('contain', 'Upload the MCP tools schema that defines this server’s tools.');
        cy.get('#wz-spec-formats').should('not.be.visible');
        cy.get('#wz-spec-input').should('have.attr', 'accept', '.json,.yaml,.yml');
        cy.get('#wz-spec-input').selectFile({ contents: Cypress.Buffer.from(TOOLS), fileName: 'definition.json' }, { force: true });
        cy.get('#wz-spec-chip').should('contain', 'Tools schema · uploaded when you save');
        cy.get('#cfg-wizard-next').click();

        cy.get('#cfg-apis-wizard .cfg-step-label').then(($labels) => {
            expect([...$labels].map((el) => el.textContent.trim())).to.deep.eq(['Details', 'Tools Schema', 'Assets']);
        });
        cy.get('#cfg-wizard-next').should('have.text', 'Create MCP server');
        cy.get('#wz-assets-panel-docs').scrollIntoView().should('be.visible')
            .and('contain', 'Add documents for this MCP server. You can change them later by editing the MCP server.');
        cy.get('#wz-docs-input').selectFile({ contents: Cypress.Buffer.from('# A'), fileName: 'A.md' }, { force: true });
        save();
        saved();

        asset(handle, 'DOC_Other', 'A.md').its('status').should('eq', 200);
        asset(handle, 'IMAGE', 'api-icon.png').its('status').should('eq', 200);

        // The table row and the wizard show the saved thumbnail, loaded from the MCP server's own assets.
        openSettings();
        cy.get(`#cfg-api-row-${handle} img.cfg-api-avatar--img`)
            .should('have.attr', 'src').and('contain', `/mcp-servers/${handle}/assets`);
        cy.get(`#cfg-api-row-${handle} img.cfg-api-avatar--img`).should(($img) => expect($img[0].naturalWidth).to.be.greaterThan(0));
        cy.get(`.cfg-edit-trigger[data-id="${handle}"]`).first().click({ force: true });
        cy.get('#wz-thumb-preview img')
            .should('have.attr', 'src').and('contain', `/mcp-servers/${handle}/assets`);
        cy.get('#wz-thumb-preview img').should(($img) => expect($img[0].naturalWidth).to.be.greaterThan(0));
    });

    it('imports a package on edit after confirming, keeping the tools schema', () => {
        const handle = `it-mcp-wizard-import-${uid}`;
        cy.login();
        cy.seedMcp({ id: handle, name: `MCP Wizard ${handle}` });
        handles.push(handle);

        openSettings();
        cy.get(`.cfg-edit-trigger[data-id="${handle}"]`).first().click({ force: true });
        cy.get('#cfg-wizard-next').click();
        cy.get('#cfg-wizard-next').click();
        cy.get('#wz-assets-to-import').click();
        cy.get('#wz-content-input').selectFile(contentPackage(), { force: true });
        save();
        cy.get('#cfg-replace-content-modal').should('contain', `the thumbnail of MCP Wizard ${handle} with the content of content.zip`);
        cy.get('#cfg-replace-content-confirm').should('be.visible').click();
        saved();

        asset(handle, 'MARKETING', 'apiContent.md').its('status').should('eq', 200);
        asset(handle, 'DOC_Other', 'Guide.md').its('status').should('eq', 200);
        asset(handle, 'SCHEMA_DEFINITION', 'definition.json').its('status').should('eq', 200);
    });
});
