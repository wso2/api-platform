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
const logger = require('../../config/logger');

/**
 * Base class for all key manager adapters.
 * Concrete adapters must implement the abstract methods.
 */
class BaseKeyManagerAdapter {
    /**
     * @param {object} kmRecord   - The DP_KEY_MANAGER row (raw Sequelize instance).
     * @param {object} credentials - { adminClientId, adminClientSecret } decrypted.
     */
    constructor(kmRecord, credentials) {
        this.kmRecord = kmRecord;
        this.adminClientId = credentials.adminClientId;
        this.adminClientSecret = credentials.adminClientSecret;
        this.tokenEndpoint = kmRecord.TOKEN_ENDPOINT;
        this.clientRegEndpoint = kmRecord.CLIENT_REG_ENDPOINT;
        this.additionalProperties = kmRecord.ADDITIONAL_PROPERTIES || {};
    }

    /**
     * Acquire an admin access token from the AS using the stored admin credentials.
     * Override if the AS uses a non-standard token flow.
     *
     * @returns {Promise<string>} Bearer access token for admin API calls.
     */
    async getAdminToken() {
        const axios = require('axios');
        const params = new URLSearchParams();
        params.append('grant_type', 'client_credentials');
        params.append('scope', this.additionalProperties.adminScope || 'openid');

        try {
            const response = await axios.post(this.tokenEndpoint, params, {
                auth: {
                    username: this.adminClientId,
                    password: this.adminClientSecret,
                },
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            });
            return response.data.access_token;
        } catch (error) {
            logger.error('Failed to acquire admin token', { error, tokenEndpoint: this.tokenEndpoint });
            throw new Error(`Failed to acquire admin token: ${error.message}`);
        }
    }

    /**
     * Create a new OAuth2 client/application in the Authorization Server.
     *
     * @param {string}   name           - Application name.
     * @param {string[]} grantTypes     - Grant types (e.g. ['client_credentials']).
     * @param {string[]} redirectUris   - Redirect URIs (may be empty for client_credentials).
     * @param {string[]} scopes         - Requested scopes.
     * @param {object}   additionalProps - AS-specific extra properties.
     * @returns {Promise<{ clientId: string, clientSecret: string }>}
     */
    async createOAuthClient(_name, _grantTypes, _redirectUris, _scopes, _additionalProps) {
        throw new Error('createOAuthClient() must be implemented by the adapter');
    }

    /**
     * Update an existing OAuth2 client in the Authorization Server.
     *
     * @param {string}   clientId       - The client_id to update.
     * @param {string[]} grantTypes     - Updated grant types.
     * @param {string[]} redirectUris   - Updated redirect URIs.
     * @param {string[]} scopes         - Updated scopes.
     * @param {object}   additionalProps - AS-specific extra properties.
     * @returns {Promise<void>}
     */
    async updateOAuthClient(_clientId, _grantTypes, _redirectUris, _scopes, _additionalProps) {
        throw new Error('updateOAuthClient() must be implemented by the adapter');
    }

    /**
     * Delete / deregister an OAuth2 client from the Authorization Server.
     *
     * @param {string} clientId - The client_id to delete.
     * @returns {Promise<void>}
     */
    async deleteOAuthClient(_clientId) {
        throw new Error('deleteOAuthClient() must be implemented by the adapter');
    }

    /**
     * Generate an access token using the given client credentials.
     * This proxies a token request to the AS token endpoint on behalf of the developer.
     *
     * @param {string}   clientId       - The developer's client_id.
     * @param {string}   clientSecret   - The developer's client_secret.
     * @param {string[]} scopes         - Scopes to request.
     * @param {number}   validityPeriod - Requested token lifetime in seconds (AS may ignore).
     * @returns {Promise<{ accessToken: string, expiresIn: number, tokenType: string, scope: string }>}
     */
    async generateToken(clientId, clientSecret, scopes, validityPeriod) {
        const axios = require('axios');
        const params = new URLSearchParams();
        params.append('grant_type', 'client_credentials');
        if (scopes && scopes.length) {
            params.append('scope', scopes.join(' '));
        }
        if (validityPeriod) {
            params.append('expiry_time', String(validityPeriod));
        }

        try {
            const response = await axios.post(this.tokenEndpoint, params, {
                auth: {
                    username: clientId,
                    password: clientSecret,
                },
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            });
            return {
                accessToken: response.data.access_token,
                expiresIn: response.data.expires_in,
                tokenType: response.data.token_type || 'Bearer',
                scope: response.data.scope || '',
            };
        } catch (error) {
            logger.error('Failed to generate token', { error, tokenEndpoint: this.tokenEndpoint });
            throw new Error(`Token generation failed: ${error.message}`);
        }
    }
}

module.exports = BaseKeyManagerAdapter;
