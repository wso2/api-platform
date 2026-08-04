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
//
// The page surfaces are unauthenticated — the point is that no credential is
// needed to attempt them, so the rejection cannot depend on one.
//
// authMiddleware's `organization` header check (resolvePortalOrg -> 403) now only
// applies to mTLS, the sole remaining credential that carries no organization of
// its own; the static service API key that used to reach it was removed, and this
// fixture provisions no client certificates, so that path is not exercised here.
// For session and bearer credentials the organization comes from the credential's
// own claim (resolveScopedOrg) and the header is never a selector — asserted
// below, and covered further by auth/file-based-login.spec.js.

const client = require('../support/client');

const OWN_ORG = client.ORG_HANDLE;
const FOREIGN_ORG = 'some-other-org';

describe('single-organization isolation', () => {
    describe('page routes', () => {
        it('serves its own organization portal home', async () => {
            const res = await client.raw().get(`${client.BASE_PATH}/${OWN_ORG}/views/default`);
            expect(res.status).toBe(200);
        });

        it('404s a portal home under another organization handle', async () => {
            const res = await client.raw().get(`${client.BASE_PATH}/${FOREIGN_ORG}/views/default`);
            expect(res.status).toBe(404);
        });

        it('404s an API listing under another organization handle', async () => {
            const res = await client.raw().get(`${client.BASE_PATH}/${FOREIGN_ORG}/views/default/apis`);
            expect(res.status).toBe(404);
        });

        it('404s the MCP registry under another organization handle', async () => {
            const res = await client.raw().get(`${client.BASE_PATH}/registry/${FOREIGN_ORG}/v0.1/servers`);
            expect(res.status).toBe(404);
        });

        it('does not name the rejected organization in the 404 body', async () => {
            // The error page's own links must point back into this portal, never into
            // the organization the caller asked for.
            const res = await client.raw().get(`${client.BASE_PATH}/${FOREIGN_ORG}/views/default`);
            expect(res.status).toBe(404);
            expect(res.text || '').not.toContain(FOREIGN_ORG);
        });

        it('redirects the root to its own organization', async () => {
            const res = await client.raw().get(client.BASE_PATH).redirects(0);
            expect(res.status).toBe(302);
            expect(res.headers.location).toBe(`${client.BASE_PATH}/${OWN_ORG}/views/default`);
        });
    });

    describe('organization request header on a session credential', () => {
        beforeAll(async () => {
            await client.login('admin');
        });

        it('serves the caller organization when no header is sent', async () => {
            const res = await client.as('admin').get('/apis');
            expect(res.status).toBe(200);
        });

        it('serves the caller organization when the header names it', async () => {
            const res = await client.as('admin').get('/apis').set('organization', OWN_ORG);
            expect(res.status).toBe(200);
        });

        it('does not let the header redirect the request to another organization', async () => {
            // The organization comes from the session's own claim, so this header is
            // not a selector. Honouring it would let one credential address every
            // tenant in the shared database; the request must stay scoped to the
            // caller's organization rather than reaching the one it named.
            const res = await client.as('admin').get('/apis').set('organization', FOREIGN_ORG);
            expect(res.status).toBe(200);

            const own = await client.as('admin').get('/apis');
            expect(res.body).toEqual(own.body);
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
