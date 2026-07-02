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

describe('AI Workspace - Custom LLM provider template lifecycle', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const templateName = `E2E Custom Template ${suffix}`;
  const templateV1Id = toTemplateId(`${templateName} v1.0`);
  const templateV2Id = toTemplateId(`${templateName} v2.0`);
  const providerName = `E2E Custom Template Provider ${suffix}`;
  const providerId = toSlug(providerName);
  let createdProviderId = '';
  let authToken = '';
  let organizationId = '';

  before(() => {
    cy.sweepE2EProviders();
  });

  beforeEach(() => {
    cy.login();
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');
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
          headers: {
            Authorization: `Bearer ${authToken}`,
          },
        });
      })
      .then((response) => {
        expect(response.status).to.eq(200);
        const orgs = response.body?.list ?? [];
        organizationId = orgs[0]?.id ?? '';
        expect(organizationId).to.not.equal('');
      });
  });

  afterEach(() => {
    const targetProviderId = createdProviderId || providerId;
    const targeted = (authToken && organizationId)
      ? deleteProvider(authToken, organizationId, targetProviderId)
          .then(() => waitForProviderGone(authToken, organizationId, targetProviderId))
          .then(() => deleteTemplateVersion(authToken, organizationId, templateV2Id))
          .then(() => deleteTemplateVersion(authToken, organizationId, templateV1Id))
      : cy.wrap(null);

    return targeted.then(() => cy.sweepE2EProviders(authToken, organizationId));
  });

  it('creates a custom template, versions it, uses it for a provider, and blocks deletion while in use', () => {
    // --- Create the custom template (v1.0) ---------------------------------
    cy.contains('Settings', { timeout: 30000 }).should('be.visible').click();
    cy.contains('LLM Provider Templates', { timeout: 30000 }).should('be.visible');

    cy.get('[data-cyid="add-provider-template-button"]', { timeout: 30000 })
      .scrollIntoView()
      .should('be.visible')
      .click();

    cy.get('input[placeholder="Enter template name"]', { timeout: 30000 })
      .should('be.visible')
      .type(templateName);
    cy.get('input[placeholder="https://api.openai.com"]').type(
      'https://api.e2e-custom-template.example.com'
    );
    cy.get('[data-cyid="create-provider-template-submit"]')
      .should('not.be.disabled')
      .click();

    cy.contains('LLM Provider Templates', { timeout: 30000 }).should('be.visible');
    cy.get(`[data-cyid="provider-template-card-${templateV1Id}"]`, {
      timeout: 30000,
    })
      .should('be.visible')
      .click();
    cy.contains(templateName, { timeout: 30000 }).should('be.visible');
    cy.contains('button', 'v1.0', { timeout: 30000 }).should('be.visible');

    // --- Create a second version (v2.0) of the same template family --------
    cy.contains('button', 'v1.0').click();
    cy.contains('Create new version', { timeout: 30000 }).click();

    cy.get('input[placeholder="v2.0"]', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type('v2.0');
    cy.get('input[placeholder="https://api.openai.com"]').type(
      'https://api.e2e-custom-template.example.com'
    );
    cy.get('[data-cyid="create-provider-template-version-submit"]')
      .should('not.be.disabled')
      .click();

    cy.contains(templateName, { timeout: 30000 }).should('be.visible');
    cy.contains('button', 'v2.0', { timeout: 30000 }).should('be.visible');

    // --- Use the template (now with 2 enabled versions) to create a provider ---
    cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.get(`[data-cyid="provider-template-${templateV2Id}-card"]`, {
      timeout: 30000,
    })
      .should('be.visible')
      .click();

    cy.get('[data-cyid="template-version-continue-button"]', {
      timeout: 45000,
    }).should('be.visible');
    cy.get('[data-cyid="template-version-option-v2.0"]', { timeout: 30000 }).click();
    cy.get('[data-cyid="template-version-continue-button"]', { timeout: 30000 })
      .should('not.be.disabled')
      .click();

    cy.get('[data-cyid="provider-name-input"] input:visible', {
      timeout: 30000,
    })
      .should('be.visible')
      .clear()
      .type(providerName);
    cy.get('[data-cyid="provider-context-input"] input:visible').should(
      'have.value',
      `/${providerId}`
    );
    cy.get('[data-cyid="provider-api-key-input"] input:visible').type(
      'sk-e2e-custom-template-provider-key'
    );
    cy.get('[data-cyid="add-provider-button"]')
      .should('not.be.disabled')
      .click();

    // Capture the provider id from the create response, not the URL. The create
    // flow issues POST /secrets before POST /llm-providers, so the redirect to
    // /service-provider/<id> lands a beat after the click — scraping the URL
    // races against it and can capture the transient "new" form route.
    cy.wait('@createProvider').then(({ response }) => {
      expect(response?.statusCode).to.be.oneOf([200, 201]);
      createdProviderId = response?.body?.id || providerId;
      expect(createdProviderId).to.not.equal('');
      expect(createdProviderId).to.not.equal('new');
    });
    // The view must settle on the created provider, never the "new" form route.
    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      new RegExp(`^/organizations/${orgHandle}/service-provider/(?!new$)[^/]+$`)
    );
    cy.contains(providerName, { timeout: 30000 }).should('be.visible');

    // --- Deleting the version while a provider uses it must be blocked -----
    cy.contains('Settings', { timeout: 30000 }).should('be.visible').click();
    cy.get(`[data-cyid="provider-template-card-${templateV2Id}"]`, {
      timeout: 30000,
    })
      .should('be.visible')
      .click();
    cy.contains('button', 'v2.0', { timeout: 30000 }).should('be.visible');

    cy.get('[data-cyid="provider-template-delete-button"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });
    cy.contains(
      'Cannot delete: one or more providers were created from this template.',
      { timeout: 30000 }
    ).should('be.visible');

    // --- Remove the provider, then deletion is allowed ----------------------
    cy.then(() => deleteProvider(authToken, organizationId, createdProviderId));
    cy.then(() => waitForProviderGone(authToken, organizationId, createdProviderId));

    cy.get('[data-cyid="provider-template-delete-button"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });
    cy.contains('button', 'v1.0', { timeout: 30000 }).should('exist');
    cy.get('[data-cyid="provider-template-delete-button"]', { timeout: 30000 })
      .scrollIntoView()
      .should('be.visible')
      .click();
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });

    // That was the only remaining version, so the whole template is gone.
    cy.contains('LLM Provider Templates', { timeout: 30000 }).should('be.visible');
    cy.contains(templateName).should('not.exist');
  });
});

function waitForProviderGone(authToken, organizationId, providerId) {
  return requestWithAuth(authToken, {
    url: `/api/proxy/api/v0.9/llm-providers?organizationId=${encodeURIComponent(organizationId)}`,
  }).then((res) => {
    const stillThere = (res.body?.list ?? []).some((p) => p.id === providerId);
    if (stillThere) {
      cy.wait(500);
      return waitForProviderGone(authToken, organizationId, providerId);
    }
  });
}


function deleteProvider(authToken, organizationId, targetProviderId) {
  if (!targetProviderId) {
    return cy.wrap(null);
  }
  return requestWithAuth(authToken, {
    method: 'DELETE',
    url: `/api/proxy/api/v0.9/llm-providers/${encodeURIComponent(targetProviderId)}?organizationId=${encodeURIComponent(organizationId)}`,
    failOnStatusCode: false,
  }).then((response) => {
    expect(response.status).to.be.oneOf([200, 204, 404]);
  });
}

function deleteTemplateVersion(authToken, organizationId, templateId) {
  return requestWithAuth(authToken, {
    method: 'DELETE',
    url: `/api/proxy/api/v0.9/llm-provider-templates/${encodeURIComponent(templateId)}`,
    failOnStatusCode: false,
  }).then((response) => {
    if (response.status === 409) {
      throw new Error(`Template version ${templateId} is still in use during cleanup`);
    }
    expect(response.status).to.be.oneOf([200, 204, 404]);
  });
}

function requestWithAuth(authToken, options) {
  const headers = {
    Authorization: `Bearer ${authToken}`,
    ...(options.headers ?? {}),
  };

  return cy.request({
    ...options,
    headers,
  });
}

function toSlug(value) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function toTemplateId(value) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
