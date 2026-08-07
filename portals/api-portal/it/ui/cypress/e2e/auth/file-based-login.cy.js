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

// UI happy-path for demo-mode login. The REST contract for this flow (does the
// session cookie work, is a bad password rejected) is covered in
// backend/auth/file-based-login.spec.js — this spec is for the actual form UX.

describe('file-based login UI', () => {

    it('logs in through the local login form, shows the profile, and logs back out', () => {
        // Home → Log In → admin/admin → submit → lands back on the portal home
        // with the profile link visible (cy.login asserts .profile-link visible).
        cy.login();

        // Redirected to the portal home, with the username shown top-right.
        cy.url().should('include', '/views/default');
        cy.get('.profile-link').should('be.visible').and('contain', Cypress.env('ADMIN_USER'));

        // Open the profile dropdown and log out.
        cy.get('.profile-link').click();
        cy.get('.profile-dropdown-link').should('be.visible').click();

        // Redirected back to the login page.
        cy.url().should('include', '/login');
        cy.get('#local-login-form').should('be.visible');
    });

    it('shows an error for an incorrect password', () => {
        cy.visitPortal();
        cy.get('.login-btn').click();
        cy.get('#username').type(Cypress.env('ADMIN_USER'));
        cy.get('#password').type('wrong-password');
        cy.get('.ln-signin-btn').click();

        // Login page re-renders with the error banner; no session established.
        cy.get('.ln-error-banner')
            .should('be.visible')
            .and('contain', 'Invalid username or password');
        cy.url().should('include', '/login');
    });
});
