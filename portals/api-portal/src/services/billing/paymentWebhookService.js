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
 * Receives payment events (webhooks) from a billing configuration's payment provider.
 *
 * The URL names the billing configuration. It gives the signing secret, the organization the event
 * belongs to, and the payment sub-connector that reads the event. Then:
 *   1. An unknown configuration, a bad signature or an unreadable body is refused.
 *   2. The event is acted on while the provider waits (paymentEventService), and logged.
 *   3. When acting on it fails, the answer is 500 and the provider sends the event again later.
 *
 * Acting on an event reads the current state from the provider, so a repeated or late event does
 * no harm. The portal keeps only the time of the last event, not the events.
 */

const billingConfigurationDao = require('../../dao/billingConfigurationDao');
const billingProvider = require('../../billing/billingProvider');
const paymentEventService = require('./paymentEventService');
const logger = require('../../config/logger');
const { config } = require('../../config/configLoader');

function reject(res, status) {
    // One body for every refusal; only the status differs. The reason is logged, not sent to the caller.
    return res.status(status).json({ error: 'rejected' });
}

const receivePaymentEvent = async (req, res) => {
    const { configurationId } = req.params;
    let event = null;
    try {
        const conn = /^[0-9a-f-]{36}$/.test(configurationId)
            ? await billingConfigurationDao.getWebhookSecret(configurationId) : null;
        if (!conn || !conn.secret) {
            logger.warn('Payment event for an unknown billing configuration', { configurationId });
            return reject(res, 404);
        }
        const rawBody = Buffer.isBuffer(req.body) ? req.body : Buffer.alloc(0);
        event = billingProvider.paymentsOf(conn).readEvent(conn.secret, rawBody, (name) => req.get(name),
            config.billing.stripe.webhookSignatureToleranceSec);
        if (!event) {
            logger.warn('Payment event with an invalid signature or body', { configurationId });
            return reject(res, 400);
        }
        await billingConfigurationDao.touchEvent(conn.uuid, new Date());
        const outcome = await paymentEventService.handle({
            orgId: conn.orgId, configurationUuid: conn.uuid, type: event.type, objectRef: event.objectRef,
        });
        logger.info('Payment event handled', {
            configurationId, eventId: event.id, type: event.type, objectId: event.objectRef, outcome,
        });
        return res.status(200).json({ received: true });
    } catch (error) {
        // A 500 makes the provider send the event again later.
        logger.error('Error while handling a payment event', {
            configurationId, eventId: event && event.id, type: event && event.type, error: error.message,
        });
        return reject(res, 500);
    }
};

module.exports = { receivePaymentEvent };
