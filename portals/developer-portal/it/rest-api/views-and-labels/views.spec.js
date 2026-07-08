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

// POST/GET/PUT/DELETE /views. A view groups a set of labels to filter which
// APIs are visible in that portal view. ViewCreateRequest requires { id, labels }.
// `admin` manages org-level config.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

async function createLabel(overrides = {}) {
    const id = overrides.id || uniqueHandle('label');
    const res = await client.as('admin').post('/labels', { id, displayName: overrides.displayName || id });
    if (res.status !== 201) {
        throw new Error(`Failed to seed label: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return res.body;
}

describe('views', () => {
    let label;

    beforeAll(async () => {
        await client.login('admin');
    });

    beforeEach(async () => {
        label = await createLabel();
    });

    it('creates a view', async () => {
        const id = uniqueHandle('view');
        const res = await client.as('admin').post('/views', { id, displayName: 'Partner APIs', labels: [label.id] });
        expect(res.status).toBe(201);
    });

    it('retrieves a view', async () => {
        const id = uniqueHandle('view');
        await client.as('admin').post('/views', { id, displayName: 'Retrievable View', labels: [label.id] });

        const res = await client.as('admin').get(`/views/${id}`);
        expect(res.status).toBe(200);
        expect(res.body.displayName).toBe('Retrievable View');
        expect(res.body.labels).toContain(label.id);
    });

    it('updates a view (display name, label associations)', async () => {
        const id = uniqueHandle('view');
        await client.as('admin').post('/views', { id, displayName: 'Original View', labels: [label.id] });
        const label2 = await createLabel();

        const res = await client.as('admin').put(`/views/${id}`, { displayName: 'Updated View', labels: [label2.id] });
        expect(res.status).toBe(200);

        const get = await client.as('admin').get(`/views/${id}`);
        expect(get.body.displayName).toBe('Updated View');
        expect(get.body.labels).toEqual([label2.id]);
    });

    it('deletes a view', async () => {
        const id = uniqueHandle('view');
        await client.as('admin').post('/views', { id, displayName: 'To Delete', labels: [label.id] });

        const del = await client.as('admin').del(`/views/${id}`);
        expect(del.status).toBe(204);

        const get = await client.as('admin').get(`/views/${id}`);
        expect(get.status).toBe(404);
    });

    it('lists views for an org', async () => {
        const id = uniqueHandle('view');
        await client.as('admin').post('/views', { id, displayName: 'Listed View', labels: [label.id] });

        const res = await client.as('admin').get('/views');
        expect(res.status).toBe(200);
        expect(res.body.list.some((v) => v.id === id)).toBe(true);
    });
});
