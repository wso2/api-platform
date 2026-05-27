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
/* eslint-disable no-undef */
const crypto = require('crypto');
const { config } = require('../config/configLoader');

const CSRF_HMAC_LABEL = 'devportal-platform-api-keys-csrf';

function ensureCsrfSecret(req) {
    if (!req.session) {
        return null;
    }
    if (!req.session.csrfSecret) {
        req.session.csrfSecret = crypto.randomBytes(32).toString('hex');
    }
    return req.session.csrfSecret;
}

/**
 * Per-session token for double-submit CSRF checks on browser cookie sessions.
 * Call when rendering pages that POST via fetch with credentials.
 */
function getSessionCsrfToken(req) {
    const secret = ensureCsrfSecret(req);
    if (!secret) {
        return '';
    }
    return crypto.createHmac('sha256', secret).update(CSRF_HMAC_LABEL).digest('base64url');
}

function hasBearerAuthorization(req) {
    const a = req.headers.authorization;
    return typeof a === 'string' && a.length > 6 && a.toLowerCase().startsWith('bearer ');
}

function hasConfiguredApiKey(req) {
    if (!config.advanced?.apiKey?.enabled || !config.advanced.apiKey.keyType) {
        return false;
    }
    const keyType = config.advanced.apiKey.keyType;
    const lower = keyType.toLowerCase();
    return Boolean(req.headers[lower] || req.headers[keyType]);
}

function hasMTLSClient(req) {
    try {
        const cert = req.socket?.getPeerCertificate?.(true);
        return Boolean(cert && Object.keys(cert).length > 0 && req.client?.authorized);
    } catch {
        return false;
    }
}

function timingSafeCompare(a, b) {
    if (typeof a !== 'string' || typeof b !== 'string') {
        return false;
    }
    const bufA = Buffer.from(a, 'utf8');
    const bufB = Buffer.from(b, 'utf8');
    if (bufA.length !== bufB.length) {
        return false;
    }
    return crypto.timingSafeEqual(bufA, bufB);
}

/**
 * For mutating /devportal routes: cookie-based sessions must send X-CSRF-Token
 * matching getSessionCsrfToken. Skips when auth matches non-browser paths used by
 * enforceSecurity (Bearer, API key, mTLS).
 */
function requireCsrfForMutatingApi(req, res, next) {
    if (hasBearerAuthorization(req)) {
        return next();
    }
    if (hasConfiguredApiKey(req)) {
        return next();
    }
    if (hasMTLSClient(req)) {
        return next();
    }
    const expected = getSessionCsrfToken(req);
    const sent =
        req.headers['x-csrf-token'] ||
        req.headers['csrf-token'] ||
        (req.body && (req.body._csrf || req.body.csrfToken));
    if (
        !expected ||
        sent == null ||
        sent === '' ||
        typeof sent !== 'string' ||
        !timingSafeCompare(sent, expected)
    ) {
        return res.status(403).json({
            description: 'CSRF token missing or invalid',
            message: 'CSRF token missing or invalid'
        });
    }
    return next();
}

module.exports = {
    getSessionCsrfToken,
    requireCsrfForMutatingApi
};
