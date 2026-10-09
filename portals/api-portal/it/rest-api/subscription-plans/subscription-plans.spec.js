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

// Bulk POST/PUT /subscription-plans and the limit/offset paging of the list.
// The shared org runs with DP_GENERATEDEFAULTSUBPLANS=true (docker-compose.test*.yaml),
// so these also prove bulk create/update are not skipped when default plans are on.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');

const plan = (id, displayName, limitCount = 100) => ({
    id,
    displayName,
    limits: [{ limitType: 'REQUEST_COUNT', timeUnit: 'MINUTE', timeAmount: 1, limitCount }],
});

describe('subscription plans', () => {
    const created = [];

    const track = (...ids) => created.push(...ids);

    beforeAll(async () => {
        await client.login('admin');
    });

    afterAll(async () => {
        for (const id of created) {
            await client.as('admin').del(`/subscription-plans/${id}`);
        }
    });

    it('creates several plans in one bulk POST', async () => {
        const a = uniqueHandle('bulk-a');
        const b = uniqueHandle('bulk-b');
        track(a, b);

        const res = await client.as('admin').post('/subscription-plans', [plan(a, 'Bulk A'), plan(b, 'Bulk B', 500)]);
        expect(res.status).toBe(201);
        expect(res.body.map((p) => p.id)).toEqual([a, b]);

        const got = await client.as('admin').get(`/subscription-plans/${b}`);
        expect(got.status).toBe(200);
        expect(got.body.displayName).toBe('Bulk B');
        expect(got.body.limits[0].limitCount).toBe(500);
    });

    it('rejects a bulk POST containing an existing handle and creates none of it', async () => {
        const existing = uniqueHandle('dup-existing');
        const fresh = uniqueHandle('dup-fresh');
        track(existing);
        await client.as('admin').post('/subscription-plans', [plan(existing, 'Existing')]);

        const res = await client.as('admin').post('/subscription-plans', [plan(fresh, 'Fresh'), plan(existing, 'Again')]);
        expect(res.status).toBe(409);
        expect((await client.as('admin').get(`/subscription-plans/${fresh}`)).status).toBe(404);
    });

    it('returns 200 for a bulk PUT that only updates existing plans', async () => {
        const a = uniqueHandle('upd-a');
        const b = uniqueHandle('upd-b');
        track(a, b);
        await client.as('admin').post('/subscription-plans', [plan(a, 'Old A'), plan(b, 'Old B')]);

        const res = await client.as('admin').put('/subscription-plans', [plan(a, 'New A', 200), plan(b, 'New B', 300)]);
        expect(res.status).toBe(200);
        expect(res.body.map((p) => p.displayName)).toEqual(['New A', 'New B']);

        const got = await client.as('admin').get(`/subscription-plans/${a}`);
        expect(got.body.displayName).toBe('New A');
        expect(got.body.limits[0].limitCount).toBe(200);
    });

    it('returns 201 for a bulk PUT that creates at least one plan', async () => {
        const existing = uniqueHandle('mix-existing');
        const fresh = uniqueHandle('mix-fresh');
        track(existing, fresh);
        await client.as('admin').post('/subscription-plans', [plan(existing, 'Existing')]);

        const res = await client.as('admin').put('/subscription-plans', [plan(existing, 'Renamed'), plan(fresh, 'Fresh')]);
        expect(res.status).toBe(201);
        expect((await client.as('admin').get(`/subscription-plans/${existing}`)).body.displayName).toBe('Renamed');
        expect((await client.as('admin').get(`/subscription-plans/${fresh}`)).status).toBe(200);
    });

    it('rejects an empty bulk body', async () => {
        expect((await client.as('admin').post('/subscription-plans', [])).status).toBe(400);
        expect((await client.as('admin').put('/subscription-plans', [])).status).toBe(400);
    });

    it('pages the list with limit and offset in handle order', async () => {
        // Same unique prefix, one differing letter: every collation orders these
        // a < b < c, and nothing else can sort between them. Created out of order
        // so a list without ORDER BY handle would return them as c, a, b.
        const prefix = uniqueHandle('page');
        const [a, b, c] = ['a', 'b', 'c'].map((s) => `${prefix}-${s}`);
        track(a, b, c);
        await client.as('admin').post('/subscription-plans', [plan(c, 'C'), plan(a, 'A'), plan(b, 'B')]);

        const handles = [];
        let total = 0;
        do {
            const res = await client.as('admin').get(`/subscription-plans?limit=100&offset=${handles.length}`);
            expect(res.status).toBe(200);
            expect(res.body.list.length).toBeGreaterThan(0);
            handles.push(...res.body.list.map((p) => p.id));
            total = res.body.pagination.total;
        } while (handles.length < total);
        expect(handles).toHaveLength(total);

        const start = handles.indexOf(a);
        expect(start).toBeGreaterThanOrEqual(0);
        expect(handles.slice(start, start + 3)).toEqual([a, b, c]);

        for (const [i, expected] of [a, b, c].entries()) {
            const offset = start + i;
            const page = await client.as('admin').get(`/subscription-plans?limit=1&offset=${offset}`);
            expect(page.status).toBe(200);
            expect(page.body.list.map((p) => p.id)).toEqual([expected]);
            expect(page.body.pagination.limit).toBe(1);
            expect(page.body.pagination.offset).toBe(offset);
            expect(page.body.pagination.total).toBe(total);
        }
    });

    it('rejects an out-of-range limit', async () => {
        expect((await client.as('admin').get('/subscription-plans?limit=101')).status).toBe(400);
    });
});
