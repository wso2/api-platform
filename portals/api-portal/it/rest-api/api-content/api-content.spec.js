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
// Roles: publisher holds dp:api_content:*; developer is read-only (403 on writes).

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

// API content ZIP wrapped in a top-level folder; resolveZipRootPath descends into it.
function buildContentZip({ iconName = 'api-icon.png', icon = PNG_BYTES, hbs = HBS_BODY } = {}) {
    return createZip([
        { name: 'bundle/web/api-content.hbs', content: hbs },
        { name: `bundle/web/${iconName}`, content: icon },
    ]);
}

async function uploadContent(role, handle, zip, { method = 'post', imageMetadata, docMetadata, base = '/apis' } = {}) {
    const req = method === 'put'
        ? client.as(role).putMultipart(`${base}/${handle}/assets`)
        : client.as(role).postMultipart(`${base}/${handle}/assets`);
    req.attach('content', zip, 'content.zip');
    if (imageMetadata) req.field('imageMetadata', typeof imageMetadata === 'string' ? imageMetadata : JSON.stringify(imageMetadata));
    if (docMetadata) req.field('docMetadata', typeof docMetadata === 'string' ? docMetadata : JSON.stringify(docMetadata));
    return req;
}

const docsZip = (names, folder = '') => createZip(names.map((name) => ({ name: `docs/${folder}${name}`, content: `# ${name}` })));
const link = (displayName, type = 'DOC_LINK') => [{ type, links: [{ displayName, url: 'https://example.com/guide' }] }];
const getAsset = (handle, type, fileName, base = '/apis') =>
    client.as('publisher').get(`${base}/${handle}/assets?type=${type}&fileName=${encodeURIComponent(fileName)}`);
const definitionReadable = async (handle) =>
    expect((await getAsset(handle, 'API_DEFINITION', 'definition.json')).status).toBe(200);

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
            expect(put.status).toBe(200);

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
        it('serves the icon to an anonymous caller', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const res = await client.raw()
                .get(`${client.API_PREFIX}/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png`)
                .responseType('blob');
            expect(res.status).toBe(200);
            expect(res.headers['content-type']).toContain('image/png');
            expect(Buffer.compare(res.body, PNG_BYTES)).toBe(0);
        });

        it('ignores orgId rather than resolving it', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const foreign = await client.raw()
                .get(`${client.API_PREFIX}/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png&orgId=00000000-0000-0000-0000-000000000000`)
                .responseType('blob');
            expect(foreign.status).toBe(200);
            expect(Buffer.compare(foreign.body, PNG_BYTES)).toBe(0);

            const own = await client.raw()
                .get(`${client.API_PREFIX}/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.png&orgId=${orgUuid}`)
                .responseType('blob');
            expect(own.status).toBe(200);
            expect(Buffer.compare(own.body, PNG_BYTES)).toBe(0);
        });

        it('keeps non-image content session-scoped — anonymous MARKETING read is 401', async () => {
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

    describe('add (POST) never removes anything', () => {
        it('adds a doc next to an existing icon and doc', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);
            expect((await uploadContent('publisher', api.id, docsZip(['A.md']))).status).toBe(201);

            const res = await uploadContent('publisher', api.id, docsZip(['B.md']));
            expect(res.status).toBe(201);
            expect(res.body).toEqual({ message: 'API content created successfully' });
            expect((await getAsset(api.id, 'IMAGE', 'api-icon.png')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_Other', 'B.md')).status).toBe(200);
        });

        it('rejects an existing doc with a 409 naming it, and stores nothing from the upload', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['A.md']))).status).toBe(201);

            const res = await uploadContent('publisher', api.id, docsZip(['A.md', 'C.md']));
            expect(res.status).toBe(409);
            expect(res.body.code).toBe('CONFLICT');
            expect(res.body.errors.map((e) => e.message)).toEqual(['Content already exists: A.md (DOC_Other)']);
            expect((await getAsset(api.id, 'DOC_Other', 'C.md')).status).toBe(404);
        });

        it('rejects a new icon when one exists, naming the image slot', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip())).status).toBe(201);

            const res = await uploadContent('publisher', api.id,
                createZip([{ name: 'web/api-icon.jpg', content: PNG_BYTES_ALT }]));
            expect(res.status).toBe(409);
            expect(res.body.errors.map((e) => e.message)).toContain('Content already exists: image slot api-icon');
        });

        it('rejects an image whose file name is already stored under another slot', async () => {
            const api = await createApi();
            const icon = createZip([{ name: 'web/icon.png', content: PNG_BYTES }]);
            expect((await uploadContent('publisher', api.id, icon, { method: 'put', imageMetadata: { 'api-icon': 'icon.png' } })).status).toBe(201);

            const res = await uploadContent('publisher', api.id, icon);
            expect(res.status).toBe(409);
            expect(res.body.errors.map((e) => e.message)).toContain('Content already exists: image slot icon');
        });

        it('rejects a link that already exists', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['A.md']), { docMetadata: link('Guide') })).status).toBe(201);

            const res = await uploadContent('publisher', api.id, docsZip(['B.md']), { docMetadata: link('Guide') });
            expect(res.status).toBe(409);
            expect(res.body.errors.map((e) => e.message)).toContain('Content already exists: LINK_Guide (DOC_LINK)');
        });

        it('rejects an empty upload', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, createZip([]));
            expect(res.status).toBe(400);
        });

        it('rejects an upload with nothing the portal stores', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, createZip([{ name: 'web/style.css', content: 'a{}' }, { name: 'web/app.js', content: '1' }]));
            expect(res.status).toBe(400);
            expect(res.body.errors[0].message).toBe('The upload has no content to add');
        });

        it('accepts an upload whose only content is a link', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, createZip([{ name: 'web/style.css', content: 'a{}' }]), { docMetadata: link('Guide') });
            expect(res.status).toBe(201);
            expect((await getAsset(api.id, 'DOC_LINK', 'LINK_Guide')).status).toBe(200);
        });

        it('rejects two entries that are the same item', async () => {
            const api = await createApi();
            const link2 = [{ type: 'DOC_LINK', links: [
                { displayName: 'Guide', url: 'https://example.com/a' },
                { displayName: 'Guide', url: 'https://example.com/b' },
            ] }];
            const res = await uploadContent('publisher', api.id, docsZip(['A.md']), { docMetadata: link2 });
            expect(res.status).toBe(400);
        });
    });

    describe('replace (PUT) mirrors the upload', () => {
        it('removes everything not in the new upload', async () => {
            const api = await createApi();
            const first = createZip([
                { name: 'docs/A.md', content: '# A' },
                { name: 'docs/B.md', content: '# B' },
                { name: 'web/api-icon.png', content: PNG_BYTES },
            ]);
            const created = await uploadContent('publisher', api.id, first, { method: 'put' });
            expect(created.status).toBe(201);
            expect(created.body).toEqual({ message: 'API content created successfully' });

            const res = await uploadContent('publisher', api.id, docsZip(['A.md']), { method: 'put' });
            expect(res.status).toBe(200);
            expect(res.body).toEqual({ message: 'API content replaced successfully' });
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_Other', 'B.md')).status).toBe(404);
            expect((await getAsset(api.id, 'IMAGE', 'api-icon.png')).status).toBe(404);
            await definitionReadable(api.id);
        });

        it('clears all content with an empty ZIP, keeping the definition', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, buildContentZip(), { docMetadata: link('Guide') })).status).toBe(201);

            const res = await uploadContent('publisher', api.id, createZip([]), { method: 'put' });
            expect(res.status).toBe(200);
            expect((await getAsset(api.id, 'IMAGE', 'api-icon.png')).status).toBe(404);
            expect((await getAsset(api.id, 'MARKETING', 'api-content.hbs')).status).toBe(404);
            expect((await getAsset(api.id, 'DOC_LINK', 'LINK_Guide')).status).toBe(404);
            await definitionReadable(api.id);
            expect((await uploadContent('publisher', api.id, createZip([]), { method: 'put' })).status).toBe(200);
        });

        it('clears all content with a ZIP holding only an empty wrapper folder', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['A.md']))).status).toBe(201);

            const res = await uploadContent('publisher', api.id, createZip([{ name: 'content/', content: '' }]), { method: 'put' });
            expect(res.status).toBe(200);
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(404);
        });

        it('drops a link that is no longer sent', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['A.md']), { method: 'put', docMetadata: link('Guide') })).status).toBe(201);
            expect((await getAsset(api.id, 'DOC_LINK', 'LINK_Guide')).status).toBe(200);

            expect((await uploadContent('publisher', api.id, docsZip(['A.md']), { method: 'put' })).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_LINK', 'LINK_Guide')).status).toBe(404);
        });

        it('reads a ZIP holding only docs/ or only web/ at the root', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['A.md'], 'HowTo/'), { method: 'put' })).status).toBe(201);
            expect((await getAsset(api.id, 'DOC_HowTo', 'A.md')).status).toBe(200);

            const webOnly = createZip([{ name: 'web/api-content.hbs', content: HBS_BODY }]);
            expect((await uploadContent('publisher', api.id, webOnly, { method: 'put' })).status).toBe(200);
            expect((await getAsset(api.id, 'MARKETING', 'api-content.hbs')).status).toBe(200);
        });
    });
});
