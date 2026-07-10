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
const axios = require('axios');
const qs = require('qs');
const { jwtVerify, importX509 } = require('jose');
const { config } = require('../config/configLoader');
const constants = require('./constants');
const logger = require('../config/logger');

const DEFAULT_TOKEN_REFRESH_TIMEOUT_MS = 10000;

function accessTokenPresent(req) {
    if (req.user && req.user[constants.ACCESS_TOKEN]) {
        return req.user[constants.ACCESS_TOKEN];
    }
    const auth = req.headers.authorization;
    if (auth && auth.toLowerCase().startsWith('bearer ')) {
        return auth.split(' ')[1];
    }
    return null;
}

async function refreshAccessToken(refreshToken) {
    const timeout = Number(config.idp?.tokenRefreshTimeoutMs);
    const timeoutMs = (Number.isFinite(timeout) && timeout > 0) ? timeout : DEFAULT_TOKEN_REFRESH_TIMEOUT_MS;
    const params = {
        grant_type: 'refresh_token',
        refresh_token: refreshToken,
        client_id: config.idp.clientId,
    };
    if (config.idp.clientSecret) {
        params.client_secret = config.idp.clientSecret;
    }
    const data = qs.stringify(params);
    const response = await axios.post(config.idp.tokenUrl, data, {
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        timeout: timeoutMs,
    });
    return response.data;
}

async function verifyWithCertificate(token, pemCertificate) {
    try {
        const publicKey = await importX509(pemCertificate, 'RS256');
        const jwtVerifyOptions = { algorithms: constants.JWT_ASYMMETRIC_ALGORITHMS };
        if (config.idp?.issuer) jwtVerifyOptions.issuer = config.idp.issuer;
        if (config.idp?.audience) jwtVerifyOptions.audience = config.idp.audience;
        const { payload } = await jwtVerify(token, publicKey, jwtVerifyOptions);
        return { valid: true, scopes: payload.scope || '' };
    } catch (err) {
        logger.error('Bearer token cert validation failed', { error: err.message, operation: 'verifyWithCertificate' });
        return { valid: false, scopes: '' };
    }
}

function resolveOrgIdp() {
    return config.idp || {};
}

module.exports = { accessTokenPresent, refreshAccessToken, verifyWithCertificate, resolveOrgIdp };
