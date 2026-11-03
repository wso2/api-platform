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
 * Columns added to tables that have already shipped. SQLite has no
 * ADD COLUMN IF NOT EXISTS, so ensureSchema (src/server.js) adds each one an
 * existing database lacks; the Postgres and SQL Server schema files carry
 * guarded ALTERs instead, and database/upgrade/ holds the same as standalone
 * scripts. Table and column names come only from this fixed list.
 */
const SQLITE_ADDED_COLUMNS = [
    { table: 'user_idp_references', column: 'display_name', definition: 'VARCHAR(255)' },
];

/**
 * Adds every SQLITE_ADDED_COLUMNS entry the database lacks.
 * @param {{ queryOne: Function, execute: Function }} db the driver API
 */
async function addMissingSqliteColumns(db, columns = SQLITE_ADDED_COLUMNS) {
    for (const { table, column, definition } of columns) {
        const row = await db.queryOne(
            'SELECT COUNT(*) AS n FROM pragma_table_info(?) WHERE name = ?', [table, column]);
        if (Number(row?.n) > 0) continue;
        await db.execute(`ALTER TABLE ${table} ADD COLUMN ${column} ${definition}`);
    }
}

module.exports = { SQLITE_ADDED_COLUMNS, addMissingSqliteColumns };
