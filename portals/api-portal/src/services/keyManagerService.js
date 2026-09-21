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
const db = require('../db/driver');
const { NotFoundError } = require('../utils/errors/customErrors');
const kmDao = require('../dao/keyManagerDao');
const kmRegistry = require('./keyManagerRegistry');
const kmConfigDao = require('../dao/keyManagerConfigurationDao');
const oauth2KeyDao = require('../dao/oauth2ConsumerKeyDao');
const { registeredTypes } = require('../keymanagers/core/registry');
const { assertDialable } = require('../keymanagers/core/httpClient');
const { DB_CLIENT_POLICY } = require('./keyManagerDriverBuilder');
const { KeyManagerDTO, KeyManagerPublicDTO } = require('../dto/keyManagerDto');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const constants = require('../utils/constants');
const util = require('../utils/util');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const crypto = require('crypto');

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

// Handles are used to build route segments, so a caller-supplied id must be restricted
// to a safe character set. Generated handles are UUIDs, which already satisfy this.
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

/*
 * A key manager declared in `[[api_portal.key_manager]]` is readable through
 * this API but not writable through it: its definition lives in the deployed
 * configuration, and the only way to change or remove it is to change that file.
 *
 * The refusal is 409 rather than 404 or 403. 404 would contradict the entry the
 * caller can see in GET /key-managers and read at GET /key-managers/{kmId}, and
 * would read as a bug. 403 would suggest a different caller could do it, which
 * no caller can. 409 says what is true: the resource exists, and its current
 * state is incompatible with the request.
 */
const CONFIG_DECLARED_MESSAGE =
    'This key manager is declared in the portal configuration and cannot be modified through the API. '
    + 'Change the deployed configuration instead.';

/*
 * Provisioning: the configuration that lets the portal register OAuth
 * applications on a key manager itself, rather than only proxying token requests
 * for one created elsewhere.
 *
 * Validated here, before anything is written, so a rejected payload leaves no
 * half-made key manager behind. Everything checkable without I/O is checked:
 * the driver type against what this build actually ships, and every endpoint
 * against the same address guard the outbound client will apply at dial time —
 * catching a private or plaintext endpoint at the point someone types it,
 * instead of on the first failed key generation.
 */
const PROVISIONING_AUTH_METHODS = ['client_credentials', 'basic', 'api_key'];

const TYPE_IMMUTABLE_ADD_MESSAGE =
    'This key manager was created without key generation, so its applications are created in the key '
    + 'manager and imported here. That cannot be changed after creation, because keys already imported '
    + 'against it are not the portal\'s to manage. Add a new key manager instead.';

const TYPE_IMMUTABLE_CHANGE_MESSAGE =
    'A key manager\'s type cannot be changed after it is created: clients already registered through it '
    + 'exist at the original key manager and a different driver cannot manage them. Add a new key manager instead.';


function _validateProvisioning(provisioning, { isUpdate = false, existing = null } = {}) {
    const type = typeof provisioning.type === 'string' ? provisioning.type.trim() : '';
    const types = registeredTypes();
    if (!types.includes(type)) {
        return { error: `Unknown key manager type "${type}". This build provides: ${types.join(', ')}.` };
    }

    const auth = provisioning.auth || {};
    if (!PROVISIONING_AUTH_METHODS.includes(auth.method)) {
        return { error: `auth.method must be one of: ${PROVISIONING_AUTH_METHODS.join(', ')}.` };
    }

    const endpoints = [
        ['provisioning.registrationEndpoint', provisioning.registrationEndpoint, true],
        ['provisioning.authorizeEndpoint', provisioning.authorizeEndpoint, false],
    ];
    for (const [field, value, required] of endpoints) {
        if (!value) {
            if (required) return { error: `${field} is required.` };
            continue;
        }
        try {
            // The same policy the driver will be built with, so what passes here
            // is exactly what will be dialable later — not a looser pre-check.
            assertDialable(value, field, DB_CLIENT_POLICY);
        } catch (err) {
            return { error: err.message };
        }
    }

    // A credential is required on create and optional on update, where omitting
    // it means "keep the stored one". Enforced against what is actually held, so
    // an update that switches method has to supply the new method's credential.
    const held = existing || {};
    const missing = [];
    if (auth.method === 'client_credentials') {
        if (!auth.clientId) missing.push('auth.clientId');
        const haveSecret = auth.clientSecret
            || (isUpdate && held.authMethod === 'client_credentials' && held.hasClientSecret);
        if (!haveSecret) missing.push('auth.clientSecret');
    } else if (auth.method === 'api_key') {
        const haveKey = auth.apiKey
            || (isUpdate && held.authMethod === 'api_key' && held.hasApiKey);
        if (!haveKey) missing.push('auth.apiKey');
        // Checked here for a message that names the field, and again in the
        // ApiKey constructor, which is what the config path goes through too.
        const headerError = _validateApiKeyHeader(auth.headerName);
        if (headerError) return { error: headerError };
    } else {
        if (!auth.username) missing.push('auth.username');
        const havePassword = auth.password
            || (isUpdate && held.authMethod === 'basic' && held.hasPassword);
        if (!havePassword) missing.push('auth.password');
    }
    if (missing.length) {
        return { error: `Missing required field(s) for auth.method "${auth.method}": ${missing.join(', ')}.` };
    }

    if (!kmConfigDao.encryptionAvailable()) {
        return {
            error: 'This portal cannot store key manager credentials: its encryption key is not configured. '
                + 'Set security.encryption_key and restart.',
        };
    }

    return {
        cfg: {
            type,
            registrationEndpoint: provisioning.registrationEndpoint,
            authorizeEndpoint: provisioning.authorizeEndpoint || '',
            authMethod: auth.method,
            authClientId: auth.clientId || '',
            authClientSecret: auth.clientSecret || '',
            authUsername: auth.username || '',
            authPassword: auth.password || '',
            authScopes: Array.isArray(auth.scopes) ? auth.scopes.filter(Boolean) : [],
            authResource: auth.resource || '',
            authHeaderName: auth.headerName || '',
            authScheme: auth.scheme || '',
            authApiKey: auth.apiKey || '',
        },
    };
}

/*
 * The header the API key travels in. Left blank it defaults to Authorization,
 * which is where most key managers want it.
 *
 * Rejected here rather than allowed through to a failed request: a header name
 * with a space or a colon in it produces an outbound request that is malformed
 * rather than unauthorised, and the resulting error would point at the key
 * manager instead of at the typo.
 */
const HEADER_NAME_TOKEN = /^[A-Za-z0-9!#$%&'*+.^_`|~-]+$/;
const RESERVED_HEADERS = new Set(['host', 'content-length', 'transfer-encoding', 'connection']);

function _validateApiKeyHeader(headerName) {
    if (!headerName) return null;
    if (typeof headerName !== 'string' || !HEADER_NAME_TOKEN.test(headerName)) {
        return 'auth.headerName must be a single HTTP header name, such as "Authorization" or "X-API-Key".';
    }
    if (RESERVED_HEADERS.has(headerName.toLowerCase())) {
        return `auth.headerName cannot be "${headerName}" — that header controls how the request is routed, not who is making it.`;
    }
    return null;
}

/**
 * The key manager's own token endpoint is dialled too — ClientCredentials mints
 * the provisioning token from it — so it is held to the same guard as the
 * endpoints inside `provisioning`. Only checked when provisioning is present: a
 * key manager that merely proxies token requests never has the portal connect
 * to it, so the stricter rule would be gratuitous there.
 */
function _validateTokenEndpointForProvisioning(tokenEndpoint) {
    try {
        assertDialable(tokenEndpoint, 'tokenEndpoint', DB_CLIENT_POLICY);
        return null;
    } catch (err) {
        return err.message;
    }
}

/**
 * Tag one registry entry with its provisioning state for the DTO.
 *
 * A config-declared key manager always can generate keys — its driver config is
 * in the deployed TOML — and has no row in `key_manager_configurations` to show.
 */
function _withProvisioning(entry, configs) {
    if (entry.source === kmRegistry.SOURCE_CONFIG) {
        return { ...entry, canGenerateKeys: true };
    }
    const cfg = configs.get(entry.uuid);
    return { ...entry, canGenerateKeys: Boolean(cfg), provisioning: cfg || undefined };
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

        // Handle rule: use the caller-supplied handle (a body `id` or YAML metadata.name)
        // when present; otherwise generate a UUID. The settings UI sends none, so those
        // get a UUID. A handle collision is always a 409 — we never rewrite the caller's
        // id or invent a variant.
        // An explicit handle (body `id` or YAML metadata.name) must be a string; reject
        // other types with a 400 rather than crashing on .trim() — the YAML-upload path
        // isn't schema-validated, so a numeric metadata.name can reach here.
        if (payload.handle != null && typeof payload.handle !== 'string') {
            return util.sendError(res, 400, "Invalid 'id'. Must contain only letters, numbers, underscores, and hyphens.");
        }
        const hadExplicitHandle = typeof payload.handle === 'string' && !!payload.handle.trim();
        if (hadExplicitHandle && !HANDLE_PATTERN.test(payload.handle.trim())) {
            return util.sendError(res, 400, "Invalid 'id'. Must contain only letters, numbers, underscores, and hyphens.");
        }
        const handle = hadExplicitHandle ? payload.handle.trim() : crypto.randomUUID();

        // Handles from both sources share one namespace, and configuration wins the
        // tie-break, so a row created under a config-declared handle would be
        // permanently invisible — written, but shadowed on every read. Refuse it
        // here instead. A generated handle is a fresh UUID and cannot collide.
        if (hadExplicitHandle && await kmRegistry.isConfigDeclared(handle)) {
            return util.sendError(res, 409, `A key manager with that id already exists in this organization.`);
        }

        // Validated before any write: a rejected provisioning payload must not leave
        // a key manager behind that the caller then has to clean up by hand.
        let provisioningCfg = null;
        if (payload.provisioning) {
            const endpointError = _validateTokenEndpointForProvisioning(payload.tokenEndpoint.trim());
            if (endpointError) return util.sendError(res, 400, endpointError);
            const checked = _validateProvisioning(payload.provisioning);
            if (checked.error) return util.sendError(res, 400, checked.error);
            provisioningCfg = checked.cfg;
        }

        const userId = util.resolveActor(req);
        try {
            const record = await kmDao.create(orgId, { ...payload, handle }, userId);
            if (provisioningCfg) {
                try {
                    await kmConfigDao.create({
                        orgId, keyManagerUuid: record.uuid, cfg: provisioningCfg, createdBy: userId,
                    });
                } catch (cfgError) {
                    // Two writes, no transaction spanning them. Rather than leave a key
                    // manager that silently cannot do what the caller asked for, undo
                    // the first one and report the failure.
                    logger.error('Failed to store key manager provisioning; rolling back the key manager', {
                        error: cfgError.message, kmId: record.uuid,
                    });
                    try {
                        await kmDao.delete(orgId, record.uuid);
                    } catch (rollbackError) {
                        logger.error('Rollback of the key manager also failed; it exists without provisioning', {
                            error: rollbackError.message, kmId: record.uuid,
                        });
                    }
                    throw cfgError;
                }
            }
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
            const dto = new KeyManagerDTO({
                ...record,
                source: kmRegistry.SOURCE_API,
                canGenerateKeys: Boolean(provisioningCfg),
                provisioning: provisioningCfg ? await kmConfigDao.get(orgId, record.uuid) : undefined,
            }, audit);
            return res.status(201).json(dto);
        } catch (error) {
            if (db.isDuplicateKeyError(error)) {
                return util.sendError(res, 409, `A key manager with that id already exists in this organization.`);
            }
            throw error;
        }
    } catch (error) {
        if (db.isDuplicateKeyError(error)) {
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

        if (await kmRegistry.isConfigDeclared(kmHandle)) {
            return util.sendError(res, 409, CONFIG_DECLARED_MESSAGE);
        }
        // The same shadowing problem as on create, reached by renaming instead:
        // a stored row moved onto a config-declared handle disappears behind the
        // config entry on every subsequent read.
        if (typeof payload?.handle === 'string' && await kmRegistry.isConfigDeclared(payload.handle.trim())) {
            return util.sendError(res, 409, `A key manager with that id already exists in this organization.`);
        }

        const kmId = await kmDao.getIdByHandle(orgId, kmHandle);
        if (!kmId) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }

        /*
         * A key manager's type is fixed when it is created, and this is where
         * that is enforced.
         *
         * Two moves are refused. Giving a provision-type key manager a
         * provisioning block would turn it into one that registers clients — and
         * every key already recorded against it was created by hand at the
         * identity server, so the next delete would destroy a client the portal
         * never owned. Changing one driver type for another is the same fault
         * with a different shape: the new driver would address existing clients
         * at a server that never issued them.
         *
         * Neither is a malformed request, so neither is a 400. The resource
         * exists and its current state is incompatible with what was asked.
         */
        let provisioningCfg = null;
        const existingCfg = await kmConfigDao.get(orgId, kmId);
        if (payload.provisioning && !existingCfg) {
            return util.sendError(res, 409, TYPE_IMMUTABLE_ADD_MESSAGE);
        }
        if (payload.provisioning && existingCfg
            && typeof payload.provisioning.type === 'string'
            && payload.provisioning.type.trim() !== existingCfg.type) {
            return util.sendError(res, 409, TYPE_IMMUTABLE_CHANGE_MESSAGE);
        }
        if (payload.provisioning) {
            const endpoint = (payload.tokenEndpoint || '').trim() || (await kmDao.get(orgId, kmId)).token_endpoint;
            const endpointError = _validateTokenEndpointForProvisioning(endpoint);
            if (endpointError) return util.sendError(res, 400, endpointError);
            const checked = _validateProvisioning(payload.provisioning, {
                isUpdate: true, existing: existingCfg,
            });
            if (checked.error) return util.sendError(res, 400, checked.error);
            provisioningCfg = checked.cfg;
        }

        const userId = util.resolveActor(req);
        const [, updatedRows] = await kmDao.update(orgId, kmId, payload, userId);
        if (provisioningCfg) {
            if (existingCfg) {
                await kmConfigDao.update({ orgId, keyManagerUuid: kmId, cfg: provisioningCfg, updatedBy: userId });
            } else {
                await kmConfigDao.create({
                    orgId, keyManagerUuid: kmId, cfg: provisioningCfg, createdBy: userId,
                });
            }
        }
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
        const storedCfg = provisioningCfg || existingCfg
            ? await kmConfigDao.get(orgId, kmId)
            : null;
        const dto = new KeyManagerDTO({
            ...updatedRows[0],
            source: kmRegistry.SOURCE_API,
            canGenerateKeys: Boolean(storedCfg),
            provisioning: storedCfg || undefined,
        }, audit);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof NotFoundError) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }
        if (db.isDuplicateKeyError(error)) {
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
        // Config-declared entries appear alongside the stored ones. Without this a
        // developer choosing where to generate a key would be shown only the rows
        // in `key_managers` — never the config entries, which are precisely the
        // ones POST /oauth2-keys can actually register a client on.
        const records = await kmRegistry.list(orgId, { includeDisabled: isAdmin });

        // Audit fields are resolved only for stored rows. A config entry has no
        // created_by, and passing it to the audit builder would report its author
        // as `deleted_user` — a fabricated answer to a question that has none.
        const stored = records.filter((r) => r.source !== kmRegistry.SOURCE_CONFIG);
        const auditByHandle = new Map();
        if (isAdmin && stored.length) {
            const auditList = await userIdpReferenceDao.buildListAuditFields(stored);
            stored.forEach((r, i) => auditByHandle.set(r.handle, auditList[i]));
        }

        // One query for the whole page rather than a lookup per row. Admin-only:
        // the public view carries no provisioning detail, so fetching it for a
        // developer would be work whose result is thrown away.
        let configs = new Map();
        if (isAdmin && stored.length) {
            try {
                configs = await kmConfigDao.listByOrg(orgId);
            } catch (cfgError) {
                logger.warn('Could not load key manager provisioning for the listing', {
                    error: cfgError.message, orgId,
                });
            }
        }

        const dtos = records.map((r) => (isAdmin
            ? new KeyManagerDTO(_withProvisioning(r, configs), auditByHandle.get(r.handle))
            : new KeyManagerPublicDTO(r)));
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
        const record = await kmRegistry.findByHandle(orgId, kmHandle);
        if (!record) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }
        // As in the list: no audit fields for a config entry, rather than
        // attributing it to a user who never created it.
        const audit = record.source === kmRegistry.SOURCE_CONFIG
            ? undefined
            : await userIdpReferenceDao.buildSingleAuditFields(record);
        let provisioning;
        if (record.source === kmRegistry.SOURCE_API) {
            try {
                provisioning = await kmConfigDao.get(orgId, record.uuid);
            } catch (cfgError) {
                logger.warn('Could not load key manager provisioning', {
                    error: cfgError.message, kmId: record.uuid,
                });
            }
        }
        const dto = new KeyManagerDTO({
            ...record,
            canGenerateKeys: record.source === kmRegistry.SOURCE_CONFIG || Boolean(provisioning),
            provisioning: provisioning || undefined,
        }, audit);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof NotFoundError) {
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
        if (await kmRegistry.isConfigDeclared(kmHandle)) {
            return util.sendError(res, 409, CONFIG_DECLARED_MESSAGE);
        }
        const kmId = await kmDao.getIdByHandle(orgId, kmHandle);
        if (!kmId) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }

        /*
         * Refuse while keys still reference this key manager.
         *
         * `oauth2_consumer_keys.key_manager_id` holds the handle and carries no
         * foreign key, so deleting the row here would not cascade — it would
         * leave keys pointing at a handle that resolves to nothing, failing every
         * later operation with no hint as to why.
         *
         * It also closes the way around type immutability: delete a
         * provision-type key manager, recreate it with the same `id` and a
         * provisioning block, and its orphaned keys would silently acquire a
         * driver that issues real deletes against clients the portal never made.
         * Requiring the keys to go first makes that sequence deliberate.
         */
        const keyCount = await oauth2KeyDao.countByKeyManager(orgId, kmHandle);
        if (keyCount > 0) {
            return util.sendError(res, 409,
                `This key manager still has ${keyCount} key${keyCount === 1 ? '' : 's'}. `
                + 'Delete them first — removing the key manager would leave them unusable.');
        }

        await kmDao.delete(orgId, kmId);
        logUserAction('KEY_MANAGER_DELETED', req, { orgId, kmId, resourceUuid: kmId, resourceType: 'key_manager' });
        return res.status(204).send();
    } catch (error) {
        if (error instanceof NotFoundError) {
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
