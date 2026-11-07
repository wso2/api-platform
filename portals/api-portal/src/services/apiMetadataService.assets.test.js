/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
'use strict';

/* The /assets handlers: what a content ZIP is read as, and how add, replace and delete
   answer. Storage is mocked; the REST integration tests cover the real database. */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-assets-'));
test.after(() => fs.rmSync(tmpDir, { recursive: true, force: true }));

const cfgFile = path.join(tmpDir, 'config.toml');
fs.writeFileSync(cfgFile, `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(path.join(tmpDir, 'portal.db'))}
`);
process.argv.push('--config', cfgFile);

const db = require('../db/driver');
const apiDao = require('../dao/apiDao');
const apiFileDao = require('../dao/apiFileDao');
const { createZip } = require('../../it/rest-api/support/zipBuilder');
const {
    extractApiContentFromUploadedZip,
    createAPIMetadata,
    createAPIContent,
    updateAPIContent,
    deleteAPIFile,
} = require('./apiMetadataService');

const zipUpload = (entries) => ({ originalname: 'content.zip', buffer: createZip(entries) });
const read = (entries) => extractApiContentFromUploadedZip(zipUpload(entries), 'org-1', 'api-1');
const names = (items) => items.map((item) => `${item.type}:${item.fileName}`).sort();
const NOT_ACCEPTED = 'Content ZIP has entries the portal does not accept';
const rejectsWith = (messages) => (err) => {
    assert.strictEqual(err.name, 'ValidationError');
    assert.strictEqual(err.message, NOT_ACCEPTED);
    assert.deepStrictEqual(err.errors.map((e) => e.message), messages);
    return true;
};

test('a ZIP with only docs/ at the root is read as is, not as a wrapper', async () => {
    const items = await read([{ name: 'docs/A.md', content: '# A' }, { name: 'docs/HowTo/B.md', content: '# B' }]);
    assert.deepStrictEqual(names(items), ['DOC_HowTo:B.md', 'DOC_Other:A.md']);
});

test('a ZIP with only web/ at the root is read as is, not as a wrapper', async () => {
    const items = await read([{ name: 'web/apiContent.md', content: '# Hi' }, { name: 'web/api-icon.png', content: 'png' }]);
    assert.deepStrictEqual(names(items), ['IMAGE:api-icon.png', 'MARKETING:apiContent.md']);
});

test('a single wrapper folder is unwrapped', async () => {
    const items = await read([{ name: 'my-api/docs/A.md', content: '# A' }]);
    assert.deepStrictEqual(names(items), ['DOC_Other:A.md']);
});

test('an unknown top-level entry is rejected and named', async () => {
    await assert.rejects(read([{ name: 'docs/A.md', content: '# A' }, { name: 'doc/B.md', content: '# B' }]),
        rejectsWith(['doc/: unknown top-level entry']));
});

test('OS-generated and hidden entries are ignored at the top level and in web/', async () => {
    const items = await read([
        { name: 'docs/A.md', content: '# A' },
        { name: '__MACOSX/docs/._A.md', content: 'x' },
        { name: '.DS_Store', content: 'x' },
        { name: '.gitkeep', content: '' },
        { name: 'web/.DS_Store', content: 'x' },
        { name: 'web/.x.png', content: 'png' },
    ]);
    assert.deepStrictEqual(names(items), ['DOC_Other:A.md']);
});

test('every entry the rules reject is listed, sorted, in one error', async () => {
    await assert.rejects(read([
        { name: 'docs/A.md', content: '# A' },
        { name: 'web/style.css', content: 'a{}' },
        { name: 'web/images/logo.png', content: 'png' },
        { name: 'web/about.html', content: '<p>' },
        { name: 'docs/guide.pdf', content: 'pdf' },
        { name: 'docs/HowTo/v1/C.md', content: '# C' },
    ]), rejectsWith([
        'docs/HowTo/v1/C.md: docs/ allows one category folder',
        'docs/guide.pdf: documents must be Markdown (.md)',
        'web/about.html: web/ holds only apiContent.md, api-content.hbs and images',
        'web/images/logo.png: web/ holds only apiContent.md, api-content.hbs and images',
        'web/style.css: web/ holds only apiContent.md, api-content.hbs and images',
    ]));
});

test('landing pages are read as text, images and documents as bytes', async () => {
    const items = await read([
        { name: 'web/apiContent.md', content: '# Hi' },
        { name: 'web/api-icon.png', content: 'png' },
        { name: 'docs/A.md', content: '# A' },
    ]);
    const byName = Object.fromEntries(items.map((item) => [item.fileName, item.content]));
    assert.strictEqual(byName['apiContent.md'], '# Hi');
    assert.ok(Buffer.isBuffer(byName['api-icon.png']));
    assert.ok(Buffer.isBuffer(byName['A.md']));
});

test('an upload with neither web/ nor docs/ reads as no content', async () => {
    assert.deepStrictEqual(await read([{ name: 'content/', content: '' }]), []);
});

test('hidden files and folders in docs/ are ignored', async () => {
    const items = await read([
        { name: 'docs/A.md', content: '# A' },
        { name: 'docs/.gitkeep', content: '' },
        { name: 'docs/._A.md', content: 'x' },
        { name: 'docs/.git/config', content: 'x' },
        { name: 'docs/HowTo/.gitkeep', content: '' },
    ]);
    assert.deepStrictEqual(names(items), ['DOC_Other:A.md']);
});

test('a document in docs/ that is not .md is rejected and named', async () => {
    await assert.rejects(read([{ name: 'docs/A.md', content: '# A' }, { name: 'docs/HowTo/guide.pdf', content: 'pdf' }]),
        rejectsWith(['docs/HowTo/guide.pdf: documents must be Markdown (.md)']));
});

// A ZIP that keeps its central directory but not the entries it lists.
const damagedZip = () => {
    const zip = createZip([{ name: 'docs/A.md', content: '# A' }]);
    return zip.subarray(zip.indexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02])));
};

test('a ZIP whose central directory lists entries it does not hold is rejected, not read as empty', async () => {
    await assert.rejects(extractApiContentFromUploadedZip({ originalname: 'content.zip', buffer: damagedZip() }, 'o', 'a'),
        { name: 'ValidationError', message: 'Invalid zip file' });
});

// The same ZIP with ZIP64 end records: the classic end record says 0xFFFF and the ZIP64 record holds `entries`.
const asZip64 = (zip, entries, { locator = true } = {}) => {
    const end = zip.length - 22;
    const record = Buffer.alloc(56);
    record.writeUInt32LE(0x06064b50, 0);
    record.writeBigUInt64LE(44n, 4);
    record.writeUInt16LE(45, 12);
    record.writeUInt16LE(45, 14);
    record.writeBigUInt64LE(BigInt(entries), 24);
    record.writeBigUInt64LE(BigInt(entries), 32);
    record.writeBigUInt64LE(BigInt(zip.readUInt32LE(end + 12)), 40);
    record.writeBigUInt64LE(BigInt(zip.readUInt32LE(end + 16)), 48);
    const zip64Locator = Buffer.alloc(20);
    zip64Locator.writeUInt32LE(0x07064b50, 0);
    zip64Locator.writeBigUInt64LE(BigInt(end), 8);
    zip64Locator.writeUInt32LE(1, 16);
    const classicEnd = Buffer.from(zip.subarray(end));
    classicEnd.writeUInt16LE(0xFFFF, 8);
    classicEnd.writeUInt16LE(0xFFFF, 10);
    return Buffer.concat([zip.subarray(0, end), ...(locator ? [record, zip64Locator] : []), classicEnd]);
};
const readZip = (buffer) => extractApiContentFromUploadedZip({ originalname: 'content.zip', buffer }, 'o', 'a');

test('a ZIP64 archive is checked against the entry count in its ZIP64 end record', async () => {
    const valid = asZip64(createZip([{ name: 'docs/A.md', content: '# A' }]), 1);
    assert.deepStrictEqual(names(await readZip(valid)), ['DOC_Other:A.md']);
    await assert.rejects(readZip(asZip64(damagedZip(), 1)), { name: 'ValidationError', message: 'Invalid zip file' });
});

test('a ZIP that declares a ZIP64 count without a ZIP64 end record is rejected', async () => {
    await assert.rejects(readZip(asZip64(damagedZip(), 1, { locator: false })), { name: 'ValidationError', message: 'Invalid zip file' });
});

test('a ZIP declaring a huge ZIP64 entry count is rejected without walking the count', async () => {
    const before = process.memoryUsage().heapUsed;
    await assert.rejects(readZip(asZip64(createZip([{ name: 'docs/A.md', content: '# A' }]), 4e9)),
        { name: 'ValidationError', message: 'Invalid zip file' });
    assert.ok(process.memoryUsage().heapUsed - before < 100 * 1024 * 1024);
});

test('a ZIP with a long archive comment is read', async () => {
    const zip = Buffer.from(createZip([{ name: 'docs/A.md', content: '# A' }]));
    const comment = Buffer.alloc(200, 'x');
    zip.writeUInt16LE(comment.length, zip.length - 2);
    assert.deepStrictEqual(names(await readZip(Buffer.concat([zip, comment]))), ['DOC_Other:A.md']);
});

test('a rejected ZIP leaves no stream open and writes nothing once it has rejected', async (t) => {
    const unzipper = require('unzipper');
    const util = require('../utils/util');
    const spy = (owner, method) => {
        const made = [];
        const original = owner[method];
        t.mock.method(owner, method, function (...args) {
            const stream = original.apply(this, args);
            made.push(stream);
            return stream;
        });
        return made;
    };
    const reads = spy(fs, 'createReadStream');
    const writes = spy(fs, 'createWriteStream');
    const parsers = spy(unzipper, 'Parse');
    const dir = fs.mkdtempSync(path.join(tmpDir, 'slip-'));
    const zip = path.join(dir, 'content.zip');
    const out = path.join(dir, 'out');
    fs.writeFileSync(zip, createZip([
        { name: 'docs/A.md', content: Buffer.alloc(1024 * 1024, 'a') },
        { name: '../evil.txt', content: 'x' },
        { name: 'docs/B.md', content: '# B' },
    ]));

    await assert.rejects(util.unzipDirectory(zip, out, { allowEmpty: true }));

    assert.ok(reads.length && reads.every((stream) => stream.destroyed));
    assert.ok(parsers.length && parsers.every((stream) => stream.destroyed));
    assert.ok(writes.length && writes.every((stream) => stream.closed));
    fs.rmSync(dir, { recursive: true, force: true });
    await new Promise((resolve) => setTimeout(resolve, 50));
    assert.strictEqual(fs.existsSync(dir), false);
});

test('an entry over the size limit stops the extraction with no write left open', async (t) => {
    const util = require('../utils/util');
    const original = fs.createWriteStream;
    const writes = [];
    t.mock.method(fs, 'createWriteStream', function (...args) {
        const stream = original.apply(this, args);
        writes.push(stream);
        return stream;
    });
    const dir = fs.mkdtempSync(path.join(tmpDir, 'big-'));
    const zip = path.join(dir, 'content.zip');
    fs.writeFileSync(zip, createZip([{ name: 'docs/A.md', content: Buffer.alloc(11 * 1024 * 1024, 'a') }]));
    await assert.rejects(util.unzipDirectory(zip, path.join(dir, 'out'), { allowEmpty: true }));
    assert.ok(writes.every((stream) => stream.closed));
});

test('a path that appears twice in a ZIP is rejected and names the path', async () => {
    await assert.rejects(read([{ name: 'docs/A.md', content: '# 1' }, { name: 'docs/A.md', content: '# 2' }]),
        rejectsWith(['docs/A.md: appears more than once']));
});

test('a name that only contains __MACOSX is kept', async () => {
    assert.deepStrictEqual(names(await read([{ name: 'docs/my__MACOSXguide.md', content: '# G' }])), ['DOC_Other:my__MACOSXguide.md']);
});

test('unknown empty folders are rejected; one empty folder is an empty wrapper', async () => {
    await assert.rejects(read([{ name: 'foo/', content: '' }, { name: 'bar/', content: '' }]),
        rejectsWith(['bar/: unknown top-level entry', 'foo/: unknown top-level entry']));
    await assert.rejects(read([{ name: 'docs/A.md', content: '# A' }, { name: 'doc/', content: '' }]),
        rejectsWith(['doc/: unknown top-level entry']));
    assert.deepStrictEqual(await read([{ name: 'content/', content: '' }]), []);
    const wrapped = await read([{ name: 'w/', content: '' }, { name: 'w/docs/', content: '' }, { name: 'w/docs/A.md', content: '# A' }]);
    assert.deepStrictEqual(names(wrapped), ['DOC_Other:A.md']);
});

test('a file that is not a ZIP is rejected naming the content field', async () => {
    await assert.rejects(extractApiContentFromUploadedZip({ originalname: 'a.txt', buffer: Buffer.from('x') }, 'o', 'a'),
        { name: 'ValidationError', message: 'content must be a .zip file' });
    await assert.rejects(extractApiContentFromUploadedZip(undefined, 'o', 'a'),
        { name: 'ValidationError', message: 'Missing required file: content' });
});

function call(handler, { body = {}, query = {}, content, files } = {}) {
    return new Promise((resolve) => {
        const res = {
            statusCode: 200,
            status(code) { this.statusCode = code; return this; },
            type() { return this; },
            json(payload) { resolve({ status: this.statusCode, body: payload }); return this; },
            send(payload) { resolve({ status: this.statusCode, body: payload }); return this; },
        };
        const req = {
            orgId: 'org-1', user: { sub: 'admin-1' }, params: { apiId: 'pets' }, query, body,
            files: files ?? (content ? { content: [content] } : {}),
        };
        Promise.resolve(handler(req, res)).catch((error) => resolve({ status: 'THREW', body: error.message }));
    });
}

const knownApi = (t) => {
    t.mock.method(apiDao, 'getIdExcludingType', async () => 'api-1');
    t.mock.method(apiDao, 'get', async () => ({ uuid: 'api-1' }));
    t.mock.method(db, 'withTransaction', (fn) => fn({}));
};

test('add rejects existing items with a 409 naming every clash, and stores nothing', async (t) => {
    knownApi(t);
    t.mock.method(apiFileDao, 'findExisting', async (items) => items);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);

    const res = await call(createAPIContent, {
        content: zipUpload([{ name: 'docs/HowTo/A.md', content: '# A' }, { name: 'web/api-icon.png', content: 'png' }]),
    });

    assert.strictEqual(res.status, 409);
    assert.strictEqual(res.body.code, 'CONFLICT');
    assert.deepStrictEqual(res.body.errors.map((e) => e.message).sort(), [
        'Content already exists: A.md (DOC_HowTo)',
        'Content already exists: image slot api-icon',
    ]);
    assert.strictEqual(stored.mock.callCount(), 0);
});

test('add stores new items without removing anything', async (t) => {
    knownApi(t);
    t.mock.method(apiFileDao, 'findExisting', async () => []);
    const deleted = t.mock.method(apiFileDao, 'deleteAllContent', async () => 0);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);

    const res = await call(createAPIContent, { content: zipUpload([{ name: 'docs/B.md', content: '# B' }]) });

    assert.strictEqual(res.status, 201);
    assert.deepStrictEqual(res.body, { message: 'API content created successfully' });
    assert.strictEqual(deleted.mock.callCount(), 0);
    assert.strictEqual(stored.mock.callCount(), 1);
});

test('add rejects files the portal does not store with a 400 listing each, and stores nothing', async (t) => {
    knownApi(t);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);
    const res = await call(createAPIContent, {
        content: zipUpload([{ name: 'web/style.css', content: 'a{}' }, { name: 'web/app.js', content: '1' }]),
    });
    assert.strictEqual(res.status, 400);
    assert.deepStrictEqual(res.body, {
        status: 'error',
        code: 'COMMON_VALIDATION_ERROR',
        message: 'Bad Request',
        errors: [
            { message: 'web/app.js: web/ holds only apiContent.md, api-content.hbs and images' },
            { message: 'web/style.css: web/ holds only apiContent.md, api-content.hbs and images' },
        ],
    });
    assert.strictEqual(stored.mock.callCount(), 0);
});

test('add with a ZIP that has no content answers that there is nothing to add', async (t) => {
    knownApi(t);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);
    for (const entries of [[], [{ name: 'content/', content: '' }], [{ name: '.gitkeep', content: '' }]]) {
        const res = await call(createAPIContent, { content: zipUpload(entries) });
        assert.strictEqual(res.status, 400);
        assert.deepStrictEqual(res.body.errors, [{ message: 'The upload has no content to add' }]);
    }
    assert.strictEqual(stored.mock.callCount(), 0);
});

test('add accepts an empty ZIP that carries only links', async (t) => {
    knownApi(t);
    t.mock.method(apiFileDao, 'findExisting', async () => []);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);
    const res = await call(createAPIContent, {
        content: zipUpload([]),
        body: { docMetadata: JSON.stringify([{ type: 'DOC_LINK', links: [{ displayName: 'Guide', url: 'https://example.com' }] }]) },
    });
    assert.strictEqual(res.status, 201);
    assert.deepStrictEqual(names(stored.mock.calls[0].arguments[0]), ['DOC_LINK:LINK_Guide']);
});

test('an image named in imageMetadata fills that slot, not its file-name slot', async (t) => {
    knownApi(t);
    t.mock.method(apiFileDao, 'findExisting', async () => []);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);
    const res = await call(createAPIContent, {
        content: zipUpload([{ name: 'web/pets-logo.png', content: 'png' }, { name: 'web/diagram.png', content: 'png' }]),
        body: { imageMetadata: JSON.stringify({ 'api-icon': 'pets-logo.png' }) },
    });
    assert.strictEqual(res.status, 201);
    const slots = Object.fromEntries(stored.mock.calls[0].arguments[0].map((item) => [item.fileName, item.key]));
    assert.deepStrictEqual(slots, { 'pets-logo.png': 'api-icon', 'diagram.png': 'diagram' });
});

test('add rejects two links with the same name in one upload', async (t) => {
    knownApi(t);
    const link = { displayName: 'Guide', url: 'https://example.com' };
    const res = await call(createAPIContent, {
        content: zipUpload([{ name: 'docs/A.md', content: '# A' }]),
        body: { docMetadata: JSON.stringify([{ type: 'DOC_LINK', links: [link, link] }]) },
    });
    assert.strictEqual(res.status, 400);
    assert.strictEqual(res.body.errors[0].message, 'Duplicate content in upload: LINK_Guide (DOC_LINK)');
});

test('add and replace reject a document that is not .md with a 400, and store nothing', async (t) => {
    knownApi(t);
    t.mock.method(apiFileDao, 'findExisting', async () => []);
    const deleted = t.mock.method(apiFileDao, 'deleteAllContent', async () => 0);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);
    for (const handler of [createAPIContent, updateAPIContent]) {
        const res = await call(handler, {
            content: zipUpload([{ name: 'docs/A.md', content: '# A' }, { name: 'docs/notes.txt', content: 'x' }]),
        });
        assert.strictEqual(res.status, 400);
        assert.deepStrictEqual(res.body.errors, [{ message: 'docs/notes.txt: documents must be Markdown (.md)' }]);
    }
    assert.strictEqual(deleted.mock.callCount(), 0);
    assert.strictEqual(stored.mock.callCount(), 0);
});

test('a replace accepts a ZIP with no entries at all', async () => {
    assert.deepStrictEqual(await read([]), []);
});

test('add and replace reject metadata that is not JSON with a 400 naming the field', async (t) => {
    knownApi(t);
    const content = zipUpload([{ name: 'docs/A.md', content: '# A' }]);
    for (const handler of [createAPIContent, updateAPIContent]) {
        const doc = await call(handler, { content, body: { docMetadata: 'not-json' } });
        assert.strictEqual(doc.status, 400);
        assert.strictEqual(doc.body.errors[0].message, 'docMetadata is not valid JSON');
        const img = await call(handler, { content, body: { imageMetadata: '{' } });
        assert.strictEqual(img.status, 400);
        assert.strictEqual(img.body.errors[0].message, 'imageMetadata is not valid JSON');
    }
});

test('replace clears existing content and stores the upload, answering 200', async (t) => {
    knownApi(t);
    const cleared = t.mock.method(apiFileDao, 'deleteAllContent', async () => 3);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);

    const res = await call(updateAPIContent, { content: zipUpload([{ name: 'docs/A.md', content: '# A' }]) });

    assert.strictEqual(res.status, 200);
    assert.deepStrictEqual(res.body, { message: 'API content replaced successfully' });
    assert.strictEqual(cleared.mock.callCount(), 1);
    assert.deepStrictEqual(names(stored.mock.calls[0].arguments[0]), ['DOC_Other:A.md']);
});

test('replace answers 201 when the API had no content and the upload adds some', async (t) => {
    knownApi(t);
    t.mock.method(apiFileDao, 'deleteAllContent', async () => 0);
    t.mock.method(apiFileDao, 'storeMany', async () => []);

    const res = await call(updateAPIContent, { content: zipUpload([{ name: 'docs/A.md', content: '# A' }]) });

    assert.strictEqual(res.status, 201);
    assert.deepStrictEqual(res.body, { message: 'API content created successfully' });
});

test('replace with a damaged ZIP answers 400 and clears nothing', async (t) => {
    knownApi(t);
    const cleared = t.mock.method(apiFileDao, 'deleteAllContent', async () => 3);
    for (const buffer of [damagedZip(), asZip64(damagedZip(), 1)]) {
        const res = await call(updateAPIContent, { content: { originalname: 'content.zip', buffer } });
        assert.strictEqual(res.status, 400);
        assert.deepStrictEqual(res.body.errors, [{ message: 'Invalid zip file' }]);
    }
    assert.strictEqual(cleared.mock.callCount(), 0);
});

test('an artifact that fails to extract answers 400 without naming a server path', async () => {
    const buffer = createZip([{ name: 'web/', content: '' }, { name: 'web', content: 'x' }]);
    const res = await call(createAPIMetadata, { files: { artifact: [{ originalname: 'api.zip', buffer }] } });
    assert.strictEqual(res.status, 400);
    assert.deepStrictEqual(res.body.errors, [{ message: 'Invalid artifact zip file' }]);
});

test('replace with only unknown empty folders answers 400 and clears nothing', async (t) => {
    knownApi(t);
    const cleared = t.mock.method(apiFileDao, 'deleteAllContent', async () => 3);
    const res = await call(updateAPIContent, { content: zipUpload([{ name: 'foo/', content: '' }, { name: 'bar/', content: '' }]) });
    assert.strictEqual(res.status, 400);
    assert.strictEqual(cleared.mock.callCount(), 0);
});

test('replace with an empty upload clears everything and stores nothing', async (t) => {
    knownApi(t);
    const cleared = t.mock.method(apiFileDao, 'deleteAllContent', async () => 3);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);

    const res = await call(updateAPIContent, { content: zipUpload([{ name: 'content/', content: '' }]) });

    assert.strictEqual(res.status, 200);
    assert.strictEqual(cleared.mock.callCount(), 1);
    assert.strictEqual(stored.mock.callCount(), 0);
});

test('delete rejects a missing or disallowed type before touching storage', async (t) => {
    knownApi(t);
    const del = t.mock.method(apiFileDao, 'deleteAll', async () => 1);
    for (const type of ['A', '_', '%', 'DOC', 'image', 'API_DEFINITION', 'SCHEMA_DEFINITION']) {
        const res = await call(deleteAPIFile, { query: { type } });
        assert.strictEqual(res.status, 400, type);
        assert.strictEqual(res.body.errors[0].message, `Invalid content type: ${type}`);
    }
    const missing = await call(deleteAPIFile, { query: {} });
    assert.strictEqual(missing.status, 400);
    assert.strictEqual(del.mock.callCount(), 0);
});

test('delete of an allowed type answers 204, or 404 when nothing matched', async (t) => {
    knownApi(t);
    t.mock.method(apiFileDao, 'deleteAll', async () => 1);
    const removed = await call(deleteAPIFile, { query: { type: 'DOC_HowTo' } });
    assert.strictEqual(removed.status, 204);

    t.mock.method(apiFileDao, 'delete', async () => 0);
    const missing = await call(deleteAPIFile, { query: { type: 'DOC_HowTo', fileName: 'A.md' } });
    assert.strictEqual(missing.status, 404);
});

// Stands in for a transaction over the content rows, enforcing the table's two unique indexes.
const contentRows = (rows) => {
    const unique = (row) => {
        const others = rows.filter((other) => other !== row);
        if (others.some((other) => other.file_name === row.file_name)
            || (row.lookup_key && others.some((other) => other.lookup_key === row.lookup_key))) {
            throw new Error('UNIQUE constraint failed');
        }
    };
    return {
        rows,
        queryOne: async (sql, params) => (sql.includes('lookup_key = ?')
            ? rows.find((row) => row.lookup_key === params[2])
            : rows.find((row) => row.file_name === params[0])) ?? null,
        execute: async (sql, params) => {
            if (sql.startsWith('INSERT')) {
                const row = { file_name: params[3], type: params[5], lookup_key: params[6] };
                rows.push(row);
                unique(row);
            } else {
                const row = rows.find((candidate) => candidate.file_name === params[6]);
                Object.assign(row, { file_name: params[1], lookup_key: params[2], updated: true });
                unique(row);
            }
            return { rowCount: 1 };
        },
    };
};
const artifactImage = (fileName) => ({ fileName, type: 'IMAGE', content: 'png', key: path.parse(fileName).name });

test('an artifact image already stored in another slot updates that row and keeps the slot', async () => {
    const t = contentRows([{ file_name: 'pets.png', type: 'IMAGE', lookup_key: 'api-icon' }]);
    await apiFileDao.upsertMany([artifactImage('pets.png')], 'api-1', 'org-1', 'admin', t);
    assert.deepStrictEqual(t.rows, [{ file_name: 'pets.png', type: 'IMAGE', lookup_key: 'api-icon', updated: true }]);
});

test('a new artifact image file takes over its slot', async () => {
    const t = contentRows([{ file_name: 'logo.png', type: 'IMAGE', lookup_key: 'logo' }]);
    await apiFileDao.upsertMany([artifactImage('logo.jpg')], 'api-1', 'org-1', 'admin', t);
    assert.deepStrictEqual(t.rows, [{ file_name: 'logo.jpg', type: 'IMAGE', lookup_key: 'logo', updated: true }]);
});

test('an artifact keeps both files when one holds the slot the other is named for', async () => {
    const t = contentRows([{ file_name: 'p.png', type: 'IMAGE', lookup_key: 'q' }]);
    await apiFileDao.upsertMany([artifactImage('p.png'), artifactImage('q.png')], 'api-1', 'org-1', 'admin', t);
    assert.deepStrictEqual(t.rows.map((row) => [row.file_name, row.lookup_key]), [['p.png', 'q'], ['q.png', null]]);
});
