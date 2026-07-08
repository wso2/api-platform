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

// POST/GET/PUT/DELETE /apis (type: GRAPHQL). Unlike REST/SOAP, GraphQL takes a
// `schemaDefinition` multipart field in addition to (or instead of) apiDefinition.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

const SAMPLE_SCHEMA = 'type Query { hello: String }';

async function createGraphQLApi(overrides = {}) {
    const id = overrides.id || uniqueHandle('graphql-api');
    const metadata = {
        id,
        name: overrides.name || `Test GraphQL API ${id}`,
        version: overrides.version || 'v1.0',
        type: 'GRAPHQL',
        status: overrides.status || 'PUBLISHED',
        endPoints: overrides.endPoints || {
            productionURL: `https://backend.example.invalid/${id}`,
            sandboxURL: `https://sandbox.example.invalid/${id}`,
        },
    };
    const res = await client
        .as('publisher')
        .postMultipart('/apis')
        .field('apiMetadata', JSON.stringify(metadata))
        .attach('schemaDefinition', Buffer.from(overrides.schema || SAMPLE_SCHEMA), 'schema.graphql');
    if (res.status !== 201) {
        throw new Error(`Failed to seed GraphQL API: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return res.body;
}

describe('GraphQL APIs', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    it('creates a GraphQL API with a schemaDefinition file', async () => {
        const api = await createGraphQLApi();
        const res = await client.as('publisher').get(`/apis/${api.id}`);
        expect(res.status).toBe(200);
        expect(res.body.type).toBe('GRAPHQL');
    });

    it('updates a GraphQL API and its schema', async () => {
        const api = await createGraphQLApi();
        const put = await client
            .as('publisher')
            .putMultipart(`/apis/${api.id}`)
            .field('apiMetadata', JSON.stringify({
                name: 'Updated GraphQL API',
                version: 'v1.0',
                type: 'GRAPHQL',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('schemaDefinition', Buffer.from('type Query { goodbye: String }'), 'schema.graphql');
        expect(put.status).toBe(200);
        expect(put.body.name).toBe('Updated GraphQL API');
    });

    it('deletes a GraphQL API', async () => {
        const api = await createGraphQLApi();
        const del = await client.as('publisher').del(`/apis/${api.id}`);
        expect(del.status).toBe(200);

        const get = await client.as('publisher').get(`/apis/${api.id}`);
        expect(get.status).toBe(404);
    });

    it('rejects a GraphQL API created without a schemaDefinition or apiDefinition', async () => {
        const id = uniqueHandle('graphql-api-no-schema');
        const res = await client
            .as('publisher')
            .postMultipart('/apis')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'No Schema API',
                version: 'v1.0',
                type: 'GRAPHQL',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }));
        expect(res.status).toBe(400);
    });

    it('retrieves the stored GraphQL schema', async () => {
        // The uploaded file's own name ('schema.graphql') isn't what's persisted — the
        // schema is always stored under the fixed name apiDefinition.graphql and
        // type=API_DEFINITION (apiMetadataService.js's createAPIMetadata, using
        // constants.FILE_NAME.API_DEFINITION_GRAPHQL), same category REST/SOAP
        // definitions use.
        const schema = 'type Query { hello: String }';
        const api = await createGraphQLApi({ schema });
        const res = await client.as('publisher').get(`/apis/${api.id}/assets?type=API_DEFINITION&fileName=apiDefinition.graphql`);
        expect(res.status).toBe(200);
        // .graphql isn't in util.isTextFile's allowlist, so it's served with a generic
        // binary content-type rather than text/plain — read whichever field supertest
        // populated for that content-type.
        const body = res.text !== undefined ? res.text : Buffer.from(res.body).toString('utf8');
        expect(body).toBe(schema);
    });

    it('returns 404 (not a hang) for a schema file that was never uploaded', async () => {
        const id = uniqueHandle('graphql-api-no-file');
        const res = await client
            .as('publisher')
            .postMultipart('/apis')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'No File API',
                version: 'v1.0',
                type: 'GRAPHQL',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }))
            .attach('schemaDefinition', Buffer.from('type Query { hello: String }'), 'schema.graphql');
        expect(res.status).toBe(201);

        const get = await client.as('publisher').get(`/apis/${res.body.id}/assets?type=API_DEFINITION&fileName=does-not-exist.graphql`);
        expect(get.status).toBe(404);
    });
});
