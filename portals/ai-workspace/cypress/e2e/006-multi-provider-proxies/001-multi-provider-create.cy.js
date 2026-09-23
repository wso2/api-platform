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
 * Reaches the create page the way the provider lifecycle spec in area 001 does —
 * through the UI, from a provider — because there is no direct route to it.
 */

import { appPathPattern } from '../../support/appPath';

describe('AI Workspace - multi-provider proxy creation', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const proxyName = `E2E Multi Proxy ${suffix}`;

  beforeEach(() => {
    cy.login();
    cy.intercept('POST', /\/llm-proxies(\?|$)/).as('createProxy');
  });

  /**
   * Opens the proxy create form from the first provider in the list.
   *
   * Assumes at least one provider exists, which area 001 leaves behind.
   */
  const openProxyCreateForm = () => {
    cy.visitWorkspace(`/organizations/${orgHandle}/service-provider`);
    cy.contains('button', 'Create App LLM Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.contains('button', 'Continue', { timeout: 30000 })
      .should('not.be.disabled')
      .click();
    cy.get('[data-cyid="proxy-name-input"]', { timeout: 30000 }).should('be.visible');
  };

  it('asks nothing extra of a single-provider proxy', () => {
    openProxyCreateForm();

    // The page opens with one provider form. No list, no primary toggle, no
    // interface choice — a user building a single-provider proxy makes exactly
    // the decisions they made before any of this existed.
    cy.get('[data-cyid="proxy-provider-select"]').should('exist');
    cy.get('[data-cyid="additional-provider-card-0"]').should('not.exist');
    cy.get('[data-cyid="inbound-interface-select"]').should('not.exist');

    // One action reveals the rest, and only then.
    cy.get('[data-cyid="add-provider-button"]').should('exist').click();
    cy.get('[data-cyid="additional-provider-card-0"]').should('be.visible');
    cy.get('[data-cyid="inbound-interface-select"]').should('be.visible');

    // And it can be taken back.
    cy.get('[data-cyid="remove-additional-provider-0"]').click();
    cy.get('[data-cyid="additional-provider-card-0"]').should('not.exist');
  });

  it('keeps everything entered when the create fails', () => {
    openProxyCreateForm();

    cy.get('[data-cyid="proxy-name-input"] input').type(proxyName);
    cy.get('[data-cyid="add-provider-button"]').click();
    cy.get('[data-cyid="additional-provider-api-key-0"] input').type('entered-by-hand');

    cy.intercept('POST', /\/llm-proxies(\?|$)/, { statusCode: 500, body: {} }).as(
      'failedCreate'
    );
    cy.get('[data-cyid="create-proxy-button"]').click();
    cy.wait('@failedCreate');

    // A failed save that empties the form costs the user everything they built,
    // and the more providers they added the more it costs.
    cy.get('[data-cyid="proxy-name-input"] input').should('have.value', proxyName);
    cy.get('[data-cyid="additional-provider-card-0"]').should('be.visible');
    cy.get('[data-cyid="additional-provider-api-key-0"] input').should(
      'have.value',
      'entered-by-hand'
    );
  });

  it('sends every provider as one list, with exactly one primary', () => {
    openProxyCreateForm();
    cy.get('[data-cyid="proxy-name-input"] input').type(proxyName);

    cy.get('[data-cyid="create-proxy-button"]').click();
    cy.wait('@createProxy').then(({ request }) => {
      expect(request.body).to.have.property('providers');
      expect(request.body.providers).to.have.length.greaterThan(0);
      expect(
        request.body.providers.filter((entry) => entry.isPrimary)
      ).to.have.length(1);
      // The superseded single-provider field is not sent at all: the list is
      // the whole truth about which providers a proxy has.
      expect(request.body).to.not.have.property('provider');
    });

    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      appPathPattern(`/organizations/${orgHandle}/.*/proxies/[^/]+$`)
    );
  });
});
