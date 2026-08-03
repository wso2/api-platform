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

// The BFF rejects state-mutating requests that lack a custom CSRF header
// (CORS is closed, so cross-site attackers cannot set it). The SPA sends this
// header on every request; mirror that here so `cy.request` calls that hit the
// BFF proxy (POST/PUT/PATCH/DELETE) are not rejected with "missing CSRF header".
const CSRF_HEADER = Cypress.env('CSRF_HEADER') || 'X-Requested-By';
const CSRF_VALUE = Cypress.env('CSRF_VALUE') || 'ai-workspace';
const CSRF_SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);

Cypress.Commands.overwrite('request', (originalFn, ...args) => {
  // Every mutating request in this suite uses the options-object form, which is
  // the only signature that carries an explicit method and headers.
  if (args.length === 1 && args[0] !== null && typeof args[0] === 'object') {
    const options = { ...args[0] };
    const method = (options.method || 'GET').toUpperCase();
    if (!CSRF_SAFE_METHODS.has(method)) {
      options.headers = { ...(options.headers || {}), [CSRF_HEADER]: CSRF_VALUE };
    }
    return originalFn(options);
  }
  return originalFn(...args);
});

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

// Authenticate against the BFF proxy directly and resolve the organization id.
// Specs need this alongside `cy.login()` (which drives the UI) so that their
// cleanup hooks can run deterministically even when the UI flow failed early.
Cypress.Commands.add('apiLogin', () => {
  return cy
    .request({
      method: 'POST',
      url: '/proxy/api/portal/v0.9/auth/login',
      form: true,
      body: {
        username: Cypress.env('ADMIN_USER'),
        password: Cypress.env('ADMIN_PASSWORD'),
      },
    })
    .then((response) => {
      expect(response.status).to.eq(200);
      const authToken = response.body?.token ?? '';
      expect(authToken, 'auth token').to.not.equal('');

      return cy
        .request({
          url: '/proxy/api/v0.9/organizations',
          headers: { Authorization: `Bearer ${authToken}` },
        })
        .then((orgResponse) => {
          expect(orgResponse.status).to.eq(200);
          // This endpoint returns a single org in some deployments and a
          // {list: []} envelope in others; accept both rather than assuming.
          const organizationId =
            orgResponse.body?.id ?? orgResponse.body?.list?.[0]?.id ?? '';
          expect(organizationId, 'organization id').to.not.equal('');
          return { authToken, organizationId };
        });
    });
});

// Create a project through the UI and land on its list entry. Several specs
// need a project purely as a container, so the flow lives here rather than
// being re-typed (and re-drifting) in each one.
Cypress.Commands.add('createProjectUI', (projectName, description) => {
  cy.get('[data-cyid="nav-projects"]', { timeout: 30000 })
    .should('be.visible')
    .click();

  cy.contains('button, a', /Create Project|Add New Project/, { timeout: 30000 })
    .should('be.visible')
    .click();

  cy.get('input[placeholder="My AI Project"]', { timeout: 30000 })
    .should('be.visible')
    .type(projectName);
  cy.get('textarea[placeholder="Short description of the project."]').type(
    description ?? 'Cypress E2E project.'
  );
  cy.contains('button', 'Create').should('not.be.disabled').click();

  cy.contains(projectName, { timeout: 30000 }).should('be.visible');
});

// Look up a project by human-readable displayName and delete it by id.
Cypress.Commands.add('deleteProjectByNameApi', (authToken, targetName) => {
  if (!authToken) return cy.wrap(null);
  return cy
    .request({
      url: '/proxy/api/v0.9/projects',
      headers: { Authorization: `Bearer ${authToken}` },
      failOnStatusCode: false,
    })
    .then((response) => {
      if (response.status !== 200) return;
      const target = (response.body?.list ?? []).find(
        (project) => project.displayName === targetName
      );
      if (!target?.id) return;
      return cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/projects/${encodeURIComponent(target.id)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
    });
});

// Delete every MCP proxy whose displayName starts with "E2E ". The org caps MCP
// proxies (MaxMCPProxiesPerOrganization), so proxies leaked by an earlier failed
// run eventually make `create` return 409. The list endpoint is project-scoped
// while the cap is per-organization, so every project has to be swept.
Cypress.Commands.add('sweepE2EMCPProxies', (authToken) => {
  if (!authToken) return cy.wrap(null);
  const headers = { Authorization: `Bearer ${authToken}` };
  return cy
    .request({ url: '/proxy/api/v0.9/projects', headers, failOnStatusCode: false })
    .then((response) => {
      if (response.status !== 200) return;
      const projects = response.body?.list ?? [];
      projects.forEach((project) => {
        if (!project.id) return;
        cy.request({
          url: `/proxy/api/v0.9/mcp-proxies?projectId=${encodeURIComponent(project.id)}&limit=100&offset=0`,
          headers,
          failOnStatusCode: false,
        }).then((listResponse) => {
          if (listResponse.status !== 200) return;
          (listResponse.body?.list ?? [])
            .filter(
              (proxy) =>
                typeof proxy.displayName === 'string' &&
                proxy.displayName.startsWith('E2E ')
            )
            .forEach((proxy) => {
              if (!proxy.id) return;
              cy.request({
                method: 'DELETE',
                url: `/proxy/api/v0.9/mcp-proxies/${encodeURIComponent(proxy.id)}`,
                headers,
                failOnStatusCode: false,
              });
            });
        });
      });
    });
});

// Provider/proxy detail pages keep edits in a draft until the sticky bottom bar
// is committed, so a tab edit is not persisted until this runs.
Cypress.Commands.add('saveDraftChanges', () => {
  cy.contains('You have unsaved changes.', { timeout: 30000 }).should('be.visible');
  cy.contains('button', 'Save').should('not.be.disabled').click();
  cy.contains('You have unsaved changes.', { timeout: 30000 }).should('not.exist');
});

// Read the value out of an MUI field located by its FormLabel text. Most of the
// provider/proxy configuration tabs have no test ids, and the label is the only
// stable anchor that survives layout changes.
Cypress.Commands.add('fieldByLabel', (labelText) => {
  return cy
    .contains('label', labelText, { timeout: 30000 })
    .parents('.MuiFormControl-root')
    .first();
});

// Resolve the connected E2E gateway, or null when none is registered. Deploy
// specs use this to skip themselves rather than fail on a control-plane-only
// stack (docker-compose.yaml without scripts/start-e2e-gateway.sh).
Cypress.Commands.add('findConnectedGateway', (authToken) => {
  const gatewayName = Cypress.env('E2E_GATEWAY_NAME');
  return cy
    .request({
      url: '/proxy/api/v0.9/gateways',
      headers: { Authorization: `Bearer ${authToken}` },
      failOnStatusCode: false,
    })
    .then((response) => {
      if (response.status !== 200) return null;
      const gateways = response.body?.list ?? [];
      // isActive only flips true once the controller completes its registration
      // handshake, so it — not mere existence of the row — is the real signal.
      return (
        gateways.find((gw) => gw.id === gatewayName && gw.isActive) ??
        gateways.find((gw) => gw.isActive) ??
        null
      );
    });
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
        url: `/proxy/api/v0.9/llm-providers?organizationId=${encodeURIComponent(orgId)}&limit=${PAGE_SIZE}&offset=${offset}`,
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
            (p) =>
              typeof p.displayName === 'string' &&
              p.displayName.startsWith('E2E ')
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
        url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(providerId)}/llm-proxies?organizationId=${encodeURIComponent(orgId)}`,
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
              url: `/proxy/api/v0.9/llm-proxies/${encodeURIComponent(proxy.id)}?organizationId=${encodeURIComponent(orgId)}`,
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
              url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(provider.id)}?organizationId=${encodeURIComponent(orgId)}`,
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
      url: '/proxy/api/portal/v0.9/auth/login',
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
          url: '/proxy/api/v0.9/organizations',
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

/**
 * Starts recording the notification snackbars the app renders from this point on
 * (until the page reloads, i.e. the end of the test).
 *
 * The snackbar auto-hides after ~3.5s, so polling for the element with `cy.get()`
 * after the triggering action is inherently racy: on a loaded machine the runner
 * can stall long enough for the notification to be gone before the first retry
 * queries the DOM, and the assertion then fails even though the notification did
 * show. A MutationObserver records each snackbar's text as it appears, so the
 * assertion no longer depends on when it runs.
 *
 * Call this *before* the action that triggers the notification, then assert with
 * `cy.expectSnackbar(...)`.
 */
Cypress.Commands.add('recordSnackbars', () => {
  const selector = '[data-testid="aiworkspace-snackbar-notification"]';

  return cy.document({ log: false }).then((doc) => {
    const recorded = [];

    // Re-scan on every mutation rather than only inspecting addedNodes: a
    // snackbar can arrive either as a fresh subtree (the provider remounts it
    // with a new key) or as a text swap inside the mounted one.
    const scan = () => {
      doc.querySelectorAll(selector).forEach((element) => {
        const text = (element.textContent || '').trim();
        if (!text) return;
        if (recorded.some((entry) => entry.element === element && entry.text === text)) return;
        recorded.push({ element, text });
      });
    };

    const observer = new doc.defaultView.MutationObserver(scan);
    observer.observe(doc.body, {
      childList: true,
      subtree: true,
      characterData: true,
    });

    return cy.wrap(recorded, { log: false }).as('recordedSnackbars');
  });
});

/**
 * Asserts a snackbar captured by `cy.recordSnackbars()` matches `expectedText`
 * (string substring or RegExp). Retries until the command timeout, so it is safe
 * to call immediately after the triggering action.
 */
Cypress.Commands.add('expectSnackbar', (expectedText, options = {}) => {
  const matches = (text) =>
    expectedText instanceof RegExp
      ? expectedText.test(text)
      : text.includes(expectedText);

  // The timeout belongs on cy.get(): it is the alias re-query that retries, and
  // each retry re-reads the same (observer-mutated) array.
  return cy
    .get('@recordedSnackbars', { log: false, timeout: options.timeout ?? 15000 })
    .should((recorded) => {
      const texts = recorded.map((entry) => entry.text);
      expect(
        texts.some(matches),
        `a notification matching ${expectedText} was rendered (saw: ${JSON.stringify(texts)})`
      ).to.be.true;
    });
});
