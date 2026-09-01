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
 * Dedicated Settings > Secrets pages (list, create, overview, rotate, delete).
 *
 * Every other *-secret-management.cy.js spec in this suite covers a secret as a
 * SIDE EFFECT of configuring a provider/proxy/MCP server — none of them ever visit
 * `/settings/secrets` itself. This spec covers the standalone CRUD pages directly:
 *
 *   TC-1  Create Secret form -> lands on the Overview page, plaintext never rendered
 *   TC-2  Secrets list: pagination (>1 page of results) and search (filters across
 *         the whole list, not just the visible page)
 *   TC-3  Rotate: submitting with a blank display name is rejected client-side with
 *         no PUT request; a valid edit (new name/value/description) persists
 *   TC-4a Delete (blocked): the "in use" conflict dialog names the blocking resource
 *         but never its internal handle/UUID; once its provider is deleted, the
 *         now-orphaned secret is auto-cleaned-up and permanently gone (404), not
 *         merely deprecated
 *   TC-4b Delete (unblocked): a plain, unreferenced secret's delete succeeds via
 *         the UI and redirects to the list
 */

import { appPathPattern } from '../../support/appPath';

const orgHandle = Cypress.env('ORG_HANDLE');

function loginAndFetchAuthContext(setAuthToken, setOrganizationId) {
  cy.login();
  cy.request({
    method: 'POST',
    url: '/api/login',
    body: {
      username: Cypress.env('ADMIN_USER'),
      password: Cypress.env('ADMIN_PASSWORD'),
    },
  })
    .then((r) => {
      setAuthToken(r.body.accessToken);
      return cy.request({
        url: '/proxy/api/v0.9/organizations',
        headers: { Authorization: `Bearer ${r.body.accessToken}` },
      });
    })
    .then((r) => {
      setOrganizationId(r.body?.list?.[0]?.id ?? '');
    });
}

// Minimal, local variant of the provider-creation helper other secret-management
// specs use — duplicated rather than imported (spec files in this suite don't
// share helpers across files; see 005-custom-provider-template.cy.js for the
// same toSlug/toTemplateId duplication pattern).
function createProviderViaUI(providerName) {
  cy.intercept('POST', /\/llm-providers(\?|$)/).as('createProviderForDeleteTest');
  cy.visitWorkspace(`/organizations/${orgHandle}`);
  cy.get('[data-cyid="nav-service-provider"]', { timeout: 30000 }).should('be.visible').click();
  cy.get('[data-cyid="add-new-provider-button"]', { timeout: 30000 }).should('be.visible').click();
  cy.get('[data-cyid="provider-template-openai-card"]', { timeout: 30000 }).should('be.visible').click();
  cy.get('[data-cyid="provider-name-input"] input:visible', { timeout: 30000 })
    .should('be.visible')
    .clear()
    .type(providerName);
  cy.get('[data-cyid="provider-api-key-input"] input:visible').type('sk-e2e-secrets-page-provider-key');
  cy.get('[data-cyid="add-provider-button"]').should('not.be.disabled').click();
  return cy.wait('@createProviderForDeleteTest', { timeout: 20000 }).then((pi) => pi.response.body?.id ?? '');
}

describe('AI Workspace — Secrets management pages', () => {
  const suffix = Date.now().toString().slice(-8);

  let authToken = '';
  let organizationId = '';
  // Secret handles this spec created directly via the API (bulk pagination fixtures,
  // or a UI-created secret whose id we captured) — swept up in afterEach regardless
  // of which test created them or whether the test itself already deleted them.
  let createdHandles = [];
  let createdProviderId = '';

  function apiCreateSecret(handle, displayName, value) {
    return cy
      .request({
        method: 'POST',
        url: '/proxy/api/v0.9/secrets',
        headers: { Authorization: `Bearer ${authToken}` },
        form: true,
        body: { id: handle, displayName, value, type: 'GENERIC' },
        failOnStatusCode: false,
      })
      .then((r) => {
        expect(r.status, `create fixture secret ${handle}`).to.be.oneOf([200, 201]);
        createdHandles.push(handle);
        return r;
      });
  }

  function apiDeleteSecret(handle) {
    return cy.request({
      method: 'DELETE',
      url: `/proxy/api/v0.9/secrets/${encodeURIComponent(handle)}?organizationId=${encodeURIComponent(organizationId)}`,
      headers: { Authorization: `Bearer ${authToken}` },
      failOnStatusCode: false,
    });
  }

  // Deleting the resource that referenced an auto-generated secret doesn't
  // remove that secret synchronously within the same request/response —
  // platform-api's cleanup (SecretService.CleanupOrphanedSecrets) runs as a
  // best-effort follow-up that itself permanently deletes the now-orphaned
  // secret. Poll for that instead of retrying a manual delete, which would
  // 404 once cleanup has already removed it.
  function pollUntilSecretGone(handle, attemptsLeft = 15) {
    return cy
      .request({
        url: `/proxy/api/v0.9/secrets/${encodeURIComponent(handle)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      })
      .then((r) => {
        if (r.status === 404) return;
        if (attemptsLeft <= 1) {
          throw new Error(`secret ${handle} was not cleaned up after its referencing resource was deleted`);
        }
        cy.wait(1000, { log: false });
        return pollUntilSecretGone(handle, attemptsLeft - 1);
      });
  }

  beforeEach(() => {
    createdHandles = [];
    createdProviderId = '';
    loginAndFetchAuthContext(
      (v) => { authToken = v; },
      (v) => { organizationId = v; }
    );
  });

  afterEach(() => {
    if (!authToken || !organizationId) return;
    if (createdProviderId) {
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(createdProviderId)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      });
    }
    createdHandles.forEach((handle) => apiDeleteSecret(handle));
  });

  // -------------------------------------------------------------------------
  // TC-1: Create Secret form -> Overview page
  // -------------------------------------------------------------------------
  it('TC-1: creates a secret via the Create Secret form and lands on its Overview page', () => {
    const secretName = `E2E Secret Create ${suffix}`;
    const secretHandle = `e2e-secret-create-${suffix}`;
    const secretValue = `sk-e2e-secret-value-${suffix}`;

    cy.intercept('POST', '**/secrets').as('createSecret');

    cy.visitWorkspace(`/organizations/${orgHandle}/settings/secrets/new`);

    cy.get('[data-cyid="secret-name-input"] input:visible').should('be.visible').type(secretName);
    // Handle auto-derives from the display name (see CreateSecret.tsx's toHandle) —
    // assert it matches what apiDeleteSecret below will clean up, rather than
    // re-typing it, so this also guards the auto-slug behavior itself.
    cy.get('[data-cyid="secret-handle-input"] input:visible').should('have.value', secretHandle);
    cy.get('[data-cyid="secret-value-input"] input:visible').type(secretValue);
    cy.get('[data-cyid="create-secret-submit"]').should('not.be.disabled').click();

    cy.wait('@createSecret').then((interception) => {
      expect(interception.response.statusCode, 'POST /secrets status').to.be.oneOf([200, 201]);
      expect(JSON.stringify(interception.response.body), 'plaintext not in secret response')
        .not.to.include(secretValue);
      createdHandles.push(interception.response.body?.id ?? secretHandle);
    });

    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      appPathPattern(`/organizations/${orgHandle}/settings/secrets/${secretHandle}$`)
    );
    cy.contains(secretName, { timeout: 30000 }).should('be.visible');
    cy.contains(secretHandle).should('be.visible');
    cy.get('body').invoke('text').then((text) => {
      expect(text, 'plaintext value never rendered on the overview page').not.to.include(secretValue);
    });
  });

  // -------------------------------------------------------------------------
  // TC-2: List page — pagination (>1 page of results) and search
  // -------------------------------------------------------------------------
  it('TC-2: paginates the secrets list and search filters across the whole list', () => {
    const findableName = `E2E Findme Unique ${suffix}`;
    const findableHandle = `e2e-pg-findme-${suffix}`;

    // 11 generic fixtures + 1 uniquely-named one = 12, one more than the default
    // 10-rows-per-page, so a second page always exists. Every fixture's name
    // includes `suffix` (a per-run timestamp), so searching for it isolates
    // exactly these 12 rows from anything else already in this org — the
    // pagination assertions below never depend on the org's total secret count.
    apiCreateSecret(findableHandle, findableName, 'sk-e2e-pg-findme');
    for (let i = 0; i < 11; i += 1) {
      apiCreateSecret(`e2e-pg-${suffix}-${i}`, `E2E Page Secret ${suffix} ${i}`, `sk-e2e-pg-${i}`);
    }

    cy.visitWorkspace(`/organizations/${orgHandle}/settings/secrets`);
    cy.get('input[placeholder="Search secrets..."]', { timeout: 30000 }).type(suffix);

    // Scoped to exactly these 12 fixtures: page 1 shows 10, page 2 the remaining 2.
    cy.get('table tbody tr').should('have.length', 10);

    // Pagination renders with no first/last buttons configured (see SecretsList.tsx),
    // so it exposes exactly two icon buttons: [0]=previous (disabled on page 1),
    // [1]=next. Asserting by position sidesteps depending on exact aria-label
    // wording, which the oxygen-ui wrapper may localize/override.
    cy.get('.MuiTablePagination-root button').eq(0).should('be.disabled');
    cy.get('.MuiTablePagination-root button').eq(1).should('not.be.disabled').click();

    cy.get('table tbody tr').should('have.length', 2);
    cy.get('.MuiTablePagination-root button').eq(0).should('not.be.disabled').click();
    cy.get('table tbody tr').should('have.length', 10);

    // Narrowing the search further isolates the single uniquely-named fixture —
    // proving search filters across the whole fetched list, not just whichever
    // page happens to be showing. Searching by the exact handle (unique to this
    // run) rather than the generic "Findme Unique" text avoids matching a
    // same-named fixture left behind by an earlier, unrelated run.
    cy.get('input[placeholder="Search secrets..."]').clear().type(findableHandle);
    cy.get('table tbody tr').should('have.length', 1);
    cy.contains('td', findableName).should('be.visible');

    cy.get('input[placeholder="Search secrets..."]').clear();
    cy.get('table tbody tr').should('have.length', 10);
  });

  // -------------------------------------------------------------------------
  // TC-3: Rotate — required display name, then a valid metadata + value update
  // -------------------------------------------------------------------------
  it('TC-3: rejects a blank display name client-side and persists a valid update', () => {
    const originalName = `E2E Secret Rotate ${suffix}`;
    const rotateHandle = `e2e-rotate-${suffix}`;
    const updatedName = `E2E Secret Rotated ${suffix}`;
    const updatedDescription = `Rotated by Cypress ${suffix}`;
    const updatedValue = `sk-e2e-rotated-value-${suffix}`;

    apiCreateSecret(rotateHandle, originalName, 'sk-e2e-rotate-initial');

    cy.visitWorkspace(`/organizations/${orgHandle}/settings/secrets/${rotateHandle}/rotate`);
    cy.get('[data-cyid="rotate-secret-name-input"] input:visible', { timeout: 30000 })
      .should('have.value', originalName);

    // Must be called after the page has finished loading (it attaches a
    // MutationObserver to the current document) and before the action that
    // triggers the notification — see cy.recordSnackbars' own doc comment.
    cy.recordSnackbars();

    // Blank display name -> client-side validation blocks submission entirely.
    let putCallCount = 0;
    cy.intercept('PUT', '**/secrets/**', (req) => {
      putCallCount += 1;
      req.continue();
    });
    cy.get('[data-cyid="rotate-secret-name-input"] input:visible').clear();
    cy.get('[data-cyid="rotate-secret-submit"]').click();
    cy.expectSnackbar(/Display name is required/i);
    cy.wrap(null).then(() => {
      expect(putCallCount, 'PUT /secrets must not fire for a blank display name').to.equal(0);
    });

    // A valid edit — new name, description, and value — persists.
    cy.intercept('PUT', '**/secrets/**').as('updateSecret');
    cy.get('[data-cyid="rotate-secret-name-input"] input:visible').type(updatedName);
    cy.get('input[placeholder="Reason for rotation, expiry info…"]').type(updatedDescription);
    cy.get('[data-cyid="rotate-secret-value-input"] input:visible').type(updatedValue);
    cy.get('[data-cyid="rotate-secret-submit"]').should('not.be.disabled').click();

    cy.wait('@updateSecret').then((interception) => {
      expect(interception.response.statusCode, 'PUT /secrets status').to.be.oneOf([200, 201]);
      expect(JSON.stringify(interception.response.body), 'plaintext not in update response')
        .not.to.include(updatedValue);
    });

    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      appPathPattern(`/organizations/${orgHandle}/settings/secrets/${rotateHandle}$`)
    );
    cy.contains(updatedName, { timeout: 30000 }).should('be.visible');
  });

  // -------------------------------------------------------------------------
  // TC-4a: Delete — "in use" conflict never names the handle; the secret is
  // auto-cleaned-up (permanently, not merely deprecated) once its owning
  // provider is deleted.
  // -------------------------------------------------------------------------
  it('TC-4a: the in-use conflict dialog omits the handle, and cleanup permanently deletes it once unreferenced', () => {
    const providerName = `E2E Secrets Page Provider ${suffix}`;

    cy.intercept('POST', '**/secrets').as('createProviderSecret');
    createProviderViaUI(providerName).then((id) => {
      createdProviderId = id;
    });

    // Everything below reads providerSecretHandle/createdProviderId, both of which
    // are only known once the commands above finish — nesting the rest of the test
    // inside this .then() (rather than reading them from bare `let`s at the top
    // level) is what guarantees that ordering, since cy commands are queued and
    // run later, but plain statement evaluation happens immediately.
    cy.wait('@createProviderSecret', { timeout: 20000 }).then((si) => {
      const providerSecretHandle = si.response.body?.id ?? '';
      expect(providerSecretHandle, 'captured the provider-backing secret handle').to.not.equal('');
      createdHandles.push(providerSecretHandle);

      cy.visitWorkspace(`/organizations/${orgHandle}/settings/secrets/${providerSecretHandle}`);
      cy.get('[data-cyid="delete-secret-button"]', { timeout: 30000 }).scrollIntoView().should('be.visible').click();
      cy.get('[role="dialog"]').within(() => {
        cy.contains('button', /^Delete$/).click();
      });

      // Blocked: the dialog must name the blocking provider but never leak the
      // secret's own internal handle/UUID anywhere in its content.
      cy.contains("Can't delete this secret", { timeout: 15000 }).should('be.visible');
      cy.get('[role="dialog"]').should(($dialog) => {
        const text = $dialog.text();
        expect(text, 'conflict dialog names the blocking provider').to.include(providerName);
        expect(text, 'referencing resource handle must not appear in the conflict dialog')
          .not.to.include(providerSecretHandle);
      });
      cy.get('[role="dialog"]').within(() => {
        cy.contains('button', /^Got it$/).click();
      });

      // Deleting the provider frees the secret. An auto-generated,
      // provider-backing secret is then cleaned up automatically by
      // platform-api (SecretService.CleanupOrphanedSecrets) — there is no
      // separate manual delete step for this one; a manual delete's own
      // success path is covered by TC-4b below against a plain secret instead.
      cy.request({
        method: 'DELETE',
        url: `/proxy/api/v0.9/llm-providers/${encodeURIComponent(createdProviderId)}?organizationId=${encodeURIComponent(organizationId)}`,
        headers: { Authorization: `Bearer ${authToken}` },
        failOnStatusCode: false,
      }).then(() => {
        createdProviderId = '';
      });

      // Permanently gone (404), not merely flipped to deprecated — see
      // 003-llm-proxy-secret-management.cy.js TC-4 for the same assertion
      // against the hard-delete-on-cleanup behavior.
      pollUntilSecretGone(providerSecretHandle);
    });
  });

  // -------------------------------------------------------------------------
  // TC-4b: Delete — a plain, unreferenced secret: confirming deletes it and
  // redirects to the list.
  // -------------------------------------------------------------------------
  it('TC-4b: deleting an unreferenced secret succeeds and redirects to the list', () => {
    const plainHandle = `e2e-delete-${suffix}`;

    apiCreateSecret(plainHandle, `E2E Secret Delete ${suffix}`, 'sk-e2e-delete-me');

    cy.visitWorkspace(`/organizations/${orgHandle}/settings/secrets/${plainHandle}`);
    cy.get('[data-cyid="delete-secret-button"]', { timeout: 30000 }).scrollIntoView().should('be.visible').click();
    cy.get('[role="dialog"]').within(() => {
      cy.contains('button', /^Delete$/).click();
    });

    cy.location('pathname', { timeout: 30000 }).should(
      'match',
      appPathPattern(`/organizations/${orgHandle}/settings/secrets$`)
    );
    cy.request({
      url: `/proxy/api/v0.9/secrets/${encodeURIComponent(plainHandle)}?organizationId=${encodeURIComponent(organizationId)}`,
      headers: { Authorization: `Bearer ${authToken}` },
      failOnStatusCode: false,
    }).then((r) => {
      expect(r.status, 'secret permanently deleted').to.eq(404);
    });
  });
});
