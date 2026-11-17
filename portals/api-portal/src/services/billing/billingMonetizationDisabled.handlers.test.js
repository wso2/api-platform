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
 * Billing setup, meter creates and plan creates with monetization.disabled, against a real database,
 * with Stripe and Moesif replaced by in-memory test clients. The config supplies the Moesif keys, so
 * setup needs only the Stripe keys. Two runs: one with monetization disabled, one with it enabled.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..', '..');
const SERVICE = path.join(__dirname, 'billingConfigurationService.js');
const METER_SERVICE = path.join(__dirname, 'meterService.js');
const PLAN_SERVICE = path.join(__dirname, '..', 'apiMetadataService.js');
const SCHEMA = path.join(PROJECT_ROOT, 'database', 'schema.sqlite.sql');

const ORG = '0b5c9a4e-1f2d-4c3b-9a8e-7d6f5e4c3b2a';
const PORTAL = 'test-portal';
const COLLECTOR_ID = Buffer.from(JSON.stringify({ app: '6447:123', ver: '2.0' })).toString('base64url');
const STRIPE_KEYS = { stripeApiKey: 'rk_test_51Habcdefghijk4f2a', stripePublishableKey: 'pk_test_51Habcdefghij0000' };

let tmpDir;
test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

const CONFIG = (dbFile, disabled) => `
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

[api_portal.monetization]
disabled = ${disabled}

[api_portal.billing.moesif]
collector_application_id = ${JSON.stringify(COLLECTOR_ID)}
management_key = "moesif-key-deployment-0000"
`;

// One run of the portal; every Stripe and Moesif call is counted.
const PROBE = `
const crypto = require('crypto');
const svc = require(${JSON.stringify(SERVICE)});
const meters = require(${JSON.stringify(METER_SERVICE)});
const plans = require(${JSON.stringify(PLAN_SERVICE)});
const stripeConnector = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/connectors/moesif/payments/stripe/index.js'))});
const moesifConnector = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/connectors/moesif/index.js'))});
const Database = require('node:sqlite').DatabaseSync;
const out = {};

const endpoints = [];
const partners = {};
let providerCalls = 0;
stripeConnector.clients.stripe = () => ({
    async listWebhookEndpoints() { providerCalls++; return endpoints.map((e) => ({ ...e })); },
    async createWebhookEndpoint({ url, events }) {
        providerCalls++;
        const e = { id: 'we_' + crypto.randomBytes(4).toString('hex'), url, enabled_events: events, status: 'enabled',
            secret: 'whsec_' + crypto.randomBytes(8).toString('hex') };
        endpoints.push(e);
        return { ...e };
    },
    async updateWebhookEndpoint(id) { providerCalls++; return { ...endpoints.find((x) => x.id === id) }; },
    async deleteWebhookEndpoint() { providerCalls++; },
    async expireCheckoutSession(id) { providerCalls++; return { id, status: 'expired' }; },
});
moesifConnector.clients.moesif = (key) => ({
    async listCatalogPlans() { providerCalls++; return []; },
    async createCatalogPlan() { providerCalls++; return { id: 'prod_1' }; },
    async createCatalogPrice() { providerCalls++; return { id: 'price_1' }; },
    async createBillingMeter() { providerCalls++; return { id: 'bm_1' }; },
    async getPartnerAccount() { providerCalls++; return partners[key] ? { partner_account: { credentials: { state: 'enabled' } } } : {}; },
    async savePartnerAccount() { providerCalls++; partners[key] = true; },
    async deletePartnerAccount() { providerCalls++; delete partners[key]; },
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
const platformApi = (extra) => as({ ...extra, auth: { mode: 'shared-key', userId: null } });
const flatRate = { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '10' };

(async () => {
    out.setup = await call(svc.createBillingConfiguration, as({ body: ${JSON.stringify(STRIPE_KEYS)} }));
    out.providerCalls = providerCalls;
    out.read = await call(svc.getBillingConfiguration, as());

    const onMeters = { path: '/api/v0.9/meters' };
    out.meter = await call(meters.createMeter, as({ ...onMeters, body: { id: 'calls', displayName: 'Calls', type: 'REQUEST_COUNT' } }));
    out.platformApiMeter = await call(meters.createMeter,
        platformApi({ ...onMeters, body: { id: 'platform-calls', displayName: 'Platform calls', type: 'REQUEST_COUNT' } }));

    const onPlans = { path: '/api/v0.9/subscription-plans' };
    out.paidPlan = await call(plans.addSubscriptionPlans, as({ ...onPlans, body: { id: 'gold', displayName: 'Gold', pricing: flatRate } }));
    out.paidPlanUpserted = await call(plans.putSubscriptionPlans,
        as({ ...onPlans, body: [{ id: 'silver', displayName: 'Silver', pricing: flatRate }] }));
    out.freePlan = await call(plans.addSubscriptionPlans, as({ ...onPlans, body: { id: 'free', displayName: 'Free' } }));
    out.freePlanNullPricing = await call(plans.addSubscriptionPlans,
        as({ ...onPlans, body: { id: 'basic', displayName: 'Basic', pricing: null } }));
    out.platformApiPaidPlan = await call(plans.addSubscriptionPlans,
        platformApi({ ...onPlans, body: { id: 'platinum', displayName: 'Platinum', pricing: flatRate } }));

    const db = new Database(${JSON.stringify('DB_FILE')});
    out.storedConfigurations = db.prepare('SELECT COUNT(*) AS n FROM billing_configurations').get().n;
    out.storedMeters = db.prepare('SELECT handle FROM meters ORDER BY handle').all().map((r) => r.handle);
    out.storedPlans = db.prepare('SELECT handle FROM subscription_plans ORDER BY handle').all().map((r) => r.handle);
    console.log('PROBE_JSON:' + JSON.stringify(out));
    process.exit(0);
})().catch((e) => { console.error(e); process.exit(1); });
`;

const _results = {};
function run(disabled) {
    if (_results[disabled]) return _results[disabled];
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-billing-disabled-'));
    const dbFile = path.join(tmpDir, `portal-${disabled}.db`);
    const seed = path.join(tmpDir, `seed-${disabled}.js`);
    fs.writeFileSync(seed, `
        const fs = require('fs');
        const Database = require('node:sqlite').DatabaseSync;
        const db = new Database(${JSON.stringify(dbFile)});
        db.exec(fs.readFileSync(${JSON.stringify(SCHEMA)}, 'utf8'));
        db.prepare(\`INSERT INTO organizations
            (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by)
            VALUES (?,?,?,?,?,?,?,?)\`)
            .run(${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'Default', 'default', 'idp-1', '{}', 'test', 'test');
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);

    const config = path.join(tmpDir, `config-${disabled}.toml`);
    fs.writeFileSync(config, CONFIG(dbFile, disabled));
    const runner = path.join(tmpDir, `probe-${disabled}.js`);
    fs.writeFileSync(runner, PROBE.replaceAll(JSON.stringify('DB_FILE'), JSON.stringify(dbFile)));
    const result = spawnSync(process.execPath, [runner, '--config', config], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: {
            PATH: process.env.PATH, HOME: process.env.HOME,
            // Keeps the child's coverage in the parent run's report.
            ...(process.env.NODE_V8_COVERAGE && { NODE_V8_COVERAGE: process.env.NODE_V8_COVERAGE }),
        },
    });
    assert.equal(result.status, 0, `probe failed: ${result.stderr}`);
    const line = result.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${result.stdout}\n${result.stderr}`);
    _results[disabled] = JSON.parse(line.slice('PROBE_JSON:'.length));
    return _results[disabled];
}

test('with monetization disabled, billing setup is refused before any provider is called', () => {
    const r = run(true);
    assert.equal(r.setup.status, 409);
    assert.equal(r.setup.body.code, 'MONETIZATION_DISABLED');
    assert.equal(r.setup.body.message, 'Monetization is disabled for this portal, so billing cannot be set up.');
    assert.equal(r.providerCalls, 0);
    assert.equal(r.storedConfigurations, 0);
    assert.equal(r.read.status, 404);
});

test('with monetization disabled, only the Platform API creates meters', () => {
    const r = run(true);
    assert.equal(r.meter.status, 409);
    assert.equal(r.meter.body.code, 'MONETIZATION_DISABLED');
    assert.equal(r.platformApiMeter.status, 201);
    assert.deepEqual(r.storedMeters, ['platform-calls']);
});

test('with monetization disabled, free plans are created and only the Platform API creates paid plans', () => {
    const r = run(true);
    for (const refused of [r.paidPlan, r.paidPlanUpserted]) {
        assert.equal(refused.status, 409);
        assert.equal(refused.body.code, 'MONETIZATION_DISABLED');
    }
    assert.equal(r.freePlan.status, 201);
    assert.equal(r.freePlanNullPricing.status, 201);
    assert.equal(r.platformApiPaidPlan.status, 201);
    assert.deepEqual(r.storedPlans, ['basic', 'free', 'platinum']);
});

test('with monetization enabled, billing setup, meters and paid plans succeed', () => {
    const r = run(false);
    assert.equal(r.setup.status, 201);
    assert.ok(r.providerCalls > 0);
    assert.equal(r.storedConfigurations, 1);
    assert.equal(r.read.status, 200);
    assert.equal(r.meter.status, 201);
    assert.equal(r.paidPlan.status, 201);
    assert.equal(r.paidPlanUpserted.status, 201);
    assert.deepEqual(r.storedMeters, ['calls', 'platform-calls']);
    assert.deepEqual(r.storedPlans, ['basic', 'free', 'gold', 'platinum', 'silver']);
});
