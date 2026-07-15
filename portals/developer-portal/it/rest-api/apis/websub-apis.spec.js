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

// POST/GET/PUT/DELETE /apis (type: WEBSUB). This is devportal's own registration
// of an already-published WebSub API's metadata — not to be confused with the
// event-gateway's WebSub API management (gateway.api-platform.wso2.com).

const client = require('../support/client');
const { createApi } = require('../support/fixtures');

describe('WebSub APIs', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    it('creates a WebSub API', async () => {
        const api = await createApi({ type: 'WEBSUB' });
        const res = await client.as('publisher').get(`/apis/${api.id}`);
        expect(res.status).toBe(200);
        expect(res.body.type).toBe('WebSubApi');
    });

    it('updates a WebSub API', async () => {
        const api = await createApi({ type: 'WEBSUB' });
        const put = await client
            .as('publisher')
            .putMultipart(`/apis/${api.id}`)
            .field('metadata', JSON.stringify({
                name: 'Updated WebSub API',
                version: 'v1.0',
                type: 'WEBSUB',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('definition', Buffer.from('{}'), 'definition.json');
        expect(put.status).toBe(200);
        expect(put.body.name).toBe('Updated WebSub API');
    });

    it('deletes a WebSub API', async () => {
        const api = await createApi({ type: 'WEBSUB' });
        const del = await client.as('publisher').del(`/apis/${api.id}`);
        expect(del.status).toBe(200);

        const get = await client.as('publisher').get(`/apis/${api.id}`);
        expect(get.status).toBe(404);
    });
});
