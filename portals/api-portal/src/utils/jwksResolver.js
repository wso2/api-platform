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

const { createRemoteJWKSet } = require('jose');

// One JWKS resolver per URL, kept at module scope and shared by every token check
// (browser sign-in in passportConfig.js and REST API bearer tokens in
// authMiddleware.js)
const JWKS_RESOLVER_MAX_ENTRIES = 1000;
const jwksResolvers = new Map();

function getJwksResolver(jwksURL) {
    let resolver = jwksResolvers.get(jwksURL);
    if (!resolver) {
        if (jwksResolvers.size >= JWKS_RESOLVER_MAX_ENTRIES) jwksResolvers.clear();
        resolver = createRemoteJWKSet(new URL(jwksURL));
        jwksResolvers.set(jwksURL, resolver);
    }
    return resolver;
}

module.exports = { getJwksResolver };
