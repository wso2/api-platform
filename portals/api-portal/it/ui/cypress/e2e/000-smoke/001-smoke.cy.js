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

describe('API Portal — Smoke', () => {
    beforeEach(() => {
        // Accept self-signed cert and suppress uncaught app exceptions that
        // are unrelated to the assertions being tested.
        cy.on('uncaught:exception', () => false);
    });

    it('root redirects to the default organization view', () => {
        const orgHandle = Cypress.env('ORG_HANDLE');
        const viewName = Cypress.env('VIEW_NAME');
        const basePath = Cypress.env('BASE_PATH');
        // The true root (/) redirects into the portal mount (${BASE_PATH}/) as a
        // convenience for direct container access; the portal root then redirects
        // straight into the default organization's view.
        cy.request({ url: '/', followRedirect: false, failOnStatusCode: false }).then((resp) => {
            expect(resp.status).to.eq(302);
            expect(resp.redirectedToUrl).to.contain(basePath);
        });
        // The portal mount root redirects on into the default view.
        cy.request({ url: basePath, followRedirect: false, failOnStatusCode: false }).then((resp) => {
            expect(resp.status).to.eq(302);
            expect(resp.redirectedToUrl).to.contain(`${basePath}/${orgHandle}/views/${viewName}`);
        });
        // Following all redirects from / lands on a rendered page, not an error.
        cy.request({ url: '/', failOnStatusCode: false }).then((resp) => {
            expect(resp.status).to.eq(200);
        });
    });

    it('Default org view loads and shows a page', () => {
        cy.visitPortal();
        cy.get('body').should('be.visible');
        // Should not show a 404/500 error page.
        cy.get('body').should('not.contain.text', '500');
        cy.get('body').should('not.contain.text', 'Cannot GET');
    });

    it('health endpoint returns 200', () => {
        cy.request({
            url: '/health',
            failOnStatusCode: false,
        }).then((resp) => {
            expect(resp.status).to.eq(200);
            expect(resp.body).to.have.property('status', 'ok');
        });
    });

    it('serves (or correctly 404s) the main CSS asset', () => {
        // Addressed by its public URL rather than the organization's internal UUID:
        // the portal serves one organization and never exposes that UUID over the API.
        cy.request({
            url: `${Cypress.env('BASE_PATH')}/styles/main.css`,
            failOnStatusCode: false,
        }).then((resp) => {
            expect(resp.status).to.be.oneOf([200, 304, 404]);
        });
    });

    it('404s a portal view under another organization handle', () => {
        // The database is shared across organizations, so the handle in the URL is
        // untrusted input — this instance serves exactly one and rejects the rest.
        cy.request({
            url: `${Cypress.env('BASE_PATH')}/some-other-org/views/default`,
            failOnStatusCode: false,
        }).then((resp) => {
            expect(resp.status).to.eq(404);
        });
    });
});
