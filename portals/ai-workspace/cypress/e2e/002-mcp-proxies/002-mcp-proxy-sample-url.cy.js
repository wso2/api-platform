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

describe('AI Workspace - MCP proxy sample URL lifecycle', () => {
  const suffix = Date.now().toString().slice(-8);
  const projectName = `E2E MCP Project ${suffix}`;
  const proxyName = `E2E MCP Proxy ${suffix}`;
  const proxyId = toSlug(proxyName);

  beforeEach(() => {
    cy.login();
  });

  it('creates and deletes an MCP proxy from the sample URL flow using only the UI', () => {
    cy.contains('Projects', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('button, a', /Create Project|Add New Project/, {
      timeout: 30000,
    })
      .should('be.visible')
      .click();

    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
      .should('be.visible')
      .type(projectName);
    cy.get('textarea[placeholder="Short description of the project."]').type(
      'Cypress project for MCP proxy sample URL coverage.'
    );
    cy.contains('button', 'Create')
      .should('not.be.disabled')
      .click();

    cy.contains(projectName, { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('MCP Proxies', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('button, a', 'Create MCP Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('Create MCP Proxy from Endpoint', { timeout: 30000 }).should(
      'be.visible'
    );
    cy.contains('button', 'Try with Sample URL').click();
    cy.contains('button', 'Next', { timeout: 60000 })
      .should('be.visible')
      .click();

    cy.get('input[placeholder="WSO2 MCP Proxy"]', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type(proxyName);
    cy.get('textarea[placeholder="Primary MCP Proxy"]').clear().type(
      'Cypress MCP proxy sample URL lifecycle test'
    );
    cy.get('input[placeholder="v1.0"]').should('not.have.value', '');
    cy.get('input[placeholder="https://example.com/mcp"]').should(
      'not.have.value',
      ''
    );
    cy.contains('button', 'Create')
      .should('not.be.disabled')
      .click();

    cy.location('pathname', { timeout: 30000 }).should(
      'include',
      `/mcp-proxy/${proxyId}`
    );
    cy.contains(proxyName, { timeout: 30000 }).should('be.visible');

    cy.contains('MCP Proxies', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('td, h6, div', proxyName, { timeout: 30000 })
      .should('be.visible')
      .closest('tr')
      .within(() => {
        cy.get(`button[aria-label="Delete ${proxyName}"]`).click();
      });

    cy.contains('Delete external server', { timeout: 30000 }).should(
      'be.visible'
    );
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });

    cy.contains(proxyName, { timeout: 30000 }).should('not.exist');

    cy.get('button[aria-label="Go to organization level"]', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('Projects', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains(projectName, { timeout: 30000 })
      .should('be.visible')
      .closest('.MuiCard-root')
      .within(() => {
        cy.get('button[aria-label="Delete project"]').click();
      });

    cy.contains('Delete Project', { timeout: 30000 }).should('be.visible');
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });

    cy.contains(projectName, { timeout: 30000 }).should('not.exist');
  });
});

function toSlug(value) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
