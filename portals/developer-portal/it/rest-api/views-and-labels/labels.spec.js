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

// POST/GET/PUT/DELETE /labels (src/routes/api/handlers/labelsHandler.js).
// LabelRequest requires { id, displayName }. `admin` manages org-level config.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

describe('labels', () => {
    beforeAll(async () => {
        await client.login('admin');
    });

    it('creates a label', async () => {
        const id = uniqueHandle('label');
        const res = await client.as('admin').post('/labels', { id, displayName: 'Premium APIs' });
        expect(res.status).toBe(201);
        expect(res.body.id).toBe(id);
        expect(res.body.displayName).toBe('Premium APIs');
    });

    it('retrieves a label', async () => {
        const id = uniqueHandle('label');
        await client.as('admin').post('/labels', { id, displayName: 'Retrieved Label' });

        const res = await client.as('admin').get(`/labels/${id}`);
        expect(res.status).toBe(200);
        expect(res.body.displayName).toBe('Retrieved Label');
    });

    it('updates a label', async () => {
        const id = uniqueHandle('label');
        await client.as('admin').post('/labels', { id, displayName: 'Original Label' });

        const res = await client.as('admin').put(`/labels/${id}`, { id, displayName: 'Renamed Label' });
        expect(res.status).toBe(200);
        expect(res.body.displayName).toBe('Renamed Label');
    });

    it('deletes a label', async () => {
        const id = uniqueHandle('label');
        await client.as('admin').post('/labels', { id, displayName: 'To Delete' });

        const del = await client.as('admin').del(`/labels/${id}`);
        expect(del.status).toBe(204);

        const get = await client.as('admin').get(`/labels/${id}`);
        expect(get.status).toBe(404);
    });

    it('lists labels for an org', async () => {
        const id = uniqueHandle('label');
        await client.as('admin').post('/labels', { id, displayName: 'Listed Label' });

        const res = await client.as('admin').get('/labels');
        expect(res.status).toBe(200);
        expect(res.body.list.some((l) => l.id === id)).toBe(true);
    });
});
