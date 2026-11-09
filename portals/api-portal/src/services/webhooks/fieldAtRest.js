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
 * At-rest sealing of a secret event's sensitive field values (API key / subscription token
 * values) between publish and delivery. The values are encrypted under the portal's own
 * encryption key (security.encryptionKey), never under a subscriber secret: the delivery worker
 * opens them and re-encrypts under the subscriber's *current* secret at send time, so a
 * secret rotation between publish and delivery cannot strand a pending delivery.
 */

const { config } = require('../../config/configLoader');
const { createCryptoUtil } = require('../../utils/cryptoUtil');

// Reserved name in a delivery's stored encrypted_fields marking its values as sealed at rest.
// It is not a payload field: the worker removes it before the delivery is sent.
const SEALED_AT_REST_KEY = '__sealed_at_rest';
const SEALED_AT_REST_VERSION = 'v1';

let cached = null;
function atRestCrypto() {
    const key = config.security && config.security.encryptionKey;
    if (!cached || cached.key !== key) cached = { key, crypto: createCryptoUtil(key) };
    return cached.crypto;
}

// The field name is bound to the ciphertext (AES-GCM AAD), so a value cannot be moved to another field.
const context = (fieldName) => `webhook-field:${fieldName}`;

/** Encrypt a field value for storage. Throws when the portal encryption key is not configured. */
function sealAtRest(fieldName, plaintext) {
    return atRestCrypto().encrypt(plaintext, context(fieldName));
}

/** Decrypt a value written by sealAtRest. */
function openAtRest(fieldName, sealed) {
    return atRestCrypto().decrypt(sealed, context(fieldName));
}

module.exports = { sealAtRest, openAtRest, SEALED_AT_REST_KEY, SEALED_AT_REST_VERSION };
