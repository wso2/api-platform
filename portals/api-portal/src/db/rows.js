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

/**
 * Small helpers for app-side "eager loading": DAOs run one query for the
 * parent rows and one query per child collection (scoped with
 * `WHERE parent_uuid IN (...)`), then stitch the results together here
 * instead of relying on Sequelize's `include:`.
 */

/**
 * Groups rows by the value of `key`, preserving row order within each group.
 * Returns a Map so callers get [] (via `.get(id) || []`) for parents with no
 * matching children, matching the old `include: { required: false }` shape.
 */
function groupBy(rows, key) {
    const map = new Map();
    for (const row of rows) {
        const k = row[key];
        if (!map.has(k)) map.set(k, []);
        map.get(k).push(row);
    }
    return map;
}

/** Indexes rows by the value of `key`, keeping the last row on duplicate keys. */
function indexBy(rows, key) {
    const map = new Map();
    for (const row of rows) {
        map.set(row[key], row);
    }
    return map;
}

/**
 * Normalizes a column that has no native BOOLEAN type on some dialects
 * (sqlite stores 0/1, mssql BIT can surface as 0/1 or boolean depending on
 * driver version) into a real JS boolean. Pass through null/undefined as-is.
 */
function toBool(value) {
    if (value === null || value === undefined || typeof value === 'boolean') {
        return value;
    }
    return value === 1 || value === '1' || value === true;
}

/**
 * Normalizes a JSON/JSONB column. postgres's `pg` driver parses JSONB columns
 * into JS values automatically; sqlite/mssql store JSON as TEXT/NVARCHAR(MAX)
 * and return it as a plain string, so it needs an explicit parse. Values that
 * are already objects (postgres) or already null pass through unchanged.
 */
function parseJsonColumn(value) {
    if (value === null || value === undefined || typeof value !== 'string') {
        return value;
    }
    try {
        return JSON.parse(value);
    } catch (_err) {
        return value;
    }
}

/**
 * Coerces a value being written to a BLOB/BYTEA/VARBINARY column into a real
 * `Buffer`. Sequelize's BLOB type used to do this coercion transparently on
 * every write, so upstream code (e.g. util.js's zip/content-extraction
 * helpers) has always freely handed plain UTF-8 strings to file-content DAO
 * writes. Written as a string, a binary column round-trips as a string on
 * read — and code that expects binary data back (e.g. `new
 * TextDecoder().decode(buf)`, which requires an ArrayBufferView) throws.
 * Apply at every DAO write site for a BLOB/BYTEA/VARBINARY column; null/undefined
 * pass through unchanged (nullable content columns stay null).
 */
function toBlobBuffer(value) {
    if (value === null || value === undefined || Buffer.isBuffer(value)) {
        return value;
    }
    return Buffer.from(String(value), 'utf8');
}

module.exports = { groupBy, indexBy, toBool, parseJsonColumn, toBlobBuffer };
