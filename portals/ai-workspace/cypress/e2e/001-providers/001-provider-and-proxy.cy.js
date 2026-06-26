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

describe('AI Workspace - OpenAI provider and proxy lifecycle', () => {
  const suffix = Date.now().toString().slice(-8);
  const orgHandle = Cypress.env('ORG_HANDLE');
  const projectName = `E2E Project ${suffix}`;
  const cleanupProjectName = 'AI Workspace E2E Cleanup Project';
  const providerName = `E2E OpenAI Provider ${suffix}`;
  const providerId = toSlug(providerName);
  const providerDescription = 'Cypress OpenAI provider UI lifecycle test';
  const proxyName = `E2E OpenAI Proxy ${suffix}`;
  const proxyId = toSlug(proxyName);
  const proxyDescription = 'Cypress App LLM proxy UI lifecycle test';
  let createdProviderId = providerId;
  let authToken = '';
  let organizationId = '';

  beforeEach(() => {
    cy.login();
    cy.request({
      method: 'POST',
      url: '/api-proxy/api/portal/v1/auth/login',
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
          url: '/api-proxy/api/v0.9/organizations',
          headers: {
            Authorization: `Bearer ${authToken}`,
          },
        });
      })
      .then((response) => {
        expect(response.status).to.eq(200);
        organizationId = response.body?.id ?? '';
        expect(organizationId).to.not.equal('');
      });
  });

  afterEach(() => {
    if (!authToken || !organizationId) {
      return;
    }

    return deleteLinkedProxies(authToken, organizationId, createdProviderId)
      .then(() => deleteProjectByName(authToken, projectName, cleanupProjectName))
      .then(() => deleteProvider(authToken, organizationId, createdProviderId));
  });

  it('creates and deletes an OpenAI provider and app llm proxy using only the UI', () => {
    cy.intercept('POST', '**/projects').as('createProject');
    cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProvider');
    cy.intercept('POST', /\/llm-proxies(\?|$)/).as('createProxy');
    cy.intercept('DELETE', '**/llm-proxies/**').as('deleteProxy');

    cy.contains('Projects', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('button, a', /Create Project|Add New Project/, {
      timeout: 30000,
    })
      .should('be.visible')
      .click();

    cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
      .should('be.visible')
      .type(projectName);
    cy.get('textarea[placeholder="Short description of the project."]').type(
      'Cypress project for provider and proxy lifecycle coverage.'
    );
    cy.contains('button', 'Create')
      .should('not.be.disabled')
      .click();
    cy.wait('@createProject').its('response.statusCode').should('be.oneOf', [200, 201]);

    cy.contains(projectName, { timeout: 30000 }).should('be.visible');

    cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.get('[data-cyid="provider-template-openai-card"]', {
      timeout: 30000,
    }).should('be.visible').click();

    cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
      .should('be.visible')
      .clear()
      .type(providerName);
    cy.get('[data-cyid="provider-description-input"] textarea:visible')
      .clear()
      .type(providerDescription);
    cy.get('[data-cyid="provider-context-input"] input:visible').should(
      'have.value',
      `/${providerId}`
    );
    cy.get('[data-cyid="provider-api-key-input"] input:visible').type(
      'sk-e2e-openai-provider-key'
    );
    cy.get('[data-cyid="add-provider-button"]')
      .should('not.be.disabled')
      .click();
    cy.wait('@createProvider').its('response.statusCode').should('be.oneOf', [200, 201]);

    cy.location('pathname', { timeout: 30000 })
      .should(
        'match',
        new RegExp(`^/organizations/${orgHandle}/service-provider/[^/]+$`)
      )
      .then((pathname) => {
        createdProviderId = pathname.split('/').pop() || '';
        expect(createdProviderId).to.not.equal('');
      });
    cy.contains(providerName, { timeout: 30000 }).should('be.visible');

    cy.contains('button', 'Create App LLM Proxy', { timeout: 30000 })
      .should('be.visible')
      .click();

    cy.contains('label', 'Projects', { timeout: 30000 })
      .parent()
      .find('[role="combobox"]')
      .click();
    cy.contains('[role="option"]', projectName, { timeout: 30000 }).click();
    cy.contains('button', 'Continue')
      .should('not.be.disabled')
      .click();

    cy.contains('Create App LLM Proxy', { timeout: 30000 }).should(
      'be.visible'
    );
    cy.get('input[placeholder="WSO2 OpenAI Provider Proxy"]', {
      timeout: 30000,
    })
      .should('be.visible')
      .type(proxyName);
    cy.get('textarea[placeholder="Primary OpenAI provider"]').type(
      proxyDescription
    );
    cy.get('input[placeholder="Enter API key"]').type('sk-e2e-openai-proxy-key');
    cy.contains('button', 'Create Proxy', { timeout: 30000 })
      .should('not.be.disabled')
      .click();
    cy.wait('@createProxy').its('response.statusCode').should('be.oneOf', [200, 201]);

    cy.location('pathname', { timeout: 30000 }).should('match', /\/proxies\/[^/]+$/);
    cy.contains(proxyName, { timeout: 30000 }).should('be.visible');

    cy.get('button[aria-label="Delete proxy"]', { timeout: 30000 })
      .should('be.visible')
      .click();
    cy.contains('Delete App LLM Proxy', { timeout: 30000 }).should('be.visible');
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', 'Delete').click();
    });
    cy.wait('@deleteProxy').its('response.statusCode').should('be.oneOf', [200, 204]);

    cy.location('pathname', { timeout: 30000 }).should('match', /\/proxies\/?$/);
    cy.contains(proxyName, { timeout: 30000 }).should('not.exist');
  });
});

function deleteLinkedProxies(authToken, organizationId, providerId) {
  return requestWithAuth(authToken, {
    url: `/api-proxy/api/v0.9/llm-providers/${encodeURIComponent(providerId)}/llm-proxies?organizationId=${encodeURIComponent(organizationId)}`,
    failOnStatusCode: false,
  }).then((response) => {
    expect(response.status).to.be.oneOf([200, 404]);

    if (response.status === 404) {
      return;
    }

    const proxies = response.body?.list ?? [];
    if (!proxies.length) {
      return;
    }

    return cy.wrap(proxies).each((proxy) =>
      requestWithAuth(authToken, {
        method: 'DELETE',
        url: `/api-proxy/api/v0.9/llm-proxies/${encodeURIComponent(proxy.id)}?organizationId=${encodeURIComponent(organizationId)}`,
        failOnStatusCode: false,
      }).then((deleteResponse) => {
        expect(deleteResponse.status).to.be.oneOf([200, 204, 404]);
      })
    );
  });
}

function deleteProjectByName(authToken, targetProjectName, fallbackProjectName) {
  return requestWithAuth(authToken, {
    url: '/api-proxy/api/v0.9/projects',
  }).then((response) => {
    expect(response.status).to.eq(200);

    const projects = response.body?.list ?? [];
    const targetProject = projects.find(
      (project) => project.name === targetProjectName
    );

    if (!targetProject?.id) {
      return;
    }

    if (projects.length <= 1) {
      return ensureFallbackProject(authToken, fallbackProjectName).then(() =>
        deleteProject(authToken, targetProject.id)
      );
    }

    return deleteProject(authToken, targetProject.id);
  });
}

function ensureFallbackProject(authToken, fallbackProjectName) {
  return requestWithAuth(authToken, {
    method: 'POST',
    url: '/api-proxy/api/v0.9/projects',
    body: {
      name: fallbackProjectName,
      description: 'Reserved project to satisfy E2E cleanup invariants.',
    },
    failOnStatusCode: false,
  }).then((response) => {
    expect(response.status).to.be.oneOf([200, 201, 409]);
  });
}

function deleteProject(authToken, projectId) {
  return requestWithAuth(authToken, {
    method: 'DELETE',
    url: `/api-proxy/api/v0.9/projects/${encodeURIComponent(projectId)}`,
    failOnStatusCode: false,
  }).then((response) => {
    expect(response.status).to.be.oneOf([200, 204, 404]);
  });
}

function deleteProvider(authToken, organizationId, providerId) {
  return requestWithAuth(authToken, {
    method: 'DELETE',
    url: `/api-proxy/api/v0.9/llm-providers/${encodeURIComponent(providerId)}?organizationId=${encodeURIComponent(organizationId)}`,
    failOnStatusCode: false,
  }).then((response) => {
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
