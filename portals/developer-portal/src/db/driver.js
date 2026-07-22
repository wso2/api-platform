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
 * Single entry point DAOs use instead of Sequelize models. Selects the
 * dialect adapter (sqlite / postgres / mssql) from config.database.type and
 * exposes a uniform, dialect-agnostic API:
 *
 *   db.query(sql, params)      -> Promise<row[]>
 *   db.queryOne(sql, params)   -> Promise<row|null>
 *   db.execute(sql, params)    -> Promise<{ rowCount, insertId }>
 *   db.withTransaction(fn)     -> Promise<T>   -- fn(tx) gets the same query/queryOne/execute shape
 *   db.getDialect()            -> 'sqlite' | 'postgres' | 'mssql'
 *   db.rebind(sql)             -> translate `?` placeholders for the active dialect (rarely needed
 *                                  directly — query/queryOne/execute/withTransaction already rebind)
 *   db.paginationClause(limit, offset)
 *   db.buildUpsert(table, insertCols, conflictCols, updateCols)
 *   db.isDuplicateKeyError(err)
 *
 * Every DAO query is written once with ANSI SQL and positional `?`
 * placeholders; this module (via ./rebind.js) is the only place that knows
 * how each dialect's driver actually wants those placeholders expressed.
 */
const { AsyncLocalStorage } = require('node:async_hooks');
const { config } = require('../config/configLoader');
const rebindHelpers = require('./rebind');

const dialect = config.database.type;

/**
 * Tracks the active transaction handle for the current async call chain, so a
 * DAO function that forgets to thread its `t`/`transaction` parameter through
 * (and instead calls the bare, module-level db.query/queryOne/execute) still
 * transparently participates in an already-open withTransaction() rather than
 * escaping it or deadlocking against it.
 *
 * Without this: on sqlite (a single shared connection guarded by one lock —
 * see adapters/sqliteAdapter.js), a bare db.* call made from inside an active
 * transaction's callback would try to re-acquire the same lock that callback
 * itself is holding, and hang forever — every other request then queues up
 * behind it until async-lock's pending-queue limit is hit ("Too many pending
 * tasks in queue"). On postgres/mssql, the bare call would instead run on a
 * *different* connection than the transaction's checked-out client, silently
 * escaping the transaction (wrong isolation, or a lock-wait against the
 * transaction's own uncommitted changes). This makes the "forgotten t" case
 * behave correctly on all three dialects instead of failing in three
 * different ways.
 */
const txStorage = new AsyncLocalStorage();

function loadAdapter() {
    switch (dialect) {
        case rebindHelpers.DIALECTS.SQLITE:
            return require('./adapters/sqliteAdapter').createSqliteAdapter(config);
        case rebindHelpers.DIALECTS.POSTGRES:
            return require('./adapters/postgresAdapter').createPostgresAdapter(config);
        case rebindHelpers.DIALECTS.MSSQL:
            return require('./adapters/mssqlAdapter').createMssqlAdapter(config);
        default:
            throw new Error(
                `Unsupported database.type "${dialect}" — expected one of: ` +
                `${Object.values(rebindHelpers.DIALECTS).join(', ')}`
            );
    }
}

const adapter = loadAdapter();

function wrap(handle) {
    return {
        getDialect: () => dialect,
        query: (sqlText, params) => handle.query(rebindHelpers.rebind(dialect, sqlText), params),
        queryOne: (sqlText, params) => handle.queryOne(rebindHelpers.rebind(dialect, sqlText), params),
        execute: (sqlText, params) => handle.execute(rebindHelpers.rebind(dialect, sqlText), params),
    };
}

const wrapped = wrap(adapter);

// Ambient-transaction-aware versions of query/queryOne/execute: if called while
// a withTransaction() callback is active anywhere up the current async chain,
// route to that transaction's handle instead of the module-level (lock/pool)
// handle — see txStorage above.
function ambient(method) {
    return (sqlText, params) => {
        const active = txStorage.getStore();
        return active ? active[method](sqlText, params) : wrapped[method](sqlText, params);
    };
}

module.exports = {
    getDialect: () => dialect,
    query: ambient('query'),
    queryOne: ambient('queryOne'),
    execute: ambient('execute'),
    withTransaction: (fn) => adapter.withTransaction((tx) => {
        const wrappedTx = wrap(tx);
        return txStorage.run(wrappedTx, () => fn(wrappedTx));
    }),
    rebind: (sqlText) => rebindHelpers.rebind(dialect, sqlText),
    paginationClause: (limit, offset) => rebindHelpers.paginationClause(dialect, limit, offset),
    buildUpsert: (table, insertCols, conflictCols, updateCols) =>
        rebindHelpers.buildUpsert(dialect, table, insertCols, conflictCols, updateCols),
    bindNamedParams: (sqlText, valuesByName) => rebindHelpers.bindNamedParams(sqlText, valuesByName),
    isDuplicateKeyError: (err) => rebindHelpers.isDuplicateKeyError(dialect, err),
    close: () => adapter.close(),
};
