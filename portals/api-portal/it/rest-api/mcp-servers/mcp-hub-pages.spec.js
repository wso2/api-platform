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

// Image URLs and landing text on the portal pages (util.apiImageUrl): MCP servers' images
// come from /mcp-servers, APIs' from /apis, both with orgId so visitors with no session
// can load them.

const client = require('../support/client');
const { uniqueHandle, createApi } = require('../support/fixtures');
const { createZip } = require('../support/zipBuilder');

const PNG_BYTES = Buffer.from(
    '89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c489' +
    '0000000d49444154789c63606060f80f00010401005fe5c34b0000000049454e44ae426082',
    'hex',
);
const MCP_TOOLS_SCHEMA = [
    '- type: TOOL',
    '  name: ping',
    '  description: Health check tool.',
    '  inputSchema:',
    '    type: object',
    '    properties: {}',
].join('\n');
const API_KEY_DEFINITION = JSON.stringify({
    openapi: '3.0.3',
    info: { title: 'Keyed API', version: '1.0.0' },
    components: { securitySchemes: { ApiKeyAuth: { type: 'apiKey', in: 'header', name: 'apikey' } } },
    security: [{ ApiKeyAuth: [] }],
    paths: { '/ping': { get: { responses: { 200: { description: 'ok' } } } } },
});
// A docs/ entry too: on main a ZIP holding only web/ is taken for a wrapper folder and rejected.
const CONTENT = [
    { name: 'web/api-icon.png', content: PNG_BYTES },
    { name: 'web/apiContent.md', content: '# Weather MCP landing page' },
    { name: 'docs/Guide.md', content: '# Guide' },
];

const viewPath = (path) => `${client.BASE_PATH}/${client.ORG_HANDLE}/views/default${path}`;
// Handlebars escapes & and = in attributes.
const html = (res) => res.text.replace(/&#x3D;/g, '=').replace(/&amp;/g, '&');
const imageUrl = (base, id) => `${base}/${id}/assets?type=IMAGE&fileName=api-icon.png&orgId=`;

async function uploadContent(base, id) {
    const res = await client.as('publisher').putMultipart(`${base}/${id}/assets`)
        .attach('content', createZip(CONTENT), 'content.zip');
    expect(res.ok).toBe(true);
}

describe('Image URLs and landing text on the portal pages', () => {
    let mcpId;

    beforeAll(async () => {
        await client.login('publisher');
        mcpId = uniqueHandle('mcp-page');
        const res = await client.as('publisher').postMultipart('/mcp-servers')
            .field('metadata', JSON.stringify({
                id: mcpId,
                name: `Page MCP ${mcpId}`,
                version: 'v1.0',
                type: 'MCP',
                status: 'PUBLISHED',
                labels: ['default'],
                endPoints: { productionURL: `https://backend.example.invalid/${mcpId}` },
            }))
            .attach('definition', Buffer.from(MCP_TOOLS_SCHEMA), 'definition.yaml');
        expect(res.status).toBe(201);
        await uploadContent('/mcp-servers', mcpId);
    });

    it('the MCP Hub listing loads the thumbnail from /mcp-servers, without a session', async () => {
        const res = await client.raw().get(viewPath('/mcps'));
        expect(res.status).toBe(200);
        expect(html(res)).toContain(imageUrl('/mcp-servers', mcpId));
    });

    it('the MCP server page loads the thumbnail from /mcp-servers and shows the landing text', async () => {
        const res = await client.raw().get(viewPath(`/mcp/${mcpId}`));
        expect(res.status).toBe(200);
        expect(html(res)).toContain(imageUrl('/mcp-servers', mcpId));
        expect(res.text).toContain('Weather MCP landing page');
    });

    it('the MCP server image is readable without a session, but its other content is not', async () => {
        const anonymous = (query) => client.raw()
            .get(`${client.API_PREFIX}/mcp-servers/${mcpId}/assets?${query}`)
            .responseType('blob');
        for (const query of ['type=IMAGE&fileName=api-icon.png', 'type=IMAGE&fileName=api-icon.png&orgId=any']) {
            const img = await anonymous(query);
            expect(img.status).toBe(200);
            expect(Buffer.compare(img.body, PNG_BYTES)).toBe(0);
        }
        expect((await anonymous('type=DOC_Other&fileName=Guide.md')).status).toBe(401);
    });

    it('an API page still loads the thumbnail from /apis, and its API keys page renders', async () => {
        const api = await createApi({ labels: ['default'], definition: API_KEY_DEFINITION });
        await uploadContent('/apis', api.id);

        const page = await client.raw().get(viewPath(`/api/${api.id}`));
        expect(page.status).toBe(200);
        expect(html(page)).toContain(imageUrl('/apis', api.id));

        // The API keys page builds the same image URLs; it needs a session.
        const keys = await client.page('publisher').get(`/${client.ORG_HANDLE}/views/default/api/${api.id}/api-keys`);
        expect(keys.status).toBe(200);
        expect(keys.text).toContain(api.name);
    });
});
