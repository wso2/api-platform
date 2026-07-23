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
const db = require('../db/driver');
const { ValidationError, NotFoundError } = require('../utils/errors/customErrors');
const apiDao = require('../dao/apiDao');
const apiFileDao = require('../dao/apiFileDao');
const labelDao = require('../dao/labelDao');
const orgDao = require('../dao/organizationDao');
const ServerResponseDTO = require('../dto/mcpServerDto');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const util = require('../utils/util');
const yaml = require('../utils/yaml');

const MCP_STATUSES = ['active', 'deprecated', 'deleted'];
const SERVER_NAME_PATTERN = /^[a-zA-Z0-9._-]+\/[a-zA-Z0-9._-]+$/;
const VERSION_RANGE_PATTERN = /^[\^~]|^>=?|^<=?|\*|(^|\.)x(\.|$)/i;
const DEFAULT_LIMIT = 30;
const MAX_LIMIT = 100;
// Canonical stored schema filename/shape across the whole platform: a flat, type-tagged
// YAML array (definition.yaml) — the same format the admin /mcp-servers API and the
// sample seeder write. Registry writes arrive grouped and are flattened to this on write
// (see toFlatSchema); parseSchema regroups on read.
const SCHEMA_FILE_NAME = constants.FILE_NAME.SCHEMA_DEFINITION_YAML_FILE_NAME;

const API_METADATA_TABLE = 'dp_api_metadata';

// metadata_search is JSONB on postgres — the ->> text-extraction operator used below
// is postgres-specific. This mirrors the previous Sequelize implementation, which used
// raw `sequelize.literal("metadata_search->>'proxyId'")` for the same reason: the MCP
// registry's proxyId/publishedAt lookups have only ever been postgres-only (same for the
// ILIKE case-insensitive search and the `FOR UPDATE` row lock further below).
const PROXY_ID_EXPR = "metadata_search->>'proxyId'";
const PUBLISHED_AT_EXPR = "(metadata_search->>'publishedAt')";

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
    const exec = transaction || db;
    const baseConditions = ['org_uuid = ?', 'type = ?', 'ref_id IS NULL'];
    const baseParams = [orgId, constants.API_TYPE.MCP];
    if (version) { baseConditions.push('version = ?'); baseParams.push(version); }
    const baseWhere = baseConditions.join(' AND ');

    const byProxyId = await exec.queryOne(
        `SELECT * FROM ${API_METADATA_TABLE} WHERE ${baseWhere} AND ${PROXY_ID_EXPR} = ?`,
        [...baseParams, serverIdentifier]
    );
    if (byProxyId) return byProxyId;

    const byApiName = await exec.queryOne(
        `SELECT * FROM ${API_METADATA_TABLE} WHERE ${baseWhere} AND name = ?`,
        [...baseParams, serverIdentifier]
    );
    if (byApiName) return byApiName;

    const slashIdx = serverIdentifier.indexOf('/');
    if (slashIdx !== -1) {
        const bareHandle = serverIdentifier.slice(slashIdx + 1);
        if (bareHandle) {
            const proxyIdData = await exec.queryOne(
                `SELECT * FROM ${API_METADATA_TABLE} WHERE ${baseWhere} AND ${PROXY_ID_EXPR} = ?`,
                [...baseParams, bareHandle]
            );
            if (proxyIdData) return proxyIdData;

            return exec.queryOne(
                `SELECT * FROM ${API_METADATA_TABLE} WHERE ${baseWhere} AND name = ?`,
                [...baseParams, bareHandle]
            );
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
    if (error instanceof ValidationError) {
        return sendError(res, 400, error.message);
    }
    if (db.isDuplicateKeyError(error)) {
        return sendError(res, 409, 'Server version already exists');
    }
    if (error instanceof NotFoundError) {
        return sendError(res, 404, 'Organization not found');
    }
    return sendError(res, 500, fallbackMessage);
}

/**
 * Resolves orgId UUID from the orgHandle path parameter.
 * Throws NotFoundError if org not found.
 */
async function resolveOrgId(orgHandle) {
    return orgDao.getId(orgHandle);
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
 * Builds the apiMetadata shape expected by apiDao.create / apiDao.update.
 */
function buildApiMetadataPayload(name, version, description, remotes, title, publishedAt, updatedAt, proxyId, orgHandle) {
    const normalizedRemotes = (Array.isArray(remotes) ? remotes : []).map(r => ({
        type: r.type || 'streamable-http',
        url: r.url || ''
    }));
    const primaryUrl = normalizedRemotes.length > 0 ? normalizedRemotes[0].url : '';
    const apiHandle = deriveApiHandle(name, orgHandle);
    return {
        referenceId: null,
        name: name,
        handle: apiHandle,
        title: title || null,
        description: description || `${name} MCP proxy`,
        version: version,
        type: constants.API_TYPE.MCP,
        status: 'PUBLISHED',
        remotes: normalizedRemotes,
        publishedAt: publishedAt || null,
        updatedAt: updatedAt || null,
        proxyId: proxyId || null,
        endPoints: {
            productionURL: primaryUrl,
            sandboxURL: null
        }
    };
}

/**
 * Parses schema content from a DP_API_CONTENT row's FILE_CONTENT buffer.
 */
function parseSchema(contentRow) {
    if (!contentRow) return null;
    try {
        const raw = contentRow.file_content;
        const str = Buffer.isBuffer(raw) ? raw.toString('utf-8') : String(raw);
        // The stored schema comes in one of two shapes depending on which path wrote it:
        //   - registry publishServer writes grouped JSON: { tools, resources, prompts }
        //   - the devportal admin API and sample seeder store the raw uploaded schemaDefinition,
        //     a flat YAML/JSON array of { type: TOOL|RESOURCE|PROMPT, ... } entries.
        // yaml.load parses both JSON and YAML; normalize a flat array into the grouped shape the
        // ServerResponseDTO expects, so the registry exposes capabilities however the server was
        // created (mirrors the landing-page parser in apiContentController).
        const parsed = yaml.load(str);
        if (Array.isArray(parsed)) {
            return {
                tools: parsed.filter(item => item && item.type === 'TOOL'),
                resources: parsed.filter(item => item && item.type === 'RESOURCE'),
                prompts: parsed.filter(item => item && item.type === 'PROMPT'),
            };
        }
        return parsed;
    } catch (e) {
        logger.warn('Failed to parse MCP schema content', { error: e.message });
        return null;
    }
}

// Inverse of parseSchema's grouping: turn grouped capabilities into the canonical flat,
// type-tagged array so every SCHEMA_DEFINITION row across the platform shares one shape.
function toFlatSchema(tools = [], resources = [], prompts = []) {
    return [
        ...tools.map((t) => ({ ...t, type: 'TOOL' })),
        ...resources.map((r) => ({ ...r, type: 'RESOURCE' })),
        ...prompts.map((p) => ({ ...p, type: 'PROMPT' })),
    ];
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

        const conditions = ['org_uuid = ?', 'type = ?'];
        const params = [orgId, constants.API_TYPE.MCP];
        if (!includeDeleted) {
            conditions.push("status != 'DELETED'");
        }
        if (search) {
            conditions.push(`(name ILIKE ? OR ${PROXY_ID_EXPR} ILIKE ?)`);
            params.push(`%${search}%`, `%${search}%`);
        }

        const rows = await db.query(
            `SELECT * FROM ${API_METADATA_TABLE} WHERE ${conditions.join(' AND ')}
             ORDER BY ${PUBLISHED_AT_EXPR} DESC NULLS LAST
             LIMIT ? OFFSET ?`,
            [...params, limit + 1, currentOffset]
        );

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

        const baseConditions = ['org_uuid = ?', 'type = ?'];
        const baseParams = [orgId, constants.API_TYPE.MCP];
        if (!includeDeleted) {
            baseConditions.push("status != 'DELETED'");
        }
        const baseWhereSql = baseConditions.join(' AND ');

        let rows = await db.query(
            `SELECT * FROM ${API_METADATA_TABLE} WHERE ${baseWhereSql} AND ${PROXY_ID_EXPR} = ?
             ORDER BY ${PUBLISHED_AT_EXPR} DESC NULLS LAST`,
            [...baseParams, serverIdentifier]
        );

        if (rows.length === 0) {
            rows = await db.query(
                `SELECT * FROM ${API_METADATA_TABLE} WHERE ${baseWhereSql} AND name = ?
                 ORDER BY ${PUBLISHED_AT_EXPR} DESC NULLS LAST`,
                [...baseParams, serverIdentifier]
            );
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
        if (!parseBool(req.query.include_deleted, false) && row.status === 'DELETED') {
            return sendError(res, 404, 'Server not found');
        }

        const schemaContent = await apiFileDao.getDoc(
            constants.DOC_TYPES.SCHEMA_DEFINITION, orgId, row.uuid, null
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
        const userId = util.resolveActor(req);

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
            ? Buffer.from(yaml.dump(toFlatSchema(tools, resources, prompts)), 'utf-8')
            : null;

        let row;
        let created = false;
        let existingApiId = null;

        await db.withTransaction(async (t) => {
            let existing = null;
            if (proxyId) {
                existing = await t.queryOne(
                    `SELECT * FROM ${API_METADATA_TABLE} WHERE org_uuid = ? AND type = ? AND version = ? AND ${PROXY_ID_EXPR} = ?`,
                    [orgId, constants.API_TYPE.MCP, version, proxyId]
                );
            }

            if (!existing) {
                existing = await t.queryOne(
                    `SELECT * FROM ${API_METADATA_TABLE} WHERE org_uuid = ? AND type = ? AND name = ? AND version = ?`,
                    [orgId, constants.API_TYPE.MCP, name, version]
                );
            }

            if (existing) {
                existingApiId = existing.uuid;
                const existingPublishedAt = existing.metadata_search?.publishedAt || now;
                const apiMetadataPayload = buildApiMetadataPayload(name, version, description, remotes, title, existingPublishedAt, now, proxyId, orgHandle);
                const [, updatedRows] = await apiDao.update(orgId, existing.uuid, apiMetadataPayload, userId, t);
                await labelDao.createApiMapping(orgId, existing.uuid, ['default'], userId, t);
                if (schemaBuffer) {
                    // Type-based upsert: an MCP server has exactly one schema row, so match by
                    // type (not filename) — this replaces any prior row regardless of its stored name.
                    await apiFileDao.upsert(
                        schemaBuffer, SCHEMA_FILE_NAME, existing.uuid, orgId,
                        constants.DOC_TYPES.SCHEMA_DEFINITION, userId, t
                    );
                }
                row = updatedRows[0];
            } else {
                const apiMetadataPayload = buildApiMetadataPayload(name, version, description, remotes, title, now, now, proxyId, orgHandle);
                const createdRow = await apiDao.create(orgId, apiMetadataPayload, userId, t);
                const apiId = createdRow.uuid;
                await labelDao.createApiMapping(orgId, apiId, ['default'], userId, t);
                const newSchemaBuffer = schemaBuffer || Buffer.from(yaml.dump([]), 'utf-8');
                await apiFileDao.store(newSchemaBuffer, SCHEMA_FILE_NAME, apiId, constants.DOC_TYPES.SCHEMA_DEFINITION, userId, t);
                row = createdRow;
                created = true;
            }
        });

        logger.info('MCP server published', { name, version, orgHandle });
        let schema;
        if (choreoMeta) {
            schema = { tools, resources, prompts };
        } else if (existingApiId) {
            const schemaContent = await apiFileDao.getDoc(constants.DOC_TYPES.SCHEMA_DEFINITION, orgId, existingApiId, null);
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
        const userId = util.resolveActor(req);

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
            ? Buffer.from(yaml.dump(toFlatSchema(tools, resources, prompts)), 'utf-8')
            : null;

        let row;
        let updatedApiId = null;
        await db.withTransaction(async (t) => {
            const existing = await findRowByServerIdentifier(orgId, serverIdentifier, version, t);
            if (!existing) return;

            updatedApiId = existing.uuid;
            const existingPublishedAt = existing.metadata_search?.publishedAt || new Date().toISOString();
            const existingProxyId = proxyId || existing.metadata_search?.proxyId || null;
            const apiMetadataPayload = buildApiMetadataPayload(existing.name, version, description, remotes, title, existingPublishedAt, new Date().toISOString(), existingProxyId, orgHandle);
            const [, updatedRows] = await apiDao.update(orgId, existing.uuid, apiMetadataPayload, userId, t);
            await labelDao.createApiMapping(orgId, existing.uuid, ['default'], userId, t);
            if (schemaBuffer) {
                // Type-based upsert: an MCP server has exactly one schema row, so match by
                // type (not filename) — this replaces any prior row regardless of its stored name.
                await apiFileDao.upsert(
                    schemaBuffer, SCHEMA_FILE_NAME, existing.uuid, orgId,
                    constants.DOC_TYPES.SCHEMA_DEFINITION, userId, t
                );
            }
            row = updatedRows[0];
        });

        if (!row) {
            return sendError(res, 404, 'Server version not found');
        }
        let schema;
        if (choreoMeta) {
            schema = { tools, resources, prompts };
        } else {
            const schemaContent = await apiFileDao.getDoc(constants.DOC_TYPES.SCHEMA_DEFINITION, orgId, updatedApiId, null);
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

        await db.execute(
            `UPDATE ${API_METADATA_TABLE} SET status = ?, updated_by = ?, updated_at = ? WHERE uuid = ? AND org_uuid = ?`,
            ['DELETED', util.resolveActor(req), new Date(), existing.uuid, orgId]
        );
        const deleted = await db.queryOne(`SELECT * FROM ${API_METADATA_TABLE} WHERE uuid = ?`, [existing.uuid]);
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
        if (existing.status === dbStatus) {
            return sendError(res, 400, `No changes to apply: status is already ${status}`);
        }

        await db.execute(
            `UPDATE ${API_METADATA_TABLE} SET status = ?, updated_by = ?, updated_at = ? WHERE uuid = ? AND org_uuid = ?`,
            [dbStatus, util.resolveActor(req), new Date(), existing.uuid, orgId]
        );
        const updated = await db.queryOne(`SELECT * FROM ${API_METADATA_TABLE} WHERE uuid = ?`, [existing.uuid]);
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

        await db.withTransaction(async (t) => {
            // FOR UPDATE row-locks every candidate row for the duration of this transaction,
            // so a concurrent updateAllVersionsStatus/publishServer call on the same server
            // can't race this bulk status change (postgres-only, per this file's own note above).
            let existing = await t.query(
                `SELECT * FROM ${API_METADATA_TABLE}
                 WHERE org_uuid = ? AND type = ? AND ref_id IS NULL AND ${PROXY_ID_EXPR} = ? FOR UPDATE`,
                [orgId, constants.API_TYPE.MCP, serverIdentifier]
            );
            if (existing.length === 0) {
                existing = await t.query(
                    `SELECT * FROM ${API_METADATA_TABLE}
                     WHERE org_uuid = ? AND type = ? AND ref_id IS NULL AND name = ? FOR UPDATE`,
                    [orgId, constants.API_TYPE.MCP, serverIdentifier]
                );
            }
            if (existing.length === 0) return;

            const ids = existing.map(r => r.uuid);
            const idPlaceholders = ids.map(() => '?').join(', ');
            await t.execute(
                `UPDATE ${API_METADATA_TABLE} SET status = ?, updated_by = ?, updated_at = ?
                 WHERE uuid IN (${idPlaceholders}) AND org_uuid = ?`,
                [dbStatus, util.resolveActor(req), new Date(), ...ids, orgId]
            );
            updated = await t.query(
                `SELECT * FROM ${API_METADATA_TABLE} WHERE uuid IN (${idPlaceholders})`,
                ids
            );
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
