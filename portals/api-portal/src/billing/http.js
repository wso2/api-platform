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
 * The HTTP client the billing providers share: the portal's SSRF-guarded client (the one key
 * managers use), bounded by [api_portal.billing.client], and an error type that carries what a
 * provider said without ever carrying the key that was sent.
 */

const { buildClient } = require('../keymanagers/core/httpClient');

function billingConfig() {
    return require('../config/configLoader').config.billing || {};
}

function createHttpClient() {
    // Stripe and Moesif are public services, so private addresses are refused unless the
    // deployment allows them ([api_portal.billing.client] allow_private_endpoints).
    const client = billingConfig().client || {};
    return buildClient({ allowPrivateEndpoints: client.allowPrivateEndpoints === true, limits: client });
}

/**
 * A provider refused a call, or could not be reached. `message` is the provider's own explanation,
 * with secrets redacted, or a description of the network failure; it is for the logs, and
 * administrators are shown the portal's own wording. `status` is the provider's HTTP status, or 0
 * when there was no response.
 */
class BillingProviderError extends Error {
    constructor(provider, status, message, code) {
        super(message);
        this.name = 'BillingProviderError';
        this.provider = provider;
        this.status = status;
        this.code = code || null;
    }

    /** The key was rejected: wrong, revoked, or lacking a permission the call needs. */
    get isAuthError() {
        return this.status === 401 || this.status === 403;
    }
}

/** A paging cursor the payment provider does not know (not from one of its own pages). */
class BillingCursorError extends Error {
    constructor(provider, message) {
        super(message);
        this.name = 'BillingCursorError';
        this.provider = provider;
    }
}

// Whole secrets a provider's text might hold: Stripe keys and webhook secrets, and the base64 JSON
// tokens Moesif's management keys and application ids are. A masked fragment such as Stripe's
// "sk_test_****1234" is not whole and stays.
const SECRET_SHAPES = [
    /\b(?:sk|rk|pk)_(?:test|live)_[A-Za-z0-9]{8,}/g,
    /\bwhsec_[A-Za-z0-9]{8,}/g,
    /\beyJ[A-Za-z0-9_-]{16,}(?:\.[A-Za-z0-9_-]+)*/g,
];

/**
 * A provider's text without secrets: each of `secrets` (the key the call was made with) and anything shaped like a
 * whole key are replaced by [REDACTED]. Provider messages are logged, and how a provider words them can change.
 */
function redactSecrets(text, secrets = []) {
    let out = String(text);
    for (const secret of secrets) {
        if (typeof secret === 'string' && secret.length >= 8) out = out.split(secret).join('[REDACTED]');
    }
    return SECRET_SHAPES.reduce((acc, shape) => acc.replace(shape, '[REDACTED]'), out);
}

/** Wraps a transport failure (timeout, DNS, refused connection, blocked address). */
function networkError(provider, err) {
    const reason = err && err.code === 'ECONNABORTED' ? 'the request timed out' : 'the service could not be reached';
    return new BillingProviderError(provider, 0, `${provider}: ${reason}.`);
}

module.exports = { billingConfig, createHttpClient, BillingProviderError,
    BillingCursorError, networkError, redactSecrets };
