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
const logger = require('../config/logger');
const { CustomError } = require('../utils/errors/customErrors');
const { buildClient } = require('../keymanagers/core/httpClient');
const { DB_CLIENT_POLICY } = require('./keyManagerDriverBuilder');

/*
 * The token endpoint comes from a key manager created through the API, so the
 * request goes through the same guarded client as every other call to one:
 * address ranges checked when the host resolves, network access control
 * applied, no redirects followed (the developer's client secret must not follow
 * a 3xx to another host), and a bounded response. Built once and reused.
 */
let tokenClient = null;
function client() {
    if (!tokenClient) tokenClient = buildClient(DB_CLIENT_POLICY);
    return tokenClient;
}

function isRefusedDestination(error) {
    return error?.code === 'DESTINATION_NOT_ALLOWED' || error?.cause?.code === 'DESTINATION_NOT_ALLOWED';
}

/**
 * Proxy a client_credentials token request to a key manager's token endpoint.
 * The OAuth client itself is created and owned externally, in the key
 * manager's own console — this only ever proxies a token request using the
 * client_id/client_secret the developer supplies.
 *
 * @param {string}   tokenEndpoint  - The key manager's OAuth2 token endpoint.
 * @param {string}   clientId       - The developer's client_id.
 * @param {string}   clientSecret   - The developer's client_secret.
 * @param {string[]} scopes         - Scopes to request.
 * @param {number}   validityPeriod - Requested token lifetime in seconds (AS may ignore).
 * @returns {Promise<{ accessToken: string, expiresIn: number, tokenType: string, scope: string }>}
 */
async function generateToken(tokenEndpoint, clientId, clientSecret, scopes, validityPeriod) {
    const params = new URLSearchParams();
    params.append('grant_type', 'client_credentials');
    if (scopes && scopes.length) {
        params.append('scope', scopes.join(' '));
    }
    if (validityPeriod) {
        params.append('expiry_time', String(validityPeriod));
    }

    try {
        const response = await client().post(tokenEndpoint, params, {
            auth: {
                username: clientId,
                password: clientSecret,
            },
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            timeout: 5000,
            // The shared client hands every status back; this caller treats a
            // non-2xx (including an unfollowed 3xx) as a failure.
            validateStatus: (status) => status >= 200 && status < 300,
        });
        return {
            accessToken: response.data.access_token,
            expiresIn: response.data.expires_in,
            tokenType: response.data.token_type || 'Bearer',
            scope: response.data.scope || '',
        };
    } catch (error) {
        logger.error('Failed to generate token', {
            errorMessage: error.message,
            errorCode: error.code || null,
            status: error.response?.status || null,
            tokenEndpoint,
        });
        // Propagate a client error from the key manager (e.g. 401 for a bad
        // consumer secret) as-is so the caller sees a 4xx, not a masked 500.
        // Upstream 5xx, timeouts, and connection failures aren't the caller's
        // fault, so they surface as a 500.
        // The reply names no host or client error text, which can reveal internal addresses; the log
        // above has the details.
        if (isRefusedDestination(error)) {
            throw new CustomError(422, 'Token generation failed: the key manager endpoint is not permitted.');
        }
        const upstreamStatus = error.response?.status;
        const clientError = upstreamStatus >= 400 && upstreamStatus < 500;
        throw new CustomError(clientError ? upstreamStatus : 500, clientError
            ? `Token generation failed: the key manager refused the request (${upstreamStatus}).`
            : 'Token generation failed: the key manager could not be reached.');
    }
}

module.exports = { generateToken };
