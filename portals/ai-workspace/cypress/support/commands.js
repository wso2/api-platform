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
