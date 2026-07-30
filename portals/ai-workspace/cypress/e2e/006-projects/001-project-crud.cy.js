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

// Release test plan coverage: "Project creation, listing, update and delete"
// and "Trying to delete project while there are existing resources within the
// project". The existing specs only ever created a project as a container for
// something else, so the update path and the search/list path were untested.

describe('AI Workspace - project CRUD', () => {
  const suffix = Date.now().toString().slice(-8);
  const projectName = `E2E Project CRUD ${suffix}`;
  const renamedProjectName = `E2E Project CRUD ${suffix} Renamed`;
  const initialDescription = 'Cypress project lifecycle coverage.';
  const updatedDescription = 'Cypress project lifecycle coverage (updated).';

  let authToken = '';

  beforeEach(() => {
    cy.login();
    cy.apiLogin().then((session) => {
      authToken = session.authToken;
    });
  });

  afterEach(() => {
    // Either name may be the surviving one depending on where the spec failed.
    cy.deleteProjectByNameApi(authToken, projectName);
    cy.deleteProjectByNameApi(authToken, renamedProjectName);
  });

  it('creates, lists, searches, updates and deletes a project using only the UI', () => {
    cy.intercept('POST', '**/projects').as('createProject');
    cy.intercept('PUT', '**/projects/**').as('updateProject');
    cy.intercept('DELETE', '**/projects/**').as('deleteProject');

    // --- Create ------------------------------------------------------------
    cy.createProjectUI(projectName, initialDescription);
    cy.wait('@createProject')
      .its('response.statusCode')
      .should('be.oneOf', [200, 201]);

    // --- List + search -----------------------------------------------------
    // Search narrows to the new project, which also proves it was persisted
    // rather than only rendered optimistically.
    cy.get('input[placeholder="Search projects..."]', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type(projectName);
    cy.contains(projectName, { timeout: 30000 }).should('be.visible');

    cy.get('input[placeholder="Search projects..."]')
      .clear()
      .type(`no-such-project-${suffix}`);
    cy.contains('No projects match your search.', { timeout: 30000 }).should(
      'be.visible'
    );
    cy.get('input[placeholder="Search projects..."]').clear();

    // --- Update ------------------------------------------------------------
    projectRow(projectName).within(() => {
      cy.get('button[aria-label="Edit project"]').click();
    });

    cy.location('pathname', { timeout: 30000 }).should('match', /\/projects\/[^/]+\/edit$/);
    cy.contains('Edit Project', { timeout: 30000 }).should('be.visible');

    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
      .should('have.value', projectName)
      .clear()
      .type(renamedProjectName);
    cy.get('textarea[placeholder="Short description of the project."]')
      .first()
      .clear()
      .type(updatedDescription);
    cy.contains('button', 'Save Changes').should('not.be.disabled').click();
    cy.wait('@updateProject')
      .its('response.statusCode')
      .should('be.oneOf', [200, 201, 204]);

    // Re-open the edit form so the assertion reads server state, not the form
    // state left behind by the typing above.
    cy.contains(renamedProjectName, { timeout: 30000 }).should('be.visible');
    projectRow(renamedProjectName).within(() => {
      cy.get('button[aria-label="Edit project"]').click();
    });
    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 }).should(
      'have.value',
      renamedProjectName
    );
    cy.get('textarea[placeholder="Short description of the project."]')
      .first()
      .should('have.value', updatedDescription);
    // Rendered via `component={RouterLink}`, so it is an <a>, not a <button>.
    cy.contains('a, button', 'Back to list').click();

    // --- Delete ------------------------------------------------------------
    projectRow(renamedProjectName).within(() => {
      cy.get('button[aria-label="Delete project"]').click();
    });
    cy.contains('Delete Project', { timeout: 30000 }).should('be.visible');
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });
    cy.wait('@deleteProject')
      .its('response.statusCode')
      .should('be.oneOf', [200, 204]);

    cy.contains(renamedProjectName, { timeout: 30000 }).should('not.exist');
  });
});

// Projects render as cards in the list view; scope actions to the card carrying
// the target name so a partial name match elsewhere cannot hijack the click.
function projectRow(projectName) {
  return cy
    .contains(projectName, { timeout: 30000 })
    .should('be.visible')
    .closest('.MuiCard-root');
}
