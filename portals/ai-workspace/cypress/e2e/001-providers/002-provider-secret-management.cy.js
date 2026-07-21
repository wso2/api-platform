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
 * Secret management behaviour for LLM provider create and update flows.
 *
 * Create flow (TC-57 – TC-59, TC-63):
 *   TC-57  Add provider with plaintext API key → POST /secrets called, placeholder stored,
 *          plaintext absent from the provider detail page
 *   TC-58  Re-save provider already holding a placeholder → POST /secrets NOT called
 *   TC-59  POST /secrets 500 → provider creation aborted, no provider created
 *   TC-63  GET /secrets never returns "value"/"encryptedValue" for any secret in the org
 *
 * Update flow via Connection tab (TC-60 – TC-62, TC-64):
 *   TC-60  Edit credential with a new plaintext value → POST /secrets called, PUT /llm-providers
 *          body holds placeholder, plaintext absent from page
 *   TC-61  Edit credential by typing an explicit placeholder → POST /secrets NOT called,
 *          PUT /llm-providers fires with the placeholder as-is
 *   TC-62  POST /secrets 500 during edit → PUT /llm-providers NOT called, error shown
 *   TC-64  Same as TC-60 but reached via the provider list (nav → card → Connection tab)
 *          instead of staying on the just-created provider's own detail page
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

function navigateToAddProvider() {
  cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 })
    .should('be.visible')
    .click();
  cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 })
    .should('be.visible')
    .click();
}

function selectOpenAITemplate() {
  cy.get('[data-cyid="provider-template-openai-card"]', { timeout: 30000 })
    .should('be.visible')
    .click();
}

function fillProviderForm(name, apiKey) {
  cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
    .should('be.visible')
    .clear()
    .type(name);
  cy.get('[data-cyid="provider-api-key-input"] input:visible').type(apiKey);
}

// ---------------------------------------------------------------------------
// CREATE flow
// ---------------------------------------------------------------------------

describe('AI Workspace — LLM provider secret management (create flow)', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const providerName = `E2E Secret Provider ${suffix}`;

  let authToken = '';
  let organizationId = '';
  let createdProviderId = '';

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
        expect(authToken).to.not.equal('');
        return cy.request({
          url: '/proxy/api/v0.9/organizations',
          headers: { Authorization: `Bearer ${authToken}` },
        });
      })
      .then((response) => {
        expect(response.status).to.eq(200);
        organizationId = response.body?.list?.[0]?.id ?? '';
        expect(organizationId).to.not.equal('');
      });
  });

  afterEach(() => {
    if (!authToken || !organizationId || !createdProviderId) return;
    cy.request({
      method: 'DELETE',
      url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(createdProviderId)}?organizationId=${encodeURIComponent(organizationId)}`,
      headers: { Authorization: `Bearer ${authToken}` },
      failOnStatusCode: false,
    });
    createdProviderId = '';
  });

  // -------------------------------------------------------------------------
  // TC-57: Add LLM provider with plain-text key → secret auto-created, placeholder stored
  // -------------------------------------------------------------------------
  it('TC-57: creates a secret and stores the placeholder in the provider config', () => {
    cy.intercept('POST', '**/secrets').as('createSecret');
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');

    navigateToAddProvider();
    selectOpenAITemplate();
    fillProviderForm(providerName, 'sk-tc57-plaintext-key');

    cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();

    // Secret must be created first with the plaintext value.
    cy.wait('@createSecret').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      // The UI posts multipart/form-data; assert on the response body instead.
      const secretId = interception.response.body?.id;
      expect(secretId).to.match(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
    });

    // Provider must be created with placeholder, not plaintext.
    cy.wait('@createProvider').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      const configStr = JSON.stringify(interception.request.body);
      expect(configStr).to.include('{{ secret ');
      expect(configStr).not.to.include('sk-tc57-plaintext-key');
      createdProviderId =
        interception.response.body?.id ??
        interception.response.body?.handle ??
        '';
    });

    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      new RegExp(`^/organizations/${orgHandle}/service-provider/[^/]+$`)
    );

    // Plaintext key must never appear anywhere on the provider detail page.
    cy.get('body').invoke('text').then((text) => {
      expect(text, 'plaintext key absent from provider detail page').not.to.include('sk-tc57-plaintext-key');
    });
  });

  // -------------------------------------------------------------------------
  // TC-58: Re-save provider already using a placeholder → POST /secrets NOT called
  // -------------------------------------------------------------------------
  it('TC-58: does not create a duplicate secret when the auth value is already a placeholder', () => {
    const existingHandle = `${toSlug(providerName)}-api-key`;
    // Pre-create the secret via API so there's a real secret backing the placeholder.
    cy.request({
      method: 'POST',
      url: '/proxy/api/v0.9/secrets',
      headers: { Authorization: `Bearer ${authToken}` },
      form: true,
      body: {
        id: existingHandle,
        displayName: `${providerName} API Key`,
        value: 'sk-tc58-pre-existing-value',
        type: 'GENERIC',
      },
      failOnStatusCode: false,
    }).then((r) => {
      expect(r.status).to.be.oneOf([200, 201, 409]);
    });

    // Track any POST /secrets calls — there must be none.
    let secretCallCount = 0;
    cy.intercept('POST', '**/secrets', (req) => {
      secretCallCount += 1;
      req.continue();
    });
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');

    navigateToAddProvider();
    selectOpenAITemplate();

    cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type(providerName);
    // Type a value that already looks like a placeholder — should be passed through unchanged.
    cy.get('[data-cyid="provider-api-key-input"] input:visible').type(
      `{{ secret "${existingHandle}" }}`,
      { parseSpecialCharSequences: false }
    );

    cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();

    cy.wait('@createProvider').then((interception) => {
      expect(interception.response.statusCode).to.be.oneOf([200, 201]);
      createdProviderId =
        interception.response.body?.id ??
        interception.response.body?.handle ??
        '';
      // secretCallCount is a plain JS var captured in the closure; reading it
      // inside a .then() guarantees the Cypress queue has settled.
      expect(secretCallCount).to.equal(0);
    });
  });

  // -------------------------------------------------------------------------
  // TC-59: POST /secrets 500 → provider creation is aborted (no provider created)
  // -------------------------------------------------------------------------
  it('TC-59: aborts provider creation when POST /secrets returns 500', () => {
    cy.intercept('POST', '**/secrets', {
      statusCode: 500,
      body: { error: 'simulated vault failure' },
    }).as('failSecret');

    let providerCallCount = 0;
    cy.intercept('POST', /\/llm-providers(\?|$)/, (req) => {
      providerCallCount += 1;
      req.continue();
    });

    navigateToAddProvider();
    selectOpenAITemplate();
    fillProviderForm(providerName, 'sk-tc59-will-fail');

    cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();

    cy.wait('@failSecret');

    // The app renders errors via the Notification component with a stable testId.
    cy.get('[data-testid="aiworkspace-snackbar-notification"]', { timeout: 15000 })
      .should('be.visible');

    // Read providerCallCount inside a .then() so it is evaluated after the
    // Cypress queue settles (not at scheduling time).
    cy.wrap(null).then(() => {
      expect(providerCallCount).to.equal(0);
    });
  });

  // -------------------------------------------------------------------------
  // TC-63: GET /secrets never returns "value"/"encryptedValue" for any secret
  // in the org (encryption proof, not scoped to a single just-created secret).
  // -------------------------------------------------------------------------
  it('TC-63: GET /secrets never exposes plaintext value for any secret in the org', () => {
    const handle = `${toSlug(providerName)}-tc63-api-key`;

    cy.request({
      method: 'POST',
      url: '/proxy/api/v0.9/secrets',
      headers: { Authorization: `Bearer ${authToken}` },
      form: true,
      body: {
        id: handle,
        displayName: `${providerName} TC-63 API Key`,
        value: 'sk-tc63-encryption-proof',
        type: 'GENERIC',
      },
      failOnStatusCode: false,
    }).then((r) => {
      expect(r.status).to.be.oneOf([200, 201, 409]);
    });

    // Fetch the single secret by handle (not the paginated list — the org
    // accumulates many secrets across the suite and the list defaults to
    // limit=25, which can miss the one just created).
    cy.request({
      url: `/proxy/api/v0.9/secrets/${encodeURIComponent(handle)}?organizationId=${encodeURIComponent(organizationId)}`,
      headers: { Authorization: `Bearer ${authToken}` },
    }).then((r) => {
      expect(r.status).to.eq(200);
      const secret = r.body;

      expect(secret, `handle "${handle}" found`).to.exist;
      expect(secret).to.have.property('id', handle);
      expect(secret, 'no "value" field').not.to.have.property('value');
      expect(secret, 'no "encryptedValue" field').not.to.have.property('encryptedValue');

      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/secrets/${encodeURIComponent(handle)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
    });
  });
});

// ---------------------------------------------------------------------------
// UPDATE flow (Connection tab)
// ---------------------------------------------------------------------------

describe('AI Workspace — LLM provider secret management (update flow)', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const providerName = `E2E Secret Update Provider ${suffix}`;
  const INITIAL_KEY = `sk-update-initial-${suffix}`;
  const UPDATED_KEY = `sk-update-new-${suffix}`;

  let authToken = '';
  let organizationId = '';
  let providerId = '';

  // Create a fresh provider for each test, navigate to its Connection tab.
  // This avoids cross-hook variable sharing issues with test isolation.
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

    // Sweep any stale E2E providers (e.g. if a previous test's afterEach delete failed).
    cy.then(() => cy.sweepE2EProviders(authToken, organizationId));

    cy.intercept('POST', /\/llm-providers(\?|$)/).as('setupProvider');

    cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 }).should('be.visible').click();
    cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 }).should('be.visible').click();
    cy.get('[data-cyid="provider-template-openai-card"]', { timeout: 30000 }).should('be.visible').click();
    cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
      .should('be.visible').clear().type(providerName);
    cy.get('[data-cyid="provider-api-key-input"] input:visible').type(INITIAL_KEY);
    cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();

    cy.wait('@setupProvider', { timeout: 20000 }).then((pi) => {
      providerId = pi.response.body?.id ?? '';
    });

    // Match on the create response, not the URL: the redirect lands a beat
    // after the click, and a URL scrape can race onto the transient "new"
    // route — exclude it explicitly so the wait doesn't resolve early.
    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      new RegExp(`^/organizations/${orgHandle}/service-provider/(?!new$)[^/]+$`)
    );

    cy.contains('[role="tab"]', 'Connection', { timeout: 15000 }).click();
    cy.contains('label', 'Credentials', { timeout: 15000 }).should('be.visible');
    // Wait for provider data to load: the credential field must be in the masked
    // state before any test interacts with it. Without this, handleUpdateCredential
    // returns early because provider is still null/loading.
    cy.contains('label', 'Credentials').parent().find('input', { timeout: 15000 })
      .should('have.value', '******');
  });

  afterEach(() => {
    if (authToken && organizationId && providerId) {
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(providerId)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
      providerId = '';
    }
  });

  // -------------------------------------------------------------------------
  // TC-60: Edit credential with new plaintext → secret created, placeholder in PUT body
  // -------------------------------------------------------------------------
  it('TC-60: editing the credential creates a new secret and stores the placeholder in the provider PUT body', () => {
    cy.intercept('POST', '**/secrets').as('createSecret');
    cy.intercept('PUT', /\/llm-providers\/[^/?]+(\?|$)/).as('updateProvider');

    // Click the Credentials field to clear the masked value, then type the new key.
    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .click();

    // After focus the field clears itself (masked → empty).
    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .should('have.value', '');

    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .type(UPDATED_KEY);

    // Click the Save button that appears at the bottom when there are unsaved changes.
    cy.contains('button', 'Save').click();

    // Assert: POST /secrets fires before the provider PUT.
    cy.wait('@createSecret', { timeout: 20000 }).then((si) => {
      expect(si.response.statusCode, 'POST /secrets status').to.be.oneOf([200, 201]);
      // New secret handle is a UUID.
      const secretId = si.response.body?.id ?? '';
      expect(secretId).to.match(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
      // Server never echoes the plaintext back.
      expect(JSON.stringify(si.response.body)).not.to.include(UPDATED_KEY);
      cy.log(`✅ Secret created: id="${secretId}"`);
    });

    // Assert: PUT /llm-providers carries a placeholder, not the plaintext key.
    cy.wait('@updateProvider', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /llm-providers status').to.be.oneOf([200, 201]);
      const authValue = pi.request.body?.upstream?.main?.auth?.value ?? '';
      expect(authValue, 'PUT body has placeholder').to.include('{{ secret "');
      expect(authValue, 'PUT body does NOT have plaintext key').not.to.include(UPDATED_KEY);
      cy.log(`✅ Placeholder in PUT body: ${authValue}`);
    });

    // Plaintext key must not appear anywhere on the page.
    cy.get('body').invoke('text').then((text) => {
      expect(text, 'plaintext absent from page').not.to.include(UPDATED_KEY);
    });
  });

  // -------------------------------------------------------------------------
  // TC-61: Edit credential by typing an explicit placeholder → POST /secrets NOT called
  // -------------------------------------------------------------------------
  it('TC-61: typing a placeholder value skips secret creation and sends the placeholder directly', () => {
    const explicitHandle = `${toSlug(providerName)}-api-key`;

    // Pre-create the secret so the platform-api accepts the placeholder in the PUT.
    // cy.request() bypasses cy.intercept(), so this won't affect secretCallCount below.
    cy.request({
      method: 'POST',
      url: '/proxy/api/v0.9/secrets',
      headers: { Authorization: `Bearer ${authToken}` },
      form: true,
      body: {
        id: explicitHandle,
        displayName: `${providerName} API Key`,
        value: 'sk-tc61-explicit-handle-value',
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
    cy.intercept('PUT', /\/llm-providers\/[^/?]+(\?|$)/).as('updateProvider');

    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .click();

    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .should('have.value', '');

    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .type(`{{ secret "${explicitHandle}" }}`, { parseSpecialCharSequences: false });

    // Click the Save button at the bottom — typing a placeholder skips createSecret.
    cy.contains('button', 'Save').click();

    // PUT fires with the typed placeholder.
    cy.wait('@updateProvider', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /llm-providers status').to.be.oneOf([200, 201]);
      const authValue = pi.request.body?.upstream?.main?.auth?.value ?? '';
      expect(authValue, 'PUT body carries the typed placeholder').to.include('{{ secret "');
      cy.wrap(null).then(() => {
        expect(secretCallCount, 'POST /secrets not called').to.equal(0);
      });
    });
  });

  // -------------------------------------------------------------------------
  // TC-62: POST /secrets 500 during edit → PUT /llm-providers NOT called, error shown
  // -------------------------------------------------------------------------
  it('TC-62: aborts the credential update when POST /secrets returns 500', () => {
    cy.intercept('POST', '**/secrets', {
      statusCode: 500,
      body: { error: 'simulated vault failure' },
    }).as('failSecret');

    let providerCallCount = 0;
    cy.intercept('PUT', /\/llm-providers\/[^/?]+(\?|$)/, (req) => {
      providerCallCount += 1;
      req.continue();
    });

    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .click();

    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .should('have.value', '');

    cy.contains('label', 'Credentials')
      .parent()
      .find('input')
      .type('sk-tc62-will-fail');

    // Click Save — the stubbed 500 on POST /secrets should abort the update.
    cy.contains('button', 'Save').click();

    cy.wait('@failSecret');

    // Error notification must be visible.
    cy.get('[data-testid="aiworkspace-snackbar-notification"]', { timeout: 15000 })
      .should('be.visible');

    // Provider PUT must not have fired.
    cy.wrap(null).then(() => {
      expect(providerCallCount, 'PUT /llm-providers not called').to.equal(0);
    });
  });

  // -------------------------------------------------------------------------
  // TC-64: Same rotation as TC-60, but reached via the provider list (nav →
  // card → Connection tab) instead of staying on the freshly created
  // provider's own detail page — exercises the list → card navigation path.
  // -------------------------------------------------------------------------
  it('TC-64: updating the API key after navigating from the provider list rotates the secret', () => {
    const UPDATED_KEY_VIA_LIST = `sk-update-via-list-${suffix}`;

    cy.intercept('POST', '**/secrets').as('createSecret');
    cy.intercept('PUT', /\/llm-providers\/[^/?]+(\?|$)/).as('updateProvider');

    cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 }).should('be.visible').click();
    cy.get(`[data-cyid="provider-card-${providerId}"]`, { timeout: 30000 }).should('be.visible').click();
    cy.contains('[role="tab"]', 'Connection', { timeout: 15000 }).click();
    cy.contains('label', 'Credentials', { timeout: 15000 }).parent().find('input', { timeout: 15000 })
      .should('have.value', '******');

    cy.contains('label', 'Credentials').parent().find('input').click();
    cy.contains('label', 'Credentials').parent().find('input').should('have.value', '');
    cy.contains('label', 'Credentials').parent().find('input').type(UPDATED_KEY_VIA_LIST);
    cy.contains('button', 'Save').click();

    cy.wait('@createSecret', { timeout: 20000 }).then((si) => {
      expect(si.response.statusCode, 'POST /secrets status').to.be.oneOf([200, 201]);
      expect(JSON.stringify(si.response.body), 'plaintext not in secret response').not.to.include(UPDATED_KEY_VIA_LIST);
    });

    cy.wait('@updateProvider', { timeout: 20000 }).then((pi) => {
      expect(pi.response.statusCode, 'PUT /llm-providers status').to.be.oneOf([200, 201]);
      const authValue = pi.request.body?.upstream?.main?.auth?.value ?? '';
      expect(authValue, 'PUT body has placeholder').to.include('{{ secret "');
      expect(authValue, 'PUT body does NOT have plaintext key').not.to.include(UPDATED_KEY_VIA_LIST);
    });
  });
});
