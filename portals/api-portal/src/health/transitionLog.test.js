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
const path = require('node:path');
const { createTransitionLogger } = require('./transitionLog');

function recordingLogger() {
    const lines = [];
    return {
        lines,
        error: (message, meta) => lines.push({ level: 'error', message, meta }),
        info: (message, meta) => lines.push({ level: 'info', message, meta }),
    };
}

const up = { status: 'up' };
const down = (error, sinceMs) => ({ status: 'down', error, sinceMs });

test('logs one line when a check goes down and one when it recovers', () => {
    const logger = recordingLogger();
    const transitions = createTransitionLogger({ logger });

    transitions.observe({ database: up }, 0);
    transitions.observe({ database: down('database unreachable', 1000) }, 1000);
    transitions.observe({ database: down('database unreachable', 1000) }, 4000);
    transitions.observe({ database: down('timed out after 2s', 1000) }, 7000);
    transitions.observe({ database: up }, 66400);
    transitions.observe({ database: up }, 70000);

    assert.deepStrictEqual(logger.lines, [
        { level: 'error', message: 'health check failed', meta: { check: 'database', reason: 'database unreachable' } },
        { level: 'info', message: 'health check recovered', meta: { check: 'database', down_for: '1m5s' } },
    ]);
});

test('a check that starts down is logged once, and the outage is timed from its since', () => {
    const logger = recordingLogger();
    const transitions = createTransitionLogger({ logger });

    transitions.observe({ database: down('database unreachable') }, 5000);
    transitions.observe({ database: up }, 8000);

    assert.equal(logger.lines.length, 2);
    assert.deepStrictEqual(logger.lines[1].meta, { check: 'database', down_for: '3s' });
});

test('checks are tracked independently', () => {
    const logger = recordingLogger();
    const transitions = createTransitionLogger({ logger });

    transitions.observe({ b: down('b failed', 0), a: down('a failed', 0) }, 0);
    transitions.observe({ a: up, b: down('b failed', 0) }, 2000);

    assert.deepStrictEqual(logger.lines.map((l) => [l.level, l.meta.check]), [
        ['error', 'a'], ['error', 'b'], ['info', 'a'],
    ]);
});

test('defaults to the portal logger', () => {
    const LOGGER_PATH = path.resolve(__dirname, '../config/logger.js');
    const logger = recordingLogger();
    require.cache[LOGGER_PATH] = { id: LOGGER_PATH, filename: LOGGER_PATH, loaded: true, exports: logger };
    try {
        createTransitionLogger().observe({ database: down('database unreachable', 0) }, 0);
    } finally {
        delete require.cache[LOGGER_PATH];
    }
    assert.equal(logger.lines.length, 1);
});
