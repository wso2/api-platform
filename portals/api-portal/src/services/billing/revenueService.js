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
 * Settings › Usage & Revenue: what the portal's paid plans have invoiced, month by month and per
 * plan, with the billable usage on those invoices.
 *
 * The report comes from two calls:
 *   - GET /billing-reports/revenue: what the portal's own records hold: the paid subscriptions, the
 *     APIs they are on, their payment issues, and when billing was set up.
 *   - GET /billing-reports/invoices: the portal's invoices of a date range, a page at a time, each
 *     with the API and plan it bills, its amounts and its usage. The page adds them up.
 * The payment provider has no call for totals and returns at most 100 invoices per call, so a
 * report over many invoices is read in pieces rather than in one request that could take minutes.
 *
 * An invoice counts as the portal's when:
 *   - its customer was created by the portal for the current billing configuration;
 *   - it is finalized (open, paid or uncollectible): drafts may still change, and void ones were
 *     never owed;
 *   - its checkout, if it has one, was completed.
 * Each line is matched to its API and plan by its price, since every price in the catalog belongs
 * to one plan on one API (and a usage price also to one plan price and its meter). So invoices of
 * subscribers who have since unsubscribed still count, and lines for anything else sold on the same
 * payment provider account are left out.
 */

const meterDao = require('../../dao/meterDao');
const billingConfigurationDao = require('../../dao/billingConfigurationDao');
const billingApiPlanDao = require('../../dao/billingApiPlanDao');
const planPricingDao = require('../../dao/planPricingDao');
const billingCheckoutDao = require('../../dao/billingCheckoutDao');
const billingSubscriptionDao = require('../../dao/billingSubscriptionDao');
const userIdpReferenceDao = require('../../dao/userIdpReferenceDao');
const subscriptionBilling = require('./subscriptionBillingService');
const billingProvider = require('../../billing/billingProvider');
const { BillingProviderError, BillingCursorError } = require('../../billing/http');
const { meterUnit } = require('../../utils/meterLabels');
const util = require('../../utils/util');
const logger = require('../../config/logger');

// Finalized invoices; drafts may still change and void ones were never owed.
const INVOICED = new Set(['open', 'paid', 'uncollectible']);
// Invoices read from the payment provider per page: its largest page.
const MAX_PAGE_SIZE = 100;
// A provider invoice id, as nextCursor gives it.
const CURSOR_PATTERN = /^[A-Za-z0-9_]{1,255}$/;
// A date (2026-10-01) or a date and time (2026-10-01T00:00:00Z).
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}(:\d{2}(\.\d{1,9})?)?(Z|[+-]\d{2}:\d{2}))?$/;

/**
 * price reference -> { apiPlan, price, meter, fee } for the organization's API plans: each item's
 * price, with the plan price and the meter that counts it for a usage price (both null for the flat
 * fee, which has the fee's name instead).
 */
async function priceIndex(orgId) {
    const index = new Map();
    const apiPlans = await billingApiPlanDao.listByOrg(orgId);
    const meters = new Map((await meterDao.list(orgId)).map((m) => [m.handle, m]));
    const pricing = await planPricingDao.listByPlans(orgId, [...new Set(apiPlans.map((e) => e.plan_uuid))]);
    for (const apiPlan of apiPlans) {
        const planPricing = pricing.get(apiPlan.plan_uuid) || {};
        const prices = new Map((planPricing.prices || []).map((p) => [p.uuid, p]));
        for (const item of apiPlan.items) {
            const fee = item.price_uuid === planPricing.flat_price_uuid;
            const price = fee ? null : prices.get(item.price_uuid) || null;
            index.set(item.price_ref, {
                apiPlan, price, meter: price ? meters.get(price.meter_handle) || null : null,
                fee: fee ? subscriptionBilling.FEE_NAMES[planPricing.billing_period] || 'Plan fee' : null,
            });
        }
    }
    return index;
}

/** The checkouts of the invoices that were never completed (open, or gone from the portal). */
async function uncompletedCheckouts(invoices) {
    const refs = [...new Set(invoices.map((inv) => inv.checkoutRef).filter(Boolean))];
    const checkouts = await Promise.all(refs.map((ref) => billingCheckoutDao.getCheckout(ref)));
    return new Set(refs.filter((ref, i) => !checkouts[i] || checkouts[i].status !== billingCheckoutDao.STATUS.COMPLETED));
}

/** GET /billing-reports/revenue: the paid subscriptions and their payment state, from the portal's own records. */
const getRevenueReport = async (req, res) => {
    const orgId = req.orgId;
    try {
        const conn = await billingConfigurationDao.getWithSecrets(orgId);
        if (!conn) return res.status(200).json({ configured: false });

        const billingRows = (await billingSubscriptionDao.listByOrg(orgId))
            .filter((b) => subscriptionBilling.belongsToConfiguration(b, conn));
        const statuses = billingRows.map((b) => subscriptionBilling.accessStatus(b.status));
        return res.status(200).json({
            configured: true,
            billingSince: new Date(conn.created_at).toISOString(),
            paidSubscriptions: billingRows.length,
            paidApis: new Set(billingRows.map((b) => b.api_uuid)).size,
            paymentIssues: {
                pastDue: statuses.filter((s) => s === 'PAST_DUE').length,
                paymentPending: statuses.filter((s) => s === 'PAYMENT_PENDING').length,
            },
        });
    } catch (error) {
        logger.error('Error building the revenue report', { error: error.message, orgId });
        return util.handleError(res, error);
    }
};

/** A `from` or `to` query value as a date, undefined when absent, or null when it is not a date. */
function queryDate(value) {
    if (value === undefined) return undefined;
    if (typeof value !== 'string' || !DATE_PATTERN.test(value)) return null;
    const date = new Date(value.length === 10 ? `${value}T00:00:00Z` : value);
    return Number.isNaN(date.getTime()) ? null : date;
}

/** One line of an invoice as the list gives it: what it charges for, its amount and, for a usage price, its usage. */
function itemOf(line, known, currency) {
    const usage = known.price && known.meter && Number.isFinite(line.quantity);
    return {
        name: known.price ? known.price.name : known.fee,
        amount: subscriptionBilling.fromMinor(line.amountMinor, currency),
        quantity: usage ? line.quantity : null,
        unit: usage ? meterUnit(known.meter) : null,
        countsRequests: Boolean(usage && known.meter.type === 'REQUEST_COUNT'),
    };
}

/**
 * GET /billing-reports/invoices?from&to&limit&after: one page of the portal's invoices created from
 * `from` up to (not including) `to`, newest first, each with the API and plan it bills. A page reads
 * `limit` of the provider's invoices and keeps the portal's own, so it may hold fewer, or none, while
 * `nextCursor` says more follow.
 */
const listRevenueInvoices = async (req, res) => {
    const orgId = req.orgId;
    try {
        const now = new Date();
        const from = queryDate(req.query.from);
        const to = queryDate(req.query.to);
        if (from === null || to === null) {
            return util.sendError(res, 400, 'from and to must be dates such as 2026-10-01, or dates and times such as 2026-10-01T00:00:00Z');
        }
        const since = from || new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
        const until = to || now;
        if (since >= until) return util.sendError(res, 400, 'from must be before to');
        const limit = req.query.limit === undefined ? MAX_PAGE_SIZE : Number(req.query.limit);
        if (!Number.isInteger(limit) || limit < 1 || limit > MAX_PAGE_SIZE) {
            return util.sendError(res, 400, `limit must be a whole number from 1 to ${MAX_PAGE_SIZE}`);
        }
        const after = req.query.after;
        if (after !== undefined && !CURSOR_PATTERN.test(after)) {
            return util.sendError(res, 400, 'after must be the nextCursor of a previous page');
        }

        const body = { invoices: [], nextCursor: null, invoicesAvailable: true };
        const conn = await billingConfigurationDao.getWithSecrets(orgId);
        if (!conn) return res.status(200).json(body);
        const customers = await billingCheckoutDao.listCustomers(orgId);

        let page;
        try {
            page = await billingProvider.forConfiguration(conn).payments.invoicePage({
                since, until, customerRefs: new Set(customers.keys()), limit, startingAfter: after,
            });
        } catch (err) {
            if (err instanceof BillingCursorError) return util.sendError(res, 400, 'after must be the nextCursor of a previous page');
            if (!(err instanceof BillingProviderError)) throw err;
            logger.warn('Could not read invoices from the payment provider for the revenue report', {
                orgId, provider: err.provider, status: err.status, error: err.message,
            });
            body.invoicesAvailable = false;
            return res.status(200).json(body);
        }
        body.nextCursor = page.cursor;

        const finalized = page.invoices.filter((inv) => INVOICED.has(inv.status));
        const uncompleted = await uncompletedCheckouts(finalized);
        const invoices = finalized.filter((inv) => !uncompleted.has(inv.checkoutRef));
        if (invoices.length === 0) return res.status(200).json(body);

        const prices = await priceIndex(orgId);
        const subscriptionIds = new Map((await billingSubscriptionDao.listByOrg(orgId))
            .filter((b) => subscriptionBilling.belongsToConfiguration(b, conn))
            .map((b) => [b.subscription_ref, b.subscription_uuid]));
        // The subscriber's name in the portal, for an invoice the provider has no customer name on.
        const names = new Map();
        const portalName = async (customerRef) => {
            if (!names.has(customerRef)) {
                const customer = customers.get(customerRef);
                const name = customer ? await userIdpReferenceDao.resolveDisplay(customer.user_uuid) : null;
                names.set(customerRef, typeof name === 'string' ? name : null);
            }
            return names.get(customerRef);
        };

        for (const inv of invoices) {
            const lines = inv.lines.map((line) => ({ line, known: line.priceRef && prices.get(line.priceRef) }))
                .filter((l) => l.known);
            const apiPlan = lines.length > 0 ? lines[0].known.apiPlan : null;
            body.invoices.push({
                number: inv.number,
                created: inv.created ? inv.created.toISOString() : null,
                currency: inv.currency,
                total: subscriptionBilling.fromMinor(inv.totalMinor, inv.currency),
                paid: inv.status === 'paid',
                subscriber: { name: inv.customerName || await portalName(inv.customerRef), email: inv.customerEmail },
                subscriptionId: subscriptionIds.get(inv.subscriptionRef) || null,
                api: apiPlan ? `${apiPlan.api_name} ${apiPlan.api_version}` : null,
                plan: apiPlan ? apiPlan.plan_name : null,
                items: lines.map(({ line, known }) => itemOf(line, known, inv.currency)),
            });
        }
        return res.status(200).json(body);
    } catch (error) {
        logger.error('Error reading invoices for the revenue report', { error: error.message, orgId });
        return util.handleError(res, error);
    }
};

module.exports = { getRevenueReport, listRevenueInvoices };
