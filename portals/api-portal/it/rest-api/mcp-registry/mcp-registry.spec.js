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

// MCP Server Registry (OpenAPI v0.1) — src/routes/pages/mcpRegistryRoute.js.
//
// Addressed differently from every other suite here: the registry is NOT under the
// /api/v0.9 REST base, so client.as(role) can't reach it. It is mounted twice on the
// portal router (app.js) — `/registry/:orgHandle` and `/:orgHandle/registry` — which is
// why the mount-parity test below exists: the two are separate app.use() calls that
// could drift.
//
// Its callers are MCP clients, i.e. programs, so every response — including rejections —
// has to be JSON. That is asserted explicitly rather than trusted: the org guard and the
// auth gate both sit in front of handlers whose natural failure mode (falling through to
// app.js) renders an HTML error page.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

const ORG = client.ORG_HANDLE;
const REGISTRY = `/registry/${ORG}/v0.1`;
// The second mount of the same router, exercised by the parity test.
const REGISTRY_ALT = `/${ORG}/registry/v0.1`;

// A reverse-DNS "namespace/server" name, which validateServerDetail enforces. The slash
// is significant: it means every path that addresses a server by name has to carry it
// percent-encoded through a single path segment.
const serverName = (suffix) => `it.example/${suffix}`;
const encodeName = (name) => encodeURIComponent(name);

function publishBody(name, overrides = {}) {
    return {
        name,
        description: overrides.description || 'Published by the MCP registry integration suite.',
        version: overrides.version || '1.0.0',
        title: overrides.title || 'Registry IT Server',
        remotes: overrides.remotes || [
            { type: 'streamable-http', url: `https://backend.example.invalid/${encodeName(name)}` },
        ],
        _meta: overrides._meta || {
            'io.api-platform/mcp-capabilities': {
                tools: [{ name: 'ping', description: 'Health check tool.', inputSchema: { type: 'object', properties: {} } }],
                resources: [],
                prompts: [],
            },
        },
        ...(overrides.extra || {}),
    };
}

// Reads the whole server collection, following metadata.nextCursor to the end. The
// registry caps a page at MAX_LIMIT=100, so any completeness claim about the listing
// has to page rather than ask for one big limit.
async function listAllServers() {
    const all = [];
    let cursor;
    // Bounded so a cursor that failed to advance ends the test instead of the run.
    for (let page = 0; page < 100; page += 1) {
        const query = cursor ? `?limit=100&cursor=${encodeURIComponent(cursor)}` : '?limit=100';
        const res = await client.raw().get(`${client.BASE_PATH}${REGISTRY}/servers${query}`);
        expect(res.status).toBe(200);
        all.push(...res.body.servers);
        cursor = res.body.metadata?.nextCursor;
        if (!cursor) return all;
    }
    throw new Error('listAllServers: nextCursor never terminated');
}

// Published servers are not created through the /api/v0.9 collections, so the suite's
// auto-cleanup (support/cleanup.js) never sees them — this spec removes its own.
const published = [];

async function publish(body, role = 'publisher') {
    const res = await client.page(role).post(`${REGISTRY}/publish`, body);
    if (res.status === 201 || res.status === 200) {
        published.push({ name: body.name, version: body.version });
    }
    return res;
}

describe('MCP server registry (v0.1)', () => {
    beforeAll(async () => {
        await client.login('publisher');
        await client.login('developer');
    });

    afterAll(async () => {
        for (const { name, version } of published) {
            await client.page('publisher')
                .del(`${REGISTRY}/servers/${encodeName(name)}/versions/${encodeURIComponent(version)}`);
        }
    });

    describe('discovery', () => {
        it('lists servers without a session — discovery is public', async () => {
            // client.raw() carries no cookies at all, so a 200 here proves the endpoint is
            // reachable anonymously rather than riding a session left over from login().
            const res = await client.raw().get(`${client.BASE_PATH}${REGISTRY}/servers`);
            expect(res.status).toBe(200);
            expect(Array.isArray(res.body.servers)).toBe(true);
            expect(res.body.metadata).toHaveProperty('count');
        });

        it('publishes a server and returns it from every discovery endpoint', async () => {
            const name = serverName(uniqueHandle('discoverable'));
            const created = await publish(publishBody(name));
            expect(created.status).toBe(201);
            expect(created.body.server).toMatchObject({ name, version: '1.0.0' });
            expect(created.body.server.remotes[0]).toHaveProperty('url');

            const list = await client.raw().get(`${client.BASE_PATH}${REGISTRY}/servers`);
            expect(list.status).toBe(200);
            expect(list.body.servers.map((s) => s.server.name)).toContain(name);

            const versions = await client.raw()
                .get(`${client.BASE_PATH}${REGISTRY}/servers/${encodeName(name)}/versions`);
            expect(versions.status).toBe(200);
            // Asserted on the version field of a list entry, not on a stringified body:
            // '1.0.0' also appears in $schema-adjacent text and could match something
            // that is not a version at all, which would make this pass for the wrong reason.
            expect(versions.body.servers.map((s) => s.server.version)).toContain('1.0.0');

            const version = await client.raw()
                .get(`${client.BASE_PATH}${REGISTRY}/servers/${encodeName(name)}/versions/1.0.0`);
            expect(version.status).toBe(200);
            expect(version.body.server).toMatchObject({ name, version: '1.0.0' });
        });

        it('serves the same server under both registry mount paths', async () => {
            // '/registry/:orgHandle' and '/:orgHandle/registry' are two independent
            // app.use() calls onto one router; nothing but this stops them diverging.
            const name = serverName(uniqueHandle('both-mounts'));
            expect((await publish(publishBody(name))).status).toBe(201);

            const primary = await client.raw()
                .get(`${client.BASE_PATH}${REGISTRY}/servers/${encodeName(name)}/versions/1.0.0`);
            const alternate = await client.raw()
                .get(`${client.BASE_PATH}${REGISTRY_ALT}/servers/${encodeName(name)}/versions/1.0.0`);
            expect(primary.status).toBe(200);
            expect(alternate.status).toBe(200);
            expect(alternate.body.server).toEqual(primary.body.server);
        });

        it('404s an unknown server, as JSON rather than an HTML error page', async () => {
            const res = await client.raw()
                .get(`${client.BASE_PATH}${REGISTRY}/servers/${encodeName(serverName('no-such-server'))}/versions/1.0.0`);
            expect(res.status).toBe(404);
            expect(res.headers['content-type']).toMatch(/application\/json/);
        });

        it('404s the registry under another organization handle', async () => {
            // The database is shared across organizations, so the handle is untrusted
            // input — orgGuardMiddleware pins it, and answers JSON because the callers
            // here are MCP clients rather than browsers.
            const res = await client.raw().get(`${client.BASE_PATH}/registry/some-other-org/v0.1/servers`);
            expect(res.status).toBe(404);
            expect(res.headers['content-type']).toMatch(/application\/json/);
        });
    });

    describe('publishing', () => {
        it('treats a re-publish of the same name and version as an update, not a create', async () => {
            const name = serverName(uniqueHandle('upsert'));
            expect((await publish(publishBody(name))).status).toBe(201);

            const again = await publish(publishBody(name, { title: 'Updated Title' }));
            expect(again.status).toBe(200);       // 200 = updated, 201 = created

            // The upsert must not leave a duplicate behind. Walked page by page rather
            // than read off a single ?limit=100 request: 100 is the registry's MAX_LIMIT
            // (mcpRegistryService.normalizeLimit), so on an org holding more servers than
            // that a duplicate could sit past the first page and go unseen.
            const matches = (await listAllServers()).filter((s) => s.server.name === name);
            expect(matches).toHaveLength(1);
        });

        // KNOWN DEFECT, pinned rather than asserted-as-correct. `title` is accepted by
        // publish and echoed in that response, but it does not survive a read: the publish
        // response is built from the in-memory payload, while ServerResponseDTO reads it
        // back from `metadata_search.title`, where it never lands. When the round trip is
        // fixed this test fails — which is the point. Swap the two expectations then.
        it('does not persist `title` across a read (known defect)', async () => {
            const name = serverName(uniqueHandle('title-roundtrip'));
            const created = await publish(publishBody(name, { title: 'Given Title' }));
            expect(created.status).toBe(201);
            expect(created.body.server.title).toBe('Given Title');   // echoed on publish

            const read = await client.raw()
                .get(`${client.BASE_PATH}${REGISTRY}/servers/${encodeName(name)}/versions/1.0.0`);
            expect(read.status).toBe(200);
            expect(read.body.server.title).toBeUndefined();          // ...and lost on read
        });

        // KNOWN DEFECT, pinned for the same reason. The registry exposes listVersions,
        // version-scoped GET/PUT/DELETE, and a name+version existence check — all of which
        // imply one server can hold several versions. It cannot: buildApiMetadataPayload
        // derives the API handle via deriveApiHandle(name, orgHandle), i.e. from the NAME
        // ALONE, so a second version collides on the unique constraint and surfaces through
        // handleUnexpectedError's isDuplicateKeyError branch as a 409. Expect 201 here once
        // the handle includes the version.
        it('rejects a second version of the same server with 409 (known defect)', async () => {
            const name = serverName(uniqueHandle('multi-version'));
            expect((await publish(publishBody(name, { version: '1.0.0' }))).status).toBe(201);

            const second = await publish(publishBody(name, { version: '2.0.0' }));
            expect(second.status).toBe(409);
            expect(second.body).toMatchObject({ error: 'Server version already exists' });

            // The first version is unaffected by the rejected publish.
            const first = await client.raw()
                .get(`${client.BASE_PATH}${REGISTRY}/servers/${encodeName(name)}/versions/1.0.0`);
            expect(first.status).toBe(200);
        });

        it.each([
            ['a missing name', { name: undefined }],
            ['a name that is not reverse-DNS', { name: 'not-reverse-dns' }],
            ['a missing version', { version: undefined }],
            ['the reserved version "latest"', { version: 'latest' }],
            ['a version range rather than a specific version', { version: '^1.0.0' }],
        ])('rejects %s with 400', async (_label, override) => {
            const body = publishBody(serverName(uniqueHandle('invalid')));
            Object.assign(body, override);
            if (override.name === undefined && 'name' in override) delete body.name;
            if (override.version === undefined && 'version' in override) delete body.version;

            const res = await client.page('publisher').post(`${REGISTRY}/publish`, body);
            expect(res.status).toBe(400);
        });
    });

    describe('authorization', () => {
        it('rejects an unauthenticated publish with 401 JSON', async () => {
            const res = await client.raw()
                .post(`${client.BASE_PATH}${REGISTRY}/publish`)
                .send(publishBody(serverName(uniqueHandle('anon'))));
            expect(res.status).toBe(401);
            expect(res.headers['content-type']).toMatch(/application\/json/);
            expect(res.body).toMatchObject({ error: 'unauthorized' });
        });

        it('rejects a publish from a read-only role with 403', async () => {
            // dp_developer_it holds dp:mcp_server:read only (it/configs/portal-roles-it.yaml),
            // so this separates "authenticated" from "entitled".
            const res = await client.page('developer')
                .post(`${REGISTRY}/publish`, publishBody(serverName(uniqueHandle('read-only'))));
            expect(res.status).toBe(403);
        });

        it('rejects unauthenticated writes to an existing server', async () => {
            const name = serverName(uniqueHandle('protected'));
            const original = publishBody(name);
            expect((await publish(original)).status).toBe(201);
            const target = `${client.BASE_PATH}${REGISTRY}/servers/${encodeName(name)}/versions/1.0.0`;

            const hijacked = 'Hijacked by an unauthenticated caller.';
            const updated = await client.raw().put(target).send(publishBody(name, { description: hijacked }));
            expect(updated.status).toBe(401);

            const deleted = await client.raw().delete(target);
            expect(deleted.status).toBe(401);

            // And the server is untouched by either attempt. Checked on `description`,
            // not `title`: title does not survive a read at all (see the known-defect
            // test above), so asserting on it would hold whether or not the write landed.
            // Asserted as equal to what was published rather than merely "not the hijack
            // value", so the check still fails if description stops round-tripping too.
            const check = await client.raw().get(target);
            expect(check.status).toBe(200);
            expect(check.body.server.description).toBe(original.description);
        });

        it('lets an entitled role delete a version, after which it is gone', async () => {
            const name = serverName(uniqueHandle('deletable'));
            expect((await publish(publishBody(name))).status).toBe(201);
            const path = `${REGISTRY}/servers/${encodeName(name)}/versions/1.0.0`;

            const res = await client.page('publisher').del(path);
            expect([200, 204]).toContain(res.status);

            const after = await client.raw().get(`${client.BASE_PATH}${path}`);
            expect(after.status).toBe(404);
        });
    });

    describe('listing behaviour', () => {
        it('paginates with limit and nextCursor', async () => {
            const first = await client.raw().get(`${client.BASE_PATH}${REGISTRY}/servers?limit=1`);
            expect(first.status).toBe(200);
            expect(first.body.servers.length).toBeLessThanOrEqual(1);

            if (first.body.metadata.nextCursor) {
                const next = await client.raw()
                    .get(`${client.BASE_PATH}${REGISTRY}/servers?limit=1&cursor=${encodeURIComponent(first.body.metadata.nextCursor)}`);
                expect(next.status).toBe(200);
                // A cursor that just repeated the first page would make pagination silently
                // non-terminating, so assert the page actually moved.
                if (next.body.servers.length && first.body.servers.length) {
                    expect(next.body.servers[0].server.name).not.toBe(first.body.servers[0].server.name);
                }
            }
        });

        it('rejects a malformed cursor with 400 rather than failing internally', async () => {
            const res = await client.raw().get(`${client.BASE_PATH}${REGISTRY}/servers?cursor=not-a-real-cursor`);
            expect(res.status).toBe(400);
        });
    });

    describe('cross-origin access', () => {
        it('allows any origin, so a browser-based MCP client can read the registry', async () => {
            const res = await client.raw().get(`${client.BASE_PATH}${REGISTRY}/servers`);
            expect(res.headers['access-control-allow-origin']).toBe('*');
        });

        it('answers preflight with 204', async () => {
            const res = await client.raw().options(`${client.BASE_PATH}${REGISTRY}/servers`);
            expect(res.status).toBe(204);
            expect(res.headers['access-control-allow-origin']).toBe('*');
        });
    });
});
