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

// Cross-cutting behavior: confirms the label/view mapping actually filters
// which APIs GET /apis?view= returns (apiViewQuery parameter). `admin` manages
// labels/views; `publisher` creates the APIs.

const client = require('../support/client');
const { createApi, uniqueHandle, createView } = require('../support/fixtures');

async function createLabel(overrides = {}) {
    const id = overrides.id || uniqueHandle('label');
    const res = await client.as('admin').post('/labels', { id, displayName: overrides.displayName || id });
    if (res.status !== 201) {
        throw new Error(`Failed to seed label: ${res.status} ${JSON.stringify(res.body)}`);
    }
    return res.body;
}

describe('view/label visibility filtering', () => {
    beforeAll(async () => {
        await client.login('admin');
        await client.login('publisher');
    });

    it('GET /apis?view={viewName} only returns APIs whose labels intersect the view', async () => {
        const labelA = await createLabel();
        const labelB = await createLabel();
        const viewA = await createView({ labels: [labelA.id] });
        const viewB = await createView({ labels: [labelB.id] });

        const apiInA = await createApi({ labels: [labelA.id] });
        const apiInB = await createApi({ labels: [labelB.id] });

        const resA = await client.as('publisher').get(`/apis?view=${viewA.id}`);
        expect(resA.status).toBe(200);
        expect(resA.body.list.some((a) => a.id === apiInA.id)).toBe(true);
        expect(resA.body.list.some((a) => a.id === apiInB.id)).toBe(false);

        const resB = await client.as('publisher').get(`/apis?view=${viewB.id}`);
        expect(resB.body.list.some((a) => a.id === apiInB.id)).toBe(true);
        expect(resB.body.list.some((a) => a.id === apiInA.id)).toBe(false);
    });

    it('an API tagged with a label only appears in views that include that label', async () => {
        const label = await createLabel();
        const matchingView = await createView({ labels: [label.id] });
        const otherLabel = await createLabel();
        const nonMatchingView = await createView({ labels: [otherLabel.id] });

        const api = await createApi({ labels: [label.id] });

        const matching = await client.as('publisher').get(`/apis?view=${matchingView.id}`);
        expect(matching.body.list.some((a) => a.id === api.id)).toBe(true);

        const nonMatching = await client.as('publisher').get(`/apis?view=${nonMatchingView.id}`);
        expect(nonMatching.body.list.some((a) => a.id === api.id)).toBe(false);
    });

    it('removing a label from a view hides previously-visible APIs from that view', async () => {
        const label = await createLabel();
        const view = await createView({ labels: [label.id] });
        const api = await createApi({ labels: [label.id] });

        const before = await client.as('publisher').get(`/apis?view=${view.id}`);
        expect(before.body.list.some((a) => a.id === api.id)).toBe(true);

        const update = await client.as('admin').put(`/views/${view.id}`, { labels: [] });
        expect(update.status).toBe(200);

        const after = await client.as('publisher').get(`/apis?view=${view.id}`);
        expect(after.body.list.some((a) => a.id === api.id)).toBe(false);
    });
});
