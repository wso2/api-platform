/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

describe('AI Workspace - AI gateway lifecycle', () => {
  const suffix = Date.now().toString().slice(-4);
  const gatewayName = `gw-${suffix}`;

  beforeEach(() => {
    cy.login();
  });

  it('creates and deletes an AI gateway using only the UI', () => {
    cy.contains('AI Gateways', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('button, a', 'Add AI Gateway', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('Add AI Gateway', { timeout: 30000 }).should('be.visible');
    cy.get('input[placeholder="Enter gateway name"]')
      .should('be.visible')
      .type(gatewayName);
    cy.get('textarea[placeholder="Enter description"]').type(
      'Cypress AI gateway lifecycle test'
    );
    cy.get('input[placeholder="Enter gateway URL"]')
      .clear()
      .type('https://localhost:8443');
    cy.contains('button', 'Add Gateway')
      .should('not.be.disabled')
      .click();

    cy.location('pathname', { timeout: 30000 }).should(
      'include',
      `/gateways/view/${gatewayName}`
    );
    cy.contains(gatewayName, { timeout: 30000 }).should('be.visible');

    cy.contains('AI Gateways', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('td, h6, div', gatewayName, { timeout: 30000 })
      .should('be.visible')
      .closest('tr')
      .within(() => {
        cy.get(`button[aria-label="Delete ${gatewayName}"]`).click();
      });

    cy.contains('Delete AI Gateway', { timeout: 30000 }).should('be.visible');
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });

    cy.contains(gatewayName, { timeout: 30000 }).should('not.exist');
  });
});
