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

/**
 * Target staleness on the MCP proxy create flow (issue #3577).
 *
 * Step 1 validates an endpoint (Fetch Server Info); step 2 pre-fills an editable
 * Target field with it. ExternalServersNew derives isTargetStale by comparing the
 * live Target against the URL discovery actually validated, so discovered
 * capabilities can never be bound to an endpoint they weren't fetched from.
 *
 * Covers:
 *   TC-105 Editing Target on step 2 to a URL other than the validated one →
 *          staleness error shown, Create disabled
 *   TC-106 Typing the validated URL back into Target → error cleared, Create
 *          re-enabled (a live derived comparison, not a one-way latch)
 */
describe('AI Workspace — MCP proxy create flow (Target staleness)', () => {
  const suffix = Date.now().toString().slice(-8);
  const projectName = `E2E MCP Target Project ${suffix}`;
  const serverName = `E2E MCP Target Server ${suffix}`;

  const VALIDATED_URL = 'https://sample.mcp.example.com/mcp';
  const EDITED_URL = 'https://edited-after-validation.mcp.example.com/mcp';
  const STALE_TARGET_MESSAGE =
    'Target has changed since validation. Go back and re-fetch server info before creating.';

  let authToken = '';
  let organizationId = '';
  let createdProjectId = '';

  beforeEach(() => {
    cy.login();
    cy.request({
      method: 'POST',
      url: '/proxy/api/portal/v0.9/auth/login',
      form: true,
      body: {
        username: Cypress.env('ADMIN_USER'),
        password: Cypress.env('ADMIN_PASSWORD'),
      },
    })
      .then((response) => {
        expect(response.status).to.eq(200);
        authToken = response.body?.token ?? '';
        return cy.request({
          url: '/proxy/api/v0.9/organizations',
          headers: { Authorization: `Bearer ${authToken}` },
        });
      })
      .then((response) => {
        const orgs = response.body?.list ?? [];
        organizationId = orgs[0]?.id ?? '';
      });
  });

  afterEach(() => {
    // Neither test clicks Create, so only the project needs cleaning up.
    if (authToken && organizationId && createdProjectId) {
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/projects/${encodeURIComponent(createdProjectId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
      createdProjectId = '';
    }
  });

  // ---------------------------------------------------------------------------
  // TC-105: Editing Target after validation → error shown, Create disabled
  // ---------------------------------------------------------------------------
  it('TC-105: editing the Target after validation shows a staleness error and disables Create', () => {
    createProjectAndNavigateToMCPCreate(projectName);
    validateAndAdvanceToCreateStep(VALIDATED_URL);

    // Precondition: Target is pre-filled with the validated URL, so it isn't stale yet.
    cy.contains(STALE_TARGET_MESSAGE).should('not.exist');
    cy.contains('button', 'Create').should('not.be.disabled');

    cy.get('input[placeholder="https://example.com/mcp"]')
      .clear()
      .type(EDITED_URL);

    cy.contains(STALE_TARGET_MESSAGE, { timeout: 15000 }).should('be.visible');
    cy.contains('button', 'Create').should('be.disabled');
  });

  // ---------------------------------------------------------------------------
  // TC-106: Restoring the validated Target → error cleared, Create re-enabled
  // ---------------------------------------------------------------------------
  it('TC-106: typing the validated URL back into Target clears the error and re-enables Create', () => {
    createProjectAndNavigateToMCPCreate(projectName);
    validateAndAdvanceToCreateStep(VALIDATED_URL);

    cy.get('input[placeholder="https://example.com/mcp"]')
      .clear()
      .type(EDITED_URL);
    cy.contains(STALE_TARGET_MESSAGE, { timeout: 15000 }).should('be.visible');
    cy.contains('button', 'Create').should('be.disabled');

    cy.get('input[placeholder="https://example.com/mcp"]')
      .clear()
      .type(VALIDATED_URL);

    cy.contains(STALE_TARGET_MESSAGE).should('not.exist');
    cy.contains('button', 'Create').should('not.be.disabled');
  });

  // ---------------------------------------------------------------------------
  // Helpers
  // ---------------------------------------------------------------------------

  /**
   * Creates a project through the UI and navigates to the MCP proxy creation page.
   * Uses UI interactions only — avoids cy.request() for project creation so the
   * test does not depend on a direct backend connection from the test runner host.
   */
  function createProjectAndNavigateToMCPCreate(name) {
    cy.intercept('POST', '**/projects').as('createProject');

    cy.contains('Projects', { timeout: 30000 }).should('be.visible').click();

    cy.contains('button, a', /Create Project|Add New Project/, { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
      .should('be.visible')
      .type(name);
    cy.get('textarea[placeholder="Short description of the project."]').type(
      'Cypress MCP target staleness project'
    );
    cy.contains('button', 'Create').should('not.be.disabled').click();
    cy.wait('@createProject').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      createdProjectId = interception.response.body?.id ?? '';
    });

    cy.contains(name, { timeout: 30000 }).should('be.visible').click();
    cy.contains('MCP Proxies', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', 'Create MCP Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();
  }

  /**
   * Stubs fetch-server-info, validates `endpointUrl` on step 1, and advances to
   * step 2. Always stubs the validation endpoint so tests do not need a real MCP
   * server. The name is set explicitly so Create's enabled state depends only on
   * Target staleness.
   */
  function validateAndAdvanceToCreateStep(endpointUrl) {
    cy.intercept('POST', '**/fetch-server-info*', {
      statusCode: 200,
      body: {
        serverInfo: { name: 'Stub MCP Server', version: '1.0.0' },
        tools: [],
        resources: [],
        prompts: [],
      },
    }).as('stubFetchInfo');

    cy.contains('Create MCP Proxy from Endpoint', { timeout: 30000 }).should('be.visible');

    cy.get('input[placeholder="Enter URL of Your MCP Proxy"]', { timeout: 15000 })
      .should('be.visible')
      .type(endpointUrl);

    cy.contains('button', 'Fetch Server Info', { timeout: 15000 })
      .should('be.visible')
      .click();
    cy.wait('@stubFetchInfo');

    // "Next" only appears after validationResult is set.
    cy.contains('button', 'Next', { timeout: 15000 })
      .should('be.visible')
      .click();

    // Step 2: Target is pre-filled from the validated endpoint.
    cy.get('input[placeholder="https://example.com/mcp"]', { timeout: 15000 })
      .should('be.visible')
      .and('have.value', endpointUrl);

    cy.get('input[placeholder="WSO2 MCP Proxy"]')
      .clear()
      .type(serverName);
  }
});
