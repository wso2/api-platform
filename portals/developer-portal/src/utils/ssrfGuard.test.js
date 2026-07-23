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
 * Regression coverage for the SSRF guard. Uses node:test (Node's built-in
 * runner) rather than adding a test dependency — these are pure functions with
 * no server, DB, or fixture needs, so they do not belong in it/rest-api, whose
 * jest suite boots a live portal via globalSetup.
 *
 * Run with: npm test
 */

const test = require('node:test');
const assert = require('node:assert/strict');

const { isDenied, createGuardedLookup, assertAllowedScheme, assertAllowedHost } = require('./ssrfGuard');

test('assertAllowedHost: IP literals are checked (dns.lookup never runs for them)', async (t) => {
    // Node skips DNS resolution when the host is already a literal, so the Agent
    // lookup hook is never invoked and cannot be the only control. Without this
    // check "http://169.254.169.254/" would connect with the denylist unconsulted.
    await t.test('denied literals throw with a 422', () => {
        for (const ip of ['169.254.169.254', '127.0.0.1', '10.0.0.5', '0.0.0.0']) {
            assert.throws(() => assertAllowedHost(ip), (err) => err.statusCode === 422, `${ip} must be rejected`);
        }
    });

    await t.test('bracketed IPv6 literals are unwrapped before matching', () => {
        // URL.hostname keeps the brackets; a naive check would see "[::1]",
        // fail net.isIP, and skip the address entirely.
        assert.throws(() => assertAllowedHost('[::1]'));
        assert.throws(() => assertAllowedHost('[fd00:ec2::254]'));
        assert.throws(() => assertAllowedHost('[::ffff:169.254.169.254]'));
    });

    await t.test('allowPrivate permits private literals but never metadata', () => {
        assert.doesNotThrow(() => assertAllowedHost('127.0.0.1', { allowPrivate: true }));
        assert.doesNotThrow(() => assertAllowedHost('10.0.0.5', { allowPrivate: true }));
        assert.throws(() => assertAllowedHost('169.254.169.254', { allowPrivate: true }));
    });

    await t.test('public literals pass, and hostnames are left to the lookup hook', () => {
        assert.doesNotThrow(() => assertAllowedHost('8.8.8.8'));
        assert.doesNotThrow(() => assertAllowedHost('gateway.example.com'));
        assert.doesNotThrow(() => assertAllowedHost('localhost')); // not a literal — hook's job
        assert.doesNotThrow(() => assertAllowedHost(''));
    });
});

test('isDenied: IPv4-mapped IPv6 is normalized before matching', async (t) => {
    // The bug this guards: ipaddr reports kind() === 'ipv6' for ::ffff:a.b.c.d,
    // so without normalization these slip past every IPv4 CIDR in the denylist.
    await t.test('mapped loopback is denied', () => {
        assert.equal(isDenied('::ffff:127.0.0.1'), true);
        assert.equal(isDenied('::ffff:127.0.0.1', { allowPrivate: false }), true);
    });

    await t.test('mapped RFC 1918 is denied', () => {
        assert.equal(isDenied('::ffff:10.0.0.5'), true);
        assert.equal(isDenied('::ffff:192.168.1.1'), true);
        assert.equal(isDenied('::ffff:172.16.0.1'), true);
    });

    await t.test('mapped cloud metadata is denied', () => {
        assert.equal(isDenied('::ffff:169.254.169.254'), true);
    });

    await t.test('mapped public address is allowed', () => {
        assert.equal(isDenied('::ffff:8.8.8.8'), false);
    });
});

test('isDenied: allowPrivate governs private ranges only', async (t) => {
    const privateAddrs = ['10.0.0.5', '172.16.0.1', '192.168.1.1', '127.0.0.1', '::1', 'fc00::1', '100.64.1.1'];

    await t.test('private ranges denied when allowPrivate is false', () => {
        for (const ip of privateAddrs) {
            assert.equal(isDenied(ip, { allowPrivate: false }), true, `${ip} should be denied`);
        }
    });

    await t.test('private ranges permitted when allowPrivate is true', () => {
        for (const ip of privateAddrs) {
            assert.equal(isDenied(ip, { allowPrivate: true }), false, `${ip} should be permitted`);
        }
    });

    await t.test('allowPrivate never unlocks metadata or link-local', () => {
        for (const ip of ['169.254.169.254', '::ffff:169.254.169.254', 'fd00:ec2::254', 'fe80::1', '0.0.0.0', '::']) {
            assert.equal(isDenied(ip, { allowPrivate: true }), true, `${ip} must stay denied`);
        }
    });

    await t.test('defaults to denying private ranges', () => {
        assert.equal(isDenied('10.0.0.5'), true);
    });
});

test('isDenied: public addresses pass, unparseable input fails closed', () => {
    assert.equal(isDenied('8.8.8.8'), false);
    assert.equal(isDenied('2001:4860:4860::8888'), false);

    for (const bad of ['not-an-ip', '', '999.999.999.999', 'localhost']) {
        assert.equal(isDenied(bad), true, `${JSON.stringify(bad)} must fail closed`);
    }
});

test('createGuardedLookup: rejects a denied resolution', (t, done) => {
    const lookup = createGuardedLookup({ allowPrivate: false });
    // localhost resolves into 127.0.0.0/8 (or ::1), both denied.
    lookup('localhost', {}, (err, address) => {
        assert.ok(err, `expected rejection, got address ${address}`);
        assert.match(err.message, /not allowed/);
        done();
    });
});

test('createGuardedLookup: allows a permitted resolution', (t, done) => {
    // No DNS: the guard is fed a resolver result directly by stubbing the
    // hostname to a literal, which dns.lookup returns verbatim.
    const lookup = createGuardedLookup({ allowPrivate: false });
    lookup('8.8.8.8', {}, (err, address) => {
        assert.equal(err, null, err && err.message);
        assert.equal(address, '8.8.8.8');
        done();
    });
});

test('createGuardedLookup: allowPrivate permits a private resolution', (t, done) => {
    const lookup = createGuardedLookup({ allowPrivate: true });
    lookup('127.0.0.1', {}, (err, address) => {
        assert.equal(err, null, err && err.message);
        assert.equal(address, '127.0.0.1');
        done();
    });
});

test('createGuardedLookup: options.all checks every candidate', async (t) => {
    // Node's Happy Eyeballs (autoSelectFamily, default-on since v20) calls a
    // custom lookup with { all: true }, where the result is an array. The socket
    // may connect to ANY entry, so one denied candidate must fail the whole
    // resolution — this is the path a naive implementation forgets.
    const dns = require('node:dns');
    const realLookup = dns.lookup;

    const withStub = (entries, fn) => new Promise((resolve) => {
        dns.lookup = (hostname, options, callback) => callback(null, entries);
        const lookup = createGuardedLookup({ allowPrivate: false });
        lookup('stub.invalid', { all: true }, (err, address) => {
            dns.lookup = realLookup;
            fn(err, address);
            resolve();
        });
    });

    await t.test('all-public candidates are allowed through unchanged', () => withStub(
        [{ address: '8.8.8.8', family: 4 }, { address: '1.1.1.1', family: 4 }],
        (err, address) => {
            assert.equal(err, null, err && err.message);
            assert.deepEqual(address, [{ address: '8.8.8.8', family: 4 }, { address: '1.1.1.1', family: 4 }]);
        }
    ));

    await t.test('a single denied candidate rejects the whole resolution', () => withStub(
        [{ address: '8.8.8.8', family: 4 }, { address: '169.254.169.254', family: 4 }],
        (err) => {
            assert.ok(err, 'a denied candidate must reject');
            assert.match(err.message, /not allowed/);
        }
    ));

    await t.test('a denied mapped-IPv6 candidate is caught', () => withStub(
        [{ address: '::ffff:10.0.0.5', family: 6 }],
        (err) => {
            assert.ok(err, 'mapped private candidate must reject');
        }
    ));
});

test('assertAllowedScheme: only http(s), and http only when opted in', () => {
    assert.doesNotThrow(() => assertAllowedScheme('https://gw.example.com/x'));

    for (const raw of ['http://gw.example.com/x', 'file:///etc/passwd', 'gopher://x/', 'javascript:alert(1)', 'not a url']) {
        assert.throws(() => assertAllowedScheme(raw), (err) => err.statusCode === 422, `${raw} must be rejected`);
    }

    assert.doesNotThrow(() => assertAllowedScheme('http://gw.example.com/x', { allowHttp: true }));
    assert.throws(() => assertAllowedScheme('file:///etc/passwd', { allowHttp: true }));
});
