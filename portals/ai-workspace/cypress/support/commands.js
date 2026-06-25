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

Cypress.Commands.add('visitWorkspace', (path = '/') => {
  const introStorageKey = Cypress.env('QS_INTRO_STORAGE_KEY');

  cy.visit(path, {
    onBeforeLoad(win) {
      win.localStorage.setItem(introStorageKey, '1');
    },
  });
});

Cypress.Commands.add('login', (username, password) => {
  const user = username || Cypress.env('ADMIN_USER');
  const pwd = password || Cypress.env('ADMIN_PASSWORD');
  const orgHandle = Cypress.env('ORG_HANDLE');

  cy.visitWorkspace('/');
  cy.get('input[placeholder="username"]').should('be.visible').type(user);
  cy.get('input[type="password"]').should('be.visible').type(pwd);
  cy.contains('button', 'Sign In').click();

  cy.location('pathname', { timeout: 30000 }).should(
    'match',
    new RegExp(`^/organizations/${orgHandle}(?:/|$)`)
  );
  cy.contains('Quick Start', { timeout: 30000 }).should('be.visible');
  cy.contains('Projects').should('be.visible');
});

Cypress.Commands.add('sweepE2EProviders', (authToken, organizationId) => {
  const PAGE_SIZE = 100;
  const headersFor = (token) => ({ Authorization: `Bearer ${token}` });

  // Collect every stale `E2E ` provider, paging until the API returns a short
  // page. Collection finishes before any deletes so the offset window stays
  // consistent.
  const collectE2EProviders = (token, orgId, offset = 0, acc = []) =>
    cy
      .request({
        method: 'GET',
        url: `/api-proxy/api/v0.9/llm-providers?organizationId=${encodeURIComponent(orgId)}&limit=${PAGE_SIZE}&offset=${offset}`,
        headers: headersFor(token),
        failOnStatusCode: false,
      })
      .then((response) => {
        // Cleanup runs in afterEach; a transient non-200 from the list endpoint
        // must not hard-fail the spec it is cleaning up after. Skip this page.
        if (response.status !== 200) return acc;
        const page = response.body?.list ?? [];
        const next = acc.concat(
          page.filter(
            (p) => typeof p.name === 'string' && p.name.startsWith('E2E ')
          )
        );
        if (page.length < PAGE_SIZE) return next;
        return collectE2EProviders(token, orgId, offset + PAGE_SIZE, next);
      });

  // A provider with linked proxies cannot be deleted directly, so clear those
  // first to keep the sweep from silently leaving stale state behind.
  const deleteLinkedProxies = (token, orgId, providerId) =>
    cy
      .request({
        method: 'GET',
        url: `/api-proxy/api/v0.9/llm-providers/${encodeURIComponent(providerId)}/llm-proxies?organizationId=${encodeURIComponent(orgId)}`,
        headers: headersFor(token),
        failOnStatusCode: false,
      })
      .then((response) => {
        if (response.status === 404) return;
        const proxies = response.body?.list ?? [];
        if (!proxies.length) return;
        return cy.wrap(proxies).each((proxy) =>
          cy
            .request({
              method: 'DELETE',
              url: `/api-proxy/api/v0.9/llm-proxies/${encodeURIComponent(proxy.id)}?organizationId=${encodeURIComponent(orgId)}`,
              headers: headersFor(token),
              failOnStatusCode: false,
            })
            .then((deleteResponse) => {
              expect(deleteResponse.status).to.be.oneOf([200, 204, 404]);
            })
        );
      });

  const doSweep = (token, orgId) =>
    collectE2EProviders(token, orgId).then((e2eProviders) => {
      if (!e2eProviders.length) return;
      return cy.wrap(e2eProviders).each((provider) =>
        deleteLinkedProxies(token, orgId, provider.id).then(() =>
          cy
            .request({
              method: 'DELETE',
              url: `/api-proxy/api/v0.9/llm-providers/${encodeURIComponent(provider.id)}?organizationId=${encodeURIComponent(orgId)}`,
              headers: headersFor(token),
              failOnStatusCode: false,
            })
            .then((deleteResponse) => {
              // Surface a failed delete so the sweep does not pass while leaving
              // the next suite to start from dirty state.
              expect(deleteResponse.status).to.be.oneOf([200, 204, 404]);
            })
        )
      );
    });

  if (authToken && organizationId) {
    return doSweep(authToken, organizationId);
  }

  return cy
    .request({
      method: 'POST',
      url: '/api-proxy/api/portal/v1/auth/login',
      form: true,
      body: {
        username: Cypress.env('ADMIN_USER'),
        password: Cypress.env('ADMIN_PASSWORD'),
      },
      failOnStatusCode: false,
    })
    .then((loginResp) => {
      if (loginResp.status !== 200) return;
      const token = loginResp.body?.token;
      if (!token) return;
      return cy
        .request({
          url: '/api-proxy/api/v0.9/organizations',
          headers: { Authorization: `Bearer ${token}` },
          failOnStatusCode: false,
        })
        .then((orgResp) => {
          if (orgResp.status !== 200) return;
          const orgId = orgResp.body?.id;
          if (!orgId) return;
          return doSweep(token, orgId);
        });
    });
});
