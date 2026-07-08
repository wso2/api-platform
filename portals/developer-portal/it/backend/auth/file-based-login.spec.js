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

// "File-based" login (config.identityProvider unset) is NOT a local credential
// store inside devportal — POST /:orgName/views/:viewName/login
// (src/controllers/authController.js handleLocalLogin) proxies the username/
// password to Platform API's own file-based auth (POST /api/portal/v0.9/auth/login,
// see it/configs/config-platform-api-it.toml for the seeded admin/publisher/
// developer accounts) and decodes the returned JWT into the devportal session.
// This is the same login flow support/client.js's login() uses for every other
// spec in this suite — here it's exercised directly to check the HTTP-level
// contract (redirects, cookies, error responses) rather than just using it as
// plumbing.

const client = require('../support/client');

describe('file-based (demo mode) login', () => {
    it('logs in with valid credentials and establishes a session', async () => {
        const agent = require('supertest').agent(client.BASE_URL);
        const res = await agent
            .post(`/${client.ORG_HANDLE}/views/default/login`)
            .type('form')
            .send({ username: 'admin', password: 'admin' })
            .redirects(0);
        expect(res.status).toBe(302);
        expect(res.headers.location).not.toContain('error=');
        expect(res.headers['set-cookie'].some((c) => c.startsWith('connect.sid='))).toBe(true);
    });

    it('rejects an incorrect password', async () => {
        const res = await client.raw()
            .post(`/${client.ORG_HANDLE}/views/default/login`)
            .type('form')
            .send({ username: 'admin', password: 'wrong-password' })
            .redirects(0);
        expect(res.status).toBe(302);
        expect(res.headers.location).toContain('error=');
    });

    it('rejects a non-existent username', async () => {
        const res = await client.raw()
            .post(`/${client.ORG_HANDLE}/views/default/login`)
            .type('form')
            .send({ username: 'no-such-user', password: 'whatever' })
            .redirects(0);
        expect(res.status).toBe(302);
        expect(res.headers.location).toContain('error=');
    });

    it('rejects when username or password is missing', async () => {
        const res = await client.raw()
            .post(`/${client.ORG_HANDLE}/views/default/login`)
            .type('form')
            .send({ username: 'admin' })
            .redirects(0);
        expect(res.status).toBe(302);
        expect(res.headers.location).toContain('Username+and+password+are+required');
    });

    it('session cookie grants access to an authenticated-only endpoint afterward', async () => {
        await client.login('admin');
        const res = await client.as('admin').get(`/organizations/${client.ORG_HANDLE}`);
        expect(res.status).toBe(200);
        expect(res.body.id).toBe(client.ORG_HANDLE);
    });

    it('rejects requests without an authenticated session', async () => {
        const res = await client.raw().get(`${client.API_PREFIX}/organizations/${client.ORG_HANDLE}`);
        expect([401, 403]).toContain(res.status);
    });
});
