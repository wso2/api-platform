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
 * The Moesif connector.
 *
 * Moesif meters usage and owns the billing catalog: the portal creates each paid plan's catalog
 * plan, prices and billing meters in Moesif, and Moesif syncs them to the billing provider it is
 * linked to. Moesif can bill through more than one provider (Stripe, Chargebee, ...), so the
 * payment side is a sub-connector, chosen by the configuration's payment provider: it creates the
 * subscriber's customer and subscription in that provider and receives its payment events. The
 * sub-connectors are listed in PAYMENT_PROVIDERS.
 *
 * The usage the gateway sends to Moesif is counted with the same filter as the billing meters
 * (catalog.js), so what a subscriber sees is what is billed.
 */

const client = require('./client');
const catalog = require('./catalog');
const stripePayments = require('./payments/stripe');

const PROVIDER = 'MOESIF';

// The payment sub-connectors, by payment provider.
const PAYMENT_PROVIDERS = Object.freeze({
    [stripePayments.PAYMENT_PROVIDER]: stripePayments,
});

// Builds the Moesif management API client; tests replace it with their own.
const clients = {
    moesif: (managementKey) => client.createMoesifClient({ managementKey }),
};

class UnsupportedPaymentProviderError extends Error {}

/** The payment sub-connector module for a payment provider. */
function paymentModule(paymentProvider) {
    const module = PAYMENT_PROVIDERS[paymentProvider];
    if (!module) throw new UnsupportedPaymentProviderError(`Moesif billing through ${paymentProvider} is not supported`);
    return module;
}

/**
 * The connector for one billing configuration. `conn` carries the decrypted settings:
 * moesifCollectorApplicationId, moesifManagementKey, payment_provider and the payment provider's own
 * settings (for Stripe, stripeApiKey).
 */
function create(conn) {
    const moesif = clients.moesif(conn.moesifManagementKey);
    const payments = paymentModule(conn.payment_provider).create(conn);
    const slug = payments.moesifProviderSlug;

    return {
        provider: PROVIDER,
        payments,

        // ── connection ─────────────────────────────────────────────────────────

        /**
         * Checks the Moesif key and the payment provider's key together. Resolves to the payment
         * provider's event endpoints; rejects with { failures } listing each refusal.
         */
        async checkKeys() {
            const [moesifResult, paymentResult] = await Promise.allSettled([
                moesif.listCatalogPlans(slug),
                payments.checkKey(),
            ]);
            const failures = [moesifResult, paymentResult].filter((r) => r.status === 'rejected').map((r) => r.reason);
            if (failures.length > 0) throw Object.assign(new Error(failures.map((f) => f.message).join(' ')), { failures });
            return paymentResult.value;
        },

        /**
         * Makes sure the payment provider sends Moesif what it needs to match usage to customers
         * and subscriptions, at Moesif's endpoint for this application. Resolves to { ref, created,
         * undo }: undo removes an endpoint created here, or puts back one that was changed.
         */
        ensureMoesifEvents(endpoints) {
            return payments.ensureMoesifEndpoint({ url: client.stripeWebhookUrl(conn.moesifCollectorApplicationId), endpoints });
        },

        /**
         * Removes Moesif's endpoint for this application from the payment provider, found by its
         * address. Resolves to { undo }: undo registers it again, or is null when there was none.
         */
        async removeMoesifEvents() {
            const url = client.stripeWebhookUrl(conn.moesifCollectorApplicationId);
            const found = (await payments.listEventEndpoints()).filter((e) => e.url === url);
            for (const endpoint of found) await payments.removeEventEndpoint(endpoint.id);
            return { undo: found.length > 0 ? () => payments.ensureMoesifEndpoint({ url, endpoints: [] }) : null };
        },

        /**
         * Connects the Moesif application to the payment provider with this configuration's key,
         * replacing any connection. Resolves to { undo }: undo removes the connection.
         */
        async connectPayments() {
            await moesif.savePartnerAccount(slug, payments.moesifPartnerAccount(true));
            return { undo: () => moesif.deletePartnerAccount(slug) };
        },

        /**
         * Removes the Moesif application's connection to the payment provider, with the key it
         * holds. Resolves to { undo }: undo saves the connection again as it was (on or off), or is
         * null when there was no connection to remove.
         */
        async disconnectPayments() {
            const before = await moesif.getPartnerAccount(slug);
            if (!before || !before.partner_account) return { undo: null };
            const wasOn = payments.moesifPartnerAccountEnabled(before);
            await moesif.deletePartnerAccount(slug);
            return { undo: () => moesif.savePartnerAccount(slug, payments.moesifPartnerAccount(wasOn)) };
        },

        // ── billing catalog ────────────────────────────────────────────────────

        catalog: {
            /**
             * Throws CatalogInputError when a price's meter (catalog.priceMeter: the meter with the
             * price's values) cannot be counted.
             */
            checkMeter(meter) {
                catalog.meterQuery(meter);
            },

            /** A catalog plan, named `name`; resolves to its reference. */
            async createPlan({ name }) {
                return (await moesif.createCatalogPlan(slug, catalog.catalogPlanPayload(slug, name))).id;
            },

            /** A flat rate plan's price, in the catalog plan `planRef`; resolves to its reference. */
            async createFlatPrice({ planRef, name, pricing }) {
                return (await moesif.createCatalogPrice(slug, catalog.flatPricePayload(slug, planRef, name, pricing))).id;
            },

            /** A usage price of a plan's pricing, in the catalog plan `planRef`; resolves to its reference. */
            async createUsagePrice({ planRef, name, pricing, price }) {
                return (await moesif.createCatalogPrice(slug,
                    catalog.usagePricePayload(slug, planRef, name, pricing, price))).id;
            },

            /**
             * For a usage price: the Moesif billing meter that counts what the price's meter
             * (`meter`, from catalog.priceMeter) counts and reports it to the price; resolves to its
             * reference.
             */
            async createUsageMeter({ name, meter, planRef, priceRef }) {
                const created = await moesif.createBillingMeter(catalog.billingMeterPayload({
                    provider: slug,
                    providerParams: payments.moesifBillingPlanParams({
                        productRef: planRef, productName: name, priceRef,
                        reportingPeriod: catalog.REPORTING_PERIOD,
                    }),
                    name, meter,
                }));
                return created.id;
            },

            /** Undo: removes a billing meter this connector created. */
            removeUsageMeter(ref) {
                return moesif.deleteBillingMeter(ref);
            },

            /** Undo: removes a price this connector created. */
            removePrice(ref) {
                return moesif.deleteCatalogPrice(slug, ref);
            },

            /** Undo: archives a catalog plan this connector created (Stripe keeps a product with prices). */
            removePlan(ref) {
                return moesif.archiveCatalogPlan(slug, ref);
            },

            /**
             * Takes one part of an API plan off sale ({ planRef, priceRef, meterRef }): its billing
             * meter is deleted, then its price and its plan are archived.
             */
            async archivePart({ planRef, priceRef, meterRef }) {
                if (meterRef) await moesif.deleteBillingMeter(meterRef);
                if (priceRef) await moesif.archiveCatalogPrice(slug, priceRef);
                if (planRef) await moesif.archiveCatalogPlan(slug, planRef);
            },

            /** What a price counts with: its meter, narrowed by the price's filters. */
            priceMeter: catalog.priceMeter,
        },

        // ── usage ──────────────────────────────────────────────────────────────

        usage: {
            /**
             * A subscription's usage of one price's meter (`meter`, from catalog.priceMeter) per
             * hour, day or week (`interval`: HOUR, DAY, WEEK), counted as the price's billing meter
             * counts it, in UTC. Resolves to [{ start, value }] for the buckets Moesif has; throws
             * CatalogInputError for a meter that cannot be counted.
             */
            async series(meter, { customerRef, subscriptionRef }, { from, to, interval }) {
                const body = catalog.usageSearch(meter, { customerId: customerRef, subscriptionId: subscriptionRef }, interval);
                const response = await moesif.searchEvents({ from: from.toISOString(), to: to.toISOString(), body });
                return catalog.usageBuckets(response);
            },
        },
    };
}

/**
 * The payment sub-connector module for a billing configuration, for the parts that need no keys
 * (reading events, key modes).
 */
function paymentsFor(paymentProvider) {
    return paymentModule(paymentProvider);
}

// An application as Moesif names it in its ids and keys: organization number, application number.
const APPLICATION = /^\d+:\d+$/;

/**
 * Checks the Moesif settings of a setup request before any call. The application ID must be a
 * Collector Application ID, since Moesif's Stripe webhook address carries it, and for the same
 * application as the management key. Returns { field, message } for a problem, or null.
 */
function checkSettings({ moesifCollectorApplicationId, moesifManagementKey }) {
    const app = client.collectorApplication(moesifCollectorApplicationId);
    if (!app) {
        return { field: 'moesifCollectorApplicationId',
            message: 'This is not a Collector Application Id. Copy it from the API Keys page in Moesif.' };
    }
    const keyApp = client.keyApplication(moesifManagementKey);
    if (keyApp && APPLICATION.test(app) && APPLICATION.test(keyApp) && app !== keyApp) {
        return { field: 'moesifCollectorApplicationId',
            message: 'This Collector Application Id is for a different Moesif application than the Management API key.' };
    }
    return null;
}

module.exports = {
    PROVIDER,
    checkSettings,
    PAYMENT_PROVIDERS,
    CatalogInputError: catalog.CatalogInputError,
    UnsupportedPaymentProviderError,
    clients,
    create,
    paymentsFor,
};
