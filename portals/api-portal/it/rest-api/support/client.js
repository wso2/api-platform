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

// Real session auth against the file-based Platform API accounts configured in
// it/configs/config-platform-api-it.toml (admin / publisher / developer,
// password == username) — not the api-portal-it-test-key API-key bypass. Every
// role is locked to the single "default" org that account was seeded into
// (file-based auth supports exactly one org), so all fixtures/specs share that
// org and rely on uniqueHandle() for per-test resource isolation instead of a
// fresh org per test.
//
// Usage: `await client.login('publisher')` once (e.g. in beforeAll), then
// `client.as('publisher').post(path, body)` synchronously anywhere after —
// this two-step split matters for postMultipart/putMultipart, whose callers
// need to chain .field()/.attach() before the request is awaited; an async
// wrapper around those would auto-adopt (and fire) the returned supertest
// Request as soon as the function returns, before the chaining happens.

const supertest = require('supertest');
const { autoTrackFromResponse } = require('./cleanup');

const BASE_URL = process.env.API_PORTAL_BASE_URL || 'http://localhost:9543';
// The whole portal (pages + REST API) is mounted under this hardcoded prefix
// (src/utils/constants.js ROUTE.BASE_PATH). Every request the suite makes carries it.
const BASE_PATH = '/api-portal';
const API_PREFIX = `${BASE_PATH}/api/v0.9`;
const ORG_HANDLE = process.env.API_PORTAL_ORG_HANDLE || 'default';

const CREDENTIALS = {
    admin: { username: process.env.API_PORTAL_ADMIN_USERNAME || 'admin', password: process.env.API_PORTAL_ADMIN_PASSWORD || 'admin' },
    publisher: { username: process.env.API_PORTAL_PUBLISHER_USERNAME || 'publisher', password: process.env.API_PORTAL_PUBLISHER_PASSWORD || 'publisher' },
    developer: { username: process.env.API_PORTAL_DEVELOPER_USERNAME || 'developer', password: process.env.API_PORTAL_DEVELOPER_PASSWORD || 'developer' },
    // Used by auth/authorization-mode.spec.js only. Its portal-side grant is
    // deliberately narrower than its token's scope claim, which is what makes the
    // scope-mode/role-mode difference observable. Don't reach for it in other specs —
    // what it can do depends on which mode the run is in.
    narrow: { username: process.env.API_PORTAL_NARROW_USERNAME || 'narrow', password: process.env.API_PORTAL_NARROW_PASSWORD || 'narrow' },
};

// Which authorization mode the portal under test is running ("scope" | "role"),
// set by the compose fixture from the Makefile's AUTH_MODE. Specs whose expectations
// differ per mode branch on this; everything else must pass in both.
const AUTH_MODE = process.env.API_PORTAL_AUTH_MODE || 'scope';

// One supertest agent per role, logged in once and reused — the agent's cookie
// jar carries the session across every request made through it.
const agents = {};
const xsrfTokens = {};
const loginPromises = {};

const { CookieAccessInfo } = require('cookiejar');

function extractXsrfToken(agent) {
    // supertest-agent (superagent under the hood) exposes its cookie jar this way.
    const jar = agent.jar || agent._jar;
    if (!jar) return undefined;
    const cookies = jar.getCookies(CookieAccessInfo.All);
    const match = cookies.find((c) => c.name === 'XSRF-TOKEN');
    return match ? match.value : undefined;
}

// Call once per role before using `as(role)` — typically in a `beforeAll`.
// Safe to call more than once; the actual login only happens the first time.
async function login(role) {
    if (loginPromises[role]) return loginPromises[role];
    const { username, password } = CREDENTIALS[role] || {};
    if (!username) throw new Error(`Unknown auth role '${role}'`);

    loginPromises[role] = (async () => {
        const agent = supertest.agent(BASE_URL);
        const res = await agent
            .post(`${BASE_PATH}/${ORG_HANDLE}/views/default/login`)
            .type('form')
            .send({ username, password })
            .redirects(0);
        if (res.status !== 302 || /error=/.test(res.headers.location || '')) {
            throw new Error(`Login failed for role '${role}': ${res.status} ${res.headers.location || ''}`);
        }
        agents[role] = agent;
        // handleLocalLogin calls req.session.regenerate() partway through the
        // login request, *after* the global CSRF-cookie middleware (src/app.js)
        // already ran once for that request — so the XSRF-TOKEN cookie on the
        // login response itself reflects the discarded pre-regenerate session,
        // not the one the session cookie actually points to. One throwaway
        // authenticated GET refreshes it to the real value for this session.
        await agent.get(`${API_PREFIX}/organizations/${ORG_HANDLE}`);
        xsrfTokens[role] = extractXsrfToken(agent);
    })();
    return loginPromises[role];
}

// Synchronous accessor — throws if `login(role)` hasn't resolved yet, so a
// missing `await client.login(...)` fails loudly instead of hanging on `undefined`.
function as(role) {
    const agent = agents[role];
    if (!agent) throw new Error(`No active session for role '${role}' — call \`await client.login('${role}')\` first (e.g. in beforeAll).`);
    const xsrf = xsrfTokens[role];
    // The XSRF-TOKEN cookie's value *is* the expected token (double-submit
    // pattern — see src/app.js), but requireCsrfForMutatingApi only reads it
    // back from X-CSRF-Token (or csrf-token), never X-XSRF-TOKEN.
    const withXsrf = (req) => (xsrf ? req.set('X-CSRF-Token', xsrf) : req);

    return {
        get: (path) => agent.get(`${API_PREFIX}${path}`),
        // post resolves to the response like before (every caller just awaits it —
        // none chain supertest methods on the return), with a tap that auto-registers
        // a created top-level resource for afterAll cleanup (support/cleanup.js).
        post: (path, body) => withXsrf(agent.post(`${API_PREFIX}${path}`)).send(body)
            .then((res) => { autoTrackFromResponse(path, body, res, role); return res; }),
        put: (path, body) => withXsrf(agent.put(`${API_PREFIX}${path}`)).send(body),
        del: (path) => withXsrf(agent.delete(`${API_PREFIX}${path}`)),
        // For multipart/form-data endpoints (e.g. POST/PUT /apis) — caller chains
        // .field()/.attach() before awaiting. Because the request is built by
        // chaining after this returns, we can't tap it with .then; instead listen
        // for superagent's 'response' event, which fires with the parsed body on
        // await and lets us auto-register the created API/MCP for afterAll cleanup.
        postMultipart: (path) => {
            const req = withXsrf(agent.post(`${API_PREFIX}${path}`));
            req.on('response', (res) => autoTrackFromResponse(path, undefined, res, role));
            return req;
        },
        putMultipart: (path) => withXsrf(agent.put(`${API_PREFIX}${path}`)),
    };
}

// For session-authenticated PAGE routes (not under /api/v0.9) that also require
// CSRF — e.g. the settings page's `PUT /:org/views/:view/llms-config` — reusing
// the same logged-in agent/token as `as(role)`, just without the API prefix.
function page(role) {
    const agent = agents[role];
    if (!agent) throw new Error(`No active session for role '${role}' — call \`await client.login('${role}')\` first (e.g. in beforeAll).`);
    const xsrf = xsrfTokens[role];
    const withXsrf = (req) => (xsrf ? req.set('X-CSRF-Token', xsrf) : req);

    // Page routes live under BASE_PATH just like the REST API, but without the
    // /api/v0.9 segment — callers pass the bare page path and BASE_PATH is prepended
    // here, mirroring how API_PREFIX carries it for as(role).
    return {
        get: (path) => agent.get(`${BASE_PATH}${path}`),
        put: (path, body) => withXsrf(agent.put(`${BASE_PATH}${path}`)).send(body),
    };
}

module.exports = {
    BASE_URL,
    BASE_PATH,
    API_PREFIX,
    ORG_HANDLE,
    AUTH_MODE,
    login,
    as,
    page,
    // Escape hatch for requests that shouldn't carry any session (auth tests,
    // unauthenticated-access checks, etc.) — a plain, cookie-less client.
    raw: () => supertest(BASE_URL),
};
