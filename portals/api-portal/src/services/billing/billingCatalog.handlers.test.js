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
 * End-to-end tests for the billing catalog, against a real database with Moesif replaced by an
 * in-memory test client. A plan's pricing is given when the plan is created, before any API offers it.
 * The request that publishes a paid plan on an API (an API created or updated by the Platform API)
 * creates the plan in Moesif before it saves anything, and the plan is offered when it answers. When
 * Moesif fails or refuses any part, everything created is removed and the request fails, changing
 * nothing; doing it again creates the plan.
 *
 * Same setup as the other billing handler tests: a temporary SQLite file built from the shipped
 * schema, driven through one child process so later calls see what earlier ones did.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..', '..');
const SRC = path.join(PROJECT_ROOT, 'src');
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

[api_portal.billing.moesif]
max_parallel_prices = 2
`;

const PROBE = `
const src = ${JSON.stringify(SRC)};
const meters = require(src + '/services/billing/meterService.js');
const pricing = require(src + '/services/billing/planPricingService.js');
const apis = require(src + '/services/apiMetadataService.js');
const subscriptions = require(src + '/services/subscriptionService.js');
const checkout = require(src + '/services/billing/checkoutService.js');
const planPresentation = require(src + '/services/billing/planPresentation.js');
const stripeConnector = require(src + '/billing/connectors/moesif/payments/stripe/index.js');
const apiDao = require(src + '/dao/apiDao.js');
const subscriptionPlanDao = require(src + '/dao/subscriptionPlanDao.js');
const billingConfigurationDao = require(src + '/dao/billingConfigurationDao.js');
const billingApiPlanDao = require(src + '/dao/billingApiPlanDao.js');
const util = require(src + '/utils/util.js');
const { BillingProviderError } = require(src + '/billing/http.js');
const moesifConnector = require(src + '/billing/connectors/moesif/index.js');
const catalogService = require(src + '/services/billing/billingCatalogService.js');
// Here the cleanup does not wait out the provider's rate limit or outages.
catalogService.undoWaits.rateLimited = () => 0;
catalogService.undoWaits.unavailable = () => 0;
const db = require(src + '/db/driver.js');
const out = {};
const ORG = ${JSON.stringify(ORG)};
const PORTAL = ${JSON.stringify(PORTAL)};

// Moesif, with switches to refuse a call: 'plan', 'meter' (unavailable), 'plan-400' (refused) or
// 'flat-price-429' (rate limit), after letting \`skip\` such calls through.
const moesifCalls = [];
const meterBodies = [];
const fail = { next: null, skip: 0, cleanup: 0 };
// Run once, the next time Moesif is asked for a usage price or a billing meter.
const hooks = { price: null, meter: null };
async function runHook(kind) {
    const hook = hooks[kind];
    hooks[kind] = null;
    if (hook) await hook();
}
let seq = 0;
function maybeFail(kind) {
    if (fail.next === kind && fail.skip > 0) {
        fail.skip -= 1;
        return;
    }
    if (fail.next === kind) {
        fail.next = null;
        moesifCalls.push([kind + '-failed']);
        throw new BillingProviderError('Moesif', 503, 'Moesif: Service Unavailable');
    }
    if (fail.next === kind + '-400') {
        fail.next = null;
        moesifCalls.push([kind + '-refused']);
        throw new BillingProviderError('Moesif', 400, 'Moesif: Client error: An error occurred');
    }
    if (fail.next === kind + '-429') {
        fail.next = null;
        moesifCalls.push([kind + '-rate-limited']);
        throw new BillingProviderError('Moesif', 429, 'Moesif: Too Many Requests');
    }
}
// While \`measure\` is on, each Moesif plan takes a moment to create, and \`most\` records how many
// were being created at the same time.
const parallel = { measure: false, active: 0, most: 0 };
moesifConnector.clients.moesif = () => ({
    async listCatalogPlans() { return { hits: [] }; },
    async createCatalogPlan(provider, body) {
        maybeFail('plan');
        if (parallel.measure) {
            parallel.active += 1;
            parallel.most = Math.max(parallel.most, parallel.active);
            await new Promise((resolve) => setTimeout(resolve, 20));
            parallel.active -= 1;
        }
        moesifCalls.push(['plan', body.name]);
        return { id: 'prod_' + (++seq) };
    },
    async createCatalogPrice(provider, body) {
        const kind = body.pricing_model === 'flat' ? 'flat-price' : 'usage-price';
        maybeFail(kind);
        moesifCalls.push([kind, body.name]);
        if (kind === 'usage-price') await runHook('price');
        return { id: 'price_' + (++seq) };
    },
    async createBillingMeter(body) {
        maybeFail('meter');
        moesifCalls.push(['meter', body.name]);
        meterBodies.push(body);
        await runHook('meter');
        return { id: 'bm_' + (++seq) };
    },
    async archiveCatalogPlan(provider, id) {
        if (fail.cleanup > 0) {
            fail.cleanup -= 1;
            moesifCalls.push(['archive-plan-rate-limited', id]);
            throw new BillingProviderError('Moesif', 429, 'Moesif: Too Many Requests');
        }
        moesifCalls.push(['archive-plan', id]);
    },
    async deleteCatalogPrice(provider, id) { moesifCalls.push(['delete-price', id]); },
    async deleteBillingMeter(id) { moesifCalls.push(['delete-meter', id]); },
});

// Stripe, for a checkout: the prices it is asked to charge.
const checkoutSessions = [];
stripeConnector.clients.stripe = () => ({
    async createCustomer() { return { id: 'cus_' + (++seq) }; },
    async createCheckoutSession(args) {
        checkoutSessions.push(args.lineItems.map((item) => item.price));
        return { id: 'cs_' + (++seq), client_secret: 'cs_' + seq + '_secret' };
    },
});

function call(fn, req) {
    return new Promise((resolve) => {
        const headers = {};
        const res = {
            statusCode: 200,
            setHeader(k, v) { headers[k] = v; },
            status(c) { this.statusCode = c; return this; },
            json(b) { resolve({ status: this.statusCode, body: b }); return this; },
            send(b) { resolve({ status: this.statusCode, body: b === undefined ? null : b }); return this; },
        };
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.stack) }));
    });
}
const as = (extra) => Object.assign(
    { orgId: ORG, auth: { userId: 'admin-1' }, baseUrl: '/api-portal', path: '/api/v0.9/meters', query: {}, params: {}, body: {} },
    extra || {});

// The Platform API updating the API's listing with the plans it offers.
const offer = (...plans) => call(apis.updateAPIMetadata, as({ params: { apiId: 'weather' }, body: { metadata: JSON.stringify({
    name: 'Weather API', version: 'v1.0', type: 'RestApi', status: 'PUBLISHED',
    endPoints: { productionURL: 'https://backend.example.com' },
    subscriptionPlans: plans.map((id) => ({ id })),
}) } }));

// The Platform API publishing a new API with the plans it offers.
const publishNew = (...plans) => call(apis.createAPIMetadata, as({ body: { metadata: JSON.stringify({
    id: 'maps', name: 'Maps API', version: 'v1.0', type: 'RestApi', status: 'PUBLISHED', labels: [],
    endPoints: { productionURL: 'https://maps.example.com' },
    subscriptionPlans: plans.map((id) => ({ id })),
}) }, files: { definition: [{ originalname: 'openapi.json',
    buffer: Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'Maps API', version: 'v1.0' }, paths: {} })) }] } }));
// A plan created with its pricing, as the Platform API or an administrator creates a paid plan.
const createPlan = (id, displayName, planPricing) => call(apis.addSubscriptionPlans, as({ body: {
    id, displayName, refId: id, pricing: planPricing } }));

const visiblePlans = async () => (await util.appendSubscriptionPlanDetails(ORG,
    [{ id: 'free' }, { id: 'gold' }, { id: 'silver' }, { id: 'platinum' }], 'api-1')).map((p) => p.id + (p.paid ? ':paid' : ''));
const offered = async () => (await subscriptionPlanDao.listByApi('api-1')).map((p) => p.handle).sort();
// The calls so far, by kind, sorted: prices are created side by side, so their calls interleave.
const calls = () => moesifCalls.splice(0).map((c) => c[0]).sort();
(async () => {
    const statusField = [{ field: 'HTTP_STATUS_CODE' }];
    await call(meters.createMeter, as({ body: {
        displayName: 'Successful calls', type: 'REQUEST_COUNT', filterFields: statusField,
    } }));
    const goldPricing = { pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', prices: [{ pricingModel: 'PER_UNIT',
        unitAmount: '0.01', meterId: 'successful-calls',
        filters: [{ field: 'HTTP_STATUS_CODE', operator: 'IN', values: ['200'] }] }] };
    // Paid plans created before billing is set up: nothing is created in Moesif.
    out.goldWithoutBilling = await createPlan('gold', 'Gold', goldPricing);
    out.silverWithoutBilling = await createPlan('silver', 'Silver',
        { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '49.00' });
    out.callsWithoutBilling = calls();

    await billingConfigurationDao.create(ORG, {
        uuid: '11111111-2222-3333-4444-555555555555', provider: 'MOESIF', paymentProvider: 'STRIPE',
        moesifCollectorApplicationId: 'app', moesifManagementKey: 'mk',
        stripeApiKey: 'rk_test_abcd1234', stripePublishableKey: 'pk_test_demo1234', stripeWebhookSecret: 'whsec_x',
    }, 'admin-1');
    const conn = '11111111-2222-3333-4444-555555555555';

    // Enabling gold creates it in Moesif in the request. Moesif fails half-way: what was created is
    // removed, the update fails, and the API's plans stay as they were.
    fail.next = 'meter';
    out.enableFailed = await offer('free', 'gold');
    out.enableFailedCalls = calls();
    out.offeredAfterFailure = await offered();
    out.visibleAfterFailure = await visiblePlans();
    out.entriesAfterFailure = (await billingApiPlanDao.listForConfiguration(ORG, conn)).length;

    // Doing it again creates the plan, saves it and offers it by the time the update answers.
    out.enabled = await offer('free', 'gold');
    out.enabledCalls = calls();
    out.offeredAfterEnable = await offered();
    out.visibleAfterEnable = await visiblePlans();

    // Sending the same plans again creates nothing.
    out.again = await offer('free', 'gold');
    out.againCalls = calls();

    // A new API published with a paid plan: when Moesif fails, the API is not created; publishing
    // again creates it with an API plan of its own.
    fail.next = 'plan';
    out.publishFailed = await publishNew('free', 'gold');
    out.publishFailedCalls = calls();
    out.mapsAfterFailure = Boolean(await apiDao.getId(ORG, 'maps'));
    out.published = await publishNew('free', 'gold');
    out.publishedCalls = calls();
    const mapsUuid = await apiDao.getId(ORG, 'maps');
    out.mapsEntries = (await billingApiPlanDao.listForConfiguration(ORG, conn)).filter((e) => e.api_uuid === mapsUuid).length;

    // A plan Moesif refuses: the update fails, naming the plan, and changes nothing.
    fail.next = 'plan-400';
    out.refused = await offer('free', 'gold', 'silver');
    out.refusedCalls = calls();
    out.offeredAfterRefusal = await offered();

    // Moesif's rate limit: the update fails immediately, without waiting, and what was created is removed.
    fail.next = 'flat-price-429';
    out.rateLimited = await offer('free', 'gold', 'silver');
    out.rateLimitedCalls = calls();
    out.offeredAfterRateLimit = await offered();

    // The cleanup after a failure waits out Moesif's rate limit: refused once, the plan is still removed.
    fail.next = 'flat-price';
    fail.cleanup = 1;
    out.cleanupRetried = await offer('free', 'gold', 'silver');
    out.cleanupRetriedCalls = calls();

    // A free plan needs nothing from Moesif.
    out.offerFree = await offer('free', 'gold', 'platinum');
    out.offerFreeCalls = calls();

    // An AI plan: an input token price and an output token price for one HTTP method, on a meter each.
    // Its build fails at the second billing meter, then works. The output meter reads the method
    // from a key of its own, as for a gateway that names its fields differently.
    await call(meters.createMeter, as({ body: { displayName: 'LLM input tokens', type: 'TOKENS',
        typeKey: 'metadata.aiTokenUsage.promptTokens', filterFields: [{ field: 'HTTP_METHOD' }] } }));
    await call(meters.createMeter, as({ body: { displayName: 'LLM output tokens', type: 'TOKENS',
        typeKey: 'metadata.usage.outputTokens', filterFields: [{ field: 'HTTP_METHOD', eventKey: 'metadata.llm.method' }] } }));
    const llm = (meterId, unitAmount, method = 'POST') => ({ meterId, pricingModel: 'PER_UNIT', unitAmount, unitQuantity: 1000000,
        filters: [{ field: 'HTTP_METHOD', operator: 'EQUALS', values: [method] }] });
    const aiPrices = [llm('llm-input-tokens', '2.50'), llm('llm-output-tokens', '10.00')];
    out.aiWithFee = await createPlan('ai', 'AI', { pricingType: 'USAGE_BASED',
        currency: 'USD', billingPeriod: 'MONTH', flatAmount: '10.00', prices: aiPrices });
    out.aiOverlap = await createPlan('ai', 'AI', { pricingType: 'USAGE_BASED',
        currency: 'USD', billingPeriod: 'MONTH', prices: [aiPrices[0], llm('llm-input-tokens', '3.00')] });
    out.aiPerMethod = await createPlan('ai-methods', 'AI methods', { pricingType: 'USAGE_BASED',
        currency: 'USD', billingPeriod: 'MONTH', prices: [aiPrices[0], llm('llm-input-tokens', '0.15', 'GET')] });
    out.aiPricing = await createPlan('ai', 'AI', { pricingType: 'USAGE_BASED',
        currency: 'USD', billingPeriod: 'MONTH', prices: aiPrices });
    fail.next = 'meter';
    fail.skip = 1;
    out.aiFailed = await offer('free', 'gold', 'platinum', 'ai');
    out.aiFailedCalls = calls();
    out.aiEnabled = await offer('free', 'gold', 'platinum', 'ai');
    out.aiEnabledCalls = moesifCalls.splice(0);
    const aiPlan = await subscriptionPlanDao.getByName(ORG, 'ai');
    const aiEntry = (await billingApiPlanDao.listForConfiguration(ORG, conn)).find((e) => e.plan_uuid === aiPlan.uuid);
    out.aiItems = (await billingApiPlanDao.listItems(aiEntry.uuid)).map((i) => (i.price_uuid === null ? 'flat' : 'price'));
    const meterQuery = (name) => meterBodies.filter((b) => b.name === name).pop().es_query;
    out.aiInputQuery = meterQuery('Weather API v1.0 - AI - LLM input tokens · POST');
    out.aiOutputQuery = meterQuery('Weather API v1.0 - AI - LLM output tokens · POST');
    out.aiMeterLocked = await call(meters.updateMeter, as({ params: { meterId: 'llm-output-tokens' }, body: {
        displayName: 'LLM output tokens', type: 'TOKENS', typeKey: 'metadata.aiTokenUsage.totalTokens',
        filterFields: [{ field: 'HTTP_METHOD', eventKey: 'metadata.llm.method' }],
    } }));

    // A sold plan's pricing and its meter stay as they are.
    out.pricingLocked = await call(apis.putSubscriptionPlans, as({ body: { id: 'gold', displayName: 'Gold',
        pricing: { ...goldPricing, prices: [{ ...goldPricing.prices[0], unitAmount: '0.02' }] } } }));
    out.pricingAfterLock = await call(pricing.getPlanPricing, as({ params: { planId: 'gold' } }));
    out.meterCountingLocked = await call(meters.updateMeter, as({ params: { meterId: 'successful-calls' }, body: {
        displayName: 'Successful calls', type: 'TOKENS', typeKey: 'metadata.aiTokenUsage.promptTokens',
        filterFields: statusField,
    } }));
    out.meterRenamed = await call(meters.updateMeter, as({ params: { meterId: 'successful-calls' }, body: {
        displayName: 'Billed calls', type: 'REQUEST_COUNT', filterFields: statusField,
    } }));

    out.subscribePaid = await call(subscriptions.createSubscription, as({ body: { artifactId: 'weather', subscriptionPlanId: 'gold' } }));

    // A request whose own save fails after its paid plans were created removes them: here a plan the
    // update lists does not exist.
    await createPlan('bronze', 'Bronze', { pricingType: 'FLAT_RATE', currency: 'USD', billingPeriod: 'MONTH', flatAmount: '5.00' });
    out.saveFailed = await offer('free', 'gold', 'platinum', 'ai', 'bronze', 'no-such-plan');
    out.saveFailedCalls = moesifCalls.splice(0).map((c) => c[0]);
    const bronze = await subscriptionPlanDao.getByName(ORG, 'bronze');
    out.bronzeEntry = Boolean(await billingApiPlanDao.findApiPlan(conn, 'api-1', bronze.uuid));

    // --- prices that do not apply to an API's type ---------------------------
    // A meter that reads the API type at the default key, and one that reads it at a key of its own.
    await call(meters.createMeter, as({ body: { displayName: 'Typed calls', type: 'REQUEST_COUNT',
        filterFields: [{ field: 'API_TYPE' }] } }));
    await call(meters.createMeter, as({ body: { displayName: 'Kind calls', type: 'REQUEST_COUNT',
        filterFields: [{ field: 'API_TYPE', eventKey: 'metadata.kind' }] } }));
    const typed = (meterId, operator, values, unitAmount) => ({ pricingModel: 'PER_UNIT', unitAmount, meterId,
        filters: [{ field: 'API_TYPE', operator, values }] });
    const usagePlan = (prices) => ({ pricingType: 'USAGE_BASED', currency: 'USD', billingPeriod: 'MONTH', prices });
    // A REST price and an LLM price: only the REST one applies to a REST API.
    out.mixedPlan = await createPlan('mixed', 'Mixed', usagePlan([
        typed('typed-calls', 'EQUALS', ['RestApi'], '0.01'),
        typed('typed-calls', 'IN', ['LlmProvider', 'LlmProxy'], '0.02'),
    ]));
    // A price for anything but REST and MCP APIs, and one whose meter reads the type at its own key.
    out.notInPlan = await createPlan('not-rest', 'Not REST', usagePlan([
        typed('typed-calls', 'NOT_IN', ['RestApi', 'Mcp'], '0.03'),
        typed('kind-calls', 'IN', ['LlmProvider'], '0.04'),
    ]));
    // Only LLM prices: none applies to a REST API.
    out.llmOnlyPlan = await createPlan('llm-only', 'LLM only', usagePlan([typed('typed-calls', 'EQUALS', ['LlmProvider'], '0.05')]));
    moesifCalls.splice(0);
    const publishApi = (id, type, ...plans) => call(apis.createAPIMetadata, as({ body: { metadata: JSON.stringify({
        id, name: id + ' API', version: 'v1', type, status: 'PUBLISHED', labels: [],
        endPoints: { productionURL: 'https://' + id + '.example.com' },
        subscriptionPlans: plans.map((plan) => ({ id: plan })),
    }) }, files: { definition: [{ originalname: 'openapi.json',
        buffer: Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: id, version: 'v1' }, paths: {} })) }] } }));
    out.typedPublished = await publishApi('shop', 'RestApi', 'mixed', 'not-rest');
    out.typedCalls = moesifCalls.splice(0).sort((a, b) => a[1].localeCompare(b[1]) || 0);
    const shopUuid = await apiDao.getId(ORG, 'shop');
    const shopEntries = (await billingApiPlanDao.listForConfiguration(ORG, conn)).filter((e) => e.api_uuid === shopUuid);
    const planIds = new Map();
    for (const handle of ['mixed', 'not-rest']) planIds.set((await subscriptionPlanDao.getByName(ORG, handle)).uuid, handle);
    out.typedItems = {};
    for (const entry of shopEntries) {
        out.typedItems[planIds.get(entry.plan_uuid)] = (await billingApiPlanDao.listItems(entry.uuid)).map((i) => i.price_ref);
    }
    // A checkout charges the prices that apply only.
    out.typedCheckout = await call(checkout.createCheckoutSession, as({ auth: { userId: 'buyer-1' },
        body: { artifactId: 'shop', subscriptionPlanId: 'mixed' } }));
    out.typedCheckoutPrices = checkoutSessions.splice(0);
    // A plan none of whose prices applies: the API is not published and nothing is left with Moesif.
    out.noPriceApplies = await publishApi('wiki', 'RestApi', 'free', 'llm-only');
    out.noPriceAppliesCalls = moesifCalls.splice(0);
    out.wikiAfterRefusal = await apiDao.getId(ORG, 'wiki');
    // Plan cards on an API's page show the prices that apply to it.
    const cards = async (apiType) => {
        const plans = await util.appendSubscriptionPlanDetails(ORG, [{ id: 'mixed' }, { id: 'not-rest' }], shopUuid);
        await planPresentation.decoratePlans(ORG, plans, apiType);
        return plans.map((p) => ({ id: p.id, pricePrefix: p.pricePrefix, priceAmount: p.priceAmount, priceUnit: p.priceUnit,
            items: (p.priceItems || []).map((i) => i.name + ': ' + i.text) }));
    };
    out.restCards = await cards('RestApi');
    out.llmCards = await cards('LlmProvider');

    // An API's status is optional in a request: a create without one publishes the API, and an
    // update without one keeps its current status, whether or not the API has subscriptions.
    const apiStatus = async (handle) => (await db.queryOne(
        "SELECT status FROM api_metadata WHERE handle = ? AND org_uuid = ?", [handle, ORG])).status;
    const statusApiBody = (status) => ({ name: 'Status API', version: 'v1', type: 'RestApi', labels: [],
        endPoints: { productionURL: 'https://status.example.com' }, subscriptionPlans: [{ id: 'free' }],
        ...(status ? { status } : {}) });
    const updateStatusApi = (status) => call(apis.updateAPIMetadata, as({ params: { apiId: 'status-api' },
        body: { metadata: JSON.stringify(statusApiBody(status)) } }));
    out.createdWithoutStatus = await call(apis.createAPIMetadata, as({ body: { metadata: JSON.stringify({
        id: 'status-api', ...statusApiBody(null) }) }, files: { definition: [{ originalname: 'openapi.json',
        buffer: Buffer.from(JSON.stringify({ openapi: '3.0.3', info: { title: 'Status API', version: 'v1' }, paths: {} })) }] } }));
    out.statusAfterCreate = await apiStatus('status-api');
    out.statusSubscribed = await call(subscriptions.createSubscription, as({ auth: { userId: 'status-user' },
        body: { artifactId: 'status-api', subscriptionPlanId: 'free' } }));
    out.updatedWithoutStatus = await updateStatusApi(null);
    out.statusAfterUpdate = await apiStatus('status-api');
    out.deprecated = await updateStatusApi('DEPRECATED');
    out.updatedDeprecatedWithoutStatus = await updateStatusApi(null);
    out.statusAfterDeprecatedUpdate = await apiStatus('status-api');

    // At most [api_portal.billing.moesif] max_parallel_prices prices (2 here) are created at a time:
    // a plan of four prices is built two by two.
    moesifCalls.splice(0);
    out.quadPlan = await createPlan('quad', 'Quad', usagePlan(['200', '201', '202', '204'].map((code) => ({
        pricingModel: 'PER_UNIT', unitAmount: '0.01', meterId: 'successful-calls',
        filters: [{ field: 'HTTP_STATUS_CODE', operator: 'EQUALS', values: [code] }] }))));
    parallel.measure = true;
    out.quadPublished = await publishApi('quad-api', 'RestApi', 'quad');
    parallel.measure = false;
    out.quadMostAtOnce = parallel.most;
    out.quadPlansCreated = moesifCalls.splice(0).filter((c) => c[0] === 'plan').length;

    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

let _results;
function results() {
    if (_results) return _results;
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-catalog-'));
    const dbFile = path.join(tmpDir, 'portal.db');
    const seed = path.join(tmpDir, 'seed.js');
    fs.writeFileSync(seed, `
        const fs = require('fs');
        const Database = require(${JSON.stringify(SQLITE)});
        const db = new Database(${JSON.stringify(dbFile)});
        db.exec(fs.readFileSync(${JSON.stringify(SCHEMA)}, 'utf8'));
        const ORG = ${JSON.stringify(ORG)}, P = ${JSON.stringify(PORTAL)};
        db.prepare('INSERT INTO organizations (uuid, portal_id, display_name, handle, idp_ref_id, configuration, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?)')
            .run(ORG, P, 'Default', 'default', 'idp-1', '{}', 'test', 'test');
        db.prepare('INSERT INTO api_metadata (uuid, org_uuid, portal_id, name, status, version, type, handle, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?,?,?)')
            .run('api-1', ORG, P, 'Weather API', 'PUBLISHED', 'v1.0', 'RestApi', 'weather', 'test', 'test');
        // Two free plans; the paid ones are created with their pricing in the probe.
        for (const [uuid, handle, name] of [['plan-free', 'free', 'Free'], ['plan-platinum', 'platinum', 'Platinum']]) {
            db.prepare('INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?)')
                .run(uuid, ORG, P, handle, name, handle, 'test', 'test');
        }
        db.prepare('INSERT INTO api_subscription_plan_mappings (uuid, api_uuid, plan_uuid, portal_id, created_by) VALUES (?,?,?,?,?)')
            .run('map-free', 'api-1', 'plan-free', P, 'test');
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);
    const cfgFile = path.join(tmpDir, 'config.toml');
    fs.writeFileSync(cfgFile, CONFIG(dbFile));
    const runner = path.join(tmpDir, 'probe.js');
    fs.writeFileSync(runner, PROBE);
    const run = spawnSync(process.execPath, [runner, '--config', cfgFile], {
        cwd: PROJECT_ROOT, encoding: 'utf8', env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(run.status, 0, `probe failed: ${run.stderr}`);
    const line = run.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${run.stdout}\n${run.stderr}`);
    _results = JSON.parse(line.slice('PROBE_JSON:'.length));
    return _results;
}

// The message of a refusal: handleError puts a validation message in errors[0].
const refusal = (r) => (r.body.errors && r.body.errors[0] ? r.body.errors[0].message : r.body.message);

test('paid plans created before billing is set up create nothing in Moesif', () => {
    assert.equal(results().goldWithoutBilling.status, 201, JSON.stringify(results().goldWithoutBilling.body));
    assert.equal(results().goldWithoutBilling.body.paid, true);
    assert.equal(results().silverWithoutBilling.status, 201, JSON.stringify(results().silverWithoutBilling.body));
    assert.deepEqual(results().callsWithoutBilling, []);
});

test('a paid plan Moesif fails to create fails the update; what was created is removed and nothing changes', () => {
    const r = results();
    assert.equal(r.enableFailed.status, 409, JSON.stringify(r.enableFailed.body));
    assert.equal(refusal(r.enableFailed), 'The paid plan "Gold" could not be set up for sale on Weather API v1.0 right now. Try again.',
        'the provider\'s own reason is only logged');
    assert.deepEqual(r.enableFailedCalls, ['archive-plan', 'delete-price', 'meter-failed', 'plan', 'usage-price']);
    assert.deepEqual(r.offeredAfterFailure, ['free'], 'the update was not applied');
    assert.deepEqual(r.visibleAfterFailure, ['free', 'platinum']);
    assert.equal(r.entriesAfterFailure, 0);
});

test('doing it again creates the plan, its price and meter, and the plan is offered when the update answers', () => {
    const r = results();
    assert.equal(r.enabled.status, 200, JSON.stringify(r.enabled.body));
    assert.deepEqual(r.enabledCalls, ['meter', 'plan', 'usage-price']);
    assert.deepEqual(r.offeredAfterEnable, ['free', 'gold']);
    // silver is paid but cannot be bought on this API, so it is not shown; platinum is free.
    assert.deepEqual(r.visibleAfterEnable, ['free', 'gold:paid', 'platinum']);
});

test('sending the same plans again creates nothing more', () => {
    assert.equal(results().again.status, 200);
    assert.deepEqual(results().againCalls, []);
});

test('a new API published with a paid plan Moesif fails to create is not created; publishing again creates both', () => {
    const r = results();
    assert.equal(r.publishFailed.status, 409, JSON.stringify(r.publishFailed.body));
    assert.deepEqual(r.publishFailedCalls, ['plan-failed']);
    assert.equal(r.mapsAfterFailure, false);
    assert.equal(r.published.status, 201, JSON.stringify(r.published.body));
    // The plan is sold on Weather API already; Maps API gets its own.
    assert.deepEqual(r.publishedCalls, ['meter', 'plan', 'usage-price']);
    assert.equal(r.mapsEntries, 1);
});

test('a plan Moesif refuses fails the update, naming the plan, and changes nothing', () => {
    const r = results();
    assert.equal(r.refused.status, 409, JSON.stringify(r.refused.body));
    assert.equal(refusal(r.refused), 'Moesif could not create the paid plan "Silver" for Weather API v1.0.');
    assert.deepEqual(r.refusedCalls, ['plan-refused']);
    assert.deepEqual(r.offeredAfterRefusal, ['free', 'gold']);
});

test('Moesif\'s rate limit fails the update immediately, saying to try again, and removes what was created', () => {
    const r = results();
    assert.equal(r.rateLimited.status, 409, JSON.stringify(r.rateLimited.body));
    assert.equal(refusal(r.rateLimited), 'The paid plan "Silver" could not be set up for sale on Weather API v1.0 right now. Try again.');
    assert.deepEqual(r.rateLimitedCalls, ['archive-plan', 'flat-price-rate-limited', 'plan']);
    assert.deepEqual(r.offeredAfterRateLimit, ['free', 'gold']);
});

test('the cleanup after a failure waits out Moesif\'s rate limit, so nothing is left behind', () => {
    const r = results();
    assert.equal(r.cleanupRetried.status, 409, JSON.stringify(r.cleanupRetried.body));
    assert.deepEqual(r.cleanupRetriedCalls, ['archive-plan', 'archive-plan-rate-limited', 'flat-price-failed', 'plan']);
});

test('a free plan needs nothing from Moesif', () => {
    assert.equal(results().offerFree.status, 200, JSON.stringify(results().offerFree.body));
    assert.deepEqual(results().offerFreeCalls, []);
});

test('a usage based plan with token prices is created price by price, side by side, or not at all', () => {
    const r = results();
    assert.equal(r.aiWithFee.status, 400);
    assert.match(refusal(r.aiWithFee), /A usage based plan has no flat amount/);
    // One meter priced per method; the same method twice would count a request twice.
    assert.equal(r.aiOverlap.status, 400);
    assert.match(refusal(r.aiOverlap), /Prices 1 and 2 both count the same requests of the meter llm-input-tokens/);
    assert.equal(r.aiPerMethod.status, 201, JSON.stringify(r.aiPerMethod.body));
    assert.equal(r.aiPricing.status, 201, JSON.stringify(r.aiPricing.body));
    assert.equal(r.aiPricing.body.pricing.pricingType, 'USAGE_BASED');

    assert.equal(r.aiFailed.status, 409, JSON.stringify(r.aiFailed.body));
    // Both prices were under way when the second meter failed: everything created is removed.
    assert.deepEqual(r.aiFailedCalls, ['archive-plan', 'archive-plan', 'delete-meter', 'delete-price', 'delete-price',
        'meter', 'meter-failed', 'plan', 'plan', 'usage-price', 'usage-price']);
    // Each price is created plan, price, meter, in that order.
    const input = 'Weather API v1.0 - AI - LLM input tokens · POST';
    const output = 'Weather API v1.0 - AI - LLM output tokens · POST';
    for (const name of [input, output]) {
        assert.deepEqual(r.aiEnabledCalls.filter((c) => c[1] === name).map((c) => c[0]), ['plan', 'usage-price', 'meter']);
    }
    assert.equal(r.aiEnabledCalls.length, 6);
    assert.equal(r.aiEnabled.status, 200, JSON.stringify(r.aiEnabled.body));
    assert.deepEqual(r.aiItems, ['price', 'price']);
    // Each price's billing meter sums its meter's event key over the method, read from the meter's
    // key: the default for the input meter, the meter's own for the output meter.
    assert.deepEqual(r.aiInputQuery.query.bool.must.slice(2), [
        { terms: { 'request.verb.raw': ['POST'] } },
    ]);
    assert.deepEqual(r.aiInputQuery.aggs.seg.aggs.usage_value, { sum: { field: 'metadata.aiTokenUsage.promptTokens', missing: 0 } });
    assert.deepEqual(r.aiOutputQuery.query.bool.must.slice(2), [
        { terms: { 'metadata.llm.method.raw': ['POST'] } },
    ]);
    assert.deepEqual(r.aiOutputQuery.aggs.seg.aggs.usage_value, { sum: { field: 'metadata.usage.outputTokens', missing: 0 } });
    assert.equal(r.aiMeterLocked.status, 409, 'what a sold price counts can no longer change');
});

test('a request whose own save fails after its paid plans were created removes them', () => {
    assert.equal(results().saveFailed.status, 404, JSON.stringify(results().saveFailed.body));
    assert.deepEqual(results().saveFailedCalls, ['plan', 'flat-price', 'delete-price', 'archive-plan']);
    assert.equal(results().bronzeEntry, false);
});

test('a sold plan\'s pricing cannot change, nor anything about its meter, the name included', () => {
    assert.equal(results().pricingLocked.status, 409);
    assert.equal(results().pricingLocked.body.code, 'PLAN_PRICING_LOCKED');
    assert.equal(results().pricingAfterLock.body.prices[0].unitAmount, '0.01');
    assert.equal(results().meterCountingLocked.status, 409);
    assert.equal(results().meterCountingLocked.body.code, 'METER_IN_USE');
    assert.equal(results().meterRenamed.status, 409);
    assert.equal(results().meterRenamed.body.code, 'METER_IN_USE');
    assert.equal(results().meterRenamed.body.message, 'The meter is in use, so it can\'t be updated.');
});

test('a paid plan cannot be subscribed to directly', () => {
    assert.equal(results().subscribePaid.status, 409);
    assert.equal(results().subscribePaid.body.code, 'PAID_PLAN_REQUIRES_CHECKOUT');
});

// ---------------------------------------------------------------------------
// prices that do not apply to an API's type
// ---------------------------------------------------------------------------

test('a price whose API type rules out the API gets nothing in Moesif, and checkout charges the others', () => {
    const r = results();
    for (const key of ['mixedPlan', 'notInPlan', 'llmOnlyPlan']) assert.equal(r[key].status, 201, JSON.stringify(r[key].body));
    assert.equal(r.typedPublished.status, 201, JSON.stringify(r.typedPublished.body));
    // Mixed: the REST price only. Not REST: its NOT_IN price rules REST APIs out; the price whose
    // meter reads the type at a key of its own is kept, since the API's value there is unknown.
    assert.deepEqual(r.typedCalls, [
        ['plan', 'shop API v1 - Mixed - Typed calls · RestApi'],
        ['usage-price', 'shop API v1 - Mixed - Typed calls · RestApi'],
        ['meter', 'shop API v1 - Mixed - Typed calls · RestApi'],
        ['plan', 'shop API v1 - Not REST - Kind calls · LlmProvider'],
        ['usage-price', 'shop API v1 - Not REST - Kind calls · LlmProvider'],
        ['meter', 'shop API v1 - Not REST - Kind calls · LlmProvider'],
    ]);
    assert.equal(r.typedItems.mixed.length, 1, 'one catalog item: the REST price');
    assert.equal(r.typedItems['not-rest'].length, 1, 'one catalog item: the custom-key price');
    assert.equal(r.typedCheckout.status, 201, JSON.stringify(r.typedCheckout.body));
    assert.deepEqual(r.typedCheckoutPrices, [r.typedItems.mixed], 'the checkout charges the REST price only');
});

test('a usage plan none of whose prices applies to the API\'s type is refused, leaving nothing behind', () => {
    const r = results().noPriceApplies;
    assert.equal(r.status, 409, JSON.stringify(r.body));
    assert.equal(r.body.message, 'ERR_BILLING_CATALOG');
    assert.equal(r.body.errors[0].message, 'None of the prices of the plan LLM only apply to this API\'s type (RestApi).');
    assert.deepEqual(results().noPriceAppliesCalls, [], 'nothing was created in Moesif');
    assert.ok(!results().wikiAfterRefusal, 'the API was not published');
});

test('plan cards on an API\'s page show only the prices that apply to its type', () => {
    // A REST API: Mixed is its REST price alone; Not REST its custom-key price alone.
    assert.deepEqual(results().restCards, [
        { id: 'mixed', pricePrefix: null, priceAmount: '$0.01', priceUnit: '/ request', items: [] },
        { id: 'not-rest', pricePrefix: null, priceAmount: '$0.04', priceUnit: '/ request', items: [] },
    ]);
    // An LLM provider: Mixed is its LLM price; Not REST both prices.
    assert.deepEqual(results().llmCards, [
        { id: 'mixed', pricePrefix: null, priceAmount: '$0.02', priceUnit: '/ request', items: [] },
        { id: 'not-rest', pricePrefix: 'From', priceAmount: '$0.03', priceUnit: '/ request', items: [
            'Kind calls · LlmProvider: $0.04 per request',
            'Typed calls · not RestApi, Mcp: $0.03 per request',
        ] },
    ]);
});

test('an API created without a status is published, and an update without one keeps its status, subscriptions or not', () => {
    const r = results();
    assert.equal(r.createdWithoutStatus.status, 201, JSON.stringify(r.createdWithoutStatus.body));
    assert.equal(r.statusAfterCreate, 'PUBLISHED');
    assert.equal(r.statusSubscribed.status, 201, JSON.stringify(r.statusSubscribed.body));
    assert.equal(r.updatedWithoutStatus.status, 200, JSON.stringify(r.updatedWithoutStatus.body));
    assert.equal(r.statusAfterUpdate, 'PUBLISHED');
    assert.equal(r.deprecated.status, 200, JSON.stringify(r.deprecated.body));
    assert.equal(r.updatedDeprecatedWithoutStatus.status, 200, JSON.stringify(r.updatedDeprecatedWithoutStatus.body));
    assert.equal(r.statusAfterDeprecatedUpdate, 'DEPRECATED');
});

test('at most max_parallel_prices prices are created in Moesif at a time', () => {
    const r = results();
    assert.equal(r.quadPlan.status, 201, JSON.stringify(r.quadPlan.body));
    assert.equal(r.quadPublished.status, 201, JSON.stringify(r.quadPublished.body));
    assert.equal(r.quadPlansCreated, 4);
    assert.equal(r.quadMostAtOnce, 2);
});
