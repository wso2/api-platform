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

// Reference spec for the suite: fully implemented, used as the template for
// the other resources. Covers POST/GET/PUT/DELETE plus validation and a basic
// authorization check (unauthenticated access). Organization CRUD is the one
// place a fresh org-per-test still makes sense — admin can manage additional
// orgs beyond the fixed one every account is seeded into (org creation isn't
// scoped to the caller's own org).

const client = require('../support/client');
const { createOrganization, deleteOrganization, uniqueHandle } = require('../support/fixtures');

describe('organizations', () => {
    let org;

    beforeAll(async () => {
        await client.login('admin');
    });

    afterEach(async () => {
        if (org) {
            await deleteOrganization(org.id);
            org = undefined;
        }
    });

    it('creates and retrieves an organization', async () => {
        org = await createOrganization();

        const res = await client.as('admin').get(`/organizations/${org.id}`);
        expect(res.status).toBe(200);
        expect(res.body.id).toBe(org.id);
        expect(res.body.displayName).toBe(org.displayName);
    });

    it('updates an organization', async () => {
        org = await createOrganization();

        const res = await client.as('admin').put(`/organizations/${org.id}`, {
            id: org.id,
            idpRefId: org.id,
            displayName: 'Updated Display Name',
        });
        expect(res.status).toBe(200);
        expect(res.body.displayName).toBe('Updated Display Name');
    });

    it('deletes an organization', async () => {
        org = await createOrganization();

        const del = await client.as('admin').del(`/organizations/${org.id}`);
        expect(del.status).toBe(204);

        const get = await client.as('admin').get(`/organizations/${org.id}`);
        expect(get.status).toBe(404);

        org = undefined; // already deleted, skip afterEach cleanup
    });

    it('rejects creation with a missing required field', async () => {
        const res = await client.as('admin').post('/organizations', {
            id: uniqueHandle('org'),
            // displayName and idpRefId omitted
        });
        expect(res.status).toBe(400);
    });

    it('rejects creating a duplicate organization handle', async () => {
        org = await createOrganization();

        const res = await client.as('admin').post('/organizations', {
            id: org.id,
            displayName: 'Duplicate',
            idpRefId: org.id,
        });
        expect(res.status).toBe(409);
    });

    it('rejects requests without an authenticated session', async () => {
        const res = await client.raw().get(`${client.API_PREFIX}/organizations`);
        expect([401, 403]).toContain(res.status);
    });

    it("rejects a role without org management scope (developer can't create an org)", async () => {
        await client.login('developer');
        const res = await client.as('developer').post('/organizations', {
            id: uniqueHandle('org'),
            displayName: 'Should Be Forbidden',
            idpRefId: uniqueHandle('org'),
        });
        expect(res.status).toBe(403);
    });
});
