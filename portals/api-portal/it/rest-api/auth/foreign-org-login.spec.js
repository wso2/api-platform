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

// Login-time organization mismatch, against the `api-portal-other-org` service in
// docker-compose.test*.yaml.
//
// The rest of this suite can't reach this branch: its portal and its
// platform-api are both configured for `default`, so a token's org_handle always
// matches the portal's organization.handle by construction. The second instance
// exists solely to break that equality — it is pinned to `other-org` while
// platform-api still mints tokens for `default`.
//
// What that isolates is important. admin/admin is a *valid* platform-api
// credential and the token that comes back is correctly signed, unexpired, and
// carries the right scopes. The only thing wrong with it is that it belongs to
// another organization. So a rejection here can't be a credential, signature, or
// clock failure — it is authController.handleLocalLogin's org_handle comparison
// (src/controllers/authController.js), which nothing else in this suite executes.
//
// The IDP-mode equivalent (passportConfig.js's assertLoginOrgAllowed) stays outside
// integration coverage — this fixture has no IDP, and standing up an OIDC provider
// for one branch isn't worth it. What it does instead is delegate the decision to
// orgContext.requirePinnedOrg, the same helper organizations.spec.js's 403 cases
// drive end-to-end, so the untested part is the thin translation into a login
// failure rather than a second copy of the rule.
//
// The complementary request-time check (authResolver rejecting a token whose org
// claim isn't this instance's) is covered by organizations/single-org-isolation.spec.js.

const supertest = require('supertest');
const client = require('../support/client');

const OTHER_ORG_BASE_URL = process.env.API_PORTAL_OTHER_ORG_BASE_URL;
const OTHER_ORG_HANDLE = process.env.API_PORTAL_OTHER_ORG_HANDLE || 'other-org';
const PLATFORM_API_ORG = client.ORG_HANDLE; // the org platform-api mints tokens for

// Skipped rather than failed when the fixture isn't running (e.g. a hand-rolled
// `docker compose up portal rest-api-tests`), so the suite stays usable
// outside the Makefile targets. Both CI matrix legs define the variable.
const describeOtherOrg = OTHER_ORG_BASE_URL ? describe : describe.skip;

const otherOrg = () => supertest(OTHER_ORG_BASE_URL);

function login(agent, orgHandle, username, password) {
    return agent
        .post(`/${orgHandle}/views/default/login`)
        .type('form')
        .send({ username, password })
        .redirects(0);
}

describeOtherOrg('login against a portal pinned to a different organization', () => {
    it('serves its own organization, proving the instance is up and seeded', async () => {
        // Establishes the baseline for everything below: this portal works, it has
        // created its own `other-org` organization, and it is reachable. A failure
        // here means the fixture is broken, not that a rejection under test misfired.
        const res = await otherOrg().get(`/${OTHER_ORG_HANDLE}/views/default`);
        expect(res.status).toBe(200);
    });

    it('404s the organization platform-api issues tokens for', async () => {
        // Same shared platform-api, but `default` is not this instance's organization.
        const res = await otherOrg().get(`/${PLATFORM_API_ORG}/views/default`);
        expect(res.status).toBe(404);
    });

    it('accepts the same credentials on the matching-organization portal', async () => {
        // The control. If this failed, a rejection below would prove nothing — the
        // credentials themselves, or platform-api, would be the explanation.
        const res = await login(client.raw(), PLATFORM_API_ORG, 'admin', 'admin');
        expect(res.status).toBe(302);
        expect(res.headers.location).not.toContain('error=');
    });

    it('rejects a valid credential whose token names another organization', async () => {
        const res = await login(otherOrg(), OTHER_ORG_HANDLE, 'admin', 'admin');
        expect(res.status).toBe(302);
        expect(res.headers.location).toContain('error=');
    });

    it('reports the mismatch as a plain credential failure', async () => {
        // Identical to the wrong-password message (file-based-login.spec.js). Whether
        // a given account belongs to this portal's organization is not something an
        // unauthenticated caller should be able to read off the response.
        const mismatch = await login(otherOrg(), OTHER_ORG_HANDLE, 'admin', 'admin');
        const wrongPassword = await login(otherOrg(), OTHER_ORG_HANDLE, 'admin', 'nope');
        expect(mismatch.headers.location).toBe(wrongPassword.headers.location);
        expect(mismatch.headers.location).toContain('Invalid+username+or+password');
    });

    it('leaves no usable session behind', async () => {
        // The redirect is the visible part; this is the part that matters. A session
        // established before the organization check would let every subsequent request
        // through on a foreign identity.
        const agent = supertest.agent(OTHER_ORG_BASE_URL);
        const res = await login(agent, OTHER_ORG_HANDLE, 'admin', 'admin');
        expect(res.headers.location).toContain('error=');

        const after = await agent.get(`${client.API_PREFIX}/organizations/${OTHER_ORG_HANDLE}`);
        expect([401, 403]).toContain(after.status);
    });
});
