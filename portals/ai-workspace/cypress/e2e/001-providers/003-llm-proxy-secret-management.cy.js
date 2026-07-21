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
 * Secret management behaviour for App LLM Proxy create and update flows.
 *
 * A proxy stores its own credential to call the underlying LLM provider in
 * `provider.auth` (separate from the provider's own `upstream.main.auth`).
 * Like providers, this value must never be persisted in plaintext.
 *
 * Create flow (TC-1 – TC-3):
 *   TC-1  Create proxy with plaintext API key → POST /secrets called, placeholder
 *         stored in provider.auth.value, plaintext absent from page
 *   TC-2  Re-save proxy already holding a placeholder → POST /secrets NOT called
 *   TC-3  POST /secrets 500 → proxy creation aborted, no proxy created
 *
 * Update flow via Provider tab (TC-4 – TC-6):
 *   Typing into the API Key field stages the new value locally (setLocalProxy) as
 *   you type — the page-level "Save" button is what actually persists it
 *   via PUT /llm-proxies/{id} and triggers secret rotation.
 *   TC-4  Edit API key with a new plaintext value → POST /secrets called, PUT
 *         body holds placeholder, old secret DELETEd afterward, plaintext absent
 *         from page
 *   TC-5  Edit API key by typing an explicit placeholder → POST /secrets NOT
 *         called, PUT fires with the placeholder as-is
 *   TC-6  POST /secrets 500 during edit → PUT /llm-proxies NOT called, error shown
 */

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

function toSlug(value) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function loginAndFetchAuthContext(setAuthToken, setOrganizationId) {
  cy.login();
  cy.request({
    method: 'POST',
    url: '/api/login',
    body: {
      username: Cypress.env('ADMIN_USER'),
      password: Cypress.env('ADMIN_PASSWORD'),
    },
  })
    .then((r) => {
      setAuthToken(r.body.accessToken);
      return cy.request({
        url: '/proxy/api/v0.9/organizations',
        headers: { Authorization: `Bearer ${r.body.accessToken}` },
      });
    })
    .then((r) => {
      setOrganizationId(r.body?.list?.[0]?.id ?? '');
    });
}

function createProjectViaUI(projectName) {
  cy.intercept('POST', '**/projects').as('createProject');
  cy.contains('Projects', { timeout: 30000 }).should('be.visible').click();
  cy.contains('button, a', /Create Project|Add New Project/, { timeout: 30000 })
    .should('be.visible')
    .click();
  cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
    .should('be.visible')
    .type(projectName);
  cy.get('textarea[placeholder="Short description of the project."]').type(
    'Cypress LLM proxy secret management project'
  );
  cy.contains('button', 'Create').should('not.be.disabled').click();
  cy.wait('@createProject').its('response.statusCode').should('be.oneOf', [200, 201]);
}

function createProviderViaUI(providerName) {
  cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProviderForProxy');
  cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 }).should('be.visible').click();
  cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 }).should('be.visible').click();
  cy.get('[data-cyid="provider-template-openai-card"]', { timeout: 30000 }).should('be.visible').click();
  cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
    .should('be.visible')
    .clear()
    .type(providerName);
  cy.get('[data-cyid="provider-api-key-input"] input:visible').type('sk-provider-backing-key');
  cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();
  return cy.wait('@createProviderForProxy', { timeout: 20000 }).then((pi) => pi.response.body?.id ?? '');
}

function navigateToCreateProxy(projectName) {
  cy.contains('button', 'Create App LLM Proxy', { timeout: 30000 }).should('be.visible').click();
  cy.contains('label', 'Projects', { timeout: 30000 }).parent().find('[role="combobox"]').click();
  cy.contains('[role="option"]', projectName, { timeout: 15000 }).click();
  cy.contains('button', 'Continue').should('not.be.disabled').click();
  cy.contains('Create App LLM Proxy', { timeout: 30000 }).should('be.visible');
}

// ---------------------------------------------------------------------------
// CREATE flow
// ---------------------------------------------------------------------------

describe('AI Workspace — LLM proxy secret management (create flow)', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const projectName = `E2E Proxy Secret Project ${suffix}`;
  const providerName = `E2E Proxy Secret Provider ${suffix}`;
  const proxyName = `E2E Proxy Secret Proxy ${suffix}`;

  let authToken = '';
  let organizationId = '';
  let createdProviderId = '';
  let createdProxyId = '';

  beforeEach(() => {
    createdProxyId = '';
    loginAndFetchAuthContext((v) => { authToken = v; }, (v) => { organizationId = v; });

    createProjectViaUI(projectName);
    createProviderViaUI(providerName).then((id) => {
      createdProviderId = id;
    });
    navigateToCreateProxy(projectName);
  });

  afterEach(() => {
    if (authToken && organizationId && createdProxyId) {
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/llm-proxies/${encodeURIComponent(createdProxyId)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
    }
    if (authToken && organizationId && createdProviderId) {
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(createdProviderId)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
    }
    if (authToken) {
      cy.request({
        url: '/proxy/api/v0.9/projects',
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      }).then((r) => {
        const proj = (r.body?.list ?? []).find((p) => p.displayName === projectName);
        if (proj?.id) {
          cy.request({
            method: 'DELETE',
            url: `/proxy/api/v0.9/projects/${encodeURIComponent(proj.id)}`,
            headers: { Authorization: `Bearer ${authToken}` },
            failOnStatusCode: false,
          });
        }
      });
    }
  });

  // -------------------------------------------------------------------------
  // TC-1: Create proxy with plaintext key → secret auto-created, placeholder stored
  // -------------------------------------------------------------------------
  it('TC-1: creates a secret and stores the placeholder in the proxy provider config', () => {
    cy.intercept('POST', '**/secrets').as('createSecret');
    cy.intercept('POST', /\/llm-proxies(\?|$)/).as('createProxy');

    cy.get('input[placeholder="WSO2 OpenAI Provider Proxy"]', { timeout: 30000 })
      .should('be.visible')
      .type(proxyName);
    cy.get('textarea[placeholder="Primary OpenAI provider"]').type('Cypress proxy secret create test');
    cy.get('input[placeholder="Enter API key"]').type('sk-tc1-proxy-plaintext-key');
    cy.contains('button', 'Create Proxy', { timeout: 30000 }).should('not.be.disabled').click();

    cy.wait('@createSecret').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      const secretId = interception.response.body?.id;
      expect(secretId).to.match(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
      expect(JSON.stringify(interception.response.body), 'plaintext not in secret response')
        .not.to.include('sk-tc1-proxy-plaintext-key');
    });

    cy.wait('@createProxy').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      const authValue = interception.request.body?.provider?.auth?.value ?? '';
      expect(authValue, 'proxy payload has placeholder').to.include('{{ secret "');
      expect(authValue, 'proxy payload does NOT have plaintext key').not.to.include('sk-tc1-proxy-plaintext-key');
      createdProxyId = interception.response.body?.id ?? '';
    });

    cy.location('pathname', { timeout: 30000 }).should('match', /\/proxies\/[^/]+$/);

    cy.get('body').invoke('text').then((text) => {
      expect(text, 'plaintext key absent from proxy detail page').not.to.include('sk-tc1-proxy-plaintext-key');
    });
  });

  // -------------------------------------------------------------------------
  // TC-2: Re-save proxy already using a placeholder → POST /secrets NOT called
  // -------------------------------------------------------------------------
  it('TC-2: does not create a duplicate secret when the API key is already a placeholder', () => {
    const existingHandle = `${toSlug(proxyName)}-provider-api-key`;
    cy.request({
      method: 'POST',
      url: '/proxy/api/v0.9/secrets',
      headers: { Authorization: `Bearer ${authToken}` },
      form: true,
      body: {
        id: existingHandle,
        displayName: `${proxyName} Provider API Key`,
        value: 'sk-tc2-pre-existing-value',
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
    cy.intercept('POST', /\/llm-proxies(\?|$)/).as('createProxy');

    cy.get('input[placeholder="WSO2 OpenAI Provider Proxy"]', { timeout: 30000 })
      .should('be.visible')
      .type(proxyName);
    cy.get('textarea[placeholder="Primary OpenAI provider"]').type('Cypress proxy secret dedup test');
    cy.get('input[placeholder="Enter API key"]').type(
      `{{ secret "${existingHandle}" }}`,
      { parseSpecialCharSequences: false }
    );
    cy.contains('button', 'Create Proxy', { timeout: 30000 }).should('not.be.disabled').click();

    cy.wait('@createProxy').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      createdProxyId = interception.response.body?.id ?? '';
      expect(secretCallCount).to.equal(0);
    });
  });

  // -------------------------------------------------------------------------
  // TC-3: POST /secrets 500 → proxy creation is aborted (no proxy created)
  // -------------------------------------------------------------------------
  it('TC-3: aborts proxy creation when POST /secrets returns 500', () => {
    cy.intercept('POST', '**/secrets', {
      statusCode: 500,
      body: { error: 'simulated vault failure' },
    }).as('failSecret');

    let proxyCallCount = 0;
    cy.intercept('POST', /\/llm-proxies(\?|$)/, (req) => {
      proxyCallCount += 1;
      req.continue();
    });

    cy.get('input[placeholder="WSO2 OpenAI Provider Proxy"]', { timeout: 30000 })
      .should('be.visible')
      .type(proxyName);
    cy.get('textarea[placeholder="Primary OpenAI provider"]').type('Cypress proxy secret abort test');
    cy.get('input[placeholder="Enter API key"]').type('sk-tc3-will-fail');
    cy.contains('button', 'Create Proxy', { timeout: 30000 }).should('not.be.disabled').click();

    cy.wait('@failSecret');

    cy.get('[data-testid="aiworkspace-snackbar-notification"]', { timeout: 15000 })
      .should('be.visible');

    cy.wrap(null).then(() => {
      expect(proxyCallCount).to.equal(0);
    });
  });
});

// ---------------------------------------------------------------------------
// UPDATE flow (Provider tab: typing the key stages it locally, persist via page Save)
// ---------------------------------------------------------------------------

describe('AI Workspace — LLM proxy secret management (update flow)', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const projectName = `E2E Proxy Update Project ${suffix}`;
  const providerName = `E2E Proxy Update Provider ${suffix}`;
  const proxyName = `E2E Proxy Update Proxy ${suffix}`;
  const INITIAL_KEY = `sk-proxy-update-initial-${suffix}`;

  let authToken = '';
  let organizationId = '';
  let createdProviderId = '';
  let proxyId = '';
  let initialSecretHandle = '';

  // Create a fresh project + provider + proxy for each test, land on the
  // proxy's Provider tab.
  beforeEach(() => {
    loginAndFetchAuthContext((v) => { authToken = v; }, (v) => { organizationId = v; });

    createProjectViaUI(projectName);
    createProviderViaUI(providerName).then((id) => {
      createdProviderId = id;
    });
    navigateToCreateProxy(projectName);

    cy.intercept('POST', '**/secrets').as('setupSecret');
    cy.intercept('POST', /\/llm-proxies(\?|$)/).as('setupProxy');
    cy.get('input[placeholder="WSO2 OpenAI Provider Proxy"]', { timeout: 30000 })
      .should('be.visible')
      .type(proxyName);
    cy.get('textarea[placeholder="Primary OpenAI provider"]').type('Cypress proxy update test');
    cy.get('input[placeholder="Enter API key"]').type(INITIAL_KEY);
    cy.contains('button', 'Create Proxy', { timeout: 30000 }).should('not.be.disabled').click();

    cy.wait('@setupSecret', { timeout: 20000 }).then((si) => {
      initialSecretHandle = si.response.body?.id ?? '';
    });
    cy.wait('@setupProxy', { timeout: 20000 }).then((pi) => {
      proxyId = pi.response.body?.id ?? '';
    });

    cy.location('pathname', { timeout: 30000 }).should('match', /\/proxies\/[^/]+$/);
    cy.contains('[role="tab"]', 'Provider', { timeout: 15000 }).click();
    cy.contains('label', 'API Key', { timeout: 15000 }).should('be.visible');
  });

  afterEach(() => {
    if (authToken && organizationId && proxyId) {
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/llm-proxies/${encodeURIComponent(proxyId)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
      proxyId = '';
    }
    if (authToken && organizationId && createdProviderId) {
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(createdProviderId)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
    }
    if (authToken) {
      cy.request({
        url: '/proxy/api/v0.9/projects',
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      }).then((r) => {
        const proj = (r.body?.list ?? []).find((p) => p.displayName === projectName);
        if (proj?.id) {
          cy.request({
            method: 'DELETE',
            url: `/proxy/api/v0.9/projects/${encodeURIComponent(proj.id)}`,
            headers: { Authorization: `Bearer ${authToken}` },
            failOnStatusCode: false,
          });
        }
      });
    }
  });

  // -------------------------------------------------------------------------
  // TC-4: Edit API key with new plaintext → secret created, PUT carries
  // placeholder, old secret cleaned up after the update succeeds.
  //
  // Cleanup happens server-side (platform-api's LLMProxyService.Update calls
  // SecretService.Delete directly once the new reference is persisted) — not
  // as a separate outgoing DELETE the browser makes, so there is no network
  // call to intercept here. Verify it via a follow-up GET on the old handle.
  // -------------------------------------------------------------------------
  it('TC-4: editing the API key rotates the secret, stores the placeholder, and cleans up the old secret', () => {
    const UPDATED_KEY = `sk-proxy-update-new-${suffix}`;

    cy.intercept('POST', '**/secrets').as('createSecret');
    cy.intercept('PUT', /\/llm-proxies\/[^/?]+(\?|$)/).as('updateProxy');

    // Typing the new key stages it into local proxy state.
    cy.get('input[placeholder="Enter API key"]').type(UPDATED_KEY);

    // Persist — page-level Save actually fires the update + secret rotation.
    cy.contains('button', /^Save$/).should('not.be.disabled').click();

    cy.wait('@createSecret', { timeout: 20000 }).then((si) => {
      expect(si.response.statusCode, 'POST /secrets status').to.be.oneOf([200, 201]);
      expect(JSON.stringify(si.response.body), 'plaintext not in secret response').not.to.include(UPDATED_KEY);
    });

    cy.wait('@updateProxy', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /llm-proxies status').to.be.oneOf([200, 201]);
      const authValue = pi.request.body?.provider?.auth?.value ?? '';
      expect(authValue, 'PUT body has placeholder').to.include('{{ secret "');
      expect(authValue, 'PUT body does NOT have plaintext key').not.to.include(UPDATED_KEY);
    });

    cy.get('body').invoke('text').then((text) => {
      expect(text, 'plaintext absent from page').not.to.include(UPDATED_KEY);
    });

    // The old secret backing INITIAL_KEY must be soft-deleted server-side —
    // GetByHandle doesn't filter by status, so it still 200s but flips to
    // DEPRECATED. This closes the gap left untested on the LLM Provider side.
    cy.wrap(null).then(() => {
      expect(initialSecretHandle, 'captured the initial secret handle in beforeEach').to.not.equal('');
      cy.request({
        url: `/proxy/api/v0.9/secrets/${encodeURIComponent(initialSecretHandle)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      }).then((r) => {
        expect(r.status, 'old secret still resolvable').to.eq(200);
        expect(r.body?.status, 'old secret marked deprecated after cleanup').to.eq('DEPRECATED');
      });
    });
  });

  // -------------------------------------------------------------------------
  // TC-5: Edit API key by typing an explicit placeholder → POST /secrets NOT called
  // -------------------------------------------------------------------------
  it('TC-5: typing a placeholder value skips secret creation and sends the placeholder directly', () => {
    const explicitHandle = `${toSlug(proxyName)}-provider-api-key`;

    cy.request({
      method: 'POST',
      url: '/proxy/api/v0.9/secrets',
      headers: { Authorization: `Bearer ${authToken}` },
      form: true,
      body: {
        id: explicitHandle,
        displayName: `${proxyName} Provider API Key`,
        value: 'sk-tc5-explicit-handle-value',
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
    cy.intercept('PUT', /\/llm-proxies\/[^/?]+(\?|$)/).as('updateProxy');

    cy.get('input[placeholder="Enter API key"]').type(
      `{{ secret "${explicitHandle}" }}`,
      { parseSpecialCharSequences: false }
    );
    cy.contains('button', /^Save$/).should('not.be.disabled').click();

    cy.wait('@updateProxy', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /llm-proxies status').to.be.oneOf([200, 201]);
      const authValue = pi.request.body?.provider?.auth?.value ?? '';
      expect(authValue, 'PUT body carries the typed placeholder').to.include('{{ secret "');
      cy.wrap(null).then(() => {
        expect(secretCallCount, 'POST /secrets not called').to.equal(0);
      });
    });
  });

  // -------------------------------------------------------------------------
  // TC-6: POST /secrets 500 during edit → PUT /llm-proxies NOT called, error shown
  // -------------------------------------------------------------------------
  it('TC-6: aborts the API key update when POST /secrets returns 500', () => {
    cy.intercept('POST', '**/secrets', {
      statusCode: 500,
      body: { error: 'simulated vault failure' },
    }).as('failSecret');

    let proxyCallCount = 0;
    cy.intercept('PUT', /\/llm-proxies\/[^/?]+(\?|$)/, (req) => {
      proxyCallCount += 1;
      req.continue();
    });

    cy.get('input[placeholder="Enter API key"]').type('sk-tc6-will-fail');
    cy.contains('button', /^Save$/).should('not.be.disabled').click();

    cy.wait('@failSecret');

    cy.get('[data-testid="aiworkspace-snackbar-notification"]', { timeout: 15000 })
      .should('be.visible');

    cy.wrap(null).then(() => {
      expect(proxyCallCount, 'PUT /llm-proxies not called').to.equal(0);
    });
  });
});
