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
const session = require('express-session');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');

function createSessionStore() {
    if (config.designMode?.enabled) {
        logger.info('Design Mode enabled. Using in-memory session store.');
        return new session.MemoryStore();
    }

    const dialect = config.database.driver;

    if (dialect === 'sqlite') {
        const SequelizeStore = require('connect-session-sequelize')(session.Store);
        const sequelize = require('./sequelizeConfig');
        const store = new SequelizeStore({
            db: sequelize,
            tableName: 'sessions',
            checkExpirationInterval: 60 * 60 * 1000,
            expiration: 60 * 60 * 1000,
        });
        store.sync();
        return store;
    }

    const pgSession = require('connect-pg-simple')(session);
    const pool = require('./dbPool');
    return new pgSession({
        pool,
        tableName: 'sessions',
        pruneSessionInterval: 3600,
        debug: (message) => logger.debug('Session store debug', { message }),
    });
}

module.exports = createSessionStore();
