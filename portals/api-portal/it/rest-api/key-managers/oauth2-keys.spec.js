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

'use strict';

/*
 * The OAuth2 key lifecycle, end to end, over the REST API.
 *
 * This is the journey the feature exists for and the one nothing covered: an
 * admin configures a key manager, a developer asks it for credentials, the
 * portal registers a real client on it over RFC 7591, manages that client over
 * RFC 7592, and deletes it again. Until this spec, the UI suites stopped at
 * "the key manager appears in the dropdown" and no test ever created a key.
 *
 * The identity server is support/dcr-server.js — a real HTTP server speaking the
 * protocol, reached by container name on the compose network, the same way
 * webhook-sink.js is. So the portal makes genuine requests and the assertions
 * can be about what it actually sent, not about a mock being called.
 *
 * `admin` configures the key manager; `developer` owns the keys, because the
 * key DAO scopes every read by the caller's created_by.
 */

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');
const { createDcrServer } = require('../support/dcr-server');

const DCR_PORT = Number(new URL(process.env.MOCK_DCR_ENDPOINT_URL || 'http://localhost:4505').port);

describe('OAuth2 keys', () => {
    let dcr;
    let kmId;

    beforeAll(async () => {
        await client.login('admin');
        await client.login('developer');

        dcr = createDcrServer();
        await dcr.start(DCR_PORT);

        // A key manager that registers clients: a provisioning block makes the
        // portal a DCR client of the server above.
        kmId = uniqueHandle('km-dcr');
        const created = await client.as('admin').post('/key-managers', {
            id: kmId,
            displayName: 'Mock DCR KM',
            tokenEndpoint: dcr.tokenEndpoint(),
            provisioning: {
                type: 'custom',
                registrationEndpoint: dcr.registrationEndpoint(),
                auth: { method: 'client_credentials', clientId: 'portal', clientSecret: 'portal-secret' },
            },
        });
        expect(created.status).toBe(201);
    });

    afterAll(async () => {
        if (dcr) await dcr.stop();
    });

    it('registers a client on the key manager and returns credentials', async () => {
        const res = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: kmId,
            properties: {
                client_name: 'checkout-service',
                redirect_uris: ['https://app.example.com/cb'],
                grant_types: ['client_credentials'],
            },
        });

        expect(res.status).toBe(201);
        expect(res.body.keyId).toEqual(expect.any(String));
        // The client id comes from the identity server, not from the portal.
        expect(res.body.consumerKey).toMatch(/^mock-client-/);
        // The secret is returned once, on this response only.
        expect(res.body.consumerSecret).toBe(`secret-${res.body.consumerKey}`);

        // The portal really did register it, with the metadata it was given.
        const registrations = dcr.requestsOf('POST').filter((r) => r.path === '/register');
        expect(registrations.length).toBeGreaterThan(0);
        const sent = registrations[registrations.length - 1].body;
        expect(sent.client_name).toBe('checkout-service');
        expect(sent.redirect_uris).toEqual(['https://app.example.com/cb']);
        expect(dcr.clientIds()).toContain(res.body.consumerKey);
    });

    it('never returns the client secret again after creation', async () => {
        const created = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: kmId,
            properties: { client_name: 'secret-check', grant_types: ['client_credentials'] },
        });
        expect(created.status).toBe(201);
        expect(created.body.consumerSecret).toBeTruthy();

        const read = await client.as('developer').get(`/oauth2-keys/${created.body.keyId}`);
        expect(read.status).toBe(200);
        // The portal stores the client's identity, never its secret — so a later
        // read cannot produce one. This is the guarantee the whole design rests on.
        expect(read.body.consumerSecret).toBeFalsy();
    });

    it('reads a key back through the RFC 7592 configuration endpoint', async () => {
        const created = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: kmId,
            properties: { client_name: 'readback', grant_types: ['client_credentials'] },
        });
        const before = dcr.requestsOf('GET').length;

        const read = await client.as('developer').get(`/oauth2-keys/${created.body.keyId}`);
        expect(read.status).toBe(200);
        expect(read.body.consumerKey).toBe(created.body.consumerKey);

        // The metadata came from the identity server rather than the portal's own
        // row: a GET was made, and it was authenticated with the registration
        // access token the server issued, not the portal's provisioning credential.
        const gets = dcr.requestsOf('GET');
        expect(gets.length).toBeGreaterThan(before);
        const last = gets[gets.length - 1];
        expect(last.path).toBe(`/clients/${created.body.consumerKey}`);
        expect(last.auth).toBe(`Bearer rat-${created.body.consumerKey}`);
    });

    it('updates a client and carries its client_id in the body', async () => {
        const created = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: kmId,
            properties: { client_name: 'before-rename', grant_types: ['client_credentials'] },
        });

        const updated = await client.as('developer').put(`/oauth2-keys/${created.body.keyId}`, {
            properties: { client_name: 'after-rename', grant_types: ['client_credentials'] },
        });
        expect(updated.status).toBe(200);

        const puts = dcr.requestsOf('PUT');
        expect(puts.length).toBeGreaterThan(0);
        const sent = puts[puts.length - 1];
        expect(sent.body.client_name).toBe('after-rename');
        // RFC 7592 §2.2 requires client_id in every update, and a conforming server
        // may refuse a PUT without it even once authenticated.
        expect(sent.body.client_id).toBe(created.body.consumerKey);
    });

    it('exchanges the developer own secret for a token without storing it', async () => {
        const created = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: kmId,
            properties: { client_name: 'token-user', grant_types: ['client_credentials'] },
        });

        const token = await client.as('developer')
            .post(`/oauth2-keys/${created.body.keyId}/generate-token`, {
                consumerSecret: created.body.consumerSecret,
            });

        expect(token.status).toBe(200);
        expect(token.body.accessToken).toBe('mock-access-token');
        // The secret travelled on this request; it is not held anywhere to be
        // reused, which is why the caller has to supply it every time.
        const tokenCalls = dcr.requestsOf('POST').filter((r) => r.path === '/token');
        expect(tokenCalls.length).toBeGreaterThan(0);
    });

    it('deletes the client at the key manager, then the local record', async () => {
        const created = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: kmId,
            properties: { client_name: 'to-delete', grant_types: ['client_credentials'] },
        });
        const consumerKey = created.body.consumerKey;
        expect(dcr.clientIds()).toContain(consumerKey);

        const removed = await client.as('developer').del(`/oauth2-keys/${created.body.keyId}`);
        expect(removed.status).toBe(204);

        // Upstream first: the client is gone from the identity server, not merely
        // forgotten by the portal — a local-only delete would leave a live client
        // nothing can reach.
        expect(dcr.clientIds()).not.toContain(consumerKey);

        const read = await client.as('developer').get(`/oauth2-keys/${created.body.keyId}`);
        expect(read.status).toBe(404);
    });

    it('keeps one developer keys invisible to another', async () => {
        const created = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: kmId,
            properties: { client_name: 'owned', grant_types: ['client_credentials'] },
        });

        // admin configured the key manager but does not own this key: ownership is
        // applied in SQL, so the row is never loaded rather than filtered after.
        const asAdmin = await client.as('admin').get(`/oauth2-keys/${created.body.keyId}`);
        expect(asAdmin.status).toBe(404);
    });

    it('refuses a property the key manager does not declare', async () => {
        const res = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: kmId,
            properties: { client_name: 'bad', not_a_real_property: 'x' },
        });
        expect(res.status).toBe(400);
        // Refused before anything was dialled: no client was registered for it.
        const names = dcr.requestsOf('POST')
            .filter((r) => r.path === '/register')
            .map((r) => r.body && r.body.client_name);
        expect(names).not.toContain('bad');
    });

    it('refuses an unknown key manager without echoing the submitted id', async () => {
        const res = await client.as('developer').post('/oauth2-keys', {
            keyManagerId: 'no-such-key-manager',
            properties: { client_name: 'x' },
        });
        expect(res.status).toBe(404);
        expect(JSON.stringify(res.body)).not.toContain('no-such-key-manager');
    });
});
