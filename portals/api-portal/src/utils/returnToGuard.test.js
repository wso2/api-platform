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
 * Regression coverage for the post-login redirect sanitiser. Pure function, no
 * server/DB/config, so it uses node:test like utils/ssrfGuard.test.js rather than
 * the live-portal jest suite in it/rest-api.
 *
 * Run with: npm test
 */

const test = require('node:test');
const assert = require('node:assert/strict');

const { sanitizeReturnTo } = require('./returnToGuard');

const FALLBACK = '/api-portal/acme';

test('sanitizeReturnTo: keeps same-origin paths intact', () => {
    for (const ok of [
        '/api-portal/acme/views/default/page',
        '/api-portal/acme/views/default/page?tab=1&x=2',
        '/',
    ]) {
        assert.equal(sanitizeReturnTo(ok, FALLBACK), ok, `expected ${ok} to survive`);
    }
});

test('sanitizeReturnTo: rejects a destination that leaves this origin', async (t) => {
    // The load-bearing case. An absolute-form request target routes on its parsed
    // pathname, so the route matches and req.params fill in while req.originalUrl
    // keeps the attacker's host — this is what would otherwise reach res.redirect().
    await t.test('absolute-form request target', () => {
        assert.equal(
            sanitizeReturnTo('http://evil.com/api-portal/acme/views/default/page', FALLBACK),
            FALLBACK,
        );
    });

    await t.test('absolute and protocol-relative and authority-ish spellings', () => {
        for (const bad of [
            'https://evil.com/x',
            '//evil.com/x',
            '/\\evil.com/x',
            '/\\\\evil.com/x',
            'javascript:alert(1)',
        ]) {
            assert.equal(sanitizeReturnTo(bad, FALLBACK), FALLBACK, `expected ${bad} to be rejected`);
        }
    });

    // Guards the output, not just the input: these all START as accepted-looking
    // absolute paths and only become protocol-relative after normalisation.
    await t.test('dot segments that normalise into a protocol-relative target', () => {
        for (const bad of ['/..//evil.com', '/../..//evil.com/x', '/a/../..//evil.com']) {
            assert.equal(sanitizeReturnTo(bad, FALLBACK), FALLBACK, `expected ${bad} to be rejected`);
        }
    });
});

test('sanitizeReturnTo: falls back on absent or non-string input', () => {
    for (const bad of ['', undefined, null, 42, {}, [], 'relative/path']) {
        assert.equal(sanitizeReturnTo(bad, FALLBACK), FALLBACK);
    }
});

test('sanitizeReturnTo: never returns a value res.redirect could send off-origin', () => {
    // Belt-and-braces sweep: whatever comes back must be an origin-relative path.
    const inputs = [
        '/ok', '//evil.com', 'http://evil.com/x', '/..//evil.com', '/\\evil.com',
        '/%2f%2fevil.com/x', '', undefined, 'javascript:alert(1)',
    ];
    for (const input of inputs) {
        const out = sanitizeReturnTo(input, FALLBACK);
        assert.equal(typeof out, 'string');
        assert.ok(out.startsWith('/'), `${JSON.stringify(input)} -> ${out} is not origin-relative`);
        assert.ok(!out.startsWith('//'), `${JSON.stringify(input)} -> ${out} is protocol-relative`);
        assert.ok(!/^[a-z][a-z0-9+.-]*:/i.test(out), `${JSON.stringify(input)} -> ${out} carries a scheme`);
    }
});
