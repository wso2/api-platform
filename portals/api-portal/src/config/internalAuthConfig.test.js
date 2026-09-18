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
 * Startup validation for [api_portal.internal_auth] (configLoader.js).
 *
 * Driven through a child process rather than by calling the validator directly:
 * the check is fail-closed via process.exit, and configLoader runs it as a side
 * effect of module load. Spawning is what lets the test assert the thing that
 * actually matters, that the portal REFUSES TO START, rather than that a
 * function returned an error object. Follows the same pattern as
 * authorizationConfig.test.js.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');
const VALID_HEX_HASH = 'a'.repeat(64);

// A config carrying only what the unrelated startup checks demand, so anything
// this suite observes comes from the internal-auth validation and nothing else.
const BASE_CONFIG = `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"
`;

let tmpDir;
function fixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-internalauth-config-'));
    const file = path.join(tmpDir, name);
    fs.writeFileSync(file, contents);
    return file;
}

// Stable per-content fixture name so concurrent tests don't clobber each other's overlay.
function hash(s) {
    let h = 0;
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
    return h;
}

function loadConfig(overlayToml) {
    const base = fixture('base.toml', BASE_CONFIG);
    const args = ['--config', base];
    if (overlayToml !== undefined) {
        args.push('--config', fixture(`overlay-${Math.abs(hash(overlayToml))}.toml`, overlayToml));
    }
    const runner = fixture('runner.js', `
        const { config } = require(${JSON.stringify(path.join(__dirname, 'configLoader.js'))});
        process.stdout.write('\\nINTERNAL_AUTH_JSON:' + JSON.stringify(config.internalAuth || null) + '\\n');
    `);
    const result = spawnSync(process.execPath, [runner, ...args], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME },
    });
    return { status: result.status, stdout: result.stdout, stderr: result.stderr };
}

function parseInternalAuth(stdout) {
    const line = String(stdout).split('\n').find(l => l.startsWith('INTERNAL_AUTH_JSON:'));
    assert.ok(line, `no INTERNAL_AUTH_JSON line in child stdout: ${stdout}`);
    return JSON.parse(line.slice('INTERNAL_AUTH_JSON:'.length));
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

test('defaults resolve to an empty hash (shared-key auth disabled)', () => {
    const { status, stdout } = loadConfig();
    assert.equal(status, 0);
    const internalAuth = parseInternalAuth(stdout);
    // Empty is fine, it means shared-key attempts are rejected at request time
    // and the OAuth / session paths keep working as if the feature was not enabled.
    assert.deepEqual(internalAuth, { hash: '' });
});

test('an explicitly empty hash boots (shared-key auth disabled)', () => {
    const { status, stdout } = loadConfig('[api_portal.internal_auth]\nhash = ""\n');
    assert.equal(status, 0);
    const internalAuth = parseInternalAuth(stdout);
    assert.equal(internalAuth.hash, '');
});

test('a valid 64-char hex hash boots and lands in config.internalAuth.hash', () => {
    const { status, stdout } = loadConfig(
        `[api_portal.internal_auth]\nhash = "${VALID_HEX_HASH}"\n`);
    assert.equal(status, 0);
    const internalAuth = parseInternalAuth(stdout);
    assert.equal(internalAuth.hash, VALID_HEX_HASH);
});

test('a hash shorter than 64 chars refuses to start', () => {
    const shortHash = 'abcd';
    const { status, stderr } = loadConfig(
        `[api_portal.internal_auth]\nhash = "${shortHash}"\n`);
    assert.equal(status, 1);
    assert.match(stderr, /internal_auth\.hash did not resolve to a 64-character hex string/);
});

test('a hash longer than 64 chars refuses to start', () => {
    const longHash = 'a'.repeat(65);
    const { status, stderr } = loadConfig(
        `[api_portal.internal_auth]\nhash = "${longHash}"\n`);
    assert.equal(status, 1);
    assert.match(stderr, /internal_auth\.hash did not resolve to a 64-character hex string/);
});

test('a hash containing non-hex characters refuses to start', () => {
    const badHash = 'z'.repeat(64);
    const { status, stderr } = loadConfig(
        `[api_portal.internal_auth]\nhash = "${badHash}"\n`);
    assert.equal(status, 1);
    assert.match(stderr, /internal_auth\.hash did not resolve to a 64-character hex string/);
});

test('the fatal message names the setup script as the recovery path', () => {
    // A malformed value is surfaced with actionable guidance rather than just a validation
    // failure, so an operator hitting this at boot knows what to do.
    const { status, stderr } = loadConfig('[api_portal.internal_auth]\nhash = "xyz"\n');
    assert.equal(status, 1);
    assert.match(stderr, /portals\/scripts\/setup\.sh/);
    assert.match(stderr, /--rotate-internal-key/);
});
