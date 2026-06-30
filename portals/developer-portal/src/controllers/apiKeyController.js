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
const { logUserAction } = require('../middlewares/auditLogger');
const util = require('../utils/util');
const constants = require('../utils/constants');

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
    const app = k.DP_API_KEY_APP_MAPPING?.DP_APPLICATION;
    return {
        keyId: k.UUID,
        name: k.NAME,
        status: k.STATUS,
        expiresAt: k.EXPIRES_AT,
        createdAt: k.CREATED_AT,
        revokedAt: k.REVOKED_AT || undefined,
        apiId: k.API_UUID,
        appId: app ? app.UUID : null,
        appName: app ? app.NAME : null
    };
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/generate
 * Body: { name, expiresAt?, subscriptionId?, appId? }
 */
async function generateApiKey(req, res) {
    const orgId = req.orgId;
    const apiId = req.params.apiId;
    const { name, expiresAt, subscriptionId, appId } = req.body || {};

    const appIdResult = normalizeOptionalId(appId);
    if (!appIdResult.ok) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'appId must be a non-empty string' });
    }

    try {
        const result = await apiKeyService.generate({
            orgId, apiId: apiId.trim(), subscriptionId, appId: appIdResult.value, name, expiresAt,
            actor: req.user.sub, userToken: req.user.accessToken,
        });
        logUserAction('API_KEY_GENERATED', req, { orgId, apiId, keyId: result.keyId });
        return res.status(201).json(result);
    } catch (err) {
        logger.error('Failed to generate API key', { error: err.message, orgId, apiId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * GET /devportal/v1/apis/:apiId/api-keys
 * Query: subscriptionId (optional), status (optional), appId (optional)
 */
async function listApiKeys(req, res) {
    const orgId = req.orgId;
    const apiId = req.params.apiId;
    const { subscriptionId, status, appId } = req.query;

    const appIdResult = normalizeOptionalId(appId);
    if (!appIdResult.ok) {
        return res.status(400).json({
            status: 'error', code: 'COMMON_VALIDATION_ERROR', message: 'Bad Request',
            errors: [{ field: 'appId', message: 'appId must be a non-empty string' }],
        });
    }
    if (status && !Object.values(constants.API_KEY_STATUS).includes(status)) {
        return res.status(400).json({
            status: 'error', code: 'COMMON_VALIDATION_ERROR', message: 'Bad Request',
            errors: [{ field: 'status', message: `status must be one of: ${Object.values(constants.API_KEY_STATUS).join(', ')}` }],
        });
    }

    try {
        const keys = await apiKeyService.list(orgId, {
            apiId: apiId.trim(),
            subscriptionId: subscriptionId || undefined,
            appId: appIdResult.value,
            status: status || undefined
        });
        const mapped = keys.map(k => mapKey(k));
        return res.status(200).json(util.toPaginatedList(mapped, req));
    } catch (err) {
        logger.error('Failed to list API keys', { error: err.message, orgId });
        return res.status(errorStatus(err)).json({
            status: 'error',
            code: 'INTERNAL_SERVER_ERROR',
            message: err.message,
            errors: [],
        });
    }
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/regenerate
 * Body: { keyId }
 */
async function regenerateApiKey(req, res) {
    const orgId = req.orgId;
    const apiId = req.params.apiId;
    const { keyId } = req.body || {};

    if (!keyId || typeof keyId !== 'string' || !keyId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        const result = await apiKeyService.regenerate({
            orgId, apiId, keyId: keyId.trim(), actor: req.user.sub, userToken: req.user.accessToken,
        });
        logUserAction('API_KEY_REGENERATED', req, { orgId, apiId, keyId });
        return res.status(200).json(result);
    } catch (err) {
        logger.error('Failed to regenerate API key', { error: err.message, orgId, apiId, keyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/revoke
 * Body: { keyId }
 */
async function revokeApiKey(req, res) {
    const orgId = req.orgId;
    const apiId = req.params.apiId;
    const { keyId } = req.body || {};

    if (!keyId || typeof keyId !== 'string' || !keyId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        await apiKeyService.revoke({ orgId, apiId, keyId: keyId.trim(), actor: req.user.sub, userToken: req.user.accessToken });
        logUserAction('API_KEY_REVOKED', req, { orgId, apiId, keyId });
        return res.status(204).send();
    } catch (err) {
        logger.error('Failed to revoke API key', { error: err.message, orgId, apiId, keyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/associate
 * Body: { keyId, appId }
 */
async function associateApiKeyApplication(req, res) {
    const orgId = req.orgId;
    const apiId = req.params.apiId;
    const { keyId, appId } = req.body || {};

    if (!keyId || typeof keyId !== 'string' || !keyId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }
    if (!appId || typeof appId !== 'string' || !appId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'appId is required' });
    }

    try {
        const result = await apiKeyService.associateApplication({
            orgId, apiId, keyId: keyId.trim(), appId: appId.trim(), actor: req.user.sub,
        });
        logUserAction('API_KEY_APP_ASSOCIATED', req, { orgId, apiId, keyId, appId });
        return res.status(200).json(result);
    } catch (err) {
        logger.error('Failed to associate application with API key', { error: err.message, orgId, apiId, keyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/dissociate
 * Body: { keyId }
 */
async function removeApiKeyApplication(req, res) {
    const orgId = req.orgId;
    const apiId = req.params.apiId;
    const { keyId } = req.body || {};

    if (!keyId || typeof keyId !== 'string' || !keyId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        await apiKeyService.removeApplicationAssociation({ orgId, apiId, keyId: keyId.trim(), actor: req.user.sub });
        logUserAction('API_KEY_APP_DISASSOCIATED', req, { orgId, apiId, keyId });
        return res.status(204).send();
    } catch (err) {
        logger.error('Failed to remove application association from API key', { error: err.message, orgId, apiId, keyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * GET /devportal/v1/applications/:applicationId/api-keys
 * Lists all API keys (across every API) currently associated with an app.
 */
async function listApplicationApiKeys(req, res) {
    const orgId = req.orgId;
    const { applicationId } = req.params;

    try {
        const app = await applicationDao.get(orgId, applicationId, req.user.sub);
        if (!app) {
            return res.status(404).json({ code: '404', message: 'Application not found' });
        }
        const keys = await apiKeyService.list(orgId, { appId: applicationId });
        return res.status(200).json(util.toPaginatedList(keys.map(mapKey), req));
    } catch (err) {
        logger.error('Failed to list application API keys', { error: err.message, orgId, applicationId });
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
