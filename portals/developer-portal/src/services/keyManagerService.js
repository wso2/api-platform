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
const yaml = require('../utils/yaml');
const { Sequelize } = require('sequelize');
const kmDao = require('../dao/keyManagerDao');
const { KeyManagerDTO, KeyManagerPublicDTO } = require('../dto/keyManagerDto');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const constants = require('../utils/constants');
const util = require('../utils/util');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');

// ---------------------------------------------------------------------------
// YAML ingestion helpers (mirrors parseIdentityProviderFromYamlFile pattern)
// ---------------------------------------------------------------------------

/**
 * Map a parsed KeyManager YAML document to the service-layer payload format.
 */
function mapYamlToKeyManager(yamlDoc) {
    const spec = yamlDoc.spec || {};
    return {
        handle: yamlDoc.metadata?.name || spec.name,
        displayName: spec.displayName || spec.name,
        enabled: spec.enabled !== undefined ? spec.enabled : true,
        tokenEndpoint: spec.tokenEndpoint,
    };
}

/**
 * Parse a single keymanager.yaml buffer into a service-layer payload.
 */
function parseKeyManagerFromYamlFile(buffer) {
    const yamlDoc = yaml.load(buffer.toString('utf8'));
    if (!yamlDoc) {
        const err = new Error('Empty YAML file');
        err.name = 'ValidationError';
        throw err;
    }
    if (yamlDoc.kind !== 'KeyManager') {
        const err = new Error(`Unexpected YAML kind: ${yamlDoc.kind}. Expected "KeyManager".`);
        err.name = 'ValidationError';
        throw err;
    }
    return mapYamlToKeyManager(yamlDoc);
}

/**
 * Parse a YAML buffer that may contain multiple KeyManager documents.
 * Supports the `---` multi-doc separator.
 */
function parseKeyManagersFromYamlFile(buffer) {
    const docs = yaml.loadAll(buffer.toString('utf8'));
    return docs
        .filter(doc => doc && doc.kind === 'KeyManager')
        .map(mapYamlToKeyManager);
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Resolve the payload from the request — either a JSON body or a YAML file upload.
 * When a `keymanager` file is attached, parse it; otherwise fall back to req.body.
 */
function _resolvePayload(req) {
    const file = req.files?.keymanager?.[0] || req.file;
    if (file) {
        return parseKeyManagerFromYamlFile(file.buffer);
    }
    const payload = req.body;
    if (payload && payload.id) {
        payload.handle = payload.id;
    }
    return payload;
}

const generateHandle = (name) =>
    name.toLowerCase().trim()
        .replace(/[^\w\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .substring(0, 100);

// Handles are used to build route segments, so user-supplied ids must be restricted
// to the same safe character set generateHandle() produces.
const HANDLE_PATTERN = /^[a-zA-Z0-9_-]+$/;

function _validateRequiredFields(payload) {
    const missing = ['displayName', 'tokenEndpoint']
        .filter(f => !payload[f]);
    if (missing.length) {
        return `Missing required fields: ${missing.join(', ')}`;
    }
    const endpoint = payload.tokenEndpoint.trim();
    if (!endpoint) {
        return 'tokenEndpoint must not be blank';
    }
    try {
        new URL(endpoint);
    } catch {
        return 'tokenEndpoint must be a valid URL';
    }
    return null;
}

// ---------------------------------------------------------------------------
// CRUD service methods
// ---------------------------------------------------------------------------

const createKeyManager = async (req, res) => {
    try {
        const orgId = req.orgId;
        const payload = _resolvePayload(req);

        const validationError = _validateRequiredFields(payload);
        if (validationError) {
            return util.sendError(res, 400, validationError);
        }
        const hadExplicitHandle = !!(payload.handle && payload.handle.trim());
        const resolvedHandle = hadExplicitHandle ? payload.handle.trim() : generateHandle(payload.displayName);
        if (!resolvedHandle || !HANDLE_PATTERN.test(resolvedHandle)) {
            return util.sendError(res, 400, "Invalid 'id'. Must contain only letters, numbers, underscores, and hyphens.");
        }

        const userId = util.resolveActor(req);
        const record = await kmDao.create(orgId, { ...payload, handle: resolvedHandle }, userId);
        logUserAction('KEY_MANAGER_CREATED', req, { orgId, kmId: record.uuid, resourceUuid: record.uuid, resourceType: 'key_manager' });
        let audit;
        try {
            audit = await userIdpReferenceDao.buildSingleAuditFields(record);
        } catch (auditError) {
            logger.error('Audit field resolution failed after key manager creation', {
                error: auditError.message,
                kmId: record.uuid
            });
            audit = { createdAt: record.created_at, updatedAt: record.updated_at };
        }
        const dto = new KeyManagerDTO(record, audit);
        return res.status(201).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            return util.sendError(res, 409, `A key manager with that id already exists in this organization.`);
        }
        if (error.name === 'YAMLException' || error.name === 'ValidationError') {
            return util.sendError(res, 400, 'Invalid payload format or validation failed.');
        }
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_CREATE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.KEY_MANAGER_CREATE_ERROR);
    }
};

const updateKeyManager = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { kmId: kmHandle } = req.params;
        const payload = _resolvePayload(req);

        const kmId = await kmDao.getIdByHandle(orgId, kmHandle);
        if (!kmId) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }

        const userId = util.resolveActor(req);
        const [, updatedRows] = await kmDao.update(kmId, payload, userId);
        logUserAction('KEY_MANAGER_UPDATED', req, { orgId, kmId, resourceUuid: kmId, resourceType: 'key_manager' });
        let audit;
        try {
            audit = await userIdpReferenceDao.buildSingleAuditFields(updatedRows[0]);
        } catch (auditError) {
            logger.error('Audit field resolution failed after key manager update', {
                error: auditError.message,
                kmId
            });
            audit = { createdAt: updatedRows[0].created_at, updatedAt: updatedRows[0].updated_at };
        }
        const dto = new KeyManagerDTO(updatedRows[0], audit);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }
        if (error instanceof Sequelize.UniqueConstraintError) {
            return util.sendError(res, 409, `A key manager with that id already exists in this organization.`);
        }
        if (error.name === 'YAMLException' || error.name === 'ValidationError') {
            return util.sendError(res, 400, 'Invalid payload format or validation failed.');
        }
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_UPDATE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.KEY_MANAGER_UPDATE_ERROR);
    }
};

/**
 * Admins get the full configuration for every key manager; other callers get the
 * minimal, developer-facing view of enabled key managers only (no admin creds).
 */
const getKeyManagers = async (req, res) => {
    try {
        const orgId = req.orgId;
        const isAdmin = req.user?.isAdmin;
        const records = isAdmin ? await kmDao.list(orgId) : await kmDao.listEnabled(orgId);
        const auditList = isAdmin ? await userIdpReferenceDao.buildListAuditFields(records) : [];
        const dtos = records.map((r, i) => (isAdmin ? new KeyManagerDTO(r, auditList[i]) : new KeyManagerPublicDTO(r)));
        return res.status(200).json(util.toPaginatedList(dtos, req));
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR);
    }
};

const getKeyManager = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { kmId: kmHandle } = req.params;
        const kmId = await kmDao.getIdByHandle(orgId, kmHandle);
        if (!kmId) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }
        const record = await kmDao.get(kmId);
        const audit = await userIdpReferenceDao.buildSingleAuditFields(record);
        const dto = new KeyManagerDTO(record, audit);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR);
    }
};

const deleteKeyManager = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { kmId: kmHandle } = req.params;
        const kmId = await kmDao.getIdByHandle(orgId, kmHandle);
        if (!kmId) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }
        await kmDao.delete(kmId);
        logUserAction('KEY_MANAGER_DELETED', req, { orgId, kmId, resourceUuid: kmId, resourceType: 'key_manager' });
        return res.status(204).send();
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_DELETE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.KEY_MANAGER_DELETE_ERROR);
    }
};

module.exports = {
    createKeyManager,
    updateKeyManager,
    getKeyManagers,
    getKeyManager,
    deleteKeyManager,
    // Exported for use in org creation YAML ingestion
    mapYamlToKeyManager,
    parseKeyManagerFromYamlFile,
    parseKeyManagersFromYamlFile,
};
