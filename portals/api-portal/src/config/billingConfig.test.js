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
 * [api_portal.billing.stripe] webhook_signature_tolerance_sec and [api_portal.billing.moesif]
 * max_parallel_prices: their defaults, an override, and the refusal to start with a value that is not
 * a whole number of at least 1. [api_portal.billing.moesif] collector_application_id and
 * management_key: set together or not at all, and not in multi-organization mode. Driven through a
 * child process because the loader runs its startup checks at module load.
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
let fixtures = 0;
function fixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-billing-config-'));
    const file = path.join(tmpDir, `${++fixtures}-${name}`);
    fs.writeFileSync(file, contents);
    return file;
}

const SETTINGS = 'SETTINGS ';

// overlay is TOML laid over the base config, or undefined for the defaults. Resolves to the loaded
// billing settings, or the exit status and error output when the portal refuses to start.
function boot(overlay) {
    const args = ['--config', fixture('base.toml', BASE_CONFIG)];
    if (overlay !== undefined) args.push('--config', fixture('overlay.toml', overlay));
    const runner = fixture('runner.js', `
        const { config } = require(${JSON.stringify(path.join(__dirname, 'configLoader.js'))});
        process.stdout.write(${JSON.stringify(SETTINGS)} + JSON.stringify({
            tolerance: config.billing.stripe.webhookSignatureToleranceSec,
            parallelPrices: config.billing.moesif.maxParallelPrices,
            moesifKeys: [config.billing.moesif.collectorApplicationId, config.billing.moesif.managementKey],
        }) + '\\n');
    `);
    const env = { PATH: process.env.PATH, HOME: process.env.HOME };
    // Keeps the child's coverage in the parent run's report.
    if (process.env.NODE_V8_COVERAGE) env.NODE_V8_COVERAGE = process.env.NODE_V8_COVERAGE;
    const result = spawnSync(process.execPath, [runner, ...args], { cwd: PROJECT_ROOT, encoding: 'utf8', env });
    const line = result.stdout.split('\n').find((l) => l.startsWith(SETTINGS));
    return {
        status: result.status,
        settings: line ? JSON.parse(line.slice(SETTINGS.length)) : null,
        stderr: result.stderr,
    };
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

test('the defaults are a 300 second signature tolerance and 4 prices at a time', () => {
    const { status, settings } = boot();
    assert.equal(status, 0);
    assert.deepEqual(settings, { tolerance: 300, parallelPrices: 4, moesifKeys: ['', ''] });
});

test('each setting can be changed', () => {
    const { status, settings } = boot(`
[api_portal.billing.stripe]
webhook_signature_tolerance_sec = 600

[api_portal.billing.moesif]
max_parallel_prices = 1
`);
    assert.equal(status, 0);
    assert.deepEqual(settings, { tolerance: 600, parallelPrices: 1, moesifKeys: ['', ''] });
});

test('a value that is not a whole number of at least 1 stops the portal from starting', () => {
    const cases = [
        ['stripe', 'webhook_signature_tolerance_sec', '0'],
        ['stripe', 'webhook_signature_tolerance_sec', '"300"'],
        ['moesif', 'max_parallel_prices', '0'],
        ['moesif', 'max_parallel_prices', '-2'],
        ['moesif', 'max_parallel_prices', '2.5'],
    ];
    for (const [section, key, value] of cases) {
        const { status, stderr } = boot(`[api_portal.billing.${section}]\n${key} = ${value}\n`);
        assert.equal(status, 1, `${key} = ${value}`);
        assert.match(stderr, new RegExp(`\\[FATAL\\] billing\\.${section}\\.${key} must be a whole number of at least 1`),
            `${key} = ${value}`);
    }
});

const MOESIF_KEYS = `
[api_portal.billing.moesif]
collector_application_id = "  eyJhcHAiOiI2NDQ3OjEyMyJ9  "
management_key = "moesif-key-0000"
`;

const MULTI_ORGANIZATION = `
[api_portal.auth]
mode = "idp"

[api_portal.auth.idp]
client_id = "portal-client"
audience = "portal-client"
authorization_url = "https://idp.example.com/oauth2/authorize"
token_url = "https://idp.example.com/oauth2/token"
callback_url = "https://portal.example.com/callback"
jwks_url = "https://idp.example.com/oauth2/jwks"

[api_portal.multi_organization]
enabled = true
`;

test('the Moesif keys set in config are loaded, trimmed', () => {
    const { status, settings } = boot(MOESIF_KEYS);
    assert.equal(status, 0);
    assert.deepEqual(settings.moesifKeys, ['eyJhcHAiOiI2NDQ3OjEyMyJ9', 'moesif-key-0000']);
});

test('only one Moesif key set stops the portal from starting', () => {
    for (const key of ['collector_application_id', 'management_key']) {
        const { status, stderr } = boot(`[api_portal.billing.moesif]\n${key} = "moesif-0000"\n`);
        assert.equal(status, 1, key);
        assert.match(stderr, /\[FATAL\] billing\.moesif\.collector_application_id and billing\.moesif\.management_key must be set together/);
    }
});

test('a Moesif key that is not a string stops the portal from starting', () => {
    const { status, stderr } = boot('[api_portal.billing.moesif]\nmanagement_key = 42\n');
    assert.equal(status, 1);
    assert.match(stderr, /\[FATAL\] billing\.moesif\.management_key must be a string/);
});

// A management key shaped like Moesif's, naming its application in the `app` claim.
const segment = (value) => Buffer.from(JSON.stringify(value)).toString('base64url');
const managementKeyFor = (app) => `${segment({ alg: 'HS256' })}.${segment({ app })}.signature`;
const keysFor = (collectorId, managementKey) =>
    `[api_portal.billing.moesif]\ncollector_application_id = "${collectorId}"\nmanagement_key = "${managementKey}"\n`;

test('a management key for the collector id\'s Moesif application is loaded', () => {
    const { status } = boot(keysFor(segment({ app: '6447:123' }), managementKeyFor('6447:123')));
    assert.equal(status, 0);
});

test('a management key for another Moesif application stops the portal from starting', () => {
    const { status, stderr } = boot(keysFor(segment({ app: '6447:123' }), managementKeyFor('6447:999')));
    assert.equal(status, 1);
    assert.match(stderr, /\[FATAL\] billing\.moesif\.management_key is for a different Moesif application than billing\.moesif\.collector_application_id/);
});

test('a value that is not a Collector Application Id stops the portal from starting', () => {
    const { status, stderr } = boot(keysFor('moesif-0000', 'moesif-key-0000'));
    assert.equal(status, 1);
    assert.match(stderr, /\[FATAL\] billing\.moesif\.collector_application_id is not a Moesif Collector Application Id/);
});

test('Moesif keys in config stop a multi-organization portal from starting', () => {
    const { status, stderr } = boot(MULTI_ORGANIZATION + MOESIF_KEYS);
    assert.equal(status, 1);
    assert.match(stderr, /\[FATAL\] billing\.moesif\.collector_application_id and billing\.moesif\.management_key are not allowed in multi-organization mode/);
});

test('a multi-organization portal without Moesif keys in config starts', () => {
    const { status, settings } = boot(MULTI_ORGANIZATION);
    assert.equal(status, 0);
    assert.deepEqual(settings.moesifKeys, ['', '']);
});
