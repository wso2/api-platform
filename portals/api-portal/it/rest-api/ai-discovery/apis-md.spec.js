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

// GET /:orgName/views/:viewName/apis.md — the agent-facing API catalog
// (src/controllers/apiContentController.js loadAPIsMd). Public agent-discovery
// route, so client.raw() rather than client.as(role).
//
// loadAPIsMd buckets by api.type, which holds the *stored* constant
// (constants.API_TYPE: REST -> "RestApi", WEBSUB -> "WebSubApi"), not the enum
// key the buckets are named after. Bucketing on the raw value dropped every
// REST and WebSub API from this catalog while GraphQL and WebSocket — whose
// stored value happens to equal the enum key — still showed up, so the
// regression is invisible unless a REST API is asserted specifically.

const client = require('../support/client');
const { createApi } = require('../support/fixtures');

describe('AI/LLM discovery (apis.md)', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    it('lists a REST API in the catalog', async () => {
        const api = await createApi({ name: 'Catalogued REST API', type: 'REST', labels: ['default'] });

        const res = await client.raw().get(`${client.BASE_PATH}/${client.ORG_HANDLE}/views/default/apis.md`);
        expect(res.status).toBe(200);
        expect(res.headers['content-type']).toMatch(/text\/markdown/);
        expect(res.text).toContain(api.name);
    });

    it('lists a WebSub API in the catalog', async () => {
        const api = await createApi({ name: 'Catalogued WebSub API', type: 'WEBSUB', labels: ['default'] });

        const res = await client.raw().get(`${client.BASE_PATH}/${client.ORG_HANDLE}/views/default/apis.md`);
        expect(res.status).toBe(200);
        expect(res.text).toContain(api.name);
    });

    it('excludes APIs with agentVisibility HIDDEN', async () => {
        const hidden = await createApi({
            name: 'Hidden REST API',
            type: 'REST',
            labels: ['default'],
            agentVisibility: 'HIDDEN',
        });

        const res = await client.raw().get(`${client.BASE_PATH}/${client.ORG_HANDLE}/views/default/apis.md`);
        expect(res.status).toBe(200);
        expect(res.text).not.toContain(hidden.name);
    });
});
