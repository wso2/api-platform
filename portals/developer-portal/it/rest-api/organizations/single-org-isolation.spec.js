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

// The database is shared across organizations, so the organization named in a URL
// or a request header is untrusted input, not a selector. Each surface that used to
// accept one must reject anything but this instance's own organization:
//
//   page URLs            /{orgHandle}/...            -> 404   (src/middlewares/orgGuard.js)
//   `organization` header on an API-key request      -> 403   (src/middlewares/authMiddleware.js)
//
// These are all unauthenticated or API-key surfaces — the point is that no
// credential is needed to attempt them, so the rejection cannot depend on one.
// Token/session organization claims are covered by auth/file-based-login.spec.js.

const client = require('../support/client');

const OWN_ORG = client.ORG_HANDLE;
const FOREIGN_ORG = 'some-other-org';
const API_KEY_HEADER = 'x-wso2-api-key';
const API_KEY = process.env.API_PORTAL_API_KEY || 'api-portal-it-test-key';

describe('single-organization isolation', () => {
    describe('page routes', () => {
        it('serves its own organization portal home', async () => {
            const res = await client.raw().get(`/${OWN_ORG}/views/default`);
            expect(res.status).toBe(200);
        });

        it('404s a portal home under another organization handle', async () => {
            const res = await client.raw().get(`/${FOREIGN_ORG}/views/default`);
            expect(res.status).toBe(404);
        });

        it('404s an API listing under another organization handle', async () => {
            const res = await client.raw().get(`/${FOREIGN_ORG}/views/default/apis`);
            expect(res.status).toBe(404);
        });

        it('404s the MCP registry under another organization handle', async () => {
            const res = await client.raw().get(`/registry/${FOREIGN_ORG}/v0.1/servers`);
            expect(res.status).toBe(404);
        });

        it('does not name the rejected organization in the 404 body', async () => {
            // The error page's own links must point back into this portal, never into
            // the organization the caller asked for.
            const res = await client.raw().get(`/${FOREIGN_ORG}/views/default`);
            expect(res.status).toBe(404);
            expect(res.text || '').not.toContain(FOREIGN_ORG);
        });

        it('redirects the root to its own organization', async () => {
            const res = await client.raw().get('/').redirects(0);
            expect(res.status).toBe(302);
            expect(res.headers.location).toBe(`/${OWN_ORG}/views/default`);
        });
    });

    describe('organization request header', () => {
        it('scopes an API-key request to this organization when no header is sent', async () => {
            const res = await client.raw()
                .get(`${client.API_PREFIX}/apis`)
                .set(API_KEY_HEADER, API_KEY);
            expect(res.status).toBe(200);
        });

        it('accepts a header naming this organization', async () => {
            const res = await client.raw()
                .get(`${client.API_PREFIX}/apis`)
                .set(API_KEY_HEADER, API_KEY)
                .set('organization', OWN_ORG);
            expect(res.status).toBe(200);
        });

        it('rejects a header naming another organization with 403', async () => {
            // Honouring this header would make one API key able to address every
            // tenant in the shared database. Rejecting rather than ignoring it also
            // keeps a caller from believing it wrote to the organization it named.
            const res = await client.raw()
                .get(`${client.API_PREFIX}/apis`)
                .set(API_KEY_HEADER, API_KEY)
                .set('organization', FOREIGN_ORG);
            expect(res.status).toBe(403);
        });
    });

    describe('llms.txt advertises the configured organization', () => {
        it('names the organization instead of a placeholder', async () => {
            const res = await client.raw().get('/llms.txt');
            expect(res.status).toBe(200);
            expect(res.text).toContain(`/${OWN_ORG}/views/`);
            expect(res.text).not.toContain('{orgName}');
        });
    });
});
