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

// API keys are scoped under their owning resource: POST /apis/{apiId}/api-keys/generate,
// GET /apis/{apiId}/api-keys, POST .../regenerate, .../revoke, .../associate, .../dissociate.
// Key `id` must match ^[a-z0-9][a-z0-9_-]{0,127}$ — uniqueHandle()'s dashes/digits satisfy this.
// `publisher` manages API keys; `developer` owns the application used in
// associate/dissociate tests.

const client = require('../support/client');
const { createApi, uniqueHandle } = require('../support/fixtures');

describe('API keys', () => {
    let api;

    beforeAll(async () => {
        await client.login('publisher');
        await client.login('developer');
    });

    beforeEach(async () => {
        api = await createApi();
    });

    it('generates an API key', async () => {
        const id = uniqueHandle('key').toLowerCase();
        const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id });
        expect(res.status).toBe(201);
        expect(res.body.id).toBe(id);
        expect(res.body.key).toBeDefined();
        expect(res.body.status).toBe('ACTIVE');
    });

    it('lists API keys for an API', async () => {
        const id = uniqueHandle('key').toLowerCase();
        await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id });

        const res = await client.as('publisher').get(`/apis/${api.id}/api-keys`);
        expect(res.status).toBe(200);
        expect(res.body.list.some((k) => k.id === id)).toBe(true);
    });

    it('regenerates an API key', async () => {
        const id = uniqueHandle('key').toLowerCase();
        const create = await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id });

        const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/regenerate`, { keyId: id });
        expect(res.status).toBe(200);
        expect(res.body.key).toBeDefined();
        expect(res.body.key).not.toBe(create.body.key);
    });

    it('revokes an API key', async () => {
        const id = uniqueHandle('key').toLowerCase();
        await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id });

        const revoke = await client.as('publisher').post(`/apis/${api.id}/api-keys/revoke`, { keyId: id });
        expect(revoke.status).toBe(204);

        const list = await client.as('publisher').get(`/apis/${api.id}/api-keys`);
        const key = list.body.list.find((k) => k.id === id);
        expect(key.status).toBe('REVOKED');
    });

    it('rejects regenerating a revoked key', async () => {
        const id = uniqueHandle('key').toLowerCase();
        await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id });
        await client.as('publisher').post(`/apis/${api.id}/api-keys/revoke`, { keyId: id });

        const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/regenerate`, { keyId: id });
        expect(res.status).toBe(409);
    });

    // NOTE: associate/dissociate resolve the target application via
    // resolveAppId(orgId, util.resolveActor(req), appHandle) — i.e. scoped to
    // the *caller's own* created_by, not just API-key scope. A publisher can
    // generate the key but can never associate it with another user's
    // application (404 "Application not found") even with full dp:api_key_manage
    // scope — only the app's own owner can call associate/dissociate on it.
    // That's surprising given the OpenAPI description ("Associates ... with an
    // application, for analytics attribution only") doesn't mention this
    // same-owner constraint, and it's not how a publisher distributing keys to
    // subscribers would expect this to work — worth a source-level look at
    // apiKeyController.js:273 (associateApiKeyApplication) and its dissociate
    // counterpart. Using `developer` throughout here since it's the only role
    // that can currently make this call succeed for its own app.
    it('associates an API key with an application', async () => {
        const keyId = uniqueHandle('key').toLowerCase();
        const appId = uniqueHandle('app');
        await client.as('developer').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });
        await client.as('developer').post('/applications', { id: appId, displayName: 'Assoc App', description: 'd' });

        const res = await client.as('developer').post(`/apis/${api.id}/api-keys/associate`, { keyId, appId });
        expect(res.status).toBe(200);
    });

    it('dissociates an API key from an application', async () => {
        const keyId = uniqueHandle('key').toLowerCase();
        const appId = uniqueHandle('app');
        await client.as('developer').post(`/apis/${api.id}/api-keys/generate`, { id: keyId });
        await client.as('developer').post('/applications', { id: appId, displayName: 'Dissoc App', description: 'd' });
        await client.as('developer').post(`/apis/${api.id}/api-keys/associate`, { keyId, appId });

        const res = await client.as('developer').post(`/apis/${api.id}/api-keys/dissociate`, { keyId });
        expect(res.status).toBe(204);
    });

    it('rejects generating a key with an invalid id format', async () => {
        const res = await client.as('publisher').post(`/apis/${api.id}/api-keys/generate`, { id: 'Invalid ID!' });
        expect(res.status).toBe(400);
    });

});
