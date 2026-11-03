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
 * A SQLite database created by a release before user_idp_references.display_name
 * gains the column at startup, keeps its rows, and survives a second run.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');
const Database = require('better-sqlite3');
const { addMissingSqliteColumns } = require('./sqliteColumns');

function driverOver(sqlite) {
    return {
        queryOne: async (sql, params) => sqlite.prepare(sql).get(...params),
        execute: async (sql) => sqlite.exec(sql),
    };
}

test('display_name is added to a previous-release table, idempotently', async () => {
    const sqlite = new Database(':memory:');
    try {
        sqlite.exec(`CREATE TABLE user_idp_references (
            portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
            uuid VARCHAR(40) NOT NULL,
            idp_id VARCHAR(255) NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (portal_id, uuid),
            UNIQUE (portal_id, idp_id)
        );
        INSERT INTO user_idp_references (portal_id, uuid, idp_id) VALUES ('p', 'u-old', 'sub-old');`);

        await addMissingSqliteColumns(driverOver(sqlite));
        await addMissingSqliteColumns(driverOver(sqlite));

        const row = sqlite.prepare("SELECT idp_id, display_name FROM user_idp_references WHERE uuid = 'u-old'").get();
        assert.deepStrictEqual(row, { idp_id: 'sub-old', display_name: null });
    } finally {
        sqlite.close();
    }
});

test('the current schema already has the column, so nothing is altered', async () => {
    const sqlite = new Database(':memory:');
    try {
        sqlite.exec(fs.readFileSync(path.join(__dirname, '../../database/schema.sqlite.sql'), 'utf8'));
        const executed = [];
        const db = driverOver(sqlite);
        await addMissingSqliteColumns({ ...db, execute: async (sql) => { executed.push(sql); return db.execute(sql); } });
        assert.deepStrictEqual(executed, []);
    } finally {
        sqlite.close();
    }
});
