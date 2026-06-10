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

describe('Developer Portal — Smoke', () => {
    beforeEach(() => {
        // Accept self-signed cert and suppress uncaught app exceptions that
        // are unrelated to the assertions being tested.
        cy.on('uncaught:exception', () => false);
    });

    it('root serves a landing page', () => {
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

    it('serves (or correctly 404s) the main CSS asset for the default view', () => {
        cy.fixture('org').then(({ orgId }) => {
            cy.request({
                url: `/devportal/organizations/${orgId}/views/default/layout?fileType=style&fileName=main.css`,
                failOnStatusCode: false,
            }).then((resp) => {
                expect(resp.status).to.be.oneOf([200, 304, 404]);
            });
        });
    });
});
