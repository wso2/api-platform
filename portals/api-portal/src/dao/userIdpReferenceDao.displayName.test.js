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

/*
 * The display-name claim is optional: a token without it must look the user up
 * exactly as before, an empty value must never clear a stored name, and a failure
 * to save it must not fail the lookup. REST audit fields (createdBy/updatedBy)
 * show the display name, falling back to the idp_id; resolveDisplay, which the
 * subscription webhook's subscriber_id uses, stays the idp_id.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const DAO_PATH = path.resolve(__dirname, './userIdpReferenceDao.js');
const STUBS = {
    driver: path.resolve(__dirname, '../db/driver.js'),
    findOrCreate: path.resolve(__dirname, './findOrCreateHelper.js'),
    orgContext: path.resolve(__dirname, '../utils/orgContext.js'),
    logger: path.resolve(__dirname, '../config/logger.js'),
};

function stub(file, exports) {
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
}

/**
 * Loads the DAO over an in-memory row (or none) for one idp_id, and a set of rows
 * for batch reads. Returns the DAO and a log of the inserts, updates and warnings.
 */
function loadDao({ row = null, rows = [], failUpdate = false } = {}) {
    const log = { inserted: null, updates: [], warnings: [] };
    stub(STUBS.orgContext, { getPortalId: () => 'portal-a' });
    stub(STUBS.logger, { warn: (msg, meta) => log.warnings.push({ msg, meta }) });
    stub(STUBS.findOrCreate, {
        findOrCreateSafe: async (_table, _where, insertCols) => {
            if (row) return row;
            log.inserted = insertCols;
            return { ...insertCols };
        },
    });
    stub(STUBS.driver, {
        execute: async (sql, params) => {
            if (failUpdate) throw Object.assign(new Error(String(failUpdate)), { code: 'EREQUEST' });
            log.updates.push({ sql, params });
        },
        query: async () => rows,
        queryOne: async (_sql, params) => rows.find((r) => r.uuid === params[1]),
    });
    delete require.cache[DAO_PATH];
    try {
        return { dao: require(DAO_PATH), log };
    } finally {
        for (const file of [...Object.values(STUBS), DAO_PATH]) delete require.cache[file];
    }
}

const ROWS = [
    { uuid: 'u-named', idp_id: 'sub-named', display_name: 'Grace Hopper' },
    { uuid: 'u-unnamed', idp_id: 'sub-unnamed', display_name: null },
    { uuid: 'u-blank', idp_id: 'sub-blank', display_name: '  ' },
];

test('a new user is created without the name, which is then saved, trimmed', async () => {
    const { dao, log } = loadDao();
    const uuid = await dao.resolveUuid('sub-1', '  Ada Lovelace ');
    assert.strictEqual(uuid, log.inserted.uuid);
    assert.ok(!('display_name' in log.inserted), 'the insert must not depend on the name');
    assert.deepStrictEqual(log.updates.map((u) => u.params), [['Ada Lovelace', 'portal-a', uuid]]);
});

test('a missing claim creates the row exactly as before, with no name write', async () => {
    for (const claim of [undefined, null, '', '   ', 42]) {
        const { dao, log } = loadDao();
        await dao.resolveUuid('sub-1', claim);
        assert.deepStrictEqual(Object.keys(log.inserted).sort(), ['idp_id', 'portal_id', 'uuid'], String(claim));
        assert.deepStrictEqual(log.updates, [], String(claim));
    }
});

test('an existing row is updated only for a new, non-empty name', async () => {
    const existing = { uuid: 'u-1', idp_id: 'sub-1', display_name: 'Ada' };

    for (const claim of [undefined, '', 'Ada']) {
        const { dao, log } = loadDao({ row: existing });
        assert.strictEqual(await dao.resolveUuid('sub-1', claim), 'u-1');
        assert.deepStrictEqual(log.updates, [], `claim ${JSON.stringify(claim)} must not write`);
    }

    const { dao, log } = loadDao({ row: existing });
    assert.strictEqual(await dao.resolveUuid('sub-1', 'Ada Lovelace'), 'u-1');
    assert.strictEqual(log.updates.length, 1);
    assert.deepStrictEqual(log.updates[0].params, ['Ada Lovelace', 'portal-a', 'u-1']);
});

test('a row from before display names existed gets one when a token carries it', async () => {
    const { dao, log } = loadDao({ row: { uuid: 'u-old', idp_id: 'sub-old', display_name: null } });
    await dao.resolveUuid('sub-old', 'Grace Hopper');
    assert.deepStrictEqual(log.updates[0].params, ['Grace Hopper', 'portal-a', 'u-old']);
});

test('a failure to save the name is logged without the driver message and reported', async () => {
    // SQL Server's truncation error quotes the rejected value.
    const { dao, log } = loadDao({
        row: { uuid: 'u-1', idp_id: 'sub-1', display_name: null },
        failUpdate: "String or binary data would be truncated, value 'Ada'",
    });
    assert.deepStrictEqual(await dao.resolveReference('sub-1', 'Ada'), { uuid: 'u-1', displayNameSaved: false });
    assert.strictEqual(log.warnings.length, 1);
    assert.ok(!JSON.stringify(log.warnings).includes('Ada'), 'neither the name nor the driver message may be logged');
});

test('a saved or unchanged name is reported as saved', async () => {
    const saved = loadDao({ row: { uuid: 'u-1', idp_id: 'sub-1', display_name: null } });
    assert.deepStrictEqual(await saved.dao.resolveReference('sub-1', 'Ada'), { uuid: 'u-1', displayNameSaved: true });
    const unchanged = loadDao({ row: { uuid: 'u-1', idp_id: 'sub-1', display_name: 'Ada' } });
    assert.deepStrictEqual(await unchanged.dao.resolveReference('sub-1', 'Ada'), { uuid: 'u-1', displayNameSaved: true });
});

test('names are capped to 255 UTF-16 units without splitting a character', () => {
    const { dao } = loadDao();
    assert.strictEqual(dao.normalizeDisplayName('é'.repeat(300)).length, 255);
    // SQL Server's NVARCHAR(255) counts UTF-16 units: an emoji is two.
    const emoji = dao.normalizeDisplayName('😀'.repeat(200));
    assert.strictEqual(emoji, '😀'.repeat(127));
    assert.ok(emoji.length <= 255);
});

test('control characters, NUL among them, are dropped', () => {
    const { dao } = loadDao();
    assert.strictEqual(dao.normalizeDisplayName('Ada\u0000 Lovelace'), 'Ada Lovelace');
    assert.strictEqual(dao.normalizeDisplayName('\tGrace\nHopper\r '), 'GraceHopper');
    assert.strictEqual(dao.normalizeDisplayName('\u0000\u0001'), '');
});

test('display names fall back to the idp_id, then deleted_user', async () => {
    const { dao } = loadDao({ rows: ROWS });
    const names = await dao.resolveDisplayNames(['u-named', 'u-unnamed', 'u-blank', 'u-gone', '', null, 'u-named']);
    assert.deepStrictEqual(Object.fromEntries(names), {
        'u-named': 'Grace Hopper',
        'u-unnamed': 'sub-unnamed',
        'u-blank': 'sub-blank',
        'u-gone': dao.DELETED_USER,
    });
});

test('REST audit fields show display names', async () => {
    const { dao } = loadDao({ rows: ROWS });
    const single = await dao.buildSingleAuditFields({ created_by: 'u-named', updated_by: 'u-unnamed' });
    assert.strictEqual(single.createdBy, 'Grace Hopper');
    assert.strictEqual(single.updatedBy, 'sub-unnamed');

    const list = await dao.buildListAuditFields([{ created_by: 'u-named' }, { created_by: 'u-gone' }]);
    assert.deepStrictEqual(list.map((a) => a.createdBy), ['Grace Hopper', dao.DELETED_USER]);
});

test('resolveDisplay stays the idp_id, for identities other systems match on', async () => {
    const { dao } = loadDao({ rows: ROWS });
    assert.strictEqual(await dao.resolveDisplay('u-named'), 'sub-named');
});
