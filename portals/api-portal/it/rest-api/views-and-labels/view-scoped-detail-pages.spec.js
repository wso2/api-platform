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

// A view-scoped PAGE URL must serve only artifacts that view includes.
//
// /{org}/views/{view}/api/{handle} used to resolve the artifact on handle + org_uuid
// alone (apiDao.getId), so the view segment was decoration: any published API or MCP
// server in the organization rendered under any view's URL, and unpublished ones too.
// Membership for APIs/MCP servers is a labels join (api_label_mappings →
// view_label_mappings) rather than a column, so it has to be asked for explicitly —
// apiDao.getIdInView now applies the same predicates GET /apis?view= applies.
//
// view-label-visibility.spec.js covers the LIST side of the same mapping. This is the
// single-artifact side, which had no coverage at all.

const client = require('../support/client');
const { createApi, createView, uniqueHandle } = require('../support/fixtures');

async function createLabel() {
    const id = uniqueHandle('label');
    const res = await client.as('admin').post('/labels', { id, displayName: id });
    if (res.status !== 201) {
        throw new Error(`Failed to seed label: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return res.body;
}

// The portal pages are HTML, not the REST API, so they're fetched with the raw agent
// against the page URL rather than through client.as(...).
function pageUrl(viewHandle, artifactPath) {
    return `${client.BASE_PATH}/${client.ORG_HANDLE}/views/${viewHandle}${artifactPath}`;
}

// MCP servers have their own resource family and require a tools schema as the
// definition — mirrors createMcpServer in mcp-servers/mcp-servers.spec.js, with labels
// added so the artifact can be placed in a view.
const MCP_TOOLS_SCHEMA = [
    '- type: TOOL',
    '  name: ping',
    '  description: Health check tool.',
    '  inputSchema:',
    '    type: object',
    '    properties: {}',
].join('\n');

async function createMcpServer(labels) {
    const id = uniqueHandle('mcp-server');
    const metadata = {
        id,
        name: `Test MCP Server ${id}`,
        version: 'v1.0',
        type: 'MCP',
        status: 'PUBLISHED',
        labels,
        endPoints: {
            productionURL: `https://backend.example.invalid/${id}`,
            sandboxURL: `https://sandbox.example.invalid/${id}`,
        },
    };
    const res = await client
        .as('publisher')
        .postMultipart('/mcp-servers')
        .field('metadata', JSON.stringify(metadata))
        .attach('definition', Buffer.from(MCP_TOOLS_SCHEMA), 'definition.yaml');
    if (res.status !== 201) {
        throw new Error(`Failed to seed MCP server: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return res.body;
}

describe('view-scoped detail pages', () => {
    let labelIn;
    let viewIn;
    let viewOut;
    let restApi;
    let mcpServer;

    beforeAll(async () => {
        await client.login('admin');
        await client.login('publisher');

        labelIn = await createLabel();
        const labelOut = await createLabel();
        // viewIn carries the artifacts' label; viewOut deliberately carries a different
        // one, so both views exist and only one includes the artifacts.
        viewIn = await createView({ labels: [labelIn.id] });
        viewOut = await createView({ labels: [labelOut.id] });

        restApi = await createApi({ labels: [labelIn.id] });
        mcpServer = await createMcpServer([labelIn.id]);
    });

    it('serves an API detail page in a view that includes it', async () => {
        const res = await client.raw().get(pageUrl(viewIn.id, `/api/${restApi.id}`));
        expect(res.status).toBe(200);
    });

    it('404s the API detail page in a view that excludes it', async () => {
        const res = await client.raw().get(pageUrl(viewOut.id, `/api/${restApi.id}`));
        expect(res.status).toBe(404);
    });

    it('serves an MCP server detail page in a view that includes it', async () => {
        const res = await client.raw().get(pageUrl(viewIn.id, `/mcp/${mcpServer.id}`));
        expect(res.status).toBe(200);
    });

    it('404s the MCP server detail page in a view that excludes it', async () => {
        const res = await client.raw().get(pageUrl(viewOut.id, `/mcp/${mcpServer.id}`));
        expect(res.status).toBe(404);
    });

    it('404s the agent-facing markdown for an API outside the view', async () => {
        const included = await client.raw().get(pageUrl(viewIn.id, `/api/${restApi.id}.md`));
        expect(included.status).toBe(200);

        const excluded = await client.raw().get(pageUrl(viewOut.id, `/api/${restApi.id}.md`));
        expect(excluded.status).toBe(404);
    });

    it('404s the raw specification for an API outside the view', async () => {
        // The spec download hangs off the same view-scoped URL, so it must not be a way
        // around the page check.
        const excluded = await client.raw()
            .get(pageUrl(viewOut.id, `/api/${restApi.id}/docs/specification.json`));
        expect(excluded.status).toBe(404);
    });

    it('404s an unknown handle in a valid view', async () => {
        // Same answer as an artifact that exists but is out of view — the two cases must
        // not be distinguishable, or the response becomes a way to enumerate other views'
        // artifacts.
        const res = await client.raw().get(pageUrl(viewIn.id, `/api/${uniqueHandle('api-absent')}`));
        expect(res.status).toBe(404);
    });

    it('keeps hiding an artifact after its label is removed from the view', async () => {
        const label = await createLabel();
        const view = await createView({ labels: [label.id] });
        const api = await createApi({ labels: [label.id] });

        expect((await client.raw().get(pageUrl(view.id, `/api/${api.id}`))).status).toBe(200);

        // Detach every label from the view — the same operation
        // view-label-visibility.spec.js checks on the list side.
        expect((await client.as('admin').put(`/views/${view.id}`, { labels: [] })).status).toBe(200);

        expect((await client.raw().get(pageUrl(view.id, `/api/${api.id}`))).status).toBe(404);
    });
});
