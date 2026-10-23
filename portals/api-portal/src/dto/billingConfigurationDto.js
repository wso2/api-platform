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
const { applyAudit } = require('./dtoUtils');

class BillingConfiguration {
    /**
     * @param {object} conn  a billingConfigurationDao public row (never the keys)
     * @param {object} extra { webhookEndpointUrl, webhookBaseUrl, mode }
     * @param {object} [audit]
     */
    constructor(conn, extra, audit) {
        this.id = conn.uuid;
        this.provider = conn.provider;
        this.paymentProvider = conn.payment_provider;
        // Read from the payment provider's publishable key: a test key means test mode.
        this.mode = extra.mode;
        this.webhookEndpointUrl = extra.webhookEndpointUrl;
        this.webhookBaseUrl = extra.webhookBaseUrl;
        this.webhookRegistered = Number(conn.has_webhook_secret) === 1;
        // Whether setup created Moesif's endpoint in Stripe (one that was there before is not the portal's).
        this.moesifWebhookRegistered = Boolean(conn.moesif_webhook_endpoint_id);
        this.lastEventAt = conn.last_event_at || null;
        applyAudit(this, audit);
    }
}

module.exports = BillingConfiguration;
