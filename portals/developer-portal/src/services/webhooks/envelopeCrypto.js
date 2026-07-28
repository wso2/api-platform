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

// Versioned HKDF info label. The subscriber's shared secret is used for two
// distinct purposes — HMAC request signing (signer.js) and the field encryption
// below — so each derives its own key from that secret rather than using the
// secret material directly. Bump the version suffix if the scheme changes; the
// label must stay byte-identical to the receiver's (platform-api
// internal/webhook/decryptor.go).
const FIELD_KEY_INFO = 'devportal-webhook-field-encryption-v1';

const KEY_BYTES = 32; // AES-256
const IV_BYTES = 12; // 96-bit GCM nonce

/**
 * Derive the AES-256 field-encryption key from a subscriber's shared secret.
 *
 * The secret is an operator-supplied free-form string, not key material, so it
 * is stretched through HKDF-SHA256 rather than hashed or truncated. An empty
 * salt is intentional and RFC 5869 compliant (extract falls back to HashLen
 * zero bytes) — it keeps derivation reproducible on the receiving side, which
 * holds the same secret but shares no other state with this process.
 *
 * @param {string} secret — the subscriber's shared HMAC/encryption secret
 * @returns {Buffer} 32-byte AES key
 */
function deriveFieldKey(secret) {
    if (!secret) {
        throw new Error('A subscriber secret is required to derive the field-encryption key');
    }
    return Buffer.from(
        crypto.hkdfSync('sha256', secret, Buffer.alloc(0), FIELD_KEY_INFO, KEY_BYTES)
    );
}

/**
 * Encrypt a sensitive event field with AES-256-GCM under a key derived from the
 * subscriber's shared secret.
 *
 * Output structure (all base64):
 * {
 *   iv:         <12-byte GCM nonce>,
 *   tag:        <16-byte GCM auth tag>,
 *   ciphertext: <AES-256-GCM encrypted plaintext>
 * }
 *
 * Subscribers decrypt with:
 *   1. HKDF-SHA256(secret, info="devportal-webhook-field-encryption-v1") → aesKey
 *   2. AES-256-GCM decrypt ciphertext with aesKey + iv + tag → plaintext
 *
 * @param {string} secret    — the subscriber's shared secret
 * @param {string} plaintext — value to encrypt (e.g. the API key secret)
 * @returns {{ iv: string, tag: string, ciphertext: string }}
 */
function encryptField(secret, plaintext) {
    const key = deriveFieldKey(secret);
    // Fresh nonce per field per subscriber — never reused under the same derived key.
    const iv = crypto.randomBytes(IV_BYTES);

    const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
    const ciphertext = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()]);
    const tag = cipher.getAuthTag();

    return {
        iv: iv.toString('base64'),
        tag: tag.toString('base64'),
        ciphertext: ciphertext.toString('base64')
    };
}

/**
 * Decrypt a value produced by encryptField (for testing / reference subscribers).
 *
 * @param {string} secret
 * @param {{ iv: string, tag: string, ciphertext: string }} envelope
 * @returns {string}
 */
function decryptField(secret, envelope) {
    const key = deriveFieldKey(secret);
    const decipher = crypto.createDecipheriv('aes-256-gcm', key, Buffer.from(envelope.iv, 'base64'));
    decipher.setAuthTag(Buffer.from(envelope.tag, 'base64'));
    return Buffer.concat([
        decipher.update(Buffer.from(envelope.ciphertext, 'base64')),
        decipher.final()
    ]).toString('utf8');
}

module.exports = { deriveFieldKey, encryptField, decryptField, FIELD_KEY_INFO };
