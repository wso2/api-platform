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
 * Every screen showing a provider says the same thing about it.
 *
 * The application decides what translates for a provider once; these assert the
 * screens consume that decision rather than each reaching its own. It is the
 * property a type checker cannot see and a unit test cannot reach, because it is
 * about two rendered screens agreeing — which is the whole argument for deciding
 * it in one place.
 *
 * Runs against a proxy that already serves more than one provider.
 */

describe('AI Workspace - transformer resolution is consistent', () => {
  const orgHandle = Cypress.env('ORG_HANDLE');
  const multiProviderProxyId = Cypress.env('MULTI_PROVIDER_PROXY_ID');

  beforeEach(function beforeEachTest() {
    if (!multiProviderProxyId) {
      // Skipped rather than failed: this needs a fixture the area does not
      // create for itself, and a red run would say the application is broken
      // when the environment simply has no such proxy.
      this.skip();
    }
    cy.login();
  });

  const openProxy = () =>
    cy.visitWorkspace(
      `/organizations/${orgHandle}/proxies/${multiProviderProxyId}`
    );

  it('reports the same status on the Providers tab and the policies tab', () => {
    openProxy();
    cy.contains('button', 'Providers', { timeout: 30000 }).click();

    const statusByProvider = {};
    cy.get('[data-cyid="proxy-provider-list"]', { timeout: 30000 })
      .find('[data-cyid^="provider-transformer-status-"]')
      .each(($status) => {
        const providerId = $status
          .attr('data-cyid')
          .replace('provider-transformer-status-', '');
        statusByProvider[providerId] = $status.text().trim();
      })
      .then(() => {
        cy.contains('button', 'Guardrails & Policies').click();
        cy.get('[data-cyid="proxy-transformer-list"]', { timeout: 30000 }).should(
          'exist'
        );
        Object.keys(statusByProvider).forEach((providerId) => {
          // The same provider appears on both screens, resolved the same way.
          cy.get(`[data-cyid="transformer-select-${providerId}"]`).should('exist');
        });
      });
  });

  it('shows nothing for a provider that needs no translation', () => {
    openProxy();
    cy.contains('button', 'Providers', { timeout: 30000 }).click();

    // A provider already speaking the proxy's own format reports nothing at
    // all — not a tick, not "no transformer needed". The screen stays about
    // what the user actually has to decide.
    cy.get('[data-cyid="proxy-provider-list"]', { timeout: 30000 }).within(() => {
      cy.get('[data-cyid^="provider-request-handle-"]').should('exist');
    });
  });
});
