// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// Test-side mirror of src/services/webhooks/envelopeCrypto.js's decryptField.
// Can't require the app source directly: the rest-api-tests container only has
// `it/rest-api` mounted (docker-compose.test*.yaml), not the rest of the repo. Kept in
// lockstep with the app's encryptField — AES-256-GCM under an HKDF-SHA3-256 key derived
// from the subscriber's shared secret, base64-encoded fields.
//
// FIELD_KEY_INFO must stay byte-identical to the app's label and to platform-api's
// (internal/webhook/decryptor.go, fieldKeyInfo) — a mismatch yields a different key.

const crypto = require('crypto');

const FIELD_KEY_INFO = 'api-portal-webhook-field-encryption-v1';

function deriveFieldKey(secret) {
    return Buffer.from(
        crypto.hkdfSync('sha3-256', secret, Buffer.alloc(0), FIELD_KEY_INFO, 32)
    );
}

function decryptField(secret, envelope) {
    const decipher = crypto.createDecipheriv(
        'aes-256-gcm', deriveFieldKey(secret), Buffer.from(envelope.iv, 'base64')
    );
    decipher.setAuthTag(Buffer.from(envelope.tag, 'base64'));
    return Buffer.concat([
        decipher.update(Buffer.from(envelope.ciphertext, 'base64')),
        decipher.final()
    ]).toString('utf8');
}

module.exports = { deriveFieldKey, decryptField, FIELD_KEY_INFO };
