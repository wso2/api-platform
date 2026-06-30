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
 * Secret management behaviour for LLM provider create/update flows.
 *
 * Covers:
 *   TC-57  Add provider with plaintext API key → POST /secrets called, placeholder stored
 *   TC-58  Re-save provider already holding a placeholder → POST /secrets NOT called
 *   TC-59  POST /secrets 500 → provider creation aborted, no provider created
 */
describe('AI Workspace — LLM provider secret management', () => {
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
      url: '/api/proxy/api/portal/v0.9/auth/login',
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
          url: '/api/proxy/api/v0.9/organizations',
          headers: { Authorization: `Bearer ${authToken}` },
        });
      })
      .then((response) => {
        expect(response.status).to.eq(200);
        organizationId = response.body?.id ?? '';
        expect(organizationId).to.not.equal('');
      });
  });

  afterEach(() => {
    if (!authToken || !organizationId || !createdProviderId) return;
    cy.request({
      method: 'DELETE',
      url: `/api/proxy/api/v0.9/llm-providers/${encodeURIComponent(createdProviderId)}?organizationId=${encodeURIComponent(organizationId)}`,
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
      const handle = interception.response.body?.handle;
      expect(handle).to.match(/-api-key$/);
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
  });

  // -------------------------------------------------------------------------
  // TC-58: Re-save provider already using a placeholder → POST /secrets NOT called
  // -------------------------------------------------------------------------
  it('TC-58: does not create a duplicate secret when the auth value is already a placeholder', () => {
    const existingHandle = `${toSlug(providerName)}-api-key`;
    // Pre-create the secret via API so there's a real secret backing the placeholder.
    cy.request({
      method: 'POST',
      url: '/api/proxy/api/v0.9/secrets',
      headers: {
        Authorization: `Bearer ${authToken}`,
      },
      form: true,
      body: {
        handle: existingHandle,
        name: `${providerName} API Key`,
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

  // ---------------------------------------------------------------------------
  // Helpers
  // ---------------------------------------------------------------------------

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
});

function toSlug(value) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
