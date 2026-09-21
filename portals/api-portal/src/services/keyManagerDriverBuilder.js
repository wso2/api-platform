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
 * Builds a live KeyManager driver from a database-backed configuration — the
 * API-created counterpart of what `buildFactory` does for TOML entries.
 *
 * Both paths converge on the same two primitives: `getDriver(type)` from the
 * registry, and `buildAuthenticator` from keyManagerConfig. That is deliberate.
 * The alternative — a second construction path for database rows — would be the
 * place where the two quietly stop behaving alike, and the difference would be
 * in the request posture: which addresses may be dialled, whether TLS is
 * verified, how the portal presents its own credential.
 *
 * One thing the two paths do NOT share is the connection policy. A TOML entry
 * may relax the SSRF guard (allow_private_endpoints, insecure_skip_verify); a
 * database row cannot, because those are operator decisions and this row was
 * written through an authenticated API. An API-created key manager therefore
 * always gets the strict defaults, and pointing one at a private address fails —
 * which is the whole point of the guard.
 */

const { getDriver, registeredTypes } = require('../keymanagers/core/registry');
const { buildClient } = require('../keymanagers/core/httpClient');
const { ProvisionKeyManager } = require('../keymanagers/drivers/provision');
const { buildAuthenticator } = require('../config/keyManagerConfig');
const { config } = require('../config/configLoader');
const kmConfigDao = require('../dao/keyManagerConfigurationDao');
const logger = require('../config/logger');

/*
 * Instances are cached because ClientCredentials caches its minted token on the
 * instance: rebuilding per request would mean a token request per DCR call, and
 * a needless stampede on the key manager when several keys are generated at
 * once.
 *
 * The key includes updated_at, so editing a key manager's configuration
 * invalidates it without any explicit eviction — the next lookup simply misses.
 * Entries for a deleted key manager are never looked up again and age out with
 * the process.
 */
const instanceCache = new Map();

/**
 * The outbound posture for every API-created key manager.
 *
 * Read from configuration, never from the row: the row was written through an
 * authenticated API, and letting it widen its own reach would make the address
 * guard something an admin token can switch off. `[api_portal.key_manager_provisioning]`
 * is off on all three counts by default, so this is strict unless the operator
 * deliberately relaxed it — typically to point a development portal at an
 * identity server on localhost.
 *
 * Resolved once and frozen. One value for all of them is the point, not a
 * limitation: these key managers are configured by API callers, so the posture
 * has to be the deployment's answer rather than each caller's.
 */
const provisioningPolicy = (config && config.keyManagerProvisioning) || {};
const DB_CLIENT_POLICY = Object.freeze({
    insecureSkipVerify: provisioningPolicy.insecureSkipVerify === true,
    allowPrivateEndpoints: provisioningPolicy.allowPrivateEndpoints === true,
    allowHttpEndpoints: provisioningPolicy.allowHttpEndpoints === true,
});

function cacheKey(orgId, record, updatedAt) {
    return `${orgId}:${record.keyManagerUuid}:${updatedAt || ''}`;
}

/**
 * Build (or return a cached) driver instance for a database-backed key manager.
 *
 * Always returns an instance for a key manager that exists: one built from its
 * provisioning row if it has one, otherwise a ProvisionKeyManager. Null is
 * reserved for "no such key manager", and for a stored driver type this build
 * does not ship.
 *
 * @param {string} orgId
 * @param {object} km          the `key_managers` row, as the registry returns it
 * @returns {Promise<object|null>} a KeyManager instance, or null
 */
async function forKeyManager(orgId, km) {
    if (!km || !km.uuid) return null;

    const cached = instanceCache.get(cacheKey(orgId, { keyManagerUuid: km.uuid }, km.updated_at));
    if (cached) return cached;

    const cfg = await kmConfigDao.getWithSecrets(orgId, km.uuid);
    if (!cfg) {
        // No provisioning row means this key manager does not register clients:
        // either it predates that capability, or an admin deliberately created it
        // as a provision-type one. Both are the same thing, and both are usable —
        // developers import a client id rather than having one minted. Built
        // fresh rather than cached: there is no credential to mint, no token to
        // reuse, and nothing here that survives the call.
        // A requester with no credential, but with the guarded client: the only
        // call this driver makes is the token request, which carries the
        // developer's own credential and must not be given the portal's.
        const client = buildClient(DB_CLIENT_POLICY);
        return new ProvisionKeyManager(
            {
                id: km.handle,
                displayName: km.display_name,
                description: '',
                tokenEndpoint: km.token_endpoint,
            },
            (url, opts = {}) => client.request({ url, ...opts })
        );
    }

    const create = getDriver(cfg.type);
    if (!create) {
        // The driver set ships in the image, so a stored type can become unknown
        // after a downgrade or a driver rename. Reported as unusable rather than
        // thrown: the key manager still exists and still lists, it just cannot
        // register clients until its type is corrected.
        logger.error('Key manager references a driver type this build does not have', {
            keyManagerId: km.handle, type: cfg.type, available: registeredTypes(),
        });
        return null;
    }

    // The shape buildFactory hands a driver, assembled from the row instead of
    // from TOML. Field names match normalizeInstance's output exactly.
    const instanceConfig = {
        id: km.handle,
        type: cfg.type,
        displayName: km.display_name,
        // `key_managers` is shipped and has no description column, so the one a
        // UI-added key manager can carry lives on its configuration row. Empty
        // until something writes it — see KeyManagerProvisioningRequest, which
        // has no `description` field yet.
        description: cfg.description || '',
        registrationEndpoint: cfg.registrationEndpoint,
        tokenEndpoint: km.token_endpoint,
        authorizeEndpoint: cfg.authorizeEndpoint,
        clientPolicy: DB_CLIENT_POLICY,
    };

    const authenticator = buildAuthenticator(
        {
            method: cfg.authMethod,
            clientId: cfg.authClientId,
            clientSecret: cfg.authClientSecret,
            username: cfg.authUsername,
            password: cfg.authPassword,
            scopes: cfg.authScopes,
            resource: cfg.authResource,
            headerName: cfg.authHeaderName,
            scheme: cfg.authScheme,
            apiKey: cfg.authApiKey,
        },
        `key manager "${km.handle}"`,
        DB_CLIENT_POLICY,
        km.token_endpoint
    );

    const instance = create(instanceConfig, await authenticator.requester());
    instanceCache.set(cacheKey(orgId, { keyManagerUuid: km.uuid }, km.updated_at), instance);
    return instance;
}

/** Test seam, and the explicit eviction a delete can use. */
function clearCache() {
    instanceCache.clear();
}

module.exports = { forKeyManager, clearCache, DB_CLIENT_POLICY };
