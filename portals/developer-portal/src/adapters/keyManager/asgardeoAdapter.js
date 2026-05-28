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
const logger = require('../../config/logger');
const BaseKeyManagerAdapter = require('./baseAdapter');

/**
 * Adapter for WSO2 Asgardeo.
 * Uses the Asgardeo DCR (Dynamic Client Registration) endpoint for OAuth client management.
 *
 * Expected clientRegistrationEndpoint format:
 *   https://api.asgardeo.io/t/{tenant}/api/identity/oauth2/dcr/v1.1/register
 *
 * The admin client (adminClientId/adminClientSecret) must hold the DCR management
 * scopes in the Asgardeo console:
 *   internal_dcr_create, internal_dcr_view, internal_dcr_update, internal_dcr_delete
 */
class AsgardeoAdapter extends BaseKeyManagerAdapter {

    constructor(kmRecord, credentials) {
        super(kmRecord, credentials);
        // Cached admin token: { accessToken, expiresAt }
        this._adminTokenCache = null;
    }

    /**
     * Acquire an admin token with DCR management scopes.
     * Token is cached and reused until 10 seconds before expiry (matching the
     * Java AsgardeoDCRAuthInterceptor skew buffer).
     */
    async getAdminToken() {
        const SKEW_MS = 10_000;
        if (this._adminTokenCache && Date.now() < this._adminTokenCache.expiresAt - SKEW_MS) {
            return this._adminTokenCache.accessToken;
        }

        const params = new URLSearchParams();
        params.append('grant_type', 'client_credentials');
        params.append('scope',
            this.additionalProperties.adminScope ||
            'internal_dcr_create internal_dcr_view internal_dcr_update internal_dcr_delete'
        );

        try {
            const response = await axios.post(this.tokenEndpoint, params, {
                auth: {
                    username: this.adminClientId,
                    password: this.adminClientSecret,
                },
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                timeout: 5000,
            });
            const expiresIn = response.data.expires_in || 3600;
            this._adminTokenCache = {
                accessToken: response.data.access_token,
                expiresAt: Date.now() + expiresIn * 1000,
            };
            return this._adminTokenCache.accessToken;
        } catch (error) {
            logger.error('Failed to acquire Asgardeo admin token', {
                errorMessage: error.message,
                errorCode: error.code || null,
                status: error.response?.status || null,
                tokenEndpoint: this.tokenEndpoint,
            });
            throw new Error(`Failed to acquire Asgardeo admin token: ${error.message}`);
        }
    }

    async createOAuthClient(name, grantTypes, redirectUris, scopes, additionalProps) {
        const adminToken = await this.getAdminToken();

        const payload = {
            client_name: name,
            grant_types: grantTypes || ['client_credentials'],
            token_type_extension: 'JWT',
        };

        if (redirectUris && redirectUris.length) {
            payload.redirect_uris = redirectUris;
        }

        try {
            const response = await axios.post(this.clientRegEndpoint, payload, {
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${adminToken}`,
                },
                timeout: 5000,
            });

            const { client_secret, ...additionalProperties } = response.data;
            return {
                clientId: response.data.client_id,
                clientSecret: client_secret,
                additionalProperties,
            };
        } catch (error) {
            logger.error('Asgardeo createOAuthClient failed', {
                errorMessage: error.message,
                errorCode: error.code || null,
                status: error.response?.status || null,
            });
            throw new Error(`Failed to create OAuth client in Asgardeo: ${error.response?.data?.error_description || error.message}`);
        }
    }

    async updateOAuthClient(clientId, grantTypes, redirectUris, scopes, additionalProps) {
        const adminToken = await this.getAdminToken();

        const payload = {
            grant_types: grantTypes,
            token_type_extension: 'JWT',
        };

        if (redirectUris && redirectUris.length) {
            payload.redirect_uris = redirectUris;
        }

        // Merge editable additional properties (ext_* fields) into the payload
        if (additionalProps && typeof additionalProps === 'object') {
            for (const [key, value] of Object.entries(additionalProps)) {
                // Only include ext_* fields and skip fields already handled above
                if (key.startsWith('ext_') || key === 'token_type_extension') {
                    payload[key] = value;
                }
            }
        }

        try {
            const response = await axios.put(`${this.clientRegEndpoint}/${clientId}`, payload, {
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${adminToken}`,
                },
                timeout: 5000,
            });
            const { client_secret, ...additionalProperties } = response.data;
            return { additionalProperties };
        } catch (error) {
            logger.error('Asgardeo updateOAuthClient failed', {
                errorMessage: error.message,
                errorCode: error.code || null,
                status: error.response?.status || null,
            });
            throw new Error(`Failed to update OAuth client in Asgardeo: ${error.message}`);
        }
    }

    async deleteOAuthClient(clientId) {
        const adminToken = await this.getAdminToken();

        try {
            await axios.delete(`${this.clientRegEndpoint}/${clientId}`, {
                headers: {
                    'Authorization': `Bearer ${adminToken}`,
                },
                timeout: 5000,
            });
        } catch (error) {
            logger.error('Asgardeo deleteOAuthClient failed', {
                errorMessage: error.message,
                errorCode: error.code || null,
                status: error.response?.status || null,
            });
            throw new Error(`Failed to delete OAuth client in Asgardeo: ${error.message}`);
        }
    }
}

module.exports = AsgardeoAdapter;
