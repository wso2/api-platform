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

// POST/GET/PUT/DELETE /webhook-subscribers, GET /webhook-subscribers/{id}/deliveries.
// Request shape: { id, displayName?, targetUrl, secret?, publicKey?, events?, enabled? }
// — see docs/devportal-openapi-spec-v0.9.yaml WebhookSubscriberRequest.
// `events` is a glob allowlist (trailing `*` only, e.g. "apikey.*"); empty/omitted
// means all event types. `admin` manages org-level integration config.

const client = require('../support/client');
const { createWebhookSubscriber, uniqueHandle } = require('../support/fixtures');

describe('webhook subscribers', () => {
    beforeAll(async () => {
        await client.login('admin');
        await client.login('developer');
    });

    it('creates and retrieves a webhook subscriber', async () => {
        const subscriber = await createWebhookSubscriber({
            targetUrl: 'https://example.invalid/webhook',
            events: ['apikey.*'],
        });
        const res = await client.as('admin').get(`/webhook-subscribers/${subscriber.id}`);
        expect(res.status).toBe(200);
        expect(res.body.targetUrl).toBe('https://example.invalid/webhook');
        // secret must never round-trip in a response
        expect(res.body.secret).toBeUndefined();
    });

    it('updates a webhook subscriber (target URL, events, enabled)', async () => {
        const subscriber = await createWebhookSubscriber({
            targetUrl: 'https://example.invalid/webhook',
            events: ['apikey.*'],
        });

        // NOTE: docs/devportal-openapi-spec-v0.9.yaml's WebhookSubscriberUpdateBody
        // description says "all fields are optional; only supplied fields are
        // updated", but it references the base WebhookSubscriberRequest schema,
        // which requires `id` and `targetUrl` — the server enforces the schema,
        // rejecting a PUT that omits `id` with a 400. Doc/implementation mismatch.
        const res = await client.as('admin').put(`/webhook-subscribers/${subscriber.id}`, {
            id: subscriber.id,
            targetUrl: 'https://updated.example.invalid/webhook',
            events: ['subscription.*'],
            enabled: false,
        });
        expect(res.status).toBe(200);
        expect(res.body.targetUrl).toBe('https://updated.example.invalid/webhook');
        expect(res.body.enabled).toBe(false);
    });

    it('deletes a webhook subscriber', async () => {
        const subscriber = await createWebhookSubscriber({
            targetUrl: 'https://example.invalid/webhook',
        });

        const del = await client.as('admin').del(`/webhook-subscribers/${subscriber.id}`);
        expect(del.status).toBe(204);

        const get = await client.as('admin').get(`/webhook-subscribers/${subscriber.id}`);
        expect(get.status).toBe(404);
    });

    it('lists deliveries for a subscriber', async () => {
        const subscriber = await createWebhookSubscriber({
            targetUrl: 'https://example.invalid/webhook',
            events: ['application.*'],
        });
        await client.as('developer').post('/applications', {
            id: uniqueHandle('delivery-list-app'),
            displayName: 'Delivery List App',
            description: 'd',
        });

        // No sink is listening at example.invalid, so the delivery attempt itself
        // fails — this only asserts the endpoint surfaces delivery attempts at all.
        const res = await client.as('admin').get(`/webhook-subscribers/${subscriber.id}/deliveries`);
        expect(res.status).toBe(200);
        expect(Array.isArray(res.body.list)).toBe(true);
    });

    it('accepts a pattern without a trailing wildcard as a literal exact-match filter', async () => {
        // webhookSubscriberDao.js's matchSubscribers only special-cases patterns
        // ending in ".*" (prefix match); anything else — like "*.created" — is
        // compared with strict equality against the event type, so it's accepted
        // at creation time but will simply never match any real event type.
        const res = await client.as('admin').post('/webhook-subscribers', {
            id: uniqueHandle('literal-pattern-subscriber'),
            targetUrl: 'https://example.invalid/webhook',
            events: ['*.created'],
        });
        expect(res.status).toBe(201);
        expect(res.body.events).toEqual(['*.created']);
    });
});
