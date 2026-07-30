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

// Release test plan coverage: "Deploy LLM Provider, View Deployments, Redeploy".
// NOT covered here: "Restore deployment to an older version" — that needs the
// expanded revision history and a restore action, and is still a manual step.
//
// This needs a gateway registered against the control plane, which
// scripts/start-e2e-gateway.sh sets up. Without one the whole spec skips, so a
// plain `docker compose up` stack still runs the rest of the suite green.

describe('AI Workspace - LLM provider deployment', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const providerName = `E2E Deploy Provider ${suffix}`;

  let createdProviderId = '';
  let authToken = '';
  let organizationId = '';
  let gateway = null;

  before(() => {
    cy.sweepE2EProviders();
  });

  beforeEach(function () {
    cy.login();
    cy.apiLogin().then((session) => {
      authToken = session.authToken;
      organizationId = session.organizationId;
    });
    // `function` (not arrow) so `this.skip()` binds to the Mocha context.
    cy.then(function () {
      return cy.findConnectedGateway(authToken).then((found) => {
        gateway = found;
        if (!gateway) {
          cy.log(
            'No connected AI gateway — skipping. Run scripts/start-e2e-gateway.sh to cover this.'
          );
          this.skip();
        }
      });
    });
  });

  afterEach(() => {
    cy.sweepE2EProviders(authToken, organizationId);
  });

  it('deploys a provider to the gateway, shows it in deployments, and redeploys after a change', () => {
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');
    cy.intercept('PUT', '**/llm-providers/**').as('updateProvider');
    cy.intercept('POST', '**/deployments**').as('deploy');

    // --- Create a provider pointed at the stack's sample backend -----------
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
      'sk-e2e-deploy-provider-key'
    );
    cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();
    cy.wait('@createProvider').then(({ response }) => {
      expect(response?.statusCode).to.be.oneOf([200, 201]);
      createdProviderId = response?.body?.id ?? '';
      expect(createdProviderId).to.not.equal('');
    });
    cy.contains(providerName, { timeout: 30000 }).should('be.visible');

    // --- Deploy -------------------------------------------------------------
    // Lives in the header card, which is scrolled off after working in the tabs.
    cy.contains('button', 'Deploy to Gateway', { timeout: 30000 })
      .scrollIntoView()
      .should('not.be.disabled')
      .click();

    // The deploy page lists a card per gateway; act on the one this spec
    // registered rather than whichever card happens to be first.
    gatewayCard().within(() => {
      cy.contains('button', 'Deploy').should('not.be.disabled').click();
    });

    cy.wait('@deploy').then(({ response }) => {
      expect(
        response?.statusCode,
        `deploy failed: ${JSON.stringify(response?.body)}`
      ).to.be.oneOf([200, 201, 202]);
    });

    // Deployment is asynchronous — the controller polls the control plane — so
    // allow well beyond the default command timeout for the row to settle.
    // The row summary reports "Current Deployment: <name>"; the Deployed/Latest
    // chips live in the expanded revision history, not here.
    gatewayCard().within(() => {
      cy.contains('Current Deployment:', { timeout: 120000 }).should('be.visible');
    });

    // --- The control plane must agree the deployment exists ----------------
    cy.then(() =>
      cy
        .request({
          url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(createdProviderId)}/deployments?organizationId=${encodeURIComponent(organizationId)}`,
          headers: { Authorization: `Bearer ${authToken}` },
          failOnStatusCode: false,
        })
        .then((response) => {
          expect(response.status).to.eq(200);
          const deployments = response.body?.list ?? [];
          expect(deployments.length, 'deployment count').to.be.greaterThan(0);
        })
    );

    // --- Change the provider, then redeploy --------------------------------
    // cy.then so createdProviderId is read when the command RUNS — a bare
    // template literal is evaluated while the body is still queueing commands,
    // producing an id-less URL that lands on the providers list instead.
    cy.then(() =>
      cy.visitWorkspace(
        `/organizations/${orgHandle}/service-provider/${createdProviderId}`
      )
    );
    openProviderTab('Connection');
    cy.fieldByLabel('Provider Endpoint')
      .find('input')
      .clear()
      .type('https://api.e2e-redeploy.example.com')
      .blur();
    cy.saveDraftChanges();
    cy.wait('@updateProvider')
      .its('response.statusCode')
      .should('be.oneOf', [200, 201, 204]);

    // Lives in the header card, which is scrolled off after working in the tabs.
    cy.contains('button', 'Deploy to Gateway', { timeout: 30000 })
      .scrollIntoView()
      .should('not.be.disabled')
      .click();
    gatewayCard().within(() => {
      cy.contains('button', 'Deploy').should('not.be.disabled').click();
    });
    cy.wait('@deploy')
      .its('response.statusCode')
      .should('be.oneOf', [200, 201, 202]);
    gatewayCard().within(() => {
      cy.contains('Current Deployment:', { timeout: 120000 }).should('be.visible');
    });

    // The redeploy must leave the provider deployed. This asserts the list is
    // still non-empty — it deliberately does NOT claim a new revision was
    // appended, which would need the revision history this spec doesn't open.
    cy.then(() =>
      cy
        .request({
          url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(createdProviderId)}/deployments?organizationId=${encodeURIComponent(organizationId)}`,
          headers: { Authorization: `Bearer ${authToken}` },
          failOnStatusCode: false,
        })
        .then((response) => {
          expect(response.status).to.eq(200);
          expect(
            (response.body?.list ?? []).length,
            'deployment revisions after redeploy'
          ).to.be.greaterThan(0);
        })
    );
  });
});

// The tab strip is `variant="scrollable"`, so a later tab can sit outside the
// visible scroller — `be.visible` fails on it even though it is clickable.
function openProviderTab(tabLabel) {
  cy.contains('button[role="tab"]', tabLabel, { timeout: 30000 })
    .should('exist')
    .scrollIntoView()
    .click({ force: true });
}

// Locate the deploy row for the gateway this run registered. The deploy page
// renders one MUI Accordion per gateway (not a Card), and other gateways —
// including inactive ones with a disabled Deploy button — sit alongside it, so
// every action must be scoped to this row.
function gatewayCard() {
  return cy
    .contains(Cypress.env('E2E_GATEWAY_NAME'), { timeout: 30000 })
    .should('be.visible')
    .closest('.MuiAccordion-root');
}
