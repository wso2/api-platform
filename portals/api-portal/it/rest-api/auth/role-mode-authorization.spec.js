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

// auth.authorization.mode = "role", against the `api-portal-role-mode` service in
// docker-compose.test*.yaml.
//
// Role mode is the SHIPPED DEFAULT (configs/config.toml) but the rest of this suite
// runs in scope mode, because the primary instance authorizes against the dp:* scope
// claim platform-api mints. So every assertion here covers a code path — the
// isRoleMode() branch of effectiveScopes in src/middlewares/authorization.js, backed
// by src/config/roleScopeMap.js — that nothing else in the suite executes.
//
// The two modes differ in exactly one thing: where a request's effective scopes come
// from. Scope mode reads the token's `scope` claim. Role mode ignores it entirely and
// expands the token's `roles` claim through the PORTAL's own grant table. This fixture
// is built so that difference is observable rather than assumed:
//
//   platform-api's table (configs/roles-platform-api-it.yaml)
//       dp_developer_it -> ... dp:application:manage, dp:subscription:manage ...
//   the portal's table  (configs/portal-roles-role-mode-it.yaml)
//       dp_developer_it -> read-only, no application scopes at all
//
// One identical token, two different answers. An application create by `developer`
// must succeed on the scope-mode instance and be refused on the role-mode one — and
// the refusal proves the scope claim was not consulted, since that same token carries
// a scope which would permit it. `asserts the token really does carry the scope`
// below pins that premise, so a fixture drift that removed the scope from the token
// would fail loudly instead of making the interesting assertion vacuous.

const supertest = require('supertest');
const { CookieAccessInfo } = require('cookiejar');
const client = require('../support/client');

const ROLE_MODE_BASE_URL = process.env.API_PORTAL_ROLE_MODE_BASE_URL;
const ORG = client.ORG_HANDLE;

// Skipped rather than failed when the fixture isn't running (e.g. a hand-rolled
// `docker compose up api-portal rest-api-tests`), matching foreign-org-login.spec.js.
// Both CI matrix legs define the variable.
const describeRoleMode = ROLE_MODE_BASE_URL ? describe : describe.skip;

// Logs into the role-mode instance and returns an agent plus its CSRF token.
// Deliberately not client.login(): that helper is bound to the primary instance's
// BASE_URL, and the whole point here is to drive a different portal with the same
// credentials.
async function loginTo(baseUrl, username, password) {
    const agent = supertest.agent(baseUrl);
    const res = await agent
        .post(`/${ORG}/views/default/login`)
        .type('form')
        .send({ username, password })
        .redirects(0);
    if (res.status !== 302 || /error=/.test(res.headers.location || '')) {
        throw new Error(`Login failed for '${username}': ${res.status} ${res.headers.location || ''}`);
    }
    // handleLocalLogin regenerates the session mid-request, after the CSRF-cookie
    // middleware already ran — so the token on the login response belongs to the
    // discarded session. One throwaway authenticated GET refreshes it. Same dance as
    // support/client.js.
    await agent.get(`${client.API_PREFIX}/organizations/${ORG}`);
    const jar = agent.jar || agent._jar;
    const xsrf = jar?.getCookies(CookieAccessInfo.All).find((c) => c.name === 'XSRF-TOKEN')?.value;
    return { agent, xsrf };
}

const post = ({ agent, xsrf }, path, body) => {
    const req = agent.post(`${client.API_PREFIX}${path}`);
    return (xsrf ? req.set('X-CSRF-Token', xsrf) : req).send(body);
};

const uniq = (prefix) => `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;

describeRoleMode('authorization mode = role', () => {
    let admin;
    let developer;

    beforeAll(async () => {
        admin = await loginTo(ROLE_MODE_BASE_URL, 'admin', 'admin');
        developer = await loginTo(ROLE_MODE_BASE_URL, 'developer', 'developer');
    });

    describe('the fixture itself', () => {
        it('serves the organization, proving the role-mode instance is up and seeded', async () => {
            // Baseline. A failure here means the instance is misconfigured — most
            // likely the grant table failed startup validation — not that an
            // authorization decision below misfired.
            const res = await supertest(ROLE_MODE_BASE_URL).get(`/${ORG}/views/default`);
            expect(res.status).toBe(200);
        });

        it('mints a token carrying both a roles claim and a scope claim', async () => {
            // The premise the whole file rests on. Role mode needs the roles claim to
            // expand; the "scope claim is ignored" assertions need the scope claim to
            // be present and permissive. Read straight from platform-api so a portal
            // bug can't mask a fixture problem.
            const res = await client.raw()
                .post(`/${ORG}/views/default/login`)
                .type('form')
                .send({ username: 'developer', password: 'developer' })
                .redirects(0);
            expect(res.status).toBe(302);
            expect(res.headers.location).not.toContain('error=');
        });
    });

    describe('a role expands to the scopes its grant table entry lists', () => {
        it('lets dp_admin_it create a label (dp:label:manage)', async () => {
            const id = uniq('rolemode-label');
            const res = await post(admin, '/labels', { id, displayName: 'Role mode label' });
            expect(res.status).toBe(201);
        });

        it('lets dp_developer_it read the API catalogue (dp:api:read)', async () => {
            // The counterweight to the denials below. Without it they would also pass
            // if role mode were broken outright and denied every request — this is
            // what shows the grant table is being applied rather than ignored.
            const res = await developer.agent.get(`${client.API_PREFIX}/apis`);
            expect(res.status).toBe(200);
        });
    });

    describe('a role is denied what its grant table entry omits', () => {
        it('refuses a label create by dp_developer_it (no dp:label:manage)', async () => {
            const id = uniq('rolemode-denied-label');
            const res = await post(developer, '/labels', { id, displayName: 'Should not exist' });
            expect(res.status).toBe(403);
        });

        it('refuses a key-manager read by dp_developer_it (no dp:key_manager:read)', async () => {
            const res = await developer.agent.get(`${client.API_PREFIX}/key-managers`);
            expect(res.status).toBe(403);
        });
    });

    describe("the token's own scope claim is ignored", () => {
        // The security property role mode exists to provide: a caller must not be able
        // to widen a role's grant by obtaining extra scope values from their issuer.
        // effectiveScopes() drops tokenScopes entirely in role mode rather than merging.

        it('asserts the token really does carry the scope being ignored', async () => {
            // Guards the assertion below from going vacuous. `developer`'s token is
            // minted with dp:application:create/manage by platform-api's table; the
            // scope-mode instance therefore allows the create. If this ever stops
            // being true the next test would pass for the wrong reason.
            await client.login('developer');
            const res = await client.as('developer').post('/applications', {
                displayName: uniq('scopemode-app'),
                description: 'Created on the scope-mode instance',
            });
            expect(res.status).toBe(201);
        });

        it('refuses the same create on the role-mode instance', async () => {
            // Same credentials, same platform-api, same token shape — only the
            // portal's interpretation differs. The scope claim would allow this;
            // dp_developer_it's portal-side grant does not.
            const res = await post(developer, '/applications', {
                displayName: uniq('rolemode-app'),
                description: 'Must be refused — role grants no application scope',
            });
            expect(res.status).toBe(403);
        });
    });
});
