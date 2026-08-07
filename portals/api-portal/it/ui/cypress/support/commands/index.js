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

// Barrel for all custom Cypress commands. support/e2e.js imports this once
// (`import './commands'`), so every module below registers its commands before
// any spec runs. Add a new command module here to make it available globally.
import './portal';       // cy.portalUrl, cy.apiRequest, cy.visitPortal
import './auth';         // cy.login, cy.completeLoginForm, cy.logout
import './seed';         // cy.seedApi, cy.seedMcp, cy.deleteApi, cy.deleteMcp, cy.seedKeyManager, cy.deleteKeyManager
import './applications'; // cy.createApplication, cy.deleteApplication
