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

// The portal serves the single organization named by its organization.handle
// config, created on startup by the seeder. So the organization lifecycle is not
// client-driven: create/list/delete answer 405, and read/update are pinned to that
// one organization. The operations remain in the spec (and their service functions
// intact) so they can be re-enabled later, hence 405 rather than a missing route.
//
// Cross-surface rejection of a *foreign* organization — page URLs, the
// `organization` request header — lives in single-org-isolation.spec.js.

const client = require('../support/client');

const OWN_ORG = client.ORG_HANDLE;

describe('organizations', () => {
    beforeAll(async () => {
        await client.login('admin');
    });

    describe('lifecycle operations are not offered', () => {
        it('rejects creating an organization with 405', async () => {
            const res = await client.as('admin').post('/organizations', {
                id: 'some-new-org',
                displayName: 'Some New Org',
                idpRefId: 'some-new-org',
            });
            expect(res.status).toBe(405);
            expect(res.body.code).toBe('METHOD_NOT_ALLOWED');
        });

        it('rejects listing organizations with 405', async () => {
            const res = await client.as('admin').get('/organizations');
            expect(res.status).toBe(405);
            expect(res.body.code).toBe('METHOD_NOT_ALLOWED');
        });

        it('rejects deleting its own organization with 405', async () => {
            const res = await client.as('admin').del(`/organizations/${OWN_ORG}`);
            expect(res.status).toBe(405);
            expect(res.body.code).toBe('METHOD_NOT_ALLOWED');

            // Still there — the 405 is a refusal, not a silent no-op.
            const get = await client.as('admin').get(`/organizations/${OWN_ORG}`);
            expect(get.status).toBe(200);
        });
    });

    describe('read and update are scoped to this instance', () => {
        it('retrieves its own organization', async () => {
            const res = await client.as('admin').get(`/organizations/${OWN_ORG}`);
            expect(res.status).toBe(200);
            expect(res.body.id).toBe(OWN_ORG);
        });

        it('updates its own organization', async () => {
            const displayName = `Updated Display Name ${Date.now()}`;
            const res = await client.as('admin').put(`/organizations/${OWN_ORG}`, {
                id: OWN_ORG,
                idpRefId: OWN_ORG,
                displayName,
            });
            expect(res.status).toBe(200);
            expect(res.body.displayName).toBe(displayName);
        });

        it('rejects reading another organization with 403, not 404', async () => {
            // 403 rather than 404 on purpose: an unknown handle and a real-but-foreign
            // one must be indistinguishable, or the response becomes a way to
            // enumerate the organizations sharing this database.
            const res = await client.as('admin').get('/organizations/some-other-org');
            expect(res.status).toBe(403);
        });

        it('rejects updating another organization with 403', async () => {
            const res = await client.as('admin').put('/organizations/some-other-org', {
                id: 'some-other-org',
                idpRefId: 'some-other-org',
                displayName: 'Hijacked',
            });
            expect(res.status).toBe(403);
        });
    });

    describe('identity fields are immutable', () => {
        // The handle and idp_ref_id are what page URLs and incoming token
        // organization claims are matched against. Renaming either would leave the
        // running instance unable to find its own organization — every page 404ing
        // and every login 403ing until config was edited to match.
        it('rejects changing the organization handle', async () => {
            const res = await client.as('admin').put(`/organizations/${OWN_ORG}`, {
                id: 'renamed-org',
                idpRefId: OWN_ORG,
                displayName: 'Renamed',
            });
            expect(res.status).toBe(400);

            // The rename didn't partially apply.
            const get = await client.as('admin').get(`/organizations/${OWN_ORG}`);
            expect(get.status).toBe(200);
            expect(get.body.id).toBe(OWN_ORG);
        });

        it('rejects changing the organization IDP reference', async () => {
            const res = await client.as('admin').put(`/organizations/${OWN_ORG}`, {
                id: OWN_ORG,
                idpRefId: 'some-other-idp-ref',
                displayName: 'Re-pointed',
            });
            expect(res.status).toBe(400);
        });
    });

    describe('authentication and authorization still run first', () => {
        // The 405 lives in the operation handler, which the OpenAPI validator only
        // reaches after the security handlers pass — so it must never be the reason
        // an unauthenticated or under-scoped caller gets turned away.
        it('rejects requests without an authenticated session', async () => {
            const res = await client.raw().get(`${client.API_PREFIX}/organizations`);
            expect([401, 403]).toContain(res.status);
        });

        it("rejects a role without org management scope (developer can't create an org)", async () => {
            await client.login('developer');
            const res = await client.as('developer').post('/organizations', {
                id: 'developer-org',
                displayName: 'Should Be Forbidden',
                idpRefId: 'developer-org',
            });
            expect(res.status).toBe(403);
        });
    });
});
