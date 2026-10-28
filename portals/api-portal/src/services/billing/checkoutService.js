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
 * Checkout for paid plans. How a subscriber buys one:
 *   1. The subscriber starts a checkout. The portal creates their customer with the payment
 *      provider (once per user) and a checkout for the plan's prices (with Stripe, a Checkout
 *      Session, which creates the Stripe subscription when it is paid). The API page shows it in a
 *      dialog (Stripe Embedded Checkout).
 *   2. Once the provider confirms the payment (or that none was due), the checkout is activated:
 *      the portal subscription is created with its billing state, exactly once. Whichever comes
 *      first activates it: the page asking for the checkout (GET /checkout-sessions/{id}, which asks
 *      the provider), or the provider's checkout event.
 * No subscription exists before the payment is confirmed.
 *
 * A user has at most one subscription and one open checkout per API (checked in code, under a lock
 * on the user's customer row):
 *   - Starting a checkout for the same plan again shows the open one.
 *   - Starting one for another plan of the API, or subscribing to a free plan of it, ends the open
 *     checkout first; if that checkout was paid meanwhile, the request is refused (409).
 *   - If a second checkout is paid anyway, no second subscription is created: its provider
 *     subscription is cancelled and the payment refunded.
 */

const db = require('../../db/driver');
const apiDao = require('../../dao/apiDao');
const subDao = require('../../dao/subscriptionDao');
const orgDao = require('../../dao/organizationDao');
const viewDao = require('../../dao/viewDao');
const subscriptionPlanDao = require('../../dao/subscriptionPlanDao');
const planPricingDao = require('../../dao/planPricingDao');
const billingConfigurationDao = require('../../dao/billingConfigurationDao');
const billingApiPlanDao = require('../../dao/billingApiPlanDao');
const billingCheckoutDao = require('../../dao/billingCheckoutDao');
const billingSubscriptionDao = require('../../dao/billingSubscriptionDao');
const userIdpReferenceDao = require('../../dao/userIdpReferenceDao');
const billingVisibility = require('./billingVisibility');
const subscriptionService = require('../subscriptionService');
const billingProvider = require('../../billing/billingProvider');
const { BillingProviderError } = require('../../billing/http');
const orgContext = require('../../utils/orgContext');
const constants = require('../../utils/constants');
const util = require('../../utils/util');
const logger = require('../../config/logger');
const { logUserAction } = require('../../middlewares/auditLogger');

const { STATUS } = billingCheckoutDao;
// The status of a checkout that ended unpaid.
const EXPIRED = 'EXPIRED';

// How long a checkout start waits for another request starting the same user's checkout for the
// API, and when such a start counts as abandoned (the request died before reaching the provider).
const START_WAIT_MS = 5000;
const START_ABANDONED_MS = 2 * 60 * 1000;
const ALREADY_SUBSCRIBED = 'You already have a subscription to this API.';
const BEING_STARTED = 'A checkout for this API is being started. Try again in a moment.';
const CHECKOUT_NOT_ENDED = 'Your open checkout for this API could not be ended. Try again in a moment.';
const CHECKOUTS_NOT_ENDED = 'A subscriber\'s open checkout could not be ended in Stripe, so nothing was changed. '
    + 'Try again in a few minutes.';

class CheckoutError extends Error {
    constructor(status, message, code) {
        super(message);
        this.status = status;
        this.code = code;
    }
}

/** The public origin the payment provider returns the browser to: the portal's configured base URL. */
function portalOrigin() {
    const { config } = require('../../config/configLoader');
    return String((config.server && config.server.baseUrl) || '').replace(/\/+$/, '');
}

/** Whether the organization has the view; viewDao.getId throws a 404 for an unknown one. */
async function viewExists(orgId, viewName) {
    try {
        return Boolean(await viewDao.getId(orgId, viewName));
    } catch (err) {
        if (err && (err.statusCode === 404 || err.status === 404)) return false;
        throw err;
    }
}

/**
 * The address of a page of a view (`page` is the path after the view, such as "/api/weather").
 * Built from configuration and stored identifiers, never from the request, so the payment provider
 * cannot be made to send the browser anywhere else; an unknown view falls back to the default one.
 */
async function viewPageUrl(orgId, viewName, page) {
    const org = await orgDao.getByUuid(orgId);
    const orgHandle = org ? org.handle : orgContext.getHandle();
    let view = await orgContext.getFallbackViewHandle(orgId);
    if (typeof viewName === 'string' && viewName && await viewExists(orgId, viewName)) view = viewName;
    return `${portalOrigin()}${constants.ROUTE.BASE_PATH}/${encodeURIComponent(orgHandle)}/views/`
        + `${encodeURIComponent(view)}${page}`;
}

/**
 * The API or MCP server page a payment method that leaves the page returns the subscriber to. Checkout
 * itself runs in a dialog on that page, so most payments never use it.
 */
async function returnUrl(orgId, viewName, api, checkoutUuid) {
    const page = await viewPageUrl(orgId, viewName, `/${util.artifactPageSection(api.type)}/${encodeURIComponent(api.handle)}`);
    return `${page}?checkout=${encodeURIComponent(checkoutUuid)}&checkoutResult=success`;
}

/** The user's customer with the payment provider, created on first use. */
async function ensureCustomer(req, orgId, payments, userUuid) {
    const existing = await billingCheckoutDao.findCustomer(orgId, userUuid);
    if (existing) return existing;
    const email = req.user && typeof req.user.email === 'string' && /^[^@\s]+@[^@\s]+$/.test(req.user.email)
        ? req.user.email : undefined;
    const name = await userIdpReferenceDao.resolveDisplay(userUuid);
    const customerRef = await payments.createCustomer({
        name: typeof name === 'string' ? name.slice(0, 255) : undefined,
        email,
        metadata: { wso2_portal_user: userUuid, wso2_organization: orgId },
    });
    return billingCheckoutDao.createCustomer({ orgId, userUuid, customerRef });
}

/**
 * The prices a subscription to the API plan has, one per item: a flat rate price fixed
 * (charged once per period), each usage price metered.
 */
async function checkoutPrices(apiPlan) {
    const items = apiPlan ? apiPlan.items : [];
    if (items.length === 0) throw new CheckoutError(409, 'This plan is not ready to be sold yet.', 'PLAN_NOT_AVAILABLE');
    // In the plan's price order (planPricingDao), as the plan page lists them; a flat rate plan has one.
    const pricing = await planPricingDao.getByPlan(apiPlan.org_uuid, apiPlan.plan_uuid);
    const position = new Map(((pricing && pricing.prices) || []).map((price, i) => [price.uuid, i]));
    const at = (item) => (position.has(item.price_uuid) ? position.get(item.price_uuid) : -1);
    const flatPrice = pricing ? pricing.flat_price_uuid : null;
    return [...items].sort((x, y) => at(x) - at(y))
        .map((item) => ({ ref: item.price_ref, fixed: item.price_uuid === flatPrice }));
}

/**
 * Waits for a checkout's start to reach the payment provider, or for the checkout to end. Resolves to
 * { checkout } (null when it was deleted), or to null when it is still being started.
 */
async function waitForStart(checkoutUuid) {
    const end = Date.now() + START_WAIT_MS;
    while (Date.now() < end) {
        const checkout = await billingCheckoutDao.getCheckout(checkoutUuid);
        if (!checkout || checkout.checkout_ref || checkout.status !== STATUS.OPEN) return { checkout };
        await new Promise((resolve) => { setTimeout(resolve, 500); });
    }
    return null;
}

/**
 * The user's open checkout for the API once its start has reached the payment provider, or null when
 * there is none (a start that never reached the provider within two minutes is removed). Throws a
 * 409 while another request is still starting it.
 */
async function startedOpenCheckout(orgId, userUuid, apiUuid) {
    const open = await billingCheckoutDao.findOpenCheckout(orgId, userUuid, apiUuid);
    if (!open || open.checkout_ref) return open;
    const startResult = await waitForStart(open.uuid);
    if (!startResult) {
        if (Date.now() - new Date(open.created_at).getTime() < START_ABANDONED_MS) {
            throw new CheckoutError(409, BEING_STARTED, 'CONFLICT');
        }
        await billingCheckoutDao.deleteOpenCheckout(open.uuid);
        return null;
    }
    if (!startResult.checkout || startResult.checkout.status !== STATUS.OPEN) return startedOpenCheckout(orgId, userUuid, apiUuid);
    return startResult.checkout;
}

/**
 * An open checkout once its start has finished: itself; null when its start was abandoned (it is
 * removed) or it ended meanwhile. Throws a 409 while it is still being started.
 */
async function startedCheckout(open) {
    if (open.checkout_ref) return open;
    const startResult = await waitForStart(open.uuid);
    if (!startResult) {
        if (Date.now() - new Date(open.created_at).getTime() < START_ABANDONED_MS) {
            throw new CheckoutError(409, BEING_STARTED, 'CONFLICT');
        }
        await billingCheckoutDao.deleteOpenCheckout(open.uuid);
        return null;
    }
    return startResult.checkout && startResult.checkout.status === STATUS.OPEN ? startResult.checkout : null;
}

/**
 * Ends with the payment provider the open checkouts of an API (`apiUuid`, or only those of its plans
 * `planUuids`) or of a plan (`planUuid`), so a deleted API or plan, or a plan taken off an API, can
 * no longer be bought. Run before the change is saved. A checkout paid meanwhile is activated as
 * usual, so the change then finds its subscription and is refused. Throws a CheckoutError (409) when the provider
 * would not end one or one is still being started, and the provider's error when it fails.
 */
async function endOpenCheckoutsFor(orgId, { apiUuid, planUuid, planUuids } = {}) {
    const open = (await billingCheckoutDao.listOpenFor(orgId, { apiUuid, planUuid }))
        .filter((c) => !planUuids || planUuids.includes(c.plan_uuid));
    if (open.length === 0) return;
    const conn = await billingConfigurationDao.getWithSecrets(orgId);
    for (const candidate of open) {
        const checkout = await startedCheckout(candidate);
        if (!checkout) continue;
        if (!conn) {
            // Billing is gone: nothing can activate the checkout any more.
            await billingCheckoutDao.deleteOpenCheckout(checkout.uuid);
            continue;
        }
        const { payments } = billingProvider.forConfiguration(conn);
        let provider = await payments.getCheckout(checkout.checkout_ref);
        if (provider.state === 'OPEN') provider = await payments.expireCheckout(checkout.checkout_ref);
        if (provider.state === 'OPEN') throw new CheckoutError(409, CHECKOUTS_NOT_ENDED, 'CONFLICT');
        await settle(checkout, provider);
    }
}

/**
 * The user's open checkout for the API, settled against the payment provider. Resolves to an open
 * checkout for the same plan for the browser to mount again ({ id, clientSecret, status }), or null when a new
 * checkout can start (there was none, it had ended, or it was for another plan and has been ended).
 * Throws a 409 when the open checkout turns out to be paid.
 */
async function reusableCheckout(orgId, userUuid, apiUuid, planUuid, payments) {
    const open = await startedOpenCheckout(orgId, userUuid, apiUuid);
    if (!open) return null;
    let provider = await payments.getCheckout(open.checkout_ref);
    const otherPlan = open.plan_uuid !== planUuid;
    if (provider.state === 'OPEN' && (otherPlan || !provider.clientSecret)) {
        provider = await payments.expireCheckout(open.checkout_ref);
        // Never show the browser a checkout for another plan: if it could not be ended, refuse.
        if (provider.state === 'OPEN') throw new CheckoutError(409, BEING_STARTED, 'CONFLICT');
    }
    if (provider.state === 'OPEN') return { id: open.uuid, clientSecret: provider.clientSecret, status: STATUS.OPEN };
    const settled = await settle(open, provider);
    if (settled.status === STATUS.COMPLETED) throw new CheckoutError(409, ALREADY_SUBSCRIBED, 'CONFLICT');
    return null;
}

/**
 * Ends the user's open checkout for the API before they subscribe to a free plan of it, so the
 * checkout can no longer be paid. Throws a 409 when it was paid meanwhile (the paid subscription is
 * activated instead) or the payment provider would not end it.
 */
async function endOpenCheckout(orgId, userUuid, apiUuid) {
    const open = await startedOpenCheckout(orgId, userUuid, apiUuid);
    if (!open) return;
    const conn = await billingConfigurationDao.getWithSecrets(orgId);
    if (!conn) {
        // Billing is gone: nothing can activate the checkout any more.
        await billingCheckoutDao.deleteOpenCheckout(open.uuid);
        return;
    }
    const { payments } = billingProvider.forConfiguration(conn);
    let provider = await payments.getCheckout(open.checkout_ref);
    if (provider.state === 'OPEN') provider = await payments.expireCheckout(open.checkout_ref);
    if (provider.state === 'OPEN') throw new CheckoutError(409, CHECKOUT_NOT_ENDED, 'CONFLICT');
    const settled = await settle(open, provider);
    if (settled.status === STATUS.COMPLETED) throw new CheckoutError(409, ALREADY_SUBSCRIBED, 'CONFLICT');
}

/** Starts a checkout: POST /checkout-sessions { artifactId, subscriptionPlanId, viewName? }. */
async function startCheckout(req) {
    const orgId = req.orgId;
    const userUuid = util.resolveActor(req);
    if (!userUuid || userUuid === constants.SYSTEM_ACTOR) {
        throw new CheckoutError(403, 'A paid plan is bought by a signed-in user.', 'FORBIDDEN');
    }
    const { artifactId: apiHandle, subscriptionPlanId: planHandle, viewName } = req.body || {};
    if (typeof apiHandle !== 'string' || !apiHandle || typeof planHandle !== 'string' || !planHandle) {
        throw new CheckoutError(400, 'artifactId and subscriptionPlanId are required');
    }
    const apiUuid = await apiDao.getId(orgId, apiHandle);
    const [api] = apiUuid ? await apiDao.get(orgId, apiUuid) : [];
    if (!api) throw new CheckoutError(404, 'API not found');
    const plan = (api.subscription_plans || []).find((p) => p.handle === planHandle);
    if (!plan) throw new CheckoutError(400, 'Subscription plan not found for this API');
    const pricing = await planPricingDao.getByPlan(orgId, plan.uuid);
    if (!pricing) {
        throw new CheckoutError(400, 'This plan is free: subscribe to it directly.', 'FREE_PLAN');
    }
    if (!(await billingVisibility.canSellPlan(orgId, apiUuid, plan.uuid))) {
        throw new CheckoutError(409, 'This plan cannot be bought right now.', 'PLAN_NOT_AVAILABLE');
    }
    const current = await subDao.list(orgId, { apiId: apiUuid, createdBy: userUuid });
    if (current.length > 0) {
        // A plan that runs to the end of its paid period still counts until then.
        const billing = await billingSubscriptionDao.getBySubscription(current[0].uuid);
        throw new CheckoutError(409, billing && billing.ends_at
            ? `Your subscription to this API ends on ${new Date(billing.ends_at).toLocaleDateString('en-US',
                { year: 'numeric', month: 'short', day: 'numeric', timeZone: 'UTC' })}. You can subscribe again after that.`
            : ALREADY_SUBSCRIBED, 'CONFLICT');
    }

    const conn = await billingConfigurationDao.getWithSecrets(orgId);
    const apiPlan = await billingApiPlanDao.getApiPlan(orgId, apiUuid, plan.uuid);
    const prices = await checkoutPrices(apiPlan);
    const { payments } = billingProvider.forConfiguration(conn);
    const customer = await ensureCustomer(req, orgId, payments, userUuid);

    // The browser mounts the checkout with the payment provider's public key and the checkout's client secret.
    const { publishableKey } = payments;
    const open = await reusableCheckout(orgId, userUuid, apiUuid, plan.uuid, payments);
    if (open) {
        logUserAction('CHECKOUT_RESUMED', req, { orgId, resourceUuid: open.id, resourceType: 'billing_checkout' });
        return { ...open, publishableKey };
    }
    // One open checkout per user and API: the check and the insert run under a lock on the user's
    // customer row, so two starts at the same moment take turns and the second finds the first's.
    const checkout = await db.withTransaction(async (t) => {
        await billingCheckoutDao.lockCustomer(orgId, userUuid, t);
        if (await billingCheckoutDao.findOpenCheckout(orgId, userUuid, apiUuid, t)) return null;
        return billingCheckoutDao.createCheckout({ orgId, userUuid, apiUuid, planUuid: plan.uuid }, t);
    });
    if (!checkout) {
        // Another request started the user's checkout for this API at the same moment: use that one.
        const again = await reusableCheckout(orgId, userUuid, apiUuid, plan.uuid, payments);
        if (again) return { ...again, publishableKey };
        throw new CheckoutError(409, BEING_STARTED, 'CONFLICT');
    }
    let started;
    try {
        started = await payments.startCheckout({
            customerRef: customer.customer_ref,
            prices,
            reference: checkout.uuid,
            returnUrl: await returnUrl(orgId, viewName, api, checkout.uuid),
        });
    } catch (err) {
        // The checkout never reached the payment provider: remove it, so nothing half-started is
        // left. The customer stays: it is the user's, and the next attempt reuses it.
        await billingCheckoutDao.deleteOpenCheckout(checkout.uuid);
        throw err;
    }
    await billingCheckoutDao.setCheckoutRef(checkout.uuid, started.ref);
    logUserAction('CHECKOUT_STARTED', req, { orgId, resourceUuid: checkout.uuid, resourceType: 'billing_checkout' });
    return { id: checkout.uuid, status: STATUS.OPEN, clientSecret: started.clientSecret, publishableKey };
}

/**
 * Creates the subscription of a checkout the payment provider has confirmed, with its billing
 * state. `providerCheckout` is the checkout as the provider reports it (payments.getCheckout).
 * Safe to call more than once and from more than one place: only the first call creates anything.
 * Resolves to the checkout as it ends up.
 */
async function activate(checkout, providerCheckout) {
    if (checkout.status !== STATUS.OPEN) return checkout;
    if (!providerCheckout || providerCheckout.ref !== checkout.checkout_ref) {
        throw new Error('The provider\'s checkout does not belong to this checkout.');
    }
    const conn = await billingConfigurationDao.getWithSecrets(checkout.org_uuid);
    const customer = await billingCheckoutDao.findCustomer(checkout.org_uuid, checkout.user_uuid);
    if (!conn || !customer) {
        throw new Error('Billing was removed since the checkout started.');
    }
    const providerSub = await billingProvider.forConfiguration(conn).payments.getSubscription(providerCheckout.subscriptionRef);
    if (!providerSub) throw new Error('The payment provider has no subscription for the completed checkout.');
    const billingStatus = providerSub.status;

    const [api] = await apiDao.get(checkout.org_uuid, checkout.api_uuid);
    const plan = await subscriptionPlanDao.getByUuid(checkout.org_uuid, checkout.plan_uuid);
    if (!api || !plan) {
        // Paid for an API or plan deleted after the checkout started: no subscription can be made, so
        // the provider's subscription is ended and its payment refunded, as for a second paid checkout,
        // and the checkout ends without one. A provider refusal is thrown, so the event is retried.
        const refunded = await billingProvider.forConfiguration(conn).payments.cancelAndRefund(providerSub.ref);
        await billingCheckoutDao.deleteOpenCheckout(checkout.uuid);
        logger.warn('Paid checkout for an API or plan that no longer exists: its subscription was cancelled and refunded', {
            checkoutId: checkout.uuid, apiGone: !api, planGone: !plan, refunded,
        });
        return { ...checkout, status: EXPIRED };
    }
    const apiPlan = await billingApiPlanDao.getApiPlan(checkout.org_uuid, checkout.api_uuid, checkout.plan_uuid);
    if (!apiPlan) throw new Error('The paid plan of the checkout has no billing API plan.');

    // One subscription per user per API: a second paid checkout does not create another. Its provider
    // subscription bills for nothing the user does not already have, so it is ended and its payment
    // refunded before the checkout is completed; a provider refusal is thrown, so the event is retried.
    const kept = await subDao.findForUser(checkout.org_uuid, checkout.api_uuid, checkout.user_uuid);
    if (kept) {
        // Read after it: a subscription this checkout created (another call activated it meanwhile) is
        // committed with the checkout's completion, so it is never taken for a second one.
        const current = await billingCheckoutDao.getCheckout(checkout.uuid);
        if (!current) return { ...checkout, status: EXPIRED };
        if (current.status !== STATUS.OPEN) return current;
        const refunded = await billingProvider.forConfiguration(conn).payments.cancelAndRefund(providerSub.ref);
        logger.warn('Second paid checkout for an API: its subscription was cancelled and refunded', {
            checkoutId: checkout.uuid, keptSubscriptionId: kept, refunded,
        });
    }
    try {
        await db.withTransaction(async (t) => {
            const existing = await subDao.findForUser(checkout.org_uuid, checkout.api_uuid, checkout.user_uuid, t);
            if (Boolean(existing) !== Boolean(kept)) {
                // The user subscribed or unsubscribed since the check above.
                throw Object.assign(new Error('subscription changed'), { subscriptionChanged: true });
            }
            if (existing) {
                // The checkout points at the subscription the user keeps.
                if (!(await billingCheckoutDao.markCompleted(checkout.uuid, existing, t))) {
                    throw Object.assign(new Error('already activated'), { alreadyActivated: true });
                }
                return;
            }
            const sub = await subscriptionService.createSubscriptionRecord({
                orgId: checkout.org_uuid,
                apiId: checkout.api_uuid,
                apiMetadata: api,
                plan,
                createdBy: checkout.user_uuid,
                billing: { status: billingStatus, customer_id: customer.customer_ref, subscription_id: providerSub.ref },
            }, t);
            if (!(await billingCheckoutDao.markCompleted(checkout.uuid, sub.uuid, t))) {
                // Activated by another caller in the meantime: undo this attempt.
                throw Object.assign(new Error('already activated'), { alreadyActivated: true });
            }
            await billingSubscriptionDao.create({
                subscriptionUuid: sub.uuid,
                orgId: checkout.org_uuid,
                subscriptionRef: providerSub.ref,
                status: billingStatus,
                currentPeriodStart: providerSub.periodStart,
                currentPeriodEnd: providerSub.periodEnd,
            }, t);
        });
        if (!kept) logger.info('Paid subscription activated', { checkoutId: checkout.uuid });
    } catch (err) {
        if (err.alreadyActivated) return billingCheckoutDao.getCheckout(checkout.uuid);
        // Subscribed meanwhile: activate again, so the payment is refunded before the checkout completes.
        if (err.subscriptionChanged && !kept) return activate(checkout, providerCheckout);
        if (err.subscriptionChanged) {
            // Unsubscribed meanwhile: the payment is already refunded, so the checkout ends without a subscription.
            await billingCheckoutDao.deleteOpenCheckout(checkout.uuid);
            return { ...checkout, status: EXPIRED };
        }
        throw err;
    }
    return billingCheckoutDao.getCheckout(checkout.uuid);
}

/**
 * Acts on a checkout as the payment provider reports it now: activates it, deletes it when it ended
 * unpaid (resolving to it with status EXPIRED), or leaves it open.
 */
async function settle(checkout, providerCheckout) {
    if (providerCheckout.state === 'PAID') return activate(checkout, providerCheckout);
    if (providerCheckout.state === 'EXPIRED') {
        await billingCheckoutDao.deleteOpenCheckout(checkout.uuid);
        return { ...checkout, status: EXPIRED };
    }
    return checkout;
}

/** Asks the payment provider for a checkout's state and settles it. */
async function confirm(checkout) {
    if (checkout.status !== STATUS.OPEN || !checkout.checkout_ref) return checkout;
    const conn = await billingConfigurationDao.getWithSecrets(checkout.org_uuid);
    if (!conn) return checkout;
    const providerCheckout = await billingProvider.forConfiguration(conn).payments.getCheckout(checkout.checkout_ref);
    return settle(checkout, providerCheckout);
}

async function toDto(checkout) {
    const api = await apiDao.get(checkout.org_uuid, checkout.api_uuid);
    const plan = await subscriptionPlanDao.getByUuid(checkout.org_uuid, checkout.plan_uuid);
    return {
        id: checkout.uuid,
        status: checkout.status,
        artifactId: api[0] ? api[0].handle : null,
        subscriptionPlanId: plan ? plan.handle : null,
        subscriptionId: checkout.subscription_uuid || null,
    };
}

function sendCheckoutError(res, error, fallback) {
    if (error instanceof CheckoutError) return util.sendError(res, error.status, error.message, { code: error.code });
    if (error instanceof BillingProviderError) {
        logger.warn(fallback, { error: error.message });
        return util.sendError(res, 502, 'The payment provider could not be reached. Try again in a moment.',
            { code: 'BILLING_PROVIDER_UNAVAILABLE' });
    }
    logger.error(fallback, { error: error.message });
    return util.sendError(res, 500, fallback);
}

const createCheckoutSession = async (req, res) => {
    try {
        const checkout = await startCheckout(req);
        res.setHeader('Location', `${req.baseUrl}${req.path.replace(/\/$/, '')}/${encodeURIComponent(checkout.id)}`);
        return res.status(201).json(checkout);
    } catch (error) {
        return sendCheckoutError(res, error, 'Error while starting checkout');
    }
};

/**
 * GET /checkout-sessions/{checkoutId}: the checkout's state, confirmed with the payment provider while it is
 * open, so the page asking after the payment (in the dialog, or back from a payment method that left the
 * page) activates the subscription without waiting for the provider's event.
 */
const getCheckoutSession = async (req, res) => {
    try {
        const checkout = await billingCheckoutDao.getCheckout(req.params.checkoutId);
        if (!checkout || checkout.org_uuid !== req.orgId || checkout.user_uuid !== util.resolveActor(req)) {
            return util.sendError(res, 404, 'Checkout not found');
        }
        return res.status(200).json(await toDto(await confirm(checkout)));
    } catch (error) {
        return sendCheckoutError(res, error, 'Error while reading checkout');
    }
};

module.exports = {
    viewPageUrl,
    activate,
    settle,
    confirm,
    endOpenCheckout,
    endOpenCheckoutsFor,
    CheckoutError,
    sendCheckoutError,
    createCheckoutSession,
    getCheckoutSession,
};
