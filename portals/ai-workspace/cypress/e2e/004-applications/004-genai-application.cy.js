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

describe('AI Workspace - GenAI application lifecycle', () => {
  const suffix = Date.now().toString().slice(-8);
  const projectName = `E2E App Project ${suffix}`;
  const applicationName = `E2E GenAI Application ${suffix}`;

  beforeEach(() => {
    cy.login();
  });

  it('creates and deletes a GenAI application using only the UI', () => {
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
      'Cypress project for GenAI application lifecycle coverage.'
    );
    cy.contains('button', 'Create')
      .should('not.be.disabled')
      .click();

    cy.contains(projectName, { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('GenAI Applications', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('button, a', /Create Application|Add New Application/, {
      timeout: 30000,
    })
      .should('be.visible')
      .click();

    cy.contains('Create GenAI Application', { timeout: 30000 }).should(
      'be.visible'
    );
    cy.get('input[placeholder="Documentation Assistant"]')
      .should('be.visible')
      .type(applicationName);
    cy.get('textarea[placeholder="Short description of the application."]').type(
      'Cypress GenAI application lifecycle test'
    );
    cy.contains('button', 'Create')
      .should('not.be.disabled')
      .click();

    cy.location('pathname', { timeout: 30000 }).should(
      'include',
      `/applications/${toSlug(applicationName)}`
    );
    cy.contains(applicationName, { timeout: 30000 }).should('be.visible');

    cy.contains('GenAI Applications', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('h5, div', applicationName, { timeout: 30000 })
      .should('be.visible')
      .closest('.MuiCard-root')
      .within(() => {
        cy.get(`button[aria-label="Delete ${applicationName}"]`).click();
      });

    cy.contains('Delete application', { timeout: 30000 }).should('be.visible');
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });

    cy.contains(applicationName, { timeout: 30000 }).should('not.exist');

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
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-+|-+$/g, '');
}
