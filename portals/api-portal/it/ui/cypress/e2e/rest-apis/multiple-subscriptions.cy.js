/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// A consumer can hold several subscriptions to one API: the same plan twice and a second
// plan, each listed in "Your subscriptions" and managed independently.

describe('REST API — multiple subscriptions', () => {
    let apiHandle;

    const subscribeTo = (plan) => {
        cy.get(`#subscribe-btn-${plan}`).scrollIntoView().should('be.visible').and('not.be.disabled').click();
        // Closing the token modal reloads the page with the new subscription listed.
        cy.get('#apiTokenModal').should('be.visible');
        cy.get('#apiTokenModalDone').click();
    };

    before(() => {
        cy.login();
        cy.seedApi({
            name: 'IT Multi Subscription API',
            subscriptionPlans: [{ id: 'Gold' }, { id: 'Silver' }],
        }).then((handle) => { apiHandle = handle; });
    });

    after(() => {
        cy.login();
        cy.deleteApi(apiHandle);
    });

    it('subscribes twice to one plan and once to another, then manages each independently', () => {
        cy.login('developer', 'developer');
        cy.visitPortal(`/api/${apiHandle}`);
        cy.get('#apiSubscriptionsTable').should('not.exist');

        subscribeTo('Gold');
        // The plan stays subscribable after the first subscription — no "Switch plan".
        cy.get('#subscribe-btn-Gold').should('contain.text', 'Subscribe');
        subscribeTo('Gold');
        subscribeTo('Silver');

        cy.get('#apiSubscriptionsTable tbody tr').should('have.length', 3);
        cy.get('#apiSubscriptionsTable tbody tr').filter(':contains("Gold")').should('have.length', 2);
        cy.get('#apiSubscriptionsTable tbody tr').filter(':contains("Silver")').should('have.length', 1);
        cy.get('#subscriptionCard-Gold .api-ribbon-label').should('contain.text', '×2');

        // Suspend the first Gold subscription: only its row changes, and the Gold card
        // is not "suspended" because the other Gold subscription is still active.
        cy.get('#apiSubscriptionsTable tbody tr').first().as('suspendedRow')
            .find('.api-view-sub-btn').click();
        cy.get('#apiManageModal').should('be.visible');
        cy.get('#apiManageSuspendBtn').click();
        cy.get('#apiManageStatusBadge').should('contain.text', 'Suspended');
        cy.get('@suspendedRow').find('.api-sub-status').should('contain.text', 'Suspended');
        cy.get('#apiSubscriptionsTable tbody tr').not(':first').each(($row) => {
            cy.wrap($row).find('.api-sub-status').should('contain.text', 'Active');
        });
        cy.get('#subscriptionCard-Gold').should('not.have.class', 'aov-plan-card--suspended');
    });
});
