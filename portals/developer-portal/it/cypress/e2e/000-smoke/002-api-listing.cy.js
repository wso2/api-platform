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

describe('Developer Portal — API Listing', () => {
    beforeEach(() => {
        cy.on('uncaught:exception', () => false);
    });

    context('REST API', () => {
        it('GET /devportal/organizations returns 200 with an array', () => {
            cy.apiRequest('GET', '/devportal/organizations').then((resp) => {
                expect(resp.status).to.eq(200);
                expect(resp.body).to.be.an('array');
            });
        });

        it('GET /devportal/organizations/:orgId/apis returns 200 with an array', () => {
            const orgId = Cypress.env('ORG_ID');
            cy.apiRequest('GET', `/devportal/organizations/${orgId}/apis`).then((resp) => {
                expect(resp.status).to.eq(200);
                expect(resp.body).to.be.an('array');
            });
        });

        it('GET /devportal/organizations/:orgId/views returns the default view', () => {
            const orgId = Cypress.env('ORG_ID');
            const viewName = Cypress.env('VIEW_NAME');
            cy.apiRequest('GET', `/devportal/organizations/${orgId}/views`).then((resp) => {
                expect(resp.status).to.eq(200);
                expect(resp.body).to.be.an('array');
                const names = resp.body.map((v) => v.name || v.NAME);
                expect(names).to.include(viewName);
            });
        });
    });

    context('UI — API browse page', () => {
        it('loads the API list page without errors', () => {
            cy.visitPortal('/apis');
            cy.get('body').should('be.visible');
            cy.get('body').should('not.contain.text', 'Cannot GET');
            cy.get('body').should('not.contain.text', '500');
        });
    });
});
