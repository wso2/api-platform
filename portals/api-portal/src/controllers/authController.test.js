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
 * The login callback: an authorization error the IDP sends back is a failed sign-in the
 * user can retry, not a server failure. passport is replaced by a test version that hands
 * the callback whatever outcome the test picks, as passport-oauth2 would.
 */

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const stub = (file, exports) => {
    require.cache[file] = { id: file, filename: file, loaded: true, exports };
};
const local = (relative) => path.resolve(__dirname, relative);

const warnings = [];
let outcome; // the (err, user) passport-oauth2 hands the callback
stub(require.resolve('passport'), {
    authenticate: (strategy, options, callback) => () => callback(...outcome),
});
stub(local('../config/configLoader.js'), { config: { auth: { mode: 'idp', idp: {} } } });
stub(local('../config/logger.js'), {
    warn: (message, meta) => warnings.push({ message, meta }),
    error: () => {}, info: () => {}, debug: () => {},
});
stub(local('../config/httpClientOptions.js'), { buildOutboundAgents: () => ({}) });
stub(local('../middlewares/auditLogger.js'), { logUserAction: () => {} });
stub(local('../utils/util.js'), { validateRequestParameters: () => [] });
stub(local('../dao/organizationDao.js'), {});
stub(local('../utils/orgContext.js'), {});
stub(local('../utils/platformJwt.js'), {});
stub(local('../middlewares/authorization.js'), {});
stub(local('../utils/sessionCookies.js'), {});

const { handleCallback } = require('./authController');

async function callback(query, result) {
    warnings.length = 0;
    outcome = result;
    const req = { query, params: { orgName: 'acme' }, session: { returnTo: '/acme/views/default/apis' } };
    const res = { redirectedTo: null, redirect(url) { this.redirectedTo = url; } };
    let forwarded;
    await handleCallback(req, res, (err) => { forwarded = err; });
    return { res, forwarded };
}

function authorizationError(code, description) {
    const err = new Error(description);
    err.name = 'AuthorizationError';
    err.code = code;
    return err;
}

test('access_denied from the IDP is a 403 sign-in failure, logged as a warning', async () => {
    const description = 'Sign up failed: the value is already in use';
    // passport-oauth2 reports access_denied as a failure with no error object.
    const { forwarded } = await callback({ error: 'access_denied', error_description: description }, [null, false]);
    assert.equal(forwarded.status, 403);
    assert.equal(forwarded.errorType, 'signin');
    assert.doesNotMatch(forwarded.message, /already in use/);
    assert.equal(warnings.length, 1);
    assert.deepEqual(warnings[0].meta, { error: 'access_denied', errorDescription: description });
});

test('other IDP authorization errors are a 400 sign-in failure', async () => {
    const { forwarded } = await callback({ error: 'server_error', error_description: 'IDP unavailable' },
        [authorizationError('server_error', 'IDP unavailable'), false]);
    assert.equal(forwarded.status, 400);
    assert.equal(forwarded.errorType, 'signin');
    assert.deepEqual(warnings[0].meta, { error: 'server_error', errorDescription: 'IDP unavailable' });
});

test('login_required still returns the user to where they were', async () => {
    const { res, forwarded } = await callback({ error: 'login_required' },
        [authorizationError('login_required', 'Login required'), false]);
    assert.equal(res.redirectedTo, '/acme/views/default/apis');
    assert.equal(forwarded, undefined);
    assert.equal(warnings.length, 0);
});

test('an unexpected failure during the callback is still a server error', async () => {
    const tokenFailure = new Error('Failed to obtain access token');
    tokenFailure.name = 'InternalOAuthError';
    const { forwarded } = await callback({ code: 'abc', state: 'xyz' }, [tokenFailure, false]);
    assert.equal(forwarded, tokenFailure);
    assert.equal(forwarded.status, undefined);
    assert.equal(warnings.length, 0);

    const { forwarded: noUser } = await callback({ code: 'abc', state: 'xyz' }, [null, false]);
    assert.equal(noUser.message, 'Authentication failed');
    assert.equal(noUser.status, undefined);
});

test('the sign-in error page offers sign in, and sign up only when the portal has it', () => {
    const Handlebars = require('handlebars');
    const { registerHelpers } = require('../helpers/handlebarsHelpers');
    registerHelpers();
    const page = Handlebars.compile(fs.readFileSync(local('../pages/error-page/page.hbs'), 'utf8'));
    const base = { baseUrl: '/acme/views/default', errorType: 'signin' };

    const html = page({ ...base, signUpEnabled: true });
    assert.match(html, /We couldn't sign you in/);
    assert.match(html, /href="\/acme\/views\/default\/login"/);
    assert.match(html, /href="\/acme\/views\/default\/signup"/);
    assert.doesNotMatch(html, /Error 500/);

    assert.doesNotMatch(page({ ...base, signUpEnabled: false }), /\/signup"/);
});
