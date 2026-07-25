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
 * Dialect-portability helpers for the hand-written SQL layer. Every DAO writes
 * queries once with positional `?` placeholders and ANSI syntax; these helpers
 * translate the handful of things that genuinely differ per database.
 *
 * This mirrors platform-api's internal/database/connection.go (Rebind,
 * PaginationClause, BuildUpsertQuery, IsDuplicateKeyError) and gateway-controller's
 * sqlx.Rebind usage, so all three services share the same "write once with ?"
 * convention. Keeping the three services consistent is intentional — see
 * .claude/rules/authentication_authorization.md and file-access.md, which apply
 * across the platform, not just one service.
 */

const DIALECTS = Object.freeze({
    SQLITE: 'sqlite',
    POSTGRES: 'postgres',
    MSSQL: 'mssql',
});

/**
 * Converts a SQL string written with positional `?` placeholders into the
 * placeholder syntax the active dialect's driver expects.
 *
 *   postgres -> $1, $2, $3, ...
 *   mssql    -> @p1, @p2, @p3, ...
 *   sqlite   -> left as `?` (better-sqlite3 accepts positional `?` natively)
 *
 * Placeholders inside string literals are not distinguished from real ones —
 * as with platform-api's Rebind, callers must not embed literal `?` characters
 * in query text (use a bind parameter instead, which every query here already does).
 */
function rebind(dialect, sqlText) {
    if (dialect === DIALECTS.POSTGRES) {
        let i = 0;
        return sqlText.replace(/\?/g, () => `$${++i}`);
    }
    if (dialect === DIALECTS.MSSQL) {
        let i = 0;
        return sqlText.replace(/\?/g, () => `@p${++i}`);
    }
    return sqlText;
}

/**
 * Dialect-appropriate row-limiting clause for pagination. SQL Server has no
 * LIMIT keyword and instead uses ANSI OFFSET/FETCH, which (a) requires an
 * ORDER BY in the statement and (b) lists OFFSET before the row count — the
 * reverse of "LIMIT ? OFFSET ?". The returned clause uses `?` placeholders;
 * pass the assembled query through rebind() as usual, with these params
 * appended after the query's own bind params (in the returned order).
 */
function paginationClause(dialect, limit, offset) {
    if (dialect === DIALECTS.MSSQL) {
        return { clause: 'OFFSET ? ROWS FETCH NEXT ? ROWS ONLY', params: [offset, limit] };
    }
    return { clause: 'LIMIT ? OFFSET ?', params: [limit, offset] };
}

/**
 * Builds a dialect-appropriate "insert, or update on conflict" statement.
 * Always uses positional `?` placeholders — pass the result through the
 * normal query()/execute() path (which calls rebind()), not through this
 * module directly.
 *
 *   table        - target table name
 *   insertCols   - all columns being inserted, in the order values will be bound
 *   conflictCols - columns whose uniqueness triggers the update path
 *   updateCols   - subset of insertCols to overwrite on conflict
 *                  (defaults to insertCols minus conflictCols; pass [] for
 *                  INSERT-or-ignore semantics, e.g. Sequelize's ignoreDuplicates)
 *
 * Bind params are always supplied in insertCols order, regardless of dialect —
 * the mssql MERGE form below only references each `?` once (in the USING
 * clause), so no param duplication is needed.
 */
function buildUpsert(dialect, table, insertCols, conflictCols, updateCols) {
    const updates = updateCols !== undefined
        ? updateCols
        : insertCols.filter((c) => !conflictCols.includes(c));

    if (dialect === DIALECTS.MSSQL) {
        return buildMssqlMerge(table, insertCols, conflictCols, updates);
    }

    const placeholders = insertCols.map(() => '?').join(', ');
    if (updates.length === 0) {
        return `INSERT INTO ${table} (${insertCols.join(', ')}) VALUES (${placeholders}) ` +
            `ON CONFLICT (${conflictCols.join(', ')}) DO NOTHING`;
    }
    const setClause = updates.map((c) => `${c} = excluded.${c}`).join(', ');
    return `INSERT INTO ${table} (${insertCols.join(', ')}) VALUES (${placeholders}) ` +
        `ON CONFLICT (${conflictCols.join(', ')}) DO UPDATE SET ${setClause}`;
}

function buildMssqlMerge(table, insertCols, conflictCols, updateCols) {
    const sourceCols = insertCols.map((c) => `? AS ${c}`).join(', ');
    const onClause = conflictCols.map((c) => `target.${c} = source.${c}`).join(' AND ');
    const insertColsList = insertCols.join(', ');
    const insertValsList = insertCols.map((c) => `source.${c}`).join(', ');

    let sql = `MERGE INTO ${table} AS target\n` +
        `USING (SELECT ${sourceCols}) AS source\n` +
        `ON ${onClause}\n`;
    if (updateCols.length > 0) {
        const updateSet = updateCols.map((c) => `target.${c} = source.${c}`).join(', ');
        sql += `WHEN MATCHED THEN UPDATE SET ${updateSet}\n`;
    }
    sql += `WHEN NOT MATCHED THEN INSERT (${insertColsList}) VALUES (${insertValsList});`;
    return sql;
}

/**
 * Converts a SQL string written with named `:param` placeholders (readable in
 * large hand-written `.sql` files, e.g. database/queries/search-apis.postgres.sql)
 * into positional `?` placeholders plus an ordered params array, so it can be
 * passed through the normal query()/execute() path like every other query here.
 *
 * This is the one bit of "prepared statement templating" Sequelize used to do
 * for us via its `replacements` option — `pg`/`better-sqlite3`/`mssql` have no
 * named-parameter support of their own, so it has to happen at this layer.
 *
 * A named token is only recognized when its leading `:` is not itself preceded
 * by another `:` — this deliberately skips PostgreSQL's `::type` cast operator
 * (e.g. `:viewId::uuid` binds `:viewId` once and leaves the `::uuid` cast alone;
 * without this guard, `::uuid` would be misparsed as a bogus `:uuid` parameter).
 * A name used more than once in the SQL text is looked up once per occurrence,
 * in left-to-right order, so repeated params are bound correctly.
 *
 * `--` line comments are matched too, but only to skip them — a `:name`-shaped
 * mention in a doc comment (e.g. this project's own header comments listing
 * each parameter) must not be bound as a real placeholder. Postgres strips
 * comments before parsing, so a bogus match there would consume a low `$N`
 * slot that never actually appears in the executable SQL, and the driver
 * fails with "could not determine data type of parameter $1" — silently
 * shifting every real placeholder's index in the process. None of this
 * codebase's .sql files use `--` inside a string literal or `/* *\/` block
 * comments, so a first-`--`-per-line split is sufficient without a full
 * SQL tokenizer.
 */
function bindNamedParams(sqlText, valuesByName) {
    const params = [];
    const sql = sqlText
        .split('\n')
        .map((line) => {
            const commentIdx = line.indexOf('--');
            const code = commentIdx === -1 ? line : line.slice(0, commentIdx);
            const comment = commentIdx === -1 ? '' : line.slice(commentIdx);
            const replacedCode = code.replace(/(?<!:):(\w+)/g, (_match, name) => {
                if (!Object.prototype.hasOwnProperty.call(valuesByName, name)) {
                    throw new Error(`bindNamedParams: missing value for :${name}`);
                }
                params.push(valuesByName[name]);
                return '?';
            });
            return replacedCode + comment;
        })
        .join('\n');
    return { sql, params };
}

/**
 * Returns the create/release/rollback SQL for a savepoint named `name`, in the
 * active dialect's syntax. Used to recover a transaction after a caught,
 * expected error (e.g. a duplicate-key race in a find-or-create) — see
 * db.withSavepoint() in driver.js for why this is necessary on Postgres/MSSQL
 * but not SQLite: on Postgres, any error inside a transaction poisons the
 * *entire* transaction ("current transaction is aborted, commands ignored
 * until end of transaction block") until a ROLLBACK or ROLLBACK TO SAVEPOINT
 * runs — a plain catch-and-continue silently fails every statement after it.
 *
 * MSSQL has no explicit "release" statement — a savepoint is automatically
 * discarded on commit or superseded by the next SAVE TRANSACTION with the
 * same name, so `release` is null there.
 */
function savepointStatements(dialect, name) {
    if (dialect === DIALECTS.MSSQL) {
        return {
            create: `SAVE TRANSACTION ${name}`,
            release: null,
            rollback: `ROLLBACK TRANSACTION ${name}`,
        };
    }
    return {
        create: `SAVEPOINT ${name}`,
        release: `RELEASE SAVEPOINT ${name}`,
        rollback: `ROLLBACK TO SAVEPOINT ${name}`,
    };
}

/**
 * Reports whether `err` is a unique-constraint / duplicate-key violation for
 * the active dialect. Replaces `error instanceof Sequelize.UniqueConstraintError`
 * checks throughout the DAOs.
 */
function isDuplicateKeyError(dialect, err) {
    if (!err) return false;
    if (dialect === DIALECTS.POSTGRES) {
        return err.code === '23505';
    }
    if (dialect === DIALECTS.SQLITE) {
        if (err.code === 'SQLITE_CONSTRAINT_UNIQUE' || err.code === 'SQLITE_CONSTRAINT_PRIMARYKEY') {
            return true;
        }
        return typeof err.message === 'string' && err.message.includes('UNIQUE constraint failed');
    }
    if (dialect === DIALECTS.MSSQL) {
        // 2601: Cannot insert duplicate key row in object with unique index
        // 2627: Violation of PRIMARY KEY or UNIQUE KEY constraint
        return err.number === 2601 || err.number === 2627;
    }
    return false;
}

module.exports = {
    DIALECTS,
    rebind,
    paginationClause,
    buildUpsert,
    bindNamedParams,
    savepointStatements,
    isDuplicateKeyError,
};
