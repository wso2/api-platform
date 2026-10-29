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
const express = require('express');
const { registerHealthRoutes } = require('./routes');
const { parseAllowedClients } = require('./allowedClients');
const { HealthChecker } = require('./checker');
const { createTransitionLogger } = require('./transitionLog');

const BASE_PATH = '/api-portal';
const quietLogger = { error() {}, info() {} };

/**
 * Starts an app with the health routes registered ahead of a stand-in for the session
 * middleware, and records whether that stand-in or the checks ever ran.
 */
async function start({ allowed = [], reason = null, trustProxy = false } = {}) {
    const app = express();
    if (trustProxy) app.set('trust proxy', true);
    const seen = { sessionTouched: 0, checkRuns: 0 };
    const checker = new HealthChecker({
        cacheTtlMs: 0,
        transitionLogger: createTransitionLogger({ logger: quietLogger }),
    });
    checker.register('database', async () => {
        seen.checkRuns += 1;
        return reason;
    });
    registerHealthRoutes(app, {
        checker,
        allowedClients: parseAllowedClients('api_portal.health', allowed),
        basePath: BASE_PATH,
    });
    app.use((req, res, next) => {
        seen.sessionTouched += 1;
        next();
    });
    app.use((err, req, res, _next) => {
        res.status(500).end();
    });
    const server = await new Promise((resolve) => {
        const s = app.listen(0, '127.0.0.1', () => resolve(s));
    });
    const base = `http://127.0.0.1:${server.address().port}`;
    return { base, seen, close: () => new Promise((resolve) => server.close(resolve)) };
}

const PATHS = ['/health', '/ready', `${BASE_PATH}/health`, `${BASE_PATH}/ready`];

test('every path answers 200 with the common body when healthy', async (t) => {
    const app = await start();
    t.after(app.close);
    for (const path of PATHS) {
        const res = await fetch(app.base + path);
        assert.equal(res.status, 200, path);
        assert.equal(res.headers.get('cache-control'), 'no-store');
        assert.match(res.headers.get('content-type'), /^application\/json/);
        const body = await res.json();
        assert.equal(body.status, 'ok');
        assert.match(body.timestamp, /^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/);
        assert.deepStrictEqual(body.checks, { database: { status: 'up' } });
    }
    assert.equal(app.seen.sessionTouched, 0);
});

test('a failing check answers 503 with its reason', async (t) => {
    const app = await start({ reason: 'database unreachable' });
    t.after(app.close);
    for (const path of PATHS) {
        const res = await fetch(app.base + path);
        assert.equal(res.status, 503, path);
        const body = await res.json();
        assert.equal(body.status, 'unhealthy');
        assert.equal(body.checks.database.status, 'down');
        assert.equal(body.checks.database.error, 'database unreachable');
        assert.ok(body.checks.database.since);
    }
});

test('HEAD gets the status and headers without a body', async (t) => {
    const app = await start();
    t.after(app.close);
    const res = await fetch(`${app.base}/ready`, { method: 'HEAD' });
    assert.equal(res.status, 200);
    assert.equal(res.headers.get('cache-control'), 'no-store');
    assert.equal(await res.text(), '');
});

test('other methods get 405 with Allow: GET, HEAD and run no check', async (t) => {
    const app = await start();
    t.after(app.close);
    for (const method of ['POST', 'PUT', 'DELETE', 'PATCH', 'OPTIONS']) {
        const res = await fetch(`${app.base}/health`, { method });
        assert.equal(res.status, 405, method);
        assert.equal(res.headers.get('allow'), 'GET, HEAD');
    }
    assert.equal(app.seen.checkRuns, 0);
    assert.equal(app.seen.sessionTouched, 0);
});

test('a peer outside the allowed set gets 403 and runs no check', async (t) => {
    const app = await start({ allowed: ['10.0.0.0/8'] });
    t.after(app.close);
    for (const path of PATHS) {
        const res = await fetch(app.base + path);
        assert.equal(res.status, 403, path);
        assert.equal(res.headers.get('cache-control'), 'no-store');
        assert.match(res.headers.get('content-type'), /^application\/json/);
        assert.deepStrictEqual(await res.json(), { status: 'forbidden' });
    }
    const post = await fetch(`${app.base}/ready`, { method: 'POST' });
    assert.equal(post.status, 403);
    assert.equal(app.seen.checkRuns, 0);
});

test('a forged X-Forwarded-For is ignored, even with trust proxy on', async (t) => {
    const app = await start({ allowed: ['10.1.2.3'], trustProxy: true });
    t.after(app.close);
    const res = await fetch(`${app.base}/ready`, { headers: { 'X-Forwarded-For': '10.1.2.3' } });
    assert.equal(res.status, 403);

    const loopbackOnly = await start({ trustProxy: true });
    t.after(loopbackOnly.close);
    const allowed = await fetch(`${loopbackOnly.base}/ready`, { headers: { 'X-Forwarded-For': '203.0.113.7' } });
    assert.equal(allowed.status, 200);
});

test('"*" allows every peer', async (t) => {
    const app = await start({ allowed: ['*'] });
    t.after(app.close);
    assert.equal((await fetch(`${app.base}/health`)).status, 200);
});

test('an evaluation that fails is handed to the error handler', async (t) => {
    const app = express();
    registerHealthRoutes(app, {
        checker: { evaluate: () => Promise.reject(new Error('boom')) },
        allowedClients: parseAllowedClients('api_portal.health', []),
        basePath: BASE_PATH,
    });
    app.use((err, req, res, _next) => {
        res.status(500).json({ error: 'internal' });
    });
    const server = await new Promise((resolve) => {
        const s = app.listen(0, '127.0.0.1', () => resolve(s));
    });
    t.after(() => new Promise((resolve) => server.close(resolve)));
    const res = await fetch(`http://127.0.0.1:${server.address().port}/health`);
    assert.equal(res.status, 500);
});
