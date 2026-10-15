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
 * The issuer is read from a token before its signature is checked, so the only thing
 * it may contribute to where keys are fetched from is one plain organization id. These
 * pin down that forged or odd issuers never produce a JWKS URL outside configuration.
 */

const test = require('node:test');
const assert = require('node:assert');
const path = require('node:path');

const CONFIG_PATH = path.resolve(__dirname, '../config/configLoader.js');
const ISSUER_PATH = path.resolve(__dirname, './orgIssuer.js');

function load(idp) {
    require.cache[CONFIG_PATH] = { id: CONFIG_PATH, filename: CONFIG_PATH, loaded: true, exports: { config: { auth: { idp } } } };
    delete require.cache[ISSUER_PATH];
    try {
        return require(ISSUER_PATH);
    } finally {
        delete require.cache[CONFIG_PATH];
        delete require.cache[ISSUER_PATH];
    }
}

const IS = {
    jwksUrl: 'https://is.example.com/oauth2/jwks',
    issuerTemplate: 'https://is.example.com/o/{org}/oauth2/token',
    jwksUrlTemplate: 'https://is.example.com/o/{org}/oauth2/jwks',
};

test('a per-organization issuer yields its organization and that organization\'s JWKS URL', () => {
    const { matchOrgIssuer } = load(IS);
    assert.deepStrictEqual(matchOrgIssuer('https://is.example.com/o/9d1a-4c1e/oauth2/token'), {
        org: '9d1a-4c1e',
        issuer: 'https://is.example.com/o/9d1a-4c1e/oauth2/token',
        jwksUrl: 'https://is.example.com/o/9d1a-4c1e/oauth2/jwks',
    });
});

test('without a template the configured jwks_url is used', () => {
    const { matchOrgIssuer } = load({ ...IS, jwksUrlTemplate: '' });
    assert.strictEqual(matchOrgIssuer('https://is.example.com/o/acme/oauth2/token').jwksUrl, IS.jwksUrl);
});

test('an issuer that does not fit the whole template is not per-organization', () => {
    const { matchOrgIssuer } = load(IS);
    for (const iss of [
        'https://is.example.com/oauth2/token',                     // the root issuer
        'https://evil.example.com/o/acme/oauth2/token',            // another host
        'https://is.example.com/o/acme/oauth2/token/extra',        // trailing path
        'xhttps://is.example.com/o/acme/oauth2/token',             // prefix
        'https://is.example.com/o/a/b/oauth2/token',               // two segments
        'https://isXexample.com/o/acme/oauth2/token',              // '.' is literal, not a wildcard
    ]) {
        assert.strictEqual(matchOrgIssuer(iss), null, iss);
    }
});

test('an {org} segment that is not a plain identifier is refused', () => {
    const { matchOrgIssuer } = load(IS);
    for (const org of ['..', '%2e%2e', 'acme%2Fx', 'acme?x=1', 'acme#x', '-acme', 'a'.repeat(129)]) {
        assert.strictEqual(matchOrgIssuer(`https://is.example.com/o/${org}/oauth2/token`), null, org);
    }
});

test('nothing matches when no template is configured', () => {
    const { matchOrgIssuer } = load({ jwksUrl: IS.jwksUrl });
    assert.strictEqual(matchOrgIssuer('https://is.example.com/o/acme/oauth2/token'), null);
    assert.strictEqual(matchOrgIssuer(undefined), null);
});
