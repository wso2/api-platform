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

  let authToken = '';

  beforeEach(() => {
    cy.login();
    // Also authenticate via the API so afterEach can clean up deterministically,
    // even when the UI flow fails before its inline cleanup runs.
    cy.request({
      method: 'POST',
      url: '/api/proxy/api/portal/v0.9/auth/login',
      form: true,
      body: {
        username: Cypress.env('ADMIN_USER'),
        password: Cypress.env('ADMIN_PASSWORD'),
      },
      failOnStatusCode: false,
    }).then((response) => {
      authToken = response.status === 200 ? response.body?.token ?? '' : '';
    });
    // Clear leaked "E2E " proxies up front so a polluted org (already at the
    // MCP proxy limit from earlier failed runs) does not fail this run's create.
    cy.then(() => {
      if (authToken) sweepE2EMCPProxies(authToken);
    });
  });

  afterEach(() => {
    if (!authToken) return;
    // The org caps MCP proxies (MaxMCPProxiesPerOrganization). If a run fails
    // before its inline UI cleanup, the proxy leaks and eventually the create
    // call starts returning 409 (limit reached). Sweep every leaked "E2E "
    // proxy so repeated local runs stay green, then drop the test project.
    sweepE2EMCPProxies(authToken);
    deleteProjectByName(authToken, projectName);
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

    // Intercept the create call so a backend rejection surfaces as a clear
    // status/message instead of an opaque "still on /mcp-proxy/new" timeout.
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('createProxy');

    cy.contains('button', 'Create')
      .should('not.be.disabled')
      .click();

    cy.wait('@createProxy').then((interception) => {
      expect(
        interception.response.statusCode,
        `POST /mcp-proxies failed: ${JSON.stringify(interception.response.body)}`
      ).to.be.oneOf([200, 201]);
    });

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

// Delete every MCP proxy whose displayName starts with "E2E " so leaked proxies
// from earlier failed runs do not push the org over its MCP proxy limit. The
// list endpoint is project-scoped, and the limit is per-organization, so we
// enumerate all projects and sweep each one's proxies.
function sweepE2EMCPProxies(authToken) {
  const headers = { Authorization: `Bearer ${authToken}` };
  cy.request({
    url: '/api/proxy/api/v0.9/projects',
    headers,
    failOnStatusCode: false,
  }).then((response) => {
    if (response.status !== 200) return;
    const projects = response.body?.list ?? [];
    projects.forEach((project) => {
      if (!project.id) return;
      cy.request({
        url: `/api/proxy/api/v0.9/mcp-proxies?projectId=${encodeURIComponent(project.id)}&limit=100&offset=0`,
        headers,
        failOnStatusCode: false,
      }).then((listResponse) => {
        if (listResponse.status !== 200) return;
        const proxies = (listResponse.body?.list ?? []).filter(
          (p) =>
            typeof p.displayName === 'string' &&
            p.displayName.startsWith('E2E ')
        );
        proxies.forEach((proxy) => {
          if (!proxy.id) return;
          cy.request({
            method: 'DELETE',
            url: `/api/proxy/api/v0.9/mcp-proxies/${encodeURIComponent(proxy.id)}`,
            headers,
            failOnStatusCode: false,
          });
        });
      });
    });
  });
}

// Look up a project by its human-readable displayName and delete it by handle.
function deleteProjectByName(authToken, targetName) {
  cy.request({
    url: '/api/proxy/api/v0.9/projects',
    headers: { Authorization: `Bearer ${authToken}` },
    failOnStatusCode: false,
  }).then((response) => {
    if (response.status !== 200) return;
    const target = (response.body?.list ?? []).find(
      (p) => p.displayName === targetName
    );
    if (!target?.id) return;
    cy.request({
      method: 'DELETE',
      url: `/api/proxy/api/v0.9/projects/${encodeURIComponent(target.id)}`,
      headers: { Authorization: `Bearer ${authToken}` },
      failOnStatusCode: false,
    });
  });
}
