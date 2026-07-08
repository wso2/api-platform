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

// POST/GET/PUT/DELETE /apis (type: REST). See docs/devportal-openapi-spec-v0.9.yaml
// ApiMetadataMultipartBody — creation is multipart (apiMetadata JSON + apiDefinition
// file). Publisher owns API management; follow support/fixtures.js createApi()
// for the request shape; organizations.spec.js for the assertion style.

const client = require('../support/client');
const { createApi, uniqueHandle } = require('../support/fixtures');

async function createLabel(overrides = {}) {
    const id = overrides.id || uniqueHandle('label');
    const res = await client.as('admin').post('/labels', { id, displayName: overrides.displayName || id });
    if (res.status !== 201) {
        throw new Error(`Failed to seed label: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return res.body;
}

describe('REST APIs', () => {
    beforeAll(async () => {
        await client.login('publisher');
        await client.login('admin');
    });

    it('creates and retrieves a REST API', async () => {
        const api = await createApi({ type: 'REST' });
        const res = await client.as('publisher').get(`/apis/${api.id}`);
        expect(res.status).toBe(200);
        // The stored/returned type is the internal constant ("RestApi"), not the
        // request-time enum value ("REST") — src/utils/constants.js API_TYPE mapping.
        expect(res.body.type).toBe('RestApi');
    });

    it('updates a REST API', async () => {
        const api = await createApi({ type: 'REST', name: 'Original Name' });
        const put = await client
            .as('publisher')
            .putMultipart(`/apis/${api.id}`)
            .field('apiMetadata', JSON.stringify({
                name: 'Updated Name',
                version: 'v1.0',
                type: 'REST',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
        expect(put.status).toBe(200);
        expect(put.body.name).toBe('Updated Name');
    });

    it('deletes a REST API', async () => {
        const api = await createApi({ type: 'REST' });
        const del = await client.as('publisher').del(`/apis/${api.id}`);
        expect(del.status).toBe(200);

        const get = await client.as('publisher').get(`/apis/${api.id}`);
        expect(get.status).toBe(404);
    });

    it('lists REST APIs with name/version/tag filters', async () => {
        const name = uniqueHandle('Filterable API');
        await createApi({ type: 'REST', name, version: 'v2.0' });

        const res = await client.as('publisher').get(`/apis?apiName=${encodeURIComponent(name)}&version=v2.0`);
        expect(res.status).toBe(200);
        expect(res.body.list.some((a) => a.name === name)).toBe(true);
    });

    it('rejects creation with an invalid OpenAPI definition', async () => {
        const id = uniqueHandle('bad-api');
        const res = await client
            .as('publisher')
            .postMultipart('/apis')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'Bad API',
                version: 'v1.0',
                type: 'REST',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }))
            .attach('apiDefinition', Buffer.from('not a valid openapi document'), 'definition.json');
        expect(res.status).toBe(400);
    });

    it('rejects creation when type is omitted', async () => {
        const id = uniqueHandle('no-type-api');
        const res = await client
            .as('publisher')
            .postMultipart('/apis')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'No Type API',
                version: 'v1.0',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
        expect(res.status).toBe(400);
    });

    it('rejects an update when type is omitted', async () => {
        const api = await createApi({ type: 'REST' });
        const put = await client
            .as('publisher')
            .putMultipart(`/apis/${api.id}`)
            .field('apiMetadata', JSON.stringify({
                name: 'Updated Without Type',
                version: 'v1.0',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
        expect(put.status).toBe(400);
    });

    // type is immutable once created — resolveTypeOrReject only guards the /apis-vs-
    // /mcp-servers boundary, so this is a separate check against the record's own
    // existing type (apiMetadataService.js's updateAPIMetadata).
    it('rejects changing type on update', async () => {
        const api = await createApi({ type: 'REST' });
        const put = await client
            .as('publisher')
            .putMultipart(`/apis/${api.id}`)
            .field('apiMetadata', JSON.stringify({
                name: 'Should Stay REST',
                version: 'v1.0',
                type: 'SOAP',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('apiDefinition', Buffer.from('<wsdl/>'), 'definition.wsdl');
        expect(put.status).toBe(409);

        const get = await client.as('publisher').get(`/apis/${api.id}`);
        expect(get.body.type).toBe('RestApi');
    });

    it('rejects requests without an authenticated session', async () => {
        const res = await client.raw().get(`${client.API_PREFIX}/apis`);
        // No credentials → both security handlers (OAuth2Security/apiKeyAuth) throw
        // 401; 403 is only for authenticated-but-insufficient-scope requests.
        expect(res.status).toBe(401);
    });

    it('rejects a request whose resolved type is MCP (must use /mcp-servers)', async () => {
        const id = uniqueHandle('should-be-mcp');
        const res = await client
            .as('publisher')
            .postMultipart('/apis')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'Should Be MCP',
                version: 'v1.0',
                type: 'MCP',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
        expect(res.status).toBe(400);
    });

    it("rejects a role without API management scope (developer can't create an API)", async () => {
        await client.login('developer');
        const id = uniqueHandle('forbidden-api');
        const res = await client
            .as('developer')
            .postMultipart('/apis')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'Should Be Forbidden',
                version: 'v1.0',
                type: 'REST',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
        expect(res.status).toBe(403);
    });

    it('uploads and retrieves an OpenAPI definition', async () => {
        // createApi()'s default upload (support/fixtures.js) — fileName must match
        // what was actually persisted, not a hardcoded 'apiDefinition.json': the file
        // keeps its original uploaded name ('definition.json') under type=API_DEFINITION
        // (src/utils/constants.js DOC_TYPES.API_DEFINITION).
        const api = await createApi({ type: 'REST' });
        const res = await client.as('publisher').get(`/apis/${api.id}/assets?type=API_DEFINITION&fileName=definition.json`);
        expect(res.status).toBe(200);
        // .json gets served as application/json (util.retrieveContentType), so supertest
        // auto-parses it into res.body rather than leaving it as raw text.
        expect(res.body).toEqual({ openapi: '3.0.3', info: { title: 'Fixture API', version: '1.0.0' }, paths: { '/ping': { get: { responses: { 200: { description: 'ok' } } } } } });
    });

    it('returns 404 (not a hang) for a definition file that was never uploaded', async () => {
        const api = await createApi({ type: 'REST' });
        const res = await client.as('publisher').get(`/apis/${api.id}/assets?type=API_DEFINITION&fileName=does-not-exist.json`);
        expect(res.status).toBe(404);
    });

    // apiDao.getByCondition (single-GET and name/version-filtered list) previously had
    // no Labels include at all, so labels set at creation never came back on GET —
    // only view-filtered/search listings (which use a different DAO query) exposed them.
    it('creates an API with labels and returns them on GET', async () => {
        const label = await createLabel();
        const api = await createApi({ type: 'REST', labels: [label.id] });

        const res = await client.as('publisher').get(`/apis/${api.id}`);
        expect(res.status).toBe(200);
        expect(res.body.labels).toEqual([label.id]);
    });

    describe('label updates', () => {
        // apiDao.update() has no `labels` column — labels only ever change via the
        // add/remove diff computed in updateAPIMetadata, which used to only run for
        // YAML/artifact uploads. A `labels` array sent via the plain JSON apiMetadata
        // field (what every other update test in this suite uses) was a silent no-op.
        //
        // NOTE: every PUT body here includes `id: api.id`. Without it, apiDao.update()
        // recomputes the handle from name+version (`handle: apiMetadata.handle ? ... :
        // slugify(name)-v(version)`) since the JSON update path never re-derives handle
        // from an omitted `id` the way create does — silently changing the resource's
        // own identifier out from under a follow-up GET by the original id. That's a
        // separate, pre-existing quirk (not something these label tests should mask by
        // relying on it) — every other update test in this suite is unaffected only
        // because none of them re-GET by the original id afterward.
        it('changes labels on update (new labels replace old ones)', async () => {
            const labelA = await createLabel();
            const labelB = await createLabel();
            const api = await createApi({ type: 'REST', labels: [labelA.id] });

            const put = await client
                .as('publisher')
                .putMultipart(`/apis/${api.id}`)
                .field('apiMetadata', JSON.stringify({
                    id: api.id,
                    name: api.name,
                    version: api.version,
                    type: 'REST',
                    status: 'PUBLISHED',
                    labels: [labelB.id],
                    endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
                }))
                .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
            expect(put.status).toBe(200);

            const get = await client.as('publisher').get(`/apis/${api.id}`);
            expect(get.body.labels).toEqual([labelB.id]);
        });

        it('keeps labels unchanged when the same labels are resent on update', async () => {
            const label = await createLabel();
            const api = await createApi({ type: 'REST', labels: [label.id] });

            const put = await client
                .as('publisher')
                .putMultipart(`/apis/${api.id}`)
                .field('apiMetadata', JSON.stringify({
                    id: api.id,
                    name: 'Same Labels Resent',
                    version: api.version,
                    type: 'REST',
                    status: 'PUBLISHED',
                    labels: [label.id],
                    endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
                }))
                .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
            expect(put.status).toBe(200);

            const get = await client.as('publisher').get(`/apis/${api.id}`);
            expect(get.body.labels).toEqual([label.id]);
        });

        it('adds labels on update to an API created without any', async () => {
            const label = await createLabel();
            const api = await createApi({ type: 'REST', labels: [] });

            const beforeGet = await client.as('publisher').get(`/apis/${api.id}`);
            expect(beforeGet.body.labels).toEqual([]);

            const put = await client
                .as('publisher')
                .putMultipart(`/apis/${api.id}`)
                .field('apiMetadata', JSON.stringify({
                    id: api.id,
                    name: api.name,
                    version: api.version,
                    type: 'REST',
                    status: 'PUBLISHED',
                    labels: [label.id],
                    endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
                }))
                .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
            expect(put.status).toBe(200);

            const get = await client.as('publisher').get(`/apis/${api.id}`);
            expect(get.body.labels).toEqual([label.id]);
        });

        it('removes all labels on update when an empty array is sent', async () => {
            const label = await createLabel();
            const api = await createApi({ type: 'REST', labels: [label.id] });

            const put = await client
                .as('publisher')
                .putMultipart(`/apis/${api.id}`)
                .field('apiMetadata', JSON.stringify({
                    id: api.id,
                    name: api.name,
                    version: api.version,
                    type: 'REST',
                    status: 'PUBLISHED',
                    labels: [],
                    endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
                }))
                .attach('apiDefinition', Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} })), 'definition.json');
            expect(put.status).toBe(200);

            const get = await client.as('publisher').get(`/apis/${api.id}`);
            expect(get.body.labels).toEqual([]);
        });
    });

    // apiViewQuery and apiSearchQuery are independently optional (docs/devportal-
    // openapi-spec-v0.9.yaml) — searchFallback used to crash with a 500 when `view`
    // was omitted (viewDao.getId querying `handle: undefined`), instead of searching
    // unscoped by view.
    it('searches APIs by free-text query without a view filter', async () => {
        const name = uniqueHandle('Searchable Query API');
        await createApi({ type: 'REST', name });

        const res = await client.as('publisher').get(`/apis?query=${encodeURIComponent(name)}`);
        expect(res.status).toBe(200);
        expect(res.body.list.some((a) => a.name === name)).toBe(true);
    });
});
