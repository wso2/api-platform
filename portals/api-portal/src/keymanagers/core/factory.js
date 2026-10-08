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
 * Builds the live key-manager instances from validated config, resolving each
 * `type` against the built-in driver registry.
 *
 * Construction is deferred to first use rather than done at startup, because
 * building an authenticator is async (mTLS reads certificate files) while
 * configLoader's bootstrap is synchronous. Everything checkable without I/O —
 * required fields, URL scheme and address range, unknown type, duplicate id —
 * is already validated at startup by config/keyManagerConfig.js, so a
 * misconfiguration still fails the boot rather than the first request.
 */

const { getDriver, registeredTypes } = require('./registry');

class Factory {
    constructor(managers) {
        this.managers = managers; // Map<id, KeyManager>
    }

    /** @returns {KeyManager|null} null when no key manager has this id */
    forId(id) {
        return this.managers.get(id) || null;
    }

    has(id) {
        return this.managers.has(id);
    }

    /**
     * Every configured instance, sorted by id for stable output.
     *
     * The read accessor for callers outside this subsystem — keyManagerRegistry
     * projects these into the `/key-managers` list. Exists so `managers` stays
     * the Factory's own business: a caller holding the Map could mutate the
     * live instance table, and nothing here would notice.
     *
     * @returns {Array<KeyManager>}
     */
    all() {
        return [...this.managers.values()].sort((a, b) => a.id.localeCompare(b.id));
    }

    /** Every configured instance's id, sorted. */
    ids() {
        return this.all().map((m) => m.id);
    }

    /** Backs GET /key-managers/metadata; sorted by id for stable output. */
    allMetadata() {
        return [...this.managers.values()]
            .map((m) => m.metadata())
            .sort((a, b) => a.id.localeCompare(b.id));
    }

    get size() {
        return this.managers.size;
    }
}

/**
 * @param {Array<object>} instances  normalized configs from keyManagerConfig.js
 * @returns {Promise<Factory>}
 */
async function buildFactory(instances) {
    const managers = new Map();
    for (const cfg of instances) {
        const create = getDriver(cfg.type);
        if (!create) {
            // Also checked at startup; repeated here because the registry is the
            // authority and this function is reachable from tests directly.
            throw new Error(
                `key manager "${cfg.id}": unknown type "${cfg.type}". ` +
                `Built-in types: ${registeredTypes().join(', ')}`
            );
        }
        if (managers.has(cfg.id)) {
            throw new Error(`duplicate key manager id "${cfg.id}"`);
        }
        const authRequest = await cfg.auth.requester();
        managers.set(cfg.id, create(cfg, authRequest));
    }
    return new Factory(managers);
}

module.exports = { Factory, buildFactory };
