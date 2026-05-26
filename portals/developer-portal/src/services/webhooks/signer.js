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
const crypto = require('crypto');
const { config } = require('../../config/configLoader');

/**
 * Build the X-Devportal-Signature header value.
 * Canonical string: "<unix_seconds>.<raw_body>"
 *
 * @param {string} secret  — per-subscriber HMAC secret
 * @param {string} rawBody — serialised JSON string of the outgoing payload
 * @param {number} [ts]    — unix seconds; defaults to now
 * @returns {{ header: string, ts: number }}
 */
function sign(secret, rawBody, ts) {
    const t = ts || Math.floor(Date.now() / 1000);
    const hmac = crypto.createHmac('sha256', secret);
    hmac.update(`${t}.${rawBody}`);
    const hex = hmac.digest('hex');
    return { header: `t=${t},v1=${hex}`, ts: t };
}

/**
 * Verify an inbound X-Devportal-Signature header (for testing / reference subscribers).
 * Returns true if the signature is valid and within the tolerance window.
 *
 * @param {string} secret
 * @param {string} rawBody
 * @param {string} signatureHeader
 * @param {number} [toleranceSec]
 */
function verify(secret, rawBody, signatureHeader, toleranceSec) {
    const delivery = config.webhooks && config.webhooks.delivery;
    const tol = (toleranceSec !== undefined && toleranceSec !== null) ? toleranceSec : ((delivery && delivery.signatureToleranceSec) || 300);
    const parts = {};
    for (const part of (signatureHeader || '').split(',')) {
        const [k, v] = part.split('=');
        if (k && v) parts[k] = v;
    }
    const t = parseInt(parts.t, 10);
    if (!t || !parts.v1) return false;
    if (Math.abs(Math.floor(Date.now() / 1000) - t) > tol) return false;
    const { header } = sign(secret, rawBody, t);
    const expected = Buffer.from(`t=${t},v1=${header.split('v1=')[1]}`);
    const actual = Buffer.from(`t=${t},v1=${parts.v1}`);
    if (expected.length !== actual.length) return false;
    return crypto.timingSafeEqual(expected, actual);
}

module.exports = { sign, verify };
