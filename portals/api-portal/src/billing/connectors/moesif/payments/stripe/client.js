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
 * Stripe, called with the billing configuration's restricted key. Bodies are form-encoded, as Stripe
 * requires; every request is pinned to [api_portal.billing.stripe] api_version; creates carry an
 * Idempotency-Key so a retried request cannot create twice.
 *
 * Only the calls the portal makes are here.
 */

const crypto = require('crypto');
const { billingConfig, createHttpClient, BillingProviderError, networkError, redactSecrets } = require('../../../../http');

const PROVIDER = 'Stripe';

/**
 * The billing status of a Stripe subscription status. PAST_DUE (Stripe is still retrying a renewal
 * payment) is still served; a past-due subscription Stripe has stopped retrying is UNPAID regardless of the
 * account's setting (index.js getSubscription). `incomplete` (the first payment has not gone through)
 * is UNPAID: nothing is served before a subscription is first paid.
 */
const BILLING_STATUS = Object.freeze({
    active: 'ACTIVE',
    trialing: 'ACTIVE',
    past_due: 'PAST_DUE',
    incomplete: 'UNPAID',
    unpaid: 'UNPAID',
    paused: 'UNPAID',
    incomplete_expired: 'UNPAID',
    canceled: 'CANCELLED',
});

function billingStatus(stripeStatus) {
    return BILLING_STATUS[stripeStatus] || 'UNPAID';
}

/** TEST or LIVE from a publishable key (pk_test_, pk_live_); null for anything else. */
function publishableKeyType(key) {
    const match = /^pk_(test|live)_[A-Za-z0-9]+$/.exec(String(key || ''));
    if (!match) return null;
    return match[1] === 'test' ? 'TEST' : 'LIVE';
}

/** TEST or LIVE from a secret or restricted key; null for anything else (a publishable key). */
function keyType(apiKey) {
    const match = /^(sk|rk)_(test|live)_[A-Za-z0-9]+$/.exec(String(apiKey || ''));
    if (!match) return null;
    return match[2] === 'test' ? 'TEST' : 'LIVE';
}

/**
 * Form-encodes a nested object the way Stripe reads it: `a[b]=1`, `list[0]=x`. Undefined and
 * null values are left out.
 */
function formEncode(value, prefix, pairs = []) {
    if (value === undefined || value === null) return pairs;
    if (Array.isArray(value)) {
        value.forEach((item, i) => formEncode(item, `${prefix}[${i}]`, pairs));
    } else if (typeof value === 'object') {
        for (const [key, item] of Object.entries(value)) {
            formEncode(item, prefix ? `${prefix}[${key}]` : key, pairs);
        }
    } else {
        pairs.push(`${encodeURIComponent(prefix)}=${encodeURIComponent(String(value))}`);
    }
    return pairs;
}

/**
 * Checks a Stripe-Signature header ("t=<unix>,v1=<hex>[,v1=<hex>...]") against the endpoint's
 * signing secret: an HMAC-SHA256 of "<t>.<raw body>". Stripe sends one v1 per active secret
 * while a secret is being rolled, so any match is accepted. Events older than `toleranceSec`
 * are refused, so a processed request cannot be sent again later.
 */
function verifyWebhookSignature(secret, rawBody, header, toleranceSec = 300, nowSec = Math.floor(Date.now() / 1000)) {
    if (!secret || !header) return false;
    let timestamp = NaN;
    const signatures = [];
    for (const part of String(header).split(',')) {
        const eq = part.indexOf('=');
        const key = part.slice(0, eq).trim();
        const value = part.slice(eq + 1).trim();
        if (key === 't') timestamp = Number(value);
        if (key === 'v1' && /^[0-9a-f]{64}$/.test(value)) signatures.push(value);
    }
    if (!Number.isInteger(timestamp) || signatures.length === 0) return false;
    if (Math.abs(nowSec - timestamp) > toleranceSec) return false;
    const expected = crypto.createHmac('sha256', secret).update(`${timestamp}.`).update(rawBody).digest();
    return signatures.some((sig) => crypto.timingSafeEqual(expected, Buffer.from(sig, 'hex')));
}

function createStripeClient({ apiKey, http }) {
    const cfg = billingConfig().stripe || {};
    const baseUrl = String(cfg.baseUrl || 'https://api.stripe.com').replace(/\/+$/, '');
    const apiVersion = cfg.apiVersion;
    const client = http || createHttpClient();

    async function call(method, path, { query, body, idempotent } = {}) {
        const headers = { Authorization: `Bearer ${apiKey}`, 'Stripe-Version': apiVersion };
        let data;
        if (body) {
            headers['Content-Type'] = 'application/x-www-form-urlencoded';
            data = formEncode(body, '').join('&');
        }
        if (idempotent) headers['Idempotency-Key'] = crypto.randomUUID();
        const qs = query ? `?${formEncode(query, '').join('&')}` : '';
        let res;
        try {
            res = await client.request({ method, url: `${baseUrl}${path}${qs}`, headers, data });
        } catch (err) {
            throw networkError(PROVIDER, err);
        }
        if (res.status >= 200 && res.status < 300) return res.data;
        const error = res.data && res.data.error ? res.data.error : {};
        throw new BillingProviderError(PROVIDER, res.status,
            `${PROVIDER}: ${redactSecrets(error.message || `request failed with HTTP ${res.status}`, [apiKey])}`,
            error.code || error.type);
    }

    return {
        apiVersion,

        /** All webhook endpoints of the account (Stripe allows at most 16). */
        async listWebhookEndpoints() {
            const page = await call('GET', '/v1/webhook_endpoints', { query: { limit: 100 } });
            return page.data || [];
        },

        /** Creates an endpoint; the result carries `secret`, which Stripe returns only now. */
        createWebhookEndpoint({ url, events, description, metadata }) {
            return call('POST', '/v1/webhook_endpoints', {
                body: { url, enabled_events: events, description, metadata, api_version: apiVersion },
                idempotent: true,
            });
        },

        /** Changes an endpoint; what is left out stays as it is. Stripe keeps its signing secret. */
        updateWebhookEndpoint(id, { url, events, disabled }) {
            return call('POST', `/v1/webhook_endpoints/${encodeURIComponent(id)}`, {
                body: { url, enabled_events: events, disabled },
            });
        },

        /** Creates a customer; `metadata` identifies the portal user. */
        createCustomer({ name, email, metadata }) {
            return call('POST', '/v1/customers', { body: { name, email, metadata }, idempotent: true });
        },

        /**
         * Creates an embedded subscription Checkout Session: the portal mounts it in its own page
         * with Stripe.js and the session's client_secret. `lineItems` are [{ price, quantity? }]: a
         * flat price with quantity 1, metered prices without one. A payment method that needs to
         * leave the page returns to `returnUrl`; the others complete in place.
         */
        createCheckoutSession({ customer, lineItems, clientReferenceId, returnUrl, metadata }) {
            return call('POST', '/v1/checkout/sessions', {
                body: {
                    mode: 'subscription',
                    ui_mode: 'embedded',
                    customer,
                    line_items: lineItems,
                    client_reference_id: clientReferenceId,
                    return_url: returnUrl,
                    redirect_on_completion: 'if_required',
                    metadata,
                    subscription_data: { metadata },
                    // Prices show in the plan's own currency, as the subscription and its invoices are in it;
                    // Adaptive Pricing would show visitors a converted local-currency amount instead.
                    adaptive_pricing: { enabled: false },
                },
                idempotent: true,
            });
        },

        retrieveCheckoutSession(id) {
            return call('GET', `/v1/checkout/sessions/${encodeURIComponent(id)}`);
        },

        /** Ends an open Checkout Session, so it can no longer be paid. */
        expireCheckoutSession(id) {
            return call('POST', `/v1/checkout/sessions/${encodeURIComponent(id)}/expire`);
        },

        /** Refunds a payment in full. */
        createRefund({ paymentIntent }) {
            return call('POST', '/v1/refunds', { body: { payment_intent: paymentIntent }, idempotent: true });
        },

        retrieveSubscription(id) {
            return call('GET', `/v1/subscriptions/${encodeURIComponent(id)}`);
        },

        /** Updates a subscription, for example to end it at the end of its period (cancel_at_period_end). */
        updateSubscription(id, body) {
            return call('POST', `/v1/subscriptions/${encodeURIComponent(id)}`, { body });
        },

        retrieveInvoice(id) {
            return call('GET', `/v1/invoices/${encodeURIComponent(id)}`);
        },

        /**
         * Cancels a subscription now. With `invoiceNow`, Stripe issues a final invoice immediately for
         * the usage reported so far, rather than dropping it.
         */
        cancelSubscription(id, { invoiceNow } = {}) {
            return call('DELETE', `/v1/subscriptions/${encodeURIComponent(id)}`, {
                query: invoiceNow ? { invoice_now: true } : undefined,
            });
        },

        /** The subscription's next invoice as it stands now: the fee and the usage reported so far. */
        previewInvoice(subscription) {
            return call('POST', '/v1/invoices/create_preview', { body: { subscription } });
        },

        /**
         * A page of an invoice's lines. An invoice embeds only its first 10; a preview's id
         * (upcoming_in_...) reads the rest of the preview's lines the same way.
         */
        listInvoiceLinePage(invoice, startingAfter, limit = 100) {
            return call('GET', `/v1/invoices/${encodeURIComponent(invoice)}/lines`, {
                query: { limit, starting_after: startingAfter },
            });
        },

        /** The subscription's invoices, newest first. */
        async listInvoices(subscription, limit = 12) {
            const page = await call('GET', '/v1/invoices', { query: { subscription, limit } });
            return page.data || [];
        },

        /**
         * One page (`limit`, at most 100) of the account's invoices created at or after `createdGte`
         * and, when given, before `createdLt` (Unix seconds), newest first; pass the last id as
         * `startingAfter` for the next page. With `customer`, only that customer's invoices.
         */
        listInvoicePage({ createdGte, createdLt, startingAfter, customer, limit = 100 }) {
            return call('GET', '/v1/invoices', {
                query: { limit, created: { gte: createdGte, lt: createdLt }, starting_after: startingAfter, customer },
            });
        },

        /** A session on Stripe's billing page, where the customer manages payment methods and invoices. */
        createBillingPortalSession({ customer, returnUrl, configuration }) {
            return call('POST', '/v1/billing_portal/sessions', { body: { customer, return_url: returnUrl, configuration } });
        },

        /** The account's active billing page configurations. */
        listBillingPortalConfigurations() {
            return call('GET', '/v1/billing_portal/configurations', { query: { active: true, limit: 100 } });
        },

        /** A billing page configuration: what customers can do on Stripe's billing page. */
        createBillingPortalConfiguration(body) {
            return call('POST', '/v1/billing_portal/configurations', { body });
        },

        async deleteWebhookEndpoint(id) {
            try {
                await call('DELETE', `/v1/webhook_endpoints/${encodeURIComponent(id)}`);
            } catch (err) {
                if (err instanceof BillingProviderError && err.status === 404) return;
                throw err;
            }
        },
    };
}

module.exports = { PROVIDER, keyType, publishableKeyType, billingStatus, formEncode, verifyWebhookSignature, createStripeClient };
