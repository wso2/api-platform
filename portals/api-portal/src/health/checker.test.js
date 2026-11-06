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
const { HealthChecker, formatTime } = require('./checker');
const { createTransitionLogger } = require('./transitionLog');

const T0 = Date.parse('2026-10-29T09:12:01.400Z');

function setup({ cacheTtlMs = 3000 } = {}) {
    const clock = { now: T0 };
    const lines = [];
    const logger = {
        error: (message, meta) => lines.push({ message, meta }),
        info: (message, meta) => lines.push({ message, meta }),
    };
    const checker = new HealthChecker({
        cacheTtlMs,
        now: () => clock.now,
        transitionLogger: createTransitionLogger({ logger }),
    });
    return { checker, clock, lines };
}

test('no checks is healthy with an empty checks object', async () => {
    const { checker } = setup();
    const result = await checker.evaluate();
    assert.deepStrictEqual(result, {
        healthy: true, status: 'ok', timestamp: '2026-10-29T09:12:01Z', checks: {},
    });
});

test('a check reporting a reason is down, and the result is unhealthy', async () => {
    const { checker } = setup();
    checker.register('database', async () => 'database unreachable');
    checker.register('other', async () => undefined);
    const result = await checker.evaluate();
    assert.equal(result.healthy, false);
    assert.equal(result.status, 'unhealthy');
    assert.deepStrictEqual(result.checks, {
        database: { status: 'down', error: 'database unreachable', since: '2026-10-29T09:12:01Z' },
        other: { status: 'up' },
    });
});

test('since is kept while a check stays down and cleared when it recovers', async () => {
    const { checker, clock, lines } = setup({ cacheTtlMs: 0 });
    let reason = 'database unreachable';
    checker.register('database', async () => reason);

    await checker.evaluate();
    clock.now += 10000;
    const later = await checker.evaluate();
    assert.equal(later.checks.database.since, '2026-10-29T09:12:01Z');
    assert.equal(later.timestamp, '2026-10-29T09:12:11Z');

    reason = null;
    clock.now += 5000;
    assert.deepStrictEqual((await checker.evaluate()).checks.database, { status: 'up' });

    reason = 'database unreachable';
    clock.now += 1000;
    assert.equal((await checker.evaluate()).checks.database.since, '2026-10-29T09:12:17Z');

    assert.deepStrictEqual(lines.map((l) => l.message), [
        'health check failed', 'health check recovered', 'health check failed',
    ]);
    assert.deepStrictEqual(lines[1].meta, { check: 'database', down_for: '15s' });
});

test('a thrown error reports a generic reason, never its message', async () => {
    const { checker } = setup();
    checker.register('database', async () => {
        throw new Error('password authentication failed for user "admin" at db.internal:5432');
    });
    const result = await checker.evaluate();
    assert.equal(result.checks.database.error, 'check failed');
});

test('a slow check times out with a reason naming the timeout', async () => {
    const { checker } = setup();
    checker.register('database', () => new Promise(() => {}), { timeoutMs: 20 });
    const result = await checker.evaluate();
    assert.equal(result.checks.database.error, 'timed out after 20ms');
});

test('a check finishing within its timeout is not affected by it', async () => {
    const { checker } = setup();
    checker.register('database', async () => null, { timeoutMs: 1000 });
    assert.equal((await checker.evaluate()).healthy, true);
});

test('a result is reused within the cache lifetime and refreshed after it', async () => {
    const { checker, clock } = setup({ cacheTtlMs: 3000 });
    let runs = 0;
    checker.register('database', async () => { runs += 1; });

    const first = await checker.evaluate();
    clock.now += 2999;
    assert.strictEqual(await checker.evaluate(), first);
    assert.equal(runs, 1);

    clock.now += 1;
    await checker.evaluate();
    assert.equal(runs, 2);
});

test('concurrent callers share one evaluation', async () => {
    const { checker } = setup({ cacheTtlMs: 0 });
    let runs = 0;
    let release;
    checker.register('database', () => {
        runs += 1;
        return new Promise((resolve) => { release = resolve; });
    });

    const pending = [checker.evaluate(), checker.evaluate(), checker.evaluate()];
    await new Promise((resolve) => setImmediate(resolve));
    release();
    const results = await Promise.all(pending);
    assert.equal(runs, 1);
    assert.ok(results.every((r) => r === results[0]));

    const next = checker.evaluate();
    await new Promise((resolve) => setImmediate(resolve));
    release();
    await next;
    assert.equal(runs, 2);
});

test('formatTime renders UTC with second precision', () => {
    assert.equal(formatTime(Date.parse('2026-01-02T03:04:05.999Z')), '2026-01-02T03:04:05Z');
});
