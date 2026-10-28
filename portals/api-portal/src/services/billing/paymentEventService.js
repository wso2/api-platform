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
 * Acts on a payment event sent by the payment provider (see paymentWebhookService.js).
 *
 * The event's own copy of the state is not trusted: the current state is read from the provider
 * through the billing provider interface. The provider can send an event more than once, and sends
 * it again when the portal fails to act on it, so acting on the same event again, or on events out
 * of order, still leaves the right state.
 *
 * The payment sub-connector says what an event is about:
 *   - CHECKOUT: activates or ends the checkout.
 *   - SUBSCRIPTION or INVOICE: updates the paid subscription's billing status and tells the
 *     Platform API (subscription.billing_updated), which tells the gateways. Access stops at UNPAID
 *     or CANCELLED, that is, once the provider has stopped retrying the payment; PAST_DUE is still
 *     served.
 *   - Anything else is ignored.
 */

const db = require('../../db/driver');
const billingConfigurationDao = require('../../dao/billingConfigurationDao');
const billingCheckoutDao = require('../../dao/billingCheckoutDao');
const billingSubscriptionDao = require('../../dao/billingSubscriptionDao');
const billingCheckoutService = require('./checkoutService');
const subDao = require('../../dao/subscriptionDao');
const subscriptionService = require('../subscriptionService');
const billingProvider = require('../../billing/billingProvider');
const logger = require('../../config/logger');

/** The result of acting on an event: PROCESSED, or IGNORED when it is not about anything the portal holds. */
const OUTCOME = Object.freeze({ PROCESSED: 'PROCESSED', IGNORED: 'IGNORED' });

/**
 * Brings a paid subscription's billing state up to date with the payment provider and, when the
 * status changed, tells the Platform API.
 *   - A subscription the provider ended on request (the subscriber unsubscribed, or an
 *     administrator cancelled it in the provider) is removed, as unsubscribing removes it.
 *   - One the provider ended after failed payments is kept, with payment pending.
 *
 * Resolves to false when the portal has no such paid subscription, including one not activated
 * yet (activation reads the status itself).
 */
async function refreshSubscription(payments, subscriptionRef) {
    const billing = await billingSubscriptionDao.getBySubscriptionRef(subscriptionRef);
    if (!billing) return false;
    const providerSub = await payments.getSubscription(subscriptionRef);
    if (!providerSub) return false;
    const billingStatus = providerSub.status;
    const changed = billingStatus !== billing.status;
    const state = {
        status: billingStatus,
        currentPeriodStart: providerSub.periodStart,
        currentPeriodEnd: providerSub.periodEnd,
        endsAt: providerSub.endsAt,
    };
    const sub = await subDao.getById(billing.org_uuid, billing.subscription_uuid);
    if (!sub) return false;
    if (providerSub.ended && providerSub.endRequested) {
        await db.withTransaction((t) => subscriptionService.removeSubscription(billing.org_uuid, sub, null, t));
        logger.info('Paid subscription ended and removed', { subscriptionId: billing.subscription_uuid });
        return true;
    }
    await db.withTransaction(async (t) => {
        await billingSubscriptionDao.updateState(billing.subscription_uuid, state, t);
        if (changed) {
            await subscriptionService.publishBillingUpdated(billing.org_uuid, sub, {
                status: billingStatus,
                customer_id: billing.customer_ref,
                subscription_id: subscriptionRef,
            }, t);
        }
    });
    if (changed) {
        logger.info('Paid subscription billing status changed', {
            subscriptionId: billing.subscription_uuid, from: billing.status, to: billingStatus,
        });
    }
    return true;
}

/**
 * Acts on one event of the organization's payment provider: { orgId, type, objectRef }. Resolves to an
 * OUTCOME; throws when acting on it failed.
 */
async function handle(event) {
    const conn = await billingConfigurationDao.getWithSecrets(event.orgId);
    if (!conn) return OUTCOME.IGNORED;
    const { payments } = billingProvider.forConfiguration(conn);

    switch (billingProvider.paymentsOf(conn).eventSubject(event.type)) {
        case 'CHECKOUT': {
            const providerCheckout = await payments.getCheckout(event.objectRef);
            const checkout = providerCheckout.reference
                ? await billingCheckoutDao.getCheckout(providerCheckout.reference) : null;
            if (!checkout || checkout.checkout_ref !== providerCheckout.ref) return OUTCOME.IGNORED;
            await billingCheckoutService.settle(checkout, providerCheckout);
            return OUTCOME.PROCESSED;
        }
        case 'SUBSCRIPTION':
            return (await refreshSubscription(payments, event.objectRef)) ? OUTCOME.PROCESSED : OUTCOME.IGNORED;
        case 'INVOICE': {
            const subscriptionRef = await payments.subscriptionOfInvoice(event.objectRef);
            if (!subscriptionRef) return OUTCOME.IGNORED;
            return (await refreshSubscription(payments, subscriptionRef)) ? OUTCOME.PROCESSED : OUTCOME.IGNORED;
        }
        default:
            return OUTCOME.IGNORED;
    }
}

module.exports = { OUTCOME, handle, refreshSubscription };
