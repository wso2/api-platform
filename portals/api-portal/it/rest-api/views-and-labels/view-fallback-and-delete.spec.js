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

// The bare org root (/{orgName}) and the portal root (/) redirect to a RESOLVED view,
// not to a hardcoded 'default'.
//
// That hardcoding is what made the 'default' view undeletable and unrenameable: a
// redirect into a view that no longer existed would 404 the portal's own front door.
// orgContentRoute now asks viewDao.getFallbackHandle, which prefers a view whose handle
// is 'default' and otherwise takes the earliest-created one.
//
// The last-view guard (deleting the only remaining view returns 400) is deliberately
// NOT exercised here: this suite shares one seeded organization with every other spec,
// so driving it down to a single view would break whatever runs next. It is covered by
// hand against a scratch database instead.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

describe('view fallback resolution and deletion rules', () => {
    beforeAll(async () => {
        await client.login('admin');
    });

    it('redirects the bare org root to the resolved fallback view', async () => {
        const res = await client.raw().get(`/${client.ORG_HANDLE}`).redirects(0);
        expect(res.status).toBe(302);
        // The fixture org still has its seeded 'default' view, so that is what the
        // resolver prefers — the assertion that matters is that the target is a view
        // that exists, reached through the resolver rather than a literal.
        expect(res.headers.location).toMatch(new RegExp(`^/${client.ORG_HANDLE}/views/[^/]+$`));
        const target = res.headers.location.split('/views/')[1].split(/[?#]/)[0];
        const view = await client.as('admin').get(`/views/${target}`);
        expect(view.status).toBe(200);
    });

    it('redirects the bare org root WITH a trailing slash to the same absolute target', async () => {
        // Express strict routing is off, so /{org}/ matches this route too. The redirect
        // must be absolute: a relative Location resolves against the current directory,
        // which for a trailing-slash URL is /{org}/ — producing /{org}/{org}/views/x.
        const res = await client.raw().get(`/${client.ORG_HANDLE}/`).redirects(0);
        expect(res.status).toBe(302);
        expect(res.headers.location).toMatch(new RegExp(`^/${client.ORG_HANDLE}/views/[^/]+$`));
    });

    it('redirects the portal root into this org and a view that exists', async () => {
        const res = await client.raw().get('/').redirects(0);
        expect(res.status).toBe(302);
        expect(res.headers.location).toContain(`/${client.ORG_HANDLE}/views/`);
        const target = res.headers.location.split('/views/')[1].split(/[?#]/)[0];
        expect((await client.as('admin').get(`/views/${target}`)).status).toBe(200);
    });

    it('deletes a view that is not the last one', async () => {
        // No longer special-cased by handle — what governs the delete is how many views
        // remain, and the fixture org has several.
        const id = uniqueHandle('view');
        expect((await client.as('admin').post('/views', { id, displayName: 'Deletable View' })).status).toBe(201);

        const del = await client.as('admin').del(`/views/${id}`);
        expect(del.status).toBe(204);
        expect((await client.as('admin').get(`/views/${id}`)).status).toBe(404);
    });

    it('returns 404, not 400, for deleting a view that does not exist', async () => {
        // The old guard rejected the handle 'default' up front with a 400. With that
        // gone, an unknown handle is a plain not-found.
        const res = await client.as('admin').del(`/views/${uniqueHandle('view-absent')}`);
        expect(res.status).toBe(404);
    });
});
