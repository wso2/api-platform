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

// API content ZIP wrapped in a top-level folder, which the portal reads from inside.
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
const WEB_ONLY = 'web/ holds only apiContent.md, api-content.hbs and images';
const rejectedEntries = (res) => {
    expect(res.status).toBe(400);
    expect(res.body.code).toBe('COMMON_VALIDATION_ERROR');
    return res.body.errors.map((e) => e.message);
};
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

        it('makes an image named in imageMetadata the thumbnail', async () => {
            const api = await createApi();
            const up = await uploadContent('publisher', api.id, createZip([{ name: 'web/pets-logo.png', content: PNG_BYTES }]), {
                imageMetadata: { 'api-icon': 'pets-logo.png' },
            });
            expect(up.status).toBe(201);
            const listed = await client.as('publisher').get(`/apis/${api.id}`);
            expect(listed.body.apiImageMetadata).toEqual(expect.objectContaining({ 'api-icon': 'pets-logo.png' }));
            expect(listed.body.apiImageMetadata['pets-logo']).toBeUndefined();
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
            expect(res.body.errors[0].message).toBe('The upload has no content to add');
        });

        it('rejects files the portal does not store, listing each', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, createZip([{ name: 'web/style.css', content: 'a{}' }, { name: 'web/app.js', content: '1' }]));
            expect(rejectedEntries(res)).toEqual([`web/app.js: ${WEB_ONLY}`, `web/style.css: ${WEB_ONLY}`]);
        });

        it('accepts an upload whose only content is a link', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, createZip([]), { docMetadata: link('Guide') });
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

    describe('delete matches the type exactly', () => {
        it('rejects a missing or disallowed type and removes nothing', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['A.md']))).status).toBe(201);

            expect((await client.as('publisher').del(`/apis/${api.id}/assets`)).status).toBe(400);
            for (const type of ['A', '_', '%25', 'DOC', 'image', 'API_DEFINITION', 'SCHEMA_DEFINITION']) {
                const res = await client.as('publisher').del(`/apis/${api.id}/assets?type=${type}`);
                expect(res.status).toBe(400);
                expect(res.body.code).toBe('COMMON_VALIDATION_ERROR');
            }
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(200);
            await definitionReadable(api.id);
        });

        it('deletes documents stored under folder names with spaces or dots', async () => {
            const api = await createApi();
            const zip = createZip([
                { name: 'docs/How To/setup.md', content: '# Setup' },
                { name: 'docs/v1.0/notes.md', content: '# Notes' },
                { name: 'docs/guide.md', content: '# Guide' },
            ]);
            expect((await uploadContent('publisher', api.id, zip)).status).toBe(201);

            const del = (type, fileName) => client.as('publisher')
                .del(`/apis/${api.id}/assets?type=${encodeURIComponent(type)}&fileName=${fileName}`);
            expect((await del('DOC_How To', 'setup.md')).status).toBe(204);
            expect((await del('DOC_v1.0', 'notes.md')).status).toBe(204);
            expect((await getAsset(api.id, encodeURIComponent('DOC_How To'), 'setup.md')).status).toBe(404);
            expect((await getAsset(api.id, 'DOC_v1.0', 'notes.md')).status).toBe(404);
            expect((await getAsset(api.id, 'DOC_Other', 'guide.md')).status).toBe(200);
        });

        it('removes only the named file, or only the exact type', async () => {
            const api = await createApi();
            const zip = createZip([
                { name: 'docs/HowTo/A.md', content: '# A' },
                { name: 'docs/HowTo/B.md', content: '# B' },
                { name: 'docs/C.md', content: '# C' },
            ]);
            expect((await uploadContent('publisher', api.id, zip)).status).toBe(201);

            expect((await client.as('publisher').del(`/apis/${api.id}/assets?type=DOC_HowTo&fileName=A.md`)).status).toBe(204);
            expect((await getAsset(api.id, 'DOC_HowTo', 'A.md')).status).toBe(404);
            expect((await getAsset(api.id, 'DOC_HowTo', 'B.md')).status).toBe(200);

            expect((await client.as('publisher').del(`/apis/${api.id}/assets?type=DOC_HowTo`)).status).toBe(204);
            expect((await getAsset(api.id, 'DOC_HowTo', 'B.md')).status).toBe(404);
            expect((await getAsset(api.id, 'DOC_Other', 'C.md')).status).toBe(200);
        });

        it('answers 404 when an allowed type has nothing stored', async () => {
            const api = await createApi();
            expect((await client.as('publisher').del(`/apis/${api.id}/assets?type=DOC_Samples`)).status).toBe(404);
        });
    });

    describe('invalid input is a 400 naming the cause', () => {
        it('rejects metadata that is not JSON on add and replace', async () => {
            const api = await createApi();
            for (const method of ['post', 'put']) {
                const doc = await uploadContent('publisher', api.id, docsZip(['A.md']), { method, docMetadata: 'not-json' });
                expect(doc.status).toBe(400);
                expect(doc.body.errors[0].message).toBe('docMetadata is not valid JSON');
                const img = await uploadContent('publisher', api.id, docsZip(['A.md']), { method, imageMetadata: 'not-json' });
                expect(img.status).toBe(400);
                expect(img.body.errors[0].message).toBe('imageMetadata is not valid JSON');
            }
        });

        it('rejects imageMetadata naming a file that is not uploaded, and stores nothing', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, buildContentZip(), { imageMetadata: { 'api-icon': 'icon.png' } });
            expect(res.status).toBe(400);
            expect(res.body.errors[0].message).toBe('imageMetadata names a file that is not in the upload: icon.png');
            expect((await getAsset(api.id, 'IMAGE', 'api-icon.png')).status).toBe(404);
        });

        it('rejects imageMetadata naming one file for two slots', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, buildContentZip(), {
                imageMetadata: { 'api-icon': 'api-icon.png', banner: 'api-icon.png' },
            });
            expect(res.status).toBe(400);
            expect(res.body.errors[0].message).toBe('imageMetadata names api-icon.png for more than one slot: api-icon, banner');
        });

        it('rejects two unnamed images that use the same slot, unless imageMetadata names one', async () => {
            const api = await createApi();
            const zip = () => createZip([
                { name: 'web/api-icon.png', content: PNG_BYTES },
                { name: 'web/api-icon.jpg', content: PNG_BYTES_ALT },
            ]);
            const clash = await uploadContent('publisher', api.id, zip());
            expect(clash.status).toBe(400);
            expect(clash.body.errors[0].message)
                .toBe('Images api-icon.jpg, api-icon.png use the same image slot api-icon; name one in imageMetadata');

            const named = await uploadContent('publisher', api.id, zip(), { imageMetadata: { 'api-icon': 'api-icon.jpg' } });
            expect(named.status).toBe(201);
            const listed = await client.as('publisher').get(`/apis/${api.id}`);
            expect(listed.body.apiImageMetadata['api-icon']).toBe('api-icon.jpg');
        });

        it('rejects a link type outside DOC_*', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, docsZip(['A.md']), { docMetadata: link('Guide', 'API_DEFINITION') });
            expect(res.status).toBe(400);
            expect(res.body.errors[0].message).toBe('Invalid link type: API_DEFINITION');
        });

        it('rejects an unknown top-level folder and changes nothing', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['A.md']))).status).toBe(201);

            const zip = createZip([{ name: 'docs/B.md', content: '# B' }, { name: 'doc/C.md', content: '# C' }]);
            const res = await uploadContent('publisher', api.id, zip, { method: 'put' });
            expect(rejectedEntries(res)).toEqual(['doc/: unknown top-level entry']);
            expect(res.body.message).toBe('Bad Request');
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(200);
        });

        it('rejects a document that is not .md on add and replace, and changes nothing', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['A.md']))).status).toBe(201);

            const zip = docsZip(['B.md', 'diagram.txt'], 'HowTo/');
            for (const method of ['post', 'put']) {
                const res = await uploadContent('publisher', api.id, zip, { method });
                expect(rejectedEntries(res)).toEqual(['docs/HowTo/diagram.txt: documents must be Markdown (.md)']);
            }
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_HowTo', 'B.md')).status).toBe(404);
        });

        it('ignores hidden files and folders in docs/', async () => {
            const api = await createApi();
            const zip = createZip([
                { name: 'docs/A.md', content: '# A' },
                { name: 'docs/.gitkeep', content: '' },
                { name: 'docs/.git/config', content: 'x' },
            ]);
            expect((await uploadContent('publisher', api.id, zip, { method: 'put' })).status).toBe(201);
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_Other', '.gitkeep')).status).toBe(404);
        });

        it('ignores OS-generated entries', async () => {
            const api = await createApi();
            const zip = createZip([
                { name: 'web/api-content.hbs', content: HBS_BODY },
                { name: '__MACOSX/web/._api-content.hbs', content: 'x' },
                { name: '.DS_Store', content: 'x' },
            ]);
            expect((await uploadContent('publisher', api.id, zip)).status).toBe(201);
        });

        it('ignores hidden files at the top level and in web/', async () => {
            const api = await createApi();
            const zip = createZip([
                { name: 'docs/A.md', content: '# A' },
                { name: '.gitkeep', content: '' },
                { name: 'web/.DS_Store', content: 'x' },
                { name: 'web/.x.png', content: PNG_BYTES },
            ]);
            expect((await uploadContent('publisher', api.id, zip)).status).toBe(201);
            expect((await getAsset(api.id, 'IMAGE', '.x.png')).status).toBe(404);
        });
    });

    describe('one set of entry rules', () => {
        const validZip = (extra = []) => createZip([
            { name: 'web/apiContent.md', content: '# Pets' },
            { name: 'web/api-icon.png', content: PNG_BYTES },
            { name: 'docs/A.md', content: '# A' },
            { name: 'docs/How To/B.md', content: '# B' },
            ...extra,
        ]);

        it('stores each accepted entry as its row says', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, validZip(), { method: 'put' })).status).toBe(201);
            expect((await getAsset(api.id, 'MARKETING', 'apiContent.md')).status).toBe(200);
            expect((await getAsset(api.id, 'IMAGE', 'api-icon.png')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_How To', 'B.md')).status).toBe(200);
        });

        it('gives the same result with hidden entries beside them', async () => {
            const api = await createApi();
            const zip = validZip([
                { name: '.gitkeep', content: '' },
                { name: 'docs/.git/config', content: 'x' },
                { name: 'docs/._A.md', content: 'x' },
                { name: '__MACOSX/docs/._B.md', content: 'x' },
            ]);
            expect((await uploadContent('publisher', api.id, zip, { method: 'put' })).status).toBe(201);
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_Other', '._A.md')).status).toBe(404);
        });

        it('lists every rejected entry in one 400 and changes nothing', async () => {
            const api = await createApi();
            expect((await uploadContent('publisher', api.id, docsZip(['Kept.md']))).status).toBe(201);
            const zip = validZip([
                { name: 'web/style.css', content: 'a{}' },
                { name: 'web/images/logo.png', content: PNG_BYTES },
                { name: 'web/about.html', content: '<p>' },
                { name: 'docs/guide.pdf', content: 'pdf' },
                { name: 'docs/HowTo/v1/C.md', content: '# C' },
            ]);
            for (const method of ['post', 'put']) {
                const res = await uploadContent('publisher', api.id, zip, { method });
                expect(res.body.message).toBe('Bad Request');
                expect(rejectedEntries(res)).toEqual([
                    'docs/HowTo/v1/C.md: docs/ allows one category folder',
                    'docs/guide.pdf: documents must be Markdown (.md)',
                    `web/about.html: ${WEB_ONLY}`,
                    `web/images/logo.png: ${WEB_ONLY}`,
                    `web/style.css: ${WEB_ONLY}`,
                ]);
            }
            expect((await getAsset(api.id, 'DOC_Other', 'Kept.md')).status).toBe(200);
            expect((await getAsset(api.id, 'DOC_Other', 'A.md')).status).toBe(404);
        });

        it('rejects the landing page under a name the portal does not read', async () => {
            const api = await createApi();
            const res = await uploadContent('publisher', api.id, createZip([{ name: 'web/api-content.md', content: '# x' }]));
            expect(rejectedEntries(res)).toEqual([`web/api-content.md: ${WEB_ONLY}`]);
        });
    });

    describe('thumbnail sent with the API save', () => {
        const JPEG_BYTES = Buffer.from('ffd8ffe000104a46494600010100000100010000ffd9', 'hex');

        it('replaces an image from the content ZIP that has the same name', async () => {
            const api = await createApi();
            const zip = createZip([
                { name: 'web/api-icon.png', content: PNG_BYTES },
                { name: 'web/api-icon.jpg', content: Buffer.from('ffd8ffdb', 'hex') },
            ]);
            // api-icon.png fills the thumbnail slot, so api-icon.jpg is stored with no slot.
            const seeded = await uploadContent('publisher', api.id, zip, { imageMetadata: { 'api-icon': 'api-icon.png' } });
            expect(seeded.status).toBe(201);

            const res = await client.as('publisher').putMultipart(`/apis/${api.id}`)
                .field('metadata', JSON.stringify({ name: api.name, version: api.version, type: 'REST', endPoints: api.endPoints }))
                .attach('thumbnail', JPEG_BYTES, 'logo.jpg');
            expect(res.status).toBe(200);

            const img = await client.as('publisher').get(`/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.jpg`).responseType('blob');
            expect(img.status).toBe(200);
            expect(Buffer.compare(img.body, JPEG_BYTES)).toBe(0);
        });
    });

    describe('serving headers', () => {
        const SVG = '<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>';

        it('serves an SVG as a download, with nosniff', async () => {
            const api = await createApi();
            const zip = createZip([{ name: 'web/api-icon.svg', content: SVG }]);
            expect((await uploadContent('publisher', api.id, zip)).status).toBe(201);

            const res = await client.raw()
                .get(`${client.API_PREFIX}/apis/${api.id}/assets?type=IMAGE&fileName=api-icon.svg`)
                .responseType('blob');
            expect(res.status).toBe(200);
            expect(res.headers['content-type']).toContain('image/svg+xml');
            expect(res.headers['content-disposition']).toMatch(/^attachment; filename="?api-icon\.svg"?$/);
            expect(res.headers['x-content-type-options']).toBe('nosniff');
        });

        it('serves PNG images and documents inline, with nosniff', async () => {
            const api = await createApi();
            const zip = createZip([
                { name: 'web/api-icon.png', content: PNG_BYTES },
                { name: 'docs/Guide.md', content: '# Guide' },
            ]);
            expect((await uploadContent('publisher', api.id, zip)).status).toBe(201);

            const img = await getAsset(api.id, 'IMAGE', 'api-icon.png').responseType('blob');
            expect(img.status).toBe(200);
            expect(img.headers['content-type']).toContain('image/png');
            expect(img.headers['content-disposition']).toBeUndefined();
            expect(img.headers['x-content-type-options']).toBe('nosniff');
            expect(Buffer.compare(img.body, PNG_BYTES)).toBe(0);

            const doc = await getAsset(api.id, 'DOC_Other', 'Guide.md');
            expect(doc.status).toBe(200);
            expect(doc.headers['content-disposition']).toBeUndefined();
            expect(doc.headers['x-content-type-options']).toBe('nosniff');
            expect(String(doc.text || doc.body)).toContain('# Guide');
        });
    });
});
