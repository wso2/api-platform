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

// POST/GET/PUT/DELETE /mcp-servers. Mirrors /apis exactly (same
// apiMetadataService.createAPIMetadata under the hood via asMcpRequest()) — see
// src/services/mcpServerService.js. `type` must be explicitly 'MCP' on both create and
// update; resolveTypeOrReject (apiMetadataService.js) rejects it being omitted or any
// other value, symmetric with /apis requiring an explicit non-MCP type.

const client = require('../support/client');
const { uniqueHandle, createApi } = require('../support/fixtures');

async function createMcpServer(overrides = {}) {
    const id = overrides.id || uniqueHandle('mcp-server');
    const metadata = {
        id,
        name: overrides.name || `Test MCP Server ${id}`,
        version: overrides.version || 'v1.0',
        type: 'MCP',
        status: overrides.status || 'PUBLISHED',
        endPoints: overrides.endPoints || {
            productionURL: `https://backend.example.invalid/${id}`,
            sandboxURL: `https://sandbox.example.invalid/${id}`,
        },
    };
    const res = await client
        .as('publisher')
        .postMultipart('/mcp-servers')
        .field('apiMetadata', JSON.stringify(metadata))
        .attach('apiDefinition', Buffer.from(JSON.stringify({ tools: [] })), 'definition.json');
    if (res.status !== 201) {
        throw new Error(`Failed to seed MCP server: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return res.body;
}

describe('MCP servers', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    it('creates and retrieves an MCP server', async () => {
        const mcp = await createMcpServer();
        const res = await client.as('publisher').get(`/mcp-servers/${mcp.id}`);
        expect(res.status).toBe(200);
        expect(res.body.type).toBe('Mcp');
    });

    it('rejects creating an MCP server when type is omitted', async () => {
        const id = uniqueHandle('mcp-server');
        const res = await client
            .as('publisher')
            .postMultipart('/mcp-servers')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'Omitted Type Test',
                version: 'v1.0',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ tools: [] })), 'definition.json');
        expect(res.status).toBe(400);
    });

    // Symmetric with the /apis rejection below ('rejects a /apis creation request whose
    // resolved type is MCP') — resolveTypeOrReject (apiMetadataService.js) now rejects an
    // explicit type mismatch on both sides instead of /mcp-servers silently overriding it.
    it('rejects a /mcp-servers creation request whose resolved type is not MCP', async () => {
        const id = uniqueHandle('should-use-apis-endpoint');
        const res = await client
            .as('publisher')
            .postMultipart('/mcp-servers')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'Wrong Endpoint',
                version: 'v1.0',
                type: 'REST',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ tools: [] })), 'definition.json');
        expect(res.status).toBe(400);
    });

    it('rejects updating an MCP server to a non-MCP type', async () => {
        const mcp = await createMcpServer();
        const put = await client
            .as('publisher')
            .putMultipart(`/mcp-servers/${mcp.id}`)
            .field('apiMetadata', JSON.stringify({
                name: 'Should Stay MCP',
                version: 'v1.0',
                type: 'REST',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ tools: [] })), 'definition.json');
        expect(put.status).toBe(400);
    });

    it('updates an MCP server', async () => {
        const mcp = await createMcpServer();
        const put = await client
            .as('publisher')
            .putMultipart(`/mcp-servers/${mcp.id}`)
            .field('apiMetadata', JSON.stringify({
                name: 'Updated MCP Server',
                version: 'v1.0',
                type: 'MCP',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ tools: [] })), 'definition.json');
        expect(put.status).toBe(200);
        expect(put.body.name).toBe('Updated MCP Server');
    });

    it('rejects updating an MCP server when type is omitted', async () => {
        const mcp = await createMcpServer();
        const put = await client
            .as('publisher')
            .putMultipart(`/mcp-servers/${mcp.id}`)
            .field('apiMetadata', JSON.stringify({
                name: 'Updated MCP Server',
                version: 'v1.0',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://updated.example.invalid', sandboxURL: 'https://updated-sandbox.example.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ tools: [] })), 'definition.json');
        expect(put.status).toBe(400);
    });

    it('deletes an MCP server', async () => {
        const mcp = await createMcpServer();
        const del = await client.as('publisher').del(`/mcp-servers/${mcp.id}`);
        expect(del.status).toBe(200);

        const get = await client.as('publisher').get(`/mcp-servers/${mcp.id}`);
        expect(get.status).toBe(404);
    });

    it('lists MCP servers', async () => {
        const name = uniqueHandle('Listable MCP Server');
        await createMcpServer({ name });

        const res = await client.as('publisher').get(`/mcp-servers?apiName=${encodeURIComponent(name)}`);
        expect(res.status).toBe(200);
        expect(res.body.list.some((m) => m.name === name)).toBe(true);
    });

    it('rejects a /apis creation request whose resolved type is MCP', async () => {
        const id = uniqueHandle('should-use-mcp-endpoint');
        const res = await client
            .as('publisher')
            .postMultipart('/apis')
            .field('apiMetadata', JSON.stringify({
                id,
                name: 'Wrong Endpoint',
                version: 'v1.0',
                type: 'MCP',
                status: 'PUBLISHED',
                endPoints: { productionURL: 'https://x.invalid', sandboxURL: 'https://x.invalid' },
            }))
            .attach('apiDefinition', Buffer.from(JSON.stringify({ tools: [] })), 'definition.json');
        expect(res.status).toBe(400);
    });

    it('generates an API key scoped to an MCP server', async () => {
        const mcp = await createMcpServer();
        const keyId = uniqueHandle('mcp-key').toLowerCase();
        const res = await client.as('publisher').post(`/mcp-servers/${mcp.id}/api-keys/generate`, { id: keyId });
        expect(res.status).toBe(201);
        expect(res.body.id).toBe(keyId);
        expect(res.body.key).toBeDefined();
    });

    // /mcp-servers and /apis share the same dp_api_metadata table, distinguished only
    // by `type` — resolveScopedApiId (apiMetadataService.js:313) is what's supposed to
    // keep the two families from resolving each other's handles. The tests above only
    // cover MCP-created-via-/apis being rejected; these cover the reverse (a plain REST
    // API resolved via /mcp-servers) and list isolation in both directions.
    describe('type isolation between /mcp-servers and /apis', () => {
        it('does not resolve a plain REST API handle via GET /mcp-servers/{id}', async () => {
            const api = await createApi();
            const res = await client.as('publisher').get(`/mcp-servers/${api.id}`);
            expect(res.status).toBe(404);
        });

        it('does not resolve an MCP server handle via GET /apis/{id}', async () => {
            const mcp = await createMcpServer();
            const res = await client.as('publisher').get(`/apis/${mcp.id}`);
            expect(res.status).toBe(404);
        });

        it('excludes plain REST APIs from the /mcp-servers list', async () => {
            const name = uniqueHandle('Should Not Appear In MCP List');
            await createApi({ name });

            const res = await client.as('publisher').get(`/mcp-servers?apiName=${encodeURIComponent(name)}`);
            expect(res.status).toBe(200);
            expect(res.body.list.some((m) => m.name === name)).toBe(false);
        });

        it('excludes MCP servers from the /apis list', async () => {
            const name = uniqueHandle('Should Not Appear In Apis List');
            await createMcpServer({ name });

            const res = await client.as('publisher').get(`/apis?apiName=${encodeURIComponent(name)}`);
            expect(res.status).toBe(200);
            expect(res.body.list.some((a) => a.name === name)).toBe(false);
        });

        // Same resolveApiId scoping (apiKeyController.js:49-62), but exercised via the
        // api-keys sub-resource rather than the parent record — a distinct code path
        // worth covering on its own since mcpServerKeysHandler's asMcpRequest aliasing
        // (mcpServerKeysHandler.js:32-36) is what's supposed to keep it scoped too.
        it('rejects GET /apis/{id}/api-keys when the handle belongs to an MCP server', async () => {
            const mcp = await createMcpServer();
            const res = await client.as('publisher').get(`/apis/${mcp.id}/api-keys`);
            expect(res.status).toBe(404);
        });

        it('rejects GET /mcp-servers/{id}/api-keys when the handle belongs to a plain REST API', async () => {
            const api = await createApi();
            const res = await client.as('publisher').get(`/mcp-servers/${api.id}/api-keys`);
            expect(res.status).toBe(404);
        });
    });
});
