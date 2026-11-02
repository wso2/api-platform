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
 * Two runs share the database and the test Stripe account: the first sets billing up with the keys
 * of one Moesif application; the second starts with another application's keys in config and
 * removes billing.
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
const state = fs.existsSync(STATE_FILE) ? JSON.parse(fs.readFileSync(STATE_FILE, 'utf8')) : { endpoints: [], partner: null };
const moesifCalls = [];
let moesifRefuses = false;

stripeConnector.clients.stripe = () => ({
    async listWebhookEndpoints() { return state.endpoints.map((e) => ({ ...e })); },
    async createWebhookEndpoint({ url, events }) {
        const e = { id: 'we_' + crypto.randomBytes(4).toString('hex'), url, enabled_events: events, status: 'enabled',
            secret: 'whsec_' + crypto.randomBytes(8).toString('hex') };
        state.endpoints.push(e);
        return { ...e };
    },
    async updateWebhookEndpoint(id) { return { ...state.endpoints.find((x) => x.id === id) }; },
    async deleteWebhookEndpoint(id) { state.endpoints = state.endpoints.filter((x) => x.id !== id); },
    async expireCheckoutSession(id) { return { id, status: 'expired' }; },
});
// Each call is recorded with the key it was made with.
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
        return state.partner && state.partner.key === key ? { partner_account: { credentials: { state: 'enabled' } } } : {};
    },
    async savePartnerAccount() { moesifCalls.push(['connect', key]); state.partner = { key }; },
    async deletePartnerAccount() { moesifCalls.push(['disconnect', key]); state.partner = null; },
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
        out.storedMoesifEndpointIsMoesifs = state.endpoints.find((e) => e.id === out.storedMoesifSettings.eventEndpointId).url.includes('moesif');
        out.storedText = Object.values(raw).map((v) => (Buffer.isBuffer(v) ? v.toString('utf8') : String(v))).join('|');
        const conn = await configurationDao.getWithSecrets(${JSON.stringify(ORG)});
        out.connectionKeys = { collector: conn.moesifCollectorApplicationId, key: conn.moesifManagementKey };
    } else {
        const conn = await configurationDao.getWithSecrets(${JSON.stringify(ORG)});
        out.connectionKeys = { collector: conn.moesifCollectorApplicationId, key: conn.moesifManagementKey };
        out.deleted = await call(svc.deleteBillingConfiguration, as());
        out.moesifCallsOnDelete = moesifCalls.splice(0);
        out.endpointsAfterDelete = state.endpoints.map((e) => e.url);
        out.partnerAfterDelete = state.partner;
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
    assert.ok(r.storedMoesifEndpointIsMoesifs);
});

test('with another application\'s keys, the connection uses them', () => {
    assert.deepEqual(moved().connectionKeys, { collector: SECOND_KEYS.moesifCollectorApplicationId, key: SECOND_KEYS.moesifManagementKey });
});

test('removing billing leaves the earlier application alone and removes everything in Stripe', () => {
    const r = moved();
    assert.equal(r.deleted.status, 204);
    assert.deepEqual(r.moesifCallsOnDelete, [], 'no Moesif call: the earlier application can\'t be reached');
    assert.deepEqual(r.endpointsAfterDelete, [], "Moesif's endpoint and the portal's are removed from Stripe");
    assert.ok(r.partnerAfterDelete, "the earlier application's connection to Stripe is left there");
    assert.equal(r.afterDelete.status, 404);
});

test('billing is then set up again in the new application', () => {
    const r = moved();
    assert.equal(r.secondSetup.status, 201);
    assert.ok(r.moesifCallsOnSecondSetup.some(([name, key]) => name === 'connect' && key === SECOND_KEYS.moesifManagementKey));
});
