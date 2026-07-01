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
const apiDao = require('../dao/apiDao');
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

async function resolveApiIdOrRespond(orgId, apiHandle, res) {
    const apiId = await apiDao.getId(orgId, apiHandle);
    if (!apiId) {
        res.status(404).json({ code: '404', message: 'Not Found', description: 'API not found' });
        return null;
    }
    return apiId;
}

// Returns undefined when no handle was given, null when the handle didn't resolve, or the uuid.
async function resolveAppId(orgId, userId, appHandle) {
    if (!appHandle) return undefined;
    const app = await applicationDao.getId(orgId, userId, appHandle);
    return app ? app.uuid : null;
}

function mapKey(k) {
    const app = k.dp_api_key_app_mapping?.dp_application;
    return {
        keyId: k.uuid,
        name: k.name,
        status: k.status,
        expiresAt: k.expires_at,
        createdAt: k.created_at,
        revokedAt: k.revoked_at || undefined,
        apiId: k.dp_api_metadata?.handle || k.api_uuid,
        appId: app ? app.handle : null,
        appDisplayName: app ? app.display_name : null
    };
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/generate
 * Body: { name, expiresAt?, subscriptionId?, appId? }
 */
async function generateApiKey(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { name, expiresAt, subscriptionId, appId: appHandle } = req.body || {};

    const appIdResult = normalizeOptionalId(appHandle);
    if (!appIdResult.ok) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'appId must be a non-empty string' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res);
        if (!apiId) return;
        const appId = await resolveAppId(orgId, req.user.sub, appIdResult.value);
        if (appIdResult.value && !appId) {
            return res.status(404).json({ code: '404', message: 'Not Found', description: 'Application not found' });
        }
        const result = await apiKeyService.generate({
            orgId, apiId, subscriptionId, appId, name, expiresAt,
            actor: req.user.sub, userToken: req.user.accessToken,
        });
        logUserAction('API_KEY_GENERATED', req, { orgId, apiId: apiHandle, keyId: result.keyId });
        return res.status(201).json(result);
    } catch (err) {
        logger.error('Failed to generate API key', { error: err.message, orgId, apiId: apiHandle });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * GET /devportal/v1/apis/:apiId/api-keys
 * Query: subscriptionId (optional), status (optional), appId (optional)
 */
async function listApiKeys(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { subscriptionId, status, appId: appHandle } = req.query;

    const appIdResult = normalizeOptionalId(appHandle);
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
        const apiId = await apiDao.getId(orgId, apiHandle);
        if (!apiId) {
            return res.status(404).json({
                status: 'error', code: '404', message: 'Not Found', errors: [{ field: 'apiId', message: 'API not found' }],
            });
        }
        const appId = await resolveAppId(orgId, req.user.sub, appIdResult.value);
        if (appIdResult.value && !appId) {
            return res.status(404).json({
                status: 'error', code: '404', message: 'Not Found', errors: [{ field: 'appId', message: 'Application not found' }],
            });
        }
        const keys = await apiKeyService.list(orgId, {
            apiId,
            subscriptionId: subscriptionId || undefined,
            appId,
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
 * Body: { keyId, expiresAt? }
 */
async function regenerateApiKey(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { keyId, expiresAt } = req.body || {};

    if (!keyId || typeof keyId !== 'string' || !keyId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res);
        if (!apiId) return;
        const result = await apiKeyService.regenerate({
            orgId, apiId, keyId: keyId.trim(), expiresAt, actor: req.user.sub, userToken: req.user.accessToken,
        });
        logUserAction('API_KEY_REGENERATED', req, { orgId, apiId: apiHandle, keyId });
        return res.status(200).json(result);
    } catch (err) {
        logger.error('Failed to regenerate API key', { error: err.message, orgId, apiId: apiHandle, keyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/revoke
 * Body: { keyId }
 */
async function revokeApiKey(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { keyId } = req.body || {};

    if (!keyId || typeof keyId !== 'string' || !keyId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res);
        if (!apiId) return;
        await apiKeyService.revoke({ orgId, apiId, keyId: keyId.trim(), actor: req.user.sub, userToken: req.user.accessToken });
        logUserAction('API_KEY_REVOKED', req, { orgId, apiId: apiHandle, keyId });
        return res.status(204).send();
    } catch (err) {
        logger.error('Failed to revoke API key', { error: err.message, orgId, apiId: apiHandle, keyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/associate
 * Body: { keyId, appId }
 */
async function associateApiKeyApplication(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { keyId, appId: appHandle } = req.body || {};

    if (!keyId || typeof keyId !== 'string' || !keyId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }
    if (!appHandle || typeof appHandle !== 'string' || !appHandle.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'appId is required' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res);
        if (!apiId) return;
        const appId = await resolveAppId(orgId, req.user.sub, appHandle.trim());
        if (!appId) {
            return res.status(404).json({ code: '404', message: 'Not Found', description: 'Application not found' });
        }
        const result = await apiKeyService.associateApplication({
            orgId, apiId, keyId: keyId.trim(), appId, actor: req.user.sub,
        });
        logUserAction('API_KEY_APP_ASSOCIATED', req, { orgId, apiId: apiHandle, keyId, appId: appHandle });
        return res.status(200).json(result);
    } catch (err) {
        logger.error('Failed to associate application with API key', { error: err.message, orgId, apiId: apiHandle, keyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * POST /devportal/v1/apis/:apiId/api-keys/dissociate
 * Body: { keyId }
 */
async function removeApiKeyApplication(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { keyId } = req.body || {};

    if (!keyId || typeof keyId !== 'string' || !keyId.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res);
        if (!apiId) return;
        await apiKeyService.removeApplicationAssociation({ orgId, apiId, keyId: keyId.trim(), actor: req.user.sub });
        logUserAction('API_KEY_APP_DISASSOCIATED', req, { orgId, apiId: apiHandle, keyId });
        return res.status(204).send();
    } catch (err) {
        logger.error('Failed to remove application association from API key', { error: err.message, orgId, apiId: apiHandle, keyId });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: err.message });
    }
}

/**
 * GET /devportal/v1/applications/:applicationId/api-keys
 * Lists all API keys (across every API) currently associated with an app.
 */
async function listApplicationApiKeys(req, res) {
    const orgId = req.orgId;
    const { applicationId: applicationHandle } = req.params;

    try {
        const appRecord = await applicationDao.getId(orgId, req.user.sub, applicationHandle);
        if (!appRecord) {
            return res.status(404).json({ code: '404', message: 'Application not found' });
        }
        const applicationId = appRecord.uuid;
        const keys = await apiKeyService.list(orgId, { appId: applicationId });
        return res.status(200).json(util.toPaginatedList(keys.map(mapKey), req));
    } catch (err) {
        logger.error('Failed to list application API keys', { error: err.message, orgId, applicationId: applicationHandle });
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
