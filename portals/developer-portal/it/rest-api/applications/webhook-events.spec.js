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

// Reference spec for the webhook pipeline end-to-end: register a subscriber
// pointed at a local sink -> trigger application.created via the real API ->
// assert the dp_events row -> assert the actual HTTP delivery payload/headers.
// Every other `*/webhook-events.spec.js` file should follow this shape.
//
// Every test in this file shares the one fixed org (client.ORG_HANDLE) — each
// registers its own webhook subscriber and deletes it in afterEach, since
// subscribers (unlike applications) would otherwise accumulate and start
// double-delivering events to earlier tests' leftover subscribers.

const client = require('../support/client');
const db = require('../support/db');
const { waitForEvent, waitForDelivery } = require('../support/wait-for');
const { createWebhookSubscriber, uniqueHandle } = require('../support/fixtures');
const { createWebhookSink, resolveSinkUrl } = require('../support/webhook-sink');

describe('applications webhook events', () => {
    let sink;
    let subscriber;

    // WEBHOOK_SINK_URL (set in it/docker-compose.test*.yaml) gives the hostname the
    // devportal container can reach this test container at — see resolveSinkUrl's
    // comment in support/webhook-sink.js for why the port always stays local to this
    // file rather than coming from the (shared-across-spec-files) env var.
    const sinkUrl = resolveSinkUrl(4500);

    beforeAll(async () => {
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
        subscriber = await createWebhookSubscriber({
            targetUrl: sinkUrl.href,
            events: ['application.*'],
        });
    });

    afterEach(async () => {
        await client.as('admin').del(`/webhook-subscribers/${subscriber.id}`);
        sink.received.length = 0;
    });

    it('publishes and delivers application.created', async () => {
        const since = new Date();
        const appRes = await client.as('developer').post('/applications', {
            id: uniqueHandle('app'),
            displayName: 'Webhook Test App',
            description: 'Created to verify application.created delivery',
        });
        expect(appRes.status).toBe(201);

        // aggregate_uuid is the application's internal id, which the REST
        // response never exposes (only its handle) — scope by org + type + time instead.
        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.created', since });
        expect(event).toBeDefined();
        expect(event.payload).toBeDefined();

        // dp_event_deliveries.subscriber_id stores the subscriber's internal
        // uuid, not the REST-facing handle (subscriber.id) — the API never
        // exposes that uuid, so scope by event only (fine here: one subscriber).
        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('application.created');
        expect(received).toBeDefined();

        // Full delivered envelope (src/services/webhooks/deliveryWorker.js buildEnvelope)
        // and the exact payload built in devportalController.js saveApplication.
        expect(received.body.event_type).toBe('application.created');
        expect(received.body.event_id).toBe(event.uuid);
        expect(received.body.occurred_at).toBeDefined();
        expect(received.body.org).toEqual({ ref_id: expect.any(String) });
        expect(received.body.encrypted_fields).toEqual([]);
        expect(received.headers['x-devportal-event-id']).toBe(event.uuid);

        expect(received.body.data).toEqual({
            application_id: expect.any(String),
            display_name: 'Webhook Test App',
            handle: appRes.body.id,
            description: 'Created to verify application.created delivery',
            type: 'web',
        });
    });

    it('publishes and delivers application.updated', async () => {
        const id = uniqueHandle('app');
        const createRes = await client.as('developer').post('/applications', { id, displayName: 'Original', description: 'd' });
        expect(createRes.status).toBe(201);

        const updateSince = new Date();
        const updateRes = await client.as('developer').put(`/applications/${id}`, { displayName: 'Renamed', description: 'd2' });
        expect(updateRes.status).toBe(200);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.updated', since: updateSince });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('application.updated');
        expect(received).toBeDefined();
        expect(received.body.event_type).toBe('application.updated');
        expect(received.body.data).toEqual({
            application_id: expect.any(String),
            display_name: 'Renamed',
            handle: id,
            description: 'd2',
            type: 'web',
        });
    });

    it('publishes and delivers application.deleted', async () => {
        const id = uniqueHandle('app');
        await client.as('developer').post('/applications', { id, displayName: 'To Delete', description: 'd' });

        const since = new Date();
        const delRes = await client.as('developer').del(`/applications/${id}`);
        expect(delRes.status).toBe(200);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.deleted', since });
        expect(event).toBeDefined();

        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        const received = sink.findDeliveryFor('application.deleted');
        expect(received).toBeDefined();
        expect(received.body.event_type).toBe('application.deleted');
        expect(received.body.data).toEqual({
            application_id: expect.any(String),
            display_name: 'To Delete',
            handle: id,
        });
    });

    it('does not deliver to a subscriber whose event pattern does not match', async () => {
        // A second subscriber on the same org that only wants apikey.* events —
        // application.created shouldn't be delivered to it.
        const nonMatching = await createWebhookSubscriber({
            targetUrl: sinkUrl.href,
            events: ['apikey.*'],
        });

        const since = new Date();
        const appRes = await client.as('developer').post('/applications', {
            id: uniqueHandle('app'),
            displayName: 'Pattern Mismatch App',
            description: 'd',
        });
        expect(appRes.status).toBe(201);

        const event = await waitForEvent({ orgHandle: client.ORG_HANDLE, type: 'application.created', since });
        const delivery = await waitForDelivery({ eventUuid: event.uuid });
        expect(delivery.status).toBe('DELIVERED');

        // Exactly one delivery row for this event — the apikey.*-only subscriber
        // never matched, so it never got one.
        const deliveries = await db.findDeliveries({ eventUuid: event.uuid });
        expect(deliveries).toHaveLength(1);

        await client.as('admin').del(`/webhook-subscribers/${nonMatching.id}`);
    });

    // Closes a gap the tests above don't cover on their own: response-correctness
    // (applications.spec.js) and event-correctness (this file) are asserted in
    // separate suites, so a bug that gets one right and the other wrong on the
    // same call would pass both. Assert the combination directly.
    describe('action + event consistency', () => {
        it('does not publish a second application.deleted when deleting an already-deleted application', async () => {
            // `since` is captured before EITHER delete (not between them): the first
            // delete's own event can land in the same millisecond as a since-the-second-
            // delete timestamp, and `db.findEvents`' `occurred_at >= since` would then
            // wrongly count it as "new". Filtering by handle instead of a narrow time
            // window avoids that boundary race.
            const id = uniqueHandle('app');
            const since = new Date();
            await client.as('developer').post('/applications', { id, displayName: 'Delete Twice', description: 'd' });
            await client.as('developer').del(`/applications/${id}`);

            const res = await client.as('developer').del(`/applications/${id}`);
            expect(res.status).toBe(404);

            const events = await db.findEvents({ orgUuid: await db.findOrgUuidByHandle(client.ORG_HANDLE), type: 'application.deleted', since });
            // payload is TEXT on SQLite (a string) but jsonb on Postgres (an object).
            const forThisApp = events.filter((e) => {
                const payload = typeof e.payload === 'string' ? JSON.parse(e.payload) : e.payload;
                return payload.handle === id;
            });
            expect(forThisApp).toHaveLength(1);
        });
    });
});
