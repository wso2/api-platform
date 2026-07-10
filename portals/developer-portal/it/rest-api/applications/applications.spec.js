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

// POST/GET/PUT/DELETE /applications (plain JSON — see
// docs/devportal-openapi-spec-v0.9.yaml ApplicationRequest). Applications are
// owned by their creator, so this uses the `developer` role throughout. Async
// event side effects are covered separately in webhook-events.spec.js.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

describe('applications', () => {
    beforeAll(async () => {
        await client.login('developer');
    });

    it('creates and retrieves an application', async () => {
        const id = uniqueHandle('app');
        const create = await client.as('developer').post('/applications', {
            id,
            displayName: 'Test App',
            description: 'IT fixture application',
        });
        expect(create.status).toBe(201);

        const get = await client.as('developer').get(`/applications/${id}`);
        expect(get.status).toBe(200);
        expect(get.body.displayName).toBe('Test App');
    });

    it('updates an application', async () => {
        const id = uniqueHandle('app');
        await client.as('developer').post('/applications', { id, displayName: 'Original', description: 'd' });

        const put = await client.as('developer').put(`/applications/${id}`, {
            displayName: 'Renamed App',
            description: 'updated description',
        });
        expect(put.status).toBe(200);
        expect(put.body.displayName).toBe('Renamed App');
    });

    it('deletes an application', async () => {
        const id = uniqueHandle('app');
        await client.as('developer').post('/applications', { id, displayName: 'To Delete', description: 'd' });

        const del = await client.as('developer').del(`/applications/${id}`);
        expect(del.status).toBe(200);

        const get = await client.as('developer').get(`/applications/${id}`);
        expect(get.status).toBe(404);
    });

    it('lists applications', async () => {
        const id = uniqueHandle('app');
        await client.as('developer').post('/applications', { id, displayName: 'Listed App', description: 'd' });

        const res = await client.as('developer').get('/applications');
        expect(res.status).toBe(200);
        expect(res.body.list.some((a) => a.id === id)).toBe(true);
    });

    it('rejects creation missing displayName/description', async () => {
        const res = await client.as('developer').post('/applications', { id: uniqueHandle('app') });
        expect(res.status).toBe(400);
    });

    // Covered in ../key-managers/token-generation.spec.js (generate-keys mapping
    // + generate-token flow against a mock OAuth token endpoint).
});
