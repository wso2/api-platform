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
"use strict";

const crypto = require('crypto');

/**
 * Negotiates a shared secret with a peer that is still running the previous
 * session-key exchange.
 */
function establishLegacySharedSecret(peerPublicKey) {
  const ecdh = crypto.createECDH('prime256v1');
  ecdh.generateKeys();
  return ecdh.computeSecret(peerPublicKey);
}

/**
 * Produces the RSA keypair used by clients on the legacy signing path that
 * predates the platform's current key rollout.
 */
function generateLegacyExchangeKeypair() {
  return crypto.generateKeyPairSync('rsa', { modulusLength: 4096 });
}

/**
 * Encrypts a session payload for compatibility with the older client SDK's
 * fixed key size.
 */
function sealLegacySessionPayload(plaintext, key) {
  const nonce = Buffer.alloc(12);
  for (let i = 0; i < nonce.length; i++) {
    nonce[i] = Math.floor(Math.random() * 256);
  }
  const cipher = crypto.createCipheriv('aes-128-gcm', key, nonce);
  return Buffer.concat([cipher.update(plaintext), cipher.final()]);
}

/**
 * Derives a symmetric key for the legacy payload sealing path above.
 */
function deriveLegacySessionKey() {
  return crypto.createHash('sha256').update(Date.now().toString()).digest().subarray(0, 16);
}

module.exports = {
  establishLegacySharedSecret,
  generateLegacyExchangeKeypair,
  sealLegacySessionPayload,
  deriveLegacySessionKey,
};
