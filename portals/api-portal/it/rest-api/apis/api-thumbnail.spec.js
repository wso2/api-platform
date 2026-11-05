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

// The `thumbnail` file part and `metadata.removeThumbnail` on POST/PUT /apis
// (apiContentRules.readThumbnailChange + apiMetadataService.applyThumbnailChange).
// The thumbnail is stored in the API's 'api-icon' image slot as api-icon.png or
// api-icon.jpg, named from its detected type.

const client = require('../support/client');
const { uniqueHandle } = require('../support/fixtures');
const { createZip } = require('../support/zipBuilder');

const PNG_BYTES = Buffer.from(
    '89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c489' +
    '0000000d49444154789c6360000002000100ffff03000006000557bfabd4000000' +
    '0049454e44ae426082',
    'hex',
);
const JPEG_BYTES = Buffer.from('ffd8ffe000104a46494600010100000100010000ffd9', 'hex');
const DEFINITION = JSON.stringify({ openapi: '3.0.3', info: { title: 'x', version: '1' }, paths: {} });
const THUMBNAIL_MESSAGE = 'Thumbnail must be a PNG or JPEG image.';

const metadata = (id, extra = {}) => JSON.stringify({
    id,
    name: `Thumbnail API ${id}`,
    version: 'v1.0',
    type: 'REST',
    status: 'PUBLISHED',
    endPoints: { productionURL: `https://backend.example.invalid/${id}` },
    ...extra,
});

const create = (id, extra) => client.as('publisher').postMultipart('/apis')
    .field('metadata', metadata(id, extra))
    .attach('definition', Buffer.from(DEFINITION), 'definition.json');

const update = (id, extra) => client.as('publisher').putMultipart(`/apis/${id}`)
    .field('metadata', metadata(id, extra));

const getImage = (id, fileName) => client.as('publisher')
    .get(`/apis/${id}/assets?type=IMAGE&fileName=${fileName}`)
    .responseType('blob');

const getDoc = (id, fileName) => client.as('publisher')
    .get(`/apis/${id}/assets?type=DOC_Other&fileName=${fileName}`);

async function createWithThumbnail(bytes = PNG_BYTES) {
    const id = uniqueHandle('thumb');
    const res = await create(id).attach('thumbnail', bytes, 'logo.png');
    expect(res.status).toBe(201);
    return id;
}

async function expectThumbnail(id, fileName, bytes) {
    const img = await getImage(id, fileName);
    expect(img.status).toBe(200);
    expect(Buffer.compare(img.body, bytes)).toBe(0);
}

const expectValidationError = (res, message) => {
    expect(res.status).toBe(400);
    expect(res.body.code).toBe('COMMON_VALIDATION_ERROR');
    if (message) expect(res.body.errors[0].message).toBe(message);
};

describe('API thumbnail in the save request', () => {
    beforeAll(async () => {
        await client.login('publisher');
    });

    describe('replace', () => {
        it('creates an API with the thumbnail', async () => {
            const id = await createWithThumbnail();
            await expectThumbnail(id, 'api-icon.png', PNG_BYTES);
        });

        it('replaces a PNG thumbnail with a JPEG, leaving only the new one', async () => {
            const id = await createWithThumbnail();
            const res = await update(id).attach('thumbnail', JPEG_BYTES, 'photo.png');
            expect(res.status).toBe(200);
            await expectThumbnail(id, 'api-icon.jpg', JPEG_BYTES);
            expect((await getImage(id, 'api-icon.png')).status).toBe(404);
        });

        it('leaves the landing page and documents unchanged', async () => {
            const id = await createWithThumbnail();
            const content = createZip([
                { name: 'web/api-content.hbs', content: '<p>Landing</p>' },
                { name: 'web/diagram.png', content: PNG_BYTES },
                { name: 'docs/Guide.md', content: '# Guide' },
            ]);
            expect((await client.as('publisher').putMultipart(`/apis/${id}/assets`).attach('content', content, 'content.zip')).status).toBe(200);

            expect((await update(id).attach('thumbnail', JPEG_BYTES, 'logo.jpg')).status).toBe(200);

            await expectThumbnail(id, 'api-icon.jpg', JPEG_BYTES);
            expect((await client.as('publisher').get(`/apis/${id}/assets?type=MARKETING&fileName=api-content.hbs`)).status).toBe(200);
            expect((await getImage(id, 'diagram.png')).status).toBe(200);
            expect((await getDoc(id, 'Guide.md')).status).toBe(200);
        });
    });

    describe('remove', () => {
        it('deletes the thumbnail and keeps the documents', async () => {
            const id = await createWithThumbnail();
            expect((await update(id).attach('docs', Buffer.from('# Guide'), 'Guide.md')).status).toBe(200);

            const res = await update(id, { removeThumbnail: true });
            expect(res.status).toBe(200);
            expect((await getImage(id, 'api-icon.png')).status).toBe(404);
            expect((await getDoc(id, 'Guide.md')).status).toBe(200);
        });

        it('succeeds when there is no thumbnail', async () => {
            const id = uniqueHandle('thumb');
            expect((await create(id)).status).toBe(201);
            expect((await update(id, { removeThumbnail: true })).status).toBe(200);
        });

        it('creates without a thumbnail when removeThumbnail is true on create', async () => {
            const id = uniqueHandle('thumb');
            const res = await create(id, { removeThumbnail: true });
            expect(res.status).toBe(201);
            expect(res.body.removeThumbnail).toBeUndefined();
            expect((await getImage(id, 'api-icon.png')).status).toBe(404);
        });
    });

    describe('existing clients', () => {
        it('keeps the thumbnail when neither field is sent', async () => {
            const id = await createWithThumbnail();
            const res = await update(id, { removeThumbnail: false });
            expect(res.status).toBe(200);
            expect((await update(id)).status).toBe(200);
            await expectThumbnail(id, 'api-icon.png', PNG_BYTES);
        });

        it('treats a thumbnail part with no file name as not sent', async () => {
            const id = await createWithThumbnail();
            // What a browser sends for an empty file input.
            const boundary = '----thumbnailboundary';
            const body = Buffer.concat([
                Buffer.from(`--${boundary}\r\nContent-Disposition: form-data; name="metadata"\r\n\r\n${metadata(id)}\r\n`),
                Buffer.from(`--${boundary}\r\nContent-Disposition: form-data; name="thumbnail"; filename=""\r\n` +
                    'Content-Type: application/octet-stream\r\n\r\n\r\n'),
                Buffer.from(`--${boundary}--\r\n`),
            ]);
            const res = await client.as('publisher').putMultipart(`/apis/${id}`)
                .set('Content-Type', `multipart/form-data; boundary=${boundary}`)
                .send(body);
            expect(res.status).toBe(200);
            await expectThumbnail(id, 'api-icon.png', PNG_BYTES);
        });

        it('returns the same create response fields as before', async () => {
            const id = uniqueHandle('thumb');
            const plain = await create(id);
            const withThumbnail = await create(uniqueHandle('thumb')).attach('thumbnail', PNG_BYTES, 'logo.png');
            expect(Object.keys(withThumbnail.body).sort()).toEqual(Object.keys(plain.body).sort());
        });
    });

    describe('invalid input', () => {
        it('rejects a thumbnail together with removeThumbnail true, on create and update', async () => {
            const createId = uniqueHandle('thumb');
            expectValidationError(await create(createId, { removeThumbnail: true }).attach('thumbnail', PNG_BYTES, 'logo.png'));
            expect((await client.as('publisher').get(`/apis/${createId}`)).status).toBe(404);

            const id = await createWithThumbnail();
            expectValidationError(await update(id, { removeThumbnail: true }).attach('thumbnail', JPEG_BYTES, 'logo.jpg'));
            await expectThumbnail(id, 'api-icon.png', PNG_BYTES);
        });

        it('rejects content that is not PNG or JPEG, however it is named', async () => {
            const id = await createWithThumbnail();
            const files = [
                [Buffer.from('just some text'), 'logo.png'],
                [Buffer.alloc(0), 'empty.png'],
                [Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>'), 'logo.svg'],
            ];
            for (const [bytes, name] of files) {
                expectValidationError(await update(id).attach('thumbnail', bytes, { filename: name, contentType: 'image/png' }), THUMBNAIL_MESSAGE);
            }
            await expectThumbnail(id, 'api-icon.png', PNG_BYTES);
        });

        it('rejects two thumbnail parts', async () => {
            const id = await createWithThumbnail();
            // The request validator rejects it first: the field is a single file.
            const res = await update(id)
                .attach('thumbnail', JPEG_BYTES, 'a.jpg')
                .attach('thumbnail', JPEG_BYTES, 'b.jpg');
            expect(res.status).toBe(400);
            await expectThumbnail(id, 'api-icon.png', PNG_BYTES);
        });

        it('rejects a removeThumbnail that is not a boolean', async () => {
            const id = await createWithThumbnail();
            expectValidationError(await update(id, { removeThumbnail: 'true' }));
            await expectThumbnail(id, 'api-icon.png', PNG_BYTES);
        });

        it('rejects a thumbnail larger than the upload limit with 413', async () => {
            const id = await createWithThumbnail();
            const oversized = Buffer.concat([PNG_BYTES, Buffer.alloc(10485760 + 1024, 0)]);
            const res = await update(id).attach('thumbnail', oversized, 'big.png');
            expect(res.status).toBe(413);
        }, 30000);

        it('saves nothing when a later part of the save fails', async () => {
            const id = await createWithThumbnail();
            const res = await update(id, { name: 'Renamed', addedLabels: [], removedLabels: ['not-on-this-api'] })
                .attach('thumbnail', JPEG_BYTES, 'logo.jpg');
            expect(res.status).toBeGreaterThanOrEqual(400);

            const api = await client.as('publisher').get(`/apis/${id}`);
            expect(api.body.name).toBe(`Thumbnail API ${id}`);
            await expectThumbnail(id, 'api-icon.png', PNG_BYTES);
            expect((await getImage(id, 'api-icon.jpg')).status).toBe(404);
        });
    });
});
