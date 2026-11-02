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
 * a whole number of at least 1. Driven through a child process because the loader runs its startup
 * checks at module load.
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
    assert.deepEqual(settings, { tolerance: 300, parallelPrices: 4 });
});

test('each setting can be changed', () => {
    const { status, settings } = boot(`
[api_portal.billing.stripe]
webhook_signature_tolerance_sec = 600

[api_portal.billing.moesif]
max_parallel_prices = 1
`);
    assert.equal(status, 0);
    assert.deepEqual(settings, { tolerance: 600, parallelPrices: 1 });
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
