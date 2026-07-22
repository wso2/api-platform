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

const db = require('../db/driver');

/**
 * SELECT-then-INSERT that survives a race between concurrent requests
 * creating the same row: on a unique-constraint violation, falls back to a
 * plain lookup instead of failing. Shared by DAOs (userIdpReferenceDao,
 * userOrganizationMappingDao, tagDao, labelDao) that need idempotent
 * find-or-create semantics under concurrent access.
 *
 *   table      - table name
 *   whereCols  - column -> value used both to look the row up and to re-look
 *                it up after losing the insert race
 *   insertCols - full column -> value set for the INSERT (superset of whereCols,
 *                including any generated primary key)
 *   exec       - db or an open transaction handle (defaults to the module-level db)
 */
const findOrCreateSafe = async (table, whereCols, insertCols, exec = db) => {
    const whereClause = Object.keys(whereCols).map((c) => `${c} = ?`).join(' AND ');
    const whereParams = Object.values(whereCols);

    const existing = await exec.queryOne(`SELECT * FROM ${table} WHERE ${whereClause}`, whereParams);
    if (existing) return existing;

    const insertColNames = Object.keys(insertCols);
    const placeholders = insertColNames.map(() => '?').join(', ');
    try {
        await exec.execute(
            `INSERT INTO ${table} (${insertColNames.join(', ')}) VALUES (${placeholders})`,
            Object.values(insertCols)
        );
    } catch (error) {
        if (!db.isDuplicateKeyError(error)) {
            throw error;
        }
        // Lost the race — another request created it first; fall through to re-select.
    }

    const created = await exec.queryOne(`SELECT * FROM ${table} WHERE ${whereClause}`, whereParams);
    if (!created) {
        // Neither our insert nor the presumed racing insert produced a row —
        // something other than a duplicate-key race went wrong.
        throw new Error(`findOrCreateSafe: row not found in "${table}" after insert`);
    }
    return created;
};

module.exports = { findOrCreateSafe };
