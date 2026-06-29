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
const logger = require('../config/logger');

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
        const response = await axios.post(tokenEndpoint, params, {
            auth: {
                username: clientId,
                password: clientSecret,
            },
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            timeout: 5000,
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
        throw new Error(`Token generation failed: ${error.message}`);
    }
}

module.exports = { generateToken };
