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
 * dialect adapter (sqlite / postgres / mssql) from config.database.driver and
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
 *   db.binaryParam(value)       -> wrap a nullable BLOB/BYTEA/VARBINARY column's
 *                                  bind value (Buffer or null) so every adapter binds
 *                                  it correctly — see ./paramTypes.js
 *   db.runDetached(fn)          -> run fn() with no ambient transaction in scope,
 *                                  even if the caller is currently inside one
 *
 * Every DAO query is written once with ANSI SQL and positional `?`
 * placeholders; this module (via ./rebind.js) is the only place that knows
 * how each dialect's driver actually wants those placeholders expressed.
 */
const { AsyncLocalStorage } = require('node:async_hooks');
const crypto = require('node:crypto');
const { config } = require('../config/configLoader');
const rebindHelpers = require('./rebind');
const { binaryParam } = require('./paramTypes');

const dialect = config.database.driver;

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

// Design mode renders every page from disk (see app.js's designMode branch) and
// never issues a query, so it needs no database connection at all — not even the
// empty sqlite file the sqlite adapter would create on open, and not the native
// better-sqlite3 binding it would load. Return a stub whose query methods fail
// loudly: reaching one means a code path that should have been gated on
// design_mode.enabled slipped through, which we want surfaced, not silently run
// against a phantom connection.
function createDesignModeStub() {
    const refuse = () => {
        throw new Error('Database access is not available in design mode (design_mode.enabled = true).');
    };
    return {
        query: refuse,
        queryOne: refuse,
        execute: refuse,
        withTransaction: refuse,
        close: () => {},
    };
}

function loadAdapter() {
    if (config.designMode?.enabled) {
        return createDesignModeStub();
    }
    switch (dialect) {
        case rebindHelpers.DIALECTS.SQLITE:
            return require('./adapters/sqliteAdapter').createSqliteAdapter(config);
        case rebindHelpers.DIALECTS.POSTGRES:
            return require('./adapters/postgresAdapter').createPostgresAdapter(config);
        case rebindHelpers.DIALECTS.MSSQL:
            return require('./adapters/mssqlAdapter').createMssqlAdapter(config);
        default:
            throw new Error(
                `Unsupported database.driver "${dialect}" — expected one of: ` +
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

const driverApi = {
    getDialect: () => dialect,
    query: ambient('query'),
    queryOne: ambient('queryOne'),
    execute: ambient('execute'),
    withTransaction: (fn) => adapter.withTransaction((tx) => {
        const wrappedTx = wrap(tx);
        return txStorage.run(wrappedTx, () => fn(wrappedTx));
    }),
    /**
     * Runs `fn` with the ambient transaction store explicitly cleared, so any
     * bare db.query/queryOne/execute call made inside it (or inside anything
     * it awaits) always targets the module-level pool handle — never a
     * transaction the *caller* happens to still be inside.
     *
     * Needed for code that can be invoked synchronously from inside someone
     * else's withTransaction() callback via an EventEmitter, without itself
     * being part of that transaction — e.g. the webhook dispatcher's poll tick,
     * which webhooks/eventPublisher.js's bus.emit('event_published') wakes
     * immediately, synchronously, from inside the publishing caller's own
     * transaction. AsyncLocalStorage propagates that caller's `wrappedTx`
     * through the *entire* async continuation the synchronous emit kicks off
     * — well past the point where the publishing transaction itself commits —
     * so an un-guarded ambient call made later in that continuation silently
     * binds to an already-committed (mssql: already-released) transaction
     * handle instead of the pool, surfacing as mssql's
     * "Transaction has not begun. Call begin() first." (ENOTBEGUN) the moment
     * a Request is built against it. Wrapping the entry point with
     * runDetached() closes that off regardless of which caller (if any)
     * happened to be mid-transaction when the tick fired.
     */
    runDetached: (fn) => txStorage.run(undefined, fn),
    rebind: (sqlText) => rebindHelpers.rebind(dialect, sqlText),
    paginationClause: (limit, offset) => rebindHelpers.paginationClause(dialect, limit, offset),
    buildUpsert: (table, insertCols, conflictCols, updateCols) =>
        rebindHelpers.buildUpsert(dialect, table, insertCols, conflictCols, updateCols),
    bindNamedParams: (sqlText, valuesByName) => rebindHelpers.bindNamedParams(sqlText, valuesByName),
    isDuplicateKeyError: (err) => rebindHelpers.isDuplicateKeyError(dialect, err),
    binaryParam,
    /**
     * Runs `fn` guarded by a SAVEPOINT when `exec` is a live transaction handle,
     * so a caught, expected error inside it (e.g. a duplicate-key race in a
     * find-or-create) can be recovered from with ROLLBACK TO SAVEPOINT instead
     * of poisoning the rest of the transaction — see rebind.js's
     * savepointStatements() for why this matters on Postgres/MSSQL.
     *
     * When `exec` is the bare module-level db (no active transaction — compared
     * by reference, since every withTransaction() call produces a fresh wrap()
     * object), there is no multi-statement session to protect: each statement
     * already commits independently, so `fn` just runs directly.
     */
    withSavepoint: async (exec, fn) => {
        if (exec === driverApi) {
            return fn();
        }
        const name = `sp_${crypto.randomBytes(4).toString('hex')}`;
        const { create, release, rollback } = rebindHelpers.savepointStatements(dialect, name);
        await exec.execute(create);
        try {
            const result = await fn();
            if (release) {
                await exec.execute(release);
            }
            return result;
        } catch (err) {
            await exec.execute(rollback);
            throw err;
        }
    },
    close: () => adapter.close(),
};

module.exports = driverApi;
