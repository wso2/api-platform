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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
/* eslint-disable no-undef */
const { Sequelize, Op } = require('sequelize');
const sequelize = require('../db/sequelizeConfig');
const { APIMetadata } = require('../models/apiMetadata');
const apiDao = require('../dao/apiMetadataDao');
const adminDao = require('../dao/adminDao');
const ServerResponseDTO = require('../dto/mcpServerDto');
const logger = require('../config/logger');
const constants = require('../utils/constants');

const MCP_STATUSES = ['active', 'deprecated', 'deleted'];
const SERVER_NAME_PATTERN = /^[a-zA-Z0-9._-]+\/[a-zA-Z0-9._-]+$/;
const VERSION_RANGE_PATTERN = /^[\^~]|^>=?|^<=?|\*|(^|\.)x(\.|$)/i;
const DEFAULT_LIMIT = 30;
const MAX_LIMIT = 100;
const SCHEMA_FILE_NAME = 'schema.json';

// Map registry spec status values to DB STATUS values
const REGISTRY_TO_DB_STATUS = {
    active: 'PUBLISHED',
    deprecated: 'DEPRECATED',
    deleted: 'DELETED'
};

function normalizeLimit(limit) {
    const n = parseInt(limit, 10);
    if (Number.isNaN(n) || n <= 0) return DEFAULT_LIMIT;
    return Math.min(n, MAX_LIMIT);
}

function unescapeParam(str) {
    return str
        .replace(/&#x2F;/gi, '/');
}



async function findRowByServerIdentifier(orgId, serverIdentifier, version, transaction) {
    const baseWhere = { ORG_ID: orgId, API_TYPE: constants.API_TYPE.MCP, REFERENCE_ID: null };
    if (version) baseWhere.API_VERSION = version;

    const byProxyId = await APIMetadata.findOne({
        where: {
            ...baseWhere,
            [Op.and]: sequelize.where(
                sequelize.literal("\"METADATA_SEARCH\"->'apiInfo'->>'proxyId'"),
                serverIdentifier
            )
        },
        transaction
    });
    if (byProxyId) return byProxyId;

    const byApiName = await APIMetadata.findOne({
        where: { ...baseWhere, API_NAME: serverIdentifier },
        transaction
    });
    if (byApiName) return byApiName;

    const slashIdx = serverIdentifier.indexOf('/');
    if (slashIdx !== -1) {
        const bareHandle = serverIdentifier.slice(slashIdx + 1);
        if (bareHandle) {
            const proxyIdData = await APIMetadata.findOne({
                where: {
                    ...baseWhere,
                    [Op.and]: sequelize.where(
                        sequelize.literal("\"METADATA_SEARCH\"->'apiInfo'->>'proxyId'"),
                        bareHandle
                    )
                },
                transaction
            });
            if (proxyIdData) return proxyIdData;

            return APIMetadata.findOne({
                where: { ...baseWhere, API_NAME: bareHandle },
                transaction
            });
        }
    }

    return null;
}

function parseBool(value, defaultValue) {
    if (value === undefined || value === null || value === '') return defaultValue;
    return String(value).toLowerCase() === 'true';
}

function validateServerDetail(detail) {
    if (!detail || typeof detail !== 'object') {
        return 'Request body must be a JSON object';
    }
    if (!detail.name || typeof detail.name !== 'string') {
        return '"name" is required';
    }
    if (detail.name.length < 3 || detail.name.length > 200) {
        return '"name" length must be between 3 and 200 characters';
    }
    if (!SERVER_NAME_PATTERN.test(detail.name)) {
        return '"name" must match reverse-DNS pattern "namespace/server"';
    }
    if (typeof detail.description !== 'string') {
        return '"description" is required';
    }
    if (!detail.version || typeof detail.version !== 'string') {
        return '"version" is required';
    }
    if (detail.version === 'latest') {
        return '"version" cannot be the reserved value "latest"';
    }
    if (VERSION_RANGE_PATTERN.test(detail.version)) {
        return '"version" must be a specific version, not a range';
    }
    return null;
}

function sendError(res, status, message) {
    return res.status(status).json({ error: message });
}

function handleUnexpectedError(res, error, operation, fallbackMessage) {
    logger.error('MCP registry operation failed', {
        operation,
        error: error.message,
        stack: error.stack
    });
    if (error instanceof Sequelize.ValidationError) {
        return sendError(res, 400, error.message);
    }
    if (error instanceof Sequelize.UniqueConstraintError) {
        return sendError(res, 409, 'Server version already exists');
    }
    if (error instanceof Sequelize.EmptyResultError) {
        return sendError(res, 404, 'Organization not found');
    }
    return sendError(res, 500, fallbackMessage);
}

/**
 * Resolves orgId UUID from the orgHandle path parameter.
 * Throws Sequelize.EmptyResultError if org not found.
 */
async function resolveOrgId(orgHandle) {
    return adminDao.getOrgId(orgHandle);
}


function deriveApiHandle(name, orgHandle) {
    if (name.includes('/')) {
        return name.slice(name.indexOf('/') + 1);
    }
    if (orgHandle) {
        const prefix = orgHandle.toLowerCase() + '-';
        if (name.toLowerCase().startsWith(prefix)) {
            return name.slice(prefix.length);
        }
    }
    return name;
}

/**
 * Builds the apiMetadata shape expected by apiDao.createAPIMetadata / updateAPIMetadata.
 */
function buildApiMetadataPayload(name, version, description, remotes, title, publishedAt, updatedAt, proxyId, orgHandle) {
    const normalizedRemotes = (Array.isArray(remotes) ? remotes : []).map(r => ({
        type: r.type || 'streamable-http',
        url: r.url || ''
    }));
    const primaryUrl = normalizedRemotes.length > 0 ? normalizedRemotes[0].url : '';
    const apiHandle = deriveApiHandle(name, orgHandle);
    return {
        apiInfo: {
            referenceID: null,
            provider: 'WSO2',
            apiName: name,
            apiHandle,
            apiTitle: title || null,
            apiDescription: description || `${name} MCP proxy`,
            apiVersion: version,
            apiType: constants.API_TYPE.MCP,
            apiStatus: 'PUBLISHED',
            visibility: 'PUBLIC',
            gatewayType: null,
            remotes: normalizedRemotes,
            publishedAt: publishedAt || null,
            updatedAt: updatedAt || null,
            proxyId: proxyId || null
        },
        endPoints: {
            productionURL: primaryUrl,
            sandboxURL: null
        }
    };
}

/**
 * Parses schema content from a DP_API_CONTENT row's API_FILE buffer.
 */
function parseSchema(contentRow) {
    if (!contentRow) return null;
    try {
        const raw = contentRow.API_FILE;
        const str = Buffer.isBuffer(raw) ? raw.toString('utf-8') : String(raw);
        return JSON.parse(str);
    } catch (e) {
        logger.warn('Failed to parse MCP schema content', { error: e.message });
        return null;
    }
}

// ─── Public discovery endpoints ──────────────────────────────────────────────

const listServers = async (req, res) => {
    try {
        const orgHandle = req.params.orgHandle;
        const orgId = await resolveOrgId(orgHandle);
        const includeDeleted = parseBool(req.query.include_deleted, false);
        const limit = normalizeLimit(req.query.limit);
        const search = req.query.search;

        let currentOffset = 0;
        if (req.query.cursor) {
            try {
                const decoded = JSON.parse(Buffer.from(req.query.cursor, 'base64url').toString('utf-8'));
                if (typeof decoded.offset === 'number' && decoded.offset >= 0) {
                    currentOffset = decoded.offset;
                }
            } catch {
                return sendError(res, 400, 'Invalid cursor');
            }
        }

        const where = { ORG_ID: orgId, API_TYPE: constants.API_TYPE.MCP };
        if (!includeDeleted) {
            where.STATUS = { [Op.ne]: 'DELETED' };
        }
        if (search) {
            where[Op.or] = [
                { API_NAME: { [Op.iLike]: `%${search}%` } },
                sequelize.where(
                    sequelize.literal("\"METADATA_SEARCH\"->'apiInfo'->>'proxyId'"),
                    { [Op.iLike]: `%${search}%` }
                )
            ];
        }

        const rows = await APIMetadata.findAll({
            where,
            order: [[sequelize.literal("(\"METADATA_SEARCH\"->'apiInfo'->>'publishedAt')"), 'DESC NULLS LAST']],
            limit: limit + 1,
            offset: currentOffset
        });

        const hasMore = rows.length > limit;
        const pageRows = hasMore ? rows.slice(0, limit) : rows;
        const metadata = { count: pageRows.length };
        if (hasMore) {
            metadata.nextCursor = Buffer.from(JSON.stringify({ offset: currentOffset + limit })).toString('base64url');
        }

        return res.status(200).json({
            servers: pageRows.map(row => new ServerResponseDTO(row)),
            metadata
        });
    } catch (error) {
        return handleUnexpectedError(res, error, 'listServers', 'Failed to list servers');
    }
};

const listVersions = async (req, res) => {
    try {
        const orgHandle = req.params.orgHandle;
        const orgId = await resolveOrgId(orgHandle);
        const serverIdentifier = unescapeParam(decodeURIComponent(req.params.serverName));
        const includeDeleted = parseBool(req.query.include_deleted, false);

        const baseWhere = { ORG_ID: orgId, API_TYPE: constants.API_TYPE.MCP };
        if (!includeDeleted) {
            baseWhere.STATUS = { [Op.ne]: 'DELETED' };
        }

        let rows = await APIMetadata.findAll({
            where: {
                ...baseWhere,
                [Op.and]: sequelize.where(
                    sequelize.literal("\"METADATA_SEARCH\"->'apiInfo'->>'proxyId'"),
                    serverIdentifier
                )
            },
            order: [[sequelize.literal("(\"METADATA_SEARCH\"->'apiInfo'->>'publishedAt')"), 'DESC NULLS LAST']]
        });

        if (rows.length === 0) {
            rows = await APIMetadata.findAll({
                where: { ...baseWhere, API_NAME: serverIdentifier },
                order: [[sequelize.literal("(\"METADATA_SEARCH\"->'apiInfo'->>'publishedAt')"), 'DESC NULLS LAST']]
            });
        }

        if (rows.length === 0) {
            return sendError(res, 404, 'Server not found');
        }

        return res.status(200).json({
            servers: rows.map(row => new ServerResponseDTO(row)),
            metadata: { count: rows.length }
        });
    } catch (error) {
        return handleUnexpectedError(res, error, 'listVersions', 'Failed to list server versions');
    }
};

const getVersion = async (req, res) => {
    try {
        const orgHandle = req.params.orgHandle;
        const orgId = await resolveOrgId(orgHandle);
        const serverIdentifier = unescapeParam(decodeURIComponent(req.params.serverName));
        const version = unescapeParam(decodeURIComponent(req.params.version));

        let row = await findRowByServerIdentifier(orgId, serverIdentifier, version, null);
        if (!row) {
            return sendError(res, 404, 'Server not found');
        }
        if (!parseBool(req.query.include_deleted, false) && row.STATUS === 'DELETED') {
            return sendError(res, 404, 'Server not found');
        }

        const schemaContent = await apiDao.getAPIDoc(
            constants.DOC_TYPES.SCHEMA_DEFINITION, orgId, row.API_ID, null
        );
        const schema = parseSchema(schemaContent);

        return res.status(200).json(new ServerResponseDTO(row, schema));
    } catch (error) {
        return handleUnexpectedError(res, error, 'getVersion', 'Failed to get server version');
    }
};

// ─── Write endpoints ──────────────────────────────────────────────────────────

const publishServer = async (req, res) => {
    try {
        const orgHandle = req.params.orgHandle;
        const detail = req.body;

        const validationError = validateServerDetail(detail);
        if (validationError) {
            return sendError(res, 400, validationError);
        }

        const orgId = await resolveOrgId(orgHandle);
        const { name, version, title, description, _meta } = detail;
        const remotes = detail.remotes || [];
        const choreoMeta = _meta && _meta['io.api-platform/mcp-capabilities'];
        const proxyId = (_meta && _meta['io.api-platform/proxy-info']?.id) || null;
        const tools = choreoMeta?.tools || [];
        const resources = choreoMeta?.resources || [];
        const prompts = choreoMeta?.prompts || [];
        const now = new Date().toISOString();
        const schemaBuffer = choreoMeta
            ? Buffer.from(JSON.stringify({ tools, resources, prompts }), 'utf-8')
            : null;

        let row;
        let created = false;
        let existingApiId = null;

        await sequelize.transaction(async (t) => {
            let existing = null;
            if (proxyId) {
                existing = await APIMetadata.findOne({
                    where: {
                        ORG_ID: orgId,
                        API_TYPE: constants.API_TYPE.MCP,
                        API_VERSION: version,
                        [Op.and]: sequelize.where(
                            sequelize.literal("\"METADATA_SEARCH\"->'apiInfo'->>'proxyId'"),
                            proxyId
                        )
                    },
                    transaction: t
                });
            }

            if (!existing) {
                existing = await APIMetadata.findOne({
                    where: { ORG_ID: orgId, API_TYPE: constants.API_TYPE.MCP, API_NAME: name, API_VERSION: version },
                    transaction: t
                });
            }

            if (existing) {
                existingApiId = existing.API_ID;
                const existingPublishedAt = existing.METADATA_SEARCH?.apiInfo?.publishedAt || now;
                const apiMetadataPayload = buildApiMetadataPayload(name, version, description, remotes, title, existingPublishedAt, now, proxyId, orgHandle);
                await apiDao.updateAPIMetadata(orgId, existing.API_ID, apiMetadataPayload, t);
                await apiDao.createAPILabelMapping(orgId, existing.API_ID, ['default'], t);
                if (schemaBuffer) {
                    await apiDao.updateOrCreateAPIFiles(
                        [{ content: schemaBuffer, fileName: SCHEMA_FILE_NAME, type: constants.DOC_TYPES.SCHEMA_DEFINITION }],
                        existing.API_ID, orgId, t
                    );
                }
                row = await APIMetadata.findOne({ where: { API_ID: existing.API_ID }, transaction: t });
            } else {
                const apiMetadataPayload = buildApiMetadataPayload(name, version, description, remotes, title, now, now, proxyId, orgHandle);
                const created_row = await apiDao.createAPIMetadata(orgId, apiMetadataPayload, t);
                const apiId = created_row.dataValues.API_ID;
                await apiDao.createAPILabelMapping(orgId, apiId, ['default'], t);
                const newSchemaBuffer = schemaBuffer || Buffer.from(JSON.stringify({ tools: [], resources: [], prompts: [] }), 'utf-8');
                await apiDao.storeAPIFile(newSchemaBuffer, SCHEMA_FILE_NAME, apiId, constants.DOC_TYPES.SCHEMA_DEFINITION, t);
                row = await APIMetadata.findOne({ where: { API_ID: apiId }, transaction: t });
                created = true;
            }
        });

        logger.info('MCP server published', { name, version, orgHandle });
        let schema;
        if (choreoMeta) {
            schema = { tools, resources, prompts };
        } else if (existingApiId) {
            const schemaContent = await apiDao.getAPIDoc(constants.DOC_TYPES.SCHEMA_DEFINITION, orgId, existingApiId, null);
            schema = parseSchema(schemaContent);
        } else {
            schema = { tools: [], resources: [], prompts: [] };
        }
        return res.status(created ? 201 : 200).json(new ServerResponseDTO(row, schema));
    } catch (error) {
        return handleUnexpectedError(res, error, 'publishServer', 'Failed to publish server');
    }
};

const updateVersion = async (req, res) => {
    try {
        const orgHandle = req.params.orgHandle;
        const orgId = await resolveOrgId(orgHandle);
        const serverIdentifier = unescapeParam(decodeURIComponent(req.params.serverName));
        const version = unescapeParam(decodeURIComponent(req.params.version));
        const detail = req.body;

        const validationError = validateServerDetail(detail);
        if (validationError) {
            return sendError(res, 400, validationError);
        }
        if (detail.version !== version) {
            return sendError(res, 400, 'Version in body must match path');
        }

        const { title, description, _meta } = detail;
        const remotes = detail.remotes || [];
        const choreoMeta = _meta && _meta['io.api-platform/mcp-capabilities'];
        const proxyId = (_meta && _meta['io.api-platform/proxy-info']?.id) || null;
        const tools = choreoMeta?.tools || [];
        const resources = choreoMeta?.resources || [];
        const prompts = choreoMeta?.prompts || [];
        const schemaBuffer = choreoMeta
            ? Buffer.from(JSON.stringify({ tools, resources, prompts }), 'utf-8')
            : null;

        let row;
        let updatedApiId = null;
        await sequelize.transaction(async (t) => {
            const existing = await findRowByServerIdentifier(orgId, serverIdentifier, version, t);
            if (!existing) return;

            updatedApiId = existing.API_ID;
            const existingPublishedAt = existing.METADATA_SEARCH?.apiInfo?.publishedAt || new Date().toISOString();
            const existingProxyId = proxyId || existing.METADATA_SEARCH?.apiInfo?.proxyId || null;
            const apiMetadataPayload = buildApiMetadataPayload(existing.API_NAME, version, description, remotes, title, existingPublishedAt, new Date().toISOString(), existingProxyId, orgHandle);
            await apiDao.updateAPIMetadata(orgId, existing.API_ID, apiMetadataPayload, t);
            await apiDao.createAPILabelMapping(orgId, existing.API_ID, ['default'], t);
            if (schemaBuffer) {
                await apiDao.updateOrCreateAPIFiles(
                    [{ content: schemaBuffer, fileName: SCHEMA_FILE_NAME, type: constants.DOC_TYPES.SCHEMA_DEFINITION }],
                    existing.API_ID, orgId, t
                );
            }
            row = await APIMetadata.findOne({ where: { API_ID: existing.API_ID }, transaction: t });
        });

        if (!row) {
            return sendError(res, 404, 'Server version not found');
        }
        let schema;
        if (choreoMeta) {
            schema = { tools, resources, prompts };
        } else {
            const schemaContent = await apiDao.getAPIDoc(constants.DOC_TYPES.SCHEMA_DEFINITION, orgId, updatedApiId, null);
            schema = parseSchema(schemaContent);
        }
        return res.status(200).json(new ServerResponseDTO(row, schema));
    } catch (error) {
        return handleUnexpectedError(res, error, 'updateVersion', 'Failed to update server version');
    }
};

const deleteVersion = async (req, res) => {
    try {
        const orgHandle = req.params.orgHandle;
        const orgId = await resolveOrgId(orgHandle);
        const serverIdentifier = unescapeParam(decodeURIComponent(req.params.serverName));
        const version = unescapeParam(decodeURIComponent(req.params.version));

        const existing = await findRowByServerIdentifier(orgId, serverIdentifier, version, null);
        if (!existing) {
            return sendError(res, 404, 'Server version not found');
        }

        await APIMetadata.update(
            { STATUS: 'DELETED' },
            { where: { API_ID: existing.API_ID, ORG_ID: orgId } }
        );
        const deleted = await APIMetadata.findOne({ where: { API_ID: existing.API_ID } });
        logger.info('MCP server deleted', { serverIdentifier, version, orgHandle });
        return res.status(200).json(new ServerResponseDTO(deleted));
    } catch (error) {
        return handleUnexpectedError(res, error, 'deleteVersion', 'Failed to delete server version');
    }
};

const updateVersionStatus = async (req, res) => {
    try {
        const orgHandle = req.params.orgHandle;
        const orgId = await resolveOrgId(orgHandle);
        const serverIdentifier = unescapeParam(decodeURIComponent(req.params.serverName));
        const version = unescapeParam(decodeURIComponent(req.params.version));
        const { status } = req.body || {};

        if (!status || !MCP_STATUSES.includes(status)) {
            return sendError(res, 400, 'Invalid status value');
        }

        const existing = await findRowByServerIdentifier(orgId, serverIdentifier, version, null);
        if (!existing) {
            return sendError(res, 404, 'Server version not found');
        }

        const dbStatus = REGISTRY_TO_DB_STATUS[status];
        if (existing.STATUS === dbStatus) {
            return sendError(res, 400, `No changes to apply: status is already ${status}`);
        }

        await APIMetadata.update(
            { STATUS: dbStatus },
            { where: { API_ID: existing.API_ID, ORG_ID: orgId } }
        );
        const updated = await APIMetadata.findOne({ where: { API_ID: existing.API_ID } });
        return res.status(200).json(new ServerResponseDTO(updated));
    } catch (error) {
        return handleUnexpectedError(res, error, 'updateVersionStatus', 'Failed to update server status');
    }
};

const updateAllVersionsStatus = async (req, res) => {
    try {
        const orgHandle = req.params.orgHandle;
        const orgId = await resolveOrgId(orgHandle);
        const serverIdentifier = unescapeParam(decodeURIComponent(req.params.serverName));
        const { status } = req.body || {};

        if (!status || !MCP_STATUSES.includes(status)) {
            return sendError(res, 400, 'Invalid status value');
        }

        const dbStatus = REGISTRY_TO_DB_STATUS[status];
        let updated;

        // Build proxyId condition, fall back to API_NAME match
        const proxyIdCondition = sequelize.where(
            sequelize.literal("\"METADATA_SEARCH\"->>'apiInfo'->>'proxyId'"),
            serverIdentifier
        );

        await sequelize.transaction(async (t) => {
            let existing = await APIMetadata.findAll({
                where: { ORG_ID: orgId, API_TYPE: constants.API_TYPE.MCP, REFERENCE_ID: null, [Op.and]: proxyIdCondition },
                lock: t.LOCK.UPDATE,
                transaction: t
            });
            if (existing.length === 0) {
                existing = await APIMetadata.findAll({
                    where: { ORG_ID: orgId, API_TYPE: constants.API_TYPE.MCP, REFERENCE_ID: null, API_NAME: serverIdentifier },
                    lock: t.LOCK.UPDATE,
                    transaction: t
                });
            }
            if (existing.length === 0) return;

            const ids = existing.map(r => r.API_ID);
            await APIMetadata.update(
                { STATUS: dbStatus },
                { where: { API_ID: { [Op.in]: ids }, ORG_ID: orgId }, transaction: t }
            );
            updated = await APIMetadata.findAll({
                where: { API_ID: { [Op.in]: ids } },
                transaction: t
            });
        });

        if (!updated) {
            return sendError(res, 404, 'Server not found');
        }
        return res.status(200).json({
            updatedCount: updated.length,
            servers: updated.map(row => new ServerResponseDTO(row))
        });
    } catch (error) {
        return handleUnexpectedError(res, error, 'updateAllVersionsStatus', 'Failed to update server status');
    }
};

module.exports = {
    listServers,
    listVersions,
    getVersion,
    publishServer,
    updateVersion,
    deleteVersion,
    updateVersionStatus,
    updateAllVersionsStatus
};
