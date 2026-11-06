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

/* The /assets handlers: what a content ZIP is read as, and how add and replace
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
} = require('./apiMetadataService');

const zipUpload = (entries) => ({ originalname: 'content.zip', buffer: createZip(entries) });
const read = (entries, options) => extractApiContentFromUploadedZip(zipUpload(entries), 'org-1', 'api-1', 'classic', options);
const names = (items) => items.map((item) => `${item.type}:${item.fileName}`).sort();

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

test('an upload with neither web/ nor docs/ is empty for a replace and rejected for an add', async () => {
    assert.deepStrictEqual(await read([{ name: 'content/', content: '' }], { allowEmpty: true }), []);
    await assert.rejects(read([{ name: 'content/', content: '' }]),
        { name: 'ValidationError', message: 'Content ZIP must contain web/ or docs/ at the top level' });
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

test('add rejects an upload with nothing the portal stores, and stores nothing', async (t) => {
    knownApi(t);
    const stored = t.mock.method(apiFileDao, 'storeMany', async () => []);
    const res = await call(createAPIContent, {
        content: zipUpload([{ name: 'web/style.css', content: 'a{}' }, { name: 'web/app.js', content: '1' }]),
    });
    assert.strictEqual(res.status, 400);
    assert.strictEqual(res.body.errors[0].message, 'The upload has no content to add');
    assert.strictEqual(stored.mock.callCount(), 0);
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

test('a replace accepts a ZIP with no entries at all', async () => {
    assert.deepStrictEqual(await read([], { allowEmpty: true }), []);
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
