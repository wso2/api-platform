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

'use strict';

/*
 * The one place that answers "which key managers exist, and where is each one
 * declared".
 *
 * There are two authorship sources and deliberately no synchronisation between
 * them:
 *
 *   config — [[api_portal.key_manager]] entries. Validated fail-closed at
 *            startup, their provisioning secrets resolved from {{ env }} and
 *            never written to the database, and editable only by changing the
 *            deployed configuration. These are the ones that can perform
 *            Dynamic Client Registration today.
 *   api    — rows in `key_managers`, created through this REST API / the UI.
 *            Mutable at runtime.
 *
 * Both live in ONE identifier space: the handle (spelled `id` in requests,
 * responses and TOML). That is what lets callers treat them uniformly and what
 * lets `oauth2_consumer_keys.key_manager_id` store a single kind of value.
 *
 * Everything that needs to resolve a key manager for the CRUD surface goes
 * through this module. Writing "try config, then the database" inline in
 * several places would drift, and the drift would be security-relevant: a
 * lookup that consulted only the database would silently treat a
 * config-declared key manager as non-existent, and one that consulted only
 * config would let a mutation through on a row it should not touch.
 *
 * NOT the resolver for the DCR path. oauth2KeyService needs a driver *instance*
 * to make RFC 7591 calls with, and only config-declared entries have one until
 * `key_manager_configurations` exists — so that path resolves against the
 * factory directly and treats a database-only key manager as unusable.
 */

const { getFactory } = require('../keymanagers');
const kmDao = require('../dao/keyManagerDao');
const driverBuilder = require('./keyManagerDriverBuilder');
const { NotFoundError } = require('../utils/errors/customErrors');
const { KeyManagerCallError } = require('../keymanagers/core/keyManager');

const SOURCE_CONFIG = 'config';
const SOURCE_API = 'api';

/**
 * Config-declared key managers, projected into the same shape a `key_managers`
 * row would take, so callers can treat both alike.
 *
 * Config entries carry no organization: they are declared per portal, in a
 * file, and this portal serves a single organization (see the 405 on the
 * organization lifecycle operations). They are therefore offered to whichever
 * org is asking, and `org_uuid` is left unset rather than faked.
 *
 * They are always enabled — a config entry that should not be offered is
 * removed from the file, which is also what makes it disappear from here.
 */
async function _configEntries() {
    const factory = await getFactory();
    return factory.all().map((km) => ({
        source: SOURCE_CONFIG,
        handle: km.id,
        display_name: km.displayName,
        token_endpoint: km.tokenEndpoint,
        enabled: 1,
        // The driver handling this entry. A stored key manager keeps this in its
        // key_manager_configurations row; a config-declared one has no such row,
        // so it has to come off the built instance or it would show as untyped.
        driver_type: km.type,
        // uuid/org_uuid/created_by/created_at have no meaning for a file-declared
        // entry. Left absent so the DTO omits them rather than inventing values.
    }));
}

/**
 * Every key manager visible to this organization, config first.
 *
 * Config precedes the database on purpose: on a handle collision the config
 * entry is the one that wins, because it is the reviewed, deployed definition
 * and the one the DCR path can actually use. Such a collision is refused at
 * create time (see keyManagerService), so it should not arise — but if a
 * database row predates the config entry, this is the tie-break.
 *
 * @param {string} orgId
 * @param {object} [opts]
 * @param {boolean} [opts.includeDisabled] include disabled database rows
 * @returns {Promise<Array<object>>} rows, each tagged with `source`
 */
async function list(orgId, { includeDisabled = false } = {}) {
    const config = await _configEntries();
    const rows = includeDisabled ? await kmDao.list(orgId) : await kmDao.listEnabled(orgId);
    const seen = new Set(config.map((entry) => entry.handle));
    const dbEntries = rows
        .filter((row) => !seen.has(row.handle))
        .map((row) => ({ ...row, source: SOURCE_API }));
    return [...config, ...dbEntries];
}

/**
 * Resolve one key manager by handle. Config wins, as in list().
 *
 * @returns {Promise<object|null>} the entry tagged with `source`, or null
 */
async function findByHandle(orgId, handle) {
    if (!handle) return null;
    const config = await _configEntries();
    const match = config.find((entry) => entry.handle === handle);
    if (match) return match;

    try {
        // getByHandle signals absence by throwing, not by returning null.
        // Translated here so every caller sees one shape: a row or null.
        const row = await kmDao.getByHandle(orgId, handle);
        return { ...row, source: SOURCE_API };
    } catch (error) {
        if (error instanceof NotFoundError) return null;
        throw error;
    }
}

/**
 * The live driver for one key manager, or null when it cannot register clients.
 *
 * This is the resolver the Dynamic Client Registration path uses, and the only
 * place the two sources of driver meet:
 *
 *   - a config-declared key manager already has an instance, built once by the
 *     factory and held for the life of the process;
 *   - an API-created one is built on demand from its `key_manager_configurations`
 *     row, and cached until that row changes.
 *
 * A key manager without a provisioning row is NOT driverless — it resolves to a
 * ProvisionKeyManager, whose clients are created at the identity server and
 * imported here. That is what every key manager predating DCR support becomes,
 * and it is why those remain usable for key creation.
 *
 * Null therefore means "no such key manager", or a stored driver type this build
 * does not ship. The caller turns that into a clear refusal rather than an error.
 *
 * @returns {Promise<object|null>} a KeyManager instance, or null
 */
async function resolveDriver(orgId, handle, { allowDisabled = false } = {}) {
    if (!handle) return null;
    const factory = await getFactory();
    const fromConfig = factory.forId(handle);
    // A config-declared entry has no disabled state: one that should not be
    // offered is removed from the file, which is also what removes it here.
    if (fromConfig) return fromConfig;

    const entry = await findByHandle(orgId, handle);
    if (!entry || entry.source !== SOURCE_API) return null;

    /*
     * Disabling is an admin taking a key manager out of service, so by default
     * nothing new may be done through it. The check lives here rather than in
     * each handler because this is the one function every operation funnels
     * through, and it defaults to refusing so a call site added later inherits
     * the safe answer without anyone remembering to add a check.
     *
     * `allowDisabled` is the explicit, per-operation widening — never inferred.
     * Two operations need it, and both are about a key that ALREADY exists:
     * reading one, and deleting one. Refusing those would mean disabling a key
     * manager stranded its keys permanently — unreadable in the UI and
     * impossible to clean up, along with the clients they left behind upstream.
     * Disabling stops new work; it does not erase what was already issued.
     */
    if (!allowDisabled && !entry.enabled) {
        throw new KeyManagerCallError(
            'key_manager_disabled',
            `key manager "${handle}" is disabled; refusing to act through it`,
            null
        );
    }
    return driverBuilder.forKeyManager(orgId, entry);
}

/**
 * Whether a handle is claimed by configuration.
 *
 * Used by the mutating operations, which must refuse a config-declared handle
 * rather than reporting it missing: a 404 on an id the caller can see in the
 * list reads as a bug, and creating a database row under that handle would
 * shadow the config entry.
 */
async function isConfigDeclared(handle) {
    if (!handle) return false;
    const config = await _configEntries();
    return config.some((entry) => entry.handle === handle);
}

module.exports = {
    list,
    findByHandle,
    resolveDriver,
    isConfigDeclared,
    SOURCE_CONFIG,
    SOURCE_API,
};
