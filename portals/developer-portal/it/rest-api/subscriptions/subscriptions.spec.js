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

// POST /subscriptions, GET/PUT/DELETE /subscriptions/{subId},
// POST /subscriptions/{subId}/change-plan, POST /subscriptions/{subId}/regenerate-token.
// The shared org gets default plans (Gold/Silver/Bronze/Unlimited/AsyncUnlimited)
// via DP_GENERATEDEFAULTSUBPLANS=true (docker-compose.test*.yaml) — an API must
// still link a plan by name via its `subscriptionPlans` field before subscribing
// to it. `publisher` creates the API; `developer` (the consumer) subscribes to it.

const client = require('../support/client');
const { createApi } = require('../support/fixtures');

describe('subscriptions', () => {
    let api;

    beforeAll(async () => {
        await client.login('publisher');
        await client.login('developer');
    });

    beforeEach(async () => {
        api = await createApi({ subscriptionPlans: [{ id: 'Gold' }, { id: 'Silver' }] });
    });

    it('creates a subscription for an application to an API plan', async () => {
        const res = await client.as('developer').post('/subscriptions', { artifactId: api.id, subscriptionPlanId: 'Gold' });
        expect(res.status).toBe(201);
        expect(res.body.artifactId).toBe(api.id);
        expect(res.body.subscriptionPlanName).toBe('Gold');
        expect(res.body.status).toBe('ACTIVE');
        expect(res.body.subscriptionToken).toBeDefined();
    });

    it('retrieves a subscription', async () => {
        const create = await client.as('developer').post('/subscriptions', { artifactId: api.id, subscriptionPlanId: 'Gold' });
        const res = await client.as('developer').get(`/subscriptions/${create.body.subscriptionId}`);
        expect(res.status).toBe(200);
        expect(res.body.artifactId).toBe(api.id);
    });

    it('changes a subscription plan', async () => {
        const create = await client.as('developer').post('/subscriptions', { artifactId: api.id, subscriptionPlanId: 'Gold' });
        const res = await client.as('developer').post(`/subscriptions/${create.body.subscriptionId}/change-plan`, { planId: 'Silver' });
        expect(res.status).toBe(200);
        expect(res.body.subscriptionPlanName).toBe('Silver');
        // Same subscription uuid/token, only the plan changed.
        expect(res.body.subscriptionToken).toBe(create.body.subscriptionToken);
    });

    it('regenerates a subscription token', async () => {
        const create = await client.as('developer').post('/subscriptions', { artifactId: api.id, subscriptionPlanId: 'Gold' });
        const res = await client.as('developer').post(`/subscriptions/${create.body.subscriptionId}/regenerate-token`, {});
        expect(res.status).toBe(200);
        expect(res.body.subscriptionToken).toBeDefined();
        expect(res.body.subscriptionToken).not.toBe(create.body.subscriptionToken);
    });

    it('deletes a subscription', async () => {
        const create = await client.as('developer').post('/subscriptions', { artifactId: api.id, subscriptionPlanId: 'Gold' });
        const del = await client.as('developer').del(`/subscriptions/${create.body.subscriptionId}`);
        expect(del.status).toBe(200);

        const get = await client.as('developer').get(`/subscriptions/${create.body.subscriptionId}`);
        expect(get.status).toBe(404);
    });

    it('rejects subscribing with a plan not linked to the API', async () => {
        const res = await client.as('developer').post('/subscriptions', { artifactId: api.id, subscriptionPlanId: 'Bronze' });
        expect(res.status).toBe(400);
    });

    it('rejects subscribing to a non-existent API', async () => {
        const res = await client.as('developer').post('/subscriptions', { artifactId: 'does-not-exist', subscriptionPlanId: 'Gold' });
        expect(res.status).toBe(404);
    });

    it('rejects change-plan to a plan that does not exist in the org', async () => {
        const create = await client.as('developer').post('/subscriptions', { artifactId: api.id, subscriptionPlanId: 'Gold' });
        const res = await client.as('developer').post(`/subscriptions/${create.body.subscriptionId}/change-plan`, { planId: 'DoesNotExist' });
        expect(res.status).toBe(400);
    });
});
