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

const test = require('node:test');
const assert = require('node:assert');

const {
    slugifyHandle,
    handleCandidate,
    uuidHandle,
    DEFAULT_MAX_HANDLE_LENGTH,
} = require('./handleSlug');

test('slugifyHandle produces the same shape the settings UI used to generate client-side', () => {
    // Handles created before generation moved server-side were slugged in the browser
    // with this exact transform; a divergence here would make new handles inconsistent
    // with existing stored ones.
    assert.strictEqual(slugifyHandle('Production listener'), 'production-listener');
    assert.strictEqual(slugifyHandle('Staging / EU'), 'staging-eu');
    assert.strictEqual(slugifyHandle('  Prod  Gateway  '), 'prod-gateway');
    assert.strictEqual(slugifyHandle('Gateway 2'), 'gateway-2');
});

test('slugifyHandle never emits a leading or trailing separator', () => {
    for (const name of ['-leading', 'trailing-', '--both--', '  /weird/  ', '...dots...']) {
        const slug = slugifyHandle(name);
        assert.ok(!slug.startsWith('-'), `${JSON.stringify(name)} -> ${JSON.stringify(slug)} starts with '-'`);
        assert.ok(!slug.endsWith('-'), `${JSON.stringify(name)} -> ${JSON.stringify(slug)} ends with '-'`);
    }
});

test('slugifyHandle returns empty for names with nothing slugifiable, so callers apply a fallback', () => {
    // Deliberately empty rather than a generated value: the service falls back to a
    // UUID handle, and silently inventing one here would hide that decision.
    for (const name of ['★★★', '___', '   ', '', null, undefined]) {
        assert.strictEqual(slugifyHandle(name), '', `expected empty slug for ${JSON.stringify(name)}`);
    }
});

test('slugifyHandle caps length and does not leave a dash from the cut', () => {
    const long = slugifyHandle('a'.repeat(300));
    assert.strictEqual(long.length, DEFAULT_MAX_HANDLE_LENGTH);

    // Cutting mid-separator must not leave a trailing dash behind.
    const cut = slugifyHandle('ab '.repeat(200), 8);
    assert.ok(!cut.endsWith('-'), `${JSON.stringify(cut)} ends with '-'`);
    assert.ok(cut.length <= 8);

    // The cap must leave room for the collision suffix inside VARCHAR(255).
    assert.ok(handleCandidate(long, 99).length < 255);
});

test('handleCandidate gives the clean handle first, then numeric suffixes', () => {
    assert.strictEqual(handleCandidate('prod', 0), 'prod');
    assert.strictEqual(handleCandidate('prod', 1), 'prod-2');
    assert.strictEqual(handleCandidate('prod', 2), 'prod-3');
});

test('handleCandidate output stays a valid slug', () => {
    const base = slugifyHandle('Production listener');
    for (let attempt = 0; attempt < 5; attempt++) {
        assert.match(handleCandidate(base, attempt), /^[a-z0-9]+(-[a-z0-9]+)*$/);
    }
});

test('uuidHandle returns a plain UUID', () => {
    assert.match(uuidHandle(), /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
});

test('uuidHandle does not repeat itself', () => {
    // The whole point of the UUID fallback is that it succeeds where the numeric
    // ladder has already collided, so two calls returning the same value would defeat it.
    const seen = new Set();
    for (let i = 0; i < 200; i++) {
        seen.add(uuidHandle());
    }
    assert.strictEqual(seen.size, 200);
});

test('a UUID handle fits the handle column', () => {
    assert.ok(uuidHandle().length < 255, 'UUID handle must fit VARCHAR(255)');
});
