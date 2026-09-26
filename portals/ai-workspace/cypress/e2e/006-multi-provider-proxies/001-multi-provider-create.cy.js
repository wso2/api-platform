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
 * Building a proxy that serves several providers.
 *
 * Each test opens the form on the project-scoped route, which names the
 * project in the path — the form is built from the project the application
 * holds, and arriving that way is what puts it there.
 */

import { appPathPattern } from '../../support/appPath';

describe('AI Workspace - multi-provider proxy creation', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const proxyName = `E2E Multi Proxy ${suffix}`;
  const projectName = `E2E Multi Project ${suffix}`;
  const primaryProviderName = `E2E Multi Provider A ${suffix}`;
  const secondProviderName = `E2E Multi Provider B ${suffix}`;
  let projectId = '';

  /**
   * A provider of its own, and a project to put a proxy in.
   *
   * Built here rather than borrowed from another area: a spec that depends on
   * what an earlier run left behind fails for reasons that have nothing to do
   * with what it is testing, and says nothing useful when it does.
   */
  /**
   * Signs in only when there is no session already.
   *
   * Asked of the server rather than read off the page: the sign-in form and
   * the signed-in shell both render some time after the page loads, so a test
   * that decides from the DOM the moment it arrives sometimes sees neither and
   * skips signing in — then fails much later on a missing element, saying
   * nothing about the session it never had.
   */
  const ensureLoggedIn = () => {
    cy.request({ url: 'api/session', failOnStatusCode: false }).then((response) => {
      if (response.status !== 200) {
        cy.login();
      }
    });
  };

  /**
   * An organization-level provider, built through the pages a user would.
   *
   * The Service Provider page hides its create action while a project is
   * selected, and the selection lives in the application rather than the URL —
   * so this starts from the organization root, which has none.
   */
  const createProvider = (name) => {
    cy.visitWorkspace(`/organizations/${orgHandle}`);
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');
    cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[data-cyid="provider-template-openai-card"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type(name);
    cy.get('[data-cyid="provider-api-key-input"] input:visible').type(
      'sk-provider-backing-key'
    );
    cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();
    cy.wait('@createProvider').its('response.statusCode').should('be.oneOf', [200, 201]);
  };

  before(() => {
    cy.login();
    cy.visitWorkspace(`/organizations/${orgHandle}`);

    cy.intercept('POST', '**/projects').as('createProject');
    cy.contains('Projects', { timeout: 30000 }).should('be.visible').click();
    cy.contains('button, a', /Create Project|Add New Project/, { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
      .should('be.visible')
      .type(projectName);
    cy.get('textarea[placeholder="Short description of the project."]').type(
      'Cypress multi-provider proxy project'
    );
    cy.contains('button', 'Create').should('not.be.disabled').click();
    cy.wait('@createProject').then(({ response }) => {
      expect(response?.statusCode).to.be.oneOf([200, 201]);
      projectId = response?.body?.id ?? '';
      expect(projectId).to.not.equal('');
    });

    // Two of them, because a proxy cannot be given a second provider it does
    // not have: the form offers only providers not already on the proxy, so an
    // organization holding exactly one leaves the add control correctly — and
    // permanently — unavailable.
    createProvider(primaryProviderName);
    createProvider(secondProviderName);
  });

  beforeEach(() => {
    ensureLoggedIn();
    cy.intercept('POST', /\/llm-proxies(\?|$)/).as('createProxy');
  });

  /**
   * Opens the proxy create form in the project this spec made.
   *
   * The project-scoped route carries the project in its own path, and the
   * shell resolves it back into application state on arrival — so the form is
   * built for this project without driving the two screens (provider list and
   * its project dialog) that a change elsewhere would turn into a failure
   * here reading as a defect in proxy creation.
   */
  const openProxyCreateForm = () => {
    cy.visitWorkspace(
      `/organizations/${orgHandle}/projects/${projectId}/proxies/create`
    );
    cy.get('[data-cyid="proxy-name-input"]', { timeout: 30000 }).should('be.visible');
    // The provider list feeds the selector; nothing on the page is meaningful
    // until it has answered.
    cy.get('[data-cyid="proxy-provider-select"]', { timeout: 30000 }).should('exist');
  };

  /**
   * Describes the proxy's own provider: the one it routes to by default.
   *
   * Create stays unavailable until this is complete — a proxy with no provider
   * has nothing to route to, and one whose provider takes a key has nothing to
   * authenticate with. Both are part of the single-provider path every test
   * here starts from, not of what any of them is checking.
   */
  const describePrimaryProvider = () => {
    cy.get('[data-cyid="proxy-provider-select"]').click();
    cy.contains('li', primaryProviderName, { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[data-cyid="proxy-api-key-input"] input', { timeout: 30000 })
      .should('be.visible')
      .type('sk-primary-proxy-key');
  };

  it('asks nothing extra of a single-provider proxy', () => {
    openProxyCreateForm();

    // The page opens with one provider form. No second provider, no rows, no
    // primary toggle — a user building a single-provider proxy makes exactly
    // the decisions they made before any of this existed.
    cy.get('[data-cyid="proxy-provider-select"]').should('exist');
    cy.get('[data-cyid="staged-provider-form"]').should('not.exist');
    cy.get('[data-cyid="provider-row-0"]').should('not.exist');

    // The interface is a decision behind a disclosure, not one on the way.
    cy.get('[data-cyid="inbound-interface-select"]').should('not.exist');
    cy.get('[data-cyid="advanced-configurations-toggle"]').click();
    cy.get('[data-cyid="inbound-interface-select"]').should('exist');
  });

  it('stages a second provider, and commits it only when asked', () => {
    openProxyCreateForm();

    // Adding opens a form to fill in. Nothing joins the list until it is
    // actually described, so an abandoned form leaves the proxy as it was.
    cy.get('[data-cyid="add-provider-button"]').click();
    cy.get('[data-cyid="staged-provider-form"]').should('be.visible');
    // The first provider collapses to a row as soon as a second is involved.
    cy.get('[data-cyid="provider-row-0"]').should('be.visible');

    cy.get('[data-cyid="staged-provider-cancel"]').click();
    cy.get('[data-cyid="staged-provider-form"]').should('not.exist');
    cy.get('[data-cyid="provider-row-0"]').should('not.exist');

    // Committed, it becomes a row of its own.
    cy.get('[data-cyid="add-provider-button"]').click();
    cy.get('[data-cyid="staged-provider-add"]', { timeout: 30000 })
      .should('not.be.disabled')
      .click();
    cy.get('[data-cyid="staged-provider-form"]').should('not.exist');
    cy.get('[data-cyid="provider-row-1"]').should('be.visible');
  });

  it('will not create while a provider is still being described', () => {
    openProxyCreateForm();
    cy.get('[data-cyid="proxy-name-input"] input').type(proxyName);
    describePrimaryProvider();

    // A provider described but not added is not on the list Create reads.
    // Creating anyway would quietly make a proxy without it.
    cy.get('[data-cyid="add-provider-button"]').click();
    cy.get('[data-cyid="create-proxy-button"]').should('be.disabled');

    cy.get('[data-cyid="staged-provider-add"]', { timeout: 30000 })
      .should('not.be.disabled')
      .click();
    cy.get('[data-cyid="create-proxy-button"]').should('not.be.disabled');
  });

  it('keeps everything entered when the create fails', () => {
    openProxyCreateForm();

    cy.get('[data-cyid="proxy-name-input"] input').type(proxyName);
    describePrimaryProvider();
    cy.get('[data-cyid="add-provider-button"]').click();
    cy.get('[data-cyid="staged-provider-add"]', { timeout: 30000 })
      .should('not.be.disabled')
      .click();

    cy.intercept('POST', /\/llm-proxies(\?|$)/, { statusCode: 500, body: {} }).as(
      'failedCreate'
    );
    cy.get('[data-cyid="create-proxy-button"]').click();
    cy.wait('@failedCreate');

    // A failed save that empties the form costs the user everything they built,
    // and the more providers they added the more it costs.
    cy.get('[data-cyid="proxy-name-input"] input').should('have.value', proxyName);
    cy.get('[data-cyid="provider-row-0"]').should('be.visible');
    cy.get('[data-cyid="provider-row-1"]').should('be.visible');
  });

  it('sends every provider as one list, with exactly one primary', () => {
    openProxyCreateForm();
    cy.get('[data-cyid="proxy-name-input"] input').type(proxyName);
    describePrimaryProvider();

    cy.get('[data-cyid="create-proxy-button"]').should('not.be.disabled').click();
    cy.wait('@createProxy').then(({ request }) => {
      expect(request.body).to.have.property('providers');
      expect(request.body.providers).to.have.length.greaterThan(0);
      expect(
        request.body.providers.filter((entry) => entry.isPrimary)
      ).to.have.length(1);
      // The superseded single-provider field is not sent at all: the list is
      // the whole truth about which providers a proxy has.
      expect(request.body).to.not.have.property('provider');
      // A translator is recorded at the major of its version, which is what a
      // gateway resolves — a fuller one is accepted here and rejected there.
      request.body.providers
        .filter((entry) => entry.transformer)
        .forEach((entry) => {
          expect(entry.transformer.version).to.match(/^v\d+$/);
        });
    });

    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      appPathPattern(`/organizations/${orgHandle}/.*/proxies/[^/]+$`)
    );
  });
});
