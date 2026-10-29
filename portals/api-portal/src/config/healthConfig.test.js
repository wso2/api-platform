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
 * Startup validation for [api_portal.health] and [api_portal.health.database]
 * (configLoader.js).
 *
 * Driven through a child process, like internalAuthConfig.test.js: the checks are
 * fail-closed via process.exit as a side effect of module load, so spawning is what
 * lets the test assert that the portal refuses to start.
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
`;

let tmpDir;
let fixtureCount = 0;
function fixture(contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-health-config-'));
    fixtureCount += 1;
    const file = path.join(tmpDir, `config-${fixtureCount}.toml`);
    fs.writeFileSync(file, contents);
    return file;
}

function loadConfig(overlayToml, env = {}) {
    const args = ['--config', fixture(BASE_CONFIG)];
    if (overlayToml !== undefined) args.push('--config', fixture(overlayToml));
    const runner = path.join(tmpDir, 'runner.js');
    fs.writeFileSync(runner, `
        const { config } = require(${JSON.stringify(path.join(__dirname, 'configLoader.js'))});
        process.stdout.write('\\nHEALTH_JSON:' + JSON.stringify(config.health) + '\\n');
    `);
    const result = spawnSync(process.execPath, [runner, ...args], {
        cwd: PROJECT_ROOT,
        encoding: 'utf8',
        env: { PATH: process.env.PATH, HOME: process.env.HOME, ...env },
    });
    return { status: result.status, stdout: result.stdout, stderr: result.stderr };
}

function parseHealth(stdout) {
    const line = String(stdout).split('\n').find((l) => l.startsWith('HEALTH_JSON:'));
    assert.ok(line, `no HEALTH_JSON line in child stdout: ${stdout}`);
    return JSON.parse(line.slice('HEALTH_JSON:'.length));
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

test('defaults apply when no health section is given', () => {
    const { status, stdout, stderr } = loadConfig();
    assert.equal(status, 0, stderr);
    assert.deepStrictEqual(parseHealth(stdout), {
        cacheTtl: '3s',
        cacheTtlMs: 3000,
        allowedClients: [],
        database: {
            checkTimeout: '2s', checkTimeoutMs: 2000,
            failureWindow: '30s', failureWindowMs: 30000,
            failureRatio: 0.5,
            minAttempts: 5,
            idlePeriod: '10s', idlePeriodMs: 10000,
            poolSaturation: 0.9,
            poolWaitLimit: '5s', poolWaitLimitMs: 5000,
        },
    });
});

test('every setting can be overridden', () => {
    const { status, stdout, stderr } = loadConfig(`
[api_portal.health]
cache_ttl = "1s"
allowed_clients = ["127.0.0.1", "10.0.0.0/8"]

[api_portal.health.database]
check_timeout   = "500ms"
failure_window  = "1m"
failure_ratio   = 1
min_attempts    = 3
idle_period     = "20s"
pool_saturation = 0.75
pool_wait_limit = "2s"
`);
    assert.equal(status, 0, stderr);
    const health = parseHealth(stdout);
    assert.equal(health.cacheTtlMs, 1000);
    assert.deepStrictEqual(health.allowedClients, ['127.0.0.1', '10.0.0.0/8']);
    assert.equal(health.database.checkTimeoutMs, 500);
    assert.equal(health.database.failureWindowMs, 60000);
    assert.equal(health.database.failureRatio, 1);
    assert.equal(health.database.minAttempts, 3);
    assert.equal(health.database.idlePeriodMs, 20000);
    assert.equal(health.database.poolSaturation, 0.75);
    assert.equal(health.database.poolWaitLimitMs, 2000);
});

test('allowed_clients accepts a comma-separated string from an env var', () => {
    const { status, stdout, stderr } = loadConfig(
        '[api_portal.health]\nallowed_clients = \'{{ env "APIP_AP_HEALTH_ALLOWED_CLIENTS" "127.0.0.1,::1" }}\'\n',
        { APIP_AP_HEALTH_ALLOWED_CLIENTS: '127.0.0.1, ::1 ,10.0.0.0/8,' },
    );
    assert.equal(status, 0, stderr);
    assert.deepStrictEqual(parseHealth(stdout).allowedClients, ['127.0.0.1', '::1', '10.0.0.0/8']);
});

test('numeric settings coming from env vars are accepted', () => {
    const { status, stdout, stderr } = loadConfig(
        '[api_portal.health.database]\nfailure_ratio = \'{{ env "X_RATIO" "0.5" }}\'\nmin_attempts = \'{{ env "X_MIN" "5" }}\'\n',
        { X_RATIO: '0.25', X_MIN: '8' },
    );
    assert.equal(status, 0, stderr);
    const health = parseHealth(stdout);
    assert.equal(health.database.failureRatio, 0.25);
    assert.equal(health.database.minAttempts, 8);
});

const REFUSED = [
    ['an invalid allowed client', '[api_portal.health]\nallowed_clients = "127.0.0.1,nope"\n',
        /api_portal\.health\.allowed_clients\[1\] is not a valid IP address or CIDR range/],
    ['"*" with other clients', '[api_portal.health]\nallowed_clients = ["*", "127.0.0.1"]\n',
        /api_portal\.health\.allowed_clients: "\*" cannot be combined with other entries/],
    ['a non-list allowed_clients', '[api_portal.health]\nallowed_clients = 5\n',
        /api_portal\.health\.allowed_clients must be a list/],
    ['a zero cache_ttl', '[api_portal.health]\ncache_ttl = "0s"\n',
        /api_portal\.health\.cache_ttl must be a positive duration/],
    ['a cache_ttl without a unit', '[api_portal.health]\ncache_ttl = 3\n',
        /api_portal\.health\.cache_ttl must be a positive duration/],
    ['a malformed check_timeout', '[api_portal.health.database]\ncheck_timeout = "soon"\n',
        /api_portal\.health\.database\.check_timeout must be a positive duration/],
    ['a zero pool_wait_limit', '[api_portal.health.database]\npool_wait_limit = "0s"\n',
        /api_portal\.health\.database\.pool_wait_limit must be a positive duration/],
    ['a failure_ratio above 1', '[api_portal.health.database]\nfailure_ratio = 1.5\n',
        /api_portal\.health\.database\.failure_ratio must be a number greater than 0 and at most 1/],
    ['a zero pool_saturation', '[api_portal.health.database]\npool_saturation = 0\n',
        /api_portal\.health\.database\.pool_saturation must be a number greater than 0 and at most 1/],
    ['a fractional min_attempts', '[api_portal.health.database]\nmin_attempts = 2.5\n',
        /api_portal\.health\.database\.min_attempts must be a whole number of at least 1/],
    ['a failure_window shorter than cache_ttl', '[api_portal.health]\ncache_ttl = "10s"\n[api_portal.health.database]\nfailure_window = "5s"\nidle_period = "5s"\n',
        /failure_window \(5s\) must not be shorter than api_portal\.health\.cache_ttl \(10s\)/],
    ['an idle_period longer than failure_window', '[api_portal.health.database]\nidle_period = "1m"\n',
        /idle_period \(1m0s\) must not be longer than api_portal\.health\.database\.failure_window \(30s\)/],
    ['a misspelt database key', '[api_portal.health.database]\nfaliure_ratio = 0.5\n',
        /api_portal\.health\.database: unknown key "faliure_ratio"/],
    ['a camelCase database key', '[api_portal.health.database]\ncheckTimeout = "2s"\n',
        /api_portal\.health\.database: unknown key "checkTimeout"/],
    ['a health section that is not a table', '[api_portal]\nhealth = "on"\n',
        /\[api_portal\.health\] and \[api_portal\.health\.database\] must be tables/],
];

for (const [name, toml, message] of REFUSED) {
    test(`${name} refuses to start`, () => {
        const { status, stderr } = loadConfig(toml);
        assert.equal(status, 1, stderr);
        assert.match(stderr, /\[FATAL\]/);
        assert.match(stderr, message);
    });
}

test('a slow check_timeout starts with a warning', () => {
    const { status, stderr } = loadConfig('[api_portal.health.database]\ncheck_timeout = "5s"\n');
    assert.equal(status, 0, stderr);
    assert.match(stderr, /\[WARN\] api_portal\.health\.database\.check_timeout is 5s/);
});
