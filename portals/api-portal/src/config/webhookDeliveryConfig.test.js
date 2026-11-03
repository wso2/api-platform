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
 * webhooks.delivery.tls_skip_verify is announced in the startup log whenever it is
 * on, the same way the key manager's insecure_skip_verify is. Driven through a child
 * process because the loader runs its startup checks at module load.
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
function fixture(name, contents) {
    if (!tmpDir) tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ap-webhook-delivery-config-'));
    const file = path.join(tmpDir, name);
    fs.writeFileSync(file, contents);
    return file;
}

// tlsSkipVerify is the TOML value text for tls_skip_verify, or undefined to leave it out.
function boot(tlsSkipVerify) {
    const base = fixture('base.toml', BASE_CONFIG);
    const args = ['--config', base];
    if (tlsSkipVerify !== undefined) {
        args.push('--config', fixture(`overlay-${tlsSkipVerify}.toml`,
            `[api_portal.webhooks.delivery]\ntls_skip_verify = ${tlsSkipVerify}\n`));
    }
    const runner = fixture('runner.js', `
        require(${JSON.stringify(path.join(__dirname, 'configLoader.js'))});
        process.stdout.write('BOOTED\\n');
    `);
    const env = { PATH: process.env.PATH, HOME: process.env.HOME };
    // Keeps the child's coverage in the parent run's report.
    if (process.env.NODE_V8_COVERAGE) env.NODE_V8_COVERAGE = process.env.NODE_V8_COVERAGE;
    const result = spawnSync(process.execPath, [runner, ...args], { cwd: PROJECT_ROOT, encoding: 'utf8', env });
    return { status: result.status, booted: result.stdout.includes('BOOTED'), stderr: result.stderr };
}

test.after(() => {
    if (tmpDir) fs.rmSync(tmpDir, { recursive: true, force: true });
});

const WARNING = /\[WARN\] webhooks\.delivery\.tls_skip_verify = true/;

test('enabling the TLS skip boots with a warning', () => {
    const { status, booted, stderr } = boot('true');
    assert.equal(status, 0);
    assert.ok(booted);
    assert.match(stderr, WARNING);
});

test('no warning when the TLS skip is off or unset', () => {
    for (const value of ['false', undefined]) {
        const { status, stderr } = boot(value);
        assert.equal(status, 0, String(value));
        assert.doesNotMatch(stderr, WARNING, String(value));
    }
});
