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
 * Secrets in request URLs, queries and bodies never reach a log: the redaction helpers, and the audit
 * log line they feed. The audit logger's own logger and database are replaced, so the test reads the
 * line it writes.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const stub = (relative, exports) => {
    const file = path.resolve(__dirname, relative);
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
};
const audited = [];
stub('../config/logger.js', { audit: (message, entry) => audited.push({ message, entry }) });
stub('../dao/auditDao.js', { record: async () => {} });
stub('./util.js', { resolveActor: () => 'admin-1' });

const { REDACTED, redactObject, redactQuery, redactUrl, sessionRef } = require('./logRedaction');
const { auditMiddleware } = require('../middlewares/auditLogger');

test('an OAuth callback URL keeps its path, but not its authorization code or state', () => {
    assert.equal(redactUrl('/api-portal/default/callback?code=AUTH123&state=XYZ'),
        `/api-portal/default/callback?code=${REDACTED}&state=${REDACTED}`);
});

test('a URL keeps its other query fields, its fragment and its form; one without a query is unchanged', () => {
    assert.equal(redactUrl('/try?api_key=k-1&page=2&flag#top'), `/try?api_key=${REDACTED}&page=2&flag#top`);
    assert.equal(redactUrl('/try?client%5Fsecret=s&q=a+b'), `/try?client%5Fsecret=${REDACTED}&q=a+b`);
    assert.equal(redactUrl('/apis/weather'), '/apis/weather');
    assert.equal(redactUrl(undefined), undefined);
});

test('a parsed query and a body lose their sensitive values at any depth; arrays stay arrays', () => {
    assert.deepEqual(redactQuery({ code: 'c', page: '1', apiKey: 'k', filter: { accessToken: 't', name: 'n' } }),
        { code: REDACTED, page: '1', apiKey: REDACTED, filter: { accessToken: REDACTED, name: 'n' } });
    assert.deepEqual(redactObject({ grantTypes: ['client_credentials'], stripeSecretKey: 'sk', name: 'App' }),
        { grantTypes: ['client_credentials'], stripeSecretKey: REDACTED, name: 'App' });
    // In a body, "code" is an ordinary field (a key manager's code, a country code).
    assert.deepEqual(redactObject({ code: 'LK' }), { code: 'LK' });
});

test('a session is logged by a short stable reference, never its id', () => {
    const ref = sessionRef('s%3Aabc.def');
    assert.match(ref, /^[0-9a-f]{16}$/);
    assert.equal(sessionRef('s%3Aabc.def'), ref);
    assert.ok(!ref.includes('abc'));
    assert.equal(sessionRef(undefined), 'no-session');
});

test('the audit line of a request names no code, key, secret or session id', () => {
    audited.length = 0;
    const req = {
        method: 'GET', path: '/api-portal/default/callback',
        originalUrl: '/api-portal/default/callback?code=AUTH123&state=XYZ&view=default',
        query: { code: 'AUTH123', state: 'XYZ', view: 'default' },
        body: { password: 'p', name: 'n' }, headers: {}, socket: {}, sessionID: 'raw-session-id',
    };
    const res = { statusCode: 302, end() {} };
    auditMiddleware()(req, res, () => {});
    res.end();
    assert.equal(audited.length, 1);
    const line = JSON.stringify(audited[0]);
    for (const secret of ['AUTH123', 'XYZ', 'raw-session-id', '"p"']) {
        assert.ok(!line.includes(secret), `${secret} is in ${line}`);
    }
    assert.match(audited[0].message, /callback\?code=\[REDACTED\]&state=\[REDACTED\]&view=default/);
    assert.deepEqual(audited[0].entry.queryParams, { code: REDACTED, state: REDACTED, view: 'default' });
});
