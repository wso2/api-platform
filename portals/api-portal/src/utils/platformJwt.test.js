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

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const { generateKeyPair, exportSPKI, SignJWT } = require('jose');

const { verifyPlatformJwtClaims, decodePlatformJwtClaims } = require('./platformJwt');
const constants = require('./constants');

const ALG = constants.JWT_ASYMMETRIC_ALGORITHMS[0];

let tmpDir;
function writeKeyFile(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-platformjwt-'));
    const p = path.join(tmpDir, name);
    fs.writeFileSync(p, contents);
    return p;
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

async function makeSignedToken(privateKey, claims = {}) {
    return new SignJWT({ sub: 'platform-api-system', ...claims })
        .setProtectedHeader({ alg: ALG })
        .setIssuedAt()
        .setExpirationTime('5m')
        .sign(privateKey);
}

test('verifyPlatformJwtClaims accepts a token signed by the paired key and parses scopes', async () => {
    const { publicKey, privateKey } = await generateKeyPair(ALG);
    const pubPath = writeKeyFile('happy.pub.pem', await exportSPKI(publicKey));
    const token = await makeSignedToken(privateKey, {
        scope: 'dp:api:manage dp:api_content:manage',
        roles: ['platform-api-system'],
    });

    const claims = await verifyPlatformJwtClaims(token, pubPath);
    assert.ok(claims, 'expected claims for a valid token');
    assert.equal(claims.sub, 'platform-api-system');
    assert.deepEqual(claims.roles, ['platform-api-system']);
    assert.deepEqual(claims.scopes, ['dp:api:manage', 'dp:api_content:manage']);
});

test('verifyPlatformJwtClaims rejects a token signed by a different key', async () => {
    const signer = await generateKeyPair(ALG);
    const verifier = await generateKeyPair(ALG);
    const wrongPubPath = writeKeyFile('wrong.pub.pem', await exportSPKI(verifier.publicKey));
    const token = await makeSignedToken(signer.privateKey);

    const claims = await verifyPlatformJwtClaims(token, wrongPubPath);
    assert.equal(claims, null, 'expected null when signature does not match the configured public key');
});

test('verifyPlatformJwtClaims rejects an expired token', async () => {
    const { publicKey, privateKey } = await generateKeyPair(ALG);
    const pubPath = writeKeyFile('expired.pub.pem', await exportSPKI(publicKey));
    const now = Math.floor(Date.now() / 1000);
    const token = await new SignJWT({ sub: 'platform-api-system' })
        .setProtectedHeader({ alg: ALG })
        .setIssuedAt(now - 3600)
        .setExpirationTime(now - 60)
        .sign(privateKey);

    const claims = await verifyPlatformJwtClaims(token, pubPath);
    assert.equal(claims, null, 'expected null for an expired token');
});

test('verifyPlatformJwtClaims returns null for malformed input', async () => {
    const { publicKey } = await generateKeyPair(ALG);
    const pubPath = writeKeyFile('malformed.pub.pem', await exportSPKI(publicKey));
    assert.equal(await verifyPlatformJwtClaims('not.a.jwt.token', pubPath), null);
    assert.equal(await verifyPlatformJwtClaims('', pubPath), null);
});

test('verifyPlatformJwtClaims returns null when the key file cannot be read', async () => {
    const { privateKey } = await generateKeyPair(ALG);
    const token = await makeSignedToken(privateKey);
    // Ensure tmpDir is materialized without writing a real key file to it.
    writeKeyFile('.marker', '');
    const missing = path.join(tmpDir, 'does-not-exist.pem');

    const claims = await verifyPlatformJwtClaims(token, missing);
    assert.equal(claims, null, 'expected null when the configured public key file is missing');
});

test('verifyPlatformJwtClaims returns empty scopes when the scope claim is absent', async () => {
    const { publicKey, privateKey } = await generateKeyPair(ALG);
    const pubPath = writeKeyFile('noscope.pub.pem', await exportSPKI(publicKey));
    const token = await makeSignedToken(privateKey); // no scope claim

    const claims = await verifyPlatformJwtClaims(token, pubPath);
    assert.ok(claims);
    assert.deepEqual(claims.scopes, []);
});

test('decodePlatformJwtClaims parses the scope claim without verifying', async () => {
    const { privateKey } = await generateKeyPair(ALG);
    const token = await makeSignedToken(privateKey, { scope: 'a b c' });

    const claims = decodePlatformJwtClaims(token);
    assert.ok(claims);
    assert.deepEqual(claims.scopes, ['a', 'b', 'c']);
});

test('decodePlatformJwtClaims returns null for malformed input', () => {
    assert.equal(decodePlatformJwtClaims('not.a.jwt'), null);
    assert.equal(decodePlatformJwtClaims(''), null);
});
