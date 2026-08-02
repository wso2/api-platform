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

// Where a request's effective scopes come from — auth.authorization.mode.
//
//   scope — the portal authorizes against the token's own scope claim.
//   role  — the portal IGNORES that claim and expands the token's roles claim
//           through its own grant table instead. This is the shipped default
//           (configs/config.toml).
//
// The whole suite runs once per mode (`make test-rest-api-scope` / `-role`), so every
// OTHER spec is already a per-mode assertion: they pass unchanged in both because the
// portal's IT grant table mirrors platform-api's. What this file adds is the part a
// mirrored table cannot show — that the two modes are genuinely different mechanisms
// rather than the same one under two names.
//
// That is what the `narrow` account is for. platform-api grants dp_narrow_it the full
// developer scope set, so its token's scope claim permits creating an application; the
// portal's table grants it read-only. One account, one token, opposite outcomes:
//
//   scope mode — create SUCCEEDS (the scope claim is honoured)
//   role  mode — create is REFUSED (the scope claim is ignored; the role decides)
//
// Neither assertion alone proves much. Together they prove the mode switch works, and
// that a caller cannot widen a role's grant by obtaining extra scopes from their
// issuer — the security property role mode exists for. Both run in exactly one mode,
// so each is meaningful where it runs rather than skipped as "not applicable".

const client = require('../support/client');

const MODE = client.AUTH_MODE;
const ORG = client.ORG_HANDLE;
const describeScopeMode = MODE === 'scope' ? describe : describe.skip;
const describeRoleMode = MODE === 'role' ? describe : describe.skip;

const uniq = (prefix) => `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

describe(`authorization mode = ${MODE}`, () => {
    beforeAll(async () => {
        await client.login('narrow');
        await client.login('admin');
    });

    describe('common to both modes', () => {
        it('runs against a portal in the mode this run was configured for', async () => {
            // Guards the whole file. API_PORTAL_AUTH_MODE (this process) and
            // APIP_AP_AUTH_AUTHORIZATION_MODE (the portal) are set from the same
            // AUTH_MODE by the compose fixture — if they ever drift, the mode-gated
            // describes below would silently skip or assert against the wrong mode.
            expect(['scope', 'role']).toContain(MODE);
            const res = await client.raw().get(`/${ORG}/views/default`);
            expect(res.status).toBe(200);
        });

        it('authorizes an admin operation for the admin account', async () => {
            // The same grant, reached by two different mechanisms depending on the
            // mode — mirrored tables are what make this hold either way.
            const id = uniq('authzmode-label');
            const res = await client.as('admin').post('/labels', { id, displayName: 'Mode label' });
            expect(res.status).toBe(201);
        });

        it('refuses an unauthenticated caller regardless of mode', async () => {
            const res = await client.raw().get(`${client.API_PREFIX}/organizations/${ORG}`);
            expect([401, 403]).toContain(res.status);
        });

        it('lets the narrow account read, in either mode', async () => {
            // Read is the one thing both its scope claim and its portal-side role
            // grant permit. Without this, the create assertions below could both be
            // explained by a broken session rather than by an authorization decision.
            const res = await client.as('narrow').get('/apis');
            expect(res.status).toBe(200);
        });
    });

    describeScopeMode('scope mode honours the token scope claim', () => {
        it("lets `narrow` create an application, because its scope claim allows it", async () => {
            // dp_narrow_it's scope claim carries dp:application:create. The portal's
            // own grant table says read-only, and in this mode that table is not
            // consulted at all — so the create succeeds.
            const res = await client.as('narrow').post('/applications', {
                displayName: uniq('scopemode-app'),
                description: 'Permitted by the scope claim',
            });
            expect(res.status).toBe(201);
        });
    });

    describeRoleMode('role mode ignores the token scope claim', () => {
        it("refuses `narrow` the same create, because its ROLE grants no application scope", async () => {
            // Same account, same token, same scope claim as the scope-mode case above.
            // Only the portal's interpretation differs. A 403 here is only meaningful
            // because the scope-mode run asserts 201 for the identical call.
            const res = await client.as('narrow').post('/applications', {
                displayName: uniq('rolemode-app'),
                description: 'Must be refused — the role grants no application scope',
            });
            expect(res.status).toBe(403);
        });

        it('refuses a key-manager read to the narrow role', async () => {
            const res = await client.as('narrow').get('/key-managers');
            expect(res.status).toBe(403);
        });

        it('expands a role into scopes rather than denying everything', async () => {
            // The counterweight to the two denials: role expansion is granting real
            // access, not failing closed across the board. Without it, both denials
            // above would also pass if role mode were simply broken.
            const res = await client.as('admin').get('/key-managers');
            expect(res.status).toBe(200);
        });
    });
});
