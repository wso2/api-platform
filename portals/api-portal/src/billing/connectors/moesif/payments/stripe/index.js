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
 * The Moesif connector's Stripe sub-connector: the payment side of a Moesif setup that bills
 * through Stripe.
 *
 * Moesif creates the catalog (products and prices) in Stripe itself, so this sub-connector creates
 * only what belongs to a subscriber: the customer, and the subscription (through a Checkout
 * Session). It also reads a subscription's state, cancels it, opens Stripe's billing page, reads
 * invoices and the current period's estimate, and receives Stripe's payment events.
 *
 * Every function answers in the billing provider interface's own terms (see billingProvider.js):
 * references are Stripe ids, amounts are in the currency's minor unit, and statuses are the
 * portal's billing statuses.
 */

const client = require('./client');
const { BillingProviderError, BillingCursorError } = require('../../../../http');

const PAYMENT_PROVIDER = 'STRIPE';
// The billing provider name Moesif knows Stripe by, for catalog objects Moesif syncs to Stripe.
const MOESIF_PROVIDER_SLUG = 'stripe';
// The Stripe API version Moesif's connection to Stripe is saved with: the one the Moesif
// dashboard sends when it connects Stripe.
const MOESIF_STRIPE_API_VERSION = '2020-08-27';
// Pages of 100 read for an invoice's lines beyond the 10 it embeds: far more than a plan has.
const MAX_LINE_PAGES = 5;

// The Stripe events the portal acts on.
const PORTAL_EVENTS = Object.freeze([
    'checkout.session.completed',
    'checkout.session.expired',
    'customer.subscription.updated',
    'customer.subscription.deleted',
    'invoice.paid',
    'invoice.payment_failed',
]);

// The events Moesif's endpoint receives: every Customer and Invoice event, as Moesif's Stripe guide
// asks ("select all events for the Customer and Invoice event types"), named as Stripe's event
// types reference lists them.
const MOESIF_EVENTS = Object.freeze([
    'customer.created',
    'customer.deleted',
    'customer.updated',
    'customer.discount.created',
    'customer.discount.deleted',
    'customer.discount.updated',
    'customer.source.created',
    'customer.source.deleted',
    'customer.source.expiring',
    'customer.source.updated',
    'customer.subscription.created',
    'customer.subscription.deleted',
    'customer.subscription.paused',
    'customer.subscription.pending_update_applied',
    'customer.subscription.pending_update_expired',
    'customer.subscription.resumed',
    'customer.subscription.trial_will_end',
    'customer.subscription.updated',
    'customer.tax_id.created',
    'customer.tax_id.deleted',
    'customer.tax_id.updated',
    'invoice.created',
    'invoice.deleted',
    'invoice.finalization_failed',
    'invoice.finalized',
    'invoice.marked_uncollectible',
    'invoice.overdue',
    'invoice.overpaid',
    'invoice.paid',
    'invoice.payment_action_required',
    'invoice.payment_attempt_required',
    'invoice.payment_failed',
    'invoice.payment_succeeded',
    'invoice.sent',
    'invoice.upcoming',
    'invoice.updated',
    'invoice.voided',
    'invoice.will_be_due',
]);

// Checkout Session payment statuses that let the subscription start.
const PAID = new Set(['paid', 'no_payment_required']);

// What the portal does for each event: read a checkout, a subscription, or an invoice's
// subscription. The event's own copy of the object is never trusted; the current state is read.
const EVENT_SUBJECTS = Object.freeze({
    'checkout.session.completed': 'CHECKOUT',
    'checkout.session.expired': 'CHECKOUT',
    'customer.subscription.updated': 'SUBSCRIPTION',
    'customer.subscription.deleted': 'SUBSCRIPTION',
    'invoice.paid': 'INVOICE',
    'invoice.payment_failed': 'INVOICE',
});

// Builds the Stripe API client; tests replace it with their own.
const clients = {
    stripe: (apiKey) => client.createStripeClient({ apiKey }),
};

function idOf(value) {
    if (typeof value === 'string') return value;
    return value && typeof value.id === 'string' ? value.id : null;
}

function checkoutDto(session) {
    let state = 'OPEN';
    if (session.status === 'complete' && PAID.has(session.payment_status)) state = 'PAID';
    else if (session.status === 'expired') state = 'EXPIRED';
    return {
        ref: session.id,
        reference: session.client_reference_id || null,
        state,
        subscriptionRef: idOf(session.subscription),
        // What the browser mounts an open embedded checkout with.
        clientSecret: state === 'OPEN' && typeof session.client_secret === 'string' ? session.client_secret : null,
    };
}

function subscriptionDto(sub) {
    const periodEnd = Number.isFinite(sub.current_period_end) ? new Date(sub.current_period_end * 1000) : null;
    let endsAt = null;
    if (Number.isFinite(sub.cancel_at)) endsAt = new Date(sub.cancel_at * 1000);
    else if (sub.cancel_at_period_end) endsAt = periodEnd;
    return {
        ref: sub.id,
        status: client.billingStatus(sub.status),
        ended: sub.status === 'canceled',
        periodStart: Number.isFinite(sub.current_period_start) ? new Date(sub.current_period_start * 1000) : null,
        periodEnd,
        // When it is set to end, and whether it ends (or ended) because that was asked for, as
        // opposed to Stripe ending it after failed payments.
        endsAt,
        endRequested: Boolean(endsAt)
            || Boolean(sub.cancellation_details && sub.cancellation_details.reason === 'cancellation_requested'),
    };
}

/**
 * Whether Stripe has stopped retrying the payment of one of the subscription's invoices: an invoice
 * still open after at least one attempt, with no next attempt planned. An invoice Stripe has not tried
 * to charge yet has no attempt.
 */
async function retriesEnded(stripe, subscriptionRef) {
    const invoices = await stripe.listInvoices(subscriptionRef, 100);
    return invoices.some((invoice) => invoice.status === 'open' && invoice.attempt_count > 0 && !invoice.next_payment_attempt);
}

// Marks the billing page configuration the portal opens Stripe's billing page with.
const BILLING_PAGE_MARKER = Object.freeze({ key: 'wso2_api_portal', value: 'billing-page' });

// What customers can do on Stripe's billing page: their payment methods, billing details and
// invoices. Subscriptions are ended in the portal only, so its rules apply.
const BILLING_PAGE_FEATURES = Object.freeze({
    payment_method_update: { enabled: true },
    customer_update: { enabled: true, allowed_updates: ['email', 'address', 'name', 'phone'] },
    invoice_history: { enabled: true },
    subscription_cancel: { enabled: false },
    subscription_update: { enabled: false },
});

function linesOf(invoice) {
    return ((invoice.lines && invoice.lines.data) || []).map((line) => ({
        priceRef: idOf(line.price),
        description: line.description || '',
        quantity: Number.isFinite(line.quantity) ? line.quantity : null,
        amountMinor: Number(line.amount || 0),
    }));
}

function invoiceDto(invoice) {
    return {
        id: invoice.id,
        created: Number.isFinite(invoice.created) ? new Date(invoice.created * 1000) : null,
        customerRef: idOf(invoice.customer),
        subscriptionRef: idOf(invoice.subscription),
        customerName: invoice.customer_name || null,
        customerEmail: invoice.customer_email || null,
        currency: String(invoice.currency || '').toUpperCase(),
        totalMinor: Number(invoice.total || 0),
        // draft, open, paid, uncollectible, void
        status: invoice.status,
        number: invoice.number || null,
        url: typeof invoice.hosted_invoice_url === 'string' ? invoice.hosted_invoice_url : null,
        pdfUrl: typeof invoice.invoice_pdf === 'string' ? invoice.invoice_pdf : null,
        // The portal checkout that started the subscription, carried in its metadata.
        checkoutRef: (invoice.subscription_details && invoice.subscription_details.metadata
            && invoice.subscription_details.metadata.wso2_checkout) || null,
        lines: linesOf(invoice),
    };
}

/**
 * The Stripe part of a Moesif billing meter's billing plan: the Stripe product (Moesif's catalog
 * plan) and the metered price the meter reports usage to.
 */
function moesifBillingPlanParams({ productRef, productName, priceRef, reportingPeriod = '15m' }) {
    return {
        stripe_params: {
            product: { id: productRef, name: productName, usage_type: 'metered' },
            prices: [{ price_id: priceRef }],
            reporting: { reporting_period: reportingPeriod },
        },
    };
}

/** TEST or LIVE for a Stripe secret or restricted key; null for anything else. */
function mode(apiKey) {
    return client.keyType(apiKey);
}

/** TEST or LIVE for a Stripe publishable key; null for anything else. */
function publishableMode(key) {
    return client.publishableKeyType(key);
}

/**
 * What Moesif needs to bill through this Stripe account: the key it uses with Stripe, and how a
 * Stripe customer maps to a Moesif company (by the customer's id, the company_id the gateway
 * reports). `enabled` false turns the connection off.
 */
function moesifPartnerAccount({ apiKey, enabled }) {
    return {
        partner_account: {
            version: 'v2',
            api_version: MOESIF_STRIPE_API_VERSION,
            credentials: enabled ? { state: 'enabled', api_key: apiKey } : { state: 'disabled' },
        },
        billing_config: { stripe: { company_id_field: 'id' } },
    };
}

/** Whether Moesif's connection to Stripe (as getPartnerAccount returns it) is on. */
function moesifPartnerAccountEnabled(account) {
    const state = account && account.partner_account && account.partner_account.credentials
        && account.partner_account.credentials.state;
    return state === 'enabled';
}

/**
 * The payment functions for one billing configuration. `settings` carries the Stripe key; the configuration's
 * other Stripe settings (the webhook endpoint and its signing secret) are passed where needed.
 */
function create(settings) {
    const stripe = clients.stripe(settings.stripeApiKey);

    /**
     * All the lines of an invoice: Stripe embeds only the first 10, and a plan has up to 20 usage
     * prices and its fee.
     */
    async function allLines(invoice) {
        const lines = [...((invoice.lines && invoice.lines.data) || [])];
        let more = Boolean(invoice.lines && invoice.lines.has_more && invoice.id);
        for (let page = 0; more && lines.length > 0 && page < MAX_LINE_PAGES; page++) {
            const next = await stripe.listInvoiceLinePage(invoice.id, idOf(lines[lines.length - 1]));
            lines.push(...((next && next.data) || []));
            more = Boolean(next && next.has_more && next.data && next.data.length > 0);
        }
        return lines;
    }

    return {
        provider: PAYMENT_PROVIDER,
        moesifProviderSlug: MOESIF_PROVIDER_SLUG,
        moesifBillingPlanParams,
        /** Moesif's connection to this Stripe account, on or off. */
        moesifPartnerAccount(enabled) {
            return moesifPartnerAccount({ apiKey: settings.stripeApiKey, enabled });
        },
        moesifPartnerAccountEnabled,

        // ── connection ─────────────────────────────────────────────────────────

        /** Checks the key; resolves to the account's event endpoints for setUpEvents. */
        checkKey() {
            return stripe.listWebhookEndpoints();
        },

        /**
         * Registers a new endpoint for the portal's events at `url`, replacing any already there
         * (among `endpoints`). Resolves to { ref, secret }.
         */
        async createEventEndpoint({ url, endpoints }) {
            for (const endpoint of endpoints.filter((e) => e.url === url)) await stripe.deleteWebhookEndpoint(endpoint.id);
            const created = await stripe.createWebhookEndpoint({
                url,
                events: PORTAL_EVENTS,
                description: 'WSO2 API Portal: paid subscription checkout and billing status',
                metadata: { wso2_api_portal: 'true' },
            });
            return { ref: created.id, secret: created.secret };
        },

        /**
         * Moves the portal's endpoint (`current`: { ref, secret }) to `url`, keeping its id and
         * signing secret. Resolves to { ref, secret }.
         */
        async moveEventEndpoint({ url, current }) {
            const endpoints = await stripe.listWebhookEndpoints();
            const found = current && current.secret && endpoints.find((e) => e.id === current.ref);
            if (found) {
                await stripe.updateWebhookEndpoint(found.id, { url, events: PORTAL_EVENTS, disabled: false });
                return { ref: found.id, secret: current.secret };
            }
            return this.createEventEndpoint({ url, endpoints });
        },

        /**
         * Makes sure Stripe sends Moesif the events it needs at `url`. An endpoint already there
         * (another portal or the administrator may have added it) is kept: missing events are
         * added and a disabled one is turned on. Resolves to { ref, created, undo }: undo removes
         * an endpoint created here, or puts back the events and state of one that was changed.
         */
        async ensureMoesifEndpoint({ url, endpoints }) {
            const found = endpoints.find((e) => e.url === url);
            if (!found) {
                const created = await stripe.createWebhookEndpoint({
                    url,
                    events: MOESIF_EVENTS,
                    description: 'Moesif: usage-based billing (added by the WSO2 API Portal)',
                });
                return { ref: created.id, created: true, undo: () => stripe.deleteWebhookEndpoint(created.id) };
            }
            const events = found.enabled_events || [];
            const missing = events.includes('*') ? [] : MOESIF_EVENTS.filter((e) => !events.includes(e));
            const disabled = found.status === 'disabled';
            if (missing.length === 0 && !disabled) return { ref: found.id, created: false, undo: null };
            await stripe.updateWebhookEndpoint(found.id, { events: [...events, ...missing], disabled: false });
            return {
                ref: found.id,
                created: false,
                undo: () => stripe.updateWebhookEndpoint(found.id, { events, disabled }),
            };
        },

        /** The account's event endpoints: [{ id, url, enabled_events, status }]. */
        listEventEndpoints() {
            return stripe.listWebhookEndpoints();
        },

        removeEventEndpoint(ref) {
            return stripe.deleteWebhookEndpoint(ref);
        },

        // ── subscribers ────────────────────────────────────────────────────────

        /** Creates the subscriber's customer; resolves to its ref. */
        async createCustomer({ name, email, metadata }) {
            const customer = await stripe.createCustomer({ name, email, metadata });
            return customer.id;
        },

        /** The key the browser starts Stripe's checkout with (public by design). */
        publishableKey: settings.stripePublishableKey || null,

        /**
         * Starts paying for a plan: a Checkout Session that creates the subscription once paid.
         * `prices` are [{ ref, fixed }]: a fixed price is charged once per period, the others by
         * usage. Resolves to { ref, clientSecret }.
         */
        async startCheckout({ customerRef, prices, reference, returnUrl }) {
            const session = await stripe.createCheckoutSession({
                customer: customerRef,
                lineItems: prices.map((p) => (p.fixed ? { price: p.ref, quantity: 1 } : { price: p.ref })),
                clientReferenceId: reference,
                returnUrl,
                metadata: { wso2_checkout: reference },
            });
            return { ref: session.id, clientSecret: session.client_secret };
        },

        /** The checkout as Stripe has it now: { ref, reference, state: OPEN|PAID|EXPIRED, subscriptionRef, clientSecret }. */
        async getCheckout(ref) {
            return checkoutDto(await stripe.retrieveCheckoutSession(ref));
        },

        /**
         * Ends an open checkout so it can no longer be paid. Resolves to the checkout as it ends up:
         * EXPIRED, or PAID when it was paid before it could be ended (Stripe expires only open ones).
         */
        async expireCheckout(ref) {
            try {
                return checkoutDto(await stripe.expireCheckoutSession(ref));
            } catch (err) {
                if (err instanceof BillingProviderError && err.status === 400) return this.getCheckout(ref);
                throw err;
            }
        },

        /** The subscription's billing status and current period, or null when Stripe has no such subscription. */
        async getSubscription(ref) {
            let sub;
            try {
                sub = await stripe.retrieveSubscription(ref);
            } catch (err) {
                if (err instanceof BillingProviderError && err.status === 404) return null;
                throw err;
            }
            const dto = subscriptionDto(sub);
            // Past due is served only while Stripe retries the payment. An account may be set to leave a
            // subscription past due after the last retry; once Stripe has given up on one of its
            // invoices, it is unpaid, and served again when that invoice is paid.
            if (sub.status === 'past_due' && await retriesEnded(stripe, sub.id)) dto.status = 'UNPAID';
            return dto;
        },

        /**
         * Ends the subscription at the end of the period already paid for. Resolves to when it
         * ends, or null when it has already ended.
         */
        async endAtPeriodEnd(ref) {
            const current = await this.getSubscription(ref);
            if (!current || current.ended) return null;
            return subscriptionDto(await stripe.updateSubscription(ref, { cancel_at_period_end: true })).endsAt;
        },

        /**
         * Ends the subscription now, with a final invoice for the usage so far. Nothing to do
         * when Stripe already ended it or has no such subscription.
         */
        async cancelSubscription(ref) {
            const current = await this.getSubscription(ref);
            if (!current || current.ended) return;
            try {
                await stripe.cancelSubscription(ref, { invoiceNow: true });
            } catch (err) {
                if (err instanceof BillingProviderError && err.status === 404) return;
                throw err;
            }
        },

        /**
         * Ends a subscription that should not exist (a second one to the same API) immediately, without
         * a final invoice, and refunds what it was charged. Resolves to the number of payments refunded.
         */
        async cancelAndRefund(ref) {
            const current = await this.getSubscription(ref);
            if (current && !current.ended) await stripe.cancelSubscription(ref);
            let refunded = 0;
            for (const invoice of await stripe.listInvoices(ref, 10)) {
                const paymentIntent = idOf(invoice.payment_intent);
                if (Number(invoice.amount_paid) > 0 && paymentIntent) {
                    await stripe.createRefund({ paymentIntent });
                    refunded += 1;
                }
            }
            return refunded;
        },

        /**
         * The billing page configuration the portal opens Stripe's billing page with (payment
         * methods, billing details and invoices; no cancelling), created on first use.
         */
        async billingPageConfiguration() {
            const list = await stripe.listBillingPortalConfigurations();
            const found = ((list && list.data) || []).find((c) => c.active
                && c.metadata && c.metadata[BILLING_PAGE_MARKER.key] === BILLING_PAGE_MARKER.value);
            if (found) return found.id;
            const created = await stripe.createBillingPortalConfiguration({
                features: BILLING_PAGE_FEATURES,
                metadata: { [BILLING_PAGE_MARKER.key]: BILLING_PAGE_MARKER.value },
            });
            return created.id;
        },

        /** The address of Stripe's billing page for the customer, returning to `returnUrl`. */
        async billingPageUrl({ customerRef, returnUrl }) {
            const configuration = await this.billingPageConfiguration();
            const session = await stripe.createBillingPortalSession({ customer: customerRef, returnUrl, configuration });
            return session && typeof session.url === 'string' ? session.url : null;
        },

        // ── amounts ────────────────────────────────────────────────────────────

        /** The subscription's next invoice as it stands now: its lines, total and period. */
        async estimate(subscriptionRef) {
            const invoice = await stripe.previewInvoice(subscriptionRef);
            const lines = await allLines(invoice);
            return {
                currency: String(invoice.currency || '').toUpperCase(),
                periodStart: Number.isFinite(invoice.period_start) ? new Date(invoice.period_start * 1000) : null,
                periodEnd: Number.isFinite(invoice.period_end) ? new Date(invoice.period_end * 1000) : null,
                lines: linesOf({ lines: { data: lines } }),
                totalMinor: Number(invoice.total || 0),
            };
        },

        /** The subscription's invoices, newest first. */
        async invoices(subscriptionRef) {
            return (await stripe.listInvoices(subscriptionRef)).map(invoiceDto);
        },

        /**
         * One page of the account's invoices created from `since` up to (not including) `until`,
         * newest first, after the invoice `startingAfter` (a previous page's `cursor`). Only the
         * invoices of the customers in `customerRefs` (a Set) are kept, each with all its lines.
         * Resolves to { invoices, cursor }: `cursor` is the page's last invoice, kept or not, or null
         * when no more follow.
         */
        async invoicePage({ since, until, customerRefs, limit, startingAfter }) {
            let result;
            try {
                result = await stripe.listInvoicePage({ createdGte: Math.floor(since.getTime() / 1000),
                    createdLt: Math.floor(until.getTime() / 1000), startingAfter, limit });
            } catch (err) {
                // Stripe answers an unknown starting_after invoice with 400 resource_missing.
                if (startingAfter && err instanceof BillingProviderError && err.status === 400 && err.code === 'resource_missing') {
                    throw new BillingCursorError('Stripe', 'The page cursor is not one of the payment provider\'s invoices.');
                }
                throw err;
            }
            const data = (result && result.data) || [];
            const invoices = [];
            for (const invoice of data.filter((inv) => customerRefs.has(idOf(inv.customer)))) {
                invoices.push(invoiceDto({ ...invoice, lines: { data: await allLines(invoice) } }));
            }
            const more = Boolean(result && result.has_more) && data.length > 0;
            return { invoices, cursor: more ? data[data.length - 1].id : null };
        },

        /**
         * One page of the customer's invoices created since `since`, newest first, after the
         * invoice `startingAfter` (a previous page's last id). Resolves to { invoices, hasMore }.
         */
        async customerInvoicePage(customerRef, { since, limit, startingAfter }) {
            let result;
            try {
                result = await stripe.listInvoicePage({
                    createdGte: Math.floor(since.getTime() / 1000), startingAfter, customer: customerRef, limit,
                });
            } catch (err) {
                // Stripe answers an unknown starting_after invoice with 400 resource_missing.
                if (startingAfter && err instanceof BillingProviderError && err.status === 400 && err.code === 'resource_missing') {
                    throw new BillingCursorError('Stripe', 'The page cursor is not one of the payment provider\'s invoices.');
                }
                throw err;
            }
            return { invoices: ((result && result.data) || []).map(invoiceDto), hasMore: Boolean(result && result.has_more) };
        },

        // ── payment events ─────────────────────────────────────────────────────

        /** The subscription an invoice belongs to, or null. */
        async subscriptionOfInvoice(invoiceRef) {
            return idOf((await stripe.retrieveInvoice(invoiceRef)).subscription);
        },
    };
}

/** What the portal reads for an event type (CHECKOUT, SUBSCRIPTION, INVOICE), or null to ignore it. */
function eventSubject(type) {
    return EVENT_SUBJECTS[type] || null;
}

/**
 * A received event, when its signature checks out against the endpoint's secret (`header` reads a
 * request header by name): { id, type,
 * subject, objectRef }, where subject says what to read (CHECKOUT, SUBSCRIPTION, INVOICE) or is
 * null for an event the portal does not act on. Resolves to null when the event is not genuine.
 */
function readEvent(secret, rawBody, header, toleranceSec) {
    if (!client.verifyWebhookSignature(secret, rawBody, header('stripe-signature'), toleranceSec)) return null;
    let event;
    try {
        event = JSON.parse(rawBody.toString('utf8'));
    } catch (_err) {
        return null;
    }
    if (!event || typeof event.id !== 'string' || typeof event.type !== 'string') return null;
    const object = event.data && event.data.object;
    return {
        id: event.id,
        type: event.type,
        subject: eventSubject(event.type),
        objectRef: object && typeof object.id === 'string' ? object.id : null,
    };
}

module.exports = {
    PAYMENT_PROVIDER,
    PORTAL_EVENTS,
    MOESIF_EVENTS,
    EVENT_SUBJECTS,
    MOESIF_PROVIDER_SLUG,
    clients,
    mode,
    publishableMode,
    moesifBillingPlanParams,
    create,
    eventSubject,
    readEvent,
};
