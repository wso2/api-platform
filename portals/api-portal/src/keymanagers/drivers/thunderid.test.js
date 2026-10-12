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
 * Unit tests for the ThunderID driver.
 *
 * What is asserted is the request this driver would have put on the wire — URL,
 * method, body shape, and which credential authenticates it — plus how a DCR
 * response maps back onto the portal's own key shape.
 *
 * No network and no key manager: `authRequest` is the seam every driver already
 * calls instead of an HTTP client, so a stub there exercises the real `_call`,
 * the real status classification and the real RFC 7592 URL resolution.
 *
 * The integration suite covers this driver only through the fixtures it
 * configures, which register a client and read it back; the RFC 7592 fallback
 * URL, the response-only field stripping and every error status are reached
 * from here instead.
 */

const test = require('node:test');
const assert = require('node:assert');

const { ThunderIdKeyManager } = require('./thunderid');
const { KeyManagerCallError } = require('../core/keyManager');

const CFG = Object.freeze({
    id: 'thunder',
    displayName: 'ThunderID',
    description: 'Test instance',
    tokenEndpoint: 'https://idp.example.com/oauth2/token',
    authorizeEndpoint: 'https://idp.example.com/oauth2/authorize',
    registrationEndpoint: 'https://idp.example.com/api/server/v1/applications',
    keyType: 'SANDBOX',
});

/** A driver whose every call is recorded instead of sent. */
function driver(responses, cfg) {
    const calls = [];
    const queue = Array.isArray(responses) ? responses.slice() : [responses];
    const km = new ThunderIdKeyManager(Object.assign({}, CFG, cfg), (url, opts) => {
        calls.push({ url, ...opts });
        const next = queue.length > 1 ? queue.shift() : queue[0];
        if (next instanceof Error) return Promise.reject(next);
        return Promise.resolve(next);
    });
    return { km, calls };
}

const REGISTERED = {
    status: 201,
    data: {
        client_id: 'abc123',
        client_secret: 's3cret',
        client_id_issued_at: 1700000000,
        client_secret_expires_at: 0,
        registration_client_uri: 'https://idp.example.com/api/server/v1/applications/abc123',
        registration_access_token: 'rat-xyz',
        client_name: 'My App',
        grant_types: ['client_credentials'],
    },
};

// ---------------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------------

test('the driver reports its own type and the environment it issues keys for', () => {
    const { km } = driver({ status: 200, data: {} });
    assert.equal(km.type, 'thunderid');
    assert.equal(km.keyType, 'SANDBOX', 'carried from config, not defaulted');
    assert.equal(km.keyCreation, 'register', 'this driver mints credentials');
});

test('keyType defaults to PRODUCTION when the config omits it', () => {
    const { km } = driver({ status: 200, data: {} }, { keyType: undefined });
    assert.equal(km.keyType, 'PRODUCTION');
});

// ---------------------------------------------------------------------------
// metadata()
// ---------------------------------------------------------------------------

test('metadata describes the envelope the API returns', () => {
    const { km } = driver({ status: 200, data: {} });
    const meta = km.metadata();
    assert.equal(meta.id, 'thunder');
    assert.equal(meta.type, 'thunderid');
    assert.equal(meta.keyCreation, 'register');
    assert.equal(meta.tokenEndpoint, CFG.tokenEndpoint);
    assert.equal(meta.authorizeEndpoint, CFG.authorizeEndpoint);
    assert.equal(meta.description, 'Test instance');
});

test('metadata omits an absent description and authorize endpoint rather than sending them empty', () => {
    // A client_credentials-only key manager has no interactive endpoint, and the
    // schema has both as optional.
    const { km } = driver({ status: 200, data: {} }, { description: '', authorizeEndpoint: '' });
    const meta = km.metadata();
    assert.ok(!('description' in meta));
    assert.ok(!('authorizeEndpoint' in meta));
});

test('metadata offers the short property list this key manager actually uses', () => {
    const { km } = driver({ status: 200, data: {} });
    const names = km.metadata().properties.map((p) => p.name);
    assert.deepEqual(names, [
        'client_name', 'redirect_uris', 'grant_types', 'response_types', 'token_endpoint_auth_method',
    ]);
    // The wider RFC 7591 set lives on the custom driver; offering it here was
    // form noise. See the comment on metadata().
    for (const absent of ['contacts', 'client_uri', 'logo_uri', 'tos_uri', 'policy_uri', 'scope']) {
        assert.ok(!names.includes(absent), `${absent} belongs to the custom driver`);
    }
});

test('only the two members needed to register a working client are required', () => {
    const { km } = driver({ status: 200, data: {} });
    const required = km.metadata().properties.filter((p) => p.required).map((p) => p.name);
    assert.deepEqual(required, ['client_name', 'grant_types']);
});

test('redirect_uris is conditional on the authorization code grant, not required outright', () => {
    /*
     * RFC 7591 §2 requires redirect_uris only for clients using redirect flows. A
     * client_credentials client never redirects, so demanding one from it would
     * reject a perfectly valid registration.
     */
    const { km } = driver({ status: 200, data: {} });
    const redirect = km.metadata().properties.find((p) => p.name === 'redirect_uris');
    assert.equal(redirect.required, false);
    assert.equal(redirect.type, 'string_list', 'a repeatable field, not one packed string');
    assert.deepEqual(redirect.requiredWhen, {
        property: 'grant_types', anyOf: ['authorization_code'],
    }, 'asked for only when the grant that redirects is selected');
});

test('the grant and response type options are the ones this driver supports', () => {
    const { km } = driver({ status: 200, data: {} });
    const byName = Object.fromEntries(km.metadata().properties.map((p) => [p.name, p]));
    assert.deepEqual(byName.grant_types.options.map((o) => o.value),
        ['authorization_code', 'refresh_token', 'client_credentials']);
    assert.deepEqual(byName.response_types.options.map((o) => o.value), ['code', 'token', 'id_token']);
    assert.deepEqual(byName.token_endpoint_auth_method.options.map((o) => o.value),
        ['client_secret_basic', 'client_secret_post', 'none']);
});

// ---------------------------------------------------------------------------
// createKey
// ---------------------------------------------------------------------------

test('createKey posts the properties to the registration endpoint', async () => {
    const { km, calls } = driver(REGISTERED);
    const key = await km.createKey({ client_name: 'My App', grant_types: ['client_credentials'] });

    assert.equal(calls.length, 1);
    assert.equal(calls[0].url, CFG.registrationEndpoint);
    assert.equal(calls[0].method, 'POST');
    assert.equal(calls[0].headers['Content-Type'], 'application/json');
    assert.deepEqual(JSON.parse(calls[0].data),
        { client_name: 'My App', grant_types: ['client_credentials'] });

    assert.equal(key.consumerKey, 'abc123');
    assert.equal(key.consumerSecret, 's3cret');
    assert.equal(key.keyManagerId, 'thunder');
    assert.equal(key.registration.clientUri, REGISTERED.data.registration_client_uri);
    assert.equal(key.registration.accessToken, 'rat-xyz');
});

test('createKey accepts 200 as well as 201', async () => {
    // Both are seen in the wild for a successful registration.
    const { km } = driver({ status: 200, data: { client_id: 'ok' } });
    assert.equal((await km.createKey({ client_name: 'x' })).consumerKey, 'ok');
});

test('createKey carries no registration access token — there is none yet', async () => {
    const { km, calls } = driver(REGISTERED);
    await km.createKey({ client_name: 'x' });
    assert.ok(!calls[0].headers.Authorization,
        'the portal credential is attached by the authenticator, not here');
});

test('an array scope is sent as the space-delimited string RFC 7591 defines', async () => {
    const { km, calls } = driver(REGISTERED);
    await km.createKey({ client_name: 'x', scope: ['openid', 'profile'] });
    assert.equal(JSON.parse(calls[0].data).scope, 'openid profile');
});

test('a registration response with no body still yields a key shape', async () => {
    // Nothing to persist, but the caller must not have to guard for null.
    const { km } = driver({ status: 201, data: null });
    const key = await km.createKey({ client_name: 'x' });
    assert.equal(key.consumerKey, '');
    assert.equal(key.consumerSecret, '');
    assert.deepEqual(key.registration, { clientUri: '', accessToken: '' });
});

// ---------------------------------------------------------------------------
// The RFC 7592 client URL
// ---------------------------------------------------------------------------

test('the registration response\'s own client URI wins, per RFC 7592 §3', async () => {
    const { km, calls } = driver({ status: 200, data: {} });
    await km.getKey('abc123', { clientUri: 'https://idp.example.com/elsewhere/abc123' });
    assert.equal(calls[0].url, 'https://idp.example.com/elsewhere/abc123');
});

test('without one, the URL is built under the registration endpoint', async () => {
    const { km, calls } = driver({ status: 200, data: {} });
    await km.getKey('abc123', null);
    assert.equal(calls[0].url, `${CFG.registrationEndpoint}/abc123`);
});

test('a client id is percent-encoded into the fallback URL', async () => {
    // A client id is the key manager's to choose; one with a slash in it must not
    // silently address a different path.
    const { km, calls } = driver({ status: 200, data: {} });
    await km.getKey('a/b c', undefined);
    assert.equal(calls[0].url, `${CFG.registrationEndpoint}/a%2Fb%20c`);
});

test('an empty clientUri falls back rather than dialling the empty string', async () => {
    const { km, calls } = driver({ status: 200, data: {} });
    await km.getKey('abc123', { clientUri: '', accessToken: 't' });
    assert.equal(calls[0].url, `${CFG.registrationEndpoint}/abc123`);
});

// ---------------------------------------------------------------------------
// getKey
// ---------------------------------------------------------------------------

test('getKey authenticates with the registration access token when there is one', async () => {
    const { km, calls } = driver({ status: 200, data: { client_id: 'abc123' } });
    await km.getKey('abc123', { clientUri: '', accessToken: 'rat-xyz' });
    assert.equal(calls[0].method, 'GET');
    assert.equal(calls[0].headers.Authorization, 'Bearer rat-xyz');
    assert.ok(!calls[0].data, 'a GET carries no body');
});

test('getKey strips the response-only members, leaving the properties bag', async () => {
    const { km } = driver({
        status: 200,
        data: {
            client_id: 'abc123',
            client_secret: 's3cret',
            client_id_issued_at: 1,
            client_secret_expires_at: 0,
            registration_client_uri: 'https://idp.example.com/x',
            registration_access_token: 'rat',
            client_name: 'My App',
            grant_types: ['client_credentials'],
        },
    });
    const key = await km.getKey('abc123', null);
    assert.deepEqual(key.properties, { client_name: 'My App', grant_types: ['client_credentials'] });
    assert.equal(key.consumerKey, 'abc123', 'the id is still reported, just not as a property');
});

test('a space-delimited scope comes back as the array the API defines', async () => {
    const { km } = driver({ status: 200, data: { client_id: 'x', scope: 'openid  profile email' } });
    const key = await km.getKey('x', null);
    assert.deepEqual(key.properties.scope, ['openid', 'profile', 'email'],
        'repeated whitespace does not produce empty entries');
});

test('an absent scope is left absent rather than becoming an empty array', async () => {
    const { km } = driver({ status: 200, data: { client_id: 'x' } });
    assert.ok(!('scope' in (await km.getKey('x', null)).properties));
});

// ---------------------------------------------------------------------------
// updateKey / deleteKey
// ---------------------------------------------------------------------------

test('updateKey PUTs the new properties to the client URL', async () => {
    const { km, calls } = driver({ status: 200, data: { client_id: 'abc123' } });
    const key = await km.updateKey('abc123', { client_name: 'Renamed' },
        { clientUri: 'https://idp.example.com/c/abc123', accessToken: 'rat' });

    assert.equal(calls[0].method, 'PUT');
    assert.equal(calls[0].url, 'https://idp.example.com/c/abc123');
    assert.deepEqual(JSON.parse(calls[0].data), { client_name: 'Renamed' });
    assert.equal(calls[0].headers.Authorization, 'Bearer rat');
    assert.deepEqual(key.properties, { client_name: 'Renamed' },
        'the request is authoritative for properties, not the echo');
});

test('deleteKey sends DELETE and accepts any of the three success statuses', async () => {
    for (const status of [200, 202, 204]) {
        const { km, calls } = driver({ status, data: null });
        await km.deleteKey('abc123', { clientUri: '', accessToken: 'rat' });
        assert.equal(calls[0].method, 'DELETE');
        assert.equal(calls[0].url, `${CFG.registrationEndpoint}/abc123`);
        assert.ok(!calls[0].data, `DELETE (${status}) carries no body`);
    }
});

// ---------------------------------------------------------------------------
// Failure classification
// ---------------------------------------------------------------------------

test('an upstream status maps to a coarse, non-identifying reason', async () => {
    const cases = [
        [400, 'rejected'], [422, 'rejected'],
        [401, 'provisioning_credential_rejected'], [403, 'provisioning_credential_rejected'],
        [404, 'client_not_found'], [409, 'conflict'], [429, 'rate_limited'],
        [500, 'upstream_error'], [503, 'upstream_error'],
    ];
    for (const [status, reason] of cases) {
        const { km } = driver({ status, data: { error: 'invalid_request' } });
        await assert.rejects(
            () => km.createKey({ client_name: 'x' }),
            (err) => {
                assert.ok(err instanceof KeyManagerCallError);
                assert.equal(err.publicReason, reason, `HTTP ${status}`);
                assert.equal(err.upstreamStatus, status);
                return true;
            }
        );
    }
});

test('the upstream URL and body stay in detail, never in the public reason', async () => {
    /*
     * error-handling.md directive 1: the message the REST layer maps to a status
     * must not name an internal host or echo what the key manager returned.
     */
    const { km } = driver({ status: 500, data: { error_description: 'internal db down on node-7' } });
    await assert.rejects(() => km.createKey({ client_name: 'x' }), (err) => {
        assert.equal(err.publicReason, 'upstream_error');
        assert.ok(!err.publicReason.includes('idp.example.com'));
        assert.ok(err.detail.includes('idp.example.com'), 'the diagnosis is kept for the log');
        assert.ok(err.detail.includes('node-7'));
        return true;
    });
});

test('a transport failure is reported as unreachable, not as a rejection', async () => {
    // DNS failure, refused connection, TLS mismatch, or the SSRF guard refusing
    // the resolved address at dial time all land here.
    const { km } = driver(new Error('getaddrinfo ENOTFOUND idp.example.com'));
    await assert.rejects(() => km.createKey({ client_name: 'x' }), (err) => {
        assert.equal(err.publicReason, 'unreachable');
        assert.equal(err.upstreamStatus, null);
        return true;
    });
});

test('an error the authenticator already classified is re-thrown unchanged', async () => {
    // Relabelling it as a connectivity problem would hide a rejected provisioning
    // credential behind "the key manager is down".
    const original = new KeyManagerCallError('provisioning_credential_rejected', 'token request 401', 401);
    const { km } = driver(original);
    await assert.rejects(() => km.createKey({ client_name: 'x' }), (err) => {
        assert.strictEqual(err, original);
        return true;
    });
});

test('a failed read is classified the same way as a failed registration', async () => {
    const { km } = driver({ status: 404, data: null });
    await assert.rejects(() => km.getKey('gone', null), (err) => {
        assert.equal(err.publicReason, 'client_not_found');
        return true;
    });
});
