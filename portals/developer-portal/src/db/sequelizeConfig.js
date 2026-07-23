/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 
const { Sequelize } = require('sequelize');

const { config } = require('../config/configLoader');
const { buildDbSsl } = require('./dbSsl');

const dialect = config.database.driver;
let sequelize;

if (dialect === 'sqlite') {
    sequelize = new Sequelize({
        dialect: 'sqlite',
        dialectModule: require('./betterSqlite3Compat'),
        storage: config.database.path || './devportal.db',
        logging: false,
        pool: { max: 1, min: 1, acquire: 30000, idle: 10000 },
    });
} else {
    const sequelizeOptions = {
        host: config.database.host,
        port: config.database.port,
        dialect,
        logging: false,
        pool: {
            max: 50,
            min: 2,
            acquire: 30000,
            idle: 10000
        },
        dialectOptions: {
            connectTimeout: 30000,
            requestTimeout: 30000,
        },
    };

    const ssl = buildDbSsl();
    if (ssl) {
        sequelizeOptions.dialectOptions.ssl = ssl;
    }

    sequelize = new Sequelize(
        config.database.name,
        config.database.user,
        config.database.password,
        sequelizeOptions
    );
}

module.exports = sequelize;
