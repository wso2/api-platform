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

// Release test plan coverage for App LLM Proxies: view/update (the existing
// 001-provider-and-proxy.cy.js only creates and deletes one), the provider
// settings tab, the definition tab, and the client-facing security settings.

describe('AI Workspace - App LLM proxy view, update and security', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const projectName = `E2E Proxy Cfg Project ${suffix}`;
  const providerName = `E2E Proxy Cfg Provider ${suffix}`;
  const proxyName = `E2E Proxy Cfg Proxy ${suffix}`;
  const updatedProxyName = `${proxyName} Updated`;
  const updatedDescription = 'Cypress proxy configuration coverage (updated)';

  let createdProxyId = '';
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
    // Sweeping providers also clears the proxies linked to them.
    cy.sweepE2EProviders(authToken, organizationId);
  });

  it('views every proxy tab, updates its details and security settings, then deletes it', () => {
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');
    cy.intercept('POST', /\/llm-proxies(\?|$)/).as('createProxy');
    cy.intercept('PUT', '**/llm-proxies/**').as('updateProxy');
    cy.intercept('DELETE', '**/llm-proxies/**').as('deleteProxy');

    cy.createProjectUI(projectName, 'Cypress project for proxy configuration coverage.');

    // --- A provider is a prerequisite for an App LLM Proxy ------------------
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
    cy.get('[data-cyid="provider-api-key-input"] input:visible').type(
      'sk-e2e-proxy-cfg-provider-key'
    );
    cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();
    cy.wait('@createProvider').its('response.statusCode').should('be.oneOf', [200, 201]);

    // --- Create the proxy ---------------------------------------------------
    cy.contains('button', 'Create App LLM Proxy', { timeout: 30000 })
      .should('be.visible')
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
    cy.get('[data-cyid="proxy-description-input"] textarea:visible')
      .clear()
      .type('Cypress proxy configuration coverage');
    cy.get('[data-cyid="proxy-api-key-input"] input:visible').type(
      'sk-e2e-proxy-cfg-proxy-key'
    );
    cy.get('[data-cyid="create-proxy-button"]').should('not.be.disabled').click();
    cy.wait('@createProxy').then(({ response }) => {
      expect(
        response?.statusCode,
        `POST /llm-proxies failed: ${JSON.stringify(response?.body)}`
      ).to.be.oneOf([200, 201]);
      createdProxyId = response?.body?.id ?? '';
      expect(createdProxyId).to.not.equal('');
    });

    cy.location('pathname', { timeout: 30000 }).should('match', /\/proxies\/[^/]+$/);
    cy.contains(proxyName, { timeout: 30000 }).should('be.visible');
    // Proxies are project-scoped, so their real path includes the project
    // segment. Capture it rather than reconstructing an org-level URL, which
    // renders a project picker instead of the proxy.
    cy.url().then((url) => {
      proxyUrl = url;
    });

    // --- View: every tab must render ---------------------------------------
    ['Overview', 'Provider', 'Definition', 'Security', 'Guardrails & Policies'].forEach(
      openProxyTab
    );

    // The Provider tab must show which provider this proxy is bound to.
    openProxyTab('Provider');
    cy.fieldByLabel('Provider').should('contain.text', providerName);

    // --- Update: client-facing security settings ---------------------------
    openProxyTab('Security');
    cy.fieldByLabel('Authentication type').should('contain.text', 'API Key');
    cy.fieldByLabel('Key name').find('input').clear().type('X-E2E-Api-Key').blur();
    cy.fieldByLabel('API Key Value Prefix').find('input').clear().type('Bearer').blur();
    cy.saveDraftChanges();
    cy.wait('@updateProxy')
      .its('response.statusCode')
      .should('be.oneOf', [200, 201, 204]);

    cy.reload();
    openProxyTab('Security');
    cy.fieldByLabel('Key name').find('input').should('have.value', 'X-E2E-Api-Key');

    // --- Update: name and description via the edit route --------------------
    cy.then(() => cy.visit(`${proxyUrl}/edit`));
    cy.get('input[placeholder="Enter proxy name"]', { timeout: 30000 })
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

    cy.then(() => cy.visit(`${proxyUrl}/edit`));
    cy.get('input[placeholder="Enter proxy name"]', { timeout: 30000 }).should(
      'have.value',
      updatedProxyName
    );
    cy.get('textarea[placeholder="Enter description"]')
      .first()
      .should('have.value', updatedDescription);

    // --- Delete -------------------------------------------------------------
    cy.then(() => cy.visit(proxyUrl));
    cy.get('button[aria-label="Delete proxy"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.contains('Delete App LLM Proxy', { timeout: 30000 }).should('be.visible');
    // Assert the renamed proxy here rather than in the page header: the header
    // truncates a long displayName ("...16567407 U…"), the dialog spells it out.
    cy.get('[role="dialog"]').within(() => {
      cy.contains(updatedProxyName).should('be.visible');
      cy.contains('button', 'Delete').click();
    });
    cy.wait('@deleteProxy').its('response.statusCode').should('be.oneOf', [200, 204]);

    cy.location('pathname', { timeout: 30000 }).should('match', /\/proxies\/?$/);
    cy.contains(updatedProxyName, { timeout: 30000 }).should('not.exist');
  });
});

// The tab strip is `variant="scrollable"`, so a later tab can sit outside the
// visible scroller — `be.visible` fails on it even though it is clickable.
function openProxyTab(tabLabel) {
  cy.contains('button[role="tab"]', tabLabel, { timeout: 30000 })
    .should('exist')
    .scrollIntoView()
    .click({ force: true });
}
