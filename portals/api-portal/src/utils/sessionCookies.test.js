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
const session = require('express-session');
const sessionCookies = require('./sessionCookies');
const constants = require('./constants');

const BASE_PATH = constants.ROUTE.BASE_PATH;

/**
 * Starts an app wired the way app.js wires the session and XSRF cookies, over plain
 * HTTP, so a TLS-terminating proxy in front is simulated with X-Forwarded-Proto.
 */
async function start({ designMode = false } = {}) {
    const app = express();
    app.set('trust proxy', sessionCookies.TRUSTED_PROXY_HOPS);
    app.use(BASE_PATH, session({
        secret: 'test-secret',
        name: sessionCookies.SESSION_COOKIE_NAME,
        resave: false,
        saveUninitialized: true,
        cookie: sessionCookies.sessionCookieOptions({ designMode }),
    }));
    app.use(BASE_PATH, (req, res, next) => {
        res.cookie(sessionCookies.CSRF_COOKIE_NAME, 'token',
            sessionCookies.csrfCookieOptions(req, { designMode }));
        next();
    });
    app.get(`${BASE_PATH}/`, (req, res) => res.send('ok'));
    const server = await new Promise((resolve) => {
        const s = app.listen(0, '127.0.0.1', () => resolve(s));
    });
    const base = `http://127.0.0.1:${server.address().port}`;
    return { base, close: () => new Promise((resolve) => server.close(resolve)) };
}

/** Returns the Set-Cookie line for `name`, or undefined when none was sent. */
async function setCookie(base, name, headers = {}) {
    const res = await fetch(`${base}${BASE_PATH}/`, { headers });
    return res.headers.getSetCookie().find((c) => c.startsWith(`${name}=`));
}

const COOKIES = [sessionCookies.SESSION_COOKIE_NAME, sessionCookies.CSRF_COOKIE_NAME];

test('both cookies are Secure and SameSite=Lax behind a TLS-terminating proxy', async (t) => {
    const app = await start();
    t.after(app.close);
    for (const name of COOKIES) {
        const cookie = await setCookie(app.base, name, { 'X-Forwarded-Proto': 'https' });
        assert.ok(cookie, `${name} was not set`);
        assert.match(cookie, /;\s*Secure/i, name);
        assert.match(cookie, /;\s*SameSite=Lax/i, name);
        assert.match(cookie, new RegExp(`;\\s*Path=${BASE_PATH}(;|$)`), name);
    }
});

test('both cookies are still set, without Secure, over plain HTTP', async (t) => {
    const app = await start();
    t.after(app.close);
    for (const name of COOKIES) {
        const cookie = await setCookie(app.base, name);
        assert.ok(cookie, `${name} was not set`);
        assert.doesNotMatch(cookie, /;\s*Secure/i, name);
        assert.match(cookie, /;\s*SameSite=Lax/i, name);
    }
});

test('the session cookie stays HttpOnly and the XSRF cookie stays script-readable', async (t) => {
    const app = await start();
    t.after(app.close);
    const headers = { 'X-Forwarded-Proto': 'https' };
    assert.match(await setCookie(app.base, sessionCookies.SESSION_COOKIE_NAME, headers), /;\s*HttpOnly/i);
    assert.doesNotMatch(await setCookie(app.base, sessionCookies.CSRF_COOKIE_NAME, headers), /;\s*HttpOnly/i);
});

test('design mode never marks the cookies Secure', async (t) => {
    const app = await start({ designMode: true });
    t.after(app.close);
    for (const name of COOKIES) {
        const cookie = await setCookie(app.base, name, { 'X-Forwarded-Proto': 'https' });
        assert.ok(cookie, `${name} was not set`);
        assert.doesNotMatch(cookie, /;\s*Secure/i, name);
    }
});
