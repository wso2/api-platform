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
const apiKeyDao = require('../dao/apiKeyDao');
const applicationDao = require('../dao/applicationDao');
const apiDao = require('../dao/apiDao');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
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

/**
 * Resolves a handle to its uuid, scoped by whether the caller is operating on the
 * `/apis` family (excludes MCP-typed records) or the `/mcp-servers` family (only
 * MCP-typed records). `req.__forceApiType` is set by mcpServerKeysHandler when it
 * delegates into these shared handlers so they can be reused for both resource
 * families without duplicating their logic.
 */
async function resolveApiId(orgId, apiHandle, req) {
    return req?.__forceApiType === constants.API_TYPE.MCP
        ? apiDao.getIdByType(orgId, apiHandle, constants.API_TYPE.MCP)
        : apiDao.getIdExcludingType(orgId, apiHandle, constants.API_TYPE.MCP);
}

async function resolveApiIdOrRespond(orgId, apiHandle, res, req) {
    const apiId = await resolveApiId(orgId, apiHandle, req);
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

// Resolves an API key's handle (as sent in `keyId` in request bodies) to its uuid, scoped to apiId.
async function resolveKeyId(orgId, apiId, keyHandle) {
    return apiKeyDao.getIdByHandle(orgId, apiId, keyHandle);
}

async function resolveKeyIdOrRespond(orgId, apiId, keyHandle, res) {
    const keyId = await resolveKeyId(orgId, apiId, keyHandle);
    if (!keyId) {
        res.status(404).json({ code: '404', message: 'Not Found', description: 'API key not found' });
        return null;
    }
    return keyId;
}

function mapKey(k, audit) {
    const app = k.dp_api_key_app_mapping?.dp_application;
    return {
        keyId: k.uuid,
        id: k.handle,
        displayName: k.display_name,
        status: k.status,
        expiresAt: k.expires_at,
        createdAt: k.created_at,
        revokedAt: k.revoked_at || undefined,
        apiId: k.dp_api_metadata?.handle || k.api_uuid,
        appId: app ? app.handle : null,
        appDisplayName: app ? app.display_name : null,
        ...audit,
    };
}

async function mapKeysWithAudit(keys) {
    const auditList = await userIdpReferenceDao.buildListAuditFields(keys);
    return keys.map((k, i) => mapKey(k, auditList[i]));
}

/**
 * POST /api/v0.9/apis/:apiId/api-keys/generate
 * Body: { id, displayName?, expiresAt?, subscriptionId?, appId? }
 */
async function generateApiKey(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { id, displayName, expiresAt, subscriptionId, appId: appHandle } = req.body || {};

    const appIdResult = normalizeOptionalId(appHandle);
    if (!appIdResult.ok) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'appId must be a non-empty string' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res, req);
        if (!apiId) return;
        const appId = await resolveAppId(orgId, util.resolveActor(req), appIdResult.value);
        if (appIdResult.value && !appId) {
            return res.status(404).json({ code: '404', message: 'Not Found', description: 'Application not found' });
        }
        const result = await apiKeyService.generate({
            orgId, apiId, subscriptionId, appId, handle: id, displayName, expiresAt,
            actor: util.resolveActor(req), userToken: req.user?.accessToken,
        });
        logUserAction('API_KEY_GENERATED', req, { orgId, apiId: apiHandle, keyId: result.keyId, resourceUuid: result.keyId, resourceType: 'api_key' });
        return res.status(201).json(result);
    } catch (err) {
        logger.error('Failed to generate API key', { error: err.message, orgId, apiId: apiHandle });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: 'Failed to generate API key' });
    }
}

/**
 * GET /api/v0.9/apis/:apiId/api-keys
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
        const apiId = await resolveApiId(orgId, apiHandle, req);
        if (!apiId) {
            return res.status(404).json({
                status: 'error', code: '404', message: 'Not Found', errors: [{ field: 'apiId', message: 'API not found' }],
            });
        }
        const appId = await resolveAppId(orgId, util.resolveActor(req), appIdResult.value);
        if (appIdResult.value && !appId) {
            return res.status(404).json({
                status: 'error', code: '404', message: 'Not Found', errors: [{ field: 'appId', message: 'Application not found' }],
            });
        }
        const keys = await apiKeyService.list(orgId, {
            apiId,
            subscriptionId: subscriptionId || undefined,
            appId,
            status: status || undefined,
            createdBy: util.resolveActor(req),
        });
        const mapped = await mapKeysWithAudit(keys);
        return res.status(200).json(util.toPaginatedList(mapped, req));
    } catch (err) {
        logger.error('Failed to list API keys', { error: err.message, orgId });
        return res.status(errorStatus(err)).json({
            status: 'error',
            code: 'INTERNAL_SERVER_ERROR',
            message: 'Failed to list API keys',
            errors: [],
        });
    }
}

/**
 * POST /api/v0.9/apis/:apiId/api-keys/regenerate
 * Body: { keyId, expiresAt? } — keyId is the key's handle (the `id` returned by generate/list).
 */
async function regenerateApiKey(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { keyId: keyHandle, expiresAt } = req.body || {};

    if (!keyHandle || typeof keyHandle !== 'string' || !keyHandle.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res, req);
        if (!apiId) return;
        const keyId = await resolveKeyIdOrRespond(orgId, apiId, keyHandle.trim(), res);
        if (!keyId) return;
        const result = await apiKeyService.regenerate({
            orgId, apiId, keyId, expiresAt, actor: util.resolveActor(req), userToken: req.user?.accessToken,
        });
        logUserAction('API_KEY_REGENERATED', req, { orgId, apiId: apiHandle, keyId, resourceUuid: keyId, resourceType: 'api_key' });
        return res.status(200).json(result);
    } catch (err) {
        logger.error('Failed to regenerate API key', { error: err.message, orgId, apiId: apiHandle, keyHandle });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: 'Failed to regenerate API key' });
    }
}

/**
 * POST /api/v0.9/apis/:apiId/api-keys/revoke
 * Body: { keyId } — keyId is the key's handle (the `id` returned by generate/list).
 */
async function revokeApiKey(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { keyId: keyHandle } = req.body || {};

    if (!keyHandle || typeof keyHandle !== 'string' || !keyHandle.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res, req);
        if (!apiId) return;
        const keyId = await resolveKeyIdOrRespond(orgId, apiId, keyHandle.trim(), res);
        if (!keyId) return;
        await apiKeyService.revoke({ orgId, apiId, keyId, actor: util.resolveActor(req), userToken: req.user?.accessToken });
        logUserAction('API_KEY_REVOKED', req, { orgId, apiId: apiHandle, keyId, resourceUuid: keyId, resourceType: 'api_key' });
        return res.status(204).send();
    } catch (err) {
        logger.error('Failed to revoke API key', { error: err.message, orgId, apiId: apiHandle, keyHandle });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: 'Failed to revoke API key' });
    }
}

/**
 * POST /api/v0.9/apis/:apiId/api-keys/associate
 * Body: { keyId, appId } — keyId is the key's handle (the `id` returned by generate/list).
 */
async function associateApiKeyApplication(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { keyId: keyHandle, appId: appHandle } = req.body || {};

    if (!keyHandle || typeof keyHandle !== 'string' || !keyHandle.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }
    if (!appHandle || typeof appHandle !== 'string' || !appHandle.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'appId is required' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res, req);
        if (!apiId) return;
        const keyId = await resolveKeyIdOrRespond(orgId, apiId, keyHandle.trim(), res);
        if (!keyId) return;
        const appId = await resolveAppId(orgId, util.resolveActor(req), appHandle.trim());
        if (!appId) {
            return res.status(404).json({ code: '404', message: 'Not Found', description: 'Application not found' });
        }
        const result = await apiKeyService.associateApplication({
            orgId, apiId, keyId, appId, actor: util.resolveActor(req),
        });
        logUserAction('API_KEY_APP_ASSOCIATED', req, { orgId, apiId: apiHandle, keyId, appId: appHandle, resourceUuid: keyId, resourceType: 'api_key' });
        return res.status(200).json(result);
    } catch (err) {
        logger.error('Failed to associate application with API key', { error: err.message, orgId, apiId: apiHandle, keyHandle });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: 'Failed to associate application with API key' });
    }
}

/**
 * POST /api/v0.9/apis/:apiId/api-keys/dissociate
 * Body: { keyId } — keyId is the key's handle (the `id` returned by generate/list).
 */
async function removeApiKeyApplication(req, res) {
    const orgId = req.orgId;
    const apiHandle = req.params.apiId;
    const { keyId: keyHandle } = req.body || {};

    if (!keyHandle || typeof keyHandle !== 'string' || !keyHandle.trim()) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: 'keyId is required' });
    }

    try {
        const apiId = await resolveApiIdOrRespond(orgId, apiHandle, res, req);
        if (!apiId) return;
        const keyId = await resolveKeyIdOrRespond(orgId, apiId, keyHandle.trim(), res);
        if (!keyId) return;
        await apiKeyService.removeApplicationAssociation({ orgId, apiId, keyId, actor: util.resolveActor(req) });
        logUserAction('API_KEY_APP_DISASSOCIATED', req, { orgId, apiId: apiHandle, keyId, resourceUuid: keyId, resourceType: 'api_key' });
        return res.status(204).send();
    } catch (err) {
        logger.error('Failed to remove application association from API key', { error: err.message, orgId, apiId: apiHandle, keyHandle });
        return res.status(errorStatus(err)).json({ code: String(errorStatus(err)), message: 'Failed to remove application association from API key' });
    }
}

/**
 * GET /api/v0.9/applications/:applicationId/api-keys
 * Lists all API keys (across every API) currently associated with an app.
 */
async function listApplicationApiKeys(req, res) {
    const orgId = req.orgId;
    const { applicationId: applicationHandle } = req.params;

    try {
        const appRecord = await applicationDao.getId(orgId, util.resolveActor(req), applicationHandle);
        if (!appRecord) {
            return res.status(404).json({ code: '404', message: 'Application not found' });
        }
        const applicationId = appRecord.uuid;
        const keys = await apiKeyService.list(orgId, { appId: applicationId });
        return res.status(200).json(util.toPaginatedList(await mapKeysWithAudit(keys), req));
    } catch (err) {
        logger.error('Failed to list application API keys', { error: err.message, orgId, applicationId: applicationHandle });
        return res.status(errorStatus(err)).json({
            status: 'error',
            code: 'INTERNAL_SERVER_ERROR',
            message: 'Failed to list application API keys',
            errors: [],
        });
    }
}

module.exports = {
    generateApiKey, listApiKeys, regenerateApiKey, revokeApiKey,
    associateApiKeyApplication, removeApiKeyApplication, listApplicationApiKeys
};
