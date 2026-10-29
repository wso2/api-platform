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
const { createDatabaseCheck, isSaturated } = require('./databaseCheck');
const { OutcomeWindow } = require('./dbWindow');

const SETTINGS = {
    checkTimeoutMs: 2000,
    failureWindowMs: 30000,
    failureRatio: 0.5,
    minAttempts: 5,
    idlePeriodMs: 10000,
    poolSaturation: 0.9,
    poolWaitLimitMs: 5000,
};

const refused = () => Object.assign(new Error('connect ECONNREFUSED 10.0.0.5:5432'), { code: 'ECONNREFUSED' });
const timedOut = () => Object.assign(new Error('timeout exceeded when trying to connect'), {});

/**
 * A check over a real outcome window and a fake clock. `pingResult` decides what the
 * next own ping does: null succeeds, an Error fails; every ping is recorded in the
 * window like the driver's.
 */
function setup({ stats = null } = {}) {
    const clock = { now: 1_000_000 };
    const window = new OutcomeWindow({ windowMs: SETTINGS.failureWindowMs, now: () => clock.now });
    const state = { pings: 0, pingResult: null, stats };
    const ping = async () => {
        state.pings += 1;
        const result = state.pingResult;
        if (result instanceof Error) {
            window.recordError(result, { ping: true });
            throw result;
        }
        window.recordSuccess({ ping: true });
    };
    const check = createDatabaseCheck({
        window, ping, poolStats: () => state.stats, settings: SETTINGS, now: () => clock.now,
    });
    const traffic = (successes, failures = 0, err = refused()) => {
        for (let i = 0; i < successes; i++) window.recordSuccess();
        for (let i = 0; i < failures; i++) window.recordError(err);
    };
    return { clock, window, state, check, traffic };
}

test('before any traffic, the check pings once and the ping decides', async () => {
    const { check, state } = setup();
    assert.equal(await check(), null);
    assert.equal(state.pings, 1);
});

test('an idle instance pings at most once per idle period', async () => {
    const { check, state, clock } = setup();
    await check();
    clock.now += 3000;
    await check();
    clock.now += 3000;
    await check();
    assert.equal(state.pings, 1);

    clock.now += 4000;
    await check();
    assert.equal(state.pings, 2);
});

test('a failed ping reports the database unreachable, and a timed-out one names the timeout', async () => {
    const { check, state, clock } = setup();
    state.pingResult = refused();
    assert.equal(await check(), 'database unreachable');

    clock.now += 10000;
    state.pingResult = timedOut();
    assert.equal(await check(), 'timed out after 2s');

    clock.now += 10000;
    state.pingResult = Object.assign(new Error('x'), { timedOut: true });
    assert.equal(await check(), 'timed out after 2s');
});

test('a busy instance is judged by its traffic and never pings', async () => {
    const { check, state, traffic, clock } = setup();
    traffic(5);
    assert.equal(await check(), null);
    for (let i = 0; i < 20; i++) {
        clock.now += 1000;
        traffic(5);
        assert.equal(await check(), null);
    }
    assert.equal(state.pings, 0);
});

test('failures reaching the ratio take the check down; fewer do not', async () => {
    const { check, traffic, clock } = setup();
    traffic(3, 2);
    assert.equal(await check(), null);

    clock.now += 1000;
    traffic(0, 1);
    assert.equal(await check(), 'database unreachable');
});

test('a deciding timeout failure is reported as a timeout', async () => {
    const { check, traffic } = setup();
    traffic(0, 5, timedOut());
    assert.equal(await check(), 'timed out after 2s');
});

test('statement errors never take the check down', async () => {
    const { check, traffic } = setup();
    traffic(0, 10, Object.assign(new Error('duplicate key'), { code: '23505' }));
    assert.equal(await check(), null);
});

test('with too few attempts, the last decided state stands until the instance goes idle', async () => {
    const { check, traffic, clock, state } = setup();
    traffic(0, 5);
    assert.equal(await check(), 'database unreachable');

    // The window drains; a trickle of traffic keeps the instance active.
    clock.now += 31000;
    traffic(1);
    assert.equal(await check(), 'database unreachable');
    assert.equal(state.pings, 0);

    // Quiet for the idle period: one own ping decides again.
    clock.now += 10000;
    assert.equal(await check(), null);
    assert.equal(state.pings, 1);
});

test('the window recovers once successes outweigh failures', async () => {
    const { check, traffic, clock } = setup();
    traffic(0, 5);
    assert.equal(await check(), 'database unreachable');
    clock.now += 31000;
    traffic(5);
    assert.equal(await check(), null);
});

test('isSaturated applies the pg and mssql rules and skips unusable pools', () => {
    assert.equal(isSaturated({ max: 10, total: 9, idle: 0, waiting: 1 }, 0.9), true);
    assert.equal(isSaturated({ max: 10, total: 8, idle: 0, waiting: 1 }, 0.9), false);
    assert.equal(isSaturated({ max: 10, total: 10, idle: 1, waiting: 1 }, 0.9), false);
    assert.equal(isSaturated({ max: 10, total: 10, idle: 0, waiting: 0 }, 0.9), false);

    assert.equal(isSaturated({ max: 10, borrowed: 9, pending: 3 }, 0.9), true);
    assert.equal(isSaturated({ max: 10, borrowed: 9, pending: 0 }, 0.9), false);
    assert.equal(isSaturated({ max: 10, borrowed: 8, pending: 3 }, 0.9), false);

    assert.equal(isSaturated(null, 0.9), false);
    assert.equal(isSaturated({ max: 1, total: 1, idle: 0, waiting: 5 }, 0.9), false);
    assert.equal(isSaturated({ max: 0, total: 1, idle: 0, waiting: 5 }, 0.9), false);
    assert.equal(isSaturated({ max: undefined, borrowed: 5, pending: 5 }, 0.9), false);
    assert.equal(isSaturated({ max: 10 }, 0.9), false);
});

test('a saturated pool takes the check down only after the wait limit', async () => {
    const saturated = { max: 10, total: 10, idle: 0, waiting: 4 };
    const { check, traffic, clock, state } = setup({ stats: saturated });
    traffic(5);
    assert.equal(await check(), null);

    clock.now += 3000;
    traffic(5);
    assert.equal(await check(), null);

    clock.now += 2000;
    traffic(5);
    assert.equal(await check(), 'database connection pool exhausted');

    // Recovery: the pool drains and the check comes back up at once.
    state.stats = { max: 10, total: 4, idle: 4, waiting: 0 };
    clock.now += 1000;
    traffic(5);
    assert.equal(await check(), null);
});

test('a burst shorter than the wait limit does not trip the pool rule', async () => {
    const saturated = { max: 10, borrowed: 10, pending: 2 };
    const { check, traffic, clock, state } = setup({ stats: saturated });
    traffic(5);
    assert.equal(await check(), null);
    clock.now += 4000;
    state.stats = { max: 10, borrowed: 2, pending: 0 };
    traffic(5);
    assert.equal(await check(), null);

    // Saturation starting again restarts the clock.
    state.stats = saturated;
    clock.now += 1000;
    traffic(5);
    assert.equal(await check(), null);
    clock.now += 4000;
    traffic(5);
    assert.equal(await check(), null);
});

test('a down database is reported as unreachable, not as an exhausted pool', async () => {
    const { check, traffic, clock } = setup({ stats: { max: 10, total: 10, idle: 0, waiting: 4 } });
    traffic(0, 5);
    assert.equal(await check(), 'database unreachable');
    clock.now += 10000;
    traffic(0, 5);
    assert.equal(await check(), 'database unreachable');
});

test('SQLite has no pool, so the pool rule never applies', async () => {
    const { check, traffic, clock } = setup({ stats: null });
    for (let i = 0; i < 5; i++) {
        traffic(5);
        assert.equal(await check(), null);
        clock.now += 5000;
    }
});
