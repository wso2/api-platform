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

const fs = require('fs');
const path = require('path');
const { config } = require('../config/configLoader');

// The subset of platform-api's ssl_mode values that devportal supports —
// mirrors devportal's original enable/disable, resolved to full verification.
const SUPPORTED_SSL_MODES = ['disable', 'verify-full'];

/**
 * Builds the node-postgres `ssl` option from config.database.sslMode. Shared by
 * sequelizeConfig.js and dbPool.js so both connect the same way:
 *
 *   disable      → no TLS (returns undefined)
 *   verify-full  → encrypt AND verify the cert chain (against ssl_root_cert)
 *                  plus the hostname
 *
 * Fails closed on an unrecognised mode rather than silently downgrading.
 *
 * @returns {object|undefined} node-postgres ssl options, or undefined for "disable".
 */
function buildDbSsl() {
    const db = config.database || {};
    const mode = db.sslMode || 'disable';

    if (!SUPPORTED_SSL_MODES.includes(mode)) {
        throw new Error(
            `Unsupported database ssl_mode "${mode}" — expected one of: ${SUPPORTED_SSL_MODES.join(' | ')}`
        );
    }

    if (mode === 'disable') return undefined;

    // verify-full
    const ssl = { require: true, rejectUnauthorized: true };
    if (db.sslRootCert) {
        ssl.ca = fs.readFileSync(path.resolve(process.cwd(), db.sslRootCert)).toString();
    }
    return ssl;
}

module.exports = { buildDbSsl };
