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

// Follows ../applications/webhook-events.spec.js for the full working pattern.
// VALID_EVENT_TYPES for this resource (src/services/webhooks/eventPublisher.js):
// subscription.created, subscription.updated, subscription.deleted,
// subscription.plan_changed, subscription.token_regenerated.

const crypto = require('crypto');
const client = require('../support/client');
const db = require('../support/db');
const { waitForEvent, waitForDelivery, poll } = require('../support/wait-for');
const { createApi, createWebhookSubscriber } = require('../support/fixtures');
const { createWebhookSink, resolveSinkUrl } = require('../support/webhook-sink');
const { decryptFromEnvelope } = require('../support/envelopeCrypto');

describe('subscriptions webhook events', () => {
    let api;
    let sink;
    let subscriber;
    const sinkUrl = resolveSinkUrl(4501);

    beforeAll(async () => {
        await client.login('publisher');
        await client.login('developer');
        await client.login('admin');
        sink = createWebhookSink();
        await sink.start(Number(sinkUrl.port));
    });

    afterAll(async () => {
        await sink.stop();
        await db.close();
    });

    beforeEach(async () => {
        api = await createApi({ subscriptionPlans: [{ id: 'Gold' }, { id: 'Silver' }] });
        subscriber = await createWebhookSubscriber({
            targetUrl: sinkUrl.href,
            events: ['subscription.*'],
        });
    });

    afterEach(async () => {
        await client.as('admin').del(`/webhook-subscribers/${subscriber.id}`);
        sink.received.length = 0;
    });

    it('publishes and delivers subscription.created', async () => {
        const since = new Date();
        const create = await client.as('developer').post('/subscriptions', { apiId: api.id, subscriptionPlanId: 'Gold' });
        expect(create.status).toBe(201);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'subscription.created', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('subscription.created');
        expect(received).toBeDefined();
        // subscriber_id is the developer's IdP subject, not the portal-internal
        // user uuid — file-based auth's `sub` claim for the `developer` account.
        expect(received.body.data).toEqual({
            subscription_id: create.body.subscriptionId,
            subscriber_id: expect.any(String),
            status: create.body.status,
            subscription_plan: { ref_id: null, name: 'Gold' },
            api: { name: api.name, version: api.version, ref_id: api.refId || '', type: api.type },
        });
    });

    it('publishes and delivers subscription.plan_changed', async () => {
        const create = await client.as('developer').post('/subscriptions', { apiId: api.id, subscriptionPlanId: 'Gold' });

        const since = new Date();
        const change = await client.as('developer').post(`/subscriptions/${create.body.subscriptionId}/change-plan`, { planId: 'Silver' });
        expect(change.status).toBe(200);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'subscription.plan_changed', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('subscription.plan_changed');
        expect(received).toBeDefined();
        expect(received.body.data).toEqual({
            subscription_id: create.body.subscriptionId,
            subscriber_id: expect.any(String),
            status: create.body.status,
            subscription_plan: { ref_id: null, name: 'Silver' },
            api: { name: api.name, version: api.version, ref_id: api.refId || '', type: api.type },
            previous_plan: { ref_id: null, name: 'Gold' },
        });
    });

    it('publishes and delivers subscription.token_regenerated', async () => {
        const create = await client.as('developer').post('/subscriptions', { apiId: api.id, subscriptionPlanId: 'Gold' });

        const since = new Date();
        const regen = await client.as('developer').post(`/subscriptions/${create.body.subscriptionId}/regenerate-token`, {});
        expect(regen.status).toBe(200);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'subscription.token_regenerated', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('subscription.token_regenerated');
        expect(received).toBeDefined();
        // `token` is passed as a secretField (subscriptionService.regenerateToken) —
        // this subscriber has no publicKey, so it's delivered without any encrypted
        // fields and the plaintext token never appears in `data`.
        expect(received.body.encrypted_fields).toEqual([]);
        expect(received.body.data).toEqual({
            subscription_id: create.body.subscriptionId,
            subscriber_id: expect.any(String),
            status: create.body.status,
            subscription_plan: { ref_id: null, name: 'Gold' },
            api: { name: api.name, version: api.version, ref_id: api.refId || '', type: api.type },
        });
        expect(received.body.data.token).toBeUndefined();
    });

    it('publishes and delivers subscription.deleted', async () => {
        const create = await client.as('developer').post('/subscriptions', { apiId: api.id, subscriptionPlanId: 'Gold' });

        const since = new Date();
        const del = await client.as('developer').del(`/subscriptions/${create.body.subscriptionId}`);
        expect(del.status).toBe(200);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'subscription.deleted', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('subscription.deleted');
        expect(received).toBeDefined();
        expect(received.body.data).toEqual({
            subscription_id: create.body.subscriptionId,
            subscriber_id: expect.any(String),
            status: create.body.status,
            subscription_plan: { ref_id: null, name: 'Gold' },
            api: { name: api.name, version: api.version, ref_id: api.refId || '', type: api.type },
        });
    });

    it('encrypts secret fields to the subscriber public key when configured', async () => {
        const { publicKey, privateKey } = crypto.generateKeyPairSync('rsa', {
            modulusLength: 2048,
            publicKeyEncoding: { type: 'spki', format: 'pem' },
            privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
        });
        const encryptedSubscriber = await createWebhookSubscriber({
            targetUrl: sinkUrl.href,
            events: ['subscription.*'],
            publicKey,
        });

        const create = await client.as('developer').post('/subscriptions', { apiId: api.id, subscriptionPlanId: 'Gold' });

        const since = new Date();
        const regen = await client.as('developer').post(`/subscriptions/${create.body.subscriptionId}/regenerate-token`, {});
        expect(regen.status).toBe(200);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'subscription.token_regenerated', since });

        // Two subscribers now match subscription.* (the outer describe's default
        // `subscriber` plus this one) — poll the sink directly for the specific
        // delivery that has encrypted_fields, rather than racing on which
        // subscriber's DB delivery row lands first.
        const received = await poll(() => sink.received.find((r) => r.body?.event_id === event.uuid && r.body?.encrypted_fields?.length));
        expect(received).toBeDefined();
        expect(received.body.encrypted_fields).toEqual(['token']);
        expect(received.body.data.token).toEqual({
            wrappedKey: expect.any(String),
            iv: expect.any(String),
            tag: expect.any(String),
            ciphertext: expect.any(String),
        });

        const decrypted = decryptFromEnvelope(privateKey, received.body.data.token);
        expect(decrypted).toBe(regen.body.subscriptionToken);

        await client.as('admin').del(`/webhook-subscribers/${encryptedSubscriber.id}`);
    });

    // Closes a gap the tests above don't cover on their own: response-correctness
    // (subscriptions.spec.js) and event-correctness (this file) are asserted in
    // separate suites, so a bug that gets one right and the other wrong on the
    // same call would pass both. Assert the combination directly.
    describe('action + event consistency', () => {
        it('does not publish subscription.plan_changed when change-plan targets a plan not linked to the API', async () => {
            const create = await client.as('developer').post('/subscriptions', { apiId: api.id, subscriptionPlanId: 'Gold' });

            const since = new Date();
            const res = await client.as('developer').post(`/subscriptions/${create.body.subscriptionId}/change-plan`, { planId: 'DoesNotExist' });
            expect(res.status).toBe(400);

            const events = await db.findEvents({ orgUuid: await db.findOrgUuidByHandle(client.ORG_HANDLE), type: 'subscription.plan_changed', since });
            expect(events).toHaveLength(0);
        });
    });
});
