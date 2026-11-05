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
 * End-to-end tests for billing with Moesif keys set in config ([api_portal.billing.moesif]
 * collector_application_id and management_key), against a real database, with Stripe and Moesif
 * replaced by in-memory test clients.
 *
 * Three runs share the database and the test Stripe account: the first sets billing up with the keys
 * of one Moesif application; the second starts with another application's keys in config, removes
 * billing and sets it up again; the third starts with a third application's keys and creates a paid
 * plan, which moves billing to that application, then removes billing.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..', '..');
const SERVICE = path.join(__dirname, 'billingConfigurationService.js');
const SCHEMA = path.join(PROJECT_ROOT, 'database', 'schema.sqlite.sql');
const SQLITE = path.join(PROJECT_ROOT, 'node_modules', 'better-sqlite3');

const ORG = '0b5c9a4e-1f2d-4c3b-9a8e-7d6f5e4c3b2a';
const PORTAL = 'test-portal';

const collectorId = (app) => Buffer.from(JSON.stringify({ app, ver: '2.0' })).toString('base64url');
const FIRST_KEYS = { moesifCollectorApplicationId: collectorId('6447:123'), moesifManagementKey: 'moesif-key-first-app-0000' };
const SECOND_KEYS = { moesifCollectorApplicationId: collectorId('6447:999'), moesifManagementKey: 'moesif-key-second-app-0000' };
const THIRD_KEYS = { moesifCollectorApplicationId: collectorId('6447:555'), moesifManagementKey: 'moesif-key-third-app-0000' };
const STRIPE_KEYS = { stripeApiKey: 'rk_test_51Habcdefghijk4f2a', stripePublishableKey: 'pk_test_51Habcdefghij0000' };

let tmpDir;
test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

const CONFIG = (dbFile, keys) => `
[api_portal.server]
base_url = "https://developer.example.com"

[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "${PORTAL}"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(dbFile)}

[api_portal.billing.moesif]
collector_application_id = ${JSON.stringify(keys.moesifCollectorApplicationId)}
management_key = ${JSON.stringify(keys.moesifManagementKey)}
`;

// One run of the portal with the Moesif keys in its config; the test Stripe account and Moesif's
// connection to it are read from and written back to STATE_FILE.
const PROBE = `
const fs = require('fs');
const crypto = require('crypto');

const svc = require(${JSON.stringify(SERVICE)});
const { BillingProviderError } = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/http.js'))});
const stripeConnector = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/connectors/moesif/payments/stripe/index.js'))});
const moesifConnector = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/connectors/moesif/index.js'))});
const configurationDao = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/dao/billingConfigurationDao.js'))});
const Database = require(${JSON.stringify(SQLITE)});
const out = {};

const STATE_FILE = process.env.STATE_FILE;
const state = fs.existsSync(STATE_FILE) ? JSON.parse(fs.readFileSync(STATE_FILE, 'utf8')) : { endpoints: [], partners: {} };
const moesifCalls = [];
let moesifRefuses = false;
let stripeRefusesEndpoints = false;
let stripeRefusesDelete = () => false;

stripeConnector.clients.stripe = () => ({
    async listWebhookEndpoints() { return state.endpoints.map((e) => ({ ...e })); },
    async createWebhookEndpoint({ url, events }) {
        if (stripeRefusesEndpoints) throw new BillingProviderError('Stripe', 400, 'Stripe: refused');
        const e = { id: 'we_' + crypto.randomBytes(4).toString('hex'), url, enabled_events: events, status: 'enabled',
            secret: 'whsec_' + crypto.randomBytes(8).toString('hex') };
        state.endpoints.push(e);
        return { ...e };
    },
    async updateWebhookEndpoint(id) { return { ...state.endpoints.find((x) => x.id === id) }; },
    async deleteWebhookEndpoint(id) {
        if (stripeRefusesDelete(id)) throw new BillingProviderError('Stripe', 400, 'Stripe: refused');
        state.endpoints = state.endpoints.filter((x) => x.id !== id);
    },
    async expireCheckoutSession(id) { return { id, status: 'expired' }; },
});
// Each call is recorded with the key it was made with; each key's application has its own connection to Stripe.
moesifConnector.clients.moesif = (key) => ({
    async listCatalogPlans() {
        if (moesifRefuses) throw new BillingProviderError('Moesif', 401, 'Moesif: Unauthorized');
        return [];
    },
    async createCatalogPlan(provider, body) { moesifCalls.push(['plan', key]); return { id: 'prod_' + moesifCalls.length }; },
    async createCatalogPrice() { moesifCalls.push(['price', key]); return { id: 'price_' + moesifCalls.length }; },
    async createBillingMeter() { moesifCalls.push(['meter', key]); return { id: 'bm_' + moesifCalls.length }; },
    async archiveCatalogPlan() { moesifCalls.push(['archive-plan', key]); },
    async archiveCatalogPrice() { moesifCalls.push(['archive-price', key]); },
    async deleteCatalogPrice() { moesifCalls.push(['delete-price', key]); },
    async deleteBillingMeter() { moesifCalls.push(['delete-meter', key]); },
    async getPartnerAccount() {
        return state.partners[key] ? { partner_account: { credentials: { state: 'enabled' } } } : {};
    },
    async savePartnerAccount() { moesifCalls.push(['connect', key]); state.partners[key] = true; },
    async deletePartnerAccount() { moesifCalls.push(['disconnect', key]); delete state.partners[key]; },
});

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
    { orgId: ${JSON.stringify(ORG)}, auth: { userId: 'admin-1' }, baseUrl: '/api-portal', path: '/api/v0.9/billing-configuration',
      query: {}, params: {}, body: {} }, extra || {});
const stripeKeys = ${JSON.stringify(STRIPE_KEYS)};
const db = new Database(${JSON.stringify('DB_FILE')});

(async () => {
    out.deploymentFields = svc.deploymentFields();
    if (process.env.PHASE === 'setup') {
        // A paid plan already enabled for an API: setting up billing creates it in Moesif.
        await require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/dao/planPricingDao.js'))}).create(${JSON.stringify(ORG)}, 'plan-gold',
            { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '99.00' });
        out.sentMoesifKey = await call(svc.createBillingConfiguration,
            as({ body: { ...stripeKeys, moesifManagementKey: 'moesif-key-entered-0000' } }));
        moesifRefuses = true;
        out.moesifRefused = await call(svc.createBillingConfiguration, as({ body: stripeKeys }));
        moesifRefuses = false;
        out.created = await call(svc.createBillingConfiguration, as({ body: stripeKeys }));
        out.moesifCallsOnCreate = moesifCalls.splice(0);
        out.endpointsAfterCreate = state.endpoints.map((e) => e.url);
        const raw = db.prepare('SELECT * FROM billing_configurations').get();
        out.storedSecretProviders = Object.keys(JSON.parse(raw.provider_secrets_enc.toString('utf8')));
        out.storedMoesifSettings = JSON.parse(raw.provider_settings.toString('utf8')).moesif;
        out.moesifEndpointInStripe = state.endpoints.some((e) => e.url.includes(encodeURIComponent(${JSON.stringify(FIRST_KEYS.moesifCollectorApplicationId)})));
        out.storedText = Object.values(raw).map((v) => (Buffer.isBuffer(v) ? v.toString('utf8') : String(v))).join('|');
        const conn = await configurationDao.getWithSecrets(${JSON.stringify(ORG)});
        out.connectionKeys = { collector: conn.moesifCollectorApplicationId, key: conn.moesifManagementKey };
    } else if (process.env.PHASE === 'promoted') {
        // Another paid plan, created with the keys of an application billing does not use yet.
        const catalogService = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/services/billing/billingCatalogService.js'))});
        db.prepare('INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?)')
            .run('plan-silver', ${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'silver', 'Silver', 'silver', 'test', 'test');
        await require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/dao/planPricingDao.js'))}).create(${JSON.stringify(ORG)}, 'plan-silver',
            { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '49.00' });
        const pairs = [{ api: { uuid: 'api-1', name: 'Weather API', version: 'v1.0' }, planUuid: 'plan-silver' }];
        const stored = () => JSON.parse(db.prepare('SELECT provider_settings FROM billing_configurations').get().provider_settings.toString('utf8')).moesif;
        out.settingsBeforeMove = stored();
        const endpointsBeforeMove = state.endpoints.map((e) => e.id);

        stripeRefusesEndpoints = true;
        out.refusedMove = await catalogService.build(${JSON.stringify(ORG)}, await configurationDao.getWithSecrets(${JSON.stringify(ORG)}), pairs)
            .then(() => 'built', (err) => err.name);
        stripeRefusesEndpoints = false;
        out.moesifCallsOnRefusedMove = moesifCalls.splice(0);
        out.settingsAfterRefusedMove = stored();
        out.endpointsUnchangedAfterRefusedMove = JSON.stringify(state.endpoints.map((e) => e.id)) === JSON.stringify(endpointsBeforeMove);

        // The new application is connected to Stripe already, as one billing used earlier is: a
        // refused move leaves it connected.
        const thirdKey = ${JSON.stringify(THIRD_KEYS.moesifManagementKey)};
        state.partners[thirdKey] = true;
        stripeRefusesEndpoints = true;
        out.refusedMoveWhenConnected = await catalogService.build(${JSON.stringify(ORG)}, await configurationDao.getWithSecrets(${JSON.stringify(ORG)}), pairs)
            .then(() => 'built', (err) => err.name);
        stripeRefusesEndpoints = false;
        out.moesifCallsOnRefusedMoveWhenConnected = moesifCalls.splice(0);
        out.connectedAfterRefusedMove = Boolean(state.partners[thirdKey]);
        delete state.partners[thirdKey];

        // Recording the new application fails after its write: the write is rolled back with the
        // rest of the move, so the next paid plan moves billing again.
        const record = configurationDao.setMoesifApplication;
        configurationDao.setMoesifApplication = async (...args) => {
            await record(...args);
            throw new Error('database unavailable');
        };
        out.failedRecord = await catalogService.build(${JSON.stringify(ORG)}, await configurationDao.getWithSecrets(${JSON.stringify(ORG)}), pairs)
            .then(() => 'built', (err) => err.name);
        configurationDao.setMoesifApplication = record;
        moesifCalls.splice(0);
        out.settingsAfterFailedRecord = stored();
        out.endpointsUnchangedAfterFailedRecord = JSON.stringify(state.endpoints.map((e) => e.id)) === JSON.stringify(endpointsBeforeMove);

        const built = await catalogService.build(${JSON.stringify(ORG)}, await configurationDao.getWithSecrets(${JSON.stringify(ORG)}), pairs);
        await catalogService.saveApiPlans(built.apiPlans);
        out.moesifCallsOnMove = moesifCalls.splice(0);
        out.builtApplications = built.apiPlans.flatMap((p) => p.items.map((i) => i.application));
        out.settingsAfterMove = stored();
        out.endpointsAfterMove = state.endpoints.map((e) => ({ id: e.id, url: e.url }));

        // Stripe refuses removing the second of the two Moesif endpoints: the first is put back.
        const moesifUrls = () => state.endpoints.map((e) => e.url).filter((url) => url.includes('moesif')).sort();
        const moesifUrlsBefore = moesifUrls();
        let deletes = 0;
        stripeRefusesDelete = () => ++deletes === 2;
        out.refusedRemoval = await call(svc.deleteBillingConfiguration, as());
        stripeRefusesDelete = () => false;
        moesifCalls.splice(0);
        out.moesifEndpointsPutBack = JSON.stringify(moesifUrls()) === JSON.stringify(moesifUrlsBefore) && moesifUrlsBefore.length === 2;
        out.billingKeptAfterRefusedRemoval = (await call(svc.getBillingConfiguration, as())).status;

        const beforeRemoval = await configurationDao.getWithSecrets(${JSON.stringify(ORG)});
        out.deletedAfterMove = await call(svc.deleteBillingConfiguration, as());
        out.moesifCallsOnDeleteAfterMove = moesifCalls.splice(0);
        out.endpointsAfterDeleteAfterMove = state.endpoints.map((e) => e.url);
        out.partnersAfterDeleteAfterMove = Object.keys(state.partners);

        // A move that started before billing was removed: recording finds no billing, so the
        // move's connection and endpoint are undone instead of being left where removal can't find them.
        out.moveAfterRemoval = await catalogService.build(${JSON.stringify(ORG)},
            { ...beforeRemoval, moesifApplication: '6447:999', moesifEarlierApplications: [] }, pairs)
            .then(() => 'built', (err) => err.name);
        out.moesifCallsOnMoveAfterRemoval = moesifCalls.splice(0);
        out.endpointsAfterMoveAfterRemoval = state.endpoints.map((e) => e.url);
        out.partnersAfterMoveAfterRemoval = Object.keys(state.partners);
    } else {
        const conn = await configurationDao.getWithSecrets(${JSON.stringify(ORG)});
        out.connectionKeys = { collector: conn.moesifCollectorApplicationId, key: conn.moesifManagementKey };
        out.deleted = await call(svc.deleteBillingConfiguration, as());
        out.moesifCallsOnDelete = moesifCalls.splice(0);
        out.endpointsAfterDelete = state.endpoints.map((e) => e.url);
        out.partnersAfterDelete = Object.keys(state.partners);
        out.afterDelete = await call(svc.getBillingConfiguration, as());
        out.secondSetup = await call(svc.createBillingConfiguration, as({ body: stripeKeys }));
        out.moesifCallsOnSecondSetup = moesifCalls.splice(0);
    }
    fs.writeFileSync(STATE_FILE, JSON.stringify(state));
    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

const _results = {};
function run(phase, supplied) {
    if (_results[phase]) return _results[phase];
    if (!tmpDir) {
        tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-billing-deployment-'));
        const seed = path.join(tmpDir, 'seed.js');
        fs.writeFileSync(seed, `
            const fs = require('fs');
            const Database = require(${JSON.stringify(SQLITE)});
            const db = new Database(${JSON.stringify(path.join(tmpDir, 'portal.db'))});
            db.exec(fs.readFileSync(${JSON.stringify(SCHEMA)}, 'utf8'));
            db.prepare(\`INSERT INTO organizations
                (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by)
                VALUES (?,?,?,?,?,?,?,?)\`)
                .run(${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'Default', 'default', 'idp-1', '{}', 'test', 'test');
            db.prepare('INSERT INTO api_metadata (uuid, org_uuid, portal_id, name, status, version, type, handle, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?,?,?)')
                .run('api-1', ${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'Weather API', 'PUBLISHED', 'v1.0', 'RestApi', 'weather', 'test', 'test');
            db.prepare('INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?)')
                .run('plan-gold', ${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'gold', 'Gold', 'gold', 'test', 'test');
            db.prepare('INSERT INTO api_subscription_plan_mappings (uuid, api_uuid, plan_uuid, portal_id, created_by) VALUES (?,?,?,?,?)')
                .run('map-gold', 'api-1', 'plan-gold', ${JSON.stringify(PORTAL)}, 'test');
        `);
        const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
        assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);
    }
    const config = path.join(tmpDir, `config-${phase}.toml`);
    fs.writeFileSync(config, CONFIG(path.join(tmpDir, 'portal.db'), supplied));
    const runner = path.join(tmpDir, `probe-${phase}.js`);
    fs.writeFileSync(runner, PROBE.replaceAll(JSON.stringify('DB_FILE'), JSON.stringify(path.join(tmpDir, 'portal.db'))));
    const result = spawnSync(process.execPath, [runner, '--config', config], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: {
            PATH: process.env.PATH, HOME: process.env.HOME, PHASE: phase,
            STATE_FILE: path.join(tmpDir, 'stripe-state.json'),
            // Keeps the child's coverage in the parent run's report.
            ...(process.env.NODE_V8_COVERAGE && { NODE_V8_COVERAGE: process.env.NODE_V8_COVERAGE }),
        },
    });
    assert.equal(result.status, 0, `probe failed: ${result.stderr}`);
    const line = result.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${result.stdout}\n${result.stderr}`);
    _results[phase] = JSON.parse(line.slice('PROBE_JSON:'.length));
    return _results[phase];
}
const setUp = () => run('setup', FIRST_KEYS);
const moved = () => { setUp(); return run('moved', SECOND_KEYS); };
const promoted = () => { moved(); return run('promoted', THIRD_KEYS); };

test('the config supplies both Moesif keys', () => {
    assert.deepEqual(setUp().deploymentFields, ['moesifCollectorApplicationId', 'moesifManagementKey']);
});

test('a setup request that sends a key the deployment supplies is refused', () => {
    const { sentMoesifKey } = setUp();
    assert.equal(sentMoesifKey.status, 400);
    assert.equal(sentMoesifKey.body.code, 'BILLING_INPUT_INVALID');
    assert.deepEqual(sentMoesifKey.body.errors, [{ field: 'moesifManagementKey',
        message: "This portal's deployment sets this value, and it can't be updated." }]);
});

test('a supplied key Moesif refuses is reported as the deployment\'s, with nothing for the administrator to fix', () => {
    const { moesifRefused } = setUp();
    assert.equal(moesifRefused.status, 400);
    assert.equal(moesifRefused.body.code, 'BILLING_KEY_INVALID');
    assert.equal(moesifRefused.body.message,
        "Moesif refused the key this portal's deployment gives it. Contact your platform's support.");
});

test('billing is set up with the Stripe keys only, using the supplied Moesif keys', () => {
    const r = setUp();
    assert.equal(r.created.status, 201);
    assert.ok(r.moesifCallsOnCreate.length > 0);
    assert.ok(r.moesifCallsOnCreate.every(([, key]) => key === FIRST_KEYS.moesifManagementKey),
        'every Moesif call uses the supplied key');
    assert.ok(r.endpointsAfterCreate.some((url) => url.includes(encodeURIComponent(FIRST_KEYS.moesifCollectorApplicationId))),
        "Moesif's endpoint carries the supplied Collector Application Id");
});

test('the supplied keys are never stored', () => {
    const r = setUp();
    assert.deepEqual(r.storedSecretProviders, ['stripe']);
    for (const value of Object.values(FIRST_KEYS)) {
        assert.ok(!r.storedText.includes(value), 'a supplied key is in the stored row');
    }
    assert.deepEqual(r.connectionKeys, { collector: FIRST_KEYS.moesifCollectorApplicationId, key: FIRST_KEYS.moesifManagementKey });
});

test('setup records the Moesif application and Moesif\'s endpoint in Stripe', () => {
    const r = setUp();
    assert.equal(r.storedMoesifSettings.application, '6447:123');
    assert.ok(r.moesifEndpointInStripe);
});

test('with another application\'s keys, the connection uses them', () => {
    assert.deepEqual(moved().connectionKeys, { collector: SECOND_KEYS.moesifCollectorApplicationId, key: SECOND_KEYS.moesifManagementKey });
});

test('removing billing leaves the earlier application alone and removes everything in Stripe', () => {
    const r = moved();
    assert.equal(r.deleted.status, 204);
    assert.deepEqual(r.moesifCallsOnDelete, [], 'no Moesif call: the earlier application can\'t be reached');
    assert.deepEqual(r.endpointsAfterDelete, [], "Moesif's endpoint and the portal's are removed from Stripe");
    assert.ok(r.partnersAfterDelete.includes(FIRST_KEYS.moesifManagementKey), "the earlier application's connection to Stripe is left there");
    assert.equal(r.afterDelete.status, 404);
});

test('billing is then set up again in the new application', () => {
    const r = moved();
    assert.equal(r.secondSetup.status, 201);
    assert.ok(r.moesifCallsOnSecondSetup.some(([name, key]) => name === 'connect' && key === SECOND_KEYS.moesifManagementKey));
});

test('a paid plan created with another application\'s keys first moves billing there with the stored Stripe key', () => {
    const r = promoted();
    assert.equal(r.settingsBeforeMove.application, '6447:999');
    assert.deepEqual(r.moesifCallsOnMove.slice(0, 1), [['connect', THIRD_KEYS.moesifManagementKey]]);
    assert.ok(r.moesifCallsOnMove.every(([, key]) => key === THIRD_KEYS.moesifManagementKey), 'every Moesif call uses the new keys');
    assert.equal(r.settingsAfterMove.application, '6447:555');
    assert.ok(r.endpointsAfterMove.some((e) => e.url.includes(encodeURIComponent(THIRD_KEYS.moesifCollectorApplicationId))),
        "Moesif's endpoint for the new application is added in Stripe");
    assert.deepEqual(r.settingsAfterMove.earlierApplications, ['6447:999'], 'the earlier application is kept, so removal finds its endpoint');
    assert.deepEqual(r.builtApplications, ['6447:555'], 'the plan is made in the new application');
});

test('when Stripe refuses the new application\'s endpoint, the move is undone and no plan is created', () => {
    const r = promoted();
    assert.equal(r.refusedMove, 'BillingCatalogError');
    assert.deepEqual(r.moesifCallsOnRefusedMove, [['connect', THIRD_KEYS.moesifManagementKey], ['disconnect', THIRD_KEYS.moesifManagementKey]]);
    assert.deepEqual(r.settingsAfterRefusedMove, r.settingsBeforeMove);
    assert.ok(r.endpointsUnchangedAfterRefusedMove);
});

test('a refused move to an application already connected to Stripe leaves it connected', () => {
    const r = promoted();
    assert.equal(r.refusedMoveWhenConnected, 'BillingCatalogError');
    const key = THIRD_KEYS.moesifManagementKey;
    assert.deepEqual(r.moesifCallsOnRefusedMoveWhenConnected, [['connect', key], ['connect', key]],
        'the connection is saved again as it was, not removed');
    assert.ok(r.connectedAfterRefusedMove);
});

test('when recording the new application fails, nothing of the move is kept', () => {
    const r = promoted();
    assert.equal(r.failedRecord, 'BillingCatalogError');
    assert.deepEqual(r.settingsAfterFailedRecord, r.settingsBeforeMove, 'the application billing uses is unchanged');
    assert.ok(r.endpointsUnchangedAfterFailedRecord, "Moesif's endpoint for the new application is removed again");
});

test('removing billing after a move leaves the earlier application\'s plans and connection, and removes every endpoint', () => {
    const r = promoted();
    assert.equal(r.deletedAfterMove.status, 204);
    const key = THIRD_KEYS.moesifManagementKey;
    assert.deepEqual(r.moesifCallsOnDeleteAfterMove, [['archive-price', key], ['archive-plan', key], ['disconnect', key]],
        "only the plan made in the new application is taken off sale; the earlier application's plan is skipped");
    assert.deepEqual(r.endpointsAfterDeleteAfterMove, [], "both applications' Moesif endpoints and the portal's are removed from Stripe");
    assert.ok(r.partnersAfterDeleteAfterMove.includes(SECOND_KEYS.moesifManagementKey), "the earlier application's connection to Stripe is left there");
    assert.ok(!r.partnersAfterDeleteAfterMove.includes(THIRD_KEYS.moesifManagementKey));
});

test('when Stripe refuses removing one of Moesif\'s endpoints, the ones removed are put back and billing stays', () => {
    const r = promoted();
    assert.notEqual(r.refusedRemoval.status, 204);
    assert.ok(r.moesifEndpointsPutBack, "both applications' Moesif endpoints are in Stripe again");
    assert.equal(r.billingKeptAfterRefusedRemoval, 200);
});

test('a move that finds billing removed undoes its connection and endpoint', () => {
    const r = promoted();
    const key = THIRD_KEYS.moesifManagementKey;
    assert.equal(r.moveAfterRemoval, 'BillingCatalogError');
    assert.deepEqual(r.moesifCallsOnMoveAfterRemoval, [['connect', key], ['disconnect', key]]);
    assert.deepEqual(r.endpointsAfterMoveAfterRemoval, [], "Moesif's endpoint is removed again");
    assert.ok(!r.partnersAfterMoveAfterRemoval.includes(key), 'the new connection is removed again');
});
