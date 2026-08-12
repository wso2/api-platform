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

// A workflow Description is optional. This walks the create wizard leaving it
// empty, which previously failed twice: the step 1 -> 2 advance and the final
// save both treated an empty Description as invalid.

describe('Settings — API Workflows', () => {
    // Not crypto.randomUUID(): Cypress runs specs against http://api-portal:9543, an
    // insecure context where the WebCrypto API is unavailable. Date.now() + a random
    // suffix is unique across (serial) runs and stays slug-safe ([0-9a-z-]).
    const uid = `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
    const WORKFLOW_NAME = `IT Workflow ${uid}`;
    const WORKFLOW_HANDLE = `it-workflow-${uid}`; // slugify(WORKFLOW_NAME)

    const settingsUrl = () => `/${Cypress.env('ORG_HANDLE')}/settings`;
    const workflowUrl = () =>
        `/api/v0.9/views/${Cypress.env('VIEW_NAME')}/api-workflows/${WORKFLOW_HANDLE}`;

    after(() => {
        // Idempotent API cleanup (404 if the create step never persisted).
        cy.login();
        cy.apiRequest('DELETE', workflowUrl(), { failOnStatusCode: false });
    });

    it('creates a workflow without a description', () => {
        cy.login();
        cy.visit(settingsUrl());

        cy.get('.cfg-nav-item[data-panel="cfg-workflows"]').click();
        cy.get('#createApiWorkflowBtn').click();

        // Step 1 — name only. The handle auto-generates; Description stays empty.
        cy.get('#afStep1').should('be.visible');
        cy.get('#apiWorkflowName').type(WORKFLOW_NAME);
        cy.get('#apiWorkflowHandle').should('have.value', WORKFLOW_HANDLE);
        cy.get('#apiWorkflowDescription').should('have.value', '');

        cy.get('#afContinueBtn').click();

        // Step 2 — supply the workflow content, which is genuinely required.
        // Uploading a .md file sets the content type to MD and fills the editor.
        cy.get('#afStep2').should('be.visible');
        cy.get('#arazoFileInput').selectFile(
            {
                contents: Cypress.Buffer.from('# IT Workflow\n\nCreated by an integration test.\n'),
                fileName: 'workflow.md',
                mimeType: 'text/markdown',
            },
            // The input sits behind a drop zone and is display:none.
            { force: true }
        );
        cy.get('#uploadFileName').should('contain', 'workflow.md');

        cy.get('#afContinueBtn').click();

        // Step 3 — an agent prompt is required while agent visibility is VISIBLE.
        // It is normally auto-generated, so only type one if it came back empty.
        cy.get('#afStep3').should('be.visible');
        cy.get('#agentPromptField')
            .invoke('val')
            .then((prompt) => {
                if (!String(prompt || '').trim()) {
                    cy.get('#agentPromptField').type('Use this workflow in an integration test.');
                }
            });

        // The readiness checklist tracks the name only, so an empty Description
        // must not leave it flagged as unmet.
        cy.get('#afReady1').should('have.class', 'is-ok').and('not.have.class', 'is-warn');

        cy.get('#saveApiWorkflowBtn').click();

        // A successful save reloads the page; the listing is rendered server-side.
        cy.get('#apiWorkflowList', { timeout: 20000 }).should('be.visible');
        cy.get(`.api-workflow-edit-btn[data-api-workflow-id="${WORKFLOW_HANDLE}"]`).should('exist');
        cy.get('#apiWorkflowList').should('contain', WORKFLOW_NAME);

        // Reopen the saved workflow to confirm the empty Description round-tripped
        // rather than being replaced on the save or read path. The edit button sits
        // in the row's three-dots dropdown, so open that first.
        cy.get(`.api-workflow-edit-btn[data-api-workflow-id="${WORKFLOW_HANDLE}"]`)
            .closest('.cfg-cell-actions')
            .find('.cfg-menu-trigger')
            .click();
        cy.get(`.api-workflow-edit-btn[data-api-workflow-id="${WORKFLOW_HANDLE}"]`)
            .should('be.visible')
            .click();

        // The form is populated from the already-loaded listing data, so a settled
        // name field is the signal that it finished opening.
        cy.get('#apiWorkflowForm').should('be.visible');
        cy.get('#apiWorkflowName').should('have.value', WORKFLOW_NAME);
        cy.get('#apiWorkflowDescription').should('have.value', '');
    });
});
