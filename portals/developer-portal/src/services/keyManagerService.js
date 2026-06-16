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
const yaml = require('js-yaml');
const { Sequelize } = require('sequelize');
const kmDao = require('../dao/keyManagerDao');
const { KeyManagerDTO, KeyManagerPublicDTO } = require('../dto/keyManagerDto');
const constants = require('../utils/constants');
const logger = require('../config/logger');

// ---------------------------------------------------------------------------
// YAML ingestion helpers (mirrors parseIdentityProviderFromYamlFile pattern)
// ---------------------------------------------------------------------------

/**
 * Map a parsed KeyManager YAML document to the service-layer payload format.
 */
function mapYamlToKeyManager(yamlDoc) {
    const spec = yamlDoc.spec || {};
    return {
        name: yamlDoc.metadata?.name || spec.name,
        type: spec.type,
        enabled: spec.enabled !== undefined ? spec.enabled : true,
        tokenEndpoint: spec.tokenEndpoint,
        clientRegistrationEndpoint: spec.clientRegistrationEndpoint,
        issuer: spec.issuer || null,
        jwksURL: spec.jwksURL || null,
        adminClientId: spec.adminClientId || '',
        adminClientSecret: spec.adminClientSecret || '',
        supportedGrantTypes: spec.grantTypes || spec.supportedGrantTypes || ['client_credentials'],
        supportedScopes: spec.scopes || spec.supportedScopes || ['openid'],
        additionalProperties: spec.additionalProperties || {},
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
    return req.body;
}

function _validateRequiredFields(payload) {
    const missing = ['name', 'type', 'tokenEndpoint', 'clientRegistrationEndpoint',
        'adminClientId', 'adminClientSecret']
        .filter(f => !payload[f]);
    if (missing.length) {
        return `Missing required fields: ${missing.join(', ')}`;
    }
    if (!Object.values(constants.KEY_MANAGER_TYPES).includes(payload.type)) {
        return `Invalid key manager type: ${payload.type}. `
            + `Supported: ${Object.values(constants.KEY_MANAGER_TYPES).join(', ')}`;
    }
    return null;
}

// ---------------------------------------------------------------------------
// CRUD service methods
// ---------------------------------------------------------------------------

const createKeyManager = async (req, res) => {
    try {
        const orgId = req.params.orgId;
        const payload = _resolvePayload(req);

        const validationError = _validateRequiredFields(payload);
        if (validationError) {
            return res.status(400).json({ error: validationError });
        }

        const record = await kmDao.createKeyManager(orgId, payload);
        const dto = new KeyManagerDTO(record);
        return res.status(201).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            return res.status(409).json({
                error: `A key manager with name "${req.body?.name}" already exists in this organization.`
            });
        }
        if (error.name === 'YAMLException' || error.name === 'ValidationError') {
            return res.status(400).json({ error: error.message });
        }
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_CREATE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_CREATE_ERROR });
    }
};

const updateKeyManager = async (req, res) => {
    try {
        const { kmId } = req.params;
        const payload = _resolvePayload(req);

        if (payload.type && !Object.values(constants.KEY_MANAGER_TYPES).includes(payload.type)) {
            return res.status(400).json({
                error: `Invalid key manager type: ${payload.type}.`
            });
        }

        const [, updatedRows] = await kmDao.updateKeyManager(kmId, payload);
        const dto = new KeyManagerDTO(updatedRows[0]);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND });
        }
        if (error instanceof Sequelize.UniqueConstraintError) {
            return res.status(409).json({
                error: `A key manager with that name already exists in this organization.`
            });
        }
        if (error.name === 'YAMLException' || error.name === 'ValidationError') {
            return res.status(400).json({ error: error.message });
        }
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_UPDATE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_UPDATE_ERROR });
    }
};

const getKeyManagers = async (req, res) => {
    try {
        const orgId = req.params.orgId;
        const records = await kmDao.getKeyManagers(orgId);
        const dtos = records.map(r => new KeyManagerDTO(r));
        return res.status(200).json(dtos);
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR });
    }
};

const getKeyManager = async (req, res) => {
    try {
        const { kmId } = req.params;
        const record = await kmDao.getKeyManager(kmId);
        const dto = new KeyManagerDTO(record);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND });
        }
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR });
    }
};

const deleteKeyManager = async (req, res) => {
    try {
        const { kmId } = req.params;
        await kmDao.deleteKeyManager(kmId);
        return res.status(204).send();
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND });
        }
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_DELETE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_DELETE_ERROR });
    }
};

/**
 * Developer-facing: list only enabled key managers (no admin creds).
 */
const getAvailableKeyManagers = async (req, res) => {
    try {
        const orgId = req.params.orgId;
        const records = await kmDao.getEnabledKeyManagers(orgId);
        const dtos = records.map(r => new KeyManagerPublicDTO(r));
        return res.status(200).json(dtos);
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR });
    }
};

module.exports = {
    createKeyManager,
    updateKeyManager,
    getKeyManagers,
    getKeyManager,
    deleteKeyManager,
    getAvailableKeyManagers,
    // Exported for use in org creation YAML ingestion
    mapYamlToKeyManager,
    parseKeyManagerFromYamlFile,
    parseKeyManagersFromYamlFile,
};
