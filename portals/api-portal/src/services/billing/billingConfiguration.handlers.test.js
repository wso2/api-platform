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
 * End-to-end tests for the billing configuration and the Stripe webhook receiver, against a real
 * database, with Stripe and Moesif replaced by in-memory test clients (set through the connectors'
 * `clients`).
 *
 * Same setup as billing.handlers.test.js: a temporary SQLite file built from the shipped schema,
 * driven through one child process so later calls see what earlier ones did.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..', '..');
const SERVICE = path.join(__dirname, 'billingConfigurationService.js');
const WEBHOOK_SERVICE = path.join(__dirname, 'paymentWebhookService.js');
const SCHEMA = path.join(PROJECT_ROOT, 'database', 'schema.sqlite.sql');
const SQLITE = path.join(PROJECT_ROOT, 'node_modules', 'better-sqlite3');

const ORG = 'org-uuid-1';
const PORTAL = 'test-portal';

let tmpDir;
test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

const CONFIG = (dbFile, baseUrl) => `
[api_portal.server]
base_url = "${baseUrl}"

[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "${PORTAL}"

[api_portal.database]
type = "sqlite"
path = ${JSON.stringify(dbFile)}

[api_portal.billing.stripe]
webhook_signature_tolerance_sec = 600
`;

const PROBE = `
const crypto = require('crypto');
const svc = require(${JSON.stringify(SERVICE)});
const webhook = require(${JSON.stringify(WEBHOOK_SERVICE)});
const { BillingProviderError } = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/http.js'))});
const stripeConnector = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/connectors/moesif/payments/stripe/index.js'))});
const moesifConnector = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/connectors/moesif/index.js'))});
const moesifClient = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/billing/connectors/moesif/client.js'))});
const Database = require(${JSON.stringify(SQLITE)});
const out = {};

// One test Stripe account per key prefix pair: rk_test_* keys share "acct_test", rk_live_* "acct_live".
const accounts = {};
const calls = [];
function account(apiKey) {
    let name = 'test';
    if (apiKey.startsWith('rk_live_')) name = 'live';
    else if (apiKey.startsWith('rk_test_fresh')) name = 'fresh';
    else if (apiKey.startsWith('rk_test_kept')) name = 'kept';
    return accounts[name] = accounts[name] || { endpoints: [] };
}
let moesifRefuses = false;
let stripeRefusesUrl = false;
let catalogRefuses = false;
// Moesif's connection to Stripe: null (never set up), or { state, hasKey } as last saved.
let partner = null;
let partnerReadRefused = false;
let partnerSaveRefused = false;
let partnerDeleteRefused = false;
const partnerSaves = [];
const partnerDeletes = [];
let stripeRefusesPortalDelete = false;
const catalogCalls = [];
stripeConnector.clients.stripe = (apiKey) => ({
    async listWebhookEndpoints() { calls.push(['list', apiKey]); return account(apiKey).endpoints.map((e) => ({ ...e })); },
    async createWebhookEndpoint({ url, events }) {
        // Refuses the portal's address only: Moesif's endpoint is public.
        if (stripeRefusesUrl && !url.includes('moesif')) {
            throw new BillingProviderError('Stripe', 400, 'Stripe: Invalid URL: URL must be publicly accessible.');
        }
        const e = { id: 'we_' + crypto.randomBytes(4).toString('hex'), url, enabled_events: events, status: 'enabled',
            secret: 'whsec_' + crypto.randomBytes(8).toString('hex') };
        account(apiKey).endpoints.push(e);
        calls.push(['create', apiKey, url]);
        return { ...e };
    },
    async updateWebhookEndpoint(id, { url, events, disabled } = {}) {
        if (url && stripeRefusesUrl && !url.includes('moesif')) {
            throw new BillingProviderError('Stripe', 400, 'Stripe: Invalid URL: URL must be publicly accessible.');
        }
        const e = account(apiKey).endpoints.find((x) => x.id === id);
        if (e && url) e.url = url;
        if (e && events) e.enabled_events = events;
        if (e && disabled !== undefined) e.status = disabled ? 'disabled' : 'enabled';
        calls.push(['update', apiKey, id]);
        return { ...e };
    },
    async deleteWebhookEndpoint(id) {
        const a = account(apiKey);
        const e = a.endpoints.find((x) => x.id === id);
        if (stripeRefusesPortalDelete && e && !e.url.includes('moesif')) {
            throw new BillingProviderError('Stripe', 500, 'Stripe: An unexpected error occurred.');
        }
        a.endpoints = a.endpoints.filter((x) => x.id !== id);
        calls.push(['delete', apiKey, id]);
    },
    // cs_paid was paid just before it could be ended; any other checkout ends.
    async expireCheckoutSession(id) {
        calls.push(['expire', apiKey, id]);
        return id === 'cs_paid' ? { id, status: 'complete', payment_status: 'paid' } : { id, status: 'expired' };
    },
    // An invoice of no subscription: the receiver accepts its event and has nothing to act on.
    async retrieveInvoice(id) { return { id, subscription: null }; },
});
moesifConnector.clients.moesif = () => ({
    async listCatalogPlans() {
        if (moesifRefuses) throw new BillingProviderError('Moesif', 401, 'Moesif: Unauthorized');
        return [];
    },
    async createCatalogPlan(provider, body) {
        if (catalogRefuses) throw new BillingProviderError('Moesif', 400, 'Moesif: Client error: An error occurred');
        catalogCalls.push(['plan', body.name]);
        return { id: 'prod_' + catalogCalls.length };
    },
    async createCatalogPrice(provider, body) { catalogCalls.push(['price', body.name]); return { id: 'price_' + catalogCalls.length }; },
    async createBillingMeter(body) { catalogCalls.push(['meter', body.slug]); return { id: 'bm_' + catalogCalls.length }; },
    async archiveCatalogPlan(provider, id) { catalogCalls.push(['archive-plan', id]); },
    async deleteCatalogPrice(provider, id) { catalogCalls.push(['delete-price', id]); },
    async deleteBillingMeter(id) { catalogCalls.push(['delete-meter', id]); },
    async getPartnerAccount() {
        if (partnerReadRefused) throw new BillingProviderError('Moesif', 401, 'Moesif: Not authorized for requested organization.');
        return partner ? { partner_account: { credentials: { state: partner.state } } } : {};
    },
    async savePartnerAccount(provider, body) {
        // As Moesif does to a management key: the save needs a user role.
        if (partnerSaveRefused) throw new BillingProviderError('Moesif', 401, 'Moesif: Not authorized for requested organization. JWT has no org scopes or roles no-scope');
        const credentials = body.partner_account.credentials;
        partner = { state: credentials.state, hasKey: credentials.api_key === undefined ? false : credentials.api_key.startsWith('rk_') };
        partnerSaves.push(provider + ':' + credentials.state + ':' + body.billing_config.stripe.company_id_field);
    },
    async deletePartnerAccount(provider) {
        // As Moesif does to a management key without create:alert_rules.
        if (partnerDeleteRefused) throw new BillingProviderError('Moesif', 403, 'Moesif: token is missing or do not have permissions for the task');
        partner = null;
        partnerDeletes.push(provider);
    },
});

function call(fn, req) {
    return new Promise((resolve) => {
        const headers = {};
        const res = {
            statusCode: 200,
            setHeader(k, v) { headers[k] = v; },
            status(c) { this.statusCode = c; return this; },
            json(b) { resolve({ status: this.statusCode, body: b, headers }); return this; },
            send(b) { resolve({ status: this.statusCode, body: b === undefined ? null : b, headers }); return this; },
        };
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.message) }));
    });
}
const as = (extra) => Object.assign(
    { orgId: ${JSON.stringify(ORG)}, auth: { userId: 'admin-1' }, baseUrl: '/api-portal', path: '/api/v0.9/billing-configuration',
      query: {}, params: {}, body: {} }, extra || {});

const keys = {
    moesifCollectorApplicationId: 'eyJhcHAiOiI2NDQ3OjEyMyIsInZlciI6IjIuMCJ9',
    moesifManagementKey: 'moesif-management-key-0000-3b9e',
    stripeApiKey: 'rk_test_51Habcdefghijk4f2a',
    stripePublishableKey: 'pk_test_51Habcdefghij0000',
};

// ageSec: how many seconds ago the event was signed.
function signed(secret, body, ageSec = 0) {
    const t = Math.floor(Date.now() / 1000) - ageSec;
    const v1 = crypto.createHmac('sha256', secret).update(t + '.' + body).digest('hex');
    return { params: {}, body: Buffer.from(body), get: (h) => (h.toLowerCase() === 'stripe-signature' ? 't=' + t + ',v1=' + v1 : undefined) };
}

(async () => {
    // A paid plan already enabled for an API: setting up billing creates it in Moesif.
    await require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/dao/planPricingDao.js'))}).create(${JSON.stringify(ORG)}, 'plan-gold',
        { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '99.00', prices: [] }, 'admin');
    out.notConnected = await call(svc.getBillingConfiguration, as());
    out.missing = await call(svc.createBillingConfiguration, as({ body: { stripeApiKey: keys.stripeApiKey } }));
    out.publishable = await call(svc.createBillingConfiguration, as({ body: { ...keys, stripeApiKey: 'pk_test_abc' } }));
    out.unknownEngine = await call(svc.createBillingConfiguration, as({ body: { ...keys, provider: 'OTHER' } }));
    out.unknownPayment = await call(svc.createBillingConfiguration, as({ body: { ...keys, paymentProvider: 'CHARGEBEE' } }));
    calls.length = 0;
    out.noPublishable = await call(svc.createBillingConfiguration, as({ body: { ...keys, stripePublishableKey: undefined } }));
    out.badPublishable = await call(svc.createBillingConfiguration, as({ body: { ...keys, stripePublishableKey: 'rk_test_51Habcdefghijk4f2a' } }));
    out.otherModePublishable = await call(svc.createBillingConfiguration, as({ body: { ...keys, stripePublishableKey: 'pk_live_51Hzyxwvutsr0000' } }));
    out.publishableCalls = calls.length;
    moesifRefuses = true;
    out.moesifRefused = await call(svc.createBillingConfiguration, as({ body: keys }));
    moesifRefuses = false;

    out.created = await call(svc.createBillingConfiguration, as({ body: { ...keys, provider: 'MOESIF', paymentProvider: 'STRIPE' } }));
    out.catalogCallsOnCreate = catalogCalls.splice(0).map((c) => c[0] + ':' + c[1]);
    out.partnerAfterCreate = { ...partner, saves: partnerSaves.splice(0) };
    out.endpointsAfterCreate = accounts.test.endpoints.map((e) => e.url);
    out.moesifEventsAfterCreate = accounts.test.endpoints.find((e) => e.url.includes('moesif')).enabled_events;
    out.get = await call(svc.getBillingConfiguration, as());

    calls.length = 0;
    out.secondSetup = await call(svc.createBillingConfiguration, as({ body: keys }));
    out.secondSetupCalls = calls.length;
    out.engines = svc.BILLING_ENGINES;

    // Two setups at the same moment: this one's check misses the other's configuration, which is
    // saved first, so its own save is refused. It answers 409, not 500, and undoes its steps.
    const configurationDao = require(${JSON.stringify(path.join(PROJECT_ROOT, 'src/dao/billingConfigurationDao.js'))});
    const getConfiguration = configurationDao.get;
    let raced = false;
    configurationDao.get = async (...args) => {
        if (raced) return getConfiguration(...args);
        raced = true;
        return null;
    };
    const endpointsBeforeRace = accounts.test.endpoints.map((e) => e.url);
    out.racedSetup = await call(svc.createBillingConfiguration, as({ body: keys }));
    configurationDao.get = getConfiguration;
    out.racedEndpointsAdded = accounts.test.endpoints.map((e) => e.url).filter((u) => !endpointsBeforeRace.includes(u));
    out.racedConfigurationId = (await call(svc.getBillingConfiguration, as())).body.id;

    // The receiver, with the secret Stripe gave the portal's endpoint.
    const portalEndpoint = accounts.test.endpoints.find((e) => e.url === out.created.body.webhookEndpointUrl);
    const configurationId = out.created.body.id;
    const event = JSON.stringify({ id: 'evt_1', type: 'invoice.paid', data: { object: { id: 'in_1' } } });
    out.eventAccepted = await call(webhook.receivePaymentEvent, { ...signed(portalEndpoint.secret, event), params: { configurationId } });
    out.eventRepeated = await call(webhook.receivePaymentEvent, { ...signed(portalEndpoint.secret, event), params: { configurationId } });
    out.eventForged = await call(webhook.receivePaymentEvent, { ...signed('whsec_forged', event), params: { configurationId } });
    // The signature may be up to [api_portal.billing.stripe] webhook_signature_tolerance_sec old (600 here).
    out.eventSignedEarlier = await call(webhook.receivePaymentEvent, { ...signed(portalEndpoint.secret, event, 500), params: { configurationId } });
    out.eventSignedTooEarly = await call(webhook.receivePaymentEvent, { ...signed(portalEndpoint.secret, event, 700), params: { configurationId } });
    out.eventUnknownConfiguration = await call(webhook.receivePaymentEvent,
        { ...signed(portalEndpoint.secret, event), params: { configurationId: crypto.randomUUID() } });
    out.afterEvent = await call(svc.getBillingConfiguration, as());

    // The webhook host: an address that is not https is refused; the endpoint moves to another
    // host keeping its id and signing secret; an address Stripe refuses changes nothing.
    out.hostNotHttps = await call(svc.updateBillingWebhookEndpoint, as({ body: { webhookBaseUrl: 'http://proxy.example.com' } }));
    calls.length = 0;
    out.hostMoved = await call(svc.updateBillingWebhookEndpoint, as({ body: { webhookBaseUrl: 'https://proxy.example.com/portal/' } }));
    out.hostMovedCalls = calls.map((c) => c[0]);
    out.hostMovedEndpoint = { ...accounts.test.endpoints.find((e) => e.id === portalEndpoint.id) };
    out.eventAfterMove = await call(webhook.receivePaymentEvent, { ...signed(portalEndpoint.secret, event), params: { configurationId } });
    stripeRefusesUrl = true;
    out.hostRefused = await call(svc.updateBillingWebhookEndpoint, as({ body: { webhookBaseUrl: 'https://intranet.example.com' } }));
    stripeRefusesUrl = false;
    out.hostAfterRefused = (await call(svc.getBillingConfiguration, as())).body.webhookBaseUrl;
    out.hostMovedBack = await call(svc.updateBillingWebhookEndpoint, as({ body: { webhookBaseUrl: out.created.body.webhookBaseUrl } }));

    const db = new Database(${JSON.stringify('DB_FILE')});
    const raw = db.prepare('SELECT * FROM billing_configurations').get();
    out.storedText = Object.values(raw).map((v) => (Buffer.isBuffer(v) ? v.toString('utf8') : String(v))).join('|');

    // An active paid subscription: removing billing is refused, and nothing is touched.
    // The fixture rows below stand alone (no subscription or customer rows behind them).
    db.pragma('foreign_keys = OFF');
    db.prepare(\`INSERT INTO subscription_billing (subscription_uuid, org_uuid, portal_id, configuration_uuid,
        customer_uuid, api_plan_uuid, billing_subscription_id, billing_status) VALUES (?,?,?,?,?,?,?,?)\`)
        .run('sub-1', ${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, configurationId, 'cust-1', 'api-plan-1', 'sub_1', 'ACTIVE');
    out.describedActive = (await svc.describeConfiguration(${JSON.stringify(ORG)})).activePaidSubscriptions;
    out.blockedBySubscription = await call(svc.deleteBillingConfiguration, as());
    out.endpointsAfterBlocked = accounts.test.endpoints.length;
    db.prepare("UPDATE subscription_billing SET billing_status = 'CANCELLED'").run();
    out.describedCancelled = (await svc.describeConfiguration(${JSON.stringify(ORG)})).activePaidSubscriptions;

    // An open checkout that was paid just before it could be ended: refused like an active subscription.
    const checkout = db.prepare(\`INSERT INTO billing_checkouts (uuid, org_uuid, portal_id, user_uuid, api_uuid, plan_uuid,
        customer_uuid, checkout_ref, status) VALUES (?,?,?,?,?,?,?,?,?)\`);
    checkout.run('co-paid', ${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'user-1', 'api-1', 'plan-gold', 'cust-1', 'cs_paid', 'OPEN');
    out.blockedByPaidCheckout = await call(svc.deleteBillingConfiguration, as());
    db.prepare("UPDATE billing_checkouts SET status = 'COMPLETED' WHERE uuid = 'co-paid'").run();

    // Moesif refuses to remove its connection to Stripe (the key lacks the scope): nothing is removed.
    partnerDeleteRefused = true;
    out.partnerDeleteRefused = await call(svc.deleteBillingConfiguration, as());
    partnerDeleteRefused = false;
    out.afterPartnerDeleteRefused = (await call(svc.getBillingConfiguration, as())).status;

    // Stripe fails to remove the portal's endpoint: the removal is undone (Moesif connected again,
    // Moesif's endpoint registered again and recorded), and nothing is removed.
    checkout.run('co-open', ${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'user-2', 'api-1', 'plan-gold', 'cust-2', 'cs_open', 'OPEN');
    partnerDeletes.length = 0;
    partnerSaves.length = 0;
    stripeRefusesPortalDelete = true;
    out.removeFailed = await call(svc.deleteBillingConfiguration, as());
    stripeRefusesPortalDelete = false;
    out.afterRemoveFailed = {
        get: (await call(svc.getBillingConfiguration, as())).status,
        partner: { ...partner, deletes: partnerDeletes.splice(0), saves: partnerSaves.splice(0) },
        endpoints: accounts.test.endpoints.map((e) => ({ id: e.id, moesif: e.url.includes('moesif') })),
        storedMoesifEndpoint: db.prepare('SELECT moesif_webhook_endpoint_id AS id FROM billing_configurations').get().id,
        openCheckout: db.prepare("SELECT status FROM billing_checkouts WHERE uuid = 'co-open'").get().status,
    };

    out.deleted = await call(svc.deleteBillingConfiguration, as());
    out.testEndpointsAfterDelete = accounts.test.endpoints.map((e) => e.url);
    out.partnerAfterDelete = { state: partner && partner.state, deletes: partnerDeletes.splice(0) };
    out.afterDelete = await call(svc.getBillingConfiguration, as());

    // Going live: remove billing, then set it up again with a live key.
    out.live = await call(svc.createBillingConfiguration, as({ body: { ...keys, stripeApiKey: 'rk_live_51Hzyxwvutsr9z9z',
        stripePublishableKey: 'pk_live_51Hzyxwvutsr0000' } }));
    out.liveEndpoints = accounts.live.endpoints.map((e) => e.url);
    await call(svc.deleteBillingConfiguration, as());

    catalogCalls.length = 0;

    // Stripe refuses the portal's address: setup fails, and Moesif's endpoint, registered just
    // before, is removed again.
    const fresh = { ...keys, stripeApiKey: 'rk_test_fresh51Habcdefghij' };
    partner = null;
    partnerSaves.length = 0;
    partnerDeletes.length = 0;
    stripeRefusesUrl = true;
    out.refusedAddress = await call(svc.createBillingConfiguration, as({ body: fresh }));
    stripeRefusesUrl = false;
    out.freshAfterRefusedAddress = (accounts.fresh || { endpoints: [] }).endpoints.map((e) => e.url);
    out.catalogCallsOnRefusedAddress = catalogCalls.splice(0).map((c) => c[0]);
    out.afterRefusedAddress = await call(svc.getBillingConfiguration, as());
    out.partnerAfterRefusedAddress = { ...partner, saves: partnerSaves.splice(0), deletes: partnerDeletes.splice(0) };

    // A Moesif key that may not read Moesif's connection to Stripe: the error is under the key.
    partnerReadRefused = true;
    out.partnerReadRefused = await call(svc.createBillingConfiguration, as({ body: fresh }));
    partnerReadRefused = false;
    out.freshAfterPartnerReadRefused = accounts.fresh.endpoints.map((e) => e.url);



    // Moesif refuses a paid plan already enabled for an API: setup fails and both endpoints go.
    catalogRefuses = true;
    out.refusedCatalog = await call(svc.createBillingConfiguration, as({ body: fresh }));
    catalogRefuses = false;
    out.freshAfterRefusedCatalog = accounts.fresh.endpoints.map((e) => e.url);
    out.afterRefusedCatalog = await call(svc.getBillingConfiguration, as());

    // Stripe not connected, and Moesif refuses to save the connection to the key: setup fails,
    // with the error under the Moesif key, and nothing is left in Stripe.
    partner = null;
    partnerSaves.length = 0;
    partnerSaveRefused = true;
    out.partnerSaveRefused = await call(svc.createBillingConfiguration, as({ body: fresh }));
    out.partnerSaveRefusedState = partner;
    out.freshAfterPartnerSaveRefused = accounts.fresh.endpoints.map((e) => e.url);
    out.afterPartnerSaveRefused = await call(svc.getBillingConfiguration, as());

    // Stripe connected in the Moesif dashboard: setup continues without saving the connection.
    partner = { state: 'enabled' };
    partnerSaves.length = 0;
    partnerDeletes.length = 0;
    out.alreadyConnected = await call(svc.createBillingConfiguration, as({ body: fresh }));
    out.alreadyConnectedSaves = partnerSaves.splice(0);
    partnerSaveRefused = false;
    await call(svc.deleteBillingConfiguration, as());
    out.partnerAfterAlreadyConnectedDelete = { ...partner, deletes: partnerDeletes.splice(0) };

    // The application ID must be a Collector Application ID, for the management key's application.
    calls.length = 0;
    out.appNumber = await call(svc.createBillingConfiguration, as({ body: { ...keys, moesifCollectorApplicationId: '524:75' } }));
    const otherAppKey = ['eyJhbGciOiJIUzI1NiJ9', Buffer.from(JSON.stringify({ app: '1:1', org: '9:9' })).toString('base64url'), 'sig'].join('.');
    out.otherApp = await call(svc.createBillingConfiguration, as({ body: { ...keys, moesifManagementKey: otherAppKey } }));
    out.appCheckCalls = calls.length;

    // Moesif answers 400 "Unable to get the Partner account" once a Stripe connection was removed
    // in the dashboard: the real client reads that as no connection; any other 400 is an error.
    const answering = (status, data) => ({ async request() { return { status, data }; } });
    out.partnerGone = await moesifClient.createMoesifClient({ managementKey: 'k',
        http: answering(400, { message: 'Client error: Unable to get the Partner account' }) }).getPartnerAccount('stripe');
    out.partnerOther400 = await moesifClient.createMoesifClient({ managementKey: 'k',
        http: answering(400, { message: 'Client error: Bad request' }) }).getPartnerAccount('stripe').then(() => 'read', (e) => e.status);

    // Moesif's endpoint already in Stripe with fewer events and turned off: setup adds the events
    // and turns it on; a failed setup puts it back as it was.
    const kept = { ...keys, stripeApiKey: 'rk_test_kept51Habcdefghij' };
    const moesifUrl = 'https://api.moesif.net/v1/partners/stripe?moesif_application_id=' + encodeURIComponent(keys.moesifCollectorApplicationId);
    accounts.kept = { endpoints: [{ id: 'we_kept', url: moesifUrl, enabled_events: ['invoice.paid'], status: 'disabled' }] };
    partner = { state: 'enabled' };
    stripeRefusesUrl = true;
    out.keptRefused = await call(svc.createBillingConfiguration, as({ body: kept }));
    stripeRefusesUrl = false;
    out.keptAfterRefused = accounts.kept.endpoints.map((e) => ({ id: e.id, events: e.enabled_events, status: e.status }));
    out.keptCreated = await call(svc.createBillingConfiguration, as({ body: kept }));
    out.keptAfterCreate = accounts.kept.endpoints.filter((e) => e.id === 'we_kept')
        .map((e) => ({ events: e.enabled_events, status: e.status }));
    await call(svc.deleteBillingConfiguration, as());
    out.keptAfterDelete = accounts.kept.endpoints.map((e) => e.id);

    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

let _results;
function results() {
    if (_results) return _results;
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-billing-conn-'));
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
        db.prepare('INSERT INTO api_metadata (uuid, org_uuid, portal_id, name, status, version, type, handle, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?,?,?)')
            .run('api-1', ${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'Weather API', 'PUBLISHED', 'v1.0', 'RestApi', 'weather', 'test', 'test');
        db.prepare('INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?)')
            .run('plan-gold', ${JSON.stringify(ORG)}, ${JSON.stringify(PORTAL)}, 'gold', 'Gold', 'gold', 'test', 'test');
        db.prepare('INSERT INTO api_subscription_plan_mappings (uuid, api_uuid, plan_uuid, portal_id, created_by) VALUES (?,?,?,?,?)')
            .run('map-gold', 'api-1', 'plan-gold', ${JSON.stringify(PORTAL)}, 'test');
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);

    const cfgFile = path.join(tmpDir, 'config.toml');
    fs.writeFileSync(cfgFile, CONFIG(dbFile, 'https://developer.example.com'));
    const runner = path.join(tmpDir, 'probe.js');
    fs.writeFileSync(runner, PROBE.replaceAll(JSON.stringify('DB_FILE'), JSON.stringify(dbFile)));
    const run = spawnSync(process.execPath, [runner, '--config', cfgFile], {
        cwd: PROJECT_ROOT, encoding: 'utf8', env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(run.status, 0, `probe failed: ${run.stderr}`);
    const line = run.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${run.stdout}\n${run.stderr}`);
    _results = JSON.parse(line.slice('PROBE_JSON:'.length));
    return _results;
}

test('without a connection, reads answer 404', () => {
    assert.equal(results().notConnected.status, 404);
});

test('the first save needs all three keys and a secret or restricted Stripe key', () => {
    assert.equal(results().missing.status, 400);
    assert.match(results().missing.body.message, /moesifCollectorApplicationId, moesifManagementKey/);
    assert.equal(results().publishable.status, 400);
});

test('the billing engine and payment provider must be a supported pair', () => {
    assert.equal(results().unknownEngine.status, 400);
    assert.match(results().unknownEngine.body.message, /provider must be one of: MOESIF/);
    assert.equal(results().unknownPayment.status, 400);
    assert.match(results().unknownPayment.body.message, /paymentProvider must be one of: STRIPE for MOESIF/);
    assert.equal(results().created.body.provider, 'MOESIF');
    assert.equal(results().created.body.paymentProvider, 'STRIPE');
    assert.deepEqual(results().engines.map((e) => [e.id, e.paymentProviders.map((p) => p.id)]),
        [['MOESIF', ['STRIPE']]]);
});

test('billing is set up once: a second setup is refused, calling no provider', () => {
    assert.equal(results().secondSetup.status, 409);
    assert.equal(results().secondSetup.body.message, 'Billing is already set up. Remove it to set it up again.');
    assert.equal(results().secondSetupCalls, 0);
});

test('a setup that loses a race with another answers 409, keeps the first configuration and removes its own endpoint', () => {
    const r = results().racedSetup;
    assert.equal(r.status, 409, JSON.stringify(r.body));
    assert.equal(r.body.message, 'Billing is already set up. Remove it to set it up again.');
    assert.equal(results().racedConfigurationId, results().created.body.id);
    assert.deepEqual(results().racedEndpointsAdded, [], 'the losing setup\'s portal endpoint is removed again');
});

test('a key a provider refuses is a 400 naming the provider, and nothing is saved', () => {
    const r = results().moesifRefused;
    assert.equal(r.status, 400);
    assert.equal(r.body.code, 'BILLING_KEY_INVALID');
    assert.match(r.body.message, /^This Moesif key is invalid\. Please enter a valid key\./);
    assert.doesNotMatch(r.body.message, /Unauthorized/, 'the provider\'s own reason is only logged');
    assert.deepEqual(r.body.errors, [{ field: 'moesifManagementKey', message: 'This Moesif key is invalid. Please enter a valid key.' }],
        'the refusal names the key field it is about, so the form can show it there');
    assert.equal(results().endpointsAfterCreate.length, 2, 'the refused save must not have registered endpoints');
});

test('saving registers the portal\'s endpoint and Moesif\'s in Stripe and returns no part of a key', () => {
    const r = results().created;
    assert.equal(r.status, 201);
    assert.equal(r.headers.Location, '/api-portal/api/v0.9/billing-configuration');
    assert.equal(r.body.mode, 'TEST');
    assert.equal(r.body.stripeKeyHint, undefined, 'no part of a key is returned');
    assert.equal(r.body.moesifManagementKeyHint, undefined, 'no part of a key is returned');
    assert.equal(r.body.moesifCollectorApplicationId, undefined, 'the Collector Application Id is a key too and is not returned');
    assert.match(r.body.webhookEndpointUrl, /^https:\/\/developer\.example\.com\/api-portal\/billing\/stripe\/webhooks\/[0-9a-f-]{36}$/);
    assert.equal(r.body.webhookRegistered, true);
    assert.equal(r.body.moesifWebhookRegistered, true);
    assert.deepEqual(results().endpointsAfterCreate.sort(), [
        'https://api.moesif.net/v1/partners/stripe?moesif_application_id=eyJhcHAiOiI2NDQ3OjEyMyIsInZlciI6IjIuMCJ9',
        r.body.webhookEndpointUrl,
    ].sort());
    assert.equal(results().get.status, 200);
});

test('a signed Stripe event is recorded once; a forged, misdirected or too old one is refused', () => {
    assert.equal(results().eventAccepted.status, 200);
    assert.equal(results().eventRepeated.status, 200);
    assert.equal(results().eventForged.status, 400);
    assert.equal(results().eventSignedEarlier.status, 200, 'signed 500 s ago, within the configured 600 s');
    assert.equal(results().eventSignedTooEarly.status, 400, 'signed 700 s ago, past the configured 600 s');
    assert.equal(results().eventUnknownConfiguration.status, 404);
    assert.ok(results().afterEvent.body.lastEventAt, 'the last event time is shown');
});

test('going live means removing billing and setting it up again with a live key', () => {
    const r = results().live;
    assert.equal(r.status, 201);
    assert.equal(r.body.mode, 'LIVE');
    assert.notEqual(r.body.id, results().created.body.id, 'a new connection, so nothing from the test one is reused');
    assert.equal(results().liveEndpoints.length, 2);
});

test('no key or signing secret is stored in plain text', () => {
    const stored = results().storedText;
    for (const secret of ['moesif-management-key-0000-3b9e', 'rk_live_51Hzyxwvutsr9z9z', 'rk_test_51Habcdefghijk4f2a', 'whsec_',
        'eyJhcHAiOiI2NDQ3OjEyMyIsInZlciI6IjIuMCJ9']) {
        assert.ok(!stored.includes(secret), `${secret} found in the stored row`);
    }
    assert.ok(stored.includes('pk_test_51Habcdefghij0000'), 'the publishable key is public and stored as is');
});

test('the Stripe publishable key is required, must be one, and must be in the secret key\'s mode; no provider is called', () => {
    assert.equal(results().noPublishable.status, 400);
    assert.match(results().noPublishable.body.message, /stripePublishableKey/);
    for (const [r, message] of [
        [results().badPublishable, 'Enter a publishable key (pk_test_… or pk_live_…).'],
        [results().otherModePublishable, 'This is a live publishable key, but the secret key is a test key. Use keys from the same mode.'],
    ]) {
        assert.equal(r.status, 400, JSON.stringify(r.body));
        assert.equal(r.body.code, 'BILLING_INPUT_INVALID');
        assert.deepEqual(r.body.errors, [{ field: 'stripePublishableKey', message }]);
    }
    assert.equal(results().publishableCalls, 0);
});

test('removing billing leaves nothing made with the entered keys: Moesif\'s Stripe connection and both endpoints go', () => {
    assert.equal(results().deleted.status, 204, JSON.stringify(results().deleted.body));
    assert.deepEqual(results().testEndpointsAfterDelete, [], 'no endpoint is left in Stripe');
    assert.deepEqual(results().partnerAfterDelete, { state: null, deletes: ['stripe'] }, 'Moesif no longer holds the Stripe key');
    assert.equal(results().afterDelete.status, 404);
});

test('removing billing is refused while a paid subscription is active, touching nothing', () => {
    assert.equal(results().describedActive, 1, 'the settings page knows before trying');
    const r = results().blockedBySubscription;
    assert.equal(r.status, 409);
    assert.equal(r.body.code, 'BILLING_SUBSCRIPTIONS_ACTIVE');
    assert.equal(r.body.message, 'Billing can\'t be removed while paid subscriptions are active.');
    assert.equal(results().endpointsAfterBlocked, 2);
    assert.equal(results().describedCancelled, 0, 'a cancelled subscription does not count');
});

test('a checkout paid just before it could be ended refuses the removal like an active subscription', () => {
    const r = results().blockedByPaidCheckout;
    assert.equal(r.status, 409);
    assert.equal(r.body.code, 'BILLING_SUBSCRIPTIONS_ACTIVE');
});

test('setup puts the webhook endpoint under the portal\'s own address by default', () => {
    const r = results().created;
    assert.equal(r.body.webhookBaseUrl, 'https://developer.example.com');
    assert.equal(r.body.webhookEndpointUrl, `https://developer.example.com/api-portal/billing/stripe/webhooks/${r.body.id}`);
});

test('the webhook host must be an https address', () => {
    const r = results().hostNotHttps;
    assert.equal(r.status, 400, JSON.stringify(r.body));
    assert.equal(r.body.code, 'BILLING_INPUT_INVALID');
    assert.deepEqual(r.body.errors, [{ field: 'webhookBaseUrl', message: 'Enter an https address, such as https://portal.example.com.' }]);
});

test('changing the webhook host moves the endpoint in Stripe, keeping its id and signing secret', () => {
    const r = results().hostMoved;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.webhookBaseUrl, 'https://proxy.example.com/portal', 'the trailing slash is dropped');
    assert.equal(r.body.webhookEndpointUrl, `https://proxy.example.com/portal/api-portal/billing/stripe/webhooks/${r.body.id}`);
    assert.deepEqual(results().hostMovedCalls, ['list', 'update'], 'moved, not deleted and created again');
    assert.equal(results().hostMovedEndpoint.url, r.body.webhookEndpointUrl);
    assert.equal(results().eventAfterMove.status, 200, 'events signed with the same secret are still accepted');
});

test('a webhook host Stripe refuses changes nothing, and Stripe\'s own reason is not shown', () => {
    const r = results().hostRefused;
    assert.equal(r.status, 400, JSON.stringify(r.body));
    assert.equal(r.body.code, 'BILLING_INPUT_INVALID');
    assert.match(r.body.errors[0].message,
        /^Stripe did not accept the webhook endpoint https:\/\/intranet\.example\.com\/\S+\. Use a public https address that Stripe can reach\.$/);
    assert.equal(r.body.errors[0].field, 'webhookBaseUrl');
    assert.equal(results().hostAfterRefused, 'https://proxy.example.com/portal');
    assert.equal(results().hostMovedBack.status, 200);
});

test('when Moesif refuses to remove its connection to Stripe, nothing is removed and Moesif\'s own reason is not shown', () => {
    const r = results().partnerDeleteRefused;
    assert.equal(r.status, 400, JSON.stringify(r.body));
    assert.equal(r.body.code, 'BILLING_REMOVE_FAILED');
    assert.equal(r.body.message, 'Moesif did not remove its connection to Stripe. Billing was not removed. '
        + 'The Moesif key does not have the Alert rules: create scope. Disconnect Stripe from the Moesif application, '
        + 'then remove billing again.');
    assert.equal(results().afterPartnerDeleteRefused, 200, 'the connection is still there');
});

test('when Stripe fails to remove the portal\'s endpoint, the removal is undone and nothing is removed', () => {
    const r = results().removeFailed;
    assert.equal(r.status, 502);
    assert.equal(r.body.code, 'BILLING_REMOVE_FAILED');
    assert.equal(r.body.message, 'Stripe did not remove the portal\'s webhook endpoint. Billing was not removed. '
        + 'Stripe could not be reached. Try again later.', 'the provider\'s own reason is only logged');
    const after = results().afterRemoveFailed;
    assert.equal(after.get, 200, 'the connection is still there');
    assert.deepEqual(after.partner.deletes, ['stripe']);
    assert.deepEqual(after.partner.saves, ['stripe:enabled:id'], 'Moesif is connected to Stripe again');
    assert.equal(after.partner.hasKey, true);
    const moesifEndpoint = after.endpoints.find((e) => e.moesif);
    assert.ok(moesifEndpoint, 'Moesif\'s endpoint is registered again');
    assert.equal(after.storedMoesifEndpoint, moesifEndpoint.id, 'and recorded');
    assert.equal(after.endpoints.length, 2);
    assert.equal(after.openCheckout, 'EXPIRED', 'open checkouts are ended first');
});

test('removing billing leaves what was there before setup: a Moesif connection made in the dashboard and an existing Moesif endpoint', () => {
    assert.deepEqual(results().partnerAfterAlreadyConnectedDelete, { state: 'enabled', deletes: [] });
    assert.deepEqual(results().keptAfterDelete, ['we_kept']);
});

test('setting up billing creates the paid plans already enabled for APIs in Moesif', () => {
    assert.deepEqual(results().catalogCallsOnCreate, ['plan:Weather API v1.0 - Gold', 'price:Weather API v1.0 - Gold']);
});

test('setup fails when Stripe refuses the portal\'s address, and Moesif\'s endpoint is removed again', () => {
    const r = results().refusedAddress;
    assert.equal(r.status, 400);
    assert.equal(r.body.code, 'BILLING_SETUP_FAILED');
    assert.match(r.body.message, /^Stripe did not accept the portal's webhook endpoint https:\/\/developer\.example\.com\/\S+\. Use a public https address that Stripe can reach\.$/);
    assert.deepEqual(results().freshAfterRefusedAddress, [], 'nothing is left in Stripe');
    assert.deepEqual(results().catalogCallsOnRefusedAddress, [], 'nothing is created in Moesif');
    assert.equal(results().afterRefusedAddress.status, 404, 'nothing is saved');
});

test('setting up billing connects the Moesif application to Stripe with the Stripe key', () => {
    assert.equal(results().partnerAfterCreate.state, 'enabled');
    assert.equal(results().partnerAfterCreate.hasKey, true);
    assert.deepEqual(results().partnerAfterCreate.saves, ['stripe:enabled:id']);
});

test('a failed setup removes Moesif\'s Stripe connection again, key included, when it was off before', () => {
    assert.deepEqual(results().partnerAfterRefusedAddress.saves, ['stripe:enabled:id']);
    assert.deepEqual(results().partnerAfterRefusedAddress.deletes, ['stripe']);
    assert.equal(results().partnerAfterRefusedAddress.state, undefined);
});

test('a Moesif key that may not read Moesif\'s connection to Stripe fails setup, with the error under that key', () => {
    const r = results().partnerReadRefused;
    assert.equal(r.status, 400);
    assert.equal(r.body.code, 'BILLING_KEY_INVALID');
    assert.deepEqual(r.body.errors, [{ field: 'moesifManagementKey',
        message: 'This Moesif key cannot connect the Moesif application to Stripe. It needs the App configs scopes listed in Settings › Billing.' }]);
    assert.deepEqual(results().freshAfterPartnerReadRefused, [], 'nothing was registered in Stripe');
});

test('a Moesif key that may not save Moesif\'s connection to Stripe fails setup, with the error under that key', () => {
    const r = results().partnerSaveRefused;
    assert.equal(r.status, 400);
    assert.equal(r.body.code, 'BILLING_KEY_INVALID');
    assert.deepEqual(r.body.errors, [{ field: 'moesifManagementKey',
        message: 'This Moesif key cannot connect the Moesif application to Stripe. It needs the App configs scopes listed in Settings › Billing.' }]);
    assert.equal(results().partnerSaveRefusedState, null, 'nothing was saved in Moesif');
    assert.deepEqual(results().freshAfterPartnerSaveRefused, [], 'nothing was registered in Stripe');
    assert.equal(results().afterPartnerSaveRefused.status, 404, 'billing is not set up');
});

test('with Stripe already connected in the Moesif dashboard, setup goes on without saving the connection', () => {
    assert.equal(results().alreadyConnected.status, 201, JSON.stringify(results().alreadyConnected.body));
    assert.deepEqual(results().alreadyConnectedSaves, []);
});

test('setup fails when Moesif refuses a paid plan, naming it, and both endpoints are removed again', () => {
    const r = results().refusedCatalog;
    assert.equal(r.status, 400);
    assert.equal(r.body.code, 'BILLING_CATALOG_FAILED');
    assert.equal(r.body.message, 'Moesif could not create the paid plan "Gold" for Weather API v1.0.',
        'the provider\'s own reason is only logged');
    assert.deepEqual(results().freshAfterRefusedCatalog, []);
    assert.equal(results().afterRefusedCatalog.status, 404);
});

test('a Stripe connection removed in Moesif reads as none, so removing billing can continue', () => {
    assert.equal(results().partnerGone, null);
    assert.equal(results().partnerOther400, 400, 'any other 400 is still an error');
});

test('the application ID must be a Collector Application ID for the management key\'s application', () => {
    for (const [r, message] of [
        [results().appNumber, 'This is not a Collector Application Id. Copy it from the API Keys page in Moesif.'],
        [results().otherApp, 'This Collector Application Id is for a different Moesif application than the Management API key.'],
    ]) {
        assert.equal(r.status, 400, JSON.stringify(r.body));
        assert.equal(r.body.code, 'BILLING_INPUT_INVALID');
        assert.deepEqual(r.body.errors, [{ field: 'moesifCollectorApplicationId', message }]);
    }
    assert.equal(results().appCheckCalls, 0, 'no provider is called');
});

test('Moesif\'s endpoint receives every Customer and Invoice event', () => {
    const events = results().moesifEventsAfterCreate;
    assert.equal(events.length, 38);
    assert.ok(events.every((e) => /^(customer|invoice)\./.test(e)));
    assert.ok(events.includes('customer.discount.created') && events.includes('invoice.will_be_due'));
});

test('an existing Moesif endpoint gets the missing events and is turned on; a failed setup puts it back', () => {
    assert.equal(results().keptRefused.status, 400, JSON.stringify(results().keptRefused.body));
    assert.deepEqual(results().keptAfterRefused, [{ id: 'we_kept', events: ['invoice.paid'], status: 'disabled' }]);
    assert.equal(results().keptCreated.status, 201, JSON.stringify(results().keptCreated.body));
    const [kept] = results().keptAfterCreate;
    assert.equal(kept.status, 'enabled');
    assert.equal(kept.events.length, 38);
    assert.equal(kept.events[0], 'invoice.paid', 'the events already there are kept');
});
