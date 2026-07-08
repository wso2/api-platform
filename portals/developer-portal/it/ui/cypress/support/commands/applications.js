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

// ---------------------------------------------------------------------------
// cy.createApplication(name)
//   Navigate to the Applications page and create an application by name.
//   Waits until the new application card is visible before resolving.
// ---------------------------------------------------------------------------
Cypress.Commands.add('createApplication', (name) => {
    cy.get('#sidebar #applications').click();
    cy.get('#applicationCreateCard').click();
    cy.get('#applicationName').clear().type(name);
    cy.get('#createAppButton').click();
    cy.contains('.application-name-link', name, { timeout: 15000 }).should('be.visible');
});

// ---------------------------------------------------------------------------
// cy.deleteApplication(name)
//   Delete the named application and confirm it is no longer listed.
// ---------------------------------------------------------------------------
Cypress.Commands.add('deleteApplication', (name) => {
    cy.get(`div[data-name="${name}"] .delete-button`).click();
    cy.contains('.application-name-link', name, { timeout: 15000 }).should('not.exist');
});
