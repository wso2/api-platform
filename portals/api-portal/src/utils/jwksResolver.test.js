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
const { getJwksResolver } = require('./jwksResolver');

// Threat model 01-9: REST bearer-token checks must reuse one cached JWKS resolver per
// URL instead of building a new one (and re-fetching the keys) on every request.
test('returns the same resolver for the same JWKS URL', () => {
    const a = getJwksResolver('https://idp.example.com/oauth2/jwks');
    const b = getJwksResolver('https://idp.example.com/oauth2/jwks');
    assert.strictEqual(a, b);
});

test('returns a different resolver for a different JWKS URL', () => {
    const a = getJwksResolver('https://idp.example.com/oauth2/jwks');
    const b = getJwksResolver('https://other-idp.example.com/oauth2/jwks');
    assert.notStrictEqual(a, b);
});
