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
 * A subscriber's paid subscription: its access status (GET /subscriptions/{subId}/billing), and
 * ending it with the payment provider when the subscriber unsubscribes. Also the parts the Usage,
 * Invoices and revenue pages share: price names, estimates and invoices ready to show, and the
 * error for a failed provider call.
 *
 * The provider is called through the billing provider interface, with the organization's billing
 * configuration, and only while that is the configuration the subscription was bought under. A
 * subscription bought under an earlier configuration was made with keys deleted when billing was
 * removed, so the portal can no longer reach it.
 */

const subDao = require('../../dao/subscriptionDao');
const planPricingDao = require('../../dao/planPricingDao');
const billingConfigurationDao = require('../../dao/billingConfigurationDao');
const billingApiPlanDao = require('../../dao/billingApiPlanDao');
const billingSubscriptionDao = require('../../dao/billingSubscriptionDao');
const billingProvider = require('../../billing/billingProvider');
const { BillingProviderError } = require('../../billing/http');
const { formatAmount } = require('../../utils/pricingLabels');
const util = require('../../utils/util');
const logger = require('../../config/logger');

/**
 * What the subscriber sees. PAYMENT_PENDING is the only state in which the gateway refuses calls
 * (403): the payment provider stopped retrying the payment, or cancelled the subscription because
 * it was not paid. PAST_DUE is still served while the provider retries.
 */
const ACCESS_STATUS = Object.freeze({
    ACTIVE: 'ACTIVE',
    PAST_DUE: 'PAST_DUE',
    PAYMENT_PENDING: 'PAYMENT_PENDING',
});

// The portal's billing statuses (see billing/billingProvider.js).
const BILLING_STATUS = Object.freeze({ PAST_DUE: 'PAST_DUE', UNPAID: 'UNPAID', CANCELLED: 'CANCELLED' });

const FEE_NAMES = { MONTH: 'Monthly fee', YEAR: 'Yearly fee' };

const NOT_PAID_CODE = 'SUBSCRIPTION_NOT_PAID';
const NOT_CONFIGURED_CODE = 'BILLING_NOT_CONFIGURED';
const PROVIDER_CODE = 'BILLING_PROVIDER_UNAVAILABLE';

class SubscriptionBillingError extends Error {
    constructor(status, message, code) {
        super(message);
        this.status = status;
        this.code = code;
    }
}

function accessStatus(billingStatus) {
    if (billingStatus === BILLING_STATUS.UNPAID || billingStatus === BILLING_STATUS.CANCELLED) {
        return ACCESS_STATUS.PAYMENT_PENDING;
    }
    if (billingStatus === BILLING_STATUS.PAST_DUE) return ACCESS_STATUS.PAST_DUE;
    return ACCESS_STATUS.ACTIVE;
}

/** The billing configuration the subscription was bought under, with its keys, or null when it is gone. */
async function configurationFor(billing) {
    const conn = await billingConfigurationDao.getWithSecrets(billing.org_uuid);
    if (!conn || conn.uuid !== billing.configuration_uuid) {
        return null;
    }
    return conn;
}

/** Amounts in the currency's minor unit: 9900 in USD -> "99.00"; 500 in JPY -> "500". */
function fromMinor(amount, currency) {
    let digits = 2;
    try {
        digits = new Intl.NumberFormat('en-US', { style: 'currency', currency }).resolvedOptions().maximumFractionDigits;
    } catch (_err) {
        // An unknown currency keeps two decimals.
    }
    return (Number(amount || 0) / 10 ** digits).toFixed(digits);
}

function money(amount, currency) {
    const upper = String(currency || '').toUpperCase();
    const value = fromMinor(amount, upper);
    return { amount: value, text: formatAmount(value, upper) };
}

function iso(date) {
    return date instanceof Date ? date.toISOString() : null;
}

function safeUrl(value) {
    return typeof value === 'string' && /^https:\/\//.test(value) ? value : null;
}

/**
 * The names of the subscription's prices, by price reference: "Monthly fee" for a flat rate plan, and
 * each usage price's name, made from its meter's name and its values ("API calls · GET"). The
 * provider's own line descriptions name the catalog plan, as invoices need, so they are shown only
 * for a line whose price is not known here.
 */
async function priceNames(orgId, billing, pricing) {
    const names = new Map();
    if (!pricing) return names;
    const items = await billingApiPlanDao.listItems(billing.api_plan_uuid);
    const prices = new Map((pricing.prices || []).map((p) => [p.uuid, p]));
    for (const item of items) {
        if (item.price_uuid === null) {
            names.set(item.price_ref, { name: FEE_NAMES[pricing.billing_period] || 'Plan fee' });
            continue;
        }
        const price = prices.get(item.price_uuid);
        if (price) names.set(item.price_ref, { name: price.name, usage: true });
    }
    return names;
}

/** The current period's estimate (payments.estimate), with lines named as the plan page names them. */
function estimateDto(estimate, names) {
    const { currency } = estimate;
    const lines = estimate.lines.map((line) => {
        const known = line.priceRef ? names.get(line.priceRef) : null;
        const total = money(line.amountMinor, currency);
        return {
            description: known ? known.name : line.description,
            usage: Boolean(known && known.usage),
            quantity: line.quantity,
            amount: total.amount,
            amountText: total.text,
        };
    });
    const total = money(estimate.totalMinor, currency);
    return {
        currency,
        periodStart: iso(estimate.periodStart),
        periodEnd: iso(estimate.periodEnd),
        lines,
        total: total.amount,
        totalText: total.text,
    };
}

const INVOICE_STATUS_LABELS = { paid: 'Paid', open: 'Unpaid', void: 'Void', uncollectible: 'Unpaid', draft: 'Draft' };

function invoiceDto(invoice) {
    const total = money(invoice.totalMinor, invoice.currency);
    return {
        id: invoice.id,
        date: iso(invoice.created),
        amount: total.amount,
        amountText: total.text,
        status: invoice.status,
        statusText: INVOICE_STATUS_LABELS[invoice.status] || invoice.status,
        number: invoice.number || null,
        url: safeUrl(invoice.url),
        pdfUrl: safeUrl(invoice.pdfUrl),
    };
}

/**
 * The error a subscriber gets when a payment provider call fails. A rejected key (401 or 403) needs
 * the administrator to fix billing; any other failure may clear, so the subscriber is asked to try
 * again later.
 */
function providerFailure(err, what) {
    if (err.status === 401 || err.status === 403) {
        return new SubscriptionBillingError(502, `${what} Billing is not set up correctly; contact the portal `
            + 'administrator.', PROVIDER_CODE);
    }
    return new SubscriptionBillingError(502, `${what} Try again later.`, PROVIDER_CODE);
}

/** The caller's paid subscription, or a SubscriptionBillingError (404) when it is not one. */
async function loadPaidSubscription(req) {
    const sub = await subDao.get(req.orgId, req.params.subId, util.resolveActor(req));
    if (!sub) throw new SubscriptionBillingError(404, 'Subscription not found');
    const billing = await billingSubscriptionDao.getBySubscription(sub.uuid);
    if (!billing) throw new SubscriptionBillingError(404, 'This subscription is not on a paid plan.', NOT_PAID_CODE);
    return { sub, billing };
}

function sendBillingError(res, error, fallback) {
    if (error instanceof SubscriptionBillingError) {
        return util.sendError(res, error.status, error.message, error.code ? { code: error.code } : {});
    }
    logger.error(fallback, { error: error.message });
    return util.handleError(res, error);
}

/** GET /subscriptions/{subId}/billing */
const getSubscriptionBilling = async (req, res) => {
    try {
        const { sub, billing } = await loadPaidSubscription(req);
        const pricing = sub.subscription_plan
            ? await planPricingDao.getByPlan(req.orgId, sub.subscription_plan.uuid) : null;
        const body = {
            subscriptionId: sub.uuid,
            status: accessStatus(billing.billing_status),
            billingStatus: billing.billing_status,
            currentPeriod: {
                start: billing.current_period_start ? new Date(billing.current_period_start).toISOString() : null,
                end: billing.current_period_end ? new Date(billing.current_period_end).toISOString() : null,
            },
            // When the subscription ends, after the subscriber unsubscribed from a flat rate plan.
            endsAt: billing.ends_at ? new Date(billing.ends_at).toISOString() : null,
            // Charged by usage: unsubscribing ends it now; a flat rate plan runs to the end of the period.
            byUsage: Boolean(pricing && pricing.pricing_type === 'USAGE_BASED'),
        };
        // Charges, estimates and invoices are on the Usage and Invoices pages (GET /usage, GET /billing/invoices).
        return res.status(200).json(body);
    } catch (error) {
        return sendBillingError(res, error, 'Error reading subscription billing');
    }
};

/**
 * Ends a paid subscription with the payment provider when the subscriber unsubscribes, and says
 * what the portal does with it: { removeNow, endsAt }.
 *
 *   - A plan charged by usage only ends now (removeNow), with a final invoice for the usage
 *     reported so far.
 *   - A flat rate plan runs to the end of the period already paid for, when the provider ends
 *     it (endsAt); the portal keeps the subscription until then. Its usage prices keep counting
 *     until then too.
 *   - A subscription the provider has already ended, or no longer has, is removed now.
 *
 * Throws a SubscriptionBillingError when the provider cannot be reached, so the subscription is
 * kept rather than left billing with no way to stop it.
 */
async function endInProvider(billing) {
    const conn = await configurationFor(billing);
    if (!conn) {
        // Billing was removed, which is allowed only once every paid subscription has ended in the
        // provider: nothing is left to cancel, so the subscription is removed now. One still running
        // could only be cancelled with the deleted keys, so it is refused.
        if (billing.billing_status === BILLING_STATUS.CANCELLED) return { removeNow: true, endsAt: null };
        throw new SubscriptionBillingError(409, 'This paid subscription cannot be cancelled while billing is '
            + 'not set up. Contact the portal administrator.', NOT_CONFIGURED_CODE);
    }
    // Charged by usage when no item is a flat rate plan's price.
    const items = await billingApiPlanDao.listItems(billing.api_plan_uuid);
    const byUsage = items.length > 0 && items.every((item) => item.price_uuid !== null);
    const { payments } = billingProvider.forConfiguration(conn);
    try {
        if (byUsage) {
            await payments.cancelSubscription(billing.billing_subscription_id);
            return { removeNow: true, endsAt: null };
        }
        const endsAt = await payments.endAtPeriodEnd(billing.billing_subscription_id);
        return { removeNow: !endsAt, endsAt };
    } catch (err) {
        if (!(err instanceof BillingProviderError)) throw err;
        logger.warn('The payment provider could not end a paid subscription', {
            subscriptionId: billing.subscription_uuid, error: err.message,
        });
        throw providerFailure(err, 'The subscription could not be cancelled with the payment provider.');
    }
}

module.exports = {
    ACCESS_STATUS,
    FEE_NAMES,
    NOT_CONFIGURED_CODE,
    SubscriptionBillingError,
    accessStatus,
    configurationFor,
    invoiceDto,
    safeUrl,
    providerFailure,
    sendBillingError,
    priceNames,
    estimateDto,
    fromMinor,
    endInProvider,
    getSubscriptionBilling,
};
