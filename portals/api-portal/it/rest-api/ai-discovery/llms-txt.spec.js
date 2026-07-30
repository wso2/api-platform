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

// GET /llms.txt (portal-wide) and GET /:orgName/views/:viewName/llms.txt
// (org/view-specific discovery index) — src/app.js, src/controllers/apiContentController.js
// loadLlmsTxt. Neither is under /api/v0.9 nor session-authenticated (public
// agent-discovery routes) — use client.raw() rather than client.as(role).

const client = require('../support/client');
const { createApi } = require('../support/fixtures');

describe('AI/LLM discovery (llms.txt)', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    it('GET /llms.txt returns the portal-wide discovery index as text/plain', async () => {
        const res = await client.raw().get('/llms.txt');
        expect(res.status).toBe(200);
        expect(res.headers['content-type']).toMatch(/text\/plain/);
        expect(res.text).toContain('AI Agent Entry Point');
    });

    it("GET /:org/views/:view/llms.txt lists that view's visible APIs and MCP servers", async () => {
        const api = await createApi({ name: 'Discoverable API', labels: ['default'] });

        const res = await client.raw().get(`/${client.ORG_HANDLE}/views/default/llms.txt`);
        expect(res.status).toBe(200);
        expect(res.headers['content-type']).toMatch(/text\/plain/);
        expect(res.text).toContain(api.name);
    });

    it('excludes APIs with agentVisibility HIDDEN from the discovery index', async () => {
        const visible = await createApi({ name: 'Visible Discoverable API', labels: ['default'], agentVisibility: 'VISIBLE' });
        const hidden = await createApi({ name: 'Hidden Discoverable API', labels: ['default'], agentVisibility: 'HIDDEN' });

        const res = await client.raw().get(`/${client.ORG_HANDLE}/views/default/llms.txt`);
        expect(res.status).toBe(200);
        expect(res.text).toContain(visible.name);
        expect(res.text).not.toContain(hidden.name);
    });

    // aiEnabled is toggled via the session-authenticated settings page route
    // (src/routes/pages/settingsRoute.js — ensureAuthenticated + CSRF), not the
    // REST API this suite otherwise exercises — client.page(role) reuses the same
    // logged-in session/CSRF token as client.as(role), just without the /api/v0.9
    // prefix, so this stays in the Jest suite instead of needing Cypress.
    it('returns 404 when aiEnabled is false for the org', async () => {
        const configPath = `/${client.ORG_HANDLE}/views/default/llms-config`;
        try {
            const save = await client.page('publisher').put(configPath, { aiEnabled: false, portalName: '', portalDescription: '' });
            expect(save.status).toBe(200);

            const res = await client.raw().get(`/${client.ORG_HANDLE}/views/default/llms.txt`);
            expect(res.status).toBe(404);
        } finally {
            // Restore — this view/org is shared by every other test in this file.
            // Inside finally so a failed/non-200 disable PUT can't leave aiEnabled off.
            await client.page('publisher').put(configPath, { aiEnabled: true, portalName: '', portalDescription: '' });
        }
    });
});
