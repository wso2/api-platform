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
const applicationDao = require('../dao/applicationDao');
const logger = require('../config/logger');
const util = require('../utils/util');

function errorStatus(err) {
    return err.status || 500;
}

/**
 * Normalizes an optional id-like field the same way apiId is handled: absent/empty is
 * fine (no filter/association), but if present it must be a non-empty string.
 */
function normalizeOptionalId(value) {
    if (value === undefined || value === null || value === '') return { ok: true, value: undefined };
    if (typeof value !== 'string' || !value.trim()) return { ok: false };
    return { ok: true, value: value.trim() };
}

function mapKey(k) {
    const app = k.DP_APPLICATION;
    return {
        keyId: k.KEY_ID,
        name: k.NAME,
        status: k.STATUS,
        expiresAt: k.EXPIRES_AT,
        createdAt: k.CREATED_AT,
        revokedAt: k.REVOKED_AT || undefined,
        apiId: k.API_ID,
        appId: app ? app.APP_ID : undefined,
        appName: app ? app.NAME : undefined
    };
}

/**
 * POST /organizations/:orgId/api-keys/generate
 * Body: { apiId, name, expiresAt?, subscriptionId?, appId? }
 */
async function generateApiKey(req, res) {
    const { orgId } = req.params;
    const { apiId, name, expiresAt, subscriptionId, appId } = req.body || {};

    if (!apiId || typeof apiId !== 'string' || !apiId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'apiId is required' });
    }
    const appIdResult = normalizeOptionalId(appId);
    if (!appIdResult.ok) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'appId must be a non-empty string' });
    }

    try {
        const result = await apiKeyService.generate({
            orgId, apiId: apiId.trim(), subscriptionId, appId: appIdResult.value, name, expiresAt,
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
 * Query: apiId (required), subscriptionId (optional), status (optional), appId (optional)
 */
async function listApiKeys(req, res) {
    const { orgId } = req.params;
    const { apiId, subscriptionId, status, appId } = req.query;

    if (!apiId || typeof apiId !== 'string' || !apiId.trim()) {
        return res.status(400).json({
            status: 'error', code: 'COMMON_VALIDATION_ERROR', message: 'Bad Request',
            errors: [{ field: 'apiId', message: 'apiId is required' }],
        });
    }
    const appIdResult = normalizeOptionalId(appId);
    if (!appIdResult.ok) {
        return res.status(400).json({
            status: 'error', code: 'COMMON_VALIDATION_ERROR', message: 'Bad Request',
            errors: [{ field: 'appId', message: 'appId must be a non-empty string' }],
        });
    }

    try {
        const keys = await apiKeyService.list(orgId, {
            apiId: apiId.trim(),
            subscriptionId: subscriptionId || undefined,
            appId: appIdResult.value,
            status: status || undefined
        });
        return res.status(200).json(util.toPaginatedList(keys.map(mapKey), req));
    } catch (err) {
        logger.error('[apiKeyController] list failed', { error: err.message, orgId });
        return res.status(errorStatus(err)).json({
            status: 'error',
            code: 'INTERNAL_SERVER_ERROR',
            message: err.message,
            errors: [],
        });
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

/**
 * PUT /organizations/:orgId/api-keys/:apiKeyId/application
 * Body: { appId }
 */
async function associateApiKeyApplication(req, res) {
    const { orgId, apiKeyId } = req.params;
    const { appId } = req.body || {};

    if (!appId || typeof appId !== 'string' || !appId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'appId is required' });
    }

    try {
        const result = await apiKeyService.associateApplication({
            orgId, keyId: apiKeyId, appId: appId.trim(), actor: req.user.sub,
        });
        return res.status(200).json(result);
    } catch (err) {
        logger.error('[apiKeyController] associate application failed', { error: err.message, orgId, apiKeyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * DELETE /organizations/:orgId/api-keys/:apiKeyId/application
 */
async function removeApiKeyApplication(req, res) {
    const { orgId, apiKeyId } = req.params;

    try {
        await apiKeyService.removeApplicationAssociation({ orgId, keyId: apiKeyId, actor: req.user.sub });
        return res.status(204).send();
    } catch (err) {
        logger.error('[apiKeyController] remove application association failed', { error: err.message, orgId, apiKeyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * GET /organizations/:orgId/applications/:applicationId/api-keys
 * Lists all API keys (across every API) currently associated with an app.
 */
async function listApplicationApiKeys(req, res) {
    const { orgId, applicationId } = req.params;

    try {
        const app = await applicationDao.get(orgId, applicationId, req.user.sub);
        if (!app) {
            return res.status(404).json({ code: '404', message: 'Application not found' });
        }
        const keys = await apiKeyService.list(orgId, { appId: applicationId });
        return res.status(200).json(util.toPaginatedList(keys.map(mapKey), req));
    } catch (err) {
        logger.error('[apiKeyController] list application api keys failed', { error: err.message, orgId, applicationId });
        return res.status(errorStatus(err)).json({
            status: 'error',
            code: 'INTERNAL_SERVER_ERROR',
            message: err.message,
            errors: [],
        });
    }
}

module.exports = {
    generateApiKey, listApiKeys, regenerateApiKey, revokeApiKey,
    associateApiKeyApplication, removeApiKeyApplication, listApplicationApiKeys
};
