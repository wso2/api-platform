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

// Multi-tenancy mode (multi_tenancy.enabled) end to end, against two portal instances
// in IDP mode backed by the suite's mock IDP (support/mock-idp.js):
//
//   API_PORTAL_MULTI_TENANCY_BASE_URL        enforce_org_validation = false — an unknown
//                                        org claim provisions the organization, a
//                                        missing claim falls back to the configured one
//   API_PORTAL_MULTI_TENANCY_STRICT_BASE_URL enforce_org_validation = true — both refused
//   API_PORTAL_MULTI_TENANCY_REPLICA_BASE_URL optional second instance on the first one's
//                                        database (Postgres/MSSQL fixtures only)
//
// Everything else in the suite runs the default single-organization mode, and must
// keep passing unchanged — that is the other half of this feature's contract.

const fs = require('fs');
const https = require('https');
const supertest = require('supertest');
const { createMockIdp } = require('../support/mock-idp');
const { createWebhookSink, resolveSinkUrl } = require('../support/webhook-sink');
const { poll, sleep } = require('../support/wait-for');
const multiTenancyDb = require('../support/multiTenancyDb');

const BASE_URL = process.env.API_PORTAL_MULTI_TENANCY_BASE_URL;
const STRICT_BASE_URL = process.env.API_PORTAL_MULTI_TENANCY_STRICT_BASE_URL;
const REPLICA_BASE_URL = process.env.API_PORTAL_MULTI_TENANCY_REPLICA_BASE_URL;
const IDP_BASE_URL = process.env.MOCK_IDP_BASE_URL || 'https://rest-api-tests:4510';
const API = '/api-portal/api/v0.9';
const ADMIN_ROLES = ['dp_admin', 'ap_admin'];

const describeMultiTenancy = BASE_URL ? describe : describe.skip;

const idp = BASE_URL && createMockIdp({
    baseUrl: IDP_BASE_URL,
    clientId: 'portal-it-client',
    certFile: process.env.MOCK_IDP_CERT_FILE,
    keyFile: process.env.MOCK_IDP_KEY_FILE,
});

// Every spec run starts from whatever the previous run left in the database (the
// fixtures reuse volumes), so organizations introduced here get a per-run suffix.
const RUN = Date.now().toString(36);
const orgId = (name) => `${name}-${RUN}`;

function bearer(login) {
    return idp.mint({ roles: ADMIN_ROLES, ...login }).access_token;
}

function api(base, token) {
    const agent = supertest(base);
    return {
        get: (path) => agent.get(`${API}${path}`).set('Authorization', `Bearer ${token}`),
        post: (path, body) => agent.post(`${API}${path}`).set('Authorization', `Bearer ${token}`).send(body),
        put: (path, body) => agent.put(`${API}${path}`).set('Authorization', `Bearer ${token}`).send(body),
    };
}

async function createApi(base, token, id) {
    const metadata = {
        id, name: `MultiTenancy ${id}`, version: 'v1.0', type: 'REST', status: 'PUBLISHED',
        endPoints: { productionURL: `https://backend.example.invalid/${id}`, sandboxURL: `https://sandbox.example.invalid/${id}` },
    };
    const definition = JSON.stringify({
        openapi: '3.0.0', info: { title: id, version: '1.0.0' },
        paths: { '/ping': { get: { responses: { 200: { description: 'ok' } } } } },
    });
    const res = await supertest(base).post(`${API}/apis`).set('Authorization', `Bearer ${token}`)
        .field('metadata', JSON.stringify(metadata))
        .attach('definition', Buffer.from(definition), 'definition.json');
    expect(res.status).toBe(201);
    return res.body.id;
}

// GET against the mock IDP itself, which uses the suite's self-signed certificate.
function idpGet(url) {
    return new Promise((resolve, reject) => {
        https.get(url, { ca: fs.readFileSync(process.env.MOCK_IDP_CERT_FILE) }, (res) => {
            res.resume();
            resolve(res);
        }).on('error', reject);
    });
}

/**
 * A browser visiting `path`: follows every redirect — including silent SSO's
 * prompt=none round trip through the IDP — and returns the final status and path.
 */
async function browse(agent, path) {
    let res = await agent.get(path);
    for (let hops = 0; res.status === 302 && hops < 8; hops++) {
        let location = res.headers.location;
        if (location.startsWith(IDP_BASE_URL)) location = (await idpGet(location)).headers.location;
        const next = new URL(location, 'http://portal.invalid');
        res = await agent.get(`${next.pathname}${next.search}`);
        path = next.pathname;
    }
    return { status: res.status, path };
}

/**
 * Drives a browser login: the portal's login route -> the IDP's authorize endpoint ->
 * the portal's callback. Returns the session-holding agent and where the callback sent
 * the browser (or its status when it did not redirect).
 */
async function browserLogin(base, startPath, login) {
    idp.nextLogin({ roles: ADMIN_ROLES, ...login });
    const agent = supertest.agent(base);
    const toIdp = await agent.get(startPath);
    expect(toIdp.status).toBe(302);
    const fromIdp = await idpGet(toIdp.headers.location);
    expect(fromIdp.statusCode).toBe(302);
    const callback = new URL(fromIdp.headers.location);
    const landed = await agent.get(`${callback.pathname}${callback.search}`);
    return { agent, status: landed.status, location: landed.headers.location };
}

describeMultiTenancy('multi-tenancy mode', () => {
    const sinks = { a: createWebhookSink(), b: createWebhookSink() };

    beforeAll(async () => {
        await idp.start(Number(new URL(IDP_BASE_URL).port));
        await sinks.a.start(4511);
        await sinks.b.start(4512);
    });

    afterAll(async () => {
        await idp.stop();
        await sinks.a.stop();
        await sinks.b.stop();
        await multiTenancyDb.close();
    });

    describe('organization claims (enforce_org_validation = false)', () => {
        test('the configured organization is served as always', async () => {
            const res = await api(BASE_URL, bearer({ sub: 'alice', orgId: 'default' })).get('/organizations/default');
            expect(res.status).toBe(200);
            expect(res.body.id).toBe('default');
        });

        test('an unknown organization is provisioned from the claim, named from the org-name claim', async () => {
            const claim = orgId('globex');
            const token = bearer({ sub: 'gina', orgId: claim, orgName: `Globex ${RUN}` });
            expect((await api(BASE_URL, token).get('/apis')).status).toBe(200);

            const rows = await multiTenancyDb.orgsByIdpRefId(claim);
            expect(rows).toHaveLength(1);
            expect(rows[0].handle).toBe(`globex-${RUN}`);
            expect(rows[0].display_name).toBe(`Globex ${RUN}`);

            // Provisioned with the same defaults as the configured organization.
            const views = await api(BASE_URL, token).get('/views');
            expect(views.status).toBe(200);
            expect(views.body.list.map((v) => v.id)).toContain('default');
            const org = await api(BASE_URL, token).get(`/organizations/globex-${RUN}`);
            expect(org.status).toBe(200);
            expect(org.body.idpRefId).toBe(claim);
        });

        test('the org-handle claim becomes the provisioned organization\'s handle as-is', async () => {
            const claim = orgId('handled');
            const token = bearer({ sub: 'hana', orgId: claim, orgName: `Handled Org ${RUN}`, orgHandle: `hnd${RUN}` });
            expect((await api(BASE_URL, token).get('/apis')).status).toBe(200);
            const rows = await multiTenancyDb.orgsByIdpRefId(claim);
            expect(rows).toHaveLength(1);
            expect(rows[0].handle).toBe(`hnd${RUN}`.toLowerCase());
            expect(rows[0].display_name).toBe(`Handled Org ${RUN}`);
            expect(rows[0].idp_ref_id).toBe(claim);
        });

        test('a token issued to another application (another audience) is rejected', async () => {
            const token = bearer({ sub: 'aud', orgId: 'default', aud: 'some-other-app' });
            expect((await api(BASE_URL, token).get('/organizations/default')).status).toBe(401);
        });

        test('a credential with no organization claim falls back to the configured organization', async () => {
            const token = bearer({ sub: 'nora', omitOrg: true });
            expect((await api(BASE_URL, token).get('/organizations/default')).status).toBe(200);
        });

        test('an org_handle claim is not taken for the organization claim', async () => {
            // Only org_id is mapped: the handle alone neither names an organization nor
            // provisions one — the token is treated as carrying no organization claim.
            const handle = `byhandle-${RUN}`;
            const token = bearer({ sub: 'hal', omitOrg: true, orgHandle: handle });
            expect((await api(BASE_URL, token).get('/organizations/default')).status).toBe(200);
            expect(await multiTenancyDb.orgsByIdpRefId(handle)).toHaveLength(0);
        });

        test('a single-entry list claim is that organization; one naming two is refused', async () => {
            const claim = orgId('listed');
            expect((await api(BASE_URL, bearer({ sub: 'l1', orgId: claim })).get('/apis')).status).toBe(200);
            expect((await api(BASE_URL, bearer({ sub: 'l2', orgClaim: [claim] })).get(`/organizations/listed-${RUN}`)).status).toBe(200);
            expect((await api(BASE_URL, bearer({ sub: 'l3', orgClaim: [claim, 'default'] })).get('/apis')).status).toBe(403);
        });

        test('concurrent first use of a new organization provisions it exactly once', async () => {
            const claim = orgId('race');
            const token = bearer({ sub: 'racer', orgId: claim, orgName: `Race ${RUN}` });
            const results = await Promise.all(Array.from({ length: 10 }, () => api(BASE_URL, token).get('/apis')));
            expect(results.map((r) => r.status)).toEqual(Array(10).fill(200));
            expect(await multiTenancyDb.orgsByIdpRefId(claim)).toHaveLength(1);
        });
    });

    describe('tenant isolation', () => {
        // Claims (idp_ref_id) deliberately differ from the handles their names derive.
        const tokenA = () => bearer({ sub: 'ann', orgId: orgId('iso-a'), orgName: `Tenant A ${RUN}` });
        const tokenB = () => bearer({ sub: 'ben', orgId: orgId('iso-b'), orgName: `Tenant B ${RUN}` });
        const handleA = `tenant-a-${RUN}`;
        const handleB = `tenant-b-${RUN}`;

        test('an organization never sees another\'s APIs', async () => {
            const apiId = await createApi(BASE_URL, tokenA(), `iso-api-${RUN}`);
            expect((await api(BASE_URL, tokenA()).get(`/apis/${apiId}`)).status).toBe(200);
            expect((await api(BASE_URL, tokenB()).get(`/apis/${apiId}`)).status).toBe(404);
            const listB = await api(BASE_URL, tokenB()).get('/apis');
            expect(listB.body.list.map((x) => x.id)).not.toContain(apiId);
            expect((await api(BASE_URL, tokenB()).post(`/apis/${apiId}/api-keys/generate`, { id: 'stolen' })).status).toBe(404);
        });

        test('the organization APIs accept only the caller\'s own organization', async () => {
            await api(BASE_URL, tokenB()).get('/apis'); // make sure B exists
            expect((await api(BASE_URL, tokenA()).get(`/organizations/${handleA}`)).status).toBe(200);
            expect((await api(BASE_URL, tokenA()).get(`/organizations/${handleB}`)).status).toBe(403);
            expect((await api(BASE_URL, tokenA()).get('/organizations/default')).status).toBe(403);
            const rename = await api(BASE_URL, tokenA()).put('/organizations/default',
                { id: 'default', displayName: 'taken over', idpRefId: 'default' });
            expect(rename.status).toBe(403);
        });

        test('the MCP registry\'s write endpoints only act in the caller\'s own organization', async () => {
            const metadata = [
                'apiVersion: api-portal.api-platform.wso2.com/v1', 'kind: MCP',
                'metadata:', `  name: iso-mcp-${RUN}`, 'spec:', '  type: MCP', `  displayName: Iso MCP ${RUN}`,
                '  version: 1.0.0', '  description: d', '  status: PUBLISHED',
                '  endpoints:', '    productionUrl: https://mcp.example.invalid',
            ].join('\n');
            const definition = '- type: TOOL\n  name: ping\n  description: ping\n  inputSchema:\n    type: object\n';
            const created = await supertest(BASE_URL).post(`${API}/mcp-servers`).set('Authorization', `Bearer ${tokenA()}`)
                .attach('metadata', Buffer.from(metadata), { filename: 'api.yaml', contentType: 'application/yaml' })
                .attach('definition', Buffer.from(definition), 'definition.yaml');
            expect(created.status).toBe(201);
            const path = `/api-portal/registry/${handleA}/v0.1/servers/${encodeURIComponent(`Iso MCP ${RUN}`)}`;
            const as = (token) => (req) => req.set('Authorization', `Bearer ${token}`);

            // Tenant B, holding every MCP scope in its own organization, cannot touch A's.
            expect((await as(tokenB())(supertest(BASE_URL).delete(`${path}/versions/1.0.0`))).status).toBe(403);
            expect((await as(tokenB())(supertest(BASE_URL).patch(`${path}/status`)).send({ status: 'deprecated' })).status).toBe(403);
            const listed = await supertest(BASE_URL).get(`/api-portal/registry/${handleA}/v0.1/servers`);
            expect(listed.body.servers.map((x) => x.server.name)).toContain(`Iso MCP ${RUN}`);
            // Tenant A can.
            expect((await as(tokenA())(supertest(BASE_URL).delete(`${path}/versions/1.0.0`))).status).toBe(200);
        });

        test('a claim is matched on idp_ref_id only, never on a handle', async () => {
            // A token claiming tenant A's *handle* is not tenant A: here it provisions a
            // separate organization (under a suffixed handle) and still can't reach A.
            const token = bearer({ sub: 'h', orgId: handleA });
            expect((await api(BASE_URL, token).get(`/organizations/${handleA}`)).status).toBe(403);
            const rows = await multiTenancyDb.orgsByIdpRefId(handleA);
            expect(rows).toHaveLength(1);
            expect(rows[0].handle).toMatch(new RegExp(`^${handleA}-[0-9a-f]{6}$`));
        });
    });

    describe('pages and browser login', () => {
        test('anyone can browse an existing organization\'s public pages; an unknown handle is 404', async () => {
            await api(BASE_URL, bearer({ sub: 'p', orgId: orgId('pages'), orgName: `Pages ${RUN}` })).get('/apis');
            idp.nextLogin(null); // no IDP session: silent SSO answers login_required
            const page = await browse(supertest.agent(BASE_URL), `/api-portal/pages-${RUN}/views/default`);
            expect(page).toEqual({ status: 200, path: `/api-portal/pages-${RUN}/views/default` });
            expect((await supertest(BASE_URL).get(`/api-portal/no-such-org-${RUN}/views/default`)).status).toBe(404);
        });

        test('a new organization\'s user signs in from the configured organization\'s page and lands in their own', async () => {
            const claim = orgId('initech');
            const { agent, status, location } = await browserLogin(BASE_URL, '/api-portal/default/views/default/login',
                { sub: 'ivy', orgId: claim, orgName: `Initech ${RUN}` });
            // No hint from the configured organization's page: the IDP chooses.
            expect(idp.lastAuthorize().org).toBeUndefined();
            expect(status).toBe(302);
            expect(location).toBe(`/api-portal/initech-${RUN}/views/default`);
            expect((await agent.get(`/api-portal/initech-${RUN}/views/default/applications`)).status).toBe(200);
            expect((await agent.get('/api-portal/default/views/default/applications')).status).toBe(403);
            expect((await agent.get(`${API}/organizations/initech-${RUN}`)).status).toBe(200);
        });

        test('an administrator sees Settings only on their own organization\'s pages', async () => {
            const { agent } = await browserLogin(BASE_URL, '/api-portal/default/views/default/login',
                { sub: 'ada', orgId: orgId('navs'), orgName: `Navs ${RUN}` });
            const own = await agent.get(`/api-portal/navs-${RUN}/views/default`);
            expect(own.status).toBe(200);
            expect(own.text).toContain('id="admin-settings"');
            expect(own.text).toContain('id="applications"');
            const other = await agent.get('/api-portal/default/views/default');
            expect(other.status).toBe(200);
            expect(other.text).not.toContain('id="admin-settings"');
            expect(other.text).toContain('id="applications"'); // shown, as for anonymous visitors
            const denied = await agent.get('/api-portal/default/views/default/applications');
            expect(denied.status).toBe(403);
            expect(denied.text).not.toContain('id="admin-settings"');
        });

        test('another organization\'s login page sends its hint; ?org= overrides it', async () => {
            const claim = orgId('hinted');
            await api(BASE_URL, bearer({ sub: 'h1', orgId: claim, orgName: `Hinted ${RUN}` })).get('/apis');
            const hintIn = async (path) => {
                const res = await supertest(BASE_URL).get(path);
                expect(res.status).toBe(302);
                return new URL(res.headers.location).searchParams.get('org');
            };
            expect(await hintIn(`/api-portal/hinted-${RUN}/views/default/login`)).toBe(claim);
            expect(await hintIn(`/api-portal/default/views/default/login?org=${encodeURIComponent(claim)}`)).toBe(claim);
            expect(await hintIn('/api-portal/default/views/default/login')).toBeNull();
        });

        test('a login with no organization claim is recorded as the configured organization', async () => {
            const { agent, status } = await browserLogin(BASE_URL, '/api-portal/default/views/default/login', { sub: 'nobody', omitOrg: true });
            expect(status).toBe(302);
            expect((await agent.get('/api-portal/default/views/default/applications')).status).toBe(200);
            expect((await agent.get(`/api-portal/initech-${RUN}/views/default/applications`)).status).toBe(403);
        });
    });

    describe('silent SSO', () => {
        const x = orgId('silent-x');
        const y = orgId('silent-y');
        const userX = { sub: 'sam', orgId: x, orgName: `Silent X ${RUN}`, roles: ADMIN_ROLES };

        beforeAll(async () => {
            await api(BASE_URL, bearer({ sub: 's-x', orgId: x, orgName: `Silent X ${RUN}` })).get('/apis');
            await api(BASE_URL, bearer({ sub: 's-y', orgId: y, orgName: `Silent Y ${RUN}` })).get('/apis');
        });

        test('it asks the IDP only for the organization being browsed', async () => {
            idp.nextLogin(userX);
            const agent = supertest.agent(BASE_URL);
            // Another organization's page: the IDP session is X's, the hint is Y's, so it
            // stays an anonymous visit — on Y's page, not bounced to X's.
            expect(await browse(agent, `/api-portal/silent-y-${RUN}/views/default`))
                .toEqual({ status: 200, path: `/api-portal/silent-y-${RUN}/views/default` });
            expect(idp.lastAuthorize()).toMatchObject({ prompt: 'none', org: y });
            expect((await agent.get(`${API}/organizations/silent-x-${RUN}`)).status).toBe(401);
        });

        test('it signs the visitor in on their own organization\'s page, in place', async () => {
            idp.nextLogin(userX);
            const agent = supertest.agent(BASE_URL);
            expect(await browse(agent, `/api-portal/silent-x-${RUN}/views/default`))
                .toEqual({ status: 200, path: `/api-portal/silent-x-${RUN}/views/default` });
            expect((await agent.get(`${API}/organizations/silent-x-${RUN}`)).status).toBe(200);
        });

        test('from the configured organization\'s page it signs in whoever the IDP knows, without moving them', async () => {
            idp.nextLogin(userX);
            const agent = supertest.agent(BASE_URL);
            expect(await browse(agent, '/api-portal/default/views/default'))
                .toEqual({ status: 200, path: '/api-portal/default/views/default' });
            expect(idp.lastAuthorize().org).toBeUndefined();
            expect((await agent.get(`${API}/organizations/silent-x-${RUN}`)).status).toBe(200);
        });

        test('an explicit login after a failed silent attempt still lands in the user\'s own organization', async () => {
            idp.nextLogin(null);
            const agent = supertest.agent(BASE_URL);
            await browse(agent, '/api-portal/default/views/default');
            idp.nextLogin(userX);
            const toIdp = await agent.get('/api-portal/default/views/default/login');
            const callback = new URL((await idpGet(toIdp.headers.location)).headers.location);
            const landed = await agent.get(`${callback.pathname}${callback.search}`);
            expect(landed.headers.location).toBe(`/api-portal/silent-x-${RUN}/views/default`);
        });
    });

    describe('issuers', () => {
        test('a token from any issuer other than auth.idp.issuer is rejected, and provisions nothing', async () => {
            // e.g. a WSO2 IS sub-organization's own issuer (".../o/<org>/oauth2/token"),
            // correctly signed by that organization's key.
            const claim = orgId('suborg');
            const token = bearer({ sub: 's1', orgId: claim, orgName: `Suborg ${RUN}`, perOrgIssuer: true });
            expect((await api(BASE_URL, token).get('/apis')).status).toBe(401);
            expect(await multiTenancyDb.orgsByIdpRefId(claim)).toHaveLength(0);
        });
    });

    describe('webhooks', () => {
        test('each organization\'s subscribers receive exactly that organization\'s events, once', async () => {
            const orgs = [
                { claim: orgId('hook-a'), sink: sinks.a, port: 4511 },
                { claim: orgId('hook-b'), sink: sinks.b, port: 4512 },
            ];
            for (const o of orgs) {
                o.token = bearer({ sub: `wh-${o.claim}`, orgId: o.claim, orgName: `Hook ${o.claim}` });
                const sub = await api(BASE_URL, o.token).post('/webhook-subscribers', {
                    displayName: `sink ${o.claim}`, targetUrl: resolveSinkUrl(o.port).toString(),
                    secret: `secret-${o.claim}`, events: ['apikey.*'], enabled: true,
                });
                expect(sub.status).toBe(201);
                o.apiId = await createApi(BASE_URL, o.token, `hook-api-${o.claim}`);
                for (const k of ['k1', 'k2']) {
                    expect((await api(BASE_URL, o.token).post(`/apis/${o.apiId}/api-keys/generate`, { id: k })).status).toBe(201);
                }
                o.uuid = (await multiTenancyDb.orgsByIdpRefId(o.claim))[0].uuid;
            }
            for (const o of orgs) {
                await poll(() => o.sink.received.filter((r) => r.body?.event_type === 'apikey.generated').length >= 2, { timeoutMs: 15000 });
            }
            // Let a would-be duplicate or cross-delivery arrive before asserting.
            await sleep(2500);
            for (const o of orgs) {
                const events = o.sink.received.map((r) => r.body);
                expect(events).toHaveLength(2);
                expect(new Set(events.map((e) => e.event_id)).size).toBe(2);
                expect(events.every((e) => e.org.ref_id === o.uuid)).toBe(true);
            }
        });
    });

    const describeReplica = REPLICA_BASE_URL ? describe : describe.skip;
    describeReplica('two instances on one database', () => {
        test('concurrent work across replicas is delivered exactly once', async () => {
            const claim = orgId('replicated');
            const token = bearer({ sub: 'rep', orgId: claim, orgName: `Replicated ${RUN}` });
            sinks.a.received.length = 0;
            const sub = await api(BASE_URL, token).post('/webhook-subscribers', {
                displayName: `rep ${RUN}`, targetUrl: resolveSinkUrl(4511).toString(), events: ['apikey.*'], enabled: true,
            });
            expect(sub.status).toBe(201);
            const apiId = await createApi(BASE_URL, token, `rep-api-${RUN}`);
            const results = await Promise.all(Array.from({ length: 20 }, (_, i) =>
                api(i % 2 ? REPLICA_BASE_URL : BASE_URL, token).post(`/apis/${apiId}/api-keys/generate`, { id: `r${i}` })));
            expect(results.map((r) => r.status)).toEqual(Array(20).fill(201));
            await poll(() => sinks.a.received.length >= 20, { timeoutMs: 20000 });
            await sleep(3000);
            expect(sinks.a.received).toHaveLength(20);
            expect(new Set(sinks.a.received.map((r) => r.body.event_id)).size).toBe(20);
        }, 40000);
    });

    const describeStrict = STRICT_BASE_URL ? describe : describe.skip;
    describeStrict('enforce_org_validation = true', () => {
        test('the configured organization is served', async () => {
            expect((await api(STRICT_BASE_URL, bearer({ sub: 'alice', orgId: 'default' })).get('/organizations/default')).status).toBe(200);
        });

        test('a claim differing only in case or trailing spaces does not match an organization', async () => {
            // SQL Server's default collation would match these to "default" in the query.
            for (const claim of ['DEFAULT', 'Default', 'default ']) {
                const res = await api(STRICT_BASE_URL, bearer({ sub: 'c', orgClaim: claim })).get('/organizations/default');
                expect(res.status).toBe(403);
            }
        });

        test('an unknown organization is refused, not provisioned', async () => {
            const token = bearer({ sub: 'x', orgId: orgId('strict-new') });
            expect((await api(STRICT_BASE_URL, token).get('/apis')).status).toBe(403);
            expect((await api(STRICT_BASE_URL, token).get('/apis')).status).toBe(403);
        });

        test('a missing organization claim is refused, for a token and for a login', async () => {
            expect((await api(STRICT_BASE_URL, bearer({ sub: 'y', omitOrg: true })).get('/apis')).status).toBe(403);
            expect((await api(STRICT_BASE_URL, bearer({ sub: 'y2', omitOrg: true, orgHandle: 'default' })).get('/apis')).status).toBe(403);
            const { status } = await browserLogin(STRICT_BASE_URL, '/api-portal/default/views/default/login', { sub: 'z', omitOrg: true });
            expect(status).toBe(403);
        });
    });
});
