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
const constants = require('./utils/constants');
const webhookDispatcher = require('./services/webhooks/dispatcher');
const webhookDeliveryWorker = require('./services/webhooks/deliveryWorker');
const sequelize = require('./db/sequelize');
const { seedDefaultOrg } = require('./services/seeder');
const app = require('./app');

const PORT = process.env.PORT || config.defaultPort;

function startBackgroundServices() {
    try {
        webhookDispatcher.start();
        webhookDeliveryWorker.start();
        logger.info('Webhook dispatcher and delivery worker started');
    } catch (error) {
        logger.warn('Could not start webhook workers', {
            error: error.message,
            stack: error.stack
        });
    }
}

function logStartupInfo() {
    logger.info(`Developer Portal V2 is running on port ${PORT}`);
    logger.info(`Mode: ${config.mode}`);

    if (config.mode === constants.DEV_MODE) {
        logger.info('⚠️  Since you are in DEV mode, ensure default content is available at configured pathToContent ' +
            'and mock folder must exist in root directory');
    }

    const visitUrl = config.baseUrl + (config.mode === constants.DEV_MODE ? "/views/default" : "/<organization>/views/default");
    logger.info(`Visit ${visitUrl}`);
}

function onListening() {
    logStartupInfo();
    startBackgroundServices();
    seedDefaultOrg().catch(err =>
        logger.error('Unexpected error during default org seeding', { error: err.message })
    );
}

let server;

async function startServer() {
    if (config.db.dialect === 'sqlite') {
        await sequelize.sync();
        logger.info('SQLite schema synced');
    }

    if (config.advanced.http) {
        server = http.createServer(app).listen(PORT, '0.0.0.0', onListening);
    } else {
    try {
        const certPath = path.resolve(config.serverCerts.pathToCert);
        const keyPath = path.resolve(config.serverCerts.pathToPK);

        const serverCert = fs.readFileSync(certPath);
        const serverKey = fs.readFileSync(keyPath);
        const caCert = fs.readFileSync(path.resolve(config.serverCerts.pathToCA));

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
});

// Handle Unhandled Rejections
process.on('unhandledRejection', (reason, promise) => {
    logger.error('Unhandled Promise Rejection - Application will exit', {
        reason: reason?.message || reason,
        promise: promise?.toString(),
        type: 'unhandledRejection'
    });
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
        server.close(done);
    } else {
        done();
    }
};

process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));
process.on('SIGINT', () => gracefulShutdown('SIGINT'));
// nodemon sends SIGUSR2 to restart; process.once so the next spawned process can re-register
process.once('SIGUSR2', () => gracefulShutdown('SIGUSR2'));
