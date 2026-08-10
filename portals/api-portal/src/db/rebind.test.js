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

const test = require('node:test');
const assert = require('node:assert/strict');

const { DIALECTS, DRIVER_ALIASES, normalizeDriver } = require('./rebind');

test('normalizeDriver accepts every platform-api driver spelling', () => {
    // The point of the alias table: one database.driver value configured once in
    // the umbrella chart has to be valid for platform-api and this portal alike.
    // These are exactly the six platform-api's validateDatabaseConfig accepts
    // (platform-api/config/config.go), plus this component's own 'sqlite'.
    assert.equal(normalizeDriver('sqlite'), DIALECTS.SQLITE);
    assert.equal(normalizeDriver('sqlite3'), DIALECTS.SQLITE);
    assert.equal(normalizeDriver('postgres'), DIALECTS.POSTGRES);
    assert.equal(normalizeDriver('postgresql'), DIALECTS.POSTGRES);
    assert.equal(normalizeDriver('pgx'), DIALECTS.POSTGRES);
    assert.equal(normalizeDriver('mssql'), DIALECTS.MSSQL);
    assert.equal(normalizeDriver('sqlserver'), DIALECTS.MSSQL);
});

test('normalizeDriver is case- and whitespace-insensitive', () => {
    // platform-api lowercases the same field (strings.ToLower in
    // internal/database/connection.go), so a value that boots one component
    // must not be rejected by the other purely on casing.
    assert.equal(normalizeDriver('SQLServer'), DIALECTS.MSSQL);
    assert.equal(normalizeDriver('POSTGRES'), DIALECTS.POSTGRES);
    assert.equal(normalizeDriver('  mssql  '), DIALECTS.MSSQL);
});

test('normalizeDriver returns null for anything unrecognised', () => {
    // Null, never a default — configLoader fails startup on it. A silent
    // fallback to sqlite would give each replica its own empty database file.
    for (const bogus of ['', '   ', 'mysql', 'postgres-sql', 'sqlite2', 'MSSQL_', 'oracle']) {
        assert.equal(normalizeDriver(bogus), null, `expected null for ${JSON.stringify(bogus)}`);
    }
});

test('normalizeDriver returns null for non-string input', () => {
    // A TOML table or number reaching this field must fail closed, not throw
    // a TypeError out of startup validation.
    for (const bogus of [undefined, null, 42, {}, [], true]) {
        assert.equal(normalizeDriver(bogus), null, `expected null for ${JSON.stringify(bogus)}`);
    }
});

test('every alias resolves to a declared dialect', () => {
    const canonical = new Set(Object.values(DIALECTS));
    for (const [alias, target] of Object.entries(DRIVER_ALIASES)) {
        assert.ok(canonical.has(target), `alias ${alias} maps to unknown dialect ${target}`);
        assert.equal(alias, alias.toLowerCase(), `alias ${alias} must be lowercase to be reachable`);
    }
});

test('every canonical dialect is itself an accepted alias', () => {
    // Guards the case where a new dialect is added to DIALECTS but never listed
    // in DRIVER_ALIASES — its own name would then be rejected at startup.
    for (const dialect of Object.values(DIALECTS)) {
        assert.equal(normalizeDriver(dialect), dialect);
    }
});
