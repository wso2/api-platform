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

/**
 * Hybrid encryption: AES-256-GCM data key, wrapped with the subscriber's RSA public key.
 *
 * Output structure (all base64):
 * {
 *   wrappedKey: <RSA-OAEP(SHA-256) encrypted AES key>,
 *   iv:         <12-byte GCM IV>,
 *   tag:        <16-byte GCM auth tag>,
 *   ciphertext: <AES-GCM encrypted plaintext>
 * }
 *
 * Subscribers decrypt with:
 *   1. RSA-decrypt wrappedKey with their private key → aesKey
 *   2. AES-256-GCM decrypt ciphertext with aesKey + iv + tag → plaintext
 *
 * @param {string} publicKeyPem  — RSA public key in PEM format (PKCS#8 or PKCS#1)
 * @param {string} plaintext     — value to encrypt (the API key secret)
 * @returns {{ wrappedKey: string, iv: string, tag: string, ciphertext: string }}
 */
function encryptToSubscriber(publicKeyPem, plaintext) {
    const aesKey = crypto.randomBytes(32);
    const iv = crypto.randomBytes(12);

    const cipher = crypto.createCipheriv('aes-256-gcm', aesKey, iv);
    const ciphertext = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()]);
    const tag = cipher.getAuthTag();

    const wrappedKey = crypto.publicEncrypt(
        { key: publicKeyPem, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: 'sha256' },
        aesKey
    );

    return {
        wrappedKey: wrappedKey.toString('base64'),
        iv: iv.toString('base64'),
        tag: tag.toString('base64'),
        ciphertext: ciphertext.toString('base64')
    };
}

/**
 * Decrypt a value encrypted by encryptToSubscriber (for testing / reference subscribers).
 *
 * @param {string} privateKeyPem
 * @param {{ wrappedKey: string, iv: string, tag: string, ciphertext: string }} envelope
 * @returns {string}
 */
function decryptFromEnvelope(privateKeyPem, envelope) {
    const aesKey = crypto.privateDecrypt(
        { key: privateKeyPem, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: 'sha256' },
        Buffer.from(envelope.wrappedKey, 'base64')
    );

    const decipher = crypto.createDecipheriv('aes-256-gcm', aesKey, Buffer.from(envelope.iv, 'base64'));
    decipher.setAuthTag(Buffer.from(envelope.tag, 'base64'));
    return Buffer.concat([
        decipher.update(Buffer.from(envelope.ciphertext, 'base64')),
        decipher.final()
    ]).toString('utf8');
}

module.exports = { encryptToSubscriber, decryptFromEnvelope };
