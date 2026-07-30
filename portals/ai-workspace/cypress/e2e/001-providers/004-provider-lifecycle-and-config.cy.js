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

// Release test plan coverage for LLM providers. 001-provider-and-proxy.cy.js
// covers only create+delete of a provider that has an API key; the plan also
// calls for creating one *without* a key, viewing/updating it, and the negative
// path where deletion must be refused while an App LLM Proxy still uses it.

describe('AI Workspace - LLM provider view, update and delete guards', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const projectName = `E2E Provider Cfg Project ${suffix}`;
  const providerName = `E2E Config Provider ${suffix}`;
  const providerDescription = 'Cypress provider configuration coverage';
  const updatedDescription = 'Cypress provider configuration coverage (updated)';
  const proxyName = `E2E Config Proxy ${suffix}`;
  const updatedEndpoint = 'https://api.e2e-updated-endpoint.example.com';

  let createdProviderId = '';
  let proxyUrl = '';
  let authToken = '';
  let organizationId = '';

  before(() => {
    cy.sweepE2EProviders();
  });

  beforeEach(() => {
    cy.login();
    cy.apiLogin().then((session) => {
      authToken = session.authToken;
      organizationId = session.organizationId;
    });
  });

  afterEach(() => {
    cy.deleteProjectByNameApi(authToken, projectName);
    cy.sweepE2EProviders(authToken, organizationId);
  });

  it('creates a provider without an API key, edits its configuration tabs, and refuses deletion while a proxy uses it', () => {
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');
    cy.intercept('PUT', '**/llm-providers/**').as('updateProvider');
    cy.intercept('POST', /\/llm-proxies(\?|$)/).as('createProxy');
    cy.intercept('DELETE', '**/llm-proxies/**').as('deleteProxy');
    cy.intercept('DELETE', '**/llm-providers/**').as('deleteProvider');

    cy.createProjectUI(projectName, 'Cypress project for provider configuration coverage.');

    // --- Create the provider with the API key field left empty --------------
    // The plan calls this out explicitly: the key is optional at create time.
    cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[data-cyid^="provider-template-openai"]', { timeout: 30000 })
      .should('be.visible')
      .click();

    // Some templates interpose a version picker; others go straight to the form.
    cy.get('body', { timeout: 30000 }).should(($body) => {
      expect(
        $body.find('[data-cyid="provider-name-input"] input:visible').length > 0 ||
          $body.find('[data-cyid="template-version-continue-button"]').length > 0
      ).to.eq(true);
    });
    cy.get('body').then(($body) => {
      if ($body.find('[data-cyid="template-version-continue-button"]').length) {
        cy.get('[data-cyid^="template-version-option-"]', { timeout: 30000 })
          .first()
          .click();
        cy.get('[data-cyid="template-version-continue-button"]')
          .should('not.be.disabled')
          .click();
      }
    });

    cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type(providerName);
    cy.get('[data-cyid="provider-description-input"] textarea:visible')
      .clear()
      .type(providerDescription);
    // Deliberately no provider-api-key-input entry — submit must still enable.
    cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();

    cy.wait('@createProvider').then(({ response }) => {
      expect(
        response?.statusCode,
        `POST /llm-providers failed: ${JSON.stringify(response?.body)}`
      ).to.be.oneOf([200, 201]);
      createdProviderId = response?.body?.id ?? '';
      expect(createdProviderId).to.not.equal('');
      expect(createdProviderId).to.not.equal('new');
    });

    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      new RegExp(`^/organizations/${orgHandle}/service-provider/(?!new$)[^/]+$`)
    );
    cy.contains(providerName, { timeout: 30000 }).should('be.visible');

    // --- View: every configuration tab must render --------------------------
    [
      'Overview',
      'Connection',
      'Access Control',
      'Security',
      'Rate Limiting',
      'Guardrails & Policies',
      'Models',
    ].forEach(openProviderTab);

    // --- Update: connection endpoint ---------------------------------------
    openProviderTab('Connection');
    cy.fieldByLabel('Provider Endpoint')
      .find('input')
      .should('be.visible')
      .clear()
      .type(updatedEndpoint)
      .blur();
    cy.saveDraftChanges();
    cy.wait('@updateProvider')
      .its('response.statusCode')
      .should('be.oneOf', [200, 201, 204]);

    // --- Update: security tab ----------------------------------------------
    openProviderTab('Security');
    cy.fieldByLabel('API Key Value Prefix')
      .find('input')
      .clear()
      .type('Bearer')
      .blur();
    // Regression guard for the released defect where a `query` key location was
    // selectable but unsupported by the gateway policy (issue #2958): the only
    // location the UI may offer today is `header`.
    cy.fieldByLabel('Key Location').find('[role="combobox"]').click();
    cy.get('[role="listbox"]').within(() => {
      cy.contains('[role="option"]', 'header').should('exist');
      cy.contains('[role="option"]', 'query').should('not.exist');
    });
    cy.get('body').type('{esc}');
    cy.saveDraftChanges();

    // --- Update: access control switches to deny-all ------------------------
    openProviderTab('Access Control');
    cy.contains('button', 'Deny all', { timeout: 30000 }).click();
    // Switching mode discards the current allow/deny selection, so the tab asks
    // for confirmation before the change reaches the draft.
    cy.contains('Confirm Resource Mode Change', { timeout: 30000 }).should(
      'be.visible'
    );
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Apply').click();
    });
    cy.saveDraftChanges();

    // Re-open the provider so the assertions below read persisted state rather
    // than the in-memory draft the edits above left behind.
    cy.reload();
    openProviderTab('Connection');
    cy.fieldByLabel('Provider Endpoint')
      .find('input')
      .should('have.value', updatedEndpoint);
    openProviderTab('Access Control');
    cy.contains('button', 'Deny all').should('have.attr', 'aria-pressed', 'true');

    // --- Create a proxy against this provider -------------------------------
    // Lives in the header card, which is scrolled off after working in the tabs.
    cy.contains('button', 'Create App LLM Proxy', { timeout: 30000 })
      .scrollIntoView()
      .should('not.be.disabled')
      .click();
    cy.get('[data-cyid="proxy-project-select"]', { timeout: 30000 }).click();
    cy.contains('[role="option"]', projectName, { timeout: 30000 }).click();
    cy.get('[data-cyid="proxy-project-continue-button"]')
      .should('not.be.disabled')
      .click();

    cy.get('[data-cyid="proxy-name-input"] input:visible', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type(proxyName);
    // A proxy needs a provider API key — entered manually or generated — before
    // the create button enables.
    cy.get('[data-cyid="proxy-api-key-input"] input:visible').type(
      'sk-e2e-config-proxy-key'
    );
    cy.get('[data-cyid="create-proxy-button"]').should('not.be.disabled').click();
    cy.wait('@createProxy').its('response.statusCode').should('be.oneOf', [200, 201]);
    cy.contains(proxyName, { timeout: 30000 }).should('be.visible');
    // Remember where the proxy lives. Proxies are project-scoped, so the
    // org-level "App LLM Proxies" nav item only offers a project picker — going
    // back via the sidebar would not reach this proxy.
    cy.url().then((url) => {
      proxyUrl = url;
    });

    // --- Deleting the provider must be refused while the proxy exists -------
    // Wrapped in cy.then so createdProviderId is read when this command RUNS.
    // A bare template literal is evaluated while the test body is queueing
    // commands — before @createProvider resolves — yielding an id-less URL
    // that lands on the providers LIST (which also has a delete button, so
    // the spec would still pass while testing the wrong page).
    cy.then(() =>
      cy.visitWorkspace(
        `/organizations/${orgHandle}/service-provider/${createdProviderId}`
      )
    );
    cy.contains(providerName, { timeout: 30000 }).should('be.visible');
    cy.get('[data-cyid="delete-provider-button"]', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains(/Cannot delete .* because 1 App LLM Proxy is using this provider/, {
      timeout: 30000,
    }).should('be.visible');
    // The blocking check must run *before* the confirmation dialog opens.
    cy.get('[data-cyid="delete-provider-confirm-input"]').should('not.exist');

    // --- Remove the proxy, then the provider deletes ------------------------
    cy.then(() => cy.visit(proxyUrl));
    cy.contains(proxyName, { timeout: 30000 }).should('be.visible');
    cy.get('button[aria-label="Delete proxy"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });
    cy.wait('@deleteProxy').its('response.statusCode').should('be.oneOf', [200, 204]);

    cy.then(() =>
      cy.visitWorkspace(
        `/organizations/${orgHandle}/service-provider/${createdProviderId}`
      )
    );
    cy.contains(providerName, { timeout: 30000 }).should('be.visible');
    cy.get('[data-cyid="delete-provider-button"]', { timeout: 30000 })
      .should('be.visible')
      .click();

    // Confirmation requires retyping the exact provider name.
    cy.get('[data-cyid="delete-provider-confirm-button"]', { timeout: 30000 }).should(
      'be.disabled'
    );
    cy.get('[data-cyid="delete-provider-confirm-input"]')
      .find('input')
      .type(providerName);
    cy.get('[data-cyid="delete-provider-confirm-button"]')
      .should('not.be.disabled')
      .click();
    cy.wait('@deleteProvider')
      .its('response.statusCode')
      .should('be.oneOf', [200, 204]);

    cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.contains(providerName, { timeout: 30000 }).should('not.exist');
  });

  it('persists a name and description change made from the provider edit form', () => {
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');
    cy.intercept('PUT', '**/llm-providers/**').as('updateProvider');

    createOpenAIProvider({ providerName, providerDescription }).then((id) => {
      createdProviderId = id;
      cy.visitWorkspace(
        `/organizations/${orgHandle}/service-provider/${id}/edit`
      );
    });

    // The edit form is a dedicated route, not a tab on the overview page.
    cy.contains('label', 'Name', { timeout: 30000 }).should('be.visible');
    cy.get('input[placeholder="Enter service provider name"]', { timeout: 30000 })
      .should('have.value', providerName)
      .clear()
      .type(`${providerName} Updated`);
    cy.get('textarea[placeholder="Enter description"]')
      .first()
      .clear()
      .type(updatedDescription);
    cy.contains('button', 'Update').should('not.be.disabled').click();
    cy.wait('@updateProvider')
      .its('response.statusCode')
      .should('be.oneOf', [200, 201, 204]);

    // Re-enter the edit form so the assertion reads persisted server state.
    cy.visitWorkspace(
      `/organizations/${orgHandle}/service-provider/${createdProviderId}/edit`
    );
    cy.get('input[placeholder="Enter service provider name"]', {
      timeout: 30000,
    }).should('have.value', `${providerName} Updated`);
    cy.get('textarea[placeholder="Enter description"]')
      .first()
      .should('have.value', updatedDescription);
  });
});

// The tab strip is `variant="scrollable"`, so a tab further along (Models,
// Guardrails & Policies) sits outside the visible scroller until scrolled to —
// asserting `be.visible` on it fails even though it is perfectly clickable.
function openProviderTab(tabLabel) {
  cy.contains('button[role="tab"]', tabLabel, { timeout: 30000 })
    .should('exist')
    .scrollIntoView()
    .click({ force: true });
}

// Minimal create-provider flow, used by specs that need an existing provider
// rather than exercising the create path itself. Yields the created id.
function createOpenAIProvider({ providerName, providerDescription }) {
  cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 })
    .should('be.visible')
    .click();
  cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 })
    .should('be.visible')
    .click();
  cy.get('[data-cyid^="provider-template-openai"]', { timeout: 30000 })
    .should('be.visible')
    .click();

  cy.get('body', { timeout: 30000 }).should(($body) => {
    expect(
      $body.find('[data-cyid="provider-name-input"] input:visible').length > 0 ||
        $body.find('[data-cyid="template-version-continue-button"]').length > 0
    ).to.eq(true);
  });
  cy.get('body').then(($body) => {
    if ($body.find('[data-cyid="template-version-continue-button"]').length) {
      cy.get('[data-cyid^="template-version-option-"]', { timeout: 30000 })
        .first()
        .click();
      cy.get('[data-cyid="template-version-continue-button"]')
        .should('not.be.disabled')
        .click();
    }
  });

  cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
    .should('be.visible')
    .clear()
    .type(providerName);
  cy.get('[data-cyid="provider-description-input"] textarea:visible')
    .clear()
    .type(providerDescription);
  cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();

  return cy.wait('@createProvider').then(({ response }) => {
    expect(response?.statusCode).to.be.oneOf([200, 201]);
    const id = response?.body?.id ?? '';
    expect(id).to.not.equal('');
    return id;
  });
}
