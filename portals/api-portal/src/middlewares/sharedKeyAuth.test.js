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
 * Unit tests for src/middlewares/sharedKeyAuth.js.
 *
 * Driven through a child process (same pattern as authorizationConfig.test.js)
 * because sharedKeyAuth requires configLoader, which fail-closes on module load
 * without a --config argument. Each test spawns a node child, loads the module
 * under a chosen fixture config, runs a probe, and emits a marker-prefixed
 * JSON line the parent parses.
 */

const test = require('node:test');
const assert = require('node:assert');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const SHIPPED_MAPPING_PATH = path.join(PROJECT_ROOT, 'resources', 'role-to-scope-mapping.yaml');
const SHARED_KEY_MODULE = path.join(__dirname, 'sharedKeyAuth.js');

// A stable raw value and its corresponding sha256 hex, precomputed here so both
// the parent (asserting) and the child (probing) can reference the same pair
// without recomputing anything.
const KNOWN_RAW = 'raw-value-for-shared-key-tests-xxxxxxxxxxxxxxxxxxxxxx';
const KNOWN_HASH = crypto.createHash('sha256').update(KNOWN_RAW, 'utf8').digest('hex');

// A config carrying only what the unrelated startup checks demand, plus role-mode
// authorization backed by the shipped mapping so the platform-api-system role
// resolves to its five dp:*:manage scopes.
const BASE_CONFIG = `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"

[api_portal.auth.claim_mappings]
roles = "roles"

[api_portal.auth.authorization]
mode = "role"
role_to_scope_mapping = ${JSON.stringify(SHIPPED_MAPPING_PATH)}
`;

let tmpDir;
function fixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-sharedkey-'));
    const file = path.join(tmpDir, name);
    fs.writeFileSync(file, contents);
    return file;
}

function hash(s) {
    let h = 0;
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
    return h;
}

/**
 * Runs `probeBody` inside a child that has loaded the module under the given
 * hash config. The body has `sharedKeyAuth` bound to the loaded module and
 * `emit(v)` bound to a marker-prefixed JSON emitter. Returns the parsed value.
 */
function runProbe(hashHex, probeBody) {
    const base = fixture('base.toml', BASE_CONFIG);
    const overlay = fixture(
        `overlay-${Math.abs(hash(hashHex + probeBody))}.toml`,
        `[api_portal.internal_auth]\nhash = "${hashHex}"\n`
    );
    const runner = fixture(`probe-${Math.abs(hash(probeBody))}.js`, `
        const sharedKeyAuth = require(${JSON.stringify(SHARED_KEY_MODULE)});
        const emit = (v) => process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(v) + '\\n');
        ${probeBody}
    `);
    const result = spawnSync(process.execPath, [runner, '--config', base, '--config', overlay], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(result.status, 0, `child failed: ${result.stderr}`);
    const line = result.stdout.split('\n').find(l => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${result.stdout}`);
    return JSON.parse(line.slice('PROBE_JSON:'.length));
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

// ---------------------------------------------------------------------------
// parseAuthorizationScheme
// ---------------------------------------------------------------------------

test('parseAuthorizationScheme returns lowercase scheme and trimmed value for well-formed headers', () => {
    const result = runProbe(KNOWN_HASH, `
        emit([
            sharedKeyAuth.parseAuthorizationScheme('SharedKey abc123'),
            sharedKeyAuth.parseAuthorizationScheme('sharedkey abc123'),
            sharedKeyAuth.parseAuthorizationScheme('Bearer xyz'),
            sharedKeyAuth.parseAuthorizationScheme('  SharedKey   value-with-spaces  '),
        ]);
    `);
    assert.deepEqual(result[0], { scheme: 'sharedkey', value: 'abc123' });
    assert.deepEqual(result[1], { scheme: 'sharedkey', value: 'abc123' });
    assert.deepEqual(result[2], { scheme: 'bearer', value: 'xyz' });
    // Leading/trailing whitespace in the header is trimmed, but internal spaces in
    // the value are preserved (openssl-generated hex has no internal spaces, so this
    // is just being conservative).
    assert.equal(result[3].scheme, 'sharedkey');
    assert.equal(result[3].value, 'value-with-spaces');
});

test('parseAuthorizationScheme returns null for missing or malformed headers', () => {
    const result = runProbe(KNOWN_HASH, `
        emit([
            sharedKeyAuth.parseAuthorizationScheme(undefined),
            sharedKeyAuth.parseAuthorizationScheme(''),
            sharedKeyAuth.parseAuthorizationScheme('   '),
            sharedKeyAuth.parseAuthorizationScheme('NoSpaces'),
            sharedKeyAuth.parseAuthorizationScheme(123),
        ]);
    `);
    assert.equal(result[0], null);
    assert.equal(result[1], null);
    assert.equal(result[2], null);
    // A scheme with no value (no space) does not parse. Callers treat this as
    // "not authenticated" rather than "empty-string authenticated".
    assert.equal(result[3], null);
    assert.equal(result[4], null);
});

// ---------------------------------------------------------------------------
// isSharedKeyRequest
// ---------------------------------------------------------------------------

test('isSharedKeyRequest is true only for the SharedKey scheme', () => {
    const result = runProbe(KNOWN_HASH, `
        emit([
            sharedKeyAuth.isSharedKeyRequest({ headers: { authorization: 'SharedKey abc' } }),
            sharedKeyAuth.isSharedKeyRequest({ headers: { authorization: 'sharedkey abc' } }),
            sharedKeyAuth.isSharedKeyRequest({ headers: { authorization: 'Bearer abc' } }),
            sharedKeyAuth.isSharedKeyRequest({ headers: { authorization: '' } }),
            sharedKeyAuth.isSharedKeyRequest({ headers: {} }),
            sharedKeyAuth.isSharedKeyRequest({}),
        ]);
    `);
    assert.deepEqual(result, [true, true, false, false, false, false]);
});

// ---------------------------------------------------------------------------
// verifyHash
// ---------------------------------------------------------------------------

test('verifyHash matches the raw value against its precomputed sha256 hex', () => {
    const result = runProbe(KNOWN_HASH, `
        emit(sharedKeyAuth.verifyHash(${JSON.stringify(KNOWN_RAW)}, ${JSON.stringify(KNOWN_HASH)}));
    `);
    assert.equal(result, true);
});

test('verifyHash rejects the wrong raw value against a real hash', () => {
    const result = runProbe(KNOWN_HASH, `
        emit(sharedKeyAuth.verifyHash('completely-different-value', ${JSON.stringify(KNOWN_HASH)}));
    `);
    assert.equal(result, false);
});

test('verifyHash rejects empty inputs on either side', () => {
    const result = runProbe(KNOWN_HASH, `
        emit([
            sharedKeyAuth.verifyHash('', ${JSON.stringify(KNOWN_HASH)}),
            sharedKeyAuth.verifyHash(${JSON.stringify(KNOWN_RAW)}, ''),
            sharedKeyAuth.verifyHash(null, ${JSON.stringify(KNOWN_HASH)}),
            sharedKeyAuth.verifyHash(${JSON.stringify(KNOWN_RAW)}, null),
        ]);
    `);
    assert.deepEqual(result, [false, false, false, false]);
});

test('verifyHash rejects a non-hex configured hash without throwing', () => {
    const result = runProbe(KNOWN_HASH, `
        emit([
            sharedKeyAuth.verifyHash(${JSON.stringify(KNOWN_RAW)}, 'not-hex-at-all-just-some-garbage'),
            sharedKeyAuth.verifyHash(${JSON.stringify(KNOWN_RAW)}, 'abc'),
        ]);
    `);
    assert.deepEqual(result, [false, false]);
});

// ---------------------------------------------------------------------------
// synthesiseSharedKeyPrincipal
// ---------------------------------------------------------------------------

test('synthesiseSharedKeyPrincipal returns a fixed shape carrying the five dp:*:manage scopes', () => {
    const principal = runProbe(KNOWN_HASH, `
        emit(sharedKeyAuth.synthesiseSharedKeyPrincipal());
    `);
    // Distinct auth mode: audit / downstream code can tell shared-key requests apart
    // from oauth / session traffic.
    assert.equal(principal.mode, 'shared-key');
    // No preauthorized shortcut: the OpenAPI validator still runs the per-operation
    // scope check against `scopes`, which is what limits shared-key to the five admin
    // write operations rather than any hand-maintained list of routes.
    assert.equal(principal.preauthorized, false);
    // No portal user represents this identity, so userId is null and rawSub records
    // the service role name for audit-log purposes.
    assert.equal(principal.userId, null);
    assert.equal(principal.rawSub, 'platform-api-system');
    // The five scopes come from expanding the platform-api-system role through the
    // shipped role-to-scope-mapping.yaml. Ordering is not stable across map iterations,
    // so compare as sets.
    assert.deepEqual([...principal.scopes].sort(), [
        'dp:api:manage',
        'dp:api_content:manage',
        'dp:mcp_server:manage',
        'dp:mcp_server_content:manage',
        'dp:subscription_plan:manage',
    ]);
});

// ---------------------------------------------------------------------------
// tryAuthenticate — the composite that authResolver calls
// ---------------------------------------------------------------------------

test('tryAuthenticate returns {matched:false} when no Authorization header is present', () => {
    const result = runProbe(KNOWN_HASH, `
        emit(sharedKeyAuth.tryAuthenticate({ headers: {} }));
    `);
    // Caller falls through to the next auth path (session / bearer / mTLS) rather
    // than treating an absent header as a shared-key attempt.
    assert.deepEqual(result, { matched: false });
});

test('tryAuthenticate returns {matched:false} for a Bearer request even when the token happens to hash to the configured value', () => {
    // The scheme name is the sole discriminator: a request that carries a Bearer
    // token whose value happens to hash to the configured shared-key hash must not
    // be accepted as a shared-key call. Scheme-based dispatch is what makes the
    // "distinct wire discriminator" claim in the design doc real.
    const result = runProbe(KNOWN_HASH, `
        emit(sharedKeyAuth.tryAuthenticate({
            headers: { authorization: 'Bearer ${KNOWN_RAW}' }
        }));
    `);
    assert.deepEqual(result, { matched: false });
});

test('tryAuthenticate returns {matched:true, auth:null} when the SharedKey value does not match the configured hash', () => {
    const result = runProbe(KNOWN_HASH, `
        emit(sharedKeyAuth.tryAuthenticate({
            headers: { authorization: 'SharedKey wrong-value-that-hashes-to-nothing' }
        }));
    `);
    // Matched (scheme is SharedKey) but auth is null: the caller (authResolver) turns
    // this into a hard 401 rather than falling through, so a bad SharedKey can never
    // quietly retry against the OAuth path with the same request.
    assert.deepEqual(result, { matched: true, auth: null });
});

test('tryAuthenticate verifies against the configured hash and returns the full principal on match', () => {
    const result = runProbe(KNOWN_HASH, `
        emit(sharedKeyAuth.tryAuthenticate({
            headers: { authorization: 'SharedKey ${KNOWN_RAW}' }
        }));
    `);
    assert.equal(result.matched, true);
    assert.ok(result.auth, 'expected a populated principal for a verified SharedKey request');
    assert.equal(result.auth.mode, 'shared-key');
    assert.equal(result.auth.preauthorized, false);
    assert.equal(result.auth.rawSub, 'platform-api-system');
    assert.deepEqual([...result.auth.scopes].sort(), [
        'dp:api:manage',
        'dp:api_content:manage',
        'dp:mcp_server:manage',
        'dp:mcp_server_content:manage',
        'dp:subscription_plan:manage',
    ]);
});

test('tryAuthenticate returns {matched:true, auth:null} when the portal has no configured hash', () => {
    // Sending SharedKey against a portal that has shared-key disabled must fail:
    // silent fall-through would let a client accidentally satisfy a different auth
    // path with the SharedKey value it was expecting the portal to hash.
    const result = runProbe('', `
        emit(sharedKeyAuth.tryAuthenticate({
            headers: { authorization: 'SharedKey ${KNOWN_RAW}' }
        }));
    `);
    assert.deepEqual(result, { matched: true, auth: null });
});
