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
 * The billing provider interface: the common functions the billing module (billing settings,
 * billing catalog, checkout and payment events, usage and revenue reports) calls for every billing
 * provider. The billing module calls only these; it never calls a provider's API itself.
 *
 * A billing configuration names its connector (`billing_provider`) and, for a connector that bills
 * through another provider, its payment provider (`payment_provider`). forConfiguration(conn) returns:
 *
 *   provider             the connector's name
 *   checkKeys()          checks the configuration's keys
 *   catalog              createPlan, createFlatPrice, createUsagePrice, createUsageMeter, checkMeter,
 *                        priceMeter, the remove functions that undo them, and archivePart, which
 *                        takes a part of an API plan off sale
 *   usage                series(meter, { customerRef, subscriptionRef }, { from, to, interval })
 *   payments             the payment functions:
 *                          createCustomer, startCheckout, getCheckout, getSubscription,
 *                          endAtPeriodEnd, cancelSubscription, billingPageUrl, estimate, invoices,
 *                          invoicePage, customerInvoicePage,
 *                          subscriptionOfInvoice, and the event endpoint functions
 *
 * References (customerRef, subscriptionRef, priceRef, ...) are the provider's own ids, kept by the
 * portal as they are. Amounts are integers in the currency's minor unit. Statuses are the portal's
 * billing statuses: ACTIVE, PAST_DUE, UNPAID, CANCELLED.
 *
 * Connectors:
 *   MOESIF   the Moesif connector: Moesif meters usage and owns the catalog, which it syncs to the
 *            billing provider; a payment sub-connector per provider (Stripe) creates customers
 *            and subscriptions and receives payment events.
 * Another connector implements this interface and is added to CONNECTORS.
 */

const moesif = require('./connectors/moesif');

// The connectors, by the name a billing configuration gives as its billing provider.
const CONNECTORS = Object.freeze({
    [moesif.PROVIDER]: moesif,
});

class UnsupportedProviderError extends Error {}

function connectorModule(provider) {
    const module = CONNECTORS[provider];
    if (!module) throw new UnsupportedProviderError(`The billing provider ${provider} is not supported`);
    return module;
}

/** The connector for a billing configuration with its decrypted settings. */
function forConfiguration(conn) {
    return connectorModule(conn.billing_provider).create(conn);
}

/**
 * The payment provider (such as Stripe) of a billing configuration, without its stored keys. Used to
 * tell whether a key is a test or live key, and to read a received payment event.
 */
function paymentsOf({ billing_provider: billingProvider, payment_provider: paymentProvider }) {
    return connectorModule(billingProvider).paymentsFor(paymentProvider);
}

/** A connector's own checks of a setup request's settings: { field, message } for a problem, or null. */
function checkSettings(setup, input) {
    const module = connectorModule(setup.billing_provider);
    return module.checkSettings ? module.checkSettings(input) : null;
}

module.exports = {
    CONNECTORS,
    checkSettings,
    CatalogInputError: moesif.CatalogInputError,
    UnsupportedProviderError,
    forConfiguration,
    paymentsOf,
};
