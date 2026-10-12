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
 * Unit tests for the key manager authenticators.
 *
 * Each one turns a configured credential into a `requester` — a function the
 * drivers call instead of an HTTP client, which attaches that credential to
 * every request. What is asserted here is what ends up on the wire: which
 * header, in what form, and when it is deliberately NOT attached.
 *
 * Only ClientCredentials is exercised by the integration suites, because that is
 * what the fixtures configure. Basic, ApiKey and MutualTLS reach production
 * untested otherwise, and each one handles a credential.
 *
 * No network: the requester is driven against a stub transport, so the assertions
 * are about the request that would have been sent. The one case that genuinely
 * needs a server — ClientCredentials minting and caching a token — is covered by
 * the REST integration suite instead.
 */

const test = require('node:test');
const assert = require('node:assert');
const Module = require('node:module');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

/*
 * buildClient is replaced before authenticators.js is loaded, so each
 * authenticator gets a transport that records the request instead of making one.
 * Patching the module cache rather than the instance keeps the authenticators
 * exactly as they ship — nothing here is injected through a seam that only
 * exists for tests.
 */
const sent = [];
const httpClientPath = require.resolve('./httpClient');
const realLoad = Module._load;
Module._load = function patched(request, parent, isMain) {
    const resolved = (() => {
        try { return require.resolve(request, { paths: [parent ? parent.path : __dirname] }); }
        catch { return null; }
    })();
    if (resolved === httpClientPath) {
        return {
            ...realLoad.call(this, request, parent, isMain),
            buildClient: (policy) => ({
                request: (config) => {
                    sent.push({ ...config, policy });
                    return Promise.resolve({ status: 200, data: {} });
                },
            }),
        };
    }
    return realLoad.call(this, request, parent, isMain);
};

const { Authenticator, ApiKey, BasicAuth, MutualTLS } = require('./authenticators');

Module._load = realLoad;

test.beforeEach(() => { sent.length = 0; });

/** Builds the requester and makes one call through it. */
async function callThrough(auth, url = 'https://idp.example.com/register', opts) {
    const request = await auth.requester();
    await request(url, opts);
    return sent[sent.length - 1];
}

// ---------------------------------------------------------------------------
// The abstract base
// ---------------------------------------------------------------------------

test('the base Authenticator refuses to be used directly', () => {
    // A new authenticator that forgets `requester` fails loudly at the first call
    // rather than silently sending unauthenticated requests.
    assert.rejects(() => new Authenticator().requester(), /requester\(\) not implemented/);
});

// ---------------------------------------------------------------------------
// BasicAuth
// ---------------------------------------------------------------------------

test('BasicAuth sends RFC 7617 credentials', async () => {
    const config = await callThrough(new BasicAuth({ username: 'alice', password: 's3cret' }));
    assert.equal(config.headers.Authorization,
        'Basic ' + Buffer.from('alice:s3cret').toString('base64'));
    // Decodes back to exactly what was configured — a colon in the password must
    // not shift the split, which is the classic basic-auth encoding bug.
    const decoded = Buffer.from(config.headers.Authorization.split(' ')[1], 'base64').toString();
    assert.equal(decoded, 'alice:s3cret');
});

test('BasicAuth encodes a password containing a colon without corrupting it', async () => {
    const config = await callThrough(new BasicAuth({ username: 'u', password: 'a:b:c' }));
    const decoded = Buffer.from(config.headers.Authorization.split(' ')[1], 'base64').toString();
    assert.equal(decoded, 'u:a:b:c', 'only the first colon separates user from password');
});

test('BasicAuth passes the key manager client policy to the transport', async () => {
    // The policy is what decides TLS trust and which addresses may be dialled, so
    // an authenticator dropping it would quietly widen a key manager's reach.
    const policy = { insecureSkipVerify: true, allowPrivateEndpoints: true };
    const config = await callThrough(new BasicAuth({ username: 'u', password: 'p', clientPolicy: policy }));
    assert.deepEqual(config.policy, policy);
});

// ---------------------------------------------------------------------------
// ApiKey
// ---------------------------------------------------------------------------

test('ApiKey defaults to the Authorization header', async () => {
    const config = await callThrough(new ApiKey({ key: 'k-123' }));
    assert.equal(config.headers.Authorization, 'k-123');
});

test('ApiKey uses the configured header and prefixes the scheme when set', async () => {
    const bare = await callThrough(new ApiKey({ headerName: 'X-Api-Key', key: 'k-123' }));
    assert.equal(bare.headers['X-Api-Key'], 'k-123');

    // The scheme is configured apart from the key, so rotating the key does not
    // mean retyping the prefix with it.
    const schemed = await callThrough(new ApiKey({ headerName: 'X-Api-Key', scheme: 'Bearer', key: 'k-123' }));
    assert.equal(schemed.headers['X-Api-Key'], 'Bearer k-123');
});

test('ApiKey trims a scheme rather than emitting a double space', async () => {
    const config = await callThrough(new ApiKey({ scheme: '  Bearer  ', key: 'k' }));
    assert.equal(config.headers.Authorization, 'Bearer k');
});

test('ApiKey refuses a header name that cannot carry a credential', () => {
    // Rejected at construction, which is startup: a header the transport would
    // drop or mangle means a request that is sent unauthenticated.
    for (const bad of ['', '   ', 'Bad Header', 'with:colon', 'new\nline']) {
        assert.throws(() => new ApiKey({ headerName: bad, key: 'k' }),
            undefined, `expected header ${JSON.stringify(bad)} to be refused`);
    }
});

// ---------------------------------------------------------------------------
// MutualTLS
// ---------------------------------------------------------------------------

test('MutualTLS reads its material from disk and hands it to the transport', async () => {
    /*
     * Real files, because `requester()` reads them — the credential here is the
     * certificate itself, so it has to reach buildClient as bytes rather than as
     * a path for something later to resolve.
     */
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-mtls-'));
    const certFile = path.join(dir, 'client.pem');
    const keyFile = path.join(dir, 'client.key');
    const caFile = path.join(dir, 'ca.pem');
    fs.writeFileSync(certFile, 'CERT-BYTES');
    fs.writeFileSync(keyFile, 'KEY-BYTES');
    fs.writeFileSync(caFile, 'CA-BYTES');

    try {
        const config = await callThrough(new MutualTLS({ certFile, keyFile, caFile }));
        assert.equal(config.policy.clientCert.cert.toString(), 'CERT-BYTES');
        assert.equal(config.policy.clientCert.key.toString(), 'KEY-BYTES');
        assert.equal(config.policy.clientCert.ca.toString(), 'CA-BYTES');

        // No Authorization header: mTLS authenticates at the transport, so adding
        // one would be a second, unrelated credential.
        const headers = config.headers || {};
        assert.ok(!Object.keys(headers).some((h) => h.toLowerCase() === 'authorization'),
            'mTLS adds no credential header');
    } finally {
        fs.rmSync(dir, { recursive: true, force: true });
    }
});

test('MutualTLS works without a CA file, which is optional', async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-mtls-'));
    const certFile = path.join(dir, 'client.pem');
    const keyFile = path.join(dir, 'client.key');
    fs.writeFileSync(certFile, 'CERT-BYTES');
    fs.writeFileSync(keyFile, 'KEY-BYTES');
    try {
        const config = await callThrough(new MutualTLS({ certFile, keyFile }));
        assert.equal(config.policy.clientCert.cert.toString(), 'CERT-BYTES');
        assert.ok(!config.policy.clientCert.ca, 'no CA is a valid configuration');
    } finally {
        fs.rmSync(dir, { recursive: true, force: true });
    }
});

test('MutualTLS surfaces a missing certificate rather than dialling without one', async () => {
    // Failing here is right: a silent fallback to an unauthenticated client would
    // make a misconfigured path look like a key manager that rejects the portal.
    await assert.rejects(() => new MutualTLS({ certFile: '/no/such.pem', keyFile: '/no/such.key' }).requester(),
        /ENOENT/);
});

// ---------------------------------------------------------------------------
// The shared "already carries a credential" rule
// ---------------------------------------------------------------------------

test('an explicit Authorization header is never overwritten', async () => {
    /*
     * This is what lets a driver send the DEVELOPER's credential — an RFC 7592
     * registration access token, or a client's own secret on a token request —
     * through the same requester that otherwise attaches the portal's. Overwriting
     * it would silently substitute the portal's credential for the caller's.
     */
    for (const auth of [
        new BasicAuth({ username: 'portal', password: 'portal-secret' }),
        new ApiKey({ key: 'portal-key' }),
    ]) {
        const config = await callThrough(auth, 'https://idp/x', {
            headers: { Authorization: 'Bearer caller-token' },
        });
        assert.equal(config.headers.Authorization, 'Bearer caller-token',
            `${auth.constructor.name} must not replace a caller-supplied credential`);
    }
});

test('a lowercase authorization header counts as a caller credential too', async () => {
    // Header names are case-insensitive on the wire; a case-sensitive check here
    // would attach a second, conflicting credential.
    const config = await callThrough(new BasicAuth({ username: 'u', password: 'p' }),
        'https://idp/x', { headers: { authorization: 'Bearer caller-token' } });
    const values = Object.entries(config.headers)
        .filter(([h]) => h.toLowerCase() === 'authorization')
        .map(([, v]) => v);
    assert.deepEqual(values, ['Bearer caller-token'], 'exactly one credential, the caller\'s');
});

test('anonymous requests are sent with no credential at all', async () => {
    /*
     * The token request carries the developer's own client_secret in the body and
     * sets no Authorization header of its own. Without `anonymous` the portal's
     * provisioning credential would be attached, and the key manager would reject
     * the request as invalid_client.
     */
    const config = await callThrough(new BasicAuth({ username: 'portal', password: 'p' }),
        'https://idp/token', { anonymous: true, method: 'POST' });
    const headers = config.headers || {};
    assert.ok(!Object.keys(headers).some((h) => h.toLowerCase() === 'authorization'),
        'an anonymous request must carry no portal credential');
});

test('the caller request options survive the credential being attached', async () => {
    const config = await callThrough(new ApiKey({ key: 'k' }), 'https://idp/register', {
        method: 'POST', data: { client_name: 'x' }, headers: { 'Content-Type': 'application/json' },
    });
    assert.equal(config.method, 'POST');
    assert.deepEqual(config.data, { client_name: 'x' });
    assert.equal(config.headers['Content-Type'], 'application/json');
    assert.equal(config.headers.Authorization, 'k');
    assert.equal(config.url, 'https://idp/register');
});
