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

// Test-side mirror of src/services/webhooks/envelopeCrypto.js's decryptFromEnvelope.
// Can't require the app source directly: the rest-api-tests container only has
// `it/rest-api` mounted (docker-compose.test*.yaml), not the rest of the repo. Kept in
// lockstep with the app's encryptToSubscriber — RSA-OAEP(SHA-256)-wrapped AES-256-GCM
// key, base64-encoded fields.

const crypto = require('crypto');

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

module.exports = { decryptFromEnvelope };
