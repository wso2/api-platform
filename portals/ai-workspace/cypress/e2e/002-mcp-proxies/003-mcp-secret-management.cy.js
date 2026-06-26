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
 * Secret management behaviour for MCP server (external server) create flows.
 *
 * Covers:
 *   TC-80  Create MCP server with auth → POST /secrets called, placeholder stored
 *   TC-81  fetchServerInfo validation → POST /secrets NOT called
 *   TC-82  Re-submit form with existing placeholder → POST /secrets NOT called
 *   TC-83  POST /secrets 500 → MCP server creation aborted, no server created
 *   TC-84  Create MCP server without auth → no POST /secrets, no auth block in config
 *   TC-85  Secret handle is URL-safe slug derived from server name + "-auth" suffix
 */
describe('AI Workspace — MCP server secret management', () => {
  const suffix = Date.now().toString().slice(-8);
  const projectName = `E2E MCP Secret Project ${suffix}`;
  const cleanupProjectName = 'AI Workspace MCP Secret E2E Cleanup Project';
  const serverName = `E2E MCP Secret Server ${suffix}`;
  const serverId = toSlug(serverName);

  const SAMPLE_MCP_URL = 'https://sample.mcp.example.com/mcp';

  let authToken = '';
  let organizationId = '';
  let createdServerId = '';

  beforeEach(() => {
    cy.login();
    cy.request({
      method: 'POST',
      url: '/api-proxy/api/portal/v1/auth/login',
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
          url: '/api-proxy/api/v0.9/organizations',
          headers: { Authorization: `Bearer ${authToken}` },
        });
      })
      .then((response) => {
        organizationId = response.body?.id ?? '';
      });
  });

  afterEach(() => {
    if (authToken && organizationId && createdServerId) {
      cy.request({
        method: 'DELETE',
        url: `/api-proxy/api/v0.9/mcp-proxies/${encodeURIComponent(createdServerId)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
      createdServerId = '';
    }
    deleteProjectByName(authToken, projectName, cleanupProjectName);
  });

  // ---------------------------------------------------------------------------
  // TC-80: Create MCP server with auth credential → POST /secrets called, placeholder stored
  // ---------------------------------------------------------------------------
  it('TC-80: creates a secret and stores the placeholder in the MCP server upstream config', () => {
    cy.intercept('POST', '**/secrets').as('createSecret');
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('createServer');

    createProjectAndNavigateToMCPCreate(projectName);

    fillMCPForm({
      name: serverName,
      endpointUrl: SAMPLE_MCP_URL,
      authHeader: 'Authorization',
      authValue: 'Bearer tok-tc80-plaintext',
    });

    cy.contains('button', 'Create', { timeout: 15000 })
      .should('not.be.disabled')
      .click();

    // Secret must be created first with the plaintext value.
    cy.wait('@createSecret').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      // The UI posts multipart/form-data; assert on the response body instead.
      const handle = interception.response.body?.handle;
      expect(handle).to.match(/-auth$/);
    });

    // MCP server payload must contain a placeholder, not the plaintext.
    cy.wait('@createServer').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      const bodyStr = JSON.stringify(interception.request.body);
      expect(bodyStr).to.include('{{ secret ');
      expect(bodyStr).not.to.include('tok-tc80-plaintext');
      createdServerId =
        interception.response.body?.id ??
        interception.response.body?.handle ??
        serverId;
    });
  });

  // ---------------------------------------------------------------------------
  // TC-81: fetchServerInfo validation → POST /secrets NOT called during fetch
  // ---------------------------------------------------------------------------
  it('TC-81: does not call POST /secrets when only validating the server endpoint', () => {
    let secretCallCount = 0;
    cy.intercept('POST', '**/secrets', (req) => {
      secretCallCount += 1;
      req.continue();
    });
    // Stub the validation call so the test does not depend on sample.mcp.example.com
    // being reachable. The assertion is that /secrets is NOT called during fetch — the
    // stub response content does not matter for that check.
    cy.intercept('POST', '**/mcp-proxies/fetch-server-info*', {
      statusCode: 200,
      body: {
        serverInfo: { name: 'Stub MCP Server', version: '1.0.0' },
        tools: [],
        resources: [],
        prompts: [],
      },
    }).as('fetchInfo');

    createProjectAndNavigateToMCPCreate(projectName);

    cy.get('input[placeholder="Enter URL of Your MCP Proxy"]', { timeout: 30000 })
      .should('be.visible')
      .type(SAMPLE_MCP_URL);

    cy.contains('Advanced Configurations', { timeout: 10000 }).click();
    cy.get('input[placeholder="Header"]', { timeout: 10000 })
      .should('be.visible')
      .type('Authorization');
    cy.get('input[placeholder="Value"]', { timeout: 10000 })
      .should('be.visible')
      .type('Bearer tok-tc81-validate-only');

    cy.contains('button', 'Fetch Server Info', { timeout: 15000 })
      .should('be.visible')
      .click();

    // After the fetch request completes, assert /secrets was never called.
    cy.wait('@fetchInfo', { timeout: 30000 }).then(() => {
      expect(secretCallCount).to.equal(0);
    });
  });

  // ---------------------------------------------------------------------------
  // TC-82: Re-submit form with existing placeholder → POST /secrets NOT called
  // ---------------------------------------------------------------------------
  it('TC-82: does not create a duplicate secret when auth value is already a placeholder', () => {
    const existingHandle = `${serverId}-auth`;
    cy.request({
      method: 'POST',
      url: '/api-proxy/api/v0.9/secrets',
      headers: {
        Authorization: `Bearer ${authToken}`,
        'Content-Type': 'application/json',
      },
      body: {
        handle: existingHandle,
        displayName: `${serverName} upstream auth`,
        value: 'Bearer tok-tc82-original',
        type: 'GENERIC',
      },
      failOnStatusCode: false,
    }).then((r) => {
      expect(r.status).to.be.oneOf([200, 201, 409]);
    });

    let secretCallCount = 0;
    cy.intercept('POST', '**/secrets', (req) => {
      secretCallCount += 1;
      req.continue();
    });
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('createServer');

    createProjectAndNavigateToMCPCreate(projectName);

    fillMCPForm({
      name: serverName,
      endpointUrl: SAMPLE_MCP_URL,
      authHeader: 'Authorization',
      authValue: `{{ secret "${existingHandle}" }}`,
      authValueParseSpecial: false,
    });

    cy.contains('button', 'Create', { timeout: 15000 })
      .should('not.be.disabled')
      .click();

    cy.wait('@createServer').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      createdServerId =
        interception.response.body?.id ??
        interception.response.body?.handle ??
        serverId;
      // Evaluate secretCallCount inside .then() — after the queue has settled.
      expect(secretCallCount).to.equal(0);
    });
  });

  // ---------------------------------------------------------------------------
  // TC-83: POST /secrets 500 → MCP server creation aborted, no server created
  // ---------------------------------------------------------------------------
  it('TC-83: aborts MCP server creation when POST /secrets returns 500', () => {
    cy.intercept('POST', '**/secrets', {
      statusCode: 500,
      body: { error: 'simulated vault failure' },
    }).as('failSecret');

    let serverCallCount = 0;
    cy.intercept('POST', /\/mcp-proxies(\?|$)/, (req) => {
      serverCallCount += 1;
      req.continue();
    });

    createProjectAndNavigateToMCPCreate(projectName);

    fillMCPForm({
      name: serverName,
      endpointUrl: SAMPLE_MCP_URL,
      authHeader: 'Authorization',
      authValue: 'Bearer tok-tc83-will-fail',
    });

    cy.contains('button', 'Create', { timeout: 15000 })
      .should('not.be.disabled')
      .click();

    cy.wait('@failSecret');

    // App surfaces errors through the Notification component with a stable testId.
    cy.get('[data-testid="aiworkspace-snackbar-notification"]', { timeout: 15000 })
      .should('be.visible');

    cy.wrap(null).then(() => {
      expect(serverCallCount).to.equal(0);
    });
  });

  // ---------------------------------------------------------------------------
  // TC-84: Create without auth → no POST /secrets, no auth block in persisted config
  // ---------------------------------------------------------------------------
  it('TC-84: does not create a secret and omits auth from config when no auth is provided', () => {
    let secretCallCount = 0;
    cy.intercept('POST', '**/secrets', (req) => {
      secretCallCount += 1;
      req.continue();
    });
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('createServer');

    createProjectAndNavigateToMCPCreate(projectName);

    // Leave auth fields blank.
    fillMCPForm({
      name: serverName,
      endpointUrl: SAMPLE_MCP_URL,
    });

    cy.contains('button', 'Create', { timeout: 15000 })
      .should('not.be.disabled')
      .click();

    cy.wait('@createServer').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      const body = interception.request.body;
      expect(body?.upstream?.main?.auth).to.be.undefined;
      expect(JSON.stringify(body)).not.to.include('{{ secret ');
      createdServerId =
        interception.response.body?.id ??
        interception.response.body?.handle ??
        serverId;
      // Evaluate counter after queue settles.
      expect(secretCallCount).to.equal(0);
    });
  });

  // ---------------------------------------------------------------------------
  // TC-85: Secret handle is URL-safe slug derived from server name + "-auth" suffix
  // ---------------------------------------------------------------------------
  it('TC-85: derives a URL-safe secret handle from the server name with an "-auth" suffix', () => {
    const mixedName = `My MCP ${suffix}`;
    const expectedHandle = `${toSlug(mixedName)}-auth`;

    cy.intercept('POST', '**/secrets').as('createSecret');
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('createServer');

    createProjectAndNavigateToMCPCreate(projectName);

    fillMCPForm({
      name: mixedName,
      endpointUrl: SAMPLE_MCP_URL,
      authHeader: 'Authorization',
      authValue: 'Bearer tok-tc85-handle-check',
    });

    cy.contains('button', 'Create', { timeout: 15000 })
      .should('not.be.disabled')
      .click();

    cy.wait('@createSecret').then((interception) => {
      // The UI posts multipart/form-data; assert on the response body instead.
      const handle = interception.response.body?.handle;
      expect(handle).to.match(/^[a-z0-9-]+-auth$/);
      expect(handle).to.equal(expectedHandle);
    });

    cy.wait('@createServer').then((interception) => {
      createdServerId =
        interception.response.body?.id ??
        interception.response.body?.handle ??
        toSlug(mixedName);
    });
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
      'Cypress MCP secret management project'
    );
    cy.contains('button', 'Create').should('not.be.disabled').click();
    cy.wait('@createProject').its('response.statusCode').should('be.oneOf', [200, 201]);

    cy.contains(name, { timeout: 30000 }).should('be.visible').click();
    cy.contains('MCP Proxies', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', 'Create MCP Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();
  }

  /**
   * Stubs fetch-server-info and fills the MCP server create form (both steps).
   * Always stubs the validation endpoint so tests do not need a real MCP server.
   *
   * @param {object} opts
   * @param {string}  opts.name         Server display name
   * @param {string}  opts.endpointUrl  Server endpoint URL
   * @param {string}  [opts.authHeader] Auth header name (optional)
   * @param {string}  [opts.authValue]  Auth header value (optional)
   */
  function fillMCPForm({ name, endpointUrl, authHeader, authValue, authValueParseSpecial = true }) {
    // Stub the validation call so we don't need a real MCP server to get past step 1.
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

    if (authHeader && authValue) {
      // Auth fields are inside the "Advanced Configurations" collapsible panel.
      cy.contains('Advanced Configurations', { timeout: 10000 }).click();
      cy.get('input[placeholder="Header"]', { timeout: 10000 })
        .should('be.visible')
        .type(authHeader);
      cy.get('input[placeholder="Value"]', { timeout: 10000 })
        .should('be.visible')
        .type(authValue, authValueParseSpecial ? {} : { parseSpecialCharSequences: false });
    }

    cy.contains('button', 'Fetch Server Info', { timeout: 15000 })
      .should('be.visible')
      .click();
    cy.wait('@stubFetchInfo');

    // "Next" only appears after validationResult is set.
    cy.contains('button', 'Next', { timeout: 15000 })
      .should('be.visible')
      .click();

    // Step 2: fill server name.
    cy.get('input[placeholder="WSO2 MCP Proxy"]', { timeout: 15000 })
      .should('be.visible')
      .clear()
      .type(name);
  }
});

// ---------------------------------------------------------------------------
// API helpers (afterEach cleanup only — not used in test flow to avoid
// direct backend dependency from the test runner host).
// ---------------------------------------------------------------------------

function deleteProjectByName(authToken, targetName, fallbackName) {
  if (!authToken) return;
  cy.request({
    url: '/api-proxy/api/v0.9/projects',
    headers: { Authorization: `Bearer ${authToken}` },
    failOnStatusCode: false,
  }).then((response) => {
    if (response.status !== 200) return;
    const projects = response.body?.list ?? [];
    const target = projects.find((p) => p.name === targetName);
    if (!target?.id) return;

    if (projects.length <= 1) {
      cy.request({
        method: 'POST',
        url: '/api-proxy/api/v0.9/projects',
        headers: {
          Authorization: `Bearer ${authToken}`,
          'Content-Type': 'application/json',
        },
        body: { name: fallbackName, description: 'Reserved for E2E cleanup' },
        failOnStatusCode: false,
      }).then(() => {
        cy.request({
          method: 'DELETE',
          url: `/api-proxy/api/v0.9/projects/${encodeURIComponent(target.id)}`,
          headers: { Authorization: `Bearer ${authToken}` },
          failOnStatusCode: false,
        });
      });
    } else {
      cy.request({
        method: 'DELETE',
        url: `/api-proxy/api/v0.9/projects/${encodeURIComponent(target.id)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
    }
  });
}

function toSlug(value) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
