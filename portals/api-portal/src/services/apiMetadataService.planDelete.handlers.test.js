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
 * Deleting a subscription plan, against a real database: a plan that subscriptions use is kept and
 * the request is refused with 409 PLAN_IN_USE, leaving the plan, its subscriptions and the events
 * for the Platform API as they were; a plan with no subscriptions is deleted and one
 * subscription_plan.deleted event is recorded for the Platform API.
 *
 * Same setup as billing/billing.handlers.test.js: a temporary SQLite file built from the shipped
 * schema, driven through one child process so later calls see what earlier ones did.
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
const plans = require(${JSON.stringify(PLAN_SERVICE)});
const portalDb = require(${JSON.stringify(DB_DRIVER)});
const Database = require(${JSON.stringify(SQLITE)});
const out = {};

function call(fn, req) {
    return new Promise((resolve) => {
        const res = {
            statusCode: 200,
            setHeader() {},
            status(c) { this.statusCode = c; return this; },
            json(b) { resolve({ status: this.statusCode, body: b }); return this; },
            send(b) { resolve({ status: this.statusCode, body: b === undefined ? null : b }); return this; },
        };
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.message) }));
    });
}

const as = (extra) => Object.assign(
    { orgId: ${JSON.stringify(ORG)}, auth: { userId: 'admin-1' }, baseUrl: '/api/v0.9', query: {}, params: {}, body: {} },
    extra || {}
);

(async () => {
    const db = new Database(${JSON.stringify('DB_FILE')});
    const count = (q, ...p) => db.prepare(q).get(...p).n;
    const deletedEvents = () => count("SELECT COUNT(*) AS n FROM events WHERE type = 'subscription_plan.deleted'");
    const deletePlan = (planId) => call(plans.deleteSubscriptionPlan, as({ params: { planId } }));
    // The delete answers before its transaction commits; this next call waits for it.
    const settle = (planId) => call(plans.getSubscriptionPlan, as({ params: { planId } }));

    const eventsBefore = count('SELECT COUNT(*) AS n FROM events');

    // --- a plan with subscriptions is kept ----------------------------------
    out.goldRefused = await deletePlan('gold');
    out.silverRefused = await deletePlan('silver');
    out.plansAfterRefusal = count("SELECT COUNT(*) AS n FROM subscription_plans WHERE handle IN ('gold', 'silver')");
    out.goldSubsAfterRefusal = db.prepare("SELECT uuid, plan_uuid, status FROM subscriptions WHERE uuid LIKE 'sub-g%' ORDER BY uuid").all();
    out.eventsAfterRefusal = count('SELECT COUNT(*) AS n FROM events') - eventsBefore;

    // --- a plan with no subscriptions is deleted ----------------------------
    out.freeDeleted = await deletePlan('free');
    out.freeAfterDelete = await settle('free');
    out.freeRows = count("SELECT COUNT(*) AS n FROM subscription_plans WHERE handle = 'free'");
    out.deletedEventsAfterFree = deletedEvents();
    out.lastEvent = db.prepare('SELECT type, aggregate_type, aggregate_uuid, payload FROM events ORDER BY rowid DESC LIMIT 1').get();
    out.missing = await deletePlan('no-such-plan');

    // --- once its subscriptions have ended, the plan can be deleted ---------
    // Written through the portal's own connection, which holds the SQLite write lock.
    await portalDb.execute("DELETE FROM subscriptions WHERE uuid LIKE 'sub-g%'");
    out.goldDeleted = await deletePlan('gold');
    out.goldAfterDelete = await settle('gold');
    out.goldRows = count("SELECT COUNT(*) AS n FROM subscription_plans WHERE handle = 'gold'");
    out.deletedEventsAfterGold = deletedEvents();
    out.silverSubAfterAll = db.prepare("SELECT plan_uuid FROM subscriptions WHERE uuid = 'sub-s1'").get().plan_uuid;

    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

let _results;
function results() {
    if (_results) return _results;
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-plan-delete-'));
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
        const ORG = ${JSON.stringify(ORG)}, P = ${JSON.stringify(PORTAL)};
        db.prepare('INSERT INTO api_metadata (uuid, org_uuid, portal_id, name, status, version, type, handle, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?,?,?)')
            .run('api-1', ORG, P, 'Weather', 'PUBLISHED', 'v1', 'REST', 'weather-v1', 'test', 'test');
        for (const [uuid, handle, name] of [['plan-gold', 'gold', 'Gold'], ['plan-silver', 'silver', 'Silver'], ['plan-free', 'free', 'Free']]) {
            db.prepare('INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?)')
                .run(uuid, ORG, P, handle, name, handle, 'test', 'test');
        }
        const subscribe = (uuid, user, plan, status) => db.prepare(\`INSERT INTO subscriptions
            (uuid, created_by, updated_by, api_uuid, plan_uuid, org_uuid, portal_id, status) VALUES (?,?,?,?,?,?,?,?)\`)
            .run(uuid, user, user, 'api-1', plan, ORG, P, status);
        // A suspended subscription has not ended, so it counts too.
        subscribe('sub-g1', 'user-1', 'plan-gold', 'ACTIVE');
        subscribe('sub-g2', 'user-2', 'plan-gold', 'ACTIVE');
        subscribe('sub-g3', 'user-3', 'plan-gold', 'INACTIVE');
        subscribe('sub-s1', 'user-1', 'plan-silver', 'ACTIVE');
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);

    const cfgFile = path.join(tmpDir, 'config.toml');
    fs.writeFileSync(cfgFile, CONFIG(dbFile));
    const runner = path.join(tmpDir, 'probe.js');
    fs.writeFileSync(runner, PROBE.replaceAll(JSON.stringify('DB_FILE'), JSON.stringify(dbFile)));

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

test('deleting a plan with subscriptions is refused with 409 PLAN_IN_USE, naming the plan', () => {
    const r = results().goldRefused;
    assert.equal(r.status, 409);
    assert.equal(r.body.code, 'PLAN_IN_USE');
    assert.equal(r.body.message, 'The plan Gold is in use and can\'t be deleted.');
});

test('a plan with one subscription gets the same refusal', () => {
    const r = results().silverRefused;
    assert.equal(r.status, 409);
    assert.equal(r.body.code, 'PLAN_IN_USE');
    assert.equal(r.body.message, 'The plan Silver is in use and can\'t be deleted.');
});

test('a refused delete keeps the plan and its subscriptions, and publishes no event', () => {
    const r = results();
    assert.equal(r.plansAfterRefusal, 2);
    assert.deepEqual(r.goldSubsAfterRefusal, [
        { uuid: 'sub-g1', plan_uuid: 'plan-gold', status: 'ACTIVE' },
        { uuid: 'sub-g2', plan_uuid: 'plan-gold', status: 'ACTIVE' },
        { uuid: 'sub-g3', plan_uuid: 'plan-gold', status: 'INACTIVE' },
    ]);
    assert.equal(r.eventsAfterRefusal, 0);
});

test('a plan with no subscriptions is deleted and one subscription_plan.deleted event is published', () => {
    const r = results();
    assert.equal(r.freeDeleted.status, 204);
    assert.equal(r.freeAfterDelete.status, 404);
    assert.equal(r.freeRows, 0);
    assert.equal(r.deletedEventsAfterFree, 1);
    assert.equal(r.lastEvent.type, 'subscription_plan.deleted');
    assert.equal(r.lastEvent.aggregate_type, 'subscription_plan');
    assert.equal(r.lastEvent.aggregate_uuid, 'plan-free');
    assert.deepEqual(JSON.parse(r.lastEvent.payload), { handle: 'free' });
});

test('deleting a plan that does not exist answers 404', () => {
    assert.equal(results().missing.status, 404);
});

test('a plan can be deleted once its subscriptions have ended, leaving other plans untouched', () => {
    const r = results();
    assert.equal(r.goldDeleted.status, 204);
    assert.equal(r.goldAfterDelete.status, 404);
    assert.equal(r.goldRows, 0);
    assert.equal(r.deletedEventsAfterGold, 2);
    assert.equal(r.silverSubAfterAll, 'plan-silver');
});
