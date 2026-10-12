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
 * Startup validation for auth.idp.jwks_url (configLoader.js#validateIdpConfig).
 *
 * The JWKS URL feeds token signature verification, so a plain http:// URL lets a
 * network attacker substitute the key set. The portal must refuse to start on one,
 * except for a development-only opt-in that is limited to localhost hosts.
 * Driven through a child process like internalAuthConfig.test.js, because the
 * check runs at module load and fails closed via process.exit.
 */

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const PROJECT_ROOT = path.join(__dirname, '..', '..');

const BASE_CONFIG = `
[api_portal.security]
encryption_key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
session_secret = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

[api_portal.organization]
handle = "default"
portal_id = "test-portal"

[api_portal.auth]
mode = "idp"

[api_portal.auth.idp]
client_id = "portal-client"
authorization_url = "https://idp.example.com/oauth2/authorize"
token_url = "https://idp.example.com/oauth2/token"
callback_url = "https://portal.example.com/callback"
`;

let tmpDir;
function fixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-jwks-config-'));
    const file = path.join(tmpDir, name);
    fs.writeFileSync(file, contents);
    return file;
}

function boot(jwksUrl, { insecureFlag } = {}) {
    const base = fixture('base.toml', BASE_CONFIG);
    const overlay = fixture(`overlay-${Buffer.from(jwksUrl).toString('hex')}.toml`,
        `[api_portal.auth.idp]\njwks_url = ${JSON.stringify(jwksUrl)}\n`);
    const runner = fixture('runner.js', `
        require(${JSON.stringify(path.join(__dirname, 'configLoader.js'))});
        process.stdout.write('BOOTED\\n');
    `);
    const env = { PATH: process.env.PATH, HOME: process.env.HOME };
    // Keeps the child's coverage in the parent run's report.
    if (process.env.NODE_V8_COVERAGE) env.NODE_V8_COVERAGE = process.env.NODE_V8_COVERAGE;
    if (insecureFlag !== undefined) env.APIP_AP_AUTH_IDP_DEV_INSECURE_JWKS = insecureFlag;
    const result = spawnSync(process.execPath, [runner, '--config', base, '--config', overlay], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env,
    });
    return { status: result.status, booted: result.stdout.includes('BOOTED'), stderr: result.stderr };
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

test('an https JWKS URL boots', () => {
    const { status, booted } = boot('https://idp.example.com/oauth2/jwks');
    assert.equal(status, 0);
    assert.ok(booted);
});

test('a plain http JWKS URL refuses to start by default', () => {
    const { status, booted, stderr } = boot('http://idp.example.com/oauth2/jwks');
    assert.equal(status, 1);
    assert.ok(!booted);
    assert.match(stderr, /jwks_url must use https:\/\//);
});

test('an unparseable JWKS URL refuses to start', () => {
    const { status, stderr } = boot('not a url');
    assert.equal(status, 1);
    assert.match(stderr, /jwks_url is not a valid URL/);
});

test('the dev escape allows http on localhost and *.localhost, with a warning', () => {
    for (const url of ['http://localhost:8090/oauth2/jwks', 'http://idp.dev.localhost:8090/oauth2/jwks']) {
        const { status, booted, stderr } = boot(url, { insecureFlag: 'true' });
        assert.equal(status, 0, url);
        assert.ok(booted, url);
        assert.match(stderr, /\[WARN\] auth\.idp\.jwks_url is plain http/, url);
    }
});

test('the dev escape never allows a remote host', () => {
    // A look-alike suffix must not pass for a .localhost host.
    for (const url of ['http://idp.example.com/oauth2/jwks', 'http://localhost.example.com/oauth2/jwks']) {
        const { status, stderr } = boot(url, { insecureFlag: 'true' });
        assert.equal(status, 1, url);
        assert.match(stderr, /jwks_url must use https:\/\//, url);
    }
});

test('the dev escape covers http only, not other schemes', () => {
    const { status } = boot('ftp://localhost/oauth2/jwks', { insecureFlag: 'true' });
    assert.equal(status, 1);
});

test('the dev escape needs the flag set to true', () => {
    const { status } = boot('http://localhost:8090/oauth2/jwks', { insecureFlag: 'yes' });
    assert.equal(status, 1);
});
