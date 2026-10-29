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
const { OutcomeWindow } = require('./dbWindow');

function setup(windowMs = 30000) {
    const clock = { now: 1_000_000 };
    return { clock, win: new OutcomeWindow({ windowMs, now: () => clock.now }) };
}

test('an empty window has no attempts and no activity', () => {
    const { win } = setup();
    assert.deepStrictEqual(win.snapshot(), {
        attempts: 0, failures: 0, lastActivity: null, lastPing: null, lastFailureKind: null,
    });
});

test('successes and failures are counted, and activity is remembered', () => {
    const { win, clock } = setup();
    win.recordSuccess();
    clock.now += 500;
    win.record({ failure: 'unreachable' });
    const snap = win.snapshot();
    assert.equal(snap.attempts, 2);
    assert.equal(snap.failures, 1);
    assert.equal(snap.lastActivity, clock.now);
    assert.equal(snap.lastFailureKind, 'unreachable');
});

test('recordError counts only connection failures, but every error is an attempt', () => {
    const { win } = setup();
    win.recordError(Object.assign(new Error('dup'), { code: '23505' }));
    win.recordError(Object.assign(new Error('refused'), { code: 'ECONNREFUSED' }));
    win.recordError(Object.assign(new Error('slow'), { code: 'ETIMEDOUT' }));
    const snap = win.snapshot();
    assert.equal(snap.attempts, 3);
    assert.equal(snap.failures, 2);
    assert.equal(snap.lastFailureKind, 'timeout');
});

test('a ping is an attempt but not application activity', () => {
    const { win, clock } = setup();
    win.recordSuccess({ ping: true });
    win.recordError(Object.assign(new Error('refused'), { code: 'ECONNREFUSED' }), { ping: true });
    const snap = win.snapshot();
    assert.equal(snap.attempts, 2);
    assert.equal(snap.failures, 1);
    assert.equal(snap.lastPing, clock.now);
    assert.equal(snap.lastActivity, null);
});

test('an outcome no caller asked for is counted without marking activity', () => {
    const { win } = setup();
    win.record({ failure: 'unreachable', activity: false });
    const snap = win.snapshot();
    assert.equal(snap.failures, 1);
    assert.equal(snap.lastActivity, null);
    assert.equal(snap.lastPing, null);
});

test('outcomes older than the window expire', () => {
    const { win, clock } = setup(3000);
    win.record({ failure: 'unreachable' });
    clock.now += 1000;
    win.recordSuccess();
    clock.now += 1000;
    win.recordSuccess();
    assert.equal(win.snapshot().attempts, 3);

    clock.now += 1000;
    assert.deepStrictEqual([win.snapshot().attempts, win.snapshot().failures], [2, 0]);

    clock.now += 5000;
    assert.equal(win.snapshot().attempts, 0);
});

test('a reused bucket starts from zero', () => {
    const { win, clock } = setup(2000);
    win.recordSuccess();
    win.recordSuccess();
    clock.now += 2000;
    win.recordSuccess();
    assert.equal(win.snapshot().attempts, 1);
});

test('a window shorter than a bucket still keeps one bucket', () => {
    const { win } = setup(200);
    win.recordSuccess();
    assert.equal(win.snapshot().attempts, 1);
});
