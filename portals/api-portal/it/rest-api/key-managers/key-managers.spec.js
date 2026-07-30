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

// POST/GET/PUT/DELETE /key-managers. Every key manager is an OAuth2
// client_credentials provider.
// Body accepts YAML or JSON per the handler; keep the fixture JSON-only.
// `admin` manages org-level integration config.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

describe('key managers', () => {
    beforeAll(async () => {
        await client.login('admin');
    });

    it('creates a key manager with a token endpoint', async () => {
        const id = uniqueHandle('km');
        const res = await client.as('admin').post('/key-managers', {
            id,
            displayName: 'Test KM',
            tokenEndpoint: 'https://asgardeo.example.invalid/oauth2/token',
        });
        expect(res.status).toBe(201);
        expect(res.body.id).toBe(id);
    });

    it('retrieves a key manager', async () => {
        const id = uniqueHandle('km');
        await client.as('admin').post('/key-managers', {
            id, displayName: 'Test KM', tokenEndpoint: 'https://asgardeo.example.invalid/oauth2/token',
        });

        const res = await client.as('admin').get(`/key-managers/${id}`);
        expect(res.status).toBe(200);
        expect(res.body.displayName).toBe('Test KM');
    });

    it('updates a key manager', async () => {
        const id = uniqueHandle('km');
        await client.as('admin').post('/key-managers', {
            id, displayName: 'Original KM', tokenEndpoint: 'https://asgardeo.example.invalid/oauth2/token',
        });

        const res = await client.as('admin').put(`/key-managers/${id}`, { displayName: 'Renamed KM' });
        expect(res.status).toBe(200);
        expect(res.body.displayName).toBe('Renamed KM');
    });

    it('deletes a key manager', async () => {
        const id = uniqueHandle('km');
        await client.as('admin').post('/key-managers', {
            id, displayName: 'To Delete', tokenEndpoint: 'https://asgardeo.example.invalid/oauth2/token',
        });

        const del = await client.as('admin').del(`/key-managers/${id}`);
        expect(del.status).toBe(204);

        const get = await client.as('admin').get(`/key-managers/${id}`);
        expect(get.status).toBe(404);
    });

    it('lists key managers for an org', async () => {
        const id = uniqueHandle('km');
        await client.as('admin').post('/key-managers', {
            id, displayName: 'Listed KM', tokenEndpoint: 'https://asgardeo.example.invalid/oauth2/token',
        });

        const res = await client.as('admin').get('/key-managers');
        expect(res.status).toBe(200);
        expect(res.body.list.some((km) => km.id === id)).toBe(true);
    });
});
