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
 * The subscriber's Invoices page: their invoices across their paid subscriptions (a page at a
 * time), and the payment provider's billing page, where they manage payment methods. Each
 * subscriber is one customer in the payment provider per billing configuration, created at their
 * first checkout.
 */

const apiDao = require('../../dao/apiDao');
const subscriptionPlanDao = require('../../dao/subscriptionPlanDao');
const billingConfigurationDao = require('../../dao/billingConfigurationDao');
const billingCheckoutDao = require('../../dao/billingCheckoutDao');
const billingProvider = require('../../billing/billingProvider');
const { BillingProviderError, BillingCursorError } = require('../../billing/http');
const checkoutService = require('./checkoutService');
const subscriptionBilling = require('./subscriptionBillingService');
const util = require('../../utils/util');
const logger = require('../../config/logger');

// How far back the invoice list reaches, in months (ALL: as far as the provider keeps them).
const INVOICE_PERIODS = Object.freeze({ LAST_3_MONTHS: 3, LAST_6_MONTHS: 6, LAST_12_MONTHS: 12, ALL: null });
const DEFAULT_PAGE_SIZE = 20;
const MAX_PAGE_SIZE = 100;
// A page cursor: the provider's id of the last invoice on the previous page.
const CURSOR_PATTERN = /^[A-Za-z0-9_]{1,255}$/;

/** The billing configuration with its keys, and the caller's customer under it (either may be null). */
async function currentCustomer(req) {
    const conn = await billingConfigurationDao.getWithSecrets(req.orgId);
    if (!conn) return { conn: null, customer: null };
    const customer = await billingCheckoutDao.findCustomer(conn.uuid, util.resolveActor(req));
    return { conn, customer };
}

/**
 * The checkout behind each invoice, found through the invoice's subscription, by invoice id:
 * { subject, completed }.
 *   - subject: "API v1 · Plan". Checkouts are kept after a subscription ends, so invoices of ended
 *     subscriptions are named too.
 *   - completed: false for a checkout never paid, such as one whose card was declined and then
 *     left. The payment provider still keeps an unpaid invoice for it.
 */
async function invoiceCheckouts(req, invoices) {
    const checkouts = new Map();
    const byCheckout = new Map();
    for (const invoice of invoices) {
        if (!invoice.checkoutRef) continue;
        if (!byCheckout.has(invoice.checkoutRef)) {
            byCheckout.set(invoice.checkoutRef, (async () => {
                const checkout = await billingCheckoutDao.getCheckout(invoice.checkoutRef);
                if (!checkout || checkout.org_uuid !== req.orgId || checkout.user_uuid !== util.resolveActor(req)) return null;
                const [[api], plan] = await Promise.all([
                    apiDao.get(req.orgId, checkout.api_uuid).catch(() => []),
                    subscriptionPlanDao.getByUuid(req.orgId, checkout.plan_uuid).catch(() => null),
                ]);
                const apiText = api ? [api.name, api.version].filter(Boolean).join(' ') : null;
                return {
                    subject: [apiText, plan && plan.display_name].filter(Boolean).join(' · ') || null,
                    completed: checkout.status === billingCheckoutDao.STATUS.COMPLETED,
                };
            })());
        }
        checkouts.set(invoice.id, await byCheckout.get(invoice.checkoutRef));
    }
    return checkouts;
}

/** GET /billing/invoices?period=&limit=&after= */
const listBillingInvoices = async (req, res) => {
    try {
        const period = req.query.period || 'LAST_12_MONTHS';
        if (!Object.prototype.hasOwnProperty.call(INVOICE_PERIODS, period)) {
            return util.sendError(res, 400, `period must be one of ${Object.keys(INVOICE_PERIODS).join(', ')}`);
        }
        const limit = req.query.limit === undefined ? DEFAULT_PAGE_SIZE : Number(req.query.limit);
        if (!Number.isInteger(limit) || limit < 1 || limit > MAX_PAGE_SIZE) {
            return util.sendError(res, 400, `limit must be a whole number from 1 to ${MAX_PAGE_SIZE}`);
        }
        const after = req.query.after;
        if (after !== undefined && !CURSOR_PATTERN.test(after)) {
            return util.sendError(res, 400, 'after must be the nextCursor of a previous page');
        }
        const { conn, customer } = await currentCustomer(req);
        const body = { period, canManageBilling: Boolean(conn && customer), invoices: [], nextCursor: null, invoicesAvailable: true };
        if (!conn || !customer) return res.status(200).json(body);
        const months = INVOICE_PERIODS[period];
        const now = new Date();
        const since = months
            ? new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - months, now.getUTCDate()))
            : new Date(0);
        let page;
        try {
            page = await billingProvider.forConfiguration(conn).payments.customerInvoicePage(customer.customer_ref,
                { since, limit, startingAfter: after });
        } catch (err) {
            if (err instanceof BillingCursorError) return util.sendError(res, 400, 'after must be the nextCursor of a previous page');
            if (!(err instanceof BillingProviderError)) throw err;
            logger.warn('Could not read invoices from the payment provider', { error: err.message });
            body.invoicesAvailable = false;
            return res.status(200).json(body);
        }
        // An invoice of a checkout never paid is not the subscriber's to pay: it is left out.
        const checkouts = await invoiceCheckouts(req, page.invoices);
        body.invoices = page.invoices
            .filter((invoice) => !checkouts.get(invoice.id) || checkouts.get(invoice.id).completed)
            .map((invoice) => ({
                ...subscriptionBilling.invoiceDto(invoice),
                subscription: checkouts.get(invoice.id) ? checkouts.get(invoice.id).subject : null,
            }));
        body.nextCursor = page.hasMore && page.invoices.length > 0 ? page.invoices[page.invoices.length - 1].id : null;
        return res.status(200).json(body);
    } catch (error) {
        return subscriptionBilling.sendBillingError(res, error, 'Error reading invoices');
    }
};

/** POST /billing/billing-portal-sessions { viewName? } */
const createAccountBillingPortalSession = async (req, res) => {
    try {
        const { conn, customer } = await currentCustomer(req);
        if (!conn || !customer) {
            throw new subscriptionBilling.SubscriptionBillingError(409,
                'Billing details are available once you subscribe to a paid plan.', subscriptionBilling.NOT_CONFIGURED_CODE);
        }
        const viewName = req.body && req.body.viewName;
        const returnUrl = await checkoutService.viewPageUrl(req.orgId, viewName, '/billing/invoices');
        let pageUrl;
        try {
            pageUrl = await billingProvider.forConfiguration(conn).payments.billingPageUrl({ customerRef: customer.customer_ref, returnUrl });
        } catch (err) {
            if (!(err instanceof BillingProviderError)) throw err;
            logger.warn('The billing page could not be opened', { error: err.message });
            throw subscriptionBilling.providerFailure(err, 'The billing page could not be opened.');
        }
        const url = subscriptionBilling.safeUrl(pageUrl);
        if (!url) {
            throw new subscriptionBilling.SubscriptionBillingError(502, 'The billing page could not be opened.', 'BILLING_PROVIDER_UNAVAILABLE');
        }
        return res.status(201).json({ url });
    } catch (error) {
        return subscriptionBilling.sendBillingError(res, error, 'Error opening the billing page');
    }
};

module.exports = { INVOICE_PERIODS, listBillingInvoices, createAccountBillingPortalSession };
