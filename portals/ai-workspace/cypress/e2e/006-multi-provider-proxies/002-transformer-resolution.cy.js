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

  const openProvidersTab = () => {
    openProxy();
    cy.contains('button', 'Providers', { timeout: 30000 }).click();
    return cy.get('[data-cyid="proxy-provider-list"]', { timeout: 30000 });
  };

  it('gives every provider a row naming what a client routes on', () => {
    openProvidersTab()
      .find('[data-cyid^="provider-row-"][data-cyid$="-handle"]')
      .should('have.length.greaterThan', 1)
      .each(($handle) => {
        // The handle is the alias where there is one and the id otherwise —
        // either way the value a client puts in the routing header. A row
        // showing something that does not route is worse than showing nothing.
        expect($handle.text().trim()).to.not.equal('');
      });
  });

  it('says the same thing about a provider on the row and in its settings', () => {
    openProvidersTab()
      .find('[data-cyid="provider-row-0"]')
      .invoke('text')
      .then((rowText) => {
        cy.get('[data-cyid="provider-row-0-edit"]').click();
        cy.get('[data-cyid="provider-settings-drawer"]', { timeout: 30000 })
          .should('be.visible');
        cy.get('[data-cyid="provider-settings-transformer"]')
          .invoke('text')
          .then((panelText) => {
            // Both read the one decision. Where the row names a translator the
            // panel names the same one; where the row says none is needed the
            // panel says so too.
            const named = rowText.match(/[a-z0-9-]+-transformer/);
            if (named) {
              expect(panelText).to.contain(named[0]);
            } else {
              expect(panelText).to.contain('No transformer');
            }
          });
        cy.get('[data-cyid="provider-settings-cancel"]').click();
      });
  });

  it('keeps transformers with their providers, not among the proxy policies', () => {
    openProxy();
    cy.contains('button', 'Guardrails & Policies', { timeout: 30000 }).click();

    // A translator belongs to one provider rather than to the proxy. Listing
    // it here as well meant two screens editing one setting.
    cy.contains('Provider Transformers').should('not.exist');
    cy.get('[data-cyid="proxy-transformer-list"]').should('not.exist');
  });

  it('reads the interface in effect, not only the one stored', () => {
    openProvidersTab();
    // A proxy created before the setting existed stores none and runs on its
    // primary provider's format. Every provider on it reading "No transformer
    // configured" is the symptom of passing the empty value through.
    cy.get('[data-cyid="providers-inbound-interface"]')
      .invoke('text')
      .should('not.contain', '—');
  });
});
