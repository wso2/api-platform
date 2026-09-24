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
 * Entry point for the key-manager subsystem: the single place the rest of the
 * portal reaches for a configured key manager.
 *
 *   const { getFactory } = require('../keymanagers');
 *   const factory = await getFactory();
 *   const km = factory.forId('thunder-local');
 *
 * The factory is built once, on first use, and memoized. It is not built at
 * startup because constructing an authenticator is async (mTLS reads
 * certificate files) whereas configLoader's bootstrap is synchronous — but the
 * config itself is fully validated during that bootstrap
 * (config/keyManagerConfig.js, called from configLoader), so a bad entry still
 * aborts the boot rather than surfacing on the first request.
 *
 * A failed build is not cached: a key manager that was unreachable when the
 * first request landed should be retried on the next one, not remembered as
 * broken for the life of the process.
 */

require('./drivers'); // built-in drivers self-register on require

const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const { validateKeyManagerConfig } = require('../config/keyManagerConfig');
const { buildFactory } = require('./core/factory');
const { registeredTypes } = require('./core/registry');

let cachedFactory = null;
let inflight = null;

/**
 * @returns {Promise<import('./core/factory').Factory>} configured key managers, keyed by id
 */
async function getFactory() {
    if (cachedFactory) return cachedFactory;
    if (!inflight) {
        inflight = (async () => {
            // Re-validating here (rather than reusing what configLoader checked)
            // keeps this callable from a test that never went through the
            // bootstrap, and costs nothing — the checks are I/O-free.
            const instances = validateKeyManagerConfig(config);
            const factory = await buildFactory(instances);
            logger.info('Key managers initialised', {
                configured: factory.size,
                ids: factory.ids(),
            });
            cachedFactory = factory;
            return factory;
        })().finally(() => {
            inflight = null;
        });
    }
    return inflight;
}

/** Test seam: drop the memoized factory so the next getFactory() rebuilds. */
function resetFactory() {
    cachedFactory = null;
    inflight = null;
}

module.exports = { getFactory, resetFactory, registeredTypes };
