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
 * End-to-end tests for a subscriber's paid subscription after checkout, against a real database
 * with Stripe and Moesif replaced by in-memory test clients: the billing details and the one
 * "payment pending" state, the Usage page counted with the billing meter's own filter, the
 * Invoices page and Stripe's billing page, the administrator's revenue report, and unsubscribing,
 * which ends the subscription in Stripe first.
 *
 * Same setup as checkout.handlers.test.js: a temporary SQLite file built from the shipped schema,
 * driven through one child process so later calls see what earlier ones did.
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
`;

const PROBE = `
const src = ${JSON.stringify(SRC)};
const checkout = require(src + '/services/billing/checkoutService.js');
const events = require(src + '/services/billing/paymentEventService.js');
const billing = require(src + '/services/billing/subscriptionBillingService.js');
const usage = require(src + '/services/billing/usageService.js');
const revenue = require(src + '/services/billing/revenueService.js');
const account = require(src + '/services/billing/billingAccountService.js');
const subscriptions = require(src + '/services/subscriptionService.js');
const billingConfigurationDao = require(src + '/dao/billingConfigurationDao.js');
const billingApiPlanDao = require(src + '/dao/billingApiPlanDao.js');
const billingCheckoutDao = require(src + '/dao/billingCheckoutDao.js');
const planPricingDao = require(src + '/dao/planPricingDao.js');
const meterDao = require(src + '/dao/meterDao.js');
const { BillingProviderError } = require(src + '/billing/http.js');
const stripeConnector = require(src + '/billing/connectors/moesif/payments/stripe/index.js');
const moesifConnector = require(src + '/billing/connectors/moesif/index.js');
const Database = require(${JSON.stringify(SQLITE)});
const ORG = ${JSON.stringify(ORG)};
const out = {};

// ── Stripe and Moesif test clients ──
const state = { sessions: {}, status: 'active', subs: {}, failCancel: false, failMoesif: false };
const calls = [];
let seq = 0;
const stripe = {
    async createCustomer() { return { id: 'cus_' + (++seq) }; },
    async createCheckoutSession(args) {
        const id = 'cs_' + (++seq);
        state.sessions[id] = { id, client_secret: id + '_secret', status: 'open', payment_status: 'unpaid', subscription: null,
            client_reference_id: args.clientReferenceId };
        return { id, client_secret: id + '_secret' };
    },
    async retrieveCheckoutSession(id) { return { ...state.sessions[id] }; },
    async retrieveSubscription(id) {
        return { id, status: state.status, current_period_start: 1760000000, current_period_end: 1762600000,
            ...(state.subs[id] || {}) };
    },
    async updateSubscription(id, body) {
        if (state.failCancel) throw new BillingProviderError('Stripe', 503, 'Stripe: Service Unavailable');
        calls.push(['end', id, body]);
        state.subs[id] = { ...(state.subs[id] || {}), ...body };
        return this.retrieveSubscription(id);
    },
    async listBillingPortalConfigurations() { return { data: state.portalConfig ? [state.portalConfig] : [] }; },
    async createBillingPortalConfiguration(body) {
        calls.push(['portalConfig', body]);
        state.portalConfig = { id: 'bpc_1', active: true, metadata: body.metadata };
        return state.portalConfig;
    },
    async retrieveInvoice(id) { return { id, subscription: 'sub_paid' }; },
    async previewInvoice(sub) {
        calls.push(['preview', sub]);
        return { currency: 'usd', total: 2812, period_start: 1760000000, period_end: 1762600000, lines: { data: [
            { price: { id: 'price_calls' }, quantity: 2812, amount: 2812, description: '2812 x calls' },
        ] } };
    },
    // The lines of an invoice past the first ones it embeds.
    async listInvoiceLinePage(invoice, startingAfter) {
        calls.push(['invoiceLines', invoice, startingAfter]);
        return { data: (state.moreLines || {})[invoice] || [], has_more: false };
    },
    async listInvoices(sub) {
        calls.push(['invoices', sub]);
        return [
            { id: 'in_2', created: 1760000000, total: 9900, currency: 'usd', status: 'open', hosted_invoice_url: 'https://invoice.stripe.test/in_2' },
            { id: 'in_1', created: 1757400000, total: 13140, currency: 'usd', status: 'paid', hosted_invoice_url: 'javascript:alert(1)' },
        ];
    },
    async listInvoicePage(args) {
        calls.push(['invoicePage', args]);
        if (state.failInvoices) throw new BillingProviderError('Stripe', 503, 'Stripe: Service Unavailable');
        let data = (state.invoices || []).filter((i) => (!args.customer || i.customer === args.customer)
            && (args.createdGte === undefined || i.created >= args.createdGte)
            && (args.createdLt === undefined || i.created < args.createdLt));
        if (args.startingAfter) {
            const at = data.findIndex((i) => i.id === args.startingAfter);
            if (at < 0) throw new BillingProviderError('Stripe', 400, "Stripe: No such invoice: '" + args.startingAfter + "'", 'resource_missing');
            data = data.slice(at + 1);
        }
        const limit = args.limit || 100;
        return { data: data.slice(0, limit), has_more: data.length > limit };
    },
    async createBillingPortalSession(args) { calls.push(['portal', args]); return { url: 'https://billing.stripe.test/s_1' }; },
    async cancelSubscription(id, opts) {
        if (state.failCancel) throw new BillingProviderError('Stripe', 503, 'Stripe: Service Unavailable');
        calls.push(['cancel', id, opts]);
        state.status = 'canceled';
        return { id, status: 'canceled' };
    },
};
stripeConnector.clients.stripe = () => stripe;
moesifConnector.clients.moesif = () => ({
    async searchEvents(args) {
        if (state.failMoesif) throw new BillingProviderError('Moesif', 503, 'Moesif: Service Unavailable');
        calls.push(['search', args]);
        // Two buckets just after the start of the range, at the interval asked for.
        const step = args.body.aggs.series.date_histogram.calendar_interval === '1h' ? 3600000 : 86400000;
        const first = Math.floor(Date.parse(args.from) / step) * step + step;
        return { aggregations: { series: { buckets: [
            { key: first, key_as_string: new Date(first).toISOString(), doc_count: 7, usage: { value: 7 } },
            { key: first + step, key_as_string: new Date(first + step).toISOString(), doc_count: 5, usage: { value: 5 } },
        ] } } };
    },
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
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.stack) }));
    });
}
const as = (user, extra) => Object.assign(
    { orgId: ORG, auth: { userId: user }, user: { email: user + '@example.com' }, baseUrl: '/api-portal',
      path: '/api/v0.9/checkout-sessions', query: {}, params: {}, body: {} },
    extra || {});
const sql = (q, ...p) => new Database(${JSON.stringify('DB_FILE')}).prepare(q).all(...p);
const run = (q, ...p) => new Database(${JSON.stringify('DB_FILE')}).prepare(q).run(...p);
// Delivers a payment event as the webhook receiver acts on it; resolves to its outcome.
function receive(type, objectId) {
    return events.handle({ orgId: ORG, type, objectRef: objectId });
}

(async () => {
    // A paid plan that is ready to sell, bought by alice through checkout. Its meter reads the status
    // code from a key of its own, which the usage search must use.
    const meter = await meterDao.create(ORG, { handle: 'calls', displayName: 'Successful calls', type: 'REQUEST_COUNT',
        typeKey: null, filterFields: [{ field: 'Status code', eventKey: 'response.statusCode' }] }, 'admin');
    const { pricing } = await planPricingDao.create(ORG, 'plan-gold', { pricingType: 'USAGE_BASED', currency: 'USD',
        billingPeriod: 'MONTH',
        prices: [{ meter, filters: [{ field: 'Status code', operator: 'IN', values: ['200'] }], pricingModel: 'PER_UNIT',
            unitAmount: '0.01', unitQuantity: 1, tiers: null }] });
    await billingConfigurationDao.create(ORG, { billing_provider: 'MOESIF', payment_provider: 'STRIPE',
        settings: { stripe: { publishableKey: 'pk_test_demo1234' } },
        secrets: { moesif: { collectorApplicationId: 'app', managementKey: 'mk' },
            stripe: { apiKey: 'rk_test_abcd1234', webhookSecret: 'whsec_x' } } }, 'admin');
    await billingApiPlanDao.insertApiPlan({ orgId: ORG, apiUuid: 'api-1', planUuid: 'plan-gold', items: [{ priceUuid: pricing.prices[0].uuid, planRef: 'prod_1', priceRef: 'price_calls', meterRef: 'bm_1' }] });
    // A flat rate plan too: unsubscribing from it runs to the end of the paid period.
    const silver = (await planPricingDao.create(ORG, 'plan-silver', { pricingType: 'FLAT_RATE', currency: 'USD',
        billingPeriod: 'MONTH', flatAmount: '19.00' })).pricing;
    await billingApiPlanDao.insertApiPlan({ orgId: ORG, apiUuid: 'api-1', planUuid: 'plan-silver',
        items: [{ priceUuid: silver.flat_price_uuid, planRef: 'prod_2', priceRef: 'price_flat', meterRef: null }] });

    const started = await call(checkout.createCheckoutSession, as('alice', { body: { artifactId: 'weather', subscriptionPlanId: 'gold' } }));
    const session = Object.values(state.sessions)[0];
    Object.assign(session, { status: 'complete', payment_status: 'paid', subscription: 'sub_paid' });
    const done = await call(checkout.getCheckoutSession, as('alice', { params: { checkoutId: started.body.id } }));
    const subId = done.body.subscriptionId;
    out.free = await call(subscriptions.createSubscription, as('bob', { body: { artifactId: 'weather', subscriptionPlanId: 'free' } }));
    // One subscription per user per API, free or paid.
    out.freeAgain = await call(subscriptions.createSubscription, as('bob', { body: { artifactId: 'weather', subscriptionPlanId: 'free' } }));
    out.freeWhilePaid = await call(subscriptions.createSubscription, as('alice', { body: { artifactId: 'weather', subscriptionPlanId: 'free' } }));

    out.billing = await call(billing.getSubscriptionBilling, as('alice', { params: { subId } }));
    out.otherUser = await call(billing.getSubscriptionBilling, as('bob', { params: { subId } }));
    out.freeBilling = await call(billing.getSubscriptionBilling, as('bob', { params: { subId: out.free.body.subscriptionId } }));
    out.suspend = await call(subscriptions.updateSubscription, as('alice', { params: { subId }, body: { status: 'INACTIVE' } }));

    out.usage = await call(usage.getUsage, as('alice'));
    out.search = (calls.find((c) => c[0] === 'search') || [])[1];
    calls.length = 0;
    out.usageHourly = await call(usage.getUsage, as('alice', { query: { period: 'LAST_24_HOURS' } }));
    out.hourlySearch = (calls.find((c) => c[0] === 'search') || [])[1];
    calls.length = 0;
    out.usageWeekly = await call(usage.getUsage, as('alice', { query: { period: 'CURRENT', interval: 'WEEK' } }));
    out.weeklySearch = (calls.find((c) => c[0] === 'search') || [])[1];
    out.usage7 = await call(usage.getUsage, as('alice', { query: { period: 'LAST_7_DAYS' } }));
    out.usageBinNotOffered = await call(usage.getUsage, as('alice', { query: { period: 'LAST_24_HOURS', interval: 'WEEK' } }));
    out.usageBinUnknown = await call(usage.getUsage, as('alice', { query: { interval: 'MINUTE' } }));
    // A billing period still running: the days to come are in the series, with no value yet.
    const at = new Date();
    out.filled = usage.fillSeries([{ start: new Date(at.getTime() - 86400000).toISOString(), value: 3 }],
        { from: new Date(at.getTime() - 2 * 86400000), end: new Date(at.getTime() + 3 * 86400000) }, 'DAY', at);
    out.badPeriod = await call(usage.getUsage, as('alice', { query: { period: 'FOREVER' } }));
    // The previous billing period is one interval back, whatever the current period's length.
    const previous = (start, end) => usage.periodRange('PREVIOUS', { current_period_start: start, current_period_end: end }).from.toISOString();
    out.previousPeriods = {
        after31Days: previous('2026-11-11T13:27:27.000Z', '2026-12-11T13:27:27.000Z'),
        afterFebruary: previous('2027-03-15T08:00:00.000Z', '2027-04-15T08:00:00.000Z'),
        intoFebruary: previous('2027-03-31T10:00:00.000Z', '2027-04-30T10:00:00.000Z'),
        fromShortMonth: previous('2027-02-28T10:00:00.000Z', '2027-03-31T10:00:00.000Z'),
        yearly: previous('2027-01-05T09:00:00.000Z', '2028-01-05T09:00:00.000Z'),
    };
    state.failMoesif = true;
    out.usageMoesifDown = await call(usage.getUsage, as('alice', { query: { period: 'LAST_30_DAYS' } }));
    state.failMoesif = false;
    out.bobUsage = await call(usage.getUsage, as('bob'));
    // Bob has no usage-based plan; asking for Alice's subscription finds nothing.
    out.bobAsksForAlice = await call(usage.getUsage, as('bob', { query: { subscriptionId: out.usage.body.subscriptionId } }));

    // The administrator's report: invoices of the last whole months, the portal's customers only.
    const cus = sql('SELECT customer_ref FROM billing_customers')[0].customer_ref;
    const now = new Date();
    const thisMonth = Math.floor(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1) / 1000) + 60;
    const lastMonth = Math.floor(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 10) / 1000);
    const monthBefore = Math.floor(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 2, 10) / 1000);
    const lines = (calls) => ({ data: [{ price: { id: 'price_calls' }, quantity: calls, amount: calls }] });
    // A checkout alice left after her card was declined: never completed, yet the payment
    // provider keeps an unpaid invoice for it.
    const paid = await billingCheckoutDao.getCheckout(started.body.id);
    const abandoned = await billingCheckoutDao.createCheckout({ orgId: paid.org_uuid, userUuid: paid.user_uuid,
        apiUuid: paid.api_uuid, planUuid: paid.plan_uuid });
    state.invoices = [
        { id: 'in_b', customer: cus, subscription: 'sub_paid', status: 'paid', currency: 'usd', total: 2812, created: thisMonth,
            customer_name: 'Alice Example', customer_email: 'alice@example.com', lines: lines(2812), number: 'INV-0002',
            invoice_pdf: 'https://pay.stripe.test/in_b.pdf', hosted_invoice_url: 'https://invoice.stripe.test/in_b',
            subscription_details: { metadata: { wso2_checkout: started.body.id } } },
        { id: 'in_a', customer: cus, subscription: 'sub_paid', status: 'paid', currency: 'usd', total: 1500, created: monthBefore, lines: lines(1500),
            hosted_invoice_url: 'javascript:alert(1)' },
        { id: 'in_draft', customer: cus, subscription: 'sub_paid', status: 'draft', currency: 'usd', total: 5000, created: lastMonth, lines: lines(5000) },
        { id: 'in_unpaid', customer: cus, subscription: 'sub_incomplete', status: 'open', currency: 'usd', total: 1900, created: lastMonth,
            lines: lines(1900), subscription_details: { metadata: { wso2_checkout: abandoned.uuid } } },
        { id: 'in_other', customer: 'cus_someone_else', subscription: 'sub_x', status: 'paid', currency: 'usd', total: 99999, created: lastMonth, lines: { data: [] } },
    ];
    // in_b has a second line past the ones Stripe embeds with the invoice.
    state.invoices[0].lines = { data: [{ id: 'il_1', price: { id: 'price_calls' }, quantity: 2800, amount: 2800 }], has_more: true };
    state.moreLines = { in_b: [{ id: 'il_2', price: { id: 'price_calls' }, quantity: 12, amount: 12 }] };
    out.revenue = await call(revenue.getRevenueReport, as('admin'));
    out.aliceSubscription = subId;
    const monthStart = (back) => new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - back, 1)).toISOString().slice(0, 10);
    calls.length = 0;
    out.revenueInvoices = await call(revenue.listRevenueInvoices, as('admin', { query: { from: monthStart(2) } }));
    out.revenueLineCalls = calls.filter((c) => c[0] === 'invoiceLines');
    out.revenueThisMonth = await call(revenue.listRevenueInvoices, as('admin'));
    out.revenueBeforeThisMonth = await call(revenue.listRevenueInvoices, as('admin', { query: { from: monthStart(2), to: monthStart(0) } }));
    // A page reads one provider invoice: pages of invoices the report leaves out are empty, with a cursor.
    out.revenuePages = [];
    let after;
    do {
        const page = await call(revenue.listRevenueInvoices, as('admin', { query: { from: monthStart(2), limit: '1',
            ...(after ? { after } : {}) } }));
        out.revenuePages.push({ status: page.status, numbers: (page.body.invoices || []).map((i) => i.number), next: page.body.nextCursor });
        after = page.body.nextCursor;
    } while (after && out.revenuePages.length < 10);
    out.revenueRefused = {
        badDate: await call(revenue.listRevenueInvoices, as('admin', { query: { from: '2026-13-01' } })),
        notADate: await call(revenue.listRevenueInvoices, as('admin', { query: { to: 'yesterday' } })),
        fromAfterTo: await call(revenue.listRevenueInvoices, as('admin', { query: { from: monthStart(0), to: monthStart(1) } })),
        badLimit: await call(revenue.listRevenueInvoices, as('admin', { query: { limit: '101' } })),
        badCursor: await call(revenue.listRevenueInvoices, as('admin', { query: { after: 'not a cursor' } })),
        unknownCursor: await call(revenue.listRevenueInvoices, as('admin', { query: { after: 'in_unknown' } })),
    };
    state.failInvoices = true;
    out.revenueStripeDown = await call(revenue.listRevenueInvoices, as('admin'));
    state.failInvoices = false;
    state.invoices[0].lines = lines(2812);
    delete state.moreLines;

    // The subscriber's Invoices page: invoices a page at a time, and the billing page.
    calls.length = 0;
    out.invoicePage1 = await call(account.listBillingInvoices, as('alice', { query: { period: 'ALL', limit: '2' } }));
    out.accountInvoiceQuery = (calls.find((c) => c[0] === 'invoicePage') || [])[1];
    out.invoicePage2 = await call(account.listBillingInvoices, as('alice',
        { query: { period: 'ALL', limit: '2', after: out.invoicePage1.body.nextCursor } }));
    // A cursor of the right shape that is not one of Stripe's invoices.
    out.invoiceUnknownCursor = await call(account.listBillingInvoices, as('alice',
        { query: { period: 'ALL', limit: '2', after: 'in_unknown' } }));
    out.invoicesFreeOnly = await call(account.listBillingInvoices, as('bob', { query: {} }));
    out.accountBadPeriod = await call(account.listBillingInvoices, as('alice', { query: { period: 'FOREVER' } }));
    out.accountBadLimit = await call(account.listBillingInvoices, as('alice', { query: { limit: '500' } }));
    // The unpaid invoice of a deleted checkout is left out of both lists.
    state.invoices.push({ id: 'in_gone', customer: cus, subscription: 'sub_gone', status: 'open', currency: 'usd', total: 1900,
        created: lastMonth, lines: lines(1900), subscription_details: { metadata: { wso2_checkout: '7c1e2d3f-0000-4000-8000-000000000000' } } });
    out.invoicesWithGone = (await call(account.listBillingInvoices, as('alice', { query: { period: 'ALL', limit: '10' } })))
        .body.invoices.map((i) => i.id);
    out.revenueWithGone = (await call(revenue.listRevenueInvoices, as('admin', { query: { from: monthStart(2) } })))
        .body.invoices.map((i) => i.total);
    state.invoices.pop();
    out.accountPage = await call(account.createAccountBillingPortalSession, as('alice', { body: { viewName: 'default' } }));
    out.portalConfigBody = (calls.find((c) => c[0] === 'portalConfig') || [])[1];
    out.accountPageArgs = (calls.find((c) => c[0] === 'portal') || [])[1];
    out.accountPageFreeOnly = await call(account.createAccountBillingPortalSession, as('bob', { body: {} }));

    // Stripe's retries end unpaid: access stops, and the subscriber sees one state for it.
    state.status = 'unpaid';
    await receive('customer.subscription.updated', 'sub_paid');
    out.unpaid = await call(billing.getSubscriptionBilling, as('alice', { params: { subId } }));

    // Unsubscribing cancels in Stripe first; while Stripe is down the subscription is kept.
    state.failCancel = true;
    out.deleteWhileDown = await call(subscriptions.deleteSubscription, as('alice', { params: { subId } }));
    out.keptRows = sql('SELECT COUNT(*) AS n FROM subscription_billing')[0].n;
    state.failCancel = false;
    out.deleted = await call(subscriptions.deleteSubscription, as('alice', { params: { subId } }));
    out.cancel = calls.find((c) => c[0] === 'cancel');
    out.rowsAfter = {
        subscriptions: sql('SELECT COUNT(*) AS n FROM subscriptions WHERE uuid = ?', subId)[0].n,
        billing: sql('SELECT COUNT(*) AS n FROM subscription_billing')[0].n,
    };
    out.deletedEvent = sql("SELECT COUNT(*) AS n FROM events WHERE type = 'subscription.deleted'")[0].n;
    // Stripe's own event for the cancellation arrives afterwards and finds nothing to do.
    out.lateEvent = await receive('customer.subscription.deleted', 'sub_paid');

    // A flat rate plan: carol buys silver, unsubscribes, and keeps it until the end of the paid period.
    const buyFlat = async (user, subRef) => {
        state.subs[subRef] = { status: 'active' };
        const begun = await call(checkout.createCheckoutSession, as(user, { body: { artifactId: 'weather', subscriptionPlanId: 'silver' } }));
        Object.assign(Object.values(state.sessions).pop(), { status: 'complete', payment_status: 'paid', subscription: subRef });
        return (await call(checkout.getCheckoutSession, as(user, { params: { checkoutId: begun.body.id } }))).body.subscriptionId;
    };
    const flatSub = await buyFlat('carol', 'sub_flat');
    calls.length = 0;
    out.flatUnsub = await call(subscriptions.deleteSubscription, as('carol', { params: { subId: flatSub } }));
    out.flatEnd = calls.filter((c) => c[0] === 'end');
    out.flatKept = sql('SELECT COUNT(*) AS n FROM subscriptions WHERE uuid = ?', flatSub)[0].n;
    out.flatBilling = await call(billing.getSubscriptionBilling, as('carol', { params: { subId: flatSub } }));
    out.flatUsage = await call(usage.getUsage, as('carol'));
    out.flatUsageNamed = await call(usage.getUsage, as('carol', { query: { subscriptionId: flatSub } }));
    out.flatUnsubAgain = await call(subscriptions.deleteSubscription, as('carol', { params: { subId: flatSub } }));
    out.flatEndCalls = calls.filter((c) => c[0] === 'end').length;
    out.flatBuyAgain = await call(checkout.createCheckoutSession, as('carol', { body: { artifactId: 'weather', subscriptionPlanId: 'silver' } }));
    out.flatSubscribeFree = await call(subscriptions.createSubscription, as('carol', { body: { artifactId: 'weather', subscriptionPlanId: 'free' } }));
    const deletedBefore = sql("SELECT COUNT(*) AS n FROM events WHERE type = 'subscription.deleted'")[0].n;
    // The period ends: Stripe ends it, as asked, and the portal removes it.
    state.subs.sub_flat = { ...state.subs.sub_flat, status: 'canceled' };
    await receive('customer.subscription.deleted', 'sub_flat');
    out.flatAfterEnd = {
        subscriptions: sql('SELECT COUNT(*) AS n FROM subscriptions WHERE uuid = ?', flatSub)[0].n,
        deletedEvents: sql("SELECT COUNT(*) AS n FROM events WHERE type = 'subscription.deleted'")[0].n - deletedBefore,
    };

    // Stripe ends dave's flat rate plan after failed payments: kept, with payment pending.
    const daveSub = await buyFlat('dave', 'sub_dave');
    state.subs.sub_dave = { status: 'canceled', cancellation_details: { reason: 'payment_failed' } };
    await receive('customer.subscription.deleted', 'sub_dave');
    out.daveKept = sql('SELECT COUNT(*) AS n FROM subscriptions WHERE uuid = ?', daveSub)[0].n;
    out.daveBilling = await call(billing.getSubscriptionBilling, as('dave', { params: { subId: daveSub } }));

    // grace sets her flat rate plan to end, then ends it immediately.
    const graceSub = await buyFlat('grace', 'sub_grace');
    await call(subscriptions.deleteSubscription, as('grace', { params: { subId: graceSub } }));
    const callsBefore = calls.length;
    out.graceImmediate = await call(subscriptions.deleteSubscription, as('grace', { params: { subId: graceSub },
        query: { immediate: 'true' } }));
    out.graceCancel = calls.slice(callsBefore).filter((c) => c[0] === 'cancel');
    out.graceKept = sql('SELECT COUNT(*) AS n FROM subscriptions WHERE uuid = ?', graceSub)[0].n;

    // Billing is removed (here: both subscriptions were bought before the current configuration).
    // Stripe already ended dave's, so unsubscribing removes it without Stripe; erin's is still
    // running, so it can't be cancelled.
    const erinSub = await buyFlat('erin', 'sub_erin');
    run("UPDATE subscription_billing SET created_at = '2000-01-01T00:00:00.000Z' WHERE subscription_uuid IN (?, ?)", daveSub, erinSub);
    out.revenueAfterEarlier = (await call(revenue.getRevenueReport, as('admin'))).body;
    const deletedBeforeGone = sql("SELECT COUNT(*) AS n FROM events WHERE type = 'subscription.deleted'")[0].n;
    calls.length = 0;
    out.daveUnsubGone = await call(subscriptions.deleteSubscription, as('dave', { params: { subId: daveSub } }));
    out.daveAfterGone = {
        subscriptions: sql('SELECT COUNT(*) AS n FROM subscriptions WHERE uuid = ?', daveSub)[0].n,
        billing: sql('SELECT COUNT(*) AS n FROM subscription_billing WHERE subscription_uuid = ?', daveSub)[0].n,
        deletedEvents: sql("SELECT COUNT(*) AS n FROM events WHERE type = 'subscription.deleted'")[0].n - deletedBeforeGone,
        stripeCalls: calls.length,
    };
    out.erinUnsubGone = await call(subscriptions.deleteSubscription, as('erin', { params: { subId: erinSub } }));
    out.erinKept = sql('SELECT COUNT(*) AS n FROM subscriptions WHERE uuid = ?', erinSub)[0].n;
    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

let _results;
function results() {
    if (_results) return _results;
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-subbilling-'));
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
        db.prepare("INSERT INTO views (uuid, org_uuid, portal_id, handle, display_name, created_by, updated_by) VALUES ('view-1', ?, ?, 'default', 'Default', 'test', 'test')").run(ORG, P);
        db.prepare('INSERT INTO api_metadata (uuid, org_uuid, portal_id, name, status, version, type, handle, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?,?,?,?)')
            .run('api-1', ORG, P, 'Weather API', 'PUBLISHED', 'v1.0', 'REST', 'weather', 'weather', 'test', 'test');
        for (const [uuid, handle] of [['plan-free', 'free'], ['plan-gold', 'gold'], ['plan-silver', 'silver']]) {
            db.prepare('INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?)')
                .run(uuid, ORG, P, handle, handle, handle, 'test', 'test');
            db.prepare('INSERT INTO api_subscription_plan_mappings (uuid, api_uuid, plan_uuid, portal_id, created_by) VALUES (?,?,?,?,?)')
                .run('map-' + handle, 'api-1', uuid, P, 'test');
        }
    `);
    const seeded = spawnSync(process.execPath, [seed], { cwd: PROJECT_ROOT, encoding: 'utf8' });
    assert.equal(seeded.status, 0, `seeding failed: ${seeded.stderr}`);
    const cfgFile = path.join(tmpDir, 'config.toml');
    fs.writeFileSync(cfgFile, CONFIG(dbFile));
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

test('a paid subscription shows its status, period and how it is charged, but not its charges or invoices', () => {
    const r = results().billing;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.status, 'ACTIVE');
    assert.equal(r.body.byUsage, true);
    assert.ok(r.body.currentPeriod && 'end' in r.body.currentPeriod);
    for (const gone of ['estimate', 'invoices', 'canManageBilling']) {
        assert.equal(gone in r.body, false, `${gone} is not returned here`);
    }
});

test('a user has one subscription per API: subscribing to a free plan again answers 409', () => {
    for (const key of ['freeAgain', 'freeWhilePaid']) {
        const r = results()[key];
        assert.equal(r.status, 409, key + ': ' + JSON.stringify(r.body));
        assert.equal(r.body.errors[0].message, 'A subscription for this API already exists', key);
    }
});

test('only the subscriber sees it, and a free subscription has no billing', () => {
    assert.equal(results().otherUser.status, 404);
    assert.equal(results().free.status, 201, JSON.stringify(results().free.body));
    assert.equal(results().freeBilling.status, 404);
    assert.equal(results().freeBilling.body.code, 'SUBSCRIPTION_NOT_PAID');
});

test('a paid subscription cannot be suspended', () => {
    assert.equal(results().suspend.status, 409);
});

test('usage is counted with the billing meter\'s filter, for this subscriber\'s ids, per day over the whole period', () => {
    const r = results().usage;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.subscriptions.length, 1, 'only paid subscriptions are listed');
    assert.equal(r.body.period, 'CURRENT');
    assert.equal(r.body.from, new Date(1760000000 * 1000).toISOString());
    assert.equal(r.body.to, new Date(1762600000 * 1000).toISOString(), 'the series runs to the end of the period');
    assert.deepEqual([r.body.usage.interval, r.body.usage.prices.length], ['DAY', 1],
        'one series per usage price of the plan');
    assert.deepEqual(r.body.usage.intervals, ['HOUR', 'DAY', 'WEEK'], 'a month can be shown per hour, day or week');
    const [u] = r.body.usage.prices;
    assert.deepEqual([u.name, u.unit, u.total], ['Successful calls · 200', 'request', 12]);
    // Every day of Oct 9 - Nov 8 (UTC), the counted ones and 0 for the others.
    assert.equal(u.buckets.length, 31);
    assert.equal(u.buckets[0].start, '2025-10-09T00:00:00.000Z');
    assert.deepEqual(u.buckets.slice(0, 4).map((b) => b.value), [0, 7, 5, 0]);
    assert.equal(r.body.meters, undefined, 'meters are not shown to subscribers');
    assert.equal(r.body.estimate.totalText, '$28.12');

    const { body } = results().search;
    const must = body.query.bool.must;
    assert.match(must[0].terms['metadata.billingCustomerId.raw'][0], /^cus_/);
    assert.deepEqual(must[1], { terms: { 'metadata.billingSubscriptionId.raw': ['sub_paid'] } });
    assert.deepEqual(must[2], { terms: { 'response.statusCode.raw': ['200'] } }, 'the meter\'s own key, matched as text');
    assert.equal(body.aggs.series.date_histogram.field, 'request.time');
    assert.equal(body.aggs.series.date_histogram.calendar_interval, '1d');
    assert.equal(body.size, 0);
});

test('the last 24 hours are shown per hour', () => {
    const r = results().usageHourly;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    const [u] = r.body.usage.prices;
    assert.deepEqual([r.body.usage.interval, u.buckets.length, u.total], ['HOUR', 24, 12]);
    const hour = new Date();
    hour.setUTCMinutes(0, 0, 0);
    assert.equal(u.buckets[23].start, hour.toISOString(), 'the last bucket is the current hour');
    assert.equal(results().hourlySearch.body.aggs.series.date_histogram.calendar_interval, '1h');
    assert.equal(r.body.estimate, null, 'charges are shown for the current period only');
});

test('the subscriber picks the bin: weeks start on Monday, and a bin the period does not offer is refused', () => {
    const w = results().usageWeekly;
    assert.equal(w.status, 200, JSON.stringify(w.body));
    assert.equal(w.body.usage.interval, 'WEEK');
    // Oct 9 - Nov 8, 2025 in weeks from Monday Oct 6; the two counted days fall in the first week.
    assert.deepEqual(w.body.usage.prices[0].buckets.map((b) => b.start.slice(0, 10)),
        ['2025-10-06', '2025-10-13', '2025-10-20', '2025-10-27', '2025-11-03']);
    assert.deepEqual(w.body.usage.prices[0].buckets.map((b) => b.value), [12, 0, 0, 0, 0]);
    assert.equal(results().weeklySearch.body.aggs.series.date_histogram.calendar_interval, '1w');
    const d = results().usage7.body.usage;
    assert.deepEqual([d.interval, d.prices[0].buckets.length, d.intervals], ['DAY', 7, ['HOUR', 'DAY']]);
    assert.equal(results().usageBinNotOffered.status, 400);
    assert.equal(results().usageBinUnknown.status, 400);
});

test('a period still running has its days to come in the series, with no value yet', () => {
    assert.deepEqual(results().filled.map((b) => b.value), [0, 3, 0, null, null, null]);
});

test('the previous billing period is one month (or year) back, not the current period\'s length', () => {
    assert.deepEqual(results().previousPeriods, {
        after31Days: '2026-10-11T13:27:27.000Z',
        afterFebruary: '2027-02-15T08:00:00.000Z',
        intoFebruary: '2027-02-28T10:00:00.000Z',
        fromShortMonth: '2027-01-31T10:00:00.000Z',
        yearly: '2026-01-05T09:00:00.000Z',
    });
});

test('the Usage page says when usage cannot be read, and rejects an unknown period', () => {
    assert.equal(results().badPeriod.status, 400);
    assert.equal(results().usageMoesifDown.status, 200);
    assert.equal(results().usageMoesifDown.body.usageAvailable, false);
    assert.equal(results().usageMoesifDown.body.estimate, null, 'charges are shown for the current period only');
    assert.deepEqual(results().bobUsage.body.subscriptions, []);
    assert.equal(results().bobAsksForAlice.status, 404, 'another user\'s subscription is not found, also for a user with no usage plan');
});

test('once Stripe\'s retries end unpaid, the subscription is payment pending', () => {
    assert.equal(results().unpaid.body.status, 'PAYMENT_PENDING');
    assert.equal(results().unpaid.body.billingStatus, 'UNPAID');
});

test('unsubscribing from a usage plan ends it now, with a final invoice, and keeps it while Stripe is down', () => {
    assert.equal(results().deleteWhileDown.status, 502);
    assert.equal(results().keptRows, 1);
    assert.equal(results().deleted.status, 200, JSON.stringify(results().deleted.body));
    assert.equal(results().deleted.body.endsAt, undefined);
    assert.deepEqual(results().cancel, ['cancel', 'sub_paid', { invoiceNow: true }]);
    assert.deepEqual(results().rowsAfter, { subscriptions: 0, billing: 0 });
    assert.equal(results().deletedEvent, 1);
    assert.equal(results().lateEvent, 'IGNORED');
});

test('the Usage page lists usage-based subscriptions only: a flat rate plan has no usage', () => {
    assert.equal(results().flatUsage.status, 200);
    assert.deepEqual(results().flatUsage.body.subscriptions, []);
    assert.equal(results().flatUsageNamed.status, 200, 'naming a flat subscription gives the same empty list');
    assert.deepEqual(results().flatUsageNamed.body.subscriptions, []);
    assert.ok(results().usage.body.subscriptions.length > 0, 'a usage-based subscription is listed');
});

test('unsubscribing from a flat rate plan runs to the end of the paid period', () => {
    const r = results().flatUnsub;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.endsAt, new Date(1762600000 * 1000).toISOString());
    assert.equal(r.body.message, 'Your subscription has been cancelled. You will retain access until Nov 8, 2025.');
    assert.deepEqual(results().flatEnd, [['end', 'sub_flat', { cancel_at_period_end: true }]]);
    assert.equal(results().flatKept, 1, 'the subscription is kept until it ends');
    assert.equal(results().flatBilling.body.endsAt, r.body.endsAt);
    assert.equal(results().flatBilling.body.byUsage, false);
    // Unsubscribing again answers the same, without asking Stripe again.
    assert.deepEqual(results().flatUnsubAgain.body, r.body);
    assert.equal(results().flatEndCalls, 1);
    // A new subscription to the API waits until this one ends.
    assert.equal(results().flatBuyAgain.status, 409);
    assert.match(JSON.stringify(results().flatBuyAgain.body), /ends on Nov 8, 2025\. You can subscribe again after that/);
    assert.equal(results().flatSubscribeFree.status, 409);
    assert.equal(results().flatSubscribeFree.body.errors[0].message,
        'Your subscription to this API ends on Nov 8, 2025. You can subscribe again after that.');
});

test('a flat rate plan set to end can be ended immediately: Stripe ends it at once, with no final invoice', () => {
    const r = results().graceImmediate;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.endsAt, undefined);
    assert.deepEqual(results().graceCancel, [['cancel', 'sub_grace', { invoiceNow: false }]]);
    assert.equal(results().graceKept, 0);
});

test('when Stripe ends a plan the subscriber left, the portal removes it; one ended unpaid is kept', () => {
    assert.deepEqual(results().flatAfterEnd, { subscriptions: 0, deletedEvents: 1 });
    assert.equal(results().daveKept, 1);
    assert.equal(results().daveBilling.body.status, 'PAYMENT_PENDING');
    assert.equal(results().daveBilling.body.billingStatus, 'CANCELLED');
});

test('once billing is removed, a subscription Stripe already ended can be unsubscribed; a running one cannot', () => {
    assert.equal(results().daveUnsubGone.status, 200, JSON.stringify(results().daveUnsubGone.body));
    assert.deepEqual(results().daveAfterGone, { subscriptions: 0, billing: 0, deletedEvents: 1, stripeCalls: 0 },
        'removed without asking Stripe, and the Platform API is told');
    assert.equal(results().erinUnsubGone.status, 409);
    assert.equal(results().erinUnsubGone.body.code, 'BILLING_NOT_CONFIGURED');
    assert.equal(results().erinKept, 1);
});

test('the revenue report gives the paid subscriptions from the portal\'s records, and when billing was set up', () => {
    const r = results().revenue;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.configured, true);
    assert.ok(!Number.isNaN(Date.parse(r.body.billingSince)));
    assert.equal(r.body.paidSubscriptions, 1);
    assert.equal(r.body.paidApis, 1);
    assert.deepEqual(r.body.paymentIssues, { pastDue: 0, paymentPending: 0 });
    assert.deepEqual(Object.keys(r.body).sort(), ['billingSince', 'configured', 'paidApis', 'paidSubscriptions', 'paymentIssues']);
});

test('the report\'s invoice list holds the portal\'s finalized invoices of the range, each with its API, plan and usage', () => {
    const r = results().revenueInvoices;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.invoicesAvailable, true);
    assert.equal(r.body.nextCursor, null);
    // Left out: the draft, the abandoned checkout's unpaid invoice, and another business's invoice.
    assert.deepEqual(r.body.invoices.map((i) => i.total), ['28.12', '15.00']);
    const [latest] = r.body.invoices;
    assert.deepEqual([latest.number, latest.currency, latest.paid, latest.api, latest.plan],
        ['INV-0002', 'USD', true, 'Weather API v1.0', 'gold']);
    assert.deepEqual(latest.subscriber, { name: 'Alice Example', email: 'alice@example.com' });
    assert.equal(latest.subscriptionId, results().aliceSubscription);
    // Both lines, the second read past the ones Stripe embeds with the invoice.
    assert.deepEqual(latest.items, [
        { name: 'Successful calls · 200', amount: '28.00', quantity: 2800, unit: 'request', countsRequests: true },
        { name: 'Successful calls · 200', amount: '0.12', quantity: 12, unit: 'request', countsRequests: true },
    ]);
    assert.deepEqual(results().revenueLineCalls, [['invoiceLines', 'in_b', 'il_1']]);
    assert.deepEqual(results().revenueThisMonth.body.invoices.map((i) => i.number), ['INV-0002'], 'from defaults to this month');
    assert.deepEqual(results().revenueBeforeThisMonth.body.invoices.map((i) => i.total), ['15.00'], 'to is not included');
});

test('the report\'s invoice list is read a page at a time, and a page may hold none of the portal\'s invoices', () => {
    const pages = results().revenuePages;
    assert.ok(pages.every((p) => p.status === 200));
    assert.equal(pages[pages.length - 1].next, null);
    assert.deepEqual(pages.flatMap((p) => p.numbers).length, 2);
    assert.ok(pages.some((p) => p.numbers.length === 0 && p.next), 'an empty page still points to the next');
});

test('the report\'s invoice list refuses a bad range, limit or cursor, and says when Stripe cannot be read', () => {
    const r = results().revenueRefused;
    for (const [name, res] of Object.entries(r)) assert.equal(res.status, 400, name);
    assert.match(r.badDate.body.message || r.badDate.body.description || JSON.stringify(r.badDate.body), /from and to must be dates/);
    assert.match(JSON.stringify(r.fromAfterTo.body), /from must be before to/);
    assert.match(JSON.stringify(r.unknownCursor.body), /after must be the nextCursor of a previous page/);
    const down = results().revenueStripeDown;
    assert.equal(down.status, 200);
    assert.deepEqual([down.body.invoicesAvailable, down.body.invoices, down.body.nextCursor], [false, [], null]);
});

test('Invoices lists only the subscriber\'s invoices, a page at a time, named by API and plan', () => {
    const page1 = results().invoicePage1;
    assert.equal(page1.status, 200, JSON.stringify(page1.body));
    assert.match(results().accountInvoiceQuery.customer, /^cus_/);
    assert.equal(page1.body.canManageBilling, true);
    assert.deepEqual(page1.body.invoices.map((i) => i.id), ['in_b', 'in_a']);
    assert.equal(page1.body.nextCursor, 'in_a');
    const paid = page1.body.invoices[0];
    assert.deepEqual([paid.number, paid.subscription, paid.amountText, paid.statusText, paid.pdfUrl],
        ['INV-0002', 'Weather API v1.0 · gold', '$28.12', 'Paid', 'https://pay.stripe.test/in_b.pdf']);
    assert.equal(paid.url, 'https://invoice.stripe.test/in_b');
    assert.equal(page1.body.invoices[1].url, null, 'only https invoice links are passed on');
    const page2 = results().invoicePage2;
    assert.deepEqual(page2.body.invoices.map((i) => i.id), ['in_draft']);
    assert.equal(page2.body.nextCursor, null);
    assert.ok(![...page1.body.invoices, ...page2.body.invoices].some((i) => i.id === 'in_other'),
        'another customer\'s invoice is not listed');
    assert.ok(![...page1.body.invoices, ...page2.body.invoices].some((i) => i.id === 'in_unpaid'),
        'the unpaid invoice of a checkout never completed is not listed');
    assert.deepEqual(results().invoicesFreeOnly.body.invoices, []);
    assert.equal(results().invoicesFreeOnly.body.canManageBilling, false, 'no billing page without a paid subscription');
    assert.equal(results().accountBadPeriod.status, 400);
    assert.equal(results().accountBadLimit.status, 400);
});

test('the unpaid invoice of a checkout the portal no longer has is left out of Invoices and the revenue report', () => {
    assert.deepEqual(results().invoicesWithGone, ['in_b', 'in_a', 'in_draft']);
    assert.deepEqual(results().revenueWithGone, ['28.12', '15.00']);
});

test('the revenue report counts only subscriptions bought under the current billing configuration', () => {
    const r = results().revenueAfterEarlier;
    assert.equal(r.paidSubscriptions, 0, 'dave\'s and erin\'s were bought under a removed configuration');
    assert.deepEqual(r.paymentIssues, { pastDue: 0, paymentPending: 0 });
});

test('an after cursor Stripe does not know is refused with 400', () => {
    const r = results().invoiceUnknownCursor;
    assert.equal(r.status, 400, JSON.stringify(r.body));
    assert.equal(r.body.message, 'after must be the nextCursor of a previous page');
});

test('Manage payment methods opens the payment provider\'s billing page, returning to Invoices', () => {
    assert.equal(results().accountPage.status, 201, JSON.stringify(results().accountPage.body));
    assert.equal(results().accountPage.body.url, 'https://billing.stripe.test/s_1');
    assert.match(results().accountPageArgs.returnUrl, /\/views\/default\/billing\/invoices$/);
    // Stripe's billing page opens with the portal's configuration: payment methods and invoices,
    // no cancelling, so subscriptions end through the portal only.
    assert.equal(results().accountPageArgs.configuration, 'bpc_1');
    const features = results().portalConfigBody.features;
    assert.equal(features.subscription_cancel.enabled, false);
    assert.equal(features.subscription_update.enabled, false);
    assert.equal(features.payment_method_update.enabled, true);
    assert.equal(features.invoice_history.enabled, true);
    assert.equal(results().accountPageFreeOnly.status, 409);
});
