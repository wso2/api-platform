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
 * Creating, saving and deleting a subscription plan answer only once the change is committed, so a
 * caller's next request sees it: an API created right after its plans offers them, instead of being
 * refused because a plan is not found yet.
 *
 * Same setup as apiMetadataService.planDelete.handlers.test.js: a temporary SQLite file built from
 * the shipped schema, driven through one child process. The probe counts the transactions open
 * when each handler answers.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const PLAN_SERVICE = path.join(__dirname, 'apiMetadataService.js');
const DB_DRIVER = path.join(__dirname, '..', 'db', 'driver.js');
const SCHEMA = path.join(PROJECT_ROOT, 'database', 'schema.sqlite.sql');
const SQLITE = path.join(PROJECT_ROOT, 'node_modules', 'better-sqlite3');

const ORG = 'org-uuid-1';
const PORTAL = 'test-portal';

let tmpDir;
test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

const CONFIG = (dbFile) => `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "${PORTAL}"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(dbFile)}
`;

const PROBE = `
const portalDb = require(${JSON.stringify(DB_DRIVER)});
let open = 0;
const withTransaction = portalDb.withTransaction;
portalDb.withTransaction = (fn) => {
    open += 1;
    return Promise.resolve(withTransaction(fn)).finally(() => { open -= 1; });
};
const plans = require(${JSON.stringify(PLAN_SERVICE)});
const out = {};

function call(fn, req) {
    return new Promise((resolve) => {
        const answer = (status, body) => resolve({ status, body, openWhenAnswered: open });
        const res = {
            statusCode: 200,
            setHeader() {},
            status(c) { this.statusCode = c; return this; },
            json(b) { answer(this.statusCode, b); return this; },
            send(b) { answer(this.statusCode, b === undefined ? null : b); return this; },
        };
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.message) }));
    });
}

const as = (extra) => Object.assign(
    { orgId: ${JSON.stringify(ORG)}, auth: { userId: 'admin-1' }, baseUrl: '/api/v0.9', query: {}, params: {}, body: {} },
    extra || {}
);

(async () => {
    out.created = await call(plans.addSubscriptionPlans, as({ body: { id: 'gold', displayName: 'Gold' } }));
    out.updated = await call(plans.putSubscriptionPlans, as({ body: { id: 'gold', displayName: 'Gold plan' } }));
    out.createdByPut = await call(plans.putSubscriptionPlans, as({ body: { id: 'silver', displayName: 'Silver' } }));
    out.deleted = await call(plans.deleteSubscriptionPlan, as({ params: { planId: 'silver' } }));
    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

let _results;
function results() {
    if (_results) return _results;
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-plan-commit-'));
    const dbFile = path.join(tmpDir, 'portal.db');

    const seed = path.join(tmpDir, 'seed.js');
    fs.writeFileSync(seed, `
        const fs = require('fs');
        const Database = require(${JSON.stringify(SQLITE)});
        const db = new Database(${JSON.stringify(dbFile)});
        db.exec(fs.readFileSync(${JSON.stringify(SCHEMA)}, 'utf8'));
        db.prepare(\`INSERT INTO organizations
            (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by)
            VALUES (?,?,?,?,?,?,?,?)\`)
            .run(${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'Default', 'default', 'idp-1', '{}', 'test', 'test');
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);

    const cfgFile = path.join(tmpDir, 'config.toml');
    fs.writeFileSync(cfgFile, CONFIG(dbFile));
    const runner = path.join(tmpDir, 'probe.js');
    fs.writeFileSync(runner, PROBE);

    const run = spawnSync(process.execPath, [runner, '--config', cfgFile], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(run.status, 0, `probe failed: ${run.stderr}`);
    const line = run.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${run.stdout}\n${run.stderr}`);
    _results = JSON.parse(line.slice('PROBE_JSON:'.length));
    return _results;
}

test('creating a plan answers 201 once its transaction is committed', () => {
    const r = results().created;
    assert.equal(r.status, 201);
    assert.equal(r.body.id, 'gold');
    assert.equal(r.openWhenAnswered, 0);
});

test('saving a plan answers 200 once its transaction is committed', () => {
    const r = results().updated;
    assert.equal(r.status, 200);
    assert.equal(r.body.displayName, 'Gold plan');
    assert.equal(r.openWhenAnswered, 0);
});

test('a PUT that creates a plan answers 201 once its transaction is committed', () => {
    const r = results().createdByPut;
    assert.equal(r.status, 201);
    assert.equal(r.body.id, 'silver');
    assert.equal(r.openWhenAnswered, 0);
});

test('deleting a plan answers 204 once its transaction is committed', () => {
    const r = results().deleted;
    assert.equal(r.status, 204);
    assert.equal(r.openWhenAnswered, 0);
});
