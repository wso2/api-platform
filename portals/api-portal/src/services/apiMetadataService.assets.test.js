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

test('a file that is not a ZIP is rejected naming the content field', async () => {
    await assert.rejects(extractApiContentFromUploadedZip({ originalname: 'a.txt', buffer: Buffer.from('x') }, 'o', 'a'),
        { name: 'ValidationError', message: 'content must be a .zip file' });
    await assert.rejects(extractApiContentFromUploadedZip(undefined, 'o', 'a'),
        { name: 'ValidationError', message: 'Missing required file: content' });
});

function call(handler, { body = {}, query = {}, content } = {}) {
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
            files: content ? { content: [content] } : {},
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
