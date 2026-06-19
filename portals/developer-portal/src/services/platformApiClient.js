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
const https = require('https');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');

function isPlatformApiPath(gatewayType) {
    return gatewayType === 'wso2/api-platform' && !!(config.platformApi?.baseUrl);
}

function createAxiosClient(userToken) {
    const baseURL = config.platformApi?.baseUrl;
    return axios.create({
        baseURL,
        headers: {
            Authorization: `Bearer ${userToken}`,
            'Content-Type': 'application/json',
        },
        httpsAgent: new https.Agent({ rejectUnauthorized: !config.platformApi?.insecure }),
        timeout: 15000,
    });
}

function mapError(err) {
    const status = err.response?.status;
    const description = err.response?.data?.description || err.message;
    const mapped = new Error(description);
    mapped.status = status || 502;
    return mapped;
}

async function createSubscription(userToken, { apiId, subscriberId, subscriptionPlanId }) {
    const client = createAxiosClient(userToken);
    const body = { apiId, subscriberId, status: 'ACTIVE' };
    if (subscriptionPlanId) body.subscriptionPlanId = subscriptionPlanId;
    try {
        const resp = await client.post('/api/v1/subscriptions', body);
        return resp.data;
    } catch (err) {
        logger.error('[platformApiClient] createSubscription failed', { apiId, error: err.message });
        throw mapError(err);
    }
}

async function findSubscription(userToken, { apiId, subscriberId }) {
    const client = createAxiosClient(userToken);
    try {
        const resp = await client.get('/api/v1/subscriptions', {
            params: { apiId, subscriberId, limit: 1 },
        });
        const items = resp.data?.subscriptions || [];
        return items.length > 0 ? items[0] : null;
    } catch (err) {
        logger.error('[platformApiClient] findSubscription failed', { apiId, error: err.message });
        throw mapError(err);
    }
}

async function deleteSubscription(userToken, { platformSubId, subscriberId }) {
    const client = createAxiosClient(userToken);
    try {
        await client.delete(`/api/v1/subscriptions/${platformSubId}`, {
            params: { subscriberId },
        });
    } catch (err) {
        logger.error('[platformApiClient] deleteSubscription failed', { platformSubId, error: err.message });
        throw mapError(err);
    }
}

async function updateSubscription(userToken, { platformSubId, subscriberId, status }) {
    const client = createAxiosClient(userToken);
    try {
        await client.put(`/api/v1/subscriptions/${platformSubId}`, { status }, {
            params: { subscriberId },
        });
    } catch (err) {
        logger.error('[platformApiClient] updateSubscription failed', { platformSubId, error: err.message });
        throw mapError(err);
    }
}

async function createApiKey(userToken, { apiRefId, apiKey, name }) {
    const client = createAxiosClient(userToken);
    try {
        await client.post(`/api/v1/rest-apis/${apiRefId}/api-keys`, {
            apiKey,
            name,
            displayName: name,
        });
    } catch (err) {
        logger.error('[platformApiClient] createApiKey failed', { apiRefId, name, error: err.message });
        throw mapError(err);
    }
}

async function updateApiKey(userToken, { apiRefId, keyName, apiKey }) {
    const client = createAxiosClient(userToken);
    try {
        await client.put(`/api/v1/rest-apis/${apiRefId}/api-keys/${keyName}`, { apiKey });
    } catch (err) {
        logger.error('[platformApiClient] updateApiKey failed', { apiRefId, keyName, error: err.message });
        throw mapError(err);
    }
}

async function revokeApiKey(userToken, { apiRefId, keyName }) {
    const client = createAxiosClient(userToken);
    try {
        await client.delete(`/api/v1/rest-apis/${apiRefId}/api-keys/${keyName}`);
    } catch (err) {
        logger.error('[platformApiClient] revokeApiKey failed', { apiRefId, keyName, error: err.message });
        throw mapError(err);
    }
}

module.exports = {
    isPlatformApiPath,
    createSubscription,
    findSubscription,
    deleteSubscription,
    updateSubscription,
    createApiKey,
    updateApiKey,
    revokeApiKey,
};
