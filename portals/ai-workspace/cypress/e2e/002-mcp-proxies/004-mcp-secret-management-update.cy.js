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
 * Secret management behaviour for MCP server (external server) update flow.
 *
 * There is no UI to change upstream.main.auth.value after creation; the
 * credential is set only at create time. `auth.value` is `writeOnly` in the
 * API (platform-api/resources/openapi.yaml), so GET /mcp-proxies/{id} never
 * returns it — the Policies tab's locally cached server object never has it
 * either. When the user saves policies, PUT /mcp-proxies/{id} re-sends that
 * object, so the request body legitimately omits auth.value while keeping
 * auth.header/auth.type intact. The backend's preserveMCPUpstreamAuthValue
 * (platform-api/internal/service/mcp.go) restores the existing stored value
 * from the DB whenever the incoming value is empty, so the persisted
 * credential is not lost. These tests verify that:
 *
 *   TC-96  Saving policies with existing auth → PUT /mcp-proxies keeps the
 *          auth block's header/type (but omits value, by design) and
 *          POST /secrets is NOT called
 *   TC-97  Saving policies on a server created WITHOUT auth → PUT /mcp-proxies
 *          has no auth block and POST /secrets is NOT called
 */

describe('AI Workspace — MCP server secret management (update / policy-save flow)', () => {
  const suffix = Date.now().toString().slice(-8);
  const projectName = `E2E MCP Update Secret Project ${suffix}`;
  const serverName = `E2E MCP Update Server ${suffix}`;

  let authToken = '';
  let organizationId = '';
  let createdProjectId = '';
  let createdServerId = '';

  // Create a project + MCP server via UI once before all tests.
  beforeEach(() => {
    cy.login();

    cy.request({
      method: 'POST',
      url: '/api/proxy/api/portal/v0.9/auth/login',
      form: true,
      body: {
        username: Cypress.env('ADMIN_USER'),
        password: Cypress.env('ADMIN_PASSWORD'),
      },
    }).then((r) => { authToken = r.body?.token ?? ''; });

    cy.then(() =>
      cy.request({
        url: '/api/proxy/api/v0.9/organizations',
        headers: { Authorization: `Bearer ${authToken}` },
      })
    ).then((r) => { organizationId = r.body?.list?.[0]?.id ?? ''; });

    cy.intercept('POST', '**/projects').as('setupProject');
    cy.intercept('POST', '**/secrets').as('setupSecret');
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('setupServer');
    // Registered up front so it can't miss the client-side navigation to
    // /mcp-proxy/:id right after creation (see usage below).
    cy.intercept('GET', /\/mcp-proxies\/[^/?]+(\?|$)/).as('getServerDetails');

    cy.contains('Projects', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', /Create Project|Add New Project/, { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
      .should('be.visible')
      .type(projectName);
    cy.get('textarea[placeholder="Short description of the project."]')
      .type('MCP update secret test project');
    cy.contains('button', 'Create').should('not.be.disabled').click();
    cy.wait('@setupProject', { timeout: 20000 }).then((pi) => {
      createdProjectId = pi.response.body?.id ?? '';
    });

    cy.contains(projectName, { timeout: 30000 }).should('be.visible').click();
    cy.contains('MCP Proxies', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', 'Create MCP Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.intercept('POST', '**/fetch-server-info*', {
      statusCode: 200,
      body: {
        serverInfo: { name: 'Stub MCP Server', version: '1.0.0' },
        tools: [],
        resources: [],
        prompts: [],
      },
    }).as('stubFetch');

    cy.contains('Create MCP Proxy from Endpoint', { timeout: 30000 }).should('be.visible');
    cy.get('input[placeholder="Enter URL of Your MCP Proxy"]', { timeout: 15000 })
      .should('be.visible')
      .type('https://sample.mcp.example.com/mcp');
    cy.contains('Advanced Configurations', { timeout: 10000 }).click();
    cy.get('input[placeholder="Header"]', { timeout: 10000 })
      .should('be.visible')
      .type('Authorization');
    cy.get('input[placeholder="Value"]', { timeout: 10000 })
      .should('be.visible')
      .type('Bearer tok-setup-key');

    cy.contains('button', 'Fetch Server Info', { timeout: 15000 })
      .should('be.visible')
      .click();
    cy.wait('@stubFetch');

    cy.contains('button', 'Next', { timeout: 15000 }).should('be.visible').click();

    cy.get('input[placeholder="WSO2 MCP Proxy"]', { timeout: 15000 })
      .should('be.visible')
      .clear()
      .type(serverName);

    cy.contains('button', 'Create', { timeout: 15000 }).should('not.be.disabled').click();

    cy.wait('@setupSecret', { timeout: 20000 });
    cy.wait('@setupServer', { timeout: 20000 }).then((pi) => {
      createdServerId = pi.response.body?.id ?? '';
    });

    // The Overview page re-fetches the server on mount and derives the initial
    // policy list from it (ExternalServersOverview's mapPolicies effect), which
    // itself calls GET **/policies*. Wait for that fetch to settle before
    // switching tabs, so it can't race with the test's own '@getPolicies' wait
    // below on 'Add Policies'.
    cy.location('pathname', { timeout: 30000 }).should('match', /\/mcp-proxy\/[^/]+$/);
    cy.wait('@getServerDetails', { timeout: 20000 });
    cy.contains('[role="tab"]', 'Policies', { timeout: 15000 }).click();
  });

  afterEach(() => {
    if (createdServerId && authToken) {
      cy.request({
        method: 'DELETE',
        url: `/api/proxy/api/v0.9/mcp-proxies/${encodeURIComponent(createdServerId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
      createdServerId = '';
    }
    if (createdProjectId && authToken) {
      cy.request({
        method: 'DELETE',
        url: `/api/proxy/api/v0.9/projects/${encodeURIComponent(createdProjectId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
      createdProjectId = '';
    }
  });

  // -------------------------------------------------------------------------
  // TC-96: Saving policies when server has an existing auth →
  //        PUT keeps the auth block's header/type (value is correctly
  //        omitted — writeOnly and never present in the locally cached
  //        server object), no POST /secrets called.
  // -------------------------------------------------------------------------
  it('TC-96: saving policies keeps the existing auth header/type without creating a new secret', () => {
    let secretCallCount = 0;
    cy.intercept('POST', '**/secrets', (req) => { secretCallCount += 1; req.continue(); });
    cy.intercept('PUT', /\/mcp-proxies\/[^/?]+(\?|$)/).as('updateServer');

    // Save/Cancel are disabled until the policy list actually changes (dirty-check
    // on selectedPolicies vs initialPolicies), so add one policy to enable Save.
    cy.intercept('GET', '**/policies*').as('getPolicies');
    cy.contains('button', 'Add Policies', { timeout: 15000 }).click();
    cy.wait('@getPolicies', { timeout: 20000 });
    cy.contains('CORS', { timeout: 15000 }).click();
    cy.get('[data-testid="policy-param-submit"]', { timeout: 15000 }).should('not.be.disabled').click();

    cy.contains('button', 'Save', { timeout: 15000 }).should('not.be.disabled').click();

    cy.wait('@updateServer', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /mcp-proxies status').to.be.oneOf([200, 201]);
      const auth = pi.request.body?.upstream?.main?.auth;
      // The auth block structure must survive the save — header/type intact.
      expect(auth?.header, 'PUT body keeps the auth header').to.equal('Authorization');
      expect(auth?.type, 'PUT body keeps the auth type').to.equal('header');
      // value is writeOnly (never returned by GET), so the PUT correctly omits
      // it; the backend's preserveMCPUpstreamAuthValue restores the stored
      // value from the DB when it sees an empty value on update.
      expect(auth?.value, 'PUT body omits value (writeOnly, not a data loss)').to.be.undefined;
      // Plaintext must NOT appear.
      const bodyStr = JSON.stringify(pi.request.body);
      expect(bodyStr, 'PUT body has no plaintext key').not.to.include('tok-setup-key');
      // Verify no secret was created.
      cy.wrap(null).then(() => {
        expect(secretCallCount, 'POST /secrets not called').to.equal(0);
      });
    });
  });
});

// ---------------------------------------------------------------------------
// TC-97: Separate describe — server WITHOUT auth, save policies
// ---------------------------------------------------------------------------

describe('AI Workspace — MCP server secret management (update / no-auth server)', () => {
  const suffix2 = (Date.now() + 1).toString().slice(-8);
  const projectName2 = `E2E MCP Update NoAuth Project ${suffix2}`;
  const serverName2 = `E2E MCP Update NoAuth Server ${suffix2}`;

  let authToken2 = '';
  let organizationId2 = '';
  let createdProjectId2 = '';
  let createdServerId2 = '';

  beforeEach(() => {
    cy.login();

    cy.request({
      method: 'POST',
      url: '/api/proxy/api/portal/v0.9/auth/login',
      form: true,
      body: {
        username: Cypress.env('ADMIN_USER'),
        password: Cypress.env('ADMIN_PASSWORD'),
      },
    }).then((r) => { authToken2 = r.body?.token ?? ''; });

    cy.then(() =>
      cy.request({
        url: '/api/proxy/api/v0.9/organizations',
        headers: { Authorization: `Bearer ${authToken2}` },
      })
    ).then((r) => { organizationId2 = r.body?.list?.[0]?.id ?? ''; });

    cy.intercept('POST', '**/projects').as('setupProject2');
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('setupServer2');
    cy.intercept('GET', /\/mcp-proxies\/[^/?]+(\?|$)/).as('getServerDetails2');

    cy.contains('Projects', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', /Create Project|Add New Project/, { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
      .should('be.visible')
      .type(projectName2);
    cy.get('textarea[placeholder="Short description of the project."]')
      .type('MCP update no-auth test project');
    cy.contains('button', 'Create').should('not.be.disabled').click();
    cy.wait('@setupProject2', { timeout: 20000 }).then((pi) => {
      createdProjectId2 = pi.response.body?.id ?? '';
    });

    cy.contains(projectName2, { timeout: 30000 }).should('be.visible').click();
    cy.contains('MCP Proxies', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', 'Create MCP Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.intercept('POST', '**/fetch-server-info*', {
      statusCode: 200,
      body: {
        serverInfo: { name: 'Stub MCP Server', version: '1.0.0' },
        tools: [],
        resources: [],
        prompts: [],
      },
    }).as('stubFetch2');

    cy.contains('Create MCP Proxy from Endpoint', { timeout: 30000 }).should('be.visible');
    cy.get('input[placeholder="Enter URL of Your MCP Proxy"]', { timeout: 15000 })
      .should('be.visible')
      .type('https://sample.mcp.example.com/mcp');

    cy.contains('button', 'Fetch Server Info', { timeout: 15000 })
      .should('be.visible')
      .click();
    cy.wait('@stubFetch2');

    cy.contains('button', 'Next', { timeout: 15000 }).should('be.visible').click();

    cy.get('input[placeholder="WSO2 MCP Proxy"]', { timeout: 15000 })
      .should('be.visible')
      .clear()
      .type(serverName2);

    cy.contains('button', 'Create', { timeout: 15000 }).should('not.be.disabled').click();

    cy.wait('@setupServer2', { timeout: 20000 }).then((pi) => {
      createdServerId2 = pi.response.body?.id ?? '';
    });

    cy.location('pathname', { timeout: 30000 }).should('match', /\/mcp-proxy\/[^/]+$/);
    cy.wait('@getServerDetails2', { timeout: 20000 });
    cy.contains('[role="tab"]', 'Policies', { timeout: 15000 }).click();
  });

  afterEach(() => {
    if (createdServerId2 && authToken2) {
      cy.request({
        method: 'DELETE',
        url: `/api/proxy/api/v0.9/mcp-proxies/${encodeURIComponent(createdServerId2)}`,
        headers: { Authorization: `Bearer ${authToken2}` },
        failOnStatusCode: false,
      });
      createdServerId2 = '';
    }
    if (createdProjectId2 && authToken2) {
      cy.request({
        method: 'DELETE',
        url: `/api/proxy/api/v0.9/projects/${encodeURIComponent(createdProjectId2)}`,
        headers: { Authorization: `Bearer ${authToken2}` },
        failOnStatusCode: false,
      });
      createdProjectId2 = '';
    }
  });

  // -------------------------------------------------------------------------
  // TC-97: Saving policies when server has NO auth →
  //        PUT has no auth block and no POST /secrets called
  // -------------------------------------------------------------------------
  it('TC-97: saving policies on a server without auth does not create a secret and omits auth from PUT', () => {
    let secretCallCount = 0;
    cy.intercept('POST', '**/secrets', (req) => { secretCallCount += 1; req.continue(); });
    cy.intercept('PUT', /\/mcp-proxies\/[^/?]+(\?|$)/).as('updateServer');

    // Save/Cancel are disabled until the policy list actually changes (dirty-check
    // on selectedPolicies vs initialPolicies), so add one policy to enable Save.
    cy.intercept('GET', '**/policies*').as('getPolicies2');
    cy.contains('button', 'Add Policies', { timeout: 15000 }).click();
    cy.wait('@getPolicies2', { timeout: 20000 });
    cy.contains('CORS', { timeout: 15000 }).click();
    cy.get('[data-testid="policy-param-submit"]', { timeout: 15000 }).should('not.be.disabled').click();

    cy.contains('button', 'Save', { timeout: 15000 }).should('not.be.disabled').click();

    cy.wait('@updateServer', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /mcp-proxies status').to.be.oneOf([200, 201]);
      const body = pi.request.body;
      expect(body?.upstream?.main?.auth, 'no auth block in PUT body').to.be.undefined;
      expect(JSON.stringify(body), 'no placeholder in PUT body').not.to.include('{{ secret "');
      cy.wrap(null).then(() => {
        expect(secretCallCount, 'POST /secrets not called').to.equal(0);
      });
    });
  });
});
