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
// apikey.generated, apikey.regenerated, apikey.revoked, apikey.application_updated.

const crypto = require('crypto');
const client = require('../support/client');
const db = require('../support/db');
const { waitForEvent, waitForDelivery, poll } = require('../support/wait-for');
const { createApi, createWebhookSubscriber, uniqueHandle } = require('../support/fixtures');
const { createWebhookSink, resolveSinkUrl } = require('../support/webhook-sink');
const { decryptFromEnvelope } = require('../support/envelopeCrypto');

describe('api-keys webhook events', () => {
    let api;
    let sink;
    let subscriber;
    const sinkUrl = resolveSinkUrl(4502);

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
        api = await createApi();
        subscriber = await createWebhookSubscriber({
            targetUrl: sinkUrl.href,
            events: ['apikey.*'],
        });
    });

    afterEach(async () => {
        await client.as('admin').del(`/webhook-subscribers/${subscriber.id}`);
        sink.received.length = 0;
    });

    it('publishes and delivers apikey.generated', async () => {
        const since = new Date();
        const keyId = uniqueHandle('key').toLowerCase();
        const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });
        expect(res.status).toBe(201);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'apikey.generated', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('apikey.generated');
        expect(received).toBeDefined();
        // `key` is passed as a secretField (see apiKeyService.generate) — this
        // subscriber has no publicKey, so it's delivered without any encrypted
        // fields rather than the plaintext ever appearing in `data`.
        expect(received.body.encrypted_fields).toEqual([]);
        expect(received.body.data).toEqual({
            key_id: res.body.keyId,
            handle: keyId,
            display_name: keyId,
            expires_at: null,
            api: { name: api.name, version: api.version, ref_id: api.refId || '', type: api.type },
        });
        expect(received.body.data.key).toBeUndefined();
    });

    it('publishes and delivers apikey.regenerated', async () => {
        const keyId = uniqueHandle('key').toLowerCase();
        const generated = await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });

        const since = new Date();
        const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/regenerate`, { keyId });
        expect(res.status).toBe(200);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'apikey.regenerated', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('apikey.regenerated');
        expect(received).toBeDefined();
        expect(received.body.encrypted_fields).toEqual([]);
        expect(received.body.data).toEqual({
            key_id: generated.body.keyId,
            handle: keyId,
            display_name: keyId,
            expires_at: null,
            api: { name: api.name, version: api.version, ref_id: api.refId || '', type: api.type },
        });
        expect(received.body.data.key).toBeUndefined();
    });

    it('publishes and delivers apikey.revoked', async () => {
        const keyId = uniqueHandle('key').toLowerCase();
        const generated = await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });

        const since = new Date();
        const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/revoke`, { keyId });
        expect(res.status).toBe(204);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'apikey.revoked', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('apikey.revoked');
        expect(received).toBeDefined();
        expect(received.body.data).toEqual({
            key_id: generated.body.keyId,
            handle: keyId,
            display_name: keyId,
            api: { name: api.name, version: api.version, ref_id: api.refId || '', type: api.type },
        });
    });

    it('publishes and delivers apikey.application_updated on associate/dissociate', async () => {
        // See api-keys.spec.js's note on associate/dissociate: the target app is
        // resolved by the caller's own created_by, so the same role must both
        // own the app and call associate.
        const keyId = uniqueHandle('key').toLowerCase();
        const appId = uniqueHandle('app');
        await client.as('developer').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });
        await client.as('developer').post('/applications', { id: appId, displayName: 'Assoc App', description: 'd' });

        const since = new Date();
        const assoc = await client.as('developer').post(`/apis/${api.id}/api-keys/associate`, { keyId, appId });
        expect(assoc.status).toBe(200);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'apikey.application_updated', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('apikey.application_updated');
        expect(received).toBeDefined();
        // `application.id` here is the app's internal uuid (apiKeyService.resolveApp),
        // never exposed over REST — only handle/display_name are checkable directly.
        expect(received.body.data).toEqual({
            key_id: expect.any(String),
            handle: keyId,
            display_name: keyId,
            api: { name: api.name, version: api.version, ref_id: api.refId || '', type: api.type },
            application: { id: expect.any(String), display_name: 'Assoc App', handle: appId },
        });
    });

    it('encrypts the key value field to the subscriber public key when configured', async () => {
        const { publicKey, privateKey } = crypto.generateKeyPairSync('rsa', {
            modulusLength: 2048,
            publicKeyEncoding: { type: 'spki', format: 'pem' },
            privateKeyEncoding: { type: 'pkcs8', format: 'pem' },
        });
        const encryptedSubscriber = await createWebhookSubscriber({
            targetUrl: sinkUrl.href,
            events: ['apikey.*'],
            publicKey,
        });

        const keyId = uniqueHandle('key').toLowerCase();
        const since = new Date();
        const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });
        expect(res.status).toBe(201);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'apikey.generated', since });

        // Two subscribers now match apikey.* (the outer describe's default `subscriber`
        // plus this one), so waitForDelivery's single-match-by-eventUuid could resolve to
        // either one's delivery row — poll the sink directly for the specific delivery
        // that has encrypted_fields, rather than racing on which subscriber's row lands
        // first in the DB.
        const received = await poll(() => sink.received.find((r) => r.body?.event_id === event.uuid && r.body?.encrypted_fields?.length));
        expect(received).toBeDefined();
        expect(received.body.encrypted_fields).toEqual(['key']);
        expect(received.body.data.key).toEqual({
            wrappedKey: expect.any(String),
            iv: expect.any(String),
            tag: expect.any(String),
            ciphertext: expect.any(String),
        });

        const decrypted = decryptFromEnvelope(privateKey, received.body.data.key);
        expect(decrypted).toBe(res.body.key);

        await client.as('admin').del(`/webhook-subscribers/${encryptedSubscriber.id}`);
    });

    // These close a gap the tests above don't cover: response-correctness and
    // event-correctness are asserted in separate suites, but a bug could make
    // one right and the other wrong on the *same* call (e.g. regenerate.js's
    // 409 guard at apiKeyService.js:210 firing after publish() instead of
    // before it). Assert the combination directly instead of trusting that
    // each half being independently correct implies the pair is.
    describe('action + event consistency', () => {
        it('does not publish apikey.regenerated when regenerate is rejected as already revoked', async () => {
            const keyId = uniqueHandle('key').toLowerCase();
            await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });
            await client.as('publisher').post(`/apis/${api.id}/api-keys/revoke`, { keyId });

            const since = new Date();
            const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/regenerate`, { keyId });
            expect(res.status).toBe(409);

            const events = await db.findEvents({ orgUuid: await db.findOrgUuidByHandle(client.ORG_HANDLE), type: 'apikey.regenerated', since });
            expect(events).toHaveLength(0);
            expect(sink.findDeliveryFor('apikey.regenerated')).toBeUndefined();
        });

        it('dissociate clears the application from both the list response and apikey.application_updated', async () => {
            const keyId = uniqueHandle('key').toLowerCase();
            const appId = uniqueHandle('app');
            await client.as('developer').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });
            await client.as('developer').post('/applications', { id: appId, displayName: 'Dissoc App', description: 'd' });
            await client.as('developer').post(`/apis/${api.id}/api-keys/associate`, { keyId, appId });

            const since = new Date();
            const res = await client.as('developer').post(`/apis/${api.id}/api-keys/dissociate`, { keyId });
            expect(res.status).toBe(204);

            const list = await client.as('developer').get(`/apis/${api.id}/api-keys`);
            const key = list.body.list.find((k) => k.id === keyId);
            expect(key.appId).toBeNull();

            const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'apikey.application_updated', since });
            expect(event).toBeDefined();
            const delivery = await waitForDelivery({ eventUuid: event.uuid });
            expect(delivery.status).toBe('DELIVERED');
            // Both associate and dissociate fire apikey.application_updated, so
            // sink.findDeliveryFor (first-match-by-type) would return associate's
            // delivery here — match on event_id instead to get the dissociate one.
            const received = sink.received.find((r) => r.body?.event_id === event.uuid);
            expect(received).toBeDefined();
            expect(received.body.data.application).toBeNull();
        });
    });
});
