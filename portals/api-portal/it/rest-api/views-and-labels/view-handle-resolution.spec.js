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

// A view is addressed by its HANDLE, never by its display name.
//
// viewDao.getId used to fall back to a display_name lookup when the handle missed.
// Only (handle, org_uuid) is unique — display_name has no constraint — so two views in
// one organization can share a display name and the fallback resolved to whichever row
// the database ordered first. It also disagreed with viewDao.get/update/deleteView,
// which are all handle-exact: a display name could clear DELETE's "view has workflows"
// gate and then 404 on a delete that matched nothing. Display names are renameable too,
// so any URL built from one breaks at the next rename.
//
// Every assertion below uses a display name that is deliberately NOT any view's handle.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

const DISPLAY_NAME = 'View Handle Resolution Display Name';

describe('view resolution is by handle, not display name', () => {
    let handle;

    beforeAll(async () => {
        await client.login('admin');
        handle = uniqueHandle('view');
        const res = await client.as('admin').post('/views', { id: handle, displayName: DISPLAY_NAME });
        expect(res.status).toBe(201);
    });

    afterAll(async () => {
        await client.as('admin').del(`/views/${handle}`);
    });

    it('resolves the handle', async () => {
        const res = await client.as('admin').get(`/views/${handle}`);
        expect(res.status).toBe(200);
        expect(res.body.displayName).toBe(DISPLAY_NAME);
    });

    it('does not resolve the display name on GET /views/{viewId}', async () => {
        const res = await client.as('admin').get(`/views/${encodeURIComponent(DISPLAY_NAME)}`);
        expect(res.status).toBe(404);
    });

    it('does not resolve the display name in the ?view= filter on /apis', async () => {
        // The apiDao.list path — this is the one the fallback made non-deterministic,
        // since it decides which APIs a portal view shows.
        const byHandle = await client.as('admin').get(`/apis?view=${handle}`);
        expect(byHandle.status).toBe(200);

        const byDisplayName = await client.as('admin').get(`/apis?view=${encodeURIComponent(DISPLAY_NAME)}`);
        expect(byDisplayName.status).toBe(404);
    });

    it('does not resolve the display name in the ?view= filter on /mcp-servers', async () => {
        const byHandle = await client.as('admin').get(`/mcp-servers?view=${handle}`);
        expect(byHandle.status).toBe(200);

        const byDisplayName = await client.as('admin').get(`/mcp-servers?view=${encodeURIComponent(DISPLAY_NAME)}`);
        expect(byDisplayName.status).toBe(404);
    });

    it('does not delete a view addressed by its display name', async () => {
        const del = await client.as('admin').del(`/views/${encodeURIComponent(DISPLAY_NAME)}`);
        expect(del.status).toBe(404);

        // The view is still there — the display name must not have reached the delete
        // through getId while the delete itself matched on handle.
        const stillThere = await client.as('admin').get(`/views/${handle}`);
        expect(stillThere.status).toBe(200);
    });

    it('keeps an omitted ?view= filter working (no view scoping)', async () => {
        // getId short-circuits on a falsy view name and returns undefined rather than
        // 404ing — an absent filter is not a missing view.
        const res = await client.as('admin').get('/apis');
        expect(res.status).toBe(200);
    });
});
