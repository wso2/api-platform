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
 * Unit tests for the checks in src/services/keyManagerService.js that decide
 * whether a key manager may be stored: its environment, its provisioning block,
 * and the two endpoints the portal will dial.
 *
 * All of these run before any write, which is the point — a rejected payload
 * must not leave a half-configured key manager behind for an admin to clean up.
 * They are also the branches a route test cannot reach without a live identity
 * server to point at.
 *
 * The address checks are asserted under both deployment postures. `allow_private`
 * and `allow_http` are off by default and an operator turns them on to point a
 * development portal at localhost, so the same payload is legitimately accepted
 * in one deployment and refused in another; a test fixed to one posture would
 * assert half the rule.
 *
 * Driven through a child process (same pattern as sharedKeyAuth.test.js) because
 * the service requires configLoader, which fail-closes on module load without a
 * --config argument — and because the policy is frozen at load, so each posture
 * needs its own process.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const SERVICE_MODULE = path.join(__dirname, 'keyManagerService.js');

const BASE_CONFIG = `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"
`;

// The permissive posture an operator opts into for a development portal.
const PERMISSIVE_CONFIG = `
[api_portal.key_manager_provisioning]
allow_http_endpoints    = true
allow_private_endpoints = true
`;

let tmpDir;
let probeSeq = 0;

function fixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-km-'));
    const file = path.join(tmpDir, name);
    fs.writeFileSync(file, contents);
    return file;
}

/**
 * Runs `probeBody` in a child that has loaded the service. `svc` is the module
 * and `emit(v)` a marker-prefixed JSON emitter.
 *
 * @param {string} probeBody
 * @param {{permissive?: boolean}} [opts] permissive loads the config an operator
 *        sets to allow plaintext and private addresses.
 */
function runProbe(probeBody, opts = {}) {
    const args = [fixture('base.toml', BASE_CONFIG)];
    if (opts.permissive) args.push(fixture('permissive.toml', PERMISSIVE_CONFIG));
    const runner = fixture(`probe-${probeSeq++}.js`, `
        const svc = require(${JSON.stringify(SERVICE_MODULE)});
        const emit = (v) => process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(v) + '\\n');
        ${probeBody}
    `);
    const argv = [runner];
    args.forEach((a) => argv.push('--config', a));
    const result = spawnSync(process.execPath, argv, {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    assert.equal(result.status, 0, `child failed: ${result.stderr}`);
    const line = result.stdout.split('\n').find((l) => l.startsWith('PROBE_JSON:'));
    assert.ok(line, `no PROBE_JSON in stdout: ${result.stdout}`);
    return JSON.parse(line.slice('PROBE_JSON:'.length));
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

// A complete, valid provisioning block. Each test below changes one thing about
// it, so a failure names the field that caused it rather than the whole payload.
const VALID = {
    type: 'thunderid',
    registrationEndpoint: 'https://idp.example.com/oauth2/register',
    auth: { method: 'client_credentials', clientId: 'cid', clientSecret: 'secret' },
};

const valid = (overrides = {}) => JSON.stringify({ ...VALID, ...overrides });

// ---------------------------------------------------------------------------
// _resolveKeyType
// ---------------------------------------------------------------------------

test('_resolveKeyType defaults to PRODUCTION when the caller says nothing', () => {
    // Absent is not invalid: the field was added after key managers already
    // existed, so every payload written against the earlier shape still applies.
    const result = runProbe(`
        emit([
            svc._resolveKeyType(undefined),
            svc._resolveKeyType(null),
            svc._resolveKeyType(''),
        ]);
    `);
    result.forEach((r) => assert.deepEqual(r, { keyType: 'PRODUCTION' }));
});

test('_resolveKeyType accepts both environments, case-insensitively and trimmed', () => {
    const result = runProbe(`
        emit([
            svc._resolveKeyType('PRODUCTION'),
            svc._resolveKeyType('SANDBOX'),
            svc._resolveKeyType('sandbox'),
            svc._resolveKeyType('  Sandbox  '),
        ]);
    `);
    assert.deepEqual(result, [
        { keyType: 'PRODUCTION' }, { keyType: 'SANDBOX' },
        { keyType: 'SANDBOX' }, { keyType: 'SANDBOX' },
    ]);
});

test('_resolveKeyType rejects an unknown environment by name', () => {
    const result = runProbe(`emit(svc._resolveKeyType('STAGING'));`);
    assert.match(result.error, /keyType must be one of: PRODUCTION, SANDBOX/);
});

test('_resolveKeyType rejects a non-string rather than coercing it', () => {
    // A number or an object here is a malformed request. Coercing would turn
    // `{}` into "[OBJECT OBJECT]" and report it as an unknown environment, which
    // describes the coercion rather than the fault.
    const result = runProbe(`
        emit([svc._resolveKeyType(1), svc._resolveKeyType({}), svc._resolveKeyType(true)]);
    `);
    result.forEach((r) => assert.match(r.error, /keyType must be one of/));
});

// ---------------------------------------------------------------------------
// _validateTokenEndpoint
// ---------------------------------------------------------------------------

test('_validateTokenEndpoint accepts a public https endpoint under either posture', () => {
    const strict = runProbe(`emit(svc._validateTokenEndpoint('https://idp.example.com/oauth2/token'));`);
    const permissive = runProbe(`emit(svc._validateTokenEndpoint('https://idp.example.com/oauth2/token'));`,
        { permissive: true });
    assert.equal(strict, null);
    assert.equal(permissive, null);
});

test('_validateTokenEndpoint refuses plaintext and private addresses by default', () => {
    // Both guards are off unless an operator turns them on, so a key manager
    // created through the API cannot reach inside the deployment on its own.
    const result = runProbe(`
        emit([
            svc._validateTokenEndpoint('http://idp.example.com/oauth2/token'),
            svc._validateTokenEndpoint('https://127.0.0.1:9443/oauth2/token'),
            svc._validateTokenEndpoint('https://10.0.0.5/oauth2/token'),
        ]);
    `);
    assert.match(result[0], /only https is permitted/);
    assert.match(result[1], /blocked address range/);
    assert.match(result[2], /blocked address range/);
});

test('_validateTokenEndpoint accepts plaintext and private addresses once opted in', () => {
    const result = runProbe(`
        emit([
            svc._validateTokenEndpoint('http://idp.example.com/oauth2/token'),
            svc._validateTokenEndpoint('https://127.0.0.1:9443/oauth2/token'),
            svc._validateTokenEndpoint('http://thunder:8090/oauth2/token'),
        ]);
    `, { permissive: true });
    assert.deepEqual(result, [null, null, null]);
});

test('_validateTokenEndpoint refuses link-local and metadata addresses under every posture', () => {
    /*
     * The one class the opt-in does not reach. 169.254.169.254 is the cloud
     * metadata endpoint on every major provider: a key manager pointed at it
     * would make the portal fetch instance credentials on an admin's behalf, so
     * no configuration flag may permit it.
     */
    const body = `
        emit([
            svc._validateTokenEndpoint('https://169.254.169.254/latest/meta-data/'),
            svc._validateTokenEndpoint('https://169.254.1.1/oauth2/token'),
        ]);
    `;
    // Strict AND permissive. Running the permissive probe twice -- which this
    // test did at first -- never exercises the default posture, so a regression
    // that let 169.254.169.254 through on a stock deployment would still pass.
    const postures = { strict: runProbe(body), permissive: runProbe(body, { permissive: true }) };
    for (const [posture, result] of Object.entries(postures)) {
        result.forEach((r) => assert.match(r, /blocked address range/,
            `expected a block under the ${posture} posture, got: ${r}`));
    }
});

test('_validateTokenEndpoint refuses a non-URL and a non-http scheme', () => {
    const result = runProbe(`
        emit([
            svc._validateTokenEndpoint('not-a-url'),
            svc._validateTokenEndpoint('file:///etc/passwd'),
            svc._validateTokenEndpoint('ftp://idp.example.com/token'),
        ]);
    `, { permissive: true });
    result.forEach((r) => assert.ok(r, 'expected a rejection message'));
});

// ---------------------------------------------------------------------------
// _declaredGrantTypes
// ---------------------------------------------------------------------------

test('_declaredGrantTypes reads the grant types a driver actually offers', () => {
    const result = runProbe(`emit(svc._declaredGrantTypes('thunderid').sort());`);
    assert.deepEqual(result, ['authorization_code', 'client_credentials', 'refresh_token']);
});

test('_declaredGrantTypes returns nothing for an unknown driver instead of throwing', () => {
    // An unreadable driver must not block configuring a key manager: the
    // restriction simply goes unvalidated here, and key creation still checks it
    // against live metadata.
    const result = runProbe(`emit([svc._declaredGrantTypes('no-such-driver'), svc._declaredGrantTypes('')]);`);
    assert.deepEqual(result, [[], []]);
});

// ---------------------------------------------------------------------------
// _validateProvisioning — shape
// ---------------------------------------------------------------------------

test('_validateProvisioning accepts a complete block and returns the stored shape', () => {
    const result = runProbe(`emit(svc._validateProvisioning(${valid()}));`);
    assert.equal(result.error, undefined);
    assert.equal(result.cfg.type, 'thunderid');
    assert.equal(result.cfg.authMethod, 'client_credentials');
    assert.equal(result.cfg.registrationEndpoint, 'https://idp.example.com/oauth2/register');
    // Absent optional fields are normalised to empty rather than undefined, so
    // the row written is the same shape whichever auth method was used.
    assert.equal(result.cfg.authorizeEndpoint, '');
    assert.deepEqual(result.cfg.supportedGrantTypes, []);
});

test('_validateProvisioning rejects an unknown driver type and lists what the build has', () => {
    const result = runProbe(`emit(svc._validateProvisioning(${valid({ type: 'not-a-driver' })}));`);
    assert.match(result.error, /Unknown key manager type "not-a-driver"/);
    assert.match(result.error, /This build provides:/);
});

test('_validateProvisioning rejects a missing or unknown auth method', () => {
    const result = runProbe(`
        emit([
            svc._validateProvisioning(${valid({ auth: {} })}),
            svc._validateProvisioning(${valid({ auth: { method: 'mtls' } })}),
            svc._validateProvisioning(${valid({ auth: { method: 'CLIENT_CREDENTIALS' } })}),
        ]);
    `);
    result.forEach((r) => assert.match(r.error, /auth\.method must be one of: client_credentials, basic, api_key/));
});

test('_validateProvisioning requires the registration endpoint', () => {
    const result = runProbe(`
        emit([
            svc._validateProvisioning(${valid({ registrationEndpoint: '' })}),
            svc._validateProvisioning(${valid({ registrationEndpoint: undefined })}),
        ]);
    `);
    result.forEach((r) => assert.match(r.error, /provisioning\.registrationEndpoint is required/));
});

test('_validateProvisioning treats the authorize endpoint as optional but still guarded', () => {
    const result = runProbe(`
        emit([
            svc._validateProvisioning(${valid({ authorizeEndpoint: '' })}).error,
            svc._validateProvisioning(${valid({ authorizeEndpoint: 'https://idp.example.com/authorize' })}).error,
            svc._validateProvisioning(${valid({ authorizeEndpoint: 'https://169.254.169.254/' })}).error,
        ]);
    `);
    assert.equal(result[0], null, 'omitted is allowed');
    assert.equal(result[1], null, 'a public https endpoint is allowed');
    assert.match(result[2], /blocked address range/, 'a supplied one is still checked');
});

test('_validateProvisioning applies the address guard to the registration endpoint', () => {
    const strict = runProbe(`
        emit([svc._validateProvisioning(${valid({ registrationEndpoint: 'https://127.0.0.1:9443/register' })}).error]);
    `);
    const permissive = runProbe(`
        emit([svc._validateProvisioning(${valid({ registrationEndpoint: 'https://127.0.0.1:9443/register' })}).error]);
    `, { permissive: true });
    assert.match(strict[0], /blocked address range/);
    assert.equal(permissive[0], null, 'permitted once the operator opts in');
});

// ---------------------------------------------------------------------------
// _validateProvisioning — credentials per auth method
// ---------------------------------------------------------------------------

test('_validateProvisioning names every missing credential field at once', () => {
    // One message listing both beats two round trips that each reveal one field.
    const result = runProbe(`
        emit(svc._validateProvisioning(${valid({ auth: { method: 'client_credentials' } })}).error);
    `);
    assert.match(result, /Missing required field\(s\) for auth\.method "client_credentials"/);
    assert.match(result, /auth\.clientId, auth\.clientSecret/);
});

test('_validateProvisioning requires username and password for basic auth', () => {
    const result = runProbe(`
        emit([
            svc._validateProvisioning(${valid({ auth: { method: 'basic' } })}).error,
            svc._validateProvisioning(${valid({ auth: { method: 'basic', username: 'u', password: 'p' } })}).error,
        ]);
    `);
    assert.match(result[0], /auth\.username, auth\.password/);
    assert.equal(result[1], undefined);
});

test('_validateProvisioning requires an api key and a header name for api_key auth', () => {
    const result = runProbe(`
        emit([
            svc._validateProvisioning(${valid({ auth: { method: 'api_key', headerName: 'X-Api-Key' } })}).error,
            svc._validateProvisioning(${valid({ auth: { method: 'api_key', apiKey: 'k', headerName: 'X-Api-Key' } })}).error,
            svc._validateProvisioning(${valid({ auth: { method: 'api_key', apiKey: 'k', headerName: 'Bad Header' } })}).error,
        ]);
    `);
    assert.match(result[0], /auth\.apiKey/);
    assert.equal(result[1], undefined);
    assert.ok(result[2], 'an invalid header name is rejected');
});

test('_validateProvisioning lets an update keep the stored credential of the same method', () => {
    /*
     * Omitting a secret on update means "keep the one you have" — the form cannot
     * show it back, so requiring it would mean retyping the secret to change the
     * display name. It is only honoured when the stored credential is of the same
     * method, or switching method would silently keep an unusable one.
     */
    const result = runProbe(`
        const held = { authMethod: 'client_credentials', hasClientSecret: true };
        emit([
            svc._validateProvisioning(${valid({ auth: { method: 'client_credentials', clientId: 'cid' } })},
                { isUpdate: true, existing: held }).error,
            svc._validateProvisioning(${valid({ auth: { method: 'client_credentials', clientId: 'cid' } })},
                { isUpdate: false, existing: held }).error,
            svc._validateProvisioning(${valid({ auth: { method: 'basic', username: 'u' } })},
                { isUpdate: true, existing: held }).error,
        ]);
    `);
    assert.equal(result[0], undefined, 'update with a stored secret of the same method');
    assert.match(result[1], /auth\.clientSecret/, 'create always needs the secret');
    assert.match(result[2], /auth\.password/, 'switching method needs the new credential');
});

// ---------------------------------------------------------------------------
// _validateProvisioning — grant-type restriction
// ---------------------------------------------------------------------------

test('_validateProvisioning accepts a restriction the driver offers', () => {
    const result = runProbe(`
        emit(svc._validateProvisioning(${valid({ supportedGrantTypes: ['client_credentials', 'refresh_token'] })}).cfg.supportedGrantTypes);
    `);
    assert.deepEqual(result, ['client_credentials', 'refresh_token']);
});

test('_validateProvisioning rejects a restriction naming a grant the driver does not offer', () => {
    // Accepting it would narrow the developer's dropdown to nothing and leave a
    // key manager no one can create a key on.
    const result = runProbe(`
        emit(svc._validateProvisioning(${valid({ supportedGrantTypes: ['password'] })}).error);
    `);
    assert.match(result, /does not offer: password/);
    assert.match(result, /It offers: authorization_code, refresh_token, client_credentials/);
});

test('_validateProvisioning treats an absent restriction as "whatever the driver offers"', () => {
    const result = runProbe(`
        emit([
            svc._validateProvisioning(${valid()}).cfg.supportedGrantTypes,
            svc._validateProvisioning(${valid({ supportedGrantTypes: null })}).cfg.supportedGrantTypes,
            svc._validateProvisioning(${valid({ supportedGrantTypes: [] })}).cfg.supportedGrantTypes,
        ]);
    `);
    assert.deepEqual(result, [[], [], []]);
});

test('_validateProvisioning rejects a non-array restriction', () => {
    const result = runProbe(`
        emit(svc._validateProvisioning(${valid({ supportedGrantTypes: 'client_credentials' })}).error);
    `);
    assert.match(result, /must be an array of grant type names/);
});

test('_validateProvisioning drops blank entries and trims the rest', () => {
    const result = runProbe(`
        emit(svc._validateProvisioning(${valid({ supportedGrantTypes: ['  client_credentials  ', '', '   ', null, 42] })}).cfg.supportedGrantTypes);
    `);
    assert.deepEqual(result, ['client_credentials']);
});
