/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
/*
 * Generates dialect-specific DDL from Sequelize model definitions.
 *
 *   node scripts/generate-ddl.js <dialect>
 *
 * Supported dialects: postgres  mysql  mariadb  mssql  sqlite
 *
 * The script injects a fresh Sequelize instance for the target dialect into
 * the module cache before loading any model file, so model files receive the
 * right dialect-aware QueryGenerator without requiring a live database
 * connection.  Output is written to database/schema.<dialect>.sql.
 *
 * Prerequisites:
 *   postgres  — pg (already in package.json)
 *   sqlite    — sqlite3 (already in package.json)
 *   mysql     — npm install mysql2
 *   mariadb   — npm install mariadb
 *   mssql     — npm install tedious
 */
'use strict';

const path = require('path');
const fs   = require('fs');
const { Sequelize } = require('sequelize');

const SUPPORTED = ['postgres', 'mysql', 'mariadb', 'mssql', 'sqlite'];
const DRIVERS   = { postgres: 'pg', mysql: 'mysql2', mariadb: 'mariadb', mssql: 'tedious', sqlite: 'sqlite3' };

const dialect = process.argv[2];
if (!dialect || !SUPPORTED.includes(dialect)) {
    console.error('Usage: node scripts/generate-ddl.js <dialect>');
    console.error('  dialect: ' + SUPPORTED.join(' | '));
    process.exit(1);
}

// ------------------------------------------------------------------
// 1. Create a Sequelize instance for the target dialect.
//    No real connection needed — we only use the QueryGenerator.
// ------------------------------------------------------------------
let seq;
try {
    if (dialect === 'sqlite') {
        seq = new Sequelize({ dialect: 'sqlite', storage: ':memory:', logging: false });
    } else {
        seq = new Sequelize('devportal', 'user', 'pass', {
            host: 'localhost',
            dialect,
            logging: false,
        });
    }
} catch (e) {
    const driver = DRIVERS[dialect];
    console.warn(`[generate-ddl] Skipping ${dialect}: ${driver} not installed. Run: npm install ${driver}`);
    process.exit(0);
}

// ------------------------------------------------------------------
// 2. Inject into the module cache so all model files receive our
//    dialect-specific instance instead of the config-driven singleton.
// ------------------------------------------------------------------
const seqCachePath = require.resolve('../src/db/sequelize');
require.cache[seqCachePath] = {
    id: seqCachePath, filename: seqCachePath, loaded: true, exports: seq,
};

// ------------------------------------------------------------------
// 3. Load all model files — each calls sequelize.define() and
//    registers itself with our seq instance.
// ------------------------------------------------------------------
const modelsDir = path.join(__dirname, '../src/models');
for (const file of fs.readdirSync(modelsDir).sort().filter(f => f.endsWith('.js'))) {
    try {
        require(path.join(modelsDir, file));
    } catch (e) {
        console.warn(`[generate-ddl] Warning: skipping ${file}: ${e.message}`);
    }
}

const allModels = Object.values(seq.models);
if (!allModels.length) {
    console.error('[generate-ddl] No models loaded — aborting');
    process.exit(1);
}

// ------------------------------------------------------------------
// 4. Topological sort: parents before children (based on FK refs).
// ------------------------------------------------------------------
function topoSort(models) {
    const byTable = Object.fromEntries(models.map(m => [m.tableName, m]));
    const deps = {};
    for (const m of models) {
        deps[m.tableName] = new Set();
        for (const attr of Object.values(m.rawAttributes)) {
            if (!attr.references) continue;
            const ref      = attr.references.model;
            const refTable = ref && typeof ref === 'object' ? ref.tableName : ref;
            if (refTable && refTable !== m.tableName) deps[m.tableName].add(refTable);
        }
    }
    const sorted  = [];
    const visited = new Set();
    function visit(t) {
        if (visited.has(t)) return;
        visited.add(t);
        for (const d of (deps[t] || [])) visit(d);
        sorted.push(t);
    }
    for (const t of Object.keys(deps)) visit(t);
    return sorted.map(t => byTable[t]).filter(Boolean);
}

const ordered = topoSort(allModels);
const qg      = seq.getQueryInterface().queryGenerator;

// ------------------------------------------------------------------
// 5. Build DDL output.
// ------------------------------------------------------------------
const lines = [
    '-- Generated by scripts/generate-ddl.js — do not edit manually.',
    `-- Dialect: ${dialect}`,
    `-- Re-generate with: make generate-ddl  (or: node scripts/generate-ddl.js ${dialect})`,
];
if (dialect === 'postgres') {
    lines.push('-- Note: PostgreSQL ENUM types (enum_*) are created automatically by Sequelize');
    lines.push('--       on first sync(). For manual init, add CREATE TYPE statements above.');
}
lines.push('');

// DROP statements in reverse create order
[...ordered].reverse().forEach(model => {
    if (dialect === 'postgres') {
        lines.push(`DROP TABLE IF EXISTS "${model.tableName}" CASCADE;`);
    } else if (dialect === 'mssql') {
        lines.push(`IF OBJECT_ID('${model.tableName}', 'U') IS NOT NULL DROP TABLE [${model.tableName}];`);
    } else if (dialect === 'sqlite') {
        lines.push(`DROP TABLE IF EXISTS "${model.tableName}";`);
    } else {
        // mysql / mariadb
        lines.push(`DROP TABLE IF EXISTS \`${model.tableName}\`;`);
    }
});
lines.push('');

// CREATE TABLE + explicit indexes
for (const model of ordered) {
    const tableOpts = (dialect === 'mysql' || dialect === 'mariadb') ? { charset: 'utf8mb4' } : {};

    // createTableQuery expects column definition strings, not DataType objects.
    // attributeToSQL converts each column to its dialect-specific SQL fragment.
    const colDefs = {};
    for (const [name, attr] of Object.entries(model.rawAttributes)) {
        try {
            colDefs[name] = qg.attributeToSQL(attr, { key: name, context: 'createTable' });
        } catch (e) {
            // Fallback: best-effort type string
            colDefs[name] = attr.type ? String(attr.type) : 'TEXT';
        }
    }

    let createSql;
    try {
        createSql = qg.createTableQuery(model.tableName, colDefs, tableOpts);
    } catch (e) {
        lines.push(`-- ERROR generating ${model.tableName}: ${e.message}`);
        lines.push('');
        continue;
    }

    lines.push(`-- ${model.tableName}`);
    lines.push(createSql.trim());
    lines.push('');

    for (const idx of (model.options.indexes || [])) {
        try {
            // QueryGenerator.addIndexQuery(tableName, options, rawTablename)
            const idxSql = qg.addIndexQuery(
                model.tableName,
                { fields: idx.fields, unique: !!idx.unique, name: idx.name },
                model.tableName,
            );
            if (idxSql) {
                const stmt = idxSql.trim();
                lines.push(stmt.endsWith(';') ? stmt : stmt + ';');
            }
        } catch (_) {
            // addIndexQuery API varies across dialects; skip silently
        }
    }
    lines.push('');
}

// ------------------------------------------------------------------
// 6. Write output file.
// ------------------------------------------------------------------
const outDir  = path.join(__dirname, '../database');
const outFile = path.join(outDir, `schema.${dialect}.sql`);
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(outFile, lines.join('\n'));
console.log(`[generate-ddl] Written: database/schema.${dialect}.sql`);
