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
 * End-to-end tests for paid plan checkout and Stripe event processing, against a real database
 * with Stripe replaced by an in-memory test client: a checkout creates the subscription exactly
 * once, whether the page's request after the payment or Stripe's event gets there first, and later
 * Stripe events move the subscription's billing status and tell the Platform API. A user has one
 * subscription per API: a second checkout start reuses or ends the open one, and a second paid
 * checkout is cancelled and refunded rather than activated.
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
const subscriptions = require(src + '/services/subscriptionService.js');
const billingConfigurationDao = require(src + '/dao/billingConfigurationDao.js');
const billingApiPlanDao = require(src + '/dao/billingApiPlanDao.js');
const billingCheckoutDao = require(src + '/dao/billingCheckoutDao.js');
const subDao = require(src + '/dao/subscriptionDao.js');
const planPricingDao = require(src + '/dao/planPricingDao.js');
const meterDao = require(src + '/dao/meterDao.js');
const { BillingProviderError } = require(src + '/billing/http.js');
const stripeConnector = require(src + '/billing/connectors/moesif/payments/stripe/index.js');
const moesifConnector = require(src + '/billing/connectors/moesif/index.js');
const Database = require(${JSON.stringify(SQLITE)});
const ORG = ${JSON.stringify(ORG)};
const out = {};

// ── Stripe test client ──
const stripeState = { sessions: {}, subscriptionStatus: 'active', failNextRetrieve: false, invoices: {},
    failNextRefund: null, refunded: new Set() };
const stripeCalls = [];
let seq = 0;
stripeConnector.clients.stripe = () => ({
    async createCustomer(args) { stripeCalls.push(['customer', args.metadata.wso2_portal_user]); return { id: 'cus_' + (++seq) }; },
    async createCheckoutSession(args) {
        const id = 'cs_' + (++seq);
        stripeState.sessions[id] = { id, client_secret: id + '_secret_' + seq, status: 'open', payment_status: 'unpaid',
            subscription: null, client_reference_id: args.clientReferenceId };
        stripeCalls.push(['session', args]);
        return { id, client_secret: id + '_secret_' + seq };
    },
    async retrieveCheckoutSession(id) { return { ...stripeState.sessions[id] }; },
    // Stripe expires open sessions only. One marked paysOnExpire is paid just before it can be; one
    // marked refusesExpire is refused for another reason and stays open.
    async expireCheckoutSession(id) {
        const session = stripeState.sessions[id];
        stripeCalls.push(['expire', id]);
        if (session.refusesExpire) throw new BillingProviderError('Stripe', 400, 'Stripe: This Checkout Session cannot be expired.');
        if (session.paysOnExpire) Object.assign(session, { status: 'complete', payment_status: 'paid', subscription: session.paysOnExpire });
        if (session.status !== 'open') {
            throw new BillingProviderError('Stripe', 400, 'Stripe: Only Checkout Sessions with a status in ["open"] can be expired.');
        }
        session.status = 'expired';
        return { ...session };
    },
    async cancelSubscription(id, opts) { stripeCalls.push(['cancel', id, opts || null]); return { id, status: 'canceled' }; },
    async listInvoices(subscription) { return stripeState.invoices[subscription] || []; },
    // A payment intent marked failNextRefund is refused once; one refunded already is refused as Stripe does.
    async createRefund(args) {
        stripeCalls.push(['refund', args.paymentIntent]);
        if (stripeState.failNextRefund === args.paymentIntent) {
            stripeState.failNextRefund = null;
            throw new BillingProviderError('Stripe', 503, 'Stripe: Service Unavailable');
        }
        if (stripeState.refunded.has(args.paymentIntent)) {
            throw new BillingProviderError('Stripe', 400, 'Stripe: Charge has already been refunded.', 'charge_already_refunded');
        }
        stripeState.refunded.add(args.paymentIntent);
        return { id: 're_' + (++seq) };
    },
    async retrieveSubscription(id) {
        if (stripeState.failNextRetrieve) {
            stripeState.failNextRetrieve = false;
            throw new BillingProviderError('Stripe', 503, 'Stripe: Service Unavailable');
        }
        return { id, status: stripeState.subscriptionStatus, current_period_start: 1760000000, current_period_end: 1762600000 };
    },
    async retrieveInvoice(id) { return { id, subscription: 'sub_paid' }; },
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
        Promise.resolve(fn(req, res)).catch((e) => resolve({ status: 'THREW', body: String(e && e.stack) }));
    });
}
const as = (user, extra) => Object.assign(
    { orgId: ORG, auth: { userId: user }, user: { email: user + '@example.com' }, baseUrl: '/api-portal',
      path: '/api/v0.9/checkout-sessions', query: {}, params: {}, body: {} },
    extra || {});
const sql = (q, ...p) => new Database(${JSON.stringify('DB_FILE')}).prepare(q).all(...p);
const run = (q, ...p) => new Database(${JSON.stringify('DB_FILE')}).prepare(q).run(...p);
const outbox = () => sql("SELECT type, payload FROM events WHERE type LIKE 'subscription.%' ORDER BY rowid")
    .map((e) => ({ type: e.type, payload: JSON.parse(e.payload) }));
// Delivers a payment event as the webhook receiver acts on it: its outcome, or the error that makes
// the provider deliver it again.
async function receive(type, objectId) {
    try {
        return await events.handle({ orgId: ORG, type, objectRef: objectId });
    } catch (err) {
        return { error: err.message };
    }
}

(async () => {
    // A paid plan that is ready to sell: pricing, a billing configuration and its API plan.
    const meter = await meterDao.create(ORG, { handle: 'calls', displayName: 'Calls', type: 'REQUEST_COUNT' }, 'admin');
    const { pricing } = await planPricingDao.create(ORG, 'plan-gold', { pricingType: 'USAGE_BASED', currency: 'USD',
        billingPeriod: 'MONTH',
        prices: [{ meter, filters: [], pricingModel: 'PER_UNIT', unitAmount: '0.01', unitQuantity: 1, tiers: null }] });
    await billingConfigurationDao.create(ORG, { billing_provider: 'MOESIF', payment_provider: 'STRIPE',
        settings: { stripe: { publishableKey: 'pk_test_demo1234' } },
        secrets: { moesif: { collectorApplicationId: 'app', managementKey: 'mk' },
            stripe: { apiKey: 'rk_test_abcd1234', webhookSecret: 'whsec_x' } } }, 'admin');
    await billingApiPlanDao.insertApiPlan({ orgId: ORG, apiUuid: 'api-1',
        planUuid: 'plan-gold', items: [{ priceUuid: pricing.prices[0].uuid, planRef: 'prod_1', priceRef: 'price_calls', meterRef: 'bm_1' }] });

    out.freePlan = await call(checkout.createCheckoutSession, as('alice', { body: { artifactId: 'weather', subscriptionPlanId: 'free' } }));
    out.started = await call(checkout.createCheckoutSession, as('alice', { body: { artifactId: 'weather', subscriptionPlanId: 'gold' } }));
    out.sessionArgs = stripeCalls.find((c) => c[0] === 'session')[1];
    const id = out.started.body.id;
    const session = stripeState.sessions[out.sessionArgs.clientReferenceId ? Object.keys(stripeState.sessions)[0] : null];

    out.whileOpen = await call(checkout.getCheckoutSession, as('alice', { params: { checkoutId: id } }));
    out.otherUser = await call(checkout.getCheckoutSession, as('bob', { params: { checkoutId: id } }));

    // Stripe confirms the payment. The subscriber returns first; Stripe's event then finds it done.
    Object.assign(session, { status: 'complete', payment_status: 'paid', subscription: 'sub_paid' });
    out.returned = await call(checkout.getCheckoutSession, as('alice', { params: { checkoutId: id } }));
    out.completedEvent = await receive('checkout.session.completed', session.id);
    out.subscriptionRows = sql('SELECT uuid, created_by FROM subscriptions');
    out.billingRow = sql('SELECT status, subscription_ref FROM subscription_billing')[0];
    out.subscribeAgain = await call(checkout.createCheckoutSession, as('alice', { body: { artifactId: 'weather', subscriptionPlanId: 'gold' } }));

    // Stripe's retries for a payment: past due (still served), then unpaid; a repeat changes nothing.
    stripeState.subscriptionStatus = 'past_due';
    await receive('invoice.payment_failed', 'in_1');
    stripeState.subscriptionStatus = 'unpaid';
    await receive('customer.subscription.updated', 'sub_paid');
    await receive('customer.subscription.updated', 'sub_paid');
    out.statusAfterRetries = sql('SELECT status FROM subscription_billing')[0].status;

    // A transient Stripe failure fails the delivery, so Stripe delivers the event again.
    stripeState.failNextRetrieve = true;
    out.failedEvent = await receive('invoice.paid', 'in_2');
    out.redeliveredEvent = await receive('invoice.paid', 'in_2');

    // Another subscriber abandons checkout; Stripe expires the session, and the checkout is deleted.
    const second = await call(checkout.createCheckoutSession, as('carol', { body: { artifactId: 'weather', subscriptionPlanId: 'gold' } }));
    const secondSession = Object.values(stripeState.sessions).find((s) => s.client_reference_id === second.body.id);
    secondSession.status = 'expired';
    out.expiredEvent = await receive('checkout.session.expired', secondSession.id);
    out.expired = await call(checkout.getCheckoutSession, as('carol', { params: { checkoutId: second.body.id } }));
    out.carolRows = sql("SELECT COUNT(*) AS n FROM billing_checkouts WHERE user_uuid = 'carol'")[0].n;
    // The page asks after a checkout Stripe expired before its event arrived: EXPIRED once, then not found.
    const third = await call(checkout.createCheckoutSession, as('cleo', { body: { artifactId: 'weather', subscriptionPlanId: 'gold' } }));
    Object.values(stripeState.sessions).find((s) => s.client_reference_id === third.body.id).status = 'expired';
    out.expiredOnRead = await call(checkout.getCheckoutSession, as('cleo', { params: { checkoutId: third.body.id } }));
    out.expiredReadAgain = await call(checkout.getCheckoutSession, as('cleo', { params: { checkoutId: third.body.id } }));

    out.outbox = outbox();

    // ── one subscription per user per API ──
    // A flat-fee plan of the same API, ready to sell too.
    const silver = (await planPricingDao.create(ORG, 'plan-silver', { pricingType: 'FLAT_RATE', currency: 'USD',
        billingPeriod: 'MONTH', flatAmount: '19.00' })).pricing;
    await billingApiPlanDao.insertApiPlan({ orgId: ORG, apiUuid: 'api-1', planUuid: 'plan-silver',
        items: [{ priceUuid: silver.flat_price_uuid, planRef: 'prod_2', priceRef: 'price_flat', meterRef: null }] });
    stripeState.subscriptionStatus = 'active';
    const start = (user, plan) => call(checkout.createCheckoutSession, as(user, { body: { artifactId: 'weather', subscriptionPlanId: plan } }));
    const sessionOf = (checkoutId) => Object.values(stripeState.sessions).find((s) => s.client_reference_id === checkoutId);
    const sessionCount = () => Object.keys(stripeState.sessions).length;
    // A checkout as stored, or null once deleted.
    const checkoutRow = (id) => {
        const row = sql('SELECT subscription_uuid FROM billing_checkouts WHERE uuid = ?', id)[0];
        return row ? { status: row.subscription_uuid ? 'COMPLETED' : 'OPEN', subscription_uuid: row.subscription_uuid } : null;
    };
    const subscriptionsOf = (user) => sql(\`SELECT s.uuid, p.handle FROM subscriptions s JOIN subscription_plans p ON p.uuid = s.plan_uuid
        WHERE s.created_by = ?\`, user);

    // Starting again for the same plan sends the browser to the same checkout.
    out.reuseFirst = await start('dan', 'gold');
    let before = sessionCount();
    out.reuseSecond = await start('dan', 'gold');
    out.reuseNewSessions = sessionCount() - before;

    // Starting for another plan ends the open checkout in Stripe first.
    out.switched = await start('dan', 'silver');
    out.switchExpired = stripeCalls.filter((c) => c[0] === 'expire').map((c) => c[1]);
    out.reuseFirstSession = sessionOf(out.reuseFirst.body.id).id;
    out.firstAfterSwitch = checkoutRow(out.reuseFirst.body.id);
    out.danOpen = sql("SELECT COUNT(*) AS n FROM billing_checkouts WHERE user_uuid = 'dan' AND subscription_uuid IS NULL")[0].n;

    // The open checkout was paid meanwhile: it is activated, and no new one starts.
    Object.assign(sessionOf(out.switched.body.id), { status: 'complete', payment_status: 'paid', subscription: 'sub_dan' });
    before = sessionCount();
    out.paidMeanwhile = await start('dan', 'gold');
    out.paidMeanwhileNewSessions = sessionCount() - before;
    out.switchedAfterPaid = checkoutRow(out.switched.body.id);
    out.danSubscriptions = subscriptionsOf('dan');

    // Paid just as it was being ended for another plan: Stripe will not expire it, so it is activated.
    out.erinFirst = await start('erin', 'gold');
    sessionOf(out.erinFirst.body.id).paysOnExpire = 'sub_erin';
    before = sessionCount();
    out.erinSwitch = await start('erin', 'silver');
    out.erinNewSessions = sessionCount() - before;
    out.erinFirstAfter = checkoutRow(out.erinFirst.body.id);
    out.erinSubscriptions = subscriptionsOf('erin');

    // Stripe refuses to end the open checkout and it stays open: no checkout for the other plan is
    // sent back, and none is started.
    out.ivyFirst = await start('ivy', 'gold');
    sessionOf(out.ivyFirst.body.id).refusesExpire = true;
    before = sessionCount();
    out.ivySwitch = await start('ivy', 'silver');
    out.ivyNewSessions = sessionCount() - before;
    out.ivyExpireCalls = stripeCalls.filter((c) => c[0] === 'expire' && c[1] === sessionOf(out.ivyFirst.body.id).id).length;
    out.ivyCheckouts = sql("SELECT uuid, plan_uuid, subscription_uuid FROM billing_checkouts WHERE user_uuid = 'ivy'");

    // Two starts at the same moment: the second does not see the first's checkout at first, finds it
    // under the lock on the user's customer row, and uses that one.
    out.raceFirst = await start('fay', 'gold');
    const findOpenCheckout = billingCheckoutDao.findOpenCheckout;
    let raced = false;
    billingCheckoutDao.findOpenCheckout = async (...args) => {
        if (raced) return findOpenCheckout(...args);
        raced = true;
        return null;
    };
    before = sessionCount();
    out.raceSecond = await start('fay', 'gold');
    billingCheckoutDao.findOpenCheckout = findOpenCheckout;
    out.raced = raced;
    out.raceNewSessions = sessionCount() - before;
    out.raceRows = sql("SELECT COUNT(*) AS n FROM billing_checkouts WHERE user_uuid = 'fay'")[0].n;

    // A checkout whose start has not reached Stripe: another start waits for it, then asks to try
    // again; one left like that for over two minutes is removed and a new one started.
    const pending = async (user) => {
        await billingCheckoutDao.createCustomer({ orgId: ORG, userUuid: user, customerRef: 'cus_' + user });
        return billingCheckoutDao.createCheckout({ orgId: ORG, userUuid: user, apiUuid: 'api-1', planUuid: 'plan-gold' });
    };
    const gilPending = await pending('gil');
    const halPending = await pending('hal');
    run('UPDATE billing_checkouts SET created_at = ? WHERE uuid = ?', new Date(Date.now() - 3 * 60 * 1000).toISOString(), halPending.uuid);
    [out.beingStarted, out.abandonedReplaced] = await Promise.all([start('gil', 'gold'), start('hal', 'gold')]);
    out.gilPendingAfter = checkoutRow(gilPending.uuid);
    out.halPendingAfter = checkoutRow(halPending.uuid) || null;
    out.halRows = sql("SELECT uuid, checkout_ref FROM billing_checkouts WHERE user_uuid = 'hal'");

    // Subscribing to a free plan while a checkout for a paid one is open ends the checkout in Stripe
    // first, so it can no longer be paid.
    const subscribeFree = (user) => call(subscriptions.createSubscription, as(user, { body: { artifactId: 'weather', subscriptionPlanId: 'free' } }));
    out.kimCheckout = await start('kim', 'gold');
    out.kimFree = await subscribeFree('kim');
    out.kimExpireCalls = stripeCalls.filter((c) => c[0] === 'expire' && c[1] === sessionOf(out.kimCheckout.body.id).id).length;
    out.kimSession = sessionOf(out.kimCheckout.body.id).status;
    out.kimCheckoutAfter = checkoutRow(out.kimCheckout.body.id);
    out.kimSubscriptions = subscriptionsOf('kim');

    // The open checkout was paid meanwhile: the paid subscription is activated, and the free one refused.
    out.leeCheckout = await start('lee', 'gold');
    Object.assign(sessionOf(out.leeCheckout.body.id), { status: 'complete', payment_status: 'paid', subscription: 'sub_lee' });
    out.leeFree = await subscribeFree('lee');
    out.leeCheckoutAfter = checkoutRow(out.leeCheckout.body.id);
    out.leeSubscriptions = subscriptionsOf('lee');

    // Stripe will not end the open checkout: the free subscription is refused, and the checkout left open.
    out.maxCheckout = await start('max', 'gold');
    sessionOf(out.maxCheckout.body.id).refusesExpire = true;
    out.maxFree = await subscribeFree('max');
    out.maxCheckoutAfter = checkoutRow(out.maxCheckout.body.id);
    out.maxSubscriptions = subscriptionsOf('max');

    // A free subscription made at the same moment as a checkout start, so it did not see the
    // checkout: paying the checkout creates no second subscription; Stripe's subscription is
    // cancelled immediately and its payment refunded.
    out.gusCheckout = await start('gus', 'gold');
    const findOpen = billingCheckoutDao.findOpenCheckout;
    billingCheckoutDao.findOpenCheckout = async () => null;
    out.gusFree = await subscribeFree('gus');
    billingCheckoutDao.findOpenCheckout = findOpen;
    stripeState.invoices.sub_gus = [{ id: 'in_g2', amount_paid: 1900, payment_intent: 'pi_g2' },
        { id: 'in_g1', amount_paid: 0, payment_intent: 'pi_g1' }];
    const gusSession = sessionOf(out.gusCheckout.body.id);
    Object.assign(gusSession, { status: 'complete', payment_status: 'paid', subscription: 'sub_gus' });
    const createdBefore = outbox().filter((e) => e.type === 'subscription.created').length;
    out.gusReturned = await call(checkout.getCheckoutSession, as('gus', { params: { checkoutId: out.gusCheckout.body.id } }));
    out.gusCompletedEvent = await receive('checkout.session.completed', gusSession.id);
    out.gusSubscriptions = subscriptionsOf('gus');
    out.gusBillingRows = sql("SELECT COUNT(*) AS n FROM subscription_billing WHERE subscription_ref = 'sub_gus'")[0].n;
    out.gusStripeCalls = stripeCalls.filter((c) => (c[0] === 'cancel' && c[1] === 'sub_gus') || c[0] === 'refund');
    out.gusCreatedEvents = outbox().filter((e) => e.type === 'subscription.created').length - createdBefore;

    // A checkout paid after its plan was deleted (here: the checkout names a plan that no longer
    // exists): no subscription can be made, so Stripe's is cancelled and the payment refunded.
    out.ivyCheckout = await start('ivy', 'gold');
    run("UPDATE billing_checkouts SET plan_uuid = 'plan-deleted' WHERE uuid = ?", out.ivyCheckout.body.id);
    stripeState.invoices.sub_ivy = [{ id: 'in_i1', amount_paid: 1900, payment_intent: 'pi_i1' }];
    const ivySession = sessionOf(out.ivyCheckout.body.id);
    Object.assign(ivySession, { status: 'complete', payment_status: 'paid', subscription: 'sub_ivy' });
    stripeCalls.length = 0;
    out.ivyReturned = await call(checkout.getCheckoutSession, as('ivy', { params: { checkoutId: out.ivyCheckout.body.id } }));
    out.ivySubscriptions = subscriptionsOf('ivy');
    out.ivyStripeCalls = stripeCalls.filter((c) => c[0] === 'cancel' || c[0] === 'refund');
    out.ivyReadAgain = await call(checkout.getCheckoutSession, as('ivy', { params: { checkoutId: out.ivyCheckout.body.id } }));

    // As for gus, but Stripe refuses one refund: the checkout is not completed, so the next attempt
    // refunds the rest and completes it, without refunding a payment twice.
    out.rexCheckout = await start('rex', 'gold');
    billingCheckoutDao.findOpenCheckout = async () => null;
    out.rexFree = await subscribeFree('rex');
    billingCheckoutDao.findOpenCheckout = findOpen;
    stripeState.invoices.sub_rex = [{ id: 'in_r2', amount_paid: 1900, payment_intent: 'pi_r2' },
        { id: 'in_r1', amount_paid: 500, payment_intent: 'pi_r1' }];
    const rexSession = sessionOf(out.rexCheckout.body.id);
    Object.assign(rexSession, { status: 'complete', payment_status: 'paid', subscription: 'sub_rex' });
    stripeState.failNextRefund = 'pi_r1';
    stripeCalls.length = 0;
    out.rexFailedEvent = await receive('checkout.session.completed', rexSession.id);
    out.rexCheckoutAfterFailure = checkoutRow(out.rexCheckout.body.id);
    out.rexReturned = await call(checkout.getCheckoutSession, as('rex', { params: { checkoutId: out.rexCheckout.body.id } }));
    out.rexSubscriptions = subscriptionsOf('rex');
    out.rexRefundCalls = stripeCalls.filter((c) => c[0] === 'refund').map((c) => c[1]);
    out.rexRefunded = ['pi_r2', 'pi_r1'].filter((p) => stripeState.refunded.has(p));

    // The user subscribes between the check before the activation and the activation itself: the
    // activation starts again, so the payment is refunded before the checkout is completed.
    out.samCheckout = await start('sam', 'gold');
    billingCheckoutDao.findOpenCheckout = async () => null;
    out.samFree = await subscribeFree('sam');
    billingCheckoutDao.findOpenCheckout = findOpen;
    stripeState.invoices.sub_sam = [{ id: 'in_s1', amount_paid: 1900, payment_intent: 'pi_s1' }];
    Object.assign(sessionOf(out.samCheckout.body.id), { status: 'complete', payment_status: 'paid', subscription: 'sub_sam' });
    const findForUser = subDao.findForUser;
    let samChecked = false;
    subDao.findForUser = async (...args) => {
        if (args.length < 4 && !samChecked) { samChecked = true; return null; }
        return findForUser(...args);
    };
    out.samReturned = await call(checkout.getCheckoutSession, as('sam', { params: { checkoutId: out.samCheckout.body.id } }));
    subDao.findForUser = findForUser;
    out.samSubscriptions = subscriptionsOf('sam');
    out.samRefunded = stripeState.refunded.has('pi_s1');

    // The other way round: the user's subscription ends between that check and the activation. The
    // payment is refunded already, so the checkout ends without a subscription.
    out.tiaCheckout = await start('tia', 'gold');
    stripeState.invoices.sub_tia = [{ id: 'in_t1', amount_paid: 1900, payment_intent: 'pi_t1' }];
    Object.assign(sessionOf(out.tiaCheckout.body.id), { status: 'complete', payment_status: 'paid', subscription: 'sub_tia' });
    subDao.findForUser = async (...args) => (args.length < 4 ? 'sub-ended' : findForUser(...args));
    out.tiaReturned = await call(checkout.getCheckoutSession, as('tia', { params: { checkoutId: out.tiaCheckout.body.id } }));
    subDao.findForUser = findForUser;
    out.tiaSubscriptions = subscriptionsOf('tia');
    out.tiaRefunded = stripeState.refunded.has('pi_t1');
    out.tiaReadAgain = await call(checkout.getCheckoutSession, as('tia', { params: { checkoutId: out.tiaCheckout.body.id } }));

    // One paid checkout activated by two calls at the same moment (the page and the event), both
    // holding it as still open, and by a third after the first has finished: the subscription the
    // first creates is the checkout's own, so nothing is cancelled or refunded.
    out.umaCheckout = await start('uma', 'gold');
    stripeState.invoices.sub_uma = [{ id: 'in_u1', amount_paid: 1900, payment_intent: 'pi_u1' }];
    Object.assign(sessionOf(out.umaCheckout.body.id), { status: 'complete', payment_status: 'paid', subscription: 'sub_uma' });
    const umaOpen = await billingCheckoutDao.getCheckout(out.umaCheckout.body.id);
    const umaPaid = { ref: umaOpen.checkout_ref, state: 'PAID', subscriptionRef: 'sub_uma' };
    stripeCalls.length = 0;
    const umaTogether = await Promise.all([checkout.activate(umaOpen, umaPaid), checkout.activate(umaOpen, umaPaid)]);
    const umaLater = await checkout.activate(umaOpen, umaPaid);
    out.umaStatuses = [...umaTogether, umaLater].map((c) => c.status);
    out.umaSubscriptions = subscriptionsOf('uma');
    out.umaStripeCalls = stripeCalls.filter((c) => c[0] === 'cancel' || c[0] === 'refund');

    // A plan taken off an API, or deleted: its open checkouts are ended in Stripe before the change.
    // The open checkouts so far are cleared first.
    const clearOpen = () => run('DELETE FROM billing_checkouts WHERE subscription_uuid IS NULL');
    clearOpen();
    // One still being started (no Stripe checkout yet) can't be ended: the change is refused.
    await billingCheckoutDao.createCheckout({ orgId: ORG, userUuid: 'kim', apiUuid: 'api-1', planUuid: 'plan-silver' });
    out.endBeingStarted = await checkout.endOpenCheckoutsFor(ORG, { planUuid: 'plan-silver' }).then(() => 'ended', (e) => [e.status, e.message]);
    clearOpen();
    out.niaStarted = await call(checkout.createCheckoutSession, as('nia', { body: { artifactId: 'weather', subscriptionPlanId: 'gold' } }));
    const niaSession = () => sessionOf(out.niaStarted.body.id);
    stripeCalls.length = 0;
    await checkout.endOpenCheckoutsFor(ORG, { apiUuid: 'api-1', planUuids: ['plan-silver'] });
    out.niaAfterOtherPlan = { kept: Boolean(await billingCheckoutDao.getCheckout(out.niaStarted.body.id)),
        expired: stripeCalls.filter((c) => c[0] === 'expire' && c[1] === niaSession().id).length };
    await checkout.endOpenCheckoutsFor(ORG, { planUuid: 'plan-gold' });
    out.niaAfterPlan = { kept: Boolean(await billingCheckoutDao.getCheckout(out.niaStarted.body.id)),
        session: niaSession().status, expired: stripeCalls.filter((c) => c[0] === 'expire' && c[1] === niaSession().id).length };

    // An account set to leave a subscription past due after the last retry: served while Stripe retries,
    // unpaid once Stripe has given up on the invoice (no next attempt), and served again once it is paid.
    const paidStatus = () => sql("SELECT status FROM subscription_billing WHERE subscription_ref = 'sub_paid'")[0].status;
    stripeState.subscriptionStatus = 'past_due';
    stripeState.invoices.sub_paid = [{ id: 'in_3', status: 'open', attempt_count: 1, next_payment_attempt: 1762000000 }];
    await receive('invoice.payment_failed', 'in_3');
    out.whileRetrying = paidStatus();
    stripeState.invoices.sub_paid = [{ id: 'in_3', status: 'open', attempt_count: 4, next_payment_attempt: null }];
    await receive('invoice.payment_failed', 'in_3');
    out.afterLastRetry = paidStatus();
    stripeState.subscriptionStatus = 'active';
    stripeState.invoices.sub_paid = [{ id: 'in_3', status: 'paid', attempt_count: 5, next_payment_attempt: null }];
    await receive('invoice.paid', 'in_3');
    out.afterPaying = paidStatus();
    // An invoice Stripe has not tried to charge yet has no attempt: past due, still served.
    stripeState.subscriptionStatus = 'past_due';
    stripeState.invoices.sub_paid = [{ id: 'in_4', status: 'open', attempt_count: 0, next_payment_attempt: null }];
    await receive('customer.subscription.updated', 'sub_paid');
    out.notYetCharged = paidStatus();

    // An MCP server sells the same plan: its checkout returns to the MCP server's page.
    await billingApiPlanDao.insertApiPlan({ orgId: ORG, apiUuid: 'mcp-1', planUuid: 'plan-gold', items: [{ priceUuid: pricing.prices[0].uuid, planRef: 'prod_2', priceRef: 'price_mcp_calls', meterRef: 'bm_2' }] });
    out.mcpStarted = await call(checkout.createCheckoutSession, as('mia', { body: { artifactId: 'files-mcp', subscriptionPlanId: 'gold' } }));
    out.mcpSessionArgs = stripeCalls.filter((c) => c[0] === 'session').pop()[1];
    process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(out) + '\\n');
    process.exit(0);
})();
`;

let _results;
function results() {
    if (_results) return _results;
    tmpDir = tmpDir || fs.mkdtempSync(path.join(os.tmpdir(), 'ap-checkout-'));
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
        db.prepare('INSERT INTO api_metadata (uuid, org_uuid, portal_id, name, status, version, type, handle, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?,?,?,?)')
            .run('mcp-1', ORG, P, 'Files MCP', 'PUBLISHED', 'v1.0', 'Mcp', 'files-mcp', 'files-mcp', 'test', 'test');
        for (const [uuid, handle] of [['plan-free', 'free'], ['plan-gold', 'gold'], ['plan-silver', 'silver']]) {
            db.prepare('INSERT INTO subscription_plans (uuid, org_uuid, portal_id, handle, display_name, ref_id, created_by, updated_by) VALUES (?,?,?,?,?,?,?,?)')
                .run(uuid, ORG, P, handle, handle, handle, 'test', 'test');
            db.prepare('INSERT INTO api_subscription_plan_mappings (uuid, api_uuid, plan_uuid, portal_id, created_by) VALUES (?,?,?,?,?)')
                .run('map-' + handle, 'api-1', uuid, P, 'test');
        }
        db.prepare('INSERT INTO api_subscription_plan_mappings (uuid, api_uuid, plan_uuid, portal_id, created_by) VALUES (?,?,?,?,?)')
            .run('map-mcp-gold', 'mcp-1', 'plan-gold', P, 'test');
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

test('a free plan is not checked out', () => {
    assert.equal(results().freePlan.status, 400);
    assert.equal(results().freePlan.body.code, 'FREE_PLAN');
});

test('checkout gives the browser what it mounts Stripe\'s form with, for the plan\'s price, and a portal-built return address', () => {
    const r = results().started;
    assert.equal(r.status, 201, JSON.stringify(r.body));
    assert.equal(r.headers.Location, '/api-portal/api/v0.9/checkout-sessions/' + r.body.id);
    assert.match(r.body.clientSecret, /^cs_\d+_secret_/);
    assert.equal(r.body.publishableKey, 'pk_test_demo1234');
    assert.equal(r.body.url, undefined, 'no Stripe page to send the browser to');
    const args = results().sessionArgs;
    assert.deepEqual(args.lineItems, [{ price: 'price_calls' }]);
    assert.equal(args.clientReferenceId, r.body.id);
    assert.equal(args.returnUrl,
        `https://developer.example.com/api-portal/default/views/default/api/weather?checkout=${r.body.id}&checkoutResult=success`);
});

test('a paid plan on an MCP server is bought the same way, and its checkout returns to the MCP server\'s page', () => {
    const r = results().mcpStarted;
    assert.equal(r.status, 201, JSON.stringify(r.body));
    const args = results().mcpSessionArgs;
    assert.deepEqual(args.lineItems, [{ price: 'price_mcp_calls' }]);
    assert.equal(args.returnUrl,
        `https://developer.example.com/api-portal/default/views/default/mcp/files-mcp?checkout=${r.body.id}&checkoutResult=success`);
});

test('a plan taken off an API, or deleted, has its open checkouts ended in Stripe first; other plans\' are left', () => {
    assert.equal(results().niaStarted.status, 201, JSON.stringify(results().niaStarted.body));
    assert.deepEqual(results().niaAfterOtherPlan, { kept: true, expired: 0 });
    assert.deepEqual(results().niaAfterPlan, { kept: false, session: 'expired', expired: 1 }, 'ended in Stripe, then deleted');
    assert.deepEqual(results().endBeingStarted, [409, 'A checkout for this API is being started. Try again in a moment.']);
});

test('no subscription exists while the payment is not confirmed, and others cannot see the checkout', () => {
    assert.equal(results().whileOpen.body.status, 'OPEN');
    assert.equal(results().whileOpen.body.subscriptionId, null);
    assert.equal(results().otherUser.status, 404);
});

test('a confirmed payment creates the subscription once, with its billing state', () => {
    const r = results().returned;
    assert.equal(r.body.status, 'COMPLETED');
    assert.ok(r.body.subscriptionId);
    assert.equal(results().completedEvent, 'PROCESSED');
    assert.equal(results().subscriptionRows.length, 1, 'Stripe\'s event after the return creates nothing more');
    assert.equal(results().subscriptionRows[0].created_by, 'alice');
    assert.deepEqual(results().billingRow, { status: 'ACTIVE', subscription_ref: 'sub_paid' });
    assert.equal(results().subscribeAgain.status, 409);

    const created = results().outbox.filter((e) => e.type === 'subscription.created');
    assert.equal(created.length, 1);
    assert.deepEqual(created[0].payload.billing, { status: 'ACTIVE', customer_id: created[0].payload.billing.customer_id, subscription_id: 'sub_paid' });
    assert.match(created[0].payload.billing.customer_id, /^cus_/);
});

test('Stripe\'s payment retries move the billing status, and only a change is sent on', () => {
    assert.equal(results().statusAfterRetries, 'UNPAID');
    const updates = results().outbox.filter((e) => e.type === 'subscription.billing_updated').map((e) => e.payload.billing.status);
    assert.deepEqual(updates, ['PAST_DUE', 'UNPAID']);
});

// The organization's Stripe account may leave a subscription past due after the last retry; access stops
// then all the same, regardless of that setting.
test('a past-due subscription is served while Stripe retries, unpaid once it gives up, and served again once paid', () => {
    assert.deepEqual([results().whileRetrying, results().afterLastRetry, results().afterPaying, results().notYetCharged],
        ['PAST_DUE', 'UNPAID', 'ACTIVE', 'PAST_DUE']);
});

test('a transient Stripe failure fails the delivery, and Stripe\'s next delivery is acted on', () => {
    assert.deepEqual(results().failedEvent, { error: 'Stripe: Service Unavailable' });
    assert.equal(results().redeliveredEvent, 'PROCESSED');
});

test('an abandoned checkout is deleted once Stripe expires it, without a subscription', () => {
    assert.equal(results().expiredEvent, 'PROCESSED');
    assert.equal(results().expired.status, 404, JSON.stringify(results().expired.body));
    assert.equal(results().carolRows, 0);
});

test('a checkout the page finds expired is answered EXPIRED once, as it is deleted, and is not found after', () => {
    const r = results().expiredOnRead;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.status, 'EXPIRED');
    assert.equal(r.body.subscriptionId, null);
    assert.equal(results().expiredReadAgain.status, 404);
});

test('starting a checkout again for the same plan gives back the same open checkout', () => {
    const first = results().reuseFirst;
    const second = results().reuseSecond;
    assert.equal(first.status, 201, JSON.stringify(first.body));
    assert.equal(second.status, 201, JSON.stringify(second.body));
    assert.deepEqual(second.body, { id: first.body.id, clientSecret: first.body.clientSecret, status: 'OPEN', publishableKey: 'pk_test_demo1234' });
    assert.equal(results().reuseNewSessions, 0);
});

test('starting a checkout for another plan of the API ends the open one in Stripe first', () => {
    const r = results().switched;
    assert.equal(r.status, 201, JSON.stringify(r.body));
    assert.notEqual(r.body.id, results().reuseFirst.body.id);
    assert.notEqual(r.body.clientSecret, results().reuseFirst.body.clientSecret);
    assert.deepEqual(results().switchExpired, [results().reuseFirstSession]);
    assert.equal(results().firstAfterSwitch, null, 'the ended checkout is deleted');
    assert.equal(results().danOpen, 1, 'one open checkout per user and API');
});

test('a checkout paid meanwhile is activated, and starting another answers 409', () => {
    const r = results().paidMeanwhile;
    assert.equal(r.status, 409, JSON.stringify(r.body));
    assert.equal(r.body.message, 'You already have a subscription to this API.');
    assert.equal(results().paidMeanwhileNewSessions, 0);
    assert.equal(results().switchedAfterPaid.status, 'COMPLETED');
    assert.deepEqual(results().danSubscriptions.map((s) => s.handle), ['silver']);
    assert.equal(results().switchedAfterPaid.subscription_uuid, results().danSubscriptions[0].uuid);
});

test('a checkout paid as it was being ended for another plan is activated, and starting another answers 409', () => {
    const r = results().erinSwitch;
    assert.equal(r.status, 409, JSON.stringify(r.body));
    assert.equal(r.body.message, 'You already have a subscription to this API.');
    assert.equal(results().erinNewSessions, 0);
    assert.equal(results().erinFirstAfter.status, 'COMPLETED');
    assert.deepEqual(results().erinSubscriptions.map((s) => s.handle), ['gold']);
});

test('an open checkout for another plan that Stripe will not end is not sent back: the start answers 409', () => {
    const r = results().ivySwitch;
    assert.equal(r.status, 409, JSON.stringify(r.body));
    assert.equal(r.body.message, 'A checkout for this API is being started. Try again in a moment.');
    assert.equal(r.body.id, undefined);
    assert.equal(r.body.clientSecret, undefined);
    assert.ok(!JSON.stringify(r.body).includes(results().ivyFirst.body.clientSecret), 'the gold checkout is not sent back');
    assert.equal(results().ivyExpireCalls, 1);
    assert.equal(results().ivyNewSessions, 0);
    assert.deepEqual(results().ivyCheckouts, [{ uuid: results().ivyFirst.body.id, plan_uuid: 'plan-gold', subscription_uuid: null }]);
});

test('a start that misses another start\'s open checkout finds it under the lock and uses that checkout', () => {
    assert.equal(results().raced, true, 'the second start did not see the open checkout');
    const r = results().raceSecond;
    assert.equal(r.status, 201, JSON.stringify(r.body));
    assert.deepEqual(r.body, { id: results().raceFirst.body.id, clientSecret: results().raceFirst.body.clientSecret, status: 'OPEN',
        publishableKey: 'pk_test_demo1234' });
    assert.equal(results().raceNewSessions, 0);
    assert.equal(results().raceRows, 1);
});

test('a start waits for a checkout not yet started with Stripe, and replaces one left like that for over two minutes', () => {
    const waiting = results().beingStarted;
    assert.equal(waiting.status, 409, JSON.stringify(waiting.body));
    assert.equal(waiting.body.message, 'A checkout for this API is being started. Try again in a moment.');
    assert.equal(results().gilPendingAfter.status, 'OPEN', 'the other start\'s checkout is left alone');
    const replaced = results().abandonedReplaced;
    assert.equal(replaced.status, 201, JSON.stringify(replaced.body));
    assert.equal(results().halPendingAfter, null, 'the abandoned checkout is removed');
    assert.equal(results().halRows.length, 1);
    assert.equal(results().halRows[0].uuid, replaced.body.id);
    assert.match(results().halRows[0].checkout_ref, /^cs_/);
});

test('subscribing to a free plan ends the open checkout in Stripe first, so it can no longer be paid', () => {
    const r = results().kimFree;
    assert.equal(r.status, 201, JSON.stringify(r.body));
    assert.equal(results().kimExpireCalls, 1);
    assert.equal(results().kimSession, 'expired');
    assert.equal(results().kimCheckoutAfter, null, 'the ended checkout is deleted');
    assert.deepEqual(results().kimSubscriptions.map((s) => s.handle), ['free']);
});

test('subscribing to a free plan after the open checkout was paid activates the paid plan and answers 409', () => {
    const r = results().leeFree;
    assert.equal(r.status, 409, JSON.stringify(r.body));
    assert.equal(r.body.message, 'You already have a subscription to this API.');
    assert.equal(results().leeCheckoutAfter.status, 'COMPLETED');
    assert.deepEqual(results().leeSubscriptions.map((s) => s.handle), ['gold']);
    assert.equal(results().leeCheckoutAfter.subscription_uuid, results().leeSubscriptions[0].uuid);
});

test('subscribing to a free plan answers 409 when Stripe will not end the open checkout, and leaves it open', () => {
    const r = results().maxFree;
    assert.equal(r.status, 409, JSON.stringify(r.body));
    assert.equal(r.body.message, 'Your open checkout for this API could not be ended. Try again in a moment.');
    assert.equal(results().maxCheckoutAfter.status, 'OPEN');
    assert.deepEqual(results().maxSubscriptions, []);
});

test('paying a checkout of a user who already subscribed to the API creates no second subscription; Stripe\'s is cancelled and refunded', () => {
    assert.equal(results().gusCheckout.status, 201, JSON.stringify(results().gusCheckout.body));
    assert.equal(results().gusFree.status, 201, JSON.stringify(results().gusFree.body));
    const r = results().gusReturned;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.status, 'COMPLETED');
    assert.equal(r.body.subscriptionId, results().gusFree.body.subscriptionId, 'the checkout points at the subscription the user has');
    assert.equal(results().gusCompletedEvent, 'PROCESSED');
    assert.deepEqual(results().gusSubscriptions.map((s) => s.handle), ['free']);
    assert.equal(results().gusBillingRows, 0);
    assert.equal(results().gusCreatedEvents, 0);
    // Cancelled without a final invoice, and only the paid invoice refunded; Stripe's later event changes nothing.
    assert.deepEqual(results().gusStripeCalls, [['cancel', 'sub_gus', null], ['refund', 'pi_g2']]);
});

test('a checkout paid after its plan was deleted ends without a subscription, and the payment is refunded', () => {
    const r = results().ivyReturned;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.status, 'EXPIRED');
    assert.deepEqual(results().ivySubscriptions, []);
    assert.deepEqual(results().ivyStripeCalls, [['cancel', 'sub_ivy', null], ['refund', 'pi_i1']]);
    assert.equal(results().ivyReadAgain.status, 404, 'the checkout is deleted');
});

test('a second paid checkout is completed only once its payment is refunded; a refused refund is tried again, and no payment is refunded twice', () => {
    assert.deepEqual(results().rexFailedEvent, { error: 'Stripe: Service Unavailable' }, 'the event fails, so Stripe delivers it again');
    assert.equal(results().rexCheckoutAfterFailure.status, 'OPEN');
    const r = results().rexReturned;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.status, 'COMPLETED');
    assert.equal(r.body.subscriptionId, results().rexFree.body.subscriptionId);
    assert.deepEqual(results().rexSubscriptions.map((s) => s.handle), ['free']);
    assert.deepEqual(results().rexRefundCalls, ['pi_r2', 'pi_r1', 'pi_r2', 'pi_r1']);
    assert.deepEqual(results().rexRefunded, ['pi_r2', 'pi_r1']);
});

test('a subscription made while a paid checkout is activated is kept, and the checkout\'s payment refunded', () => {
    const r = results().samReturned;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.status, 'COMPLETED');
    assert.equal(r.body.subscriptionId, results().samFree.body.subscriptionId);
    assert.deepEqual(results().samSubscriptions.map((s) => s.handle), ['free']);
    assert.equal(results().samRefunded, true);
});

test('a paid checkout whose payment was refunded for a subscription that has since ended ends without a subscription', () => {
    const r = results().tiaReturned;
    assert.equal(r.status, 200, JSON.stringify(r.body));
    assert.equal(r.body.status, 'EXPIRED');
    assert.deepEqual(results().tiaSubscriptions, []);
    assert.equal(results().tiaRefunded, true);
    assert.equal(results().tiaReadAgain.status, 404, 'the checkout is deleted');
});

test('a paid checkout activated by more than one call at once creates one subscription and refunds nothing', () => {
    assert.deepEqual(results().umaStatuses, ['COMPLETED', 'COMPLETED', 'COMPLETED']);
    assert.deepEqual(results().umaSubscriptions.map((s) => s.handle), ['gold']);
    assert.deepEqual(results().umaStripeCalls, []);
});
