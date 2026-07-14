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

// API content assets REST API — POST/PUT/GET/DELETE /apis/{apiId}/assets
// (apiMetadataService.createAPIContent / updateAPIContent / getAPIFile). Uploads a
// ZIP whose `web/` directory holds the landing-page marketing content
// (api-content.hbs → stored as MARKETING) and images (api-icon.png → stored as
// IMAGE, keyed by its role). Two behaviours matter beyond status codes:
//   1. Image bytes survive the BLOB round-trip (stored raw, served raw).
//   2. Image reads are PUBLIC (the icon renders on the public listing/landing pages
//      with no session) — anonymous callers may read type=IMAGE only, and must pass
//      the org via `orgId`; every other content type stays session-scoped.
//
// Roles: publisher holds dp:api_content_*; developer is read-only (403 on writes).

const client = require('../support/client');
const db = require('../support/db');
const { createApi } = require('../support/fixtures');
const { createZip } = require('../support/zipBuilder');

// A real 1x1 PNG — arbitrary binary whose bytes must survive the BLOB round-trip.
const PNG_BYTES = Buffer.from(
    '89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c489' +
    '0000000d49444154789c6360000002000100ffff03000006000557bfabd4000000' +
    '0049454e44ae426082',
    'hex',
);
const PNG_BYTES_ALT = Buffer.from(
    '89504e470d0a1a0a0000000d4948445200000001000000010806000000' +
    '1f15c4890000000a49444154789c6300010000050001aabbccdd0000000049454e44ae426082',
    'hex',
);

const HBS_BODY = '<section class="marketing">Buy our pets</section>';

// API content ZIP wrapped in a top-level folder so resolveZipRootPath descends into
// it and finds web/ (a web-only ZIP with web/ at the very root is treated as the
// wrapper and would look for web/web — the wrapper is the intended shape).
function buildContentZip({ iconName = 'api-icon.png', icon = PNG_BYTES, hbs = HBS_BODY } = {}) {
    return createZip([
        { name: 'bundle/web/api-content.hbs', content: hbs },
        { name: `bundle/web/${iconName}`, content: icon },
    ]);
}

async function uploadContent(role, handle, zip, { method = 'post', imageMetadata } = {}) {
    const req = method === 'put'
        ? client.as(role).putMultipart(`/apis/${handle}/assets`)
        : client.as(role).postMultipart(`/apis/${handle}/assets`);
    req.attach('apiContent', zip, 'content.zip');
    if (imageMetadata) req.field('imageMetadata', JSON.stringify(imageMetadata));
    return req;
}

describe('API content assets', () => {
    let orgUuid;

    beforeAll(async () => {
        await client.login('publisher');
        await client.login('developer');
        orgUuid = await db.findOrgUuidByHandle(client.ORG_HANDLE);
    });

    describe('upload + retrieval', () => {
        it('stores web/ content and serves the image back with matching bytes', async () => {
            const api = await createApi();
            const up = await uploadContent('publisher', api.id, buildContentZip());
            expect(up.status).toBe(201);

            const img = await client.as('publisher')
                .get(`/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png`)
                .responseType('blob');
            expect(img.status).toBe(200);
            expect(img.headers['content-type']).toContain('image/png');
            expect(Buffer.compare(img.body, PNG_BYTES)).toBe(0);
        });

        it('serves the marketing landing content back as text', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const hbs = await client.as('publisher').get(`/apis/${api.id}/assets?type=MARKETING&fileName=api-content.hbs`);
            expect(hbs.status).toBe(200);
            expect(String(hbs.text || hbs.body)).toContain('Buy our pets');
        });

        it('replaces the image via PUT and serves the new bytes', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const put = await uploadContent('publisher', api.id, buildContentZip({ icon: PNG_BYTES_ALT }), { method: 'put' });
            expect(put.status).toBe(201);

            const img = await client.as('publisher')
                .get(`/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png`)
                .responseType('blob');
            expect(img.status).toBe(200);
            expect(Buffer.compare(img.body, PNG_BYTES_ALT)).toBe(0);
        });

        it('honors an explicit imageMetadata mapping on upload', async () => {
            const api = await createApi();
            const up = await uploadContent('publisher', api.id, buildContentZip(), {
                imageMetadata: { 'api-icon': 'api-icon.png' },
            });
            expect(up.status).toBe(201);
            const img = await client.as('publisher').get(`/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png`).responseType('blob');
            expect(img.status).toBe(200);
        });

        it('404s for a content file that does not exist', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);
            const res = await client.as('publisher').get(`/apis/${api.id}/assets?type=IMAGE&fileName=missing.png`);
            expect(res.status).toBe(404);
        });
    });

    describe('public image access (no session)', () => {
        it('serves the icon to an anonymous caller that supplies orgId', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const res = await client.raw()
                .get(`${client.API_PREFIX}/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png&orgId=${orgUuid}`)
                .responseType('blob');
            expect(res.status).toBe(200);
            expect(res.headers['content-type']).toContain('image/png');
            expect(Buffer.compare(res.body, PNG_BYTES)).toBe(0);
        });

        it('rejects an anonymous image read with no orgId (401)', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const res = await client.raw().get(`${client.API_PREFIX}/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png`);
            expect(res.status).toBe(401);
        });

        it('keeps non-image content session-scoped — anonymous MARKETING read is 401 even with orgId', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const res = await client.raw()
                .get(`${client.API_PREFIX}/apis/${api.id}/assets?type=MARKETING&fileName=api-content.hbs&orgId=${orgUuid}`);
            expect(res.status).toBe(401);
        });
    });

    describe('delete', () => {
        it('deletes a single content file so it is no longer served', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const del = await client.as('publisher').del(`/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png`);
            expect(del.status).toBe(204);

            const img = await client.as('publisher').get(`/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png`);
            expect(img.status).toBe(404);
        });
    });

    describe('authorization', () => {
        it('forbids a read-only developer from uploading content', async () => {
            const api = await createApi();
            const res = await uploadContent('developer', api.id, buildContentZip());
            expect(res.status).toBe(403);
        });

        it('allows a read-only developer to read a public image (with orgId)', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);
            const img = await client.as('developer')
                .get(`/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png`)
                .responseType('blob');
            expect(img.status).toBe(200);
        });
    });
});
