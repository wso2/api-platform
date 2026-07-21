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
const http = require('http');
const https = require('https');
const fs = require('fs');
const path = require('path');
const logger = require('./config/logger');
const { config } = require('./config/configLoader');
const webhookDispatcher = require('./services/webhooks/dispatcher');
const webhookDeliveryWorker = require('./services/webhooks/deliveryWorker');
const sequelize = require('./db/sequelizeConfig');
const { seedDefaultOrg } = require('./services/seederService');
const app = require('./app');

const liveReload = process.env.NODE_ENV === 'development' ? require('./liveReload') : null;

const PORT = process.env.PORT || config.server.port;

function startBackgroundServices() {
    if (config.designMode?.enabled) return;
    try {
        webhookDispatcher.start();
        webhookDeliveryWorker.start();
        logger.info('Services: webhook dispatcher + delivery worker started ✓');
    } catch (error) {
        logger.warn('Could not start webhook workers', {
            error: error.message,
            stack: error.stack
        });
    }
}

// Prints a startup banner horizontally centered in an 80-column terminal, with
// blank-line padding above and below the title — matches ai-workspace/bff/main.go's
// printBanner(). Written directly to stdout (not through the structured logger) so
// timestamp/level prefixes don't break the centering.
function printBanner(visitUrl) {
    const termWidth = 80;
    const lines = [
        '='.repeat(40),
        '',
        '',
        'Developer Portal Started',
        '',
        `Visit Portal: ${visitUrl}`,
        '',
        '',
        '='.repeat(40),
    ];
    console.log();
    for (const line of lines) {
        const pad = Math.max(0, Math.floor((termWidth - line.length) / 2));
        console.log(' '.repeat(pad) + line);
    }
    console.log();
}

function logStartupBanner() {
    const orgSegment = config.designMode?.enabled ? '' : `/${config.organization.defaultName || '<organization>'}`;
    // The bare org URL redirects server-side to /views/default (orgContentRoute.js) —
    // shorter and avoids baking view-naming details into the banner.
    const scheme = config.tls.enabled && !config.designMode?.enabled ? 'https' : 'http';
    const visitUrl = `${scheme}://localhost:${PORT}${orgSegment}`;
    printBanner(visitUrl);
}

async function onListening() {
    startBackgroundServices();
    await seedDefaultOrg().catch(err =>
        logger.error('Unexpected error during default org seeding', { error: err.message })
    );
    logStartupBanner();
}

let server;

async function startServer() {
    logger.info('Developer Portal starting...');
    // Sync database schema for SQLite in production mode
    if (config.database.type === 'sqlite' && !config.designMode?.enabled) {
        await sequelize.sync();
        logger.info('Database: SQLite schema synced ✓');
    }

    if (!config.tls.enabled || config.designMode?.enabled) {
        server = http.createServer(app).listen(PORT, '0.0.0.0', onListening);
    } else {
    try {
        const certPath = path.resolve(config.tls.certFile);
        const keyPath = path.resolve(config.tls.keyFile);

        const serverCert = fs.readFileSync(certPath);
        const serverKey = fs.readFileSync(keyPath);
        const caCert = fs.readFileSync(path.resolve(config.tls.caFile));

        server = https.createServer({
            key: serverKey,
            cert: serverCert,
            ca: caCert,
            requestCert: true,
            rejectUnauthorized: false,
        }, app).listen(PORT, onListening);

    } catch (err) {
        logger.error('Error setting up HTTPS server', {
            error: err.message,
            stack: err.stack,
            operation: 'httpsServerSetup'
        });
        process.exit(1);
    }
    }
}

startServer();

// Handle Uncaught Exceptions
process.on('uncaughtException', (err) => {
    logger.error('Uncaught Exception - Application will exit', {
        error: err.message,
        stack: err.stack,
        type: 'uncaughtException'
    });
    process.exit(1);
});

// Handle Unhandled Rejections
process.on('unhandledRejection', (reason, promise) => {
    logger.error('Unhandled Promise Rejection - Application will exit', {
        reason: reason?.message || reason,
        promise: promise?.toString(),
        type: 'unhandledRejection'
    });
    process.exit(1);
});

// Graceful shutdown handlers
const gracefulShutdown = (signal) => {
    logger.info('Graceful shutdown initiated...', {
        signal,
        message: `Received ${signal}. Gracefully shutting down...`
    });

    const done = () => {
        logger.info('Application shutdown complete');
        process.exit(0);
    };

    if (server) {
        // Close keep-alive connections immediately so server.close() doesn't hang
        server.closeAllConnections();
        server.close(done);
    } else {
        done();
    }

    // Force-exit after 3 s if graceful close hangs (e.g. long-polling connections)
    setTimeout(() => {
        logger.warn('Graceful shutdown timed out — forcing exit');
        process.exit(1);
    }, 3000).unref();
};

process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));
process.on('SIGINT', () => gracefulShutdown('SIGINT'));
// nodemon sends SIGUSR2 to restart; process.once so the next spawned process can re-register
process.once('SIGUSR2', () => {
    if (liveReload) liveReload.notify();
    gracefulShutdown('SIGUSR2');
});
