// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// ---------------------------------------------------------------------------
// cy.login(username, password)
//   Log in via the local auth form and wait for the portal home to reload.
//   Defaults to the admin credentials from the users fixture.
// ---------------------------------------------------------------------------
Cypress.Commands.add('login', (username, password) => {
    const user = username || Cypress.env('ADMIN_USER');
    const pwd  = password || Cypress.env('ADMIN_PASSWORD');

    cy.visitPortal();
    // Idempotent: if a session is already active (e.g. carried over from a
    // before() hook into the first test), the home page shows the profile link
    // rather than the login button — skip the form in that case.
    cy.get('body').then(($body) => {
        if ($body.find('.profile-link').length > 0) {
            return;
        }
        cy.get('.login-btn').click();
        cy.get('#username').type(user);
        cy.get('#password').type(pwd);
        cy.get('.ln-signin-btn').click();
        // Wait until redirected back to the portal home and profile link is visible.
        cy.get('.profile-link', { timeout: 15000 }).should('be.visible');
    });
});

// ---------------------------------------------------------------------------
// cy.completeLoginForm(username, password)
//   Fill in and submit the local login form on the CURRENT page. Unlike
//   cy.login (which starts from the portal home), this assumes the browser is
//   already sitting on the login page — e.g. after an auth-gated page redirected
//   an anonymous visitor there — so the server-side `returnTo` set by that
//   redirect survives and sends the user back to the originally requested page.
// ---------------------------------------------------------------------------
Cypress.Commands.add('completeLoginForm', (username, password) => {
    const user = username || Cypress.env('ADMIN_USER');
    const pwd  = password || Cypress.env('ADMIN_PASSWORD');

    cy.get('#local-login-form').should('be.visible');
    cy.get('#username').type(user);
    cy.get('#password').type(pwd);
    cy.get('.ln-signin-btn').click();
});

// ---------------------------------------------------------------------------
// cy.logout()
//   Log out by navigating to the logout endpoint.
// ---------------------------------------------------------------------------
Cypress.Commands.add('logout', () => {
    const basePath = Cypress.env('BASE_PATH');
    const orgHandle = Cypress.env('ORG_HANDLE');
    cy.visit(`${basePath}/${orgHandle}/logout`);
});
