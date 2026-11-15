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

/**
 * Reads the HTTPS server's certificate and key, or returns null when the server
 * runs plain HTTP (HTTPS off, or design mode). Throws when HTTPS is on and either
 * file can't be read.
 *
 * server.js calls this before it loads the database driver, so a missing
 * certificate stops startup before any database is opened or created. (The
 * Docker image used to check this in docker-entrypoint.sh; the check lives here
 * so the image needs no shell, and it uses the configured paths.)
 *
 * The paths come from the operator's own config (server.https.cert_file /
 * key_file), never from a request.
 */
function readServerCertificate(config) {
    if (!config.server.https.enabled || config.designMode?.enabled) {
        return null;
    }
    const certFile = path.resolve(config.server.https.certFile);
    const keyFile = path.resolve(config.server.https.keyFile);
    try {
        return { cert: fs.readFileSync(certFile), key: fs.readFileSync(keyFile) };
    } catch (err) {
        const wrapped = new Error(
            'HTTPS is enabled but the TLS certificate or key could not be read. Run setup.sh first, ' +
            'or mount your own certificate and key at the paths set in server.https.cert_file / key_file.'
        );
        wrapped.details = { certFile, keyFile, error: err.message };
        throw wrapped;
    }
}

/**
 * readServerCertificate for startup: on failure, logs the reason and exits the
 * process instead of throwing. `exit` is injectable for tests.
 */
function readServerCertificateOrExit(config, logger, exit = process.exit) {
    try {
        return readServerCertificate(config);
    } catch (err) {
        logger.error(err.message, { ...err.details, operation: 'httpsServerSetup' });
        return exit(1);
    }
}

module.exports = { readServerCertificate, readServerCertificateOrExit };
