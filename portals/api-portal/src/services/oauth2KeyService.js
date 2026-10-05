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

/*
 * OAuth2 keys — Dynamic Client Registration (RFC 7591/7592) against a
 * configured key manager. Tag: "OAuth2 Keys", plus getKeyManagerMetadata under
 * "Key Managers".
 *
 * The key managers come from [[api_portal.key_manager]] config, resolved through
 * src/keymanagers (built-in drivers, config-activated). Registration is a real
 * HTTP call: the credentials returned here are issued by the key manager.
 *
 * PERSISTENCE
 * ---------------------------------------------------------------------------
 * The portal's record of each key lives in `oauth2_consumer_keys`, via
 * dao/oauth2ConsumerKeyDao.js. It holds the client's identity only. Not stored,
 * and the consequence of each:
 *
 *   - The client secret. The key manager returns it once, so the create
 *     response is the only place it appears.
 *   - The client metadata. Re-read from the key manager on GET, so there is one
 *     copy; a driver with no read reports an empty set rather than stale data.
 *
 * The RFC 7592 credentials ARE stored, and this is the one place to look for
 * where that bearer token lives: `registration_access_token_enc` (encrypted at
 * rest with security.encryption_key) and `registration_client_uri`, both written
 * by `oauth2ConsumerKeyDao.create` and rewritten by `setRegistration` whenever a
 * response carries a rotated pair. Neither rides along on an ordinary read —
 * they sit outside COLUMNS, and `getWithRegistration` is the only path that opts
 * in — so a list or detail response never carries a credential it has no use for.
 *
 * `_withRegistration` is what read/update/delete go through: it presents the
 * stored token when there is one, and falls back to the portal's PROVISIONING
 * credential when there is not — which is every key on a key manager that issues
 * no token, and any key whose stored token the server has since rejected
 * (`provisioning_credential_rejected`, after which the stale pair is cleared).
 * The configuration URI is stored rather than reconstructed because RFC 7592 §3
 * lets a server put it on a different host or path than registration, as
 * Keycloak does.
 *
 * The key↔application association lives in `oauth2_consumer_key_app_mappings`,
 * reached through `keyAppMappingDao`. A key belongs to at most one application;
 * an application may hold any number of keys.
 */

const crypto = require('crypto');

const applicationDao = require('../dao/applicationDao');
const oauth2KeyDao = require('../dao/oauth2ConsumerKeyDao');
const constants = require('../utils/constants');
const util = require('../utils/util');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const { getFactory } = require('../keymanagers');
const kmConfigDao = require('../dao/keyManagerConfigurationDao');
const kmRegistry = require('./keyManagerRegistry');
const keyAppMappingDao = require('../dao/oauth2KeyAppMappingDao');
const { KeyManagerCallError } = require('../keymanagers/core/keyManager');

// ---------------------------------------------------------------------------
// Response shaping
// ---------------------------------------------------------------------------

/*
 * `registration` (the RFC 7592 client URI and access token) is deliberately
 * absent from both DTOs below. The access token is a bearer credential for
 * managing that client at the key manager; it is internal state, never part of
 * an API response.
 */

function _toDetailDto(record, km, { secret = '', properties = {}, application } = {}) {
    const dto = {
        keyId: record.keyId,
        // Stored at creation rather than read back, because the key manager is not
        // asked for it on every list — and for a provision key manager there is
        // nothing to ask.
        name: record.name || '',
        // Stamped on the key when it was created, from its key manager. Read from
        // the record, never from the key manager now: the key manager's own flag is
        // frozen, but reading it live would still be answering a different question
        // — what this credential IS, versus what that key manager issues today.
        keyType: record.keyType || constants.KEY_TYPE.PRODUCTION,
        keyManagerId: record.keyManagerId,
        // Display name comes from the live config entry, not a stored copy, so
        // renaming a key manager does not leave stale names on old keys. A key
        // manager since removed from config falls back to its id.
        keyManagerName: km ? km.displayName : record.keyManagerId,
        consumerKey: record.consumerKey,
        // Only ever non-empty on the create response — the key manager does not
        // hand a secret back afterwards, so nothing is stored to return here.
        consumerSecret: secret,
        // Read from the key manager, not from a local copy. `{}` when its driver
        // implements no read.
        properties,
        createdBy: record.createdBy,
        updatedBy: record.updatedBy,
        createdAt: record.createdAt,
        updatedAt: record.updatedAt,
    };
    // Optional in the schema: absent means "attached to no application", which is
    // a real state, not missing data. Assigned conditionally so it is omitted
    // rather than sent as null.
    if (application) dto.application = application;
    return dto;
}

function _toListItemDto(record, km, application) {
    const dto = {
        keyId: record.keyId,
        name: record.name || '',
        keyType: record.keyType || constants.KEY_TYPE.PRODUCTION,
        keyManagerId: record.keyManagerId,
        keyManagerName: km ? km.displayName : record.keyManagerId,
        consumerKey: record.consumerKey,
        consumerSecret: '',
        createdBy: record.createdBy,
        createdAt: record.createdAt,
        updatedAt: record.updatedAt,
    };
    if (application) dto.application = application;
    return dto;
}

/**
 * handle → display name for every key manager in the org, resolved once.
 *
 * A display name does not justify constructing a driver per row, and a key issued
 * on an API-created key manager would otherwise fall back to showing its raw
 * handle. A failure here is not fatal: _toListItemDto already falls back to the
 * handle for an unknown key manager, and the keys are what the caller asked for.
 */
async function _keyManagerNames(orgId) {
    const names = new Map();
    try {
        for (const entry of await kmRegistry.list(orgId, { includeDisabled: true })) {
            names.set(entry.handle, entry.display_name || entry.handle);
        }
    } catch (error) {
        logger.warn('Could not resolve key manager names', { error: error.message, orgId });
    }
    return names;
}

/**
 * The application a key is attached to, in the shape the spec's
 * `OAuth2KeyApplication` expects — or undefined when it is attached to none,
 * which the schema models as the field being absent rather than null.
 */
function _toApplicationDto(app) {
    if (!app) return undefined;
    return { id: app.handle, displayName: app.display_name };
}

// ---------------------------------------------------------------------------
// Key manager error mapping
// ---------------------------------------------------------------------------

/*
 * A driver classifies a failed call into a coarse reason; this maps that to a
 * status and a sterile message.
 *
 * Two rules, both from error-handling.md:
 *
 * 1. Nothing from the key manager's own response reaches the client — not its
 *    URL, not its status line, not its body (directive 1). The full detail goes
 *    to the internal log under a tracking id, which is what the client gets to
 *    correlate with.
 *
 * 2. Every server-side cause collapses to ONE identical message (directive 4's
 *    unified-response principle, applied to the 5xx class). A caller must not be
 *    able to tell an unreachable key manager from a rejected provisioning
 *    credential from a rate limit: those describe the portal's own configuration
 *    and dependency state, the caller cannot act on any of them, and
 *    distinguishing them hands an outsider a probe into the deployment. The
 *    operator gets the distinction from `reason` in the log, not from the
 *    response.
 *
 * Only reasons the CALLER caused or can act on get a specific message, and those
 * are the 4xx ones — their specificity is about the request, not about us.
 */
const REASON_TO_STATUS = Object.freeze({
    // Caller-caused: their client metadata, their key, their request.
    rejected: 400,
    client_not_found: 404,
    conflict: 409,
    // The developer's own consumer secret or requested scope was refused. Theirs
    // to fix, so it is a 4xx and says what happened.
    token_request_rejected: 400,
    // The driver has no implementation for this operation — this key manager
    // simply does not offer it. Caller-facing: the request named an operation
    // that is not available here.
    unsupported_operation: 409,
    // The client itself cannot do what was asked: no credential to present, or a
    // credential mechanism the portal cannot present for it. The request named a
    // real key; that key is simply not eligible. Nothing was dialled.
    public_client_no_credentials: 409,
    client_auth_method_unsupported: 409,
    // The key manager exists and the caller may see it; an admin has taken it out
    // of service. Caller-facing, and distinguishable from "no such key manager" on
    // purpose: a 404 would send someone looking for a deleted key manager that is
    // sitting right there in the list marked Disabled.
    key_manager_disabled: 409,
    // Server-side: the portal's configuration or its link to the key manager.
    // All of these answer identically — see rule 2 above.
    provisioning_credential_rejected: 500,
    rate_limited: 500,
    unreachable: 500,
    upstream_error: 500,
});

// Specific messages exist ONLY for the caller-caused reasons above. A reason
// absent here is answered with the operation's own generic message, so no
// server-side cause is distinguishable from any other.
const CALLER_FACING_MESSAGE = Object.freeze({
    rejected: 'The key manager rejected the supplied client metadata.',
    client_not_found: constants.ERROR_MESSAGE.OAUTH2_KEY_NOT_FOUND,
    conflict: 'The key manager reported a conflict with an existing client.',
    token_request_rejected: 'The key manager refused these credentials. Check the consumer secret, '
        + 'and that this client is allowed the scopes requested.',
    unsupported_operation: constants.ERROR_MESSAGE.OAUTH2_KEY_OPERATION_UNSUPPORTED,
    key_manager_disabled: 'This key manager is disabled. Existing keys can still be viewed and '
        + 'deleted, but no new keys, updates or tokens can be issued through it until an '
        + 'administrator enables it again.',
    public_client_no_credentials: 'This is a public client, so it has no secret and cannot get a token '
        + 'from here. Public clients get tokens by signing a user in, not with the client credentials '
        + 'grant.',
    client_auth_method_unsupported: 'This client authenticates with a private key, which the portal '
        + 'cannot present on your behalf. Request its token directly from the key manager.',
});

/**
 * Turn a driver failure into a response, logging the upstream detail internally.
 *
 * @param {object} res
 * @param {Error} error
 * @param {string} genericMessage  the operation's own message, used for every
 *                                 server-side cause so they are indistinguishable
 * @param {object} context         extra fields for the internal log
 * @returns {boolean} true when `error` was a key manager failure and handled
 */
function _sendKeyManagerError(res, error, genericMessage, context) {
    if (!(error instanceof KeyManagerCallError)) return false;

    const status = REASON_TO_STATUS[error.publicReason] || 500;
    const message = CALLER_FACING_MESSAGE[error.publicReason] || genericMessage;
    const trackingId = crypto.randomUUID();

    // The precise reason is logged, never returned. `detail` carries the upstream
    // URL, status and (bounded) body: internal only — it names an internal host
    // and echoes the key manager's own text.
    logger.error('Key manager call failed', {
        trackingId,
        reason: error.publicReason,
        upstreamStatus: error.upstreamStatus,
        detail: error.detail,
        ...context,
    });

    util.sendError(res, status, message, { trackingId });
    return true;
}

// ---------------------------------------------------------------------------
// Resolution helpers
// ---------------------------------------------------------------------------

/**
 * Resolve a usable key manager by id, or null.
 *
 * Null covers two cases the caller answers identically with 404: no key manager
 * has that id, and one does but has no driver behind it — an API-created record
 * whose configuration is missing, so it cannot register clients. Collapsing them
 * is deliberate. The submitted id is never echoed back either way, so a rejected
 * id cannot be used to probe which key managers exist.
 */
/**
 * @param {object} [opts]
 * @param {boolean} [opts.allowDisabled] act through a key manager an admin has
 *        disabled. Only for operations on a key that already exists — see the
 *        note in kmRegistry.resolveDriver.
 */
async function _resolveKeyManager(orgId, keyManagerId, opts) {
    return kmRegistry.resolveDriver(orgId, keyManagerId, opts);
}

// ---------------------------------------------------------------------------
// Operations
// ---------------------------------------------------------------------------

/**
 * Check a submitted property bag against what this key manager actually accepts.
 *
 * Until now nothing did: `properties` went straight from the request body into
 * `km.createKey()` and on to the identity server, so the declared metadata was a
 * description of the form rather than a contract. That is what makes an admin's
 * grant-type restriction meaningful — a dropdown narrowed in the browser is not a
 * restriction if the same request can be sent by hand.
 *
 * Three rules, all from the descriptors the driver emits:
 *   - a property not declared is rejected, rather than forwarded to the server
 *   - a declared `required` property must be present
 *   - a value for a property with `options` must be one of them
 *
 * `requiredWhen` is deliberately not enforced here. It depends on another
 * property's value, the drivers already express it, and getting the dependency
 * subtly wrong would reject valid requests — the key manager itself rejects a
 * registration missing a conditionally-required member, with a better message.
 *
 * @returns {string|null} an error message, or null when the bag is acceptable
 */
function _validateProperties(meta, properties) {
    const declared = (meta && meta.properties) || [];
    if (!declared.length) return null;
    const byName = new Map(declared.map((p) => [p.name, p]));
    const submitted = properties && typeof properties === 'object' && !Array.isArray(properties)
        ? properties
        : {};

    const unknown = Object.keys(submitted).filter((k) => !byName.has(k));
    if (unknown.length) {
        return `This key manager does not accept the propert${unknown.length === 1 ? 'y' : 'ies'}: `
            + `${unknown.join(', ')}.`;
    }

    const missing = declared
        .filter((p) => p.required)
        .filter((p) => {
            const v = submitted[p.name];
            if (v === undefined || v === null || v === '') return true;
            return Array.isArray(v) && v.length === 0;
        })
        .map((p) => p.name);
    if (missing.length) {
        return `Missing required propert${missing.length === 1 ? 'y' : 'ies'}: ${missing.join(', ')}.`;
    }

    for (const [name, value] of Object.entries(submitted)) {
        const descriptor = byName.get(name);
        if (!Array.isArray(descriptor.options) || !descriptor.options.length) continue;
        if (value === undefined || value === null || value === '') continue;
        const permitted = descriptor.options.map((o) => o.value);
        // A multiselect submits an array, a select a single value. Both are checked
        // against the same list, which is what the admin's restriction narrowed.
        const offered = Array.isArray(value) ? value : [value];
        const rejected = offered.filter((v) => !permitted.includes(v));
        if (rejected.length) {
            return `"${name}" does not accept: ${rejected.join(', ')}. `
                + `Permitted here: ${permitted.join(', ')}.`;
        }
    }
    return null;
}

/**
 * The metadata for one key manager with its own settings applied — the same view
 * GET /key-managers/metadata returns, which is what the property check above has
 * to be made against, or a restriction would be advertised and not enforced.
 */
async function _effectiveMetadata(orgId, km, keyManagerId) {
    const entry = await kmRegistry.findByHandle(orgId, keyManagerId);
    if (entry && entry.source === kmRegistry.SOURCE_API && entry.uuid) {
        const cfg = await kmConfigDao.get(orgId, entry.uuid);
        return _applyKeyManagerSettings(km.metadata(), cfg, entry);
    }
    return _applyKeyManagerSettings(km.metadata(), null, entry);
}

/**
 * Fold a stored key manager's own settings into the metadata its driver emits.
 *
 * Two things the driver cannot know about itself:
 *
 *   keyType           which environment this key manager issues keys for. A
 *                     developer choosing a key manager is choosing that, so it
 *                     has to be visible at the point of choice.
 *
 *   supportedGrantTypes  the admin's restriction on which grants may be picked.
 *                     Applied by narrowing the declared options, not by adding a
 *                     separate field, so a renderer needs to know nothing new —
 *                     it draws whatever options it is given.
 *
 * Narrowing only: a configured value that the driver no longer declares is
 * dropped rather than offered, so a restriction written against an older build
 * cannot reintroduce a grant this one does not support. If that leaves nothing,
 * the restriction is ignored and the driver's full set stands — an empty
 * dropdown would make the key manager unusable with no way to tell why.
 */
function _applyKeyManagerSettings(meta, cfg, entry) {
    const result = {
        ...meta,
        keyType: (entry && entry.key_type) || constants.KEY_TYPE.PRODUCTION,
    };
    const allowed = (cfg && cfg.supportedGrantTypes) || [];
    if (!allowed.length || !Array.isArray(result.properties)) return result;

    result.properties = result.properties.map((p) => {
        if (p.name !== 'grant_types' || !Array.isArray(p.options)) return p;
        const narrowed = p.options.filter((o) => allowed.includes(o.value));
        if (!narrowed.length) {
            logger.warn('Grant type restriction matched none of the driver\'s options; ignoring it', {
                keyManagerId: entry && entry.handle, allowed,
            });
            return p;
        }
        return { ...p, options: narrowed };
    });
    return result;
}

/**
 * GET /key-managers/metadata — tag "Key Managers", operationId getKeyManagerMetadata.
 *
 * The configured key managers and the properties each accepts. Drivers emit the
 * spec's field names directly, so this returns their metadata unchanged rather
 * than through a translation step that could drift from the schema.
 */
const getKeyManagerMetadata = async (req, res) => {
    try {
        const orgId = req.orgId;
        const factory = await getFactory();
        /*
         * Config-declared key managers. Their grant types are not restrictable —
         * the restriction lives in `key_manager_configurations`, which a config
         * entry has no row in — so only keyType is attached here. An operator who
         * wants a narrower set on a config entry edits the file.
         */
        const configKeyTypes = new Map(
            factory.all().map((km) => [km.id, km.keyType || constants.KEY_TYPE.PRODUCTION])
        );
        const metadata = factory.allMetadata().map((m) => ({
            ...m,
            keyType: configKeyTypes.get(m.id) || constants.KEY_TYPE.PRODUCTION,
        }));

        // Every API-created key manager appears here, whichever kind it is. The
        // endpoint answers "where can I get a key", and both kinds are an answer:
        // one registers a client for you, the other takes a client id you already
        // have. Each declares its own form through metadata(), so the difference
        // needs no flag here — a provision key manager simply asks for one field.
        const entries = await kmRegistry.list(orgId);
        for (const entry of entries) {
            if (entry.source !== kmRegistry.SOURCE_API) continue;
            let driver = null;
            try {
                driver = await kmRegistry.resolveDriver(orgId, entry.handle);
            } catch (entryError) {
                /*
                 * One key manager must not empty this list. Building a driver
                 * decrypts its stored credential, and that throws a plain Error
                 * — not a KeyManagerCallError — when the credential cannot be
                 * read: after `security.encryption_key` is rotated, or a database
                 * is restored under a portal holding a different key. Unhandled,
                 * that answers 500 here and the key generation form fails for
                 * every developer, including for the key managers that are fine.
                 *
                 * Skipped the same way an unshipped driver type already is, and
                 * logged so the cause is visible to an operator.
                 */
                logger.warn('Key manager left out of the metadata listing', {
                    orgId, keyManagerId: entry.handle, error: entryError.message,
                });
                continue;
            }
            // Null means "driver type this build does not ship" — left out rather
            // than listed as broken.
            if (!driver) continue;
            const cfg = await kmConfigDao.get(orgId, entry.uuid);
            metadata.push(_applyKeyManagerSettings(driver.metadata(), cfg, entry));
        }
        metadata.sort((a, b) => a.id.localeCompare(b.id));

        // No key manager configured is an empty list, not an error — the caller
        // renders "no key manager available" from a 200.
        return res.status(200).json(util.toPaginatedList(metadata, req));
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.KEY_MANAGER_RETRIEVE_ERROR);
    }
};

/**
 * Run one RFC 7592 lifecycle call, with the stored registration credentials if
 * this key has any, and recover from a stale one.
 *
 * Three things have to happen around every such call, and putting them here keeps
 * them from being remembered separately at four call sites:
 *
 * 1. USE the stored token. Absent, the driver falls back to the provisioning
 *    credential against a constructed URL — which is what every key on a key
 *    manager that issues no token does.
 *
 * 2. PERSIST a rotated one. RFC 7592 §5 lets a server issue a new token on any
 *    read or update. Keycloak does it on update and kills the old token at once,
 *    so a missed write makes the key unmanageable from the very next call.
 *    Best-effort: the operation the caller asked for has already succeeded, so a
 *    failed write must not fail the response — step 3 is what recovers.
 *
 * 3. RECOVER from a stale one. A token the server no longer accepts comes back as
 *    `provisioning_credential_rejected` (401/403). Rather than leaving the key
 *    permanently unmanageable, the stored pair is cleared and the call retried
 *    once with the provisioning credential — which a conforming server may well
 *    accept: Keycloak's configuration endpoint takes an admin bearer. Retried
 *    once only, and only on that reason: a 404 means the client is gone and
 *    retrying proves nothing.
 *
 * @param {object} ctx  { orgId, keyId, actor, record }
 * @param {(registration: object|null) => Promise<object>} call
 */
async function _withRegistration(ctx, call) {
    const { orgId, keyId, actor } = ctx;
    let stored = null;
    try {
        stored = await oauth2KeyDao.getWithRegistration(orgId, keyId, actor);
    } catch (error) {
        // Reading the credential must not break the operation; without it the
        // driver simply uses the provisioning credential.
        logger.warn('Could not read stored registration credentials', {
            orgId, keyId, error: error.message,
        });
    }
    const registration = stored && stored.registration && stored.registration.accessToken
        ? stored.registration
        : null;

    const persist = async (result) => {
        const issued = result && result.registration;
        if (!issued || !issued.accessToken) return result;
        const unchanged = registration
            && issued.accessToken === registration.accessToken
            && (issued.clientUri || '') === (registration.clientUri || '');
        if (unchanged) return result;
        try {
            await oauth2KeyDao.setRegistration(orgId, keyId, actor, issued, actor);
        } catch (error) {
            logger.error('Failed to persist a rotated registration access token; this key may '
                + 'need the provisioning-credential fallback on its next call', {
                orgId, keyId, error: error.message,
            });
        }
        return result;
    };

    try {
        return await persist(await call(registration));
    } catch (error) {
        const stale = registration
            && error instanceof KeyManagerCallError
            && error.publicReason === 'provisioning_credential_rejected';
        if (!stale) throw error;
        logger.warn('Stored registration access token was rejected; clearing it and retrying '
            + 'with the provisioning credential', { orgId, keyId });
        try {
            await oauth2KeyDao.setRegistration(orgId, keyId, actor, { accessToken: '', clientUri: '' }, actor);
        } catch (clearError) {
            logger.error('Could not clear a rejected registration access token', {
                orgId, keyId, error: clearError.message,
            });
        }
        return persist(await call(null));
    }
}

/**
 * POST /oauth2-keys — operationId createOAuth2Key.
 *
 * Registers a new OAuth application on the selected key manager over DCR.
 */
const createOAuth2Key = async (req, res) => {
    const orgId = req.orgId;
    const actor = util.resolveActor(req);
    const { keyManagerId, properties } = req.body;
    try {
        const km = await _resolveKeyManager(orgId, keyManagerId);
        if (!km) {
            // Generic 404 with no echo of the submitted id.
            return util.sendError(res, 404, constants.ERROR_MESSAGE.KEY_MANAGER_NOT_FOUND);
        }
        const propertyError = _validateProperties(
            await _effectiveMetadata(orgId, km, keyManagerId), properties
        );
        if (propertyError) return util.sendError(res, 400, propertyError);

        const issued = await km.createKey(properties);

        // Persist only what the portal needs to find this client again and to
        // manage it afterwards. The secret and the metadata are not stored — see
        // the note at the top. `issued.registration` is: where the key manager
        // implements RFC 7592, its token and configuration URI are what later
        // read/update/delete calls authenticate with.
        // `client_name` is RFC 7591's member for the display name, and every driver
        // here declares it under that name — so one lookup covers all of them rather
        // than a per-driver mapping. Stored because a consumer key is not something a
        // person can pick out of a list; it is not used for anything else.
        let record;
        try {
            record = await oauth2KeyDao.create({
                orgId,
                keyManagerId: issued.keyManagerId,
                consumerKey: issued.consumerKey,
                name: (issued.properties && issued.properties.client_name) || '',
                /*
                 * Inherited from the key manager, resolved here and nowhere else.
                 * When a per-key choice is added later this one expression becomes
                 * `req.body.keyType ?? km.keyType` and nothing else moves.
                 */
                keyType: km.keyType || constants.KEY_TYPE.PRODUCTION,
                createdBy: actor,
                registration: issued.registration,
            });
        } catch (error) {
            /*
             * The client is already live at the key manager and the row that was
             * going to record it did not get written, so nothing here can reach
             * it any more: it is absent from every list, and delete has no row to
             * start from. Left alone it is a credential the portal issued, cannot
             * see, and cannot revoke.
             *
             * So undo the half that did succeed. The registration credentials the
             * key manager just issued are still in hand — they were never stored,
             * which is the whole problem — and they are exactly what RFC 7592
             * delete authenticates with.
             *
             * If the compensating delete fails too, the client really is orphaned
             * and only an operator can clear it: log the consumer key (the handle
             * for it at the key manager, not a credential) so there is something
             * to act on. The caller gets the same 500 either way — from their side
             * the key was not created.
             */
            logger.error('Failed to record a newly registered OAuth client; rolling it back at '
                + 'the key manager', {
                orgId, keyManagerId: km.id, consumerKey: issued.consumerKey, error: error.message,
            });
            try {
                await km.deleteKey(issued.consumerKey, issued.registration || null);
            } catch (rollbackError) {
                logger.error('ORPHANED OAUTH CLIENT: registered at the key manager, not recorded '
                    + 'by the portal, and could not be rolled back. It must be deleted at the key '
                    + 'manager by hand.', {
                    orgId,
                    keyManagerId: km.id,
                    consumerKey: issued.consumerKey,
                    rollbackError: rollbackError.message,
                });
            }
            throw error;
        }

        logUserAction('OAUTH2_KEY_CREATED', req, {
            orgId,
            keyId: record.keyId,
            keyManagerId: km.id,
            resourceUuid: record.keyId,
            resourceType: 'oauth2_key',
        });

        res.setHeader('Location', `${req.baseUrl}${req.path}/${encodeURIComponent(record.keyId)}`);
        // The only response that carries the secret, straight from the DCR
        // response — it was never written down.
        return res.status(201).json(_toDetailDto(record, km, {
            secret: issued.consumerSecret,
            properties: issued.properties,
        }));
    } catch (error) {
        if (_sendKeyManagerError(res, error, constants.ERROR_MESSAGE.OAUTH2_KEY_CREATE_ERROR,
            { orgId, keyManagerId, operation: 'createKey' })) {
            return undefined;
        }
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_CREATE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_CREATE_ERROR);
    }
};

/**
 * GET /oauth2-keys — operationId listOAuth2Keys.
 */
const listOAuth2Keys = async (req, res) => {
    try {
        const orgId = req.orgId;
        const actor = util.resolveActor(req);
        const { keyManagerId } = req.query;

        // Ownership and the key-manager filter are both applied in SQL, so a
        // row belonging to another user is never loaded.
        const records = await oauth2KeyDao.listByCreator(orgId, actor, { keyManagerId });
        const names = await _keyManagerNames(orgId);
        // Associations for the whole page in one query, then the applications they
        // point at in a second — rather than two lookups per row.
        const appByKey = await keyAppMappingDao.getByKeys(records.map((r) => r.keyId));
        const apps = new Map();
        for (const appUuid of new Set(appByKey.values())) {
            const app = await applicationDao.get(orgId, appUuid, actor);
            if (app) apps.set(appUuid, _toApplicationDto(app));
        }
        const list = [];
        for (const record of records) {
            const name = names.get(record.keyManagerId);
            list.push(_toListItemDto(
                record,
                name ? { displayName: name } : null,
                apps.get(appByKey.get(record.keyId))
            ));
        }
        return res.status(200).json(util.toPaginatedList(list, req));
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_RETRIEVE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_RETRIEVE_ERROR);
    }
};

/**
 * GET /oauth2-keys/{keyId} — operationId getOAuth2Key.
 *
 * The portal's own record is authoritative for the envelope; the property set is
 * re-read from the key manager when its driver supports the RFC 7592 read, so a
 * client changed in the key manager's own console is reflected here.
 */
const getOAuth2Key = async (req, res) => {
    const orgId = req.orgId;
    const { keyId } = req.params;
    try {
        const actor = util.resolveActor(req);
        const record = await oauth2KeyDao.get(orgId, keyId, actor);
        if (!record) {
            // A key owned by someone else is answered exactly as a missing one.
            return util.sendError(res, 404, constants.ERROR_MESSAGE.OAUTH2_KEY_NOT_FOUND);
        }

        // A key already issued stays readable even if its key manager was disabled;
            // the list shows it, so opening it must not 409.
        const km = await _resolveKeyManager(orgId, record.keyManagerId, { allowDisabled: true });
        // The metadata lives at the key manager, so it is read from there. Two
        // cases yield an empty set instead of failing: a key manager removed
        // from config since the key was issued, and one whose driver implements
        // no read. Any other failure is real and propagates.
        let properties = {};
        if (km) {
            try {
                const fresh = await _withRegistration(
                    { orgId, keyId, actor },
                    (registration) => km.getKey(record.consumerKey, registration)
                );
                properties = fresh.properties;
            } catch (err) {
                if (!(err instanceof KeyManagerCallError) ||
                    err.publicReason !== 'unsupported_operation') {
                    throw err;
                }
            }
        }
        const appUuid = await keyAppMappingDao.getByKey(keyId);
        const application = appUuid
            ? _toApplicationDto(await applicationDao.get(orgId, appUuid, actor))
            : undefined;
        return res.status(200).json(_toDetailDto(record, km, { properties, application }));
    } catch (error) {
        if (_sendKeyManagerError(res, error, constants.ERROR_MESSAGE.OAUTH2_KEY_RETRIEVE_ERROR,
            { orgId, keyId, operation: 'getKey' })) {
            return undefined;
        }
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_RETRIEVE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_RETRIEVE_ERROR);
    }
};

/**
 * PUT /oauth2-keys/{keyId} — operationId updateOAuth2Key.
 *
 * Full replacement of the client metadata, per RFC 7592.
 */
const updateOAuth2Key = async (req, res) => {
    const orgId = req.orgId;
    const { keyId } = req.params;
    try {
        const actor = util.resolveActor(req);
        const record = await oauth2KeyDao.get(orgId, keyId, actor);
        if (!record) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.OAUTH2_KEY_NOT_FOUND);
        }
        const km = await _resolveKeyManager(orgId, record.keyManagerId);
        if (!km) {
            // The key manager is gone from config, so its client cannot be
            // reached — the same answer as a driver that cannot update.
            return util.sendError(res, 409, constants.ERROR_MESSAGE.OAUTH2_KEY_OPERATION_UNSUPPORTED);
        }

        // The same contract as create. PUT replaces the client metadata wholesale,
        // so an update is exactly as able to set a restricted grant type as a
        // create is — checking only one of the two would leave the restriction
        // trivially reachable by creating a key and then editing it.
        const propertyError = _validateProperties(
            await _effectiveMetadata(orgId, km, record.keyManagerId), req.body.properties
        );
        if (propertyError) return util.sendError(res, 400, propertyError);

        // Upstream first: the key manager is the system of record for the
        // metadata, so a failed call must leave nothing changed here either.
        const updated = await _withRegistration(
            { orgId, keyId, actor },
            (registration) => km.updateKey(record.consumerKey, req.body.properties, registration)
        );

        /*
         * The name is the one thing persisted here, so it is the one thing an
         * update has to write back — otherwise the list keeps showing the old
         * name for a client that has been renamed at the key manager.
         *
         * Read from the driver's result rather than the raw request body, so a
         * driver that normalises or drops a value decides what gets stored. Only
         * written when it actually changed, to leave `updated_at` alone on an
         * update that touched something else.
         */
        const newName = (updated.properties && updated.properties.client_name) || '';
        if (newName !== (record.name || '')) {
            await oauth2KeyDao.setName(orgId, keyId, actor, newName, actor);
            record.name = newName;
        }

        // Nothing else to persist HERE: the rest of the metadata lives at the key
        // manager. A rotated RFC 7592 credential is stored, but by
        // `_withRegistration`, which sees the driver's response — this path only
        // ever has the name the caller sent.

        logUserAction('OAUTH2_KEY_UPDATED', req, {
            orgId,
            keyId,
            keyManagerId: km.id,
            resourceUuid: keyId,
            resourceType: 'oauth2_key',
        });
        const appUuid = await keyAppMappingDao.getByKey(keyId);
        const application = appUuid
            ? _toApplicationDto(await applicationDao.get(orgId, appUuid, actor))
            : undefined;
        /*
         * A rotated secret is passed on, not dropped.
         *
         * RFC 7592 §2.2 lets the key manager return a NEW client_secret from an
         * update, and says the client "MUST immediately discard its previous client
         * secret" when it does. The portal stores no secrets, so if this response
         * omits the new one it exists nowhere: the developer's old secret is dead by
         * the spec's own rule, the replacement was thrown away, and a successful
         * update has locked them out of their own client.
         *
         * Empty in the ordinary case where nothing rotated, which is the same shape
         * this response has always had.
         */
        return res.status(200).json(_toDetailDto(record, km, {
            secret: updated.consumerSecret,
            properties: updated.properties, application,
        }));
    } catch (error) {
        if (_sendKeyManagerError(res, error, constants.ERROR_MESSAGE.OAUTH2_KEY_UPDATE_ERROR,
            { orgId, keyId, operation: 'updateKey' })) {
            return undefined;
        }
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_UPDATE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_UPDATE_ERROR);
    }
};

/**
 * DELETE /oauth2-keys/{keyId} — operationId deleteOAuth2Key.
 *
 * Deletes the OAuth application at the key manager, then drops the local record.
 */
const deleteOAuth2Key = async (req, res) => {
    const orgId = req.orgId;
    const { keyId } = req.params;
    try {
        const actor = util.resolveActor(req);
        const record = await oauth2KeyDao.get(orgId, keyId, actor);
        if (!record) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.OAUTH2_KEY_NOT_FOUND);
        }
        // Deleting must keep working, or disabling a key manager would strand its keys
            // permanently — and the clients they left behind at the key manager.
        const km = await _resolveKeyManager(orgId, record.keyManagerId, { allowDisabled: true });
        if (!km) {
            return util.sendError(res, 409, constants.ERROR_MESSAGE.OAUTH2_KEY_OPERATION_UNSUPPORTED);
        }

        // Upstream first: dropping the row before the key manager confirms would
        // leave a live OAuth client that nothing here can reach or delete.
        try {
            await _withRegistration(
                { orgId, keyId, actor },
                (registration) => km.deleteKey(record.consumerKey, registration)
            );
        } catch (error) {
            /*
             * The client is already gone from the key manager — deleted there
             * directly, or lost with the tenant it lived in. The upstream half of
             * this operation is therefore already in the state it was asked to
             * reach, so the local row is dropped and the caller gets its 204.
             *
             * Propagating the 404 instead would be the worst of both: the row
             * survives, every later delete repeats the same 404, and the key is
             * permanently stuck in the list with no way to remove it.
             */
            if (!(error instanceof KeyManagerCallError) || error.publicReason !== 'client_not_found') {
                throw error;
            }
            logger.warn('Key manager reported the client as already absent; removing the local '
                + 'record only', { orgId, keyId, keyManagerId: km.id });
        }
        await oauth2KeyDao.remove(orgId, keyId, actor);

        logUserAction('OAUTH2_KEY_DELETED', req, {
            orgId,
            keyId,
            keyManagerId: km.id,
            resourceUuid: keyId,
            resourceType: 'oauth2_key',
        });
        return res.status(204).send();
    } catch (error) {
        if (_sendKeyManagerError(res, error, constants.ERROR_MESSAGE.OAUTH2_KEY_DELETE_ERROR,
            { orgId, keyId, operation: 'deleteKey' })) {
            return undefined;
        }
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_DELETE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_DELETE_ERROR);
    }
};

/**
 * Validate an RFC 8707 resource indicator.
 *
 * Two rules, both MUST-level in RFC 8707 §2 and restated by the MCP authorization
 * spec: it has to be an absolute URI, and it must carry no fragment. Checked here
 * rather than passed through because an invalid indicator does not fail loudly —
 * an authorization server that does not implement RFC 8707 ignores the parameter
 * entirely, so the caller gets a token that simply is not bound to anything, and
 * finds out when the resource server rejects it.
 *
 * Deliberately not enforced: https-only (a local MCP server on http is a normal
 * development case) and the no-trailing-slash preference, which the spec states as
 * SHOULD. Rejecting either would refuse requests the spec permits.
 *
 * @returns {string|null} an error message, or null when the value is usable
 */
function _resourceIndicatorError(value) {
    if (typeof value !== 'string' || !value.trim()) {
        return 'A resource indicator must be a non-empty string.';
    }
    /*
     * A fragment is forbidden, and an EMPTY one still counts: `URL` parses
     * "https://api.example.com/orders#" with `hash === ''`, so testing `parsed.hash`
     * for truthiness lets it through. RFC 3986 makes a literal `#` the fragment
     * delimiter wherever it appears, so the raw string is what to check. A
     * percent-encoded `%23` is an ordinary character and stays allowed.
     */
    if (value.indexOf('#') !== -1) {
        return 'A resource indicator must not contain a fragment.';
    }
    let parsed;
    try {
        parsed = new URL(value);
    } catch (_err) {
        return 'A resource indicator must be an absolute URI, including a scheme.';
    }
    /*
     * A scheme is required; a host is NOT. RFC 8707 §2 asks for an absolute URI and
     * explicitly allows an abstract identifier — `urn:example:orders` is a valid
     * resource indicator with no authority component at all. Requiring a host
     * refused those before they ever reached the key manager.
     */
    if (!parsed.protocol) {
        return 'A resource indicator must be an absolute URI, including a scheme.';
    }
    return null;
}

/**
 * POST /oauth2-keys/{keyId}/generate-token — operationId generateOAuth2KeyToken.
 *
 * The portal holds no secret for a key — the key manager issues one once and never
 * again — so the caller supplies it per request. It is used for this one call and
 * is never stored, logged, or echoed back; it is redacted even from the upstream
 * error body that goes to the internal log.
 */
const generateOAuth2KeyToken = async (req, res) => {
    const orgId = req.orgId;
    const { keyId } = req.params;
    try {
        const actor = util.resolveActor(req);
        const record = await oauth2KeyDao.get(orgId, keyId, actor);
        if (!record) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.OAUTH2_KEY_NOT_FOUND);
        }

        const km = await _resolveKeyManager(orgId, record.keyManagerId);
        if (!km) {
            // The key outlived its key manager's configuration, so there is no
            // token endpoint left to ask. Answered as the key being unusable
            // rather than as a server fault.
            return util.sendError(res, 404, constants.ERROR_MESSAGE.OAUTH2_KEY_NOT_FOUND);
        }

        /*
         * Ask the key manager what this client is before presenting its credential.
         *
         * Two things come out of the read and neither can be guessed:
         *
         *   token_endpoint_auth_method  where the secret goes. Sending it the wrong
         *                               way earns a bare 401, indistinguishable from
         *                               a wrong secret — so a guess here surfaces as
         *                               "your credentials were refused", which sends
         *                               the developer looking in the wrong place.
         *   grant_types                 whether client_credentials is available to
         *                               this client at all. A client registered only
         *                               for authorization_code has a perfectly good
         *                               secret and still cannot use this endpoint.
         *
         * Not fatal if the read fails or says nothing. A provision-type key manager
         * has nothing to read back, and an older server may omit either field — so
         * absence falls through to client_secret_basic, which is what a server
         * assumes when a registration does not say otherwise.
         */
        let clientMeta = {};
        try {
            const current = await _withRegistration(
                { orgId, keyId, actor },
                (registration) => km.getKey(record.consumerKey, registration)
            );
            clientMeta = (current && current.properties) || {};
        } catch (readError) {
            logger.debug('Could not read the client back before issuing a token', {
                orgId, keyId, keyManagerId: km.id, reason: readError.message,
            });
        }

        const grants = clientMeta.grant_types;
        if (Array.isArray(grants) && grants.length && !grants.includes('client_credentials')) {
            return util.sendError(res, 409,
                'This client is not registered for the client credentials grant, so it cannot get a '
                + 'token here. It is registered for: ' + grants.join(', ') + '.');
        }

        const { consumerSecret, scopes, validityPeriod, resources } = req.body;

        /*
         * Resource indicators are what bind the issued token to the API it is for,
         * so a malformed one is refused rather than dropped: an authorization
         * server without RFC 8707 support ignores the parameter silently, and the
         * caller would get an unbound token believing it was bound.
         */
        if (Array.isArray(resources)) {
            for (const resource of resources) {
                const problem = _resourceIndicatorError(resource);
                if (problem) {
                    return util.sendError(res, 400, problem);
                }
            }
        }
        const token = await km.requestToken(record.consumerKey, consumerSecret, {
            scopes: Array.isArray(scopes) ? scopes.filter(Boolean) : [],
            resources: Array.isArray(resources) ? resources.filter(Boolean) : [],
            validityPeriod,
            authMethod: clientMeta.token_endpoint_auth_method,
        });

        // Audited without the token: that a token was issued is worth recording,
        // the token itself is a live credential and belongs in no log.
        logUserAction('OAUTH2_KEY_TOKEN_GENERATED', req, {
            orgId, keyId, resourceUuid: keyId, resourceType: 'oauth2_key',
        });

        return res.status(200).json({
            accessToken: token.accessToken,
            tokenType: token.tokenType,
            validityTime: token.expiresIn,
            // What the key manager actually granted, which may be narrower than
            // what was asked for.
            tokenScopes: token.scope ? token.scope.split(/\s+/).filter(Boolean) : [],
        });
    } catch (error) {
        if (_sendKeyManagerError(res, error, constants.ERROR_MESSAGE.OAUTH2_KEY_TOKEN_ERROR,
            { orgId, keyId, operation: 'requestToken' })) {
            return undefined;
        }
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_TOKEN_ERROR, {
            error: error.message, code: error.code, stack: error.stack, orgId, keyId,
        });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_TOKEN_ERROR);
    }
};

/**
 * POST /oauth2-keys/{keyId}/associate — operationId associateOAuth2KeyApplication.
 *
 * Portal-side bookkeeping only: no key manager call is involved in associating a
 * key with an application.
 */
const associateOAuth2KeyApplication = async (req, res) => {
    const orgId = req.orgId;
    const { keyId } = req.params;
    try {
        const actor = util.resolveActor(req);
        const record = await oauth2KeyDao.get(orgId, keyId, actor);
        if (!record) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.OAUTH2_KEY_NOT_FOUND);
        }
        // The application is resolved through its own DAO, which scopes by org AND
        // creator — so associating a key with someone else's application is a 404
        // here rather than a row that quietly links across owners.
        const { applicationId } = req.body;
        const appRow = await applicationDao.getId(orgId, actor, applicationId);
        if (!appRow || !appRow.uuid) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.APPLICATION_NOT_FOUND);
        }

        await keyAppMappingDao.associate(keyId, appRow.uuid, actor);
        logUserAction('OAUTH2_KEY_ASSOCIATED', req, {
            orgId, keyId, applicationId, resourceUuid: keyId, resourceType: 'oauth2_key',
        });

        // The response carries the association and nothing else
        // (OAuth2KeyApplicationResponseSchema): the caller already holds the key,
        // and re-sending it would invite treating this as a key read.
        const app = await applicationDao.get(orgId, appRow.uuid, actor);
        return res.status(200).json({ application: _toApplicationDto(app) });
    } catch (error) {
        // message and stack explicitly: `{ error }` serialises an Error to its
        // enumerable own properties, which for a driver error is the code alone —
        // enough to know something failed, not enough to know what.
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_UPDATE_ERROR, {
            error: error.message, code: error.code, stack: error.stack, orgId, keyId,
        });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_UPDATE_ERROR);
    }
};

/**
 * POST /oauth2-keys/{keyId}/dissociate — operationId dissociateOAuth2KeyApplication.
 */
const dissociateOAuth2KeyApplication = async (req, res) => {
    const orgId = req.orgId;
    const { keyId } = req.params;
    try {
        const actor = util.resolveActor(req);
        const record = await oauth2KeyDao.get(orgId, keyId, actor);
        if (!record) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.OAUTH2_KEY_NOT_FOUND);
        }
        // Idempotent by contract: the spec says dissociate succeeds whether or not
        // the key was attached, so "no row removed" is a 204 too. The distinction
        // is kept for the audit log, where "dissociated something" and "confirmed
        // nothing was attached" are different events.
        const removed = await keyAppMappingDao.dissociate(keyId);
        logUserAction('OAUTH2_KEY_DISSOCIATED', req, {
            orgId, keyId, removed, resourceUuid: keyId, resourceType: 'oauth2_key',
        });
        return res.status(204).send();
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_UPDATE_ERROR, {
            error: error.message, code: error.code, stack: error.stack, orgId, keyId,
        });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_UPDATE_ERROR);
    }
};

/**
 * GET /applications/{applicationId}/oauth2-keys — operationId listApplicationOAuth2Keys.
 */
const listApplicationOAuth2Keys = async (req, res) => {
    const orgId = req.orgId;
    const { applicationId } = req.params;
    try {
        const actor = util.resolveActor(req);
        const idRow = await applicationDao.getId(orgId, actor, applicationId);
        if (!idRow || !idRow.uuid) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.APPLICATION_NOT_FOUND);
        }
        const keyIds = await keyAppMappingDao.listKeyIdsByApplication(idRow.uuid);
        if (!keyIds.length) {
            return res.status(200).json(util.toPaginatedList([], req));
        }

        // Each key is re-read through its own DAO, which scopes by org, portal and
        // creator. The mapping table carries no organization of its own, so this is
        // what keeps a key out of the response if it is not the caller's — the
        // association alone is not authority to see it.
        const app = _toApplicationDto(await applicationDao.get(orgId, idRow.uuid, actor));
        const names = await _keyManagerNames(orgId);
        const list = [];
        for (const id of keyIds) {
            const record = await oauth2KeyDao.get(orgId, id, actor);
            if (!record) continue;
            const name = names.get(record.keyManagerId);
            list.push(_toListItemDto(record, name ? { displayName: name } : null, app));
        }
        return res.status(200).json(util.toPaginatedList(list, req));
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.OAUTH2_KEY_RETRIEVE_ERROR, {
            error: error.message, code: error.code, stack: error.stack, orgId, applicationId,
        });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.OAUTH2_KEY_RETRIEVE_ERROR);
    }
};

module.exports = {
    // Exported for tests: the property contract and the settings narrowing are
    // what make an admin's grant-type restriction real rather than cosmetic, so
    // they are worth asserting directly rather than only through a route.
    _validateProperties,
    _applyKeyManagerSettings,
    getKeyManagerMetadata,
    createOAuth2Key,
    listOAuth2Keys,
    getOAuth2Key,
    updateOAuth2Key,
    deleteOAuth2Key,
    generateOAuth2KeyToken,
    associateOAuth2KeyApplication,
    dissociateOAuth2KeyApplication,
    listApplicationOAuth2Keys,
};
