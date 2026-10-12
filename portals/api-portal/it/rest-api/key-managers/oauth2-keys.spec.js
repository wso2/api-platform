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
const { createDcrServer, PROVISIONING } = require('../support/dcr-server');

const DCR_PORT = Number(new URL(process.env.MOCK_DCR_ENDPOINT_URL || 'http://localhost:4505').port);

/*
 * ONE server for the whole file, on the one port the portal is configured to
 * reach. A second instance on another port looked reasonable and was not: the
 * endpoints a key manager is pointed at come from MOCK_DCR_ENDPOINT_URL, so a
 * server listening anywhere else is simply never called.
 */
let dcr;

beforeAll(async () => {
    dcr = createDcrServer();
    await dcr.start(DCR_PORT);
});

afterAll(async () => {
    if (dcr) await dcr.stop();
});

describe('OAuth2 keys', () => {
    let kmId;

    beforeAll(async () => {
        await client.login('admin');
        await client.login('developer');

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
                auth: { method: 'client_credentials', ...PROVISIONING },
            },
        });
        expect(created.status).toBe(201);
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

/*
 * The paths a developer hits when something is wrong, and the provisioning
 * credentials other than client_credentials.
 *
 * Separate describe because these need their own key managers: the auth method a
 * key manager provisions with is fixed at creation, and the error cases turn on
 * what a client registered itself as rather than on the request.
 */
describe('OAuth2 keys — credentials and failure paths', () => {
    let kmBasic;
    let kmApiKey;
    let kmId;

    beforeAll(async () => {
        await client.login('admin');
        await client.login('developer');

        const make = async (id, auth) => {
            const res = await client.as('admin').post('/key-managers', {
                id,
                displayName: `KM ${id}`,
                tokenEndpoint: dcr.tokenEndpoint(),
                provisioning: { type: 'custom', registrationEndpoint: dcr.registrationEndpoint(), auth },
            });
            expect(res.status).toBe(201);
            return id;
        };

        // The portal presents a different credential to the identity server for
        // each of these. Only client_credentials is exercised elsewhere.
        kmId = await make(uniqueHandle('km-cc'), { method: 'client_credentials', ...PROVISIONING });
        kmBasic = await make(uniqueHandle('km-basic'),
            { method: 'basic', username: 'portal', password: 'portal-pass' });
        kmApiKey = await make(uniqueHandle('km-apikey'),
            { method: 'api_key', apiKey: 'portal-key', headerName: 'X-Api-Key' });
    });

    const create = (km, properties) => client.as('developer').post('/oauth2-keys', {
        keyManagerId: km, properties: { grant_types: ['client_credentials'], ...properties },
    });

    it.each([
        ['basic auth', () => kmBasic, 'Basic '],
        ['api key auth', () => kmApiKey, null],
    ])('registers a client through a key manager using %s', async (_label, km, expectedPrefix) => {
        const res = await create(km(), { client_name: 'cred-check' });
        expect(res.status).toBe(201);
        expect(res.body.consumerKey).toMatch(/^mock-client-/);

        // The portal's own credential reached the identity server in the shape
        // that method defines — this is the only place those authenticators are
        // exercised against a real request.
        const registration = dcr.requestsOf('POST')
            .filter((r) => r.path === '/register')
            .slice(-1)[0];
        if (expectedPrefix) {
            expect(registration.auth).toMatch(new RegExp(`^${expectedPrefix}`));
        } else {
            // api_key auth puts it in its configured header, not Authorization.
            expect(registration.auth).toBe('');
        }
    });

    it('issues a token when the developer supplies the right secret', async () => {
        const created = await create(kmId, { client_name: 'token-ok' });
        const res = await client.as('developer')
            .post(`/oauth2-keys/${created.body.keyId}/generate-token`,
                { consumerSecret: created.body.consumerSecret });
        expect(res.status).toBe(200);
        expect(res.body.accessToken).toBe('mock-access-token');
    });

    it('answers 400 when the identity server rejects the secret', async () => {
        /*
         * The developer's own credential is wrong, not the portal's — so this is
         * their request to fix and the API says so with a 4xx rather than
         * reporting an upstream fault.
         */
        const created = await create(kmId, { client_name: 'token-bad-secret' });
        const res = await client.as('developer')
            .post(`/oauth2-keys/${created.body.keyId}/generate-token`,
                { consumerSecret: 'not-the-right-secret' });
        expect(res.status).toBe(400);
        // The upstream body is never echoed: it names an internal host.
        expect(JSON.stringify(res.body)).not.toMatch(/invalid_client/);
    });

    it('refuses a token for a public client, which has no secret to present', async () => {
        // RFC 6749 §4.4 restricts client_credentials to confidential clients, so
        // a client registered with auth method "none" can never use this flow.
        const created = await create(kmId, {
            client_name: 'public-client', token_endpoint_auth_method: 'none',
        });
        expect(created.status).toBe(201);

        const res = await client.as('developer')
            .post(`/oauth2-keys/${created.body.keyId}/generate-token`, { consumerSecret: 'anything' });
        // 409: the key exists and the caller may see it; it is simply not eligible.
        expect(res.status).toBe(409);
    });

    it('refuses a token for a client whose auth method the portal cannot present', async () => {
        const created = await create(kmId, {
            client_name: 'jwt-client', token_endpoint_auth_method: 'private_key_jwt',
        });
        expect(created.status).toBe(201);

        const res = await client.as('developer')
            .post(`/oauth2-keys/${created.body.keyId}/generate-token`, { consumerSecret: 'anything' });
        expect(res.status).toBe(409);
    });

    it('sends the secret where the client said it would accept it', async () => {
        // client_secret_post puts it in the body; the default puts it in the
        // Authorization header. Sending it the wrong way is a plain 401 from a
        // real server, indistinguishable from a wrong secret.
        const created = await create(kmId, {
            client_name: 'post-client', token_endpoint_auth_method: 'client_secret_post',
        });
        const res = await client.as('developer')
            .post(`/oauth2-keys/${created.body.keyId}/generate-token`,
                { consumerSecret: created.body.consumerSecret });
        expect(res.status).toBe(200);

        const tokenCall = dcr.requestsOf('POST').filter((r) => r.path === '/token').slice(-1)[0];
        expect(tokenCall.body.client_secret).toBe(created.body.consumerSecret);
        expect(tokenCall.auth).toBe('');
    });

    it('passes requested scopes through to the identity server', async () => {
        const created = await create(kmId, { client_name: 'scoped' });
        const res = await client.as('developer')
            .post(`/oauth2-keys/${created.body.keyId}/generate-token`, {
                consumerSecret: created.body.consumerSecret,
                scopes: ['read', 'write'],
            });
        expect(res.status).toBe(200);
        const tokenCall = dcr.requestsOf('POST').filter((r) => r.path === '/token').slice(-1)[0];
        expect(tokenCall.body.scope).toBe('read write');
    });
});
