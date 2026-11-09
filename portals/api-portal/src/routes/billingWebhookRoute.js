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
 * The payment event endpoint: POST <base path>/billing/stripe/webhooks/<organization id>. Billing
 * setup registers this URL with Stripe (through the Moesif connector's Stripe sub-connector), and
 * Stripe sends its payment events here.
 *
 * Mounted at app level, ahead of the JSON and form parsers, the session and CSRF: the provider
 * signs the exact bytes it sends, so the body must reach the handler unparsed, and the provider is
 * not a browser session. The signature is the authentication.
 */

const express = require('express');
const constants = require('../utils/constants');
const { receivePaymentEvent } = require('../services/billing/paymentWebhookService');

// Payment events are small; a body far larger than any of them is refused before it is read.
const MAX_EVENT_BYTES = 1048576;

const PATH = `${constants.ROUTE.BASE_PATH}/billing/stripe/webhooks/:orgId`;

function mount(app) {
    app.post(PATH, express.raw({ type: () => true, limit: MAX_EVENT_BYTES }), receivePaymentEvent);
}

module.exports = { mount, PATH };
