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
 * Subscription plan conditions through the plan handlers, against a real SQLite database built
 * from the shipped schema (same setup as apiMetadataService.planDelete.handlers.test.js). Two
 * databases from an earlier release are covered as well: one upgraded, which still has
 * subscription_plan_limits, and one whose schema has not been upgraded yet.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const PLAN_SERVICE = path.join(__dirname, 'apiMetadataService.js');
const SCHEMA = path.join(PROJECT_ROOT, 'database', 'schema.sqlite.sql');

const ORG = 'org-uuid-1';
const PORTAL = 'test-portal';

// subscription_plan_limits as earlier releases created it.
const LEGACY_LIMITS_TABLE = `CREATE TABLE subscription_plan_limits (
    uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
    time_unit VARCHAR(20),
    time_amount INTEGER NOT NULL DEFAULT 1,
    limit_count BIGINT NOT NULL,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE
)`;

const tmpDirs = [];
test.after(() => {
    for (const dir of tmpDirs) fs.rmSync(dir, { recursive: true, force: true });
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

const PROBE_HEAD = `
const plans = require(${JSON.stringify(PLAN_SERVICE)});
const Database = require('node:sqlite').DatabaseSync;
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
    { orgId: ${JSON.stringify(ORG)}, auth: { userId: 'admin-1' }, baseUrl: '/api/v0.9', query: {}, params: {}, body: {}, headers: {} },
    extra || {}
);
const create = (body) => call(plans.addSubscriptionPlans, as({ body }));
const put = (body) => call(plans.putSubscriptionPlans, as({ body }));
const get = (planId) => call(plans.getSubscriptionPlan, as({ params: { planId } }));
const yamlPlan = (fn, text) => call(fn, as({ files: { subscriptionPlan: [{ buffer: Buffer.from(text) }] } }));
const window_ = (count, timeAmount, timeUnit) => ({ count, timeAmount, timeUnit });
const db = new Database(DB_FILE);
const conditionRows = (handle) => db.prepare(
    'SELECT c.condition_id FROM subscription_plan_conditions c JOIN subscription_plans p ON p.uuid = c.plan_uuid WHERE p.handle = ? ORDER BY c.sort_order'
).all(handle).map((r) => r.condition_id);
const legacyRows = (handle) => db.prepare(
    'SELECT l.limit_type, l.limit_count FROM subscription_plan_limits l JOIN subscription_plans p ON p.uuid = l.plan_uuid WHERE p.handle = ? ORDER BY l.limit_count'
).all(handle).map((r) => [r.limit_type, Number(r.limit_count)]);
const planCount = (handle) => db.prepare('SELECT COUNT(*) AS n FROM subscription_plans WHERE handle = ?').get(handle).n;
const lastEvent = () => JSON.parse(db.prepare('SELECT payload FROM events ORDER BY rowid DESC LIMIT 1').get().payload);
`;

// A database upgraded from an earlier release: it has subscription_plan_conditions and still has
// subscription_plan_limits, with a plan saved before conditions.
const UPGRADED_PROBE = `
(async () => {
    out.created = await create({
        id: 'gold', displayName: 'Gold',
        conditions: [
            { type: 'request-count', params: { limits: [window_(100, 1, 'minute'), window_(1000, 0, 'HOUR')], onExceed: 'allow' } },
            { type: 'tool-call-count', params: { limits: [window_(50, 1, 'MINUTE')] } },
            { type: 'token-count', params: { limits: [window_(10000, 1, 'DAY')], tokenType: 'prompt' } },
            { id: 'region', type: 'custom', params: { zone: 'eu', a: 1 } },
        ],
        limits: [{ limitType: 'EVENT_COUNT', limitCount: 5, timeUnit: 'MINUTE' }, { limitType: 'REQUEST_COUNT', limitCount: 7, timeUnit: 'DAY' }],
    });
    out.createdEvent = lastEvent();
    out.goldRows = conditionRows('gold');

    out.legacy = await get('legacy');
    out.async = await get('async');
    out.events = await create({ id: 'events', displayName: 'Events', limits: [{ limitType: 'EVENT_COUNT', limitCount: 30, timeUnit: 'MINUTE' }] });
    out.eventsConditions = await put({ id: 'events', displayName: 'Events', conditions: [
        { type: 'event-count', params: { limits: [window_(60, 1, 'MINUTE')], onExceed: 'allow' } },
        { id: 'region', type: 'custom', params: { zone: 'eu' } },
    ] });
    out.eventsLimitsOnly = await put({ id: 'events', displayName: 'Events', limits: [
        { limitType: 'EVENT_COUNT', limitCount: 20, timeUnit: 'HOUR' },
        { limitType: 'REQUEST_COUNT', limitCount: 10, timeUnit: 'MINUTE' },
    ] });
    out.eventsLimitsOnlyEvent = lastEvent();
    out.eventsRequestsOnly = await put({ id: 'events', displayName: 'Events', limits: [{ limitType: 'REQUEST_COUNT', limitCount: 5, timeUnit: 'MINUTE' }] });
    out.legacyRowsBefore = legacyRows('legacy');
    out.legacyRenamed = await put({ id: 'legacy', displayName: 'Legacy 2' });
    out.legacyRowsAfterRename = legacyRows('legacy');
    out.legacySaved = await put({ id: 'legacy', displayName: 'Legacy 2', limits: [{ limitType: 'REQUEST_COUNT', limitCount: 40, timeUnit: 'HOUR' }] });
    out.legacyConditionRows = conditionRows('legacy');
    out.legacyRowsAfterSave = legacyRows('legacy');

    out.silver = await create({ id: 'silver', displayName: 'Silver', limits: [
        { limitType: 'REQUEST_COUNT', limitCount: 500, timeUnit: 'MINUTE' },
        { limitType: 'REQUEST_COUNT', limitCount: -1 },
    ] });
    out.silverRows = conditionRows('silver');
    out.silverEvent = lastEvent();

    out.conditionsOnly = await put({ id: 'gold', displayName: 'Gold', conditions: [
        { type: 'request-count', params: { limits: [window_(300, 1, 'MINUTE')], onExceed: 'allow' } },
        { type: 'tool-call-count', params: { limits: [window_(50, 1, 'MINUTE')] } },
    ] });
    out.limitsOnly = await put({ id: 'gold', displayName: 'Gold', limits: [{ limitType: 'REQUEST_COUNT', limitCount: 200, timeUnit: 'MINUTE' }] });
    out.limitsOnlyRemoved = await put({ id: 'gold', displayName: 'Gold', limits: [] });

    out.silverToolCalls = await put({ id: 'silver', displayName: 'Silver', conditions: [
        { type: 'tool-call-count', params: { limits: [window_(10, 1, 'MINUTE')] } },
    ] });

    out.duplicate = await create({ id: 'bad', displayName: 'Bad', conditions: [
        { type: 'request-count', params: { limits: [window_(1, 1, 'MINUTE')] } },
        { type: 'request-count', params: { limits: [window_(2, 1, 'HOUR')] } },
    ] });
    out.badStored = planCount('bad');
    out.badUpdate = await put({ id: 'silver', displayName: 'Silver', conditions: [
        { type: 'request-count', params: { limits: [window_(60, 1, 'MINUTE'), window_(70, 60, 'MINUTE'), window_(80, 1, 'HOUR')] } },
    ] });
    out.afterBadUpdate = await get('silver');

    out.cleared = await put({ id: 'silver', displayName: 'Silver', conditions: [] });
    out.clearedRows = conditionRows('silver');

    // Old-style limits that would otherwise drop the request quota are refused.
    out.fractional = await put({ id: 'silver', displayName: 'Silver', limits: [{ limitType: 'REQUEST_COUNT', limitCount: 1.5, timeUnit: 'MINUTE' }] });
    out.unitless = await create({ id: 'unitless', displayName: 'Unitless', limits: [{ limitType: 'REQUEST_COUNT', limitCount: 100 }] });
    // YAML: the requestCount shorthand is a per-minute limit; an update that gives no limits keeps them.
    out.yamlShorthand = await yamlPlan(plans.addSubscriptionPlans,
        'kind: SubscriptionPlan\\nmetadata:\\n  name: yaml-plan\\nspec:\\n  displayName: YAML plan\\n  type: requestcount\\n  requestCount: 10000\\n');
    out.yamlRenamed = await yamlPlan(plans.putSubscriptionPlans,
        'kind: SubscriptionPlan\\nmetadata:\\n  name: yaml-plan\\nspec:\\n  displayName: YAML plan 2\\n');
    // The eventCount shorthand is a per-minute event-count condition; -1 is unlimited, so no condition.
    out.yamlEvents = await yamlPlan(plans.addSubscriptionPlans,
        'kind: SubscriptionPlan\\nmetadata:\\n  name: yaml-events\\nspec:\\n  displayName: YAML events\\n  type: eventcount\\n  eventCount: 2000\\n');
    out.yamlEventsUnlimited = await yamlPlan(plans.addSubscriptionPlans,
        'kind: SubscriptionPlan\\nmetadata:\\n  name: yaml-events-unlimited\\nspec:\\n  displayName: YAML events unlimited\\n  type: eventcount\\n  eventCount: -1\\n');
    // A limits-only update can't take a plan past 20 conditions.
    out.full = await create({ id: 'full', displayName: 'Full', conditions: Array.from({ length: 20 }, (_, i) => ({ id: 'c' + i, type: 'custom', params: {} })) });
    out.fullPlusLimit = await put({ id: 'full', displayName: 'Full', limits: [{ limitType: 'REQUEST_COUNT', limitCount: 5, timeUnit: 'MINUTE' }] });

    await call(plans.deleteSubscriptionPlan, as({ params: { planId: 'gold' } }));
    out.orphanRows = db.prepare('SELECT COUNT(*) AS n FROM subscription_plan_conditions WHERE plan_uuid NOT IN (SELECT uuid FROM subscription_plans)').get().n;

    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

// A PostgreSQL or SQL Server database from an earlier release before its schema script is re-run:
// no subscription_plan_conditions yet.
const NOT_UPGRADED_PROBE = `
(async () => {
    out.basic = await create({ id: 'basic', displayName: 'Basic', limits: [{ limitType: 'REQUEST_COUNT', limitCount: 60, timeUnit: 'MINUTE' }] });
    out.basicRows = legacyRows('basic');
    out.basicUpdated = await put({ id: 'basic', displayName: 'Basic', conditions: [
        { type: 'request-count', params: { limits: [window_(90, 1, 'MINUTE'), window_(900, 1, 'HOUR')] } },
    ] });
    out.basicRowsAfter = legacyRows('basic');
    out.tools = await create({ id: 'tools', displayName: 'Tools', conditions: [
        { type: 'tool-call-count', params: { limits: [window_(5, 1, 'MINUTE')] } },
    ] });
    out.toolsStored = planCount('tools');
    // The legacy table has no onExceed, so allow can't be kept there.
    out.allow = await create({ id: 'allow', displayName: 'Allow', conditions: [
        { type: 'request-count', params: { limits: [window_(5, 1, 'MINUTE')], onExceed: 'allow' } },
    ] });
    out.allowStored = planCount('allow');
    // Event limits are kept there too, as EVENT_COUNT rows.
    out.async = await create({ id: 'async', displayName: 'Async', conditions: [
        { type: 'event-count', params: { limits: [window_(30, 1, 'MINUTE')] } },
        { type: 'request-count', params: { limits: [window_(60, 1, 'MINUTE')] } },
    ] });
    out.asyncRows = legacyRows('async');
    out.asyncRead = await get('async');

    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

function runProbe(name, seedSql, probe) {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), `ap-plan-conditions-${name}-`));
    tmpDirs.push(tmpDir);
    const dbFile = path.join(tmpDir, 'portal.db');

    const seed = path.join(tmpDir, 'seed.js');
    fs.writeFileSync(seed, `
        const fs = require('fs');
        const Database = require('node:sqlite').DatabaseSync;
        const db = new Database(${JSON.stringify(dbFile)});
        db.exec(fs.readFileSync(${JSON.stringify(SCHEMA)}, 'utf8'));
        db.prepare(\`INSERT INTO organizations
            (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by)
            VALUES (?,?,?,?,?,?,?,?)\`)
            .run(${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'Default', 'default', 'idp-1', '{}', 'test', 'test');
        db.exec(${JSON.stringify(seedSql)});
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);

    const cfgFile = path.join(tmpDir, 'config.toml');
    fs.writeFileSync(cfgFile, CONFIG(dbFile));
    const runner = path.join(tmpDir, 'probe.js');
    fs.writeFileSync(runner, PROBE_HEAD.replace('DB_FILE', JSON.stringify(dbFile)) + probe);

    const run = spawnSync(process.execPath, [runner, '--config', cfgFile], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(run.status, 0, `probe failed: ${run.stderr}`);
    const line = run.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${run.stdout}\n${run.stderr}`);
    return JSON.parse(line.slice('PROBE_JSON:'.length));
}

const P = PORTAL;
let upgradedResults;
const upgraded = () => upgradedResults || (upgradedResults = runProbe('upgraded', `
    ${LEGACY_LIMITS_TABLE};
    INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by)
        VALUES ('plan-legacy', '${ORG}', '${P}', 'legacy', 'Legacy', 'legacy', 'test', 'test');
    INSERT INTO subscription_plan_limits (uuid, plan_uuid, portal_id, limit_type, time_unit, time_amount, limit_count) VALUES
        ('l1', 'plan-legacy', '${P}', 'REQUEST_COUNT', 'DAY', 1, 5000),
        ('l2', 'plan-legacy', '${P}', 'REQUEST_COUNT', 'MINUTE', 1, 100),
        ('l3', 'plan-legacy', '${P}', 'REQUEST_COUNT', NULL, 1, -1),
        ('l4', 'plan-legacy', '${P}', 'EVENT_COUNT', 'MINUTE', 1, 9),
        ('l5', 'plan-legacy', '${P}', 'EVENT_COUNT', 'HOUR', 1, 900),
        ('l6', 'plan-legacy', '${P}', 'BANDWIDTH', 'DAY', 1, 1);
    INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by)
        VALUES ('plan-async', '${ORG}', '${P}', 'async', 'Async', 'async', 'test', 'test');
    INSERT INTO subscription_plan_limits (uuid, plan_uuid, portal_id, limit_type, time_unit, time_amount, limit_count) VALUES
        ('a1', 'plan-async', '${P}', 'EVENT_COUNT', 'MINUTE', 1, 50);
`, UPGRADED_PROBE));

let notUpgradedResults;
const notUpgraded = () => notUpgradedResults || (notUpgradedResults = runProbe('not-upgraded', `
    DROP TABLE subscription_plan_conditions;
    ${LEGACY_LIMITS_TABLE};
`, NOT_UPGRADED_PROBE));

const requests = (limits, onExceed = 'reject') => ({ id: 'requests', type: 'request-count', params: { limits, onExceed } });
const events = (limits, onExceed = 'reject') => ({ id: 'events', type: 'event-count', params: { limits, onExceed } });
const toolCalls = (limits) => ({ id: 'tool-calls', type: 'tool-call-count', params: { limits, onExceed: 'reject' } });
const w = (count, timeAmount, timeUnit) => ({ count, timeAmount, timeUnit });
const requestLimit = (limitCount, timeUnit, timeAmount = 1) => ({ limitType: 'REQUEST_COUNT', timeUnit, timeAmount, limitCount });
const eventLimit = (limitCount, timeUnit, timeAmount = 1) => ({ limitType: 'EVENT_COUNT', timeUnit, timeAmount, limitCount });

test('a plan created with conditions returns them normalized; its limits are the request-count windows', () => {
    const r = upgraded().created;
    assert.equal(r.status, 201);
    assert.deepEqual(r.body.conditions, [
        requests([w(100, 1, 'MINUTE'), w(1000, 1, 'HOUR')], 'allow'),
        toolCalls([w(50, 1, 'MINUTE')]),
        { id: 'tokens', type: 'token-count', params: { limits: [w(10000, 1, 'DAY')], onExceed: 'reject', tokenType: 'prompt' } },
        { id: 'region', type: 'custom', params: { a: 1, zone: 'eu' } },
    ]);
    assert.deepEqual(r.body.limits, [requestLimit(100, 'MINUTE'), requestLimit(1000, 'HOUR')]);
    assert.deepEqual(upgraded().goldRows, ['requests', 'tool-calls', 'tokens', 'region']);
});

test('the plan events carry the conditions, and limits for subscribers that predate them', () => {
    const e = upgraded().createdEvent;
    assert.equal(e.handle, 'gold');
    assert.deepEqual(e.conditions.map((c) => c.id), ['requests', 'tool-calls', 'tokens', 'region']);
    assert.deepEqual(e.limits, [
        { limit_type: 'REQUEST_COUNT', limit_count: 100, time_unit: 'MINUTE', time_amount: 1 },
        { limit_type: 'REQUEST_COUNT', limit_count: 1000, time_unit: 'HOUR', time_amount: 1 },
    ]);
});

test('a plan saved before conditions reads its request and event limits as request-count and event-count conditions, shortest first', () => {
    const r = upgraded();
    assert.equal(r.legacy.status, 200);
    // The unlimited limit and the bandwidth limit have no condition.
    assert.deepEqual(r.legacy.body.conditions, [
        requests([w(100, 1, 'MINUTE'), w(5000, 1, 'DAY')]),
        events([w(9, 1, 'MINUTE'), w(900, 1, 'HOUR')]),
    ]);
    assert.deepEqual(r.legacy.body.limits, [
        requestLimit(100, 'MINUTE'), requestLimit(5000, 'DAY'), eventLimit(9, 'MINUTE'), eventLimit(900, 'HOUR'),
    ]);
    assert.equal(r.legacyRowsBefore.length, 6);
    assert.deepEqual(r.legacyRenamed.body.conditions, r.legacy.body.conditions);
    assert.equal(r.legacyRowsAfterRename.length, 6);
    assert.deepEqual(r.async.body.conditions, [events([w(50, 1, 'MINUTE')])]);
    assert.deepEqual(r.async.body.limits, [eventLimit(50, 'MINUTE')]);
});

test('event limits sent alone become the event-count condition', () => {
    const r = upgraded().events;
    assert.equal(r.status, 201);
    assert.deepEqual(r.body.conditions, [events([w(30, 1, 'MINUTE')])]);
    assert.deepEqual(r.body.limits, [eventLimit(30, 'MINUTE')]);
});

test('limits sent alone set both the request-count and event-count conditions, and keep the others', () => {
    const r = upgraded();
    assert.equal(r.eventsConditions.status, 200);
    assert.deepEqual(r.eventsLimitsOnly.body.conditions, [
        requests([w(10, 1, 'MINUTE')]),
        events([w(20, 1, 'HOUR')], 'allow'),
        { id: 'region', type: 'custom', params: { zone: 'eu' } },
    ]);
    assert.deepEqual(r.eventsLimitsOnly.body.limits, [requestLimit(10, 'MINUTE'), eventLimit(20, 'HOUR')]);
    // The plan events carry the event-count condition, and its windows as EVENT_COUNT limits.
    assert.deepEqual(r.eventsLimitsOnlyEvent.conditions.map((c) => c.type), ['request-count', 'event-count', 'custom']);
    assert.deepEqual(r.eventsLimitsOnlyEvent.limits, [
        { limit_type: 'REQUEST_COUNT', limit_count: 10, time_unit: 'MINUTE', time_amount: 1 },
        { limit_type: 'EVENT_COUNT', limit_count: 20, time_unit: 'HOUR', time_amount: 1 },
    ]);
    // Request limits alone remove the event-count condition.
    assert.deepEqual(r.eventsRequestsOnly.body.conditions, [requests([w(5, 1, 'MINUTE')]), { id: 'region', type: 'custom', params: { zone: 'eu' } }]);
});

test('saving the limits of a plan saved before conditions moves it to conditions', () => {
    const r = upgraded();
    assert.deepEqual(r.legacySaved.body.conditions, [requests([w(40, 1, 'HOUR')])]);
    assert.deepEqual(r.legacyConditionRows, ['requests']);
    assert.deepEqual(r.legacyRowsAfterSave, []);
});

test('limits sent alone become the request-count condition; unlimited limits are left out', () => {
    const r = upgraded();
    assert.equal(r.silver.status, 201);
    assert.deepEqual(r.silver.body.conditions, [requests([w(500, 1, 'MINUTE')])]);
    assert.deepEqual(r.silverRows, ['requests']);
    assert.deepEqual(r.silverEvent.conditions, [requests([w(500, 1, 'MINUTE')])]);
});

test('limits sent alone replace the request-count condition and keep its onExceed', () => {
    const r = upgraded();
    assert.deepEqual(r.conditionsOnly.body.limits, [requestLimit(300, 'MINUTE')]);
    assert.deepEqual(r.limitsOnly.body.conditions, [requests([w(200, 1, 'MINUTE')], 'allow'), toolCalls([w(50, 1, 'MINUTE')])]);
    assert.deepEqual(r.limitsOnly.body.limits, [requestLimit(200, 'MINUTE')]);
    assert.deepEqual(r.limitsOnlyRemoved.body.conditions, [toolCalls([w(50, 1, 'MINUTE')])]);
    assert.deepEqual(r.limitsOnlyRemoved.body.limits, []);
    assert.deepEqual(upgraded().silverToolCalls.body.conditions, [toolCalls([w(10, 1, 'MINUTE')])]);
});

test('invalid conditions are refused with 400 and nothing is saved', () => {
    const r = upgraded();
    assert.equal(r.duplicate.status, 400);
    assert.match(JSON.stringify(r.duplicate.body), /conditions\[1\]: a plan can have only one request-count condition/);
    assert.equal(r.badStored, 0);
    assert.equal(r.badUpdate.status, 400);
    assert.match(JSON.stringify(r.badUpdate.body), /conditions\[0\]\.params\.limits\[2\]: the condition already has a limit over 1 HOUR/);
    assert.deepEqual(r.afterBadUpdate.body.conditions, [toolCalls([w(10, 1, 'MINUTE')])]);
});

test('an empty conditions list removes them all, and deleting a plan removes its conditions', () => {
    const r = upgraded();
    assert.deepEqual(r.cleared.body.conditions, []);
    assert.deepEqual(r.clearedRows, []);
    assert.equal(r.orphanRows, 0);
});

test('a database without the conditions table keeps request limits in subscription_plan_limits', () => {
    const r = notUpgraded();
    assert.equal(r.basic.status, 201);
    assert.deepEqual(r.basic.body.conditions, [requests([w(60, 1, 'MINUTE')])]);
    assert.deepEqual(r.basicRows, [['REQUEST_COUNT', 60]]);
    assert.equal(r.basicUpdated.status, 200);
    assert.deepEqual(r.basicUpdated.body.conditions, [requests([w(90, 1, 'MINUTE'), w(900, 1, 'HOUR')])]);
    assert.deepEqual(r.basicRowsAfter, [['REQUEST_COUNT', 90], ['REQUEST_COUNT', 900]]);
});

test('a database without the conditions table refuses conditions it can\'t hold, and saves nothing', () => {
    const r = notUpgraded();
    assert.equal(r.tools.status, 503);
    assert.match(JSON.stringify(r.tools.body), /run this release's database schema script/);
    assert.equal(r.toolsStored, 0);
});

test('old-style limits that would drop the request quota are refused with 400', () => {
    const r = upgraded();
    assert.equal(r.fractional.status, 400);
    assert.match(JSON.stringify(r.fractional.body), /positive whole number/);
    assert.equal(r.unitless.status, 400);
    assert.match(JSON.stringify(r.unitless.body), /timeUnit \(MINUTE, HOUR, DAY or MONTH\) is required/);
    assert.equal(r.full.status, 201);
    assert.equal(r.fullPlusLimit.status, 400);
    assert.match(JSON.stringify(r.fullPlusLimit.body), /a plan can have at most 20 conditions/);
});

test('a database without the conditions table refuses onExceed allow, which it can\'t keep', () => {
    const r = notUpgraded();
    assert.equal(r.allow.status, 503);
    assert.equal(r.allowStored, 0);
});

test('a database without the conditions table keeps event limits as EVENT_COUNT rows', () => {
    const r = notUpgraded();
    assert.equal(r.async.status, 201);
    assert.deepEqual(r.asyncRows, [['EVENT_COUNT', 30], ['REQUEST_COUNT', 60]]);
    assert.deepEqual(r.asyncRead.body.conditions, [requests([w(60, 1, 'MINUTE')]), events([w(30, 1, 'MINUTE')])]);
});

test('YAML: requestCount is a per-minute limit, and an update without limits keeps them', () => {
    const r = upgraded();
    assert.equal(r.yamlShorthand.status, 201);
    assert.deepEqual(r.yamlShorthand.body.conditions, [requests([w(10000, 1, 'MINUTE')])]);
    assert.equal(r.yamlRenamed.status, 200);
    assert.equal(r.yamlRenamed.body.displayName, 'YAML plan 2');
    assert.deepEqual(r.yamlRenamed.body.conditions, [requests([w(10000, 1, 'MINUTE')])]);
});

test('YAML: eventCount is a per-minute event-count condition, and -1 is unlimited', () => {
    const r = upgraded();
    assert.equal(r.yamlEvents.status, 201);
    assert.deepEqual(r.yamlEvents.body.conditions, [events([w(2000, 1, 'MINUTE')])]);
    assert.equal(r.yamlEventsUnlimited.status, 201);
    assert.deepEqual(r.yamlEventsUnlimited.body.conditions, []);
});
