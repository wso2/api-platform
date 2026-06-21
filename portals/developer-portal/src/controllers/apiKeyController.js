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
const apiKeyService = require('../services/apiKeyService');
const logger = require('../config/logger');

function errorStatus(err) {
    return err.status || 500;
}

/**
 * POST /organizations/:orgId/api-keys/generate
 * Body: { apiId, name, expiresAt?, subscriptionId? }
 */
async function generateApiKey(req, res) {
    const { orgId } = req.params;
    const { apiId, name, expiresAt, subscriptionId } = req.body || {};

    if (!apiId || typeof apiId !== 'string' || !apiId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'apiId is required' });
    }

    try {
        const result = await apiKeyService.generate({
            orgId, apiId: apiId.trim(), subscriptionId, name, expiresAt,
            actor: req.user.sub, userToken: req.user.accessToken,
        });
        return res.status(201).json(result);
    } catch (err) {
        logger.error('[apiKeyController] generate failed', { error: err.message, orgId, apiId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * GET /organizations/:orgId/api-keys
 * Query: apiId (required), subscriptionId (optional), status (optional)
 */
async function listApiKeys(req, res) {
    const { orgId } = req.params;
    const { apiId, subscriptionId, status } = req.query;

    if (!apiId || typeof apiId !== 'string' || !apiId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'apiId is required' });
    }

    try {
        const keys = await apiKeyService.list(orgId, {
            apiId: apiId.trim(),
            subscriptionId: subscriptionId || undefined,
            status: status || undefined
        });
        return res.status(200).json(keys.map(k => ({
            keyId: k.KEY_ID,
            name: k.NAME,
            status: k.STATUS,
            expiresAt: k.EXPIRES_AT,
            createdAt: k.CREATED_AT,
            revokedAt: k.REVOKED_AT || undefined,
            apiId: k.API_ID
        })));
    } catch (err) {
        logger.error('[apiKeyController] list failed', { error: err.message, orgId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * POST /organizations/:orgId/api-keys/:apiKeyId/regenerate
 */
async function regenerateApiKey(req, res) {
    const { orgId, apiKeyId } = req.params;

    try {
        const result = await apiKeyService.regenerate({
            orgId, keyId: apiKeyId, actor: req.user.sub, userToken: req.user.accessToken,
        });
        return res.status(200).json(result);
    } catch (err) {
        logger.error('[apiKeyController] regenerate failed', { error: err.message, orgId, apiKeyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * POST /organizations/:orgId/api-keys/:apiKeyId/revoke
 */
async function revokeApiKey(req, res) {
    const { orgId, apiKeyId } = req.params;

    try {
        await apiKeyService.revoke({ orgId, keyId: apiKeyId, actor: req.user.sub, userToken: req.user.accessToken });
        return res.status(204).send();
    } catch (err) {
        logger.error('[apiKeyController] revoke failed', { error: err.message, orgId, apiKeyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

module.exports = { generateApiKey, listApiKeys, regenerateApiKey, revokeApiKey };
