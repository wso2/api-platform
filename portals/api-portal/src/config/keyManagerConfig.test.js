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
 * Unit tests for src/config/keyManagerConfig.js.
 *
 * This module decides whether the portal starts. Every throw here is a startup
 * failure by design — the alternative to rejecting a malformed key manager is
 * booting with one that silently cannot register clients, or worse, one whose
 * address policy was relaxed by a flag nobody noticed was in the wrong table.
 * So the assertions below are mostly about which inputs are refused, and that
 * each refusal says enough to act on.
 *
 * Required in-process rather than through a child, unlike its sibling tests:
 * this module is deliberately free of configLoader and logger (requiring either
 * back would be a load cycle), so it imports with no --config argument.
 */

const test = require('node:test');
const assert = require('node:assert');

/*
 * The drivers register themselves when this module is required, and the type
 * check below resolves against that registry. The application does this during
 * its own bootstrap (keymanagers/index.js); without it every `type` here is
 * unknown and the registry reports "(none registered)".
 */
require('../keymanagers/drivers');

const {
    validateKeyManagerConfig,
    buildAuthenticator,
    AUTH_METHODS,
    ID_PATTERN,
} = require('./keyManagerConfig');

const { ApiKey, BasicAuth, ClientCredentials, MutualTLS } = require('../keymanagers/core/authenticators');

/** A complete, valid entry. Each test changes one thing about it. */
const entry = (overrides = {}) => ({
    id: 'thunder-local',
    name: 'ThunderID',
    type: 'thunderid',
    registrationEndpoint: 'https://idp.example.com/oauth2/dcr/register',
    tokenEndpoint: 'https://idp.example.com/oauth2/token',
    auth: { method: 'client_credentials', clientId: 'cid', clientSecret: 'secret' },
    ...overrides,
});

const cfg = (...entries) => ({ keyManager: entries });

// ---------------------------------------------------------------------------
// The array itself
// ---------------------------------------------------------------------------

test('validateKeyManagerConfig treats an absent key_manager as no key managers', () => {
    // Not an error: a portal with no key manager configured is a normal
    // deployment, it simply cannot register OAuth2 clients.
    assert.deepEqual(validateKeyManagerConfig({}), []);
    assert.deepEqual(validateKeyManagerConfig({ keyManager: null }), []);
    assert.deepEqual(validateKeyManagerConfig({ keyManager: [] }), []);
});

test('validateKeyManagerConfig rejects a single table and names the bracket mistake', () => {
    /*
     * `[api_portal.key_manager]` instead of `[[...]]` produces an object, not an
     * array — the single most likely TOML slip here, so the message names the
     * fix rather than reporting a type.
     */
    assert.throws(() => validateKeyManagerConfig({ keyManager: entry() }),
        /must be an array of tables.*double brackets/s);
});

test('validateKeyManagerConfig rejects a non-table entry', () => {
    assert.throws(() => validateKeyManagerConfig(cfg('not-a-table')), /key_manager\[0\] is not a table/);
    assert.throws(() => validateKeyManagerConfig(cfg(['nested'])), /key_manager\[0\] is not a table/);
    assert.throws(() => validateKeyManagerConfig(cfg(null)), /key_manager\[0\] is not a table/);
});

test('validateKeyManagerConfig rejects duplicate ids', () => {
    // The id is how a caller selects a key manager, so two with the same id make
    // the choice ambiguous rather than merely redundant.
    assert.throws(() => validateKeyManagerConfig(cfg(entry(), entry())),
        /duplicate id "thunder-local"/);
});

test('validateKeyManagerConfig accepts several distinct key managers', () => {
    const out = validateKeyManagerConfig(cfg(entry(), entry({ id: 'asgardeo', type: 'asgardeo' })));
    assert.equal(out.length, 2);
    assert.deepEqual(out.map((k) => k.id), ['thunder-local', 'asgardeo']);
});

// ---------------------------------------------------------------------------
// Address-policy flags in the wrong place
// ---------------------------------------------------------------------------

test('validateKeyManagerConfig refuses an address flag left in key_manager_client', () => {
    /*
     * These moved from the global section onto each key manager. Left behind
     * they would be silently ignored, which quietly re-imposes the deny-by-default
     * posture on a key manager the operator believes they opted in — a security
     * setting that reads as applied and is not. Hence a startup failure.
     */
    for (const camel of ['allowPrivateEndpoints', 'allowHttpEndpoints']) {
        assert.throws(
            () => validateKeyManagerConfig({ keyManager: [entry()], keyManagerClient: { [camel]: true } }),
            /has moved: set it on the individual/,
            `expected ${camel} in key_manager_client to be refused`
        );
    }
});

test('validateKeyManagerConfig leaves the remaining key_manager_client settings alone', () => {
    // timeout_ms and the size caps still belong there; only the address flags moved.
    const out = validateKeyManagerConfig({
        keyManager: [entry()],
        keyManagerClient: { timeoutMs: 5000, maxRequestBytes: 1024, maxResponseBytes: 2048 },
    });
    assert.equal(out.length, 1);
});

test('normalizeInstance refuses an address flag placed under the auth table', () => {
    // Same reasoning one level down: the policy describes the key manager's host,
    // not its token request, so it is rejected rather than quietly dropped.
    assert.throws(
        () => validateKeyManagerConfig(cfg(entry({
            auth: { method: 'client_credentials', clientId: 'c', clientSecret: 's', insecureSkipVerify: true },
        }))),
        /belongs directly under/
    );
});

// ---------------------------------------------------------------------------
// Required fields and the id rule
// ---------------------------------------------------------------------------

test('validateKeyManagerConfig names each missing required field', () => {
    const required = {
        id: 'id',
        type: 'type',
        name: 'name',
        registrationEndpoint: 'registration_endpoint',
        tokenEndpoint: 'token_endpoint',
    };
    for (const [key, toml] of Object.entries(required)) {
        const without = entry();
        delete without[key];
        assert.throws(() => validateKeyManagerConfig(cfg(without)),
            new RegExp(`missing required field "${toml}"`),
            `expected a missing "${toml}" to be reported`);
    }
});

test('validateKeyManagerConfig treats an empty string as missing, not as a value', () => {
    assert.throws(() => validateKeyManagerConfig(cfg(entry({ name: '' }))),
        /missing required field "name"/);
});

test('validateKeyManagerConfig rejects a non-string where a string is required', () => {
    assert.throws(() => validateKeyManagerConfig(cfg(entry({ name: 42 }))),
        /"name" must be a string/);
});

test('ID_PATTERN accepts URL-safe ids and rejects the rest', () => {
    // The id lands in a URL path segment and in log lines, so the rule is the
    // same one every other operator-chosen identifier here follows.
    for (const ok of ['thunder-local', 'km1', 'a.b_c-d', '9lives']) {
        assert.ok(ID_PATTERN.test(ok), `${ok} should be accepted`);
    }
    for (const bad of ['-leading', '.dot', 'has space', 'slash/es', 'uni✓code', '']) {
        assert.ok(!ID_PATTERN.test(bad), `${bad} should be rejected`);
    }
});

test('validateKeyManagerConfig rejects an id that is not URL-safe, explaining the rule', () => {
    assert.throws(() => validateKeyManagerConfig(cfg(entry({ id: 'has space' }))),
        /must start with a letter or digit/);
});

test('validateKeyManagerConfig rejects an unknown driver type and lists the known ones', () => {
    assert.throws(() => validateKeyManagerConfig(cfg(entry({ type: 'nope' }))),
        /unknown type "nope".*Built-in types:/s);
});

// ---------------------------------------------------------------------------
// The auth table
// ---------------------------------------------------------------------------

test('validateKeyManagerConfig requires the auth table', () => {
    for (const bad of [undefined, null, 'string', ['array']]) {
        const e = entry();
        e.auth = bad;
        assert.throws(() => validateKeyManagerConfig(cfg(e)),
            /missing its \[api_portal\.key_manager\.auth\] table/,
            `expected auth=${JSON.stringify(bad)} to be refused`);
    }
});

test('buildAuthenticator rejects an unknown method and lists the supported ones', () => {
    assert.throws(() => buildAuthenticator({ method: 'magic' }, 'km "x"', {}, 'https://t'),
        /unknown auth\.method "magic"/);
    assert.throws(() => buildAuthenticator({ method: 'magic' }, 'km "x"', {}, 'https://t'),
        new RegExp(AUTH_METHODS.join(', ').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
});

test('buildAuthenticator builds each supported method', () => {
    const policy = {};
    const built = {
        client_credentials: buildAuthenticator(
            { method: 'client_credentials', clientId: 'c', clientSecret: 's' }, 'km', policy, 'https://t'),
        basic: buildAuthenticator({ method: 'basic', username: 'u', password: 'p' }, 'km', policy, 'https://t'),
        api_key: buildAuthenticator({ method: 'api_key', apiKey: 'k' }, 'km', policy, 'https://t'),
        mtls: buildAuthenticator(
            { method: 'mtls', certFile: '/c.pem', keyFile: '/k.pem' }, 'km', policy, 'https://t'),
    };
    assert.ok(built.client_credentials instanceof ClientCredentials);
    assert.ok(built.basic instanceof BasicAuth);
    assert.ok(built.api_key instanceof ApiKey);
    assert.ok(built.mtls instanceof MutualTLS);
});

test('buildAuthenticator names the missing credential per method', () => {
    const cases = [
        [{ method: 'client_credentials', clientSecret: 's' }, /requires "auth\.client_id"/],
        [{ method: 'client_credentials', clientId: 'c' }, /requires "auth\.client_secret"/],
        [{ method: 'basic', password: 'p' }, /requires "auth\.username"/],
        [{ method: 'basic', username: 'u' }, /requires "auth\.password"/],
        [{ method: 'api_key' }, /requires "auth\.api_key"/],
        [{ method: 'mtls', keyFile: '/k' }, /requires "auth\.cert_file"/],
        [{ method: 'mtls', certFile: '/c' }, /requires "auth\.key_file"/],
    ];
    for (const [auth, expected] of cases) {
        assert.throws(() => buildAuthenticator(auth, 'km "x"', {}, 'https://t'), expected,
            `expected ${auth.method} to report its missing field`);
    }
});

test('buildAuthenticator rejects a non-string credential', () => {
    assert.throws(() => buildAuthenticator(
        { method: 'client_credentials', clientId: 1, clientSecret: 's' }, 'km', {}, 'https://t'),
    /"auth\.client_id" must be a string/);
});

test('client_credentials falls back to the key manager token endpoint', () => {
    // The common case is the same URL for both; repeating it in the auth table
    // invites the two drifting apart.
    const fellBack = buildAuthenticator(
        { method: 'client_credentials', clientId: 'c', clientSecret: 's' }, 'km', {}, 'https://km/token');
    const explicit = buildAuthenticator(
        { method: 'client_credentials', clientId: 'c', clientSecret: 's', tokenEndpoint: 'https://auth/token' },
        'km', {}, 'https://km/token');
    assert.equal(fellBack.tokenEndpoint, 'https://km/token');
    assert.equal(explicit.tokenEndpoint, 'https://auth/token');
});

test('api_key defaults its header to Authorization and keeps the scheme separate', () => {
    // The scheme is configured apart from the key so rotating the key does not
    // mean retyping "Bearer " in front of it.
    const dflt = buildAuthenticator({ method: 'api_key', apiKey: 'k' }, 'km', {}, 'https://t');
    const named = buildAuthenticator(
        { method: 'api_key', apiKey: 'k', headerName: 'X-Api-Key', scheme: 'Bearer' }, 'km', {}, 'https://t');
    assert.equal(dflt.headerName, 'Authorization');
    assert.equal(dflt.scheme, '');
    assert.equal(named.headerName, 'X-Api-Key');
    assert.equal(named.scheme, 'Bearer');
});

// ---------------------------------------------------------------------------
// Typed fields: booleans, string arrays, key type
// ---------------------------------------------------------------------------

test('client policy flags default per flag, not uniformly', () => {
    /*
     * http is permitted by default because an identity server commonly sits
     * behind a TLS-terminating ingress; private addresses are not, because one
     * key manager opting into localhost must not grant the same reach to a
     * public one configured beside it.
     */
    const [km] = validateKeyManagerConfig(cfg(entry()));
    assert.equal(km.clientPolicy.allowHttpEndpoints, true);
    assert.equal(km.clientPolicy.allowPrivateEndpoints, false);
    assert.equal(km.clientPolicy.insecureSkipVerify, false);
});

test('client policy flags are taken from the entry when set', () => {
    const [km] = validateKeyManagerConfig(cfg(entry({
        insecureSkipVerify: true, allowPrivateEndpoints: true, allowHttpEndpoints: false,
    })));
    assert.deepEqual(km.clientPolicy,
        { insecureSkipVerify: true, allowPrivateEndpoints: true, allowHttpEndpoints: false });
});

test('a non-boolean flag is refused rather than coerced', () => {
    // "false" as a string is truthy — coercing it would turn an operator's
    // intended off into on, silently.
    for (const bad of ['true', 'false', 1, 0]) {
        assert.throws(() => validateKeyManagerConfig(cfg(entry({ insecureSkipVerify: bad }))),
            /"insecure_skip_verify" must be a boolean/,
            `expected ${JSON.stringify(bad)} to be refused`);
    }
});

test('auth.scopes accepts a string array and refuses anything else', () => {
    const ok = buildAuthenticator(
        { method: 'client_credentials', clientId: 'c', clientSecret: 's', scopes: ['a', 'b'] },
        'km', {}, 'https://t');
    assert.deepEqual(ok.scopes, ['a', 'b']);
    assert.throws(() => buildAuthenticator(
        { method: 'client_credentials', clientId: 'c', clientSecret: 's', scopes: 'a b' },
        'km "x"', {}, 'https://t'), /auth\.scopes/);
});

test('key_type defaults to PRODUCTION and accepts SANDBOX in any case', () => {
    assert.equal(validateKeyManagerConfig(cfg(entry()))[0].keyType, 'PRODUCTION');
    assert.equal(validateKeyManagerConfig(cfg(entry({ keyType: 'sandbox' })))[0].keyType, 'SANDBOX');
    assert.equal(validateKeyManagerConfig(cfg(entry({ keyType: '  SandBox ' })))[0].keyType, 'SANDBOX');
});

test('a mistyped key_type is refused, not defaulted', () => {
    // "sandbx" quietly meaning production is exactly the kind of wrong answer a
    // startup check exists to prevent, so it fails the boot instead.
    assert.throws(() => validateKeyManagerConfig(cfg(entry({ keyType: 'sandbx' }))),
        /key_type must be one of PRODUCTION, SANDBOX \(got "sandbx"\)/);
});

// ---------------------------------------------------------------------------
// The normalised shape
// ---------------------------------------------------------------------------

test('a valid entry normalises to the shape the factory consumes', () => {
    const [km] = validateKeyManagerConfig(cfg(entry({
        description: 'local dev',
        authorizeEndpoint: 'https://idp.example.com/oauth2/authorize',
    })));
    assert.equal(km.id, 'thunder-local');
    assert.equal(km.type, 'thunderid');
    assert.equal(km.displayName, 'ThunderID');
    assert.equal(km.description, 'local dev');
    assert.equal(km.registrationEndpoint, 'https://idp.example.com/oauth2/dcr/register');
    assert.equal(km.tokenEndpoint, 'https://idp.example.com/oauth2/token');
    assert.equal(km.authorizeEndpoint, 'https://idp.example.com/oauth2/authorize');
    assert.ok(km.auth instanceof ClientCredentials);
});

test('optional text fields normalise to empty rather than undefined', () => {
    // The factory reads these directly; undefined would become the string
    // "undefined" the moment one reached a URL or a log line.
    const [km] = validateKeyManagerConfig(cfg(entry()));
    assert.equal(km.description, '');
    assert.equal(km.authorizeEndpoint, '');
});
