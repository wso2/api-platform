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

// Cross-cutting delivery-pipeline behavior, as opposed to any one resource's
// event payload. Follows ../applications/webhook-events.spec.js for the base
// sink/subscriber pattern.

const crypto = require('crypto');
const client = require('../support/client');
const db = require('../support/db');
const { waitForEvent, waitForDelivery, sleep } = require('../support/wait-for');
const { createWebhookSubscriber, uniqueHandle } = require('../support/fixtures');
const { createWebhookSink, resolveSinkUrl } = require('../support/webhook-sink');

describe('webhook delivery pipeline', () => {
    let sink;
    let subscriber;
    const sinkUrl = resolveSinkUrl(4503);

    beforeAll(async () => {
        await client.login('admin');
        await client.login('developer');
        sink = createWebhookSink();
        await sink.start(Number(sinkUrl.port));
    });

    afterAll(async () => {
        await sink.stop();
        await db.close();
    });

    afterEach(async () => {
        if (subscriber) {
            await client.as('admin').del(`/webhook-subscribers/${subscriber.id}`);
            subscriber = undefined;
        }
        sink.received.length = 0;
    });

    it('does not deliver to a subscriber whose event pattern does not match', async () => {
        subscriber = await createWebhookSubscriber({ targetUrl: sinkUrl.href, events: ['apikey.*'] });

        const since = new Date();
        await client.as('developer').post('/applications', { id: uniqueHandle('app'), displayName: 'Pattern Test', description: 'd' });

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.created', since });
        expect(event).toBeDefined();

        // Give the dispatcher a full poll cycle to prove it deliberately skips
        // this subscriber, rather than just not having gotten to it yet.
        await sleep(2500);
        const deliveries = await db.findDeliveries({ eventUuid: event.uuid });
        expect(deliveries).toHaveLength(0);
    });

    it('signs the payload with HMAC when the subscriber has a secret', async () => {
        const secret = 'test-signing-secret';
        subscriber = await createWebhookSubscriber({ targetUrl: sinkUrl.href, events: ['application.*'], secret });

        const since = new Date();
        await client.as('developer').post('/applications', { id: uniqueHandle('app'), displayName: 'Signed', description: 'd' });

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.created', since });
        await waitForDelivery({ eventUuid: event.uuid });

        const received = sink.findDeliveryFor('application.created');
        expect(received).toBeDefined();
        const sigHeader = received.headers['x-devportal-signature'];
        expect(sigHeader).toBeDefined();

        // Recompute per signer.js: HMAC-SHA256("<t>.<raw_body>", secret), hex.
        const [tPart, v1Part] = sigHeader.split(',');
        const ts = tPart.split('=')[1];
        const expectedHmac = crypto.createHmac('sha256', secret).update(`${ts}.${received.rawBody}`).digest('hex');
        expect(v1Part).toBe(`v1=${expectedHmac}`);
    });

    it('marks a delivery FAILED when the target responds with a non-2xx status', async () => {
        subscriber = await createWebhookSubscriber({ targetUrl: sinkUrl.href, events: ['application.*'] });
        sink.respondWith(500);

        const since = new Date();
        await client.as('developer').post('/applications', { id: uniqueHandle('app'), displayName: 'Failing', description: 'd' });

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.created', since });
        const delivery = await waitForDelivery({ eventUuid: event.uuid });

        // No retry scheduling for ordinary non-2xx responses — a single failed
        // attempt is terminal (src/dao/eventDao.js markFailed: "Single attempt —
        // no retry scheduling"). last_http_status/last_error record the failure.
        expect(delivery.status).toBe('FAILED');
        expect(delivery.last_http_status).toBe(500);
    });

    it('does not attempt delivery to a disabled subscriber', async () => {
        subscriber = await createWebhookSubscriber({ targetUrl: sinkUrl.href, events: ['application.*'], enabled: false });

        const since = new Date();
        await client.as('developer').post('/applications', { id: uniqueHandle('app'), displayName: 'Disabled Sub', description: 'd' });

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.created', since });
        await sleep(2500);
        const deliveries = await db.findDeliveries({ eventUuid: event.uuid });
        expect(deliveries).toHaveLength(0);
    });

    it('marks the source event ALL_DELIVERED once every matching subscriber has a terminal delivery status', async () => {
        subscriber = await createWebhookSubscriber({ targetUrl: sinkUrl.href, events: ['application.*'] });

        const since = new Date();
        await client.as('developer').post('/applications', { id: uniqueHandle('app'), displayName: 'All Delivered', description: 'd' });

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.created', since });
        await waitForDelivery({ eventUuid: event.uuid });

        const [finalEvent] = await db.findEvents({ orgUuid: (await db.findOrgUuidByHandle(client.ORG_HANDLE)), type: 'application.created' });
        expect(finalEvent.status).toBe('ALL_DELIVERED');
    });
});
