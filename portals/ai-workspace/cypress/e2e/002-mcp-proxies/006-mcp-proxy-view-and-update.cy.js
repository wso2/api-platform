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

// Release test plan coverage for MCP proxies. 002-mcp-proxy-sample-url.cy.js
// covers create+delete, and 004/005 cover the Backend Connection tab against
// stubbed upstreams. The remaining plan items are the update path and the
// "View MCP Proxy URLs" surface, which this spec picks up.

describe('AI Workspace - MCP proxy view and update', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const projectName = `E2E MCP View Project ${suffix}`;
  const proxyName = `E2E MCP View Proxy ${suffix}`;
  const updatedProxyName = `${proxyName} Updated`;
  const updatedDescription = 'Cypress MCP proxy update coverage';

  let createdProxyId = '';
  let proxyUrl = '';
  let authToken = '';

  beforeEach(() => {
    cy.login();
    cy.apiLogin().then((session) => {
      authToken = session.authToken;
      // A polluted org can already sit at MaxMCPProxiesPerOrganization, which
      // would make this run's create fail with a 409 for unrelated reasons.
      cy.sweepE2EMCPProxies(authToken);
    });
  });

  afterEach(() => {
    cy.sweepE2EMCPProxies(authToken);
    cy.deleteProjectByNameApi(authToken, projectName);
  });

  it('views every MCP proxy tab, then updates its details', () => {
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('createProxy');
    cy.intercept('PUT', '**/mcp-proxies/**').as('updateProxy');

    cy.createProjectUI(projectName, 'Cypress project for MCP proxy view/update coverage.');

    // --- Create from the sample URL flow ------------------------------------
    cy.contains(projectName, { timeout: 30000 }).should('be.visible').click();
    cy.contains('MCP Proxies', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', 'Create MCP Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('Create MCP Proxy from Endpoint', { timeout: 30000 }).should(
      'be.visible'
    );
    cy.contains('button', 'Try with Sample URL').click();
    cy.contains('button', 'Next', { timeout: 60000 }).should('be.visible').click();

    cy.get('input[placeholder="WSO2 MCP Proxy"]', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type(proxyName);
    cy.get('textarea[placeholder="Primary MCP Proxy"]')
      .clear()
      .type('Cypress MCP proxy view coverage');
    cy.contains('button', 'Create').should('not.be.disabled').click();

    cy.wait('@createProxy').then(({ response }) => {
      expect(
        response?.statusCode,
        `POST /mcp-proxies failed: ${JSON.stringify(response?.body)}`
      ).to.be.oneOf([200, 201]);
      createdProxyId = response?.body?.id ?? '';
      expect(createdProxyId).to.not.equal('');
    });

    cy.contains(proxyName, { timeout: 30000 }).should('be.visible');
    // MCP proxies are project-scoped, so their real path includes the project
    // segment; an org-level URL renders a project picker instead of the proxy.
    cy.url().then((url) => {
      proxyUrl = url;
    });

    // --- View: every tab must render ---------------------------------------
    ['Overview', 'Policies', 'Backend Connection'].forEach((tabLabel) => {
      cy.contains('button[role="tab"]', tabLabel, { timeout: 30000 })
        .should('be.visible')
        .click();
    });

    // The "MCP Proxy URL" panel is NOT asserted here: it renders only when
    // deployedGateways.length > 0, so it is absent until the proxy is deployed.
    // That path needs a gateway and lives in 007-deployments.
    openMcpTab('Overview');

    // --- Update: name and description via the edit route --------------------
    cy.then(() => cy.visit(`${proxyUrl}/edit`));
    cy.get('input[placeholder="Enter server name"]', { timeout: 30000 })
      .should('have.value', proxyName)
      .clear()
      .type(updatedProxyName);
    cy.get('textarea[placeholder="Enter description"]')
      .first()
      .clear()
      .type(updatedDescription);
    cy.contains('button', 'Update').should('not.be.disabled').click();
    cy.wait('@updateProxy')
      .its('response.statusCode')
      .should('be.oneOf', [200, 201, 204]);

    // Re-enter the edit form so the assertion reads persisted server state.
    cy.then(() => cy.visit(`${proxyUrl}/edit`));
    cy.get('input[placeholder="Enter server name"]', { timeout: 30000 }).should(
      'have.value',
      updatedProxyName
    );
    cy.get('textarea[placeholder="Enter description"]')
      .first()
      .should('have.value', updatedDescription);
  });
});

function openMcpTab(tabLabel) {
  cy.contains('button[role="tab"]', tabLabel, { timeout: 30000 })
    .should('be.visible')
    .click();
}
