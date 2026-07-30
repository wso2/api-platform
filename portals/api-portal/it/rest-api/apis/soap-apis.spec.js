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

// POST/GET/PUT/DELETE /apis (type: SOAP). definition is a WSDL file instead
// of an OpenAPI document.

const client = require('../support/client');
const { createApi, uniqueHandle } = require('../support/fixtures');

const SAMPLE_WSDL = '<?xml version="1.0"?><definitions xmlns="http://schemas.xmlsoap.org/wsdl/"></definitions>';

describe('SOAP APIs', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    it('creates a SOAP API with a WSDL definition', async () => {
        const api = await createApi({ type: 'SOAP', definition: SAMPLE_WSDL, definitionFileName: 'definition.wsdl' });
        const res = await client.as('publisher').get(`/apis/${api.id}`);
        expect(res.status).toBe(200);
        expect(res.body.type).toBe('SOAP');
    });

    it('updates a SOAP API', async () => {
        const api = await createApi({ type: 'SOAP', definition: SAMPLE_WSDL, definitionFileName: 'definition.wsdl' });
        const put = await client
            .as('publisher')
            .putMultipart(`/apis/${api.id}`)
            .field('metadata', JSON.stringify({
                name: 'Updated SOAP API',
                version: 'v1.0',
                type: 'SOAP',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('definition', Buffer.from(SAMPLE_WSDL), 'definition.wsdl');
        expect(put.status).toBe(200);
        expect(put.body.name).toBe('Updated SOAP API');
    });

    it('deletes a SOAP API', async () => {
        const api = await createApi({ type: 'SOAP', definition: SAMPLE_WSDL, definitionFileName: 'definition.wsdl' });
        const del = await client.as('publisher').del(`/apis/${api.id}`);
        expect(del.status).toBe(200);

        const get = await client.as('publisher').get(`/apis/${api.id}`);
        expect(get.status).toBe(404);
    });

    it('rejects creation without any definition file', async () => {
        const id = uniqueHandle('soap-api-no-def');
        const res = await client
            .as('publisher')
            .postMultipart('/apis')
            .field('metadata', JSON.stringify({
                id,
                name: 'No Definition SOAP API',
                version: 'v1.0',
                type: 'SOAP',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }));
        expect(res.status).toBe(400);
    });
});
