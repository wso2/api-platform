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
 * Unit tests for the two parts of src/services/oauth2KeyService.js that decide
 * whether a developer's submission is allowed: the property contract, and the
 * narrowing an admin's key manager settings apply to it.
 *
 * These are worth asserting directly rather than only through a route, because
 * together they are what makes a grant-type restriction real. A dropdown
 * narrowed in the browser is a suggestion; `_applyKeyManagerSettings` is what
 * narrows the advertised options and `_validateProperties` is what refuses a
 * submission that ignores them. A gap between the two is silent: the form
 * advertises a restriction the server does not enforce.
 *
 * Driven through a child process (same pattern as sharedKeyAuth.test.js) because
 * the service requires configLoader, which fail-closes on module load without a
 * --config argument.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const SERVICE_MODULE = path.join(__dirname, 'oauth2KeyService.js');

// Only what the startup checks demand. Nothing here reaches the database: both
// functions under test are pure, and the module's DB handles are opened lazily.
const BASE_CONFIG = `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"
`;

let tmpDir;
let probeSeq = 0;

function fixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-oauth2key-'));
    const file = path.join(tmpDir, name);
    fs.writeFileSync(file, contents);
    return file;
}

/**
 * Runs `probeBody` in a child that has loaded the service. The body has `svc`
 * bound to the module and `emit(v)` to a marker-prefixed JSON emitter.
 */
function runProbe(probeBody) {
    const base = fixture('base.toml', BASE_CONFIG);
    const runner = fixture(`probe-${probeSeq++}.js`, `
        const svc = require(${JSON.stringify(SERVICE_MODULE)});
        const emit = (v) => process.stdout.write('\\nPROBE_JSON:' + JSON.stringify(v) + '\\n');
        ${probeBody}
    `);
    const result = spawnSync(process.execPath, [runner, '--config', base], {
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

// A driver's metadata, in the shape the drivers actually emit: RFC 7591 member
// names, `required` on the ones the server insists on, and `options` only where
// the value is drawn from a fixed set.
const META = {
    properties: [
        { name: 'client_name', label: 'Name', type: 'string', required: true },
        { name: 'redirect_uris', label: 'Callback URLs', type: 'string_list' },
        {
            name: 'grant_types',
            label: 'Grant types',
            type: 'multiselect',
            options: [
                { value: 'client_credentials', label: 'Client credentials' },
                { value: 'authorization_code', label: 'Authorization code' },
                { value: 'refresh_token', label: 'Refresh token' },
            ],
        },
    ],
};

const metaLiteral = JSON.stringify(META);

// ---------------------------------------------------------------------------
// _validateProperties
// ---------------------------------------------------------------------------

test('_validateProperties accepts a submission that matches the declared contract', () => {
    const result = runProbe(`
        emit(svc._validateProperties(${metaLiteral}, {
            client_name: 'checkout',
            redirect_uris: ['https://app.example.com/cb'],
            grant_types: ['client_credentials'],
        }));
    `);
    assert.equal(result, null);
});

test('_validateProperties accepts anything when the driver declares no properties', () => {
    // No contract to check against is not the same as an empty contract: a driver
    // that declares nothing takes the bag as-is rather than rejecting every key.
    const result = runProbe(`
        emit([
            svc._validateProperties({}, { anything: 'goes' }),
            svc._validateProperties({ properties: [] }, { anything: 'goes' }),
            svc._validateProperties(null, { anything: 'goes' }),
        ]);
    `);
    assert.deepEqual(result, [null, null, null]);
});

test('_validateProperties rejects a property the driver does not declare', () => {
    const result = runProbe(`
        emit(svc._validateProperties(${metaLiteral}, { client_name: 'x', nope: 1 }));
    `);
    assert.match(result, /does not accept the property: nope/);
});

test('_validateProperties names every undeclared property, pluralised', () => {
    const result = runProbe(`
        emit(svc._validateProperties(${metaLiteral}, { client_name: 'x', nope: 1, alsoNope: 2 }));
    `);
    assert.match(result, /does not accept the properties: nope, alsoNope/);
});

test('_validateProperties rejects a missing required property', () => {
    const result = runProbe(`
        emit(svc._validateProperties(${metaLiteral}, { redirect_uris: ['https://a/cb'] }));
    `);
    assert.match(result, /Missing required property: client_name/);
});

test('_validateProperties treats empty string, null and empty array as missing', () => {
    // The three ways a form can submit "nothing" for a required field. An empty
    // array matters most: a cleared list field posts [] rather than being absent.
    const result = runProbe(`
        emit([
            svc._validateProperties(${metaLiteral}, { client_name: '' }),
            svc._validateProperties(${metaLiteral}, { client_name: null }),
            svc._validateProperties(${metaLiteral}, { client_name: [] }),
        ]);
    `);
    result.forEach((r) => assert.match(r, /Missing required property: client_name/));
});

test('_validateProperties ignores a non-object properties bag rather than throwing', () => {
    // An array or a string reaching here is a malformed request, not a crash: it
    // is treated as "nothing submitted", so the required-property check reports it.
    const result = runProbe(`
        emit([
            svc._validateProperties(${metaLiteral}, []),
            svc._validateProperties(${metaLiteral}, 'nonsense'),
            svc._validateProperties(${metaLiteral}, undefined),
        ]);
    `);
    result.forEach((r) => assert.match(r, /Missing required property: client_name/));
});

test('_validateProperties rejects a value outside a declared option set', () => {
    const result = runProbe(`
        emit(svc._validateProperties(${metaLiteral}, {
            client_name: 'x', grant_types: ['password'],
        }));
    `);
    assert.match(result, /"grant_types" does not accept: password/);
    // The permitted list is named, so the caller can correct the request without
    // a second round trip.
    assert.match(result, /Permitted here: client_credentials, authorization_code, refresh_token/);
});

test('_validateProperties checks every member of a multiselect, not just the first', () => {
    const result = runProbe(`
        emit(svc._validateProperties(${metaLiteral}, {
            client_name: 'x', grant_types: ['client_credentials', 'password'],
        }));
    `);
    assert.match(result, /does not accept: password/);
});

test('_validateProperties applies the option check to a single value as well as an array', () => {
    const single = JSON.stringify({
        properties: [{
            name: 'token_endpoint_auth_method',
            type: 'select',
            options: [{ value: 'client_secret_basic' }, { value: 'client_secret_post' }],
        }],
    });
    const result = runProbe(`
        emit([
            svc._validateProperties(${single}, { token_endpoint_auth_method: 'client_secret_basic' }),
            svc._validateProperties(${single}, { token_endpoint_auth_method: 'none' }),
        ]);
    `);
    assert.equal(result[0], null);
    assert.match(result[1], /does not accept: none/);
});

test('_validateProperties leaves an optional property with options unset', () => {
    // Unset is not a rejected value: only what was actually submitted is checked
    // against the option list, or an optional select could never be left blank.
    const result = runProbe(`
        emit([
            svc._validateProperties(${metaLiteral}, { client_name: 'x' }),
            svc._validateProperties(${metaLiteral}, { client_name: 'x', grant_types: '' }),
        ]);
    `);
    assert.deepEqual(result, [null, null]);
});

test('_validateProperties reports an undeclared property before a missing required one', () => {
    // Order matters for the message the caller sees: an unknown key usually means
    // they are posting against the wrong key manager, which is the more useful
    // thing to say first.
    const result = runProbe(`
        emit(svc._validateProperties(${metaLiteral}, { bogus: 1 }));
    `);
    assert.match(result, /does not accept the property: bogus/);
});

// ---------------------------------------------------------------------------
// _applyKeyManagerSettings
// ---------------------------------------------------------------------------

test('_applyKeyManagerSettings narrows grant_types to the admin-permitted set', () => {
    const result = runProbe(`
        const out = svc._applyKeyManagerSettings(
            ${metaLiteral},
            { supportedGrantTypes: ['client_credentials', 'refresh_token'] },
            { handle: 'km-1', key_type: 'PRODUCTION' }
        );
        emit(out.properties.find((p) => p.name === 'grant_types').options.map((o) => o.value));
    `);
    assert.deepEqual(result, ['client_credentials', 'refresh_token']);
});

test('_applyKeyManagerSettings leaves other properties untouched while narrowing', () => {
    const result = runProbe(`
        const out = svc._applyKeyManagerSettings(
            ${metaLiteral},
            { supportedGrantTypes: ['client_credentials'] },
            { handle: 'km-1' }
        );
        emit(out.properties.map((p) => p.name));
    `);
    assert.deepEqual(result, ['client_name', 'redirect_uris', 'grant_types']);
});

test('_applyKeyManagerSettings returns the driver metadata unchanged with no restriction', () => {
    const result = runProbe(`
        const out = svc._applyKeyManagerSettings(${metaLiteral}, null, { handle: 'km-1' });
        const out2 = svc._applyKeyManagerSettings(${metaLiteral}, { supportedGrantTypes: [] }, { handle: 'km-1' });
        emit([
            out.properties.find((p) => p.name === 'grant_types').options.length,
            out2.properties.find((p) => p.name === 'grant_types').options.length,
        ]);
    `);
    assert.deepEqual(result, [3, 3]);
});

test('_applyKeyManagerSettings ignores a restriction matching none of the driver options', () => {
    /*
     * A restriction naming only grant types this driver does not offer would
     * otherwise narrow the list to nothing, leaving a form with an empty dropdown
     * and no way to submit. Ignoring it keeps the key manager usable; the
     * mismatch is logged rather than enforced into a dead end.
     */
    const result = runProbe(`
        const out = svc._applyKeyManagerSettings(
            ${metaLiteral},
            { supportedGrantTypes: ['urn:ietf:params:oauth:grant-type:device_code'] },
            { handle: 'km-1' }
        );
        emit(out.properties.find((p) => p.name === 'grant_types').options.map((o) => o.value));
    `);
    assert.deepEqual(result, ['client_credentials', 'authorization_code', 'refresh_token']);
});

test('_applyKeyManagerSettings carries the key manager key type onto the metadata', () => {
    const result = runProbe(`
        emit([
            svc._applyKeyManagerSettings(${metaLiteral}, null, { key_type: 'SANDBOX' }).keyType,
            svc._applyKeyManagerSettings(${metaLiteral}, null, { key_type: 'PRODUCTION' }).keyType,
        ]);
    `);
    assert.deepEqual(result, ['SANDBOX', 'PRODUCTION']);
});

test('_applyKeyManagerSettings defaults the key type to PRODUCTION when absent', () => {
    // A config-declared key manager has no row to read key_type from, and a key
    // manager stored before the column existed has none either. Neither may come
    // out undefined: the value is stamped onto every key created through it.
    const result = runProbe(`
        emit([
            svc._applyKeyManagerSettings(${metaLiteral}, null, {}).keyType,
            svc._applyKeyManagerSettings(${metaLiteral}, null, null).keyType,
            svc._applyKeyManagerSettings(${metaLiteral}, null, undefined).keyType,
        ]);
    `);
    assert.deepEqual(result, ['PRODUCTION', 'PRODUCTION', 'PRODUCTION']);
});

test('_applyKeyManagerSettings does not mutate the metadata it was given', () => {
    // The driver's metadata object is shared across callers — it is emitted once
    // by the driver and read per request — so narrowing has to copy rather than
    // edit, or one tenant's restriction would leak into every later reader.
    const result = runProbe(`
        const meta = ${metaLiteral};
        svc._applyKeyManagerSettings(meta, { supportedGrantTypes: ['client_credentials'] }, { handle: 'km' });
        emit(meta.properties.find((p) => p.name === 'grant_types').options.map((o) => o.value));
    `);
    assert.deepEqual(result, ['client_credentials', 'authorization_code', 'refresh_token']);
});

test('_applyKeyManagerSettings tolerates metadata with no properties array', () => {
    const result = runProbe(`
        emit([
            svc._applyKeyManagerSettings({}, { supportedGrantTypes: ['x'] }, {}).keyType,
            svc._applyKeyManagerSettings({ properties: null }, { supportedGrantTypes: ['x'] }, {}).keyType,
        ]);
    `);
    assert.deepEqual(result, ['PRODUCTION', 'PRODUCTION']);
});

// ---------------------------------------------------------------------------
// The two together
// ---------------------------------------------------------------------------

test('a grant type narrowed away by settings is refused by validation', () => {
    /*
     * The pair's whole purpose, asserted end to end: what the metadata endpoint
     * advertises after narrowing is exactly what the create path will accept. If
     * these two ever disagree, the restriction becomes advisory — the form hides
     * an option the server still takes from a hand-written request.
     */
    const result = runProbe(`
        const narrowed = svc._applyKeyManagerSettings(
            ${metaLiteral},
            { supportedGrantTypes: ['client_credentials'] },
            { handle: 'km-1' }
        );
        emit({
            advertised: narrowed.properties.find((p) => p.name === 'grant_types').options.map((o) => o.value),
            permitted: svc._validateProperties(narrowed, { client_name: 'x', grant_types: ['client_credentials'] }),
            refused: svc._validateProperties(narrowed, { client_name: 'x', grant_types: ['authorization_code'] }),
        });
    `);
    assert.deepEqual(result.advertised, ['client_credentials']);
    assert.equal(result.permitted, null, 'the advertised grant type must be accepted');
    assert.match(result.refused, /does not accept: authorization_code/);
});
