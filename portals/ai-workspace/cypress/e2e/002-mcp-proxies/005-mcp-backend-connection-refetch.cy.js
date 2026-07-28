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
 * "Refetch Server Info" behaviour on the MCP Proxy — Backend Connection tab.
 *
 * ExternalServersOverview's handleRefetch has two mutually-exclusive request
 * shapes, chosen by whether the endpoint URL / auth header / auth value differ
 * from what is currently persisted on the server:
 *
 *   - Unedited: POST /mcp-proxies/fetch-server-info { url, proxyId } — the
 *     backend resolves the stored URL and auth (via the saved secret handle)
 *     itself, so the browser never needs to re-send a credential it can't
 *     read back (auth.value is writeOnly).
 *   - Edited: POST /mcp-proxies/fetch-server-info { url, auth: { type,
 *     header, value } } — validates the live, not-yet-saved values directly.
 *     proxyId and an auth override are mutually exclusive by API contract, so
 *     this omits proxyId.
 *
 * Covers:
 *   TC-98  Refetch with nothing edited → request uses proxyId, omits auth
 *   TC-99  Refetch after editing endpoint/header/value → request sends the
 *          live values directly (plaintext value, since it was just typed),
 *          omits proxyId
 *   TC-100 Saving the edited connection → PUT stores a NEW secret placeholder
 *          (never the plaintext); a subsequent refetch (now unedited again)
 *          goes back to proxyId mode
 *   TC-101 Saving a URL-only edit (credential left untouched/masked) → PUT
 *          preserves the existing auth header/type, omits value (relying on
 *          the backend's preserveMCPUpstreamAuthValue fallback, same as the
 *          Policies-only save path), and creates no new secret
 */
describe('AI Workspace — MCP proxy Backend Connection tab (Refetch Server Info)', () => {
  const suffix = Date.now().toString().slice(-8);
  const projectName = `E2E MCP Refetch Project ${suffix}`;
  const serverName = `E2E MCP Refetch Server ${suffix}`;

  const ORIGINAL_URL = 'https://sample.mcp.example.com/mcp';
  const ORIGINAL_HEADER = 'Authorization';
  const ORIGINAL_VALUE = 'Bearer tok-setup-key';

  let authToken = '';
  let organizationId = '';
  let createdProjectId = '';
  let createdServerId = '';

  const stubFetchServerInfo = (alias) =>
    cy.intercept('POST', '**/fetch-server-info*', {
      statusCode: 200,
      body: {
        serverInfo: { name: 'Stub MCP Server', version: '1.0.0' },
        tools: [],
        resources: [],
        prompts: [],
      },
    }).as(alias);

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
    }).then((r) => { authToken = r.body?.token ?? ''; });

    cy.then(() =>
      cy.request({
        url: '/proxy/api/v0.9/organizations',
        headers: { Authorization: `Bearer ${authToken}` },
      })
    ).then((r) => { organizationId = r.body?.list?.[0]?.id ?? ''; });

    cy.intercept('POST', '**/projects').as('setupProject');
    cy.intercept('POST', '**/secrets').as('setupSecret');
    cy.intercept('POST', /\/mcp-proxies(\?|$)/).as('setupServer');
    // Registered up front so it can't miss the client-side navigation to
    // /mcp-proxy/:id right after creation.
    cy.intercept('GET', /\/mcp-proxies\/[^/?]+(\?|$)/).as('getServerDetails');

    cy.contains('Projects', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', /Create Project|Add New Project/, { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
      .should('be.visible')
      .type(projectName);
    cy.get('textarea[placeholder="Short description of the project."]')
      .type('MCP Backend Connection refetch test project');
    cy.contains('button', 'Create').should('not.be.disabled').click();
    cy.wait('@setupProject', { timeout: 20000 }).then((pi) => {
      createdProjectId = pi.response.body?.id ?? '';
    });

    cy.contains(projectName, { timeout: 30000 }).should('be.visible').click();
    cy.contains('MCP Proxies', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', 'Create MCP Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();

    stubFetchServerInfo('stubFetch');

    cy.contains('Create MCP Proxy from Endpoint', { timeout: 30000 }).should('be.visible');
    cy.get('input[placeholder="Enter URL of Your MCP Proxy"]', { timeout: 15000 })
      .should('be.visible')
      .type(ORIGINAL_URL);
    cy.contains('Advanced Configurations', { timeout: 10000 }).click();
    cy.get('input[placeholder="Header"]', { timeout: 10000 })
      .should('be.visible')
      .type(ORIGINAL_HEADER);
    cy.get('input[placeholder="Value"]', { timeout: 10000 })
      .should('be.visible')
      .type(ORIGINAL_VALUE);

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

    cy.location('pathname', { timeout: 30000 }).should('match', /\/mcp-proxy\/[^/]+$/);
    cy.wait('@getServerDetails', { timeout: 20000 });
    cy.contains('[role="tab"]', 'Backend Connection', { timeout: 15000 }).click();

    // Sanity check: the tab loaded with the values set at creation. The Value
    // field is write-only server-side, so it shows the masked sentinel, not
    // the real credential.
    cy.get('[data-testid="backend-connection-endpoint-url"]', { timeout: 15000 })
      .should('have.value', ORIGINAL_URL);
    cy.get('[data-testid="backend-connection-auth-header"]')
      .should('have.value', ORIGINAL_HEADER);
    cy.get('[data-testid="backend-connection-auth-value"]')
      .should('have.value', '******');
  });

  afterEach(() => {
    if (createdServerId && authToken) {
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/mcp-proxies/${encodeURIComponent(createdServerId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
      createdServerId = '';
    }
    if (createdProjectId && authToken) {
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
  // TC-98
  // ---------------------------------------------------------------------------
  it('TC-98: refetching without any edits uses proxyId and omits auth entirely', () => {
    stubFetchServerInfo('refetch');

    cy.get('[data-testid="backend-connection-refetch"]', { timeout: 15000 })
      .should('not.be.disabled')
      .click();

    cy.wait('@refetch').then((pi) => {
      const body = pi.request.body;
      expect(body.proxyId, 'refetch request carries proxyId').to.equal(createdServerId);
      expect(body.url, 'refetch request still carries the current url').to.equal(ORIGINAL_URL);
      expect(body.auth, 'no auth override sent alongside proxyId').to.be.undefined;
    });

    cy.contains('Connection verified', { timeout: 15000 }).should('be.visible');
  });

  // ---------------------------------------------------------------------------
  // TC-99
  // ---------------------------------------------------------------------------
  it('TC-99: refetching after editing the endpoint, header, and value sends the live values directly', () => {
    const newUrl = 'https://updated.mcp.example.com/mcp';
    const newHeader = 'X-Api-Key';
    const newValue = 'super-secret-live-value';

    cy.get('[data-testid="backend-connection-endpoint-url"]')
      .clear()
      .type(newUrl);
    cy.get('[data-testid="backend-connection-auth-header"]')
      .clear()
      .type(newHeader);
    // The Value field is masked; focusing it clears the sentinel for editing.
    cy.get('[data-testid="backend-connection-auth-value"]')
      .focus()
      .clear()
      .type(newValue);

    stubFetchServerInfo('refetch');

    cy.get('[data-testid="backend-connection-refetch"]', { timeout: 15000 })
      .should('not.be.disabled')
      .click();

    cy.wait('@refetch').then((pi) => {
      const body = pi.request.body;
      expect(body.proxyId, 'proxyId is omitted once fields are edited').to.be.undefined;
      expect(body.url, 'refetch validates the new, unsaved url').to.equal(newUrl);
      expect(body.auth?.type, 'auth type').to.equal('header');
      expect(body.auth?.header, 'auth header name').to.equal(newHeader);
      expect(body.auth?.value, 'auth value is the live plaintext just typed').to.equal(newValue);
    });

    cy.contains('Connection verified', { timeout: 15000 }).should('be.visible');
  });

  // ---------------------------------------------------------------------------
  // TC-100
  // ---------------------------------------------------------------------------
  it('TC-100: saving edited connection details rotates the secret, and a later refetch goes back to proxyId mode', () => {
    const newUrl = 'https://updated-and-saved.mcp.example.com/mcp';
    const newHeader = 'X-Api-Key';
    const newValue = 'super-secret-value-to-be-saved';

    let secretCallCount = 0;
    cy.intercept('POST', '**/secrets', (req) => { secretCallCount += 1; req.continue(); });
    cy.intercept('PUT', /\/mcp-proxies\/[^/?]+(\?|$)/).as('updateServer');

    cy.get('[data-testid="backend-connection-endpoint-url"]')
      .clear()
      .type(newUrl);
    cy.get('[data-testid="backend-connection-auth-header"]')
      .clear()
      .type(newHeader);
    cy.get('[data-testid="backend-connection-auth-value"]')
      .focus()
      .clear()
      .type(newValue);

    cy.contains('button', 'Save', { timeout: 15000 }).should('not.be.disabled').click();

    cy.wait('@updateServer', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /mcp-proxies status').to.be.oneOf([200, 201]);
      const auth = pi.request.body?.upstream?.main?.auth;
      expect(pi.request.body?.upstream?.main?.url, 'PUT carries the new url').to.equal(newUrl);
      expect(auth?.header, 'PUT carries the new header name').to.equal(newHeader);
      // The rotated credential must be a secret placeholder, never the plaintext.
      expect(auth?.value, 'PUT sends a secret placeholder').to.match(/^\{\{ secret ".+" \}\}$/);
      const bodyStr = JSON.stringify(pi.request.body);
      expect(bodyStr, 'PUT body has no plaintext credential').not.to.include(newValue);
      cy.wrap(null).then(() => {
        expect(secretCallCount, 'exactly one new secret created for the rotation').to.equal(1);
      });
    });

    // After a successful save the tab re-populates from the (now current) server
    // object: the value field re-masks, so an unedited refetch should use proxyId
    // again rather than the value just saved.
    cy.get('[data-testid="backend-connection-auth-value"]', { timeout: 15000 })
      .should('have.value', '******');

    stubFetchServerInfo('refetchAfterSave');
    cy.get('[data-testid="backend-connection-refetch"]', { timeout: 15000 })
      .should('not.be.disabled')
      .click();

    cy.wait('@refetchAfterSave').then((pi) => {
      const body = pi.request.body;
      expect(body.proxyId, 'refetch after save uses proxyId again').to.equal(createdServerId);
      expect(body.url, 'refetch after save uses the newly saved url').to.equal(newUrl);
      expect(body.auth, 'no auth override needed post-save').to.be.undefined;
    });
  });

  // ---------------------------------------------------------------------------
  // TC-101
  // ---------------------------------------------------------------------------
  it('TC-101: saving a URL-only edit preserves the existing auth header/type without creating a new secret', () => {
    const newUrl = 'https://url-only-edit.mcp.example.com/mcp';

    let secretCallCount = 0;
    cy.intercept('POST', '**/secrets', (req) => { secretCallCount += 1; req.continue(); });
    cy.intercept('PUT', /\/mcp-proxies\/[^/?]+(\?|$)/).as('updateServer');

    // Only the endpoint URL changes — the auth header/value fields are left exactly
    // as populated at load (value stays masked, never focused).
    cy.get('[data-testid="backend-connection-endpoint-url"]')
      .clear()
      .type(newUrl);

    cy.contains('button', 'Save', { timeout: 15000 }).should('not.be.disabled').click();

    cy.wait('@updateServer', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /mcp-proxies status').to.be.oneOf([200, 201]);
      const body = pi.request.body;
      const auth = body?.upstream?.main?.auth;
      expect(body?.upstream?.main?.url, 'PUT carries the new url').to.equal(newUrl);
      // The existing credential must survive an unrelated (URL-only) edit — this is
      // the bug: auth was previously dropped (auth: undefined) whenever the save
      // path didn't rotate the credential, because resolvedAuthValue was seeded from
      // the never-populated (writeOnly) server.upstream.main.auth.value.
      expect(auth, 'auth block is preserved, not dropped, for a URL-only edit').to.exist;
      expect(auth?.header, 'existing auth header survives the save').to.equal(ORIGINAL_HEADER);
      expect(auth?.type, 'existing auth type survives the save').to.equal('header');
      // value is writeOnly and never cached client-side, so it's correctly omitted —
      // the backend's preserveMCPUpstreamAuthValue restores the stored value.
      expect(auth?.value, 'value omitted, not stripped (backend preserves it)').to.be.undefined;
      cy.wrap(null).then(() => {
        expect(secretCallCount, 'no new secret created for a non-credential edit').to.equal(0);
      });
    });
  });
});
