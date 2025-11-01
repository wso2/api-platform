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
/* eslint-disable no-undef */
const { Sequelize } = require('sequelize');
const path = require('path');
const fs = require('fs');

const config = require(process.cwd() + '/config.json');
const secret = require(process.cwd() + '/secret.json');

const filePrefix = config.pathToDBCert;
const dbCAPath = path.join(process.cwd(), filePrefix);

const sequelizeOptions = {
    host: config.db.host,
    port: config.db.port,
    dialect: config.db.dialect,
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

if (config.advanced.dbSslDialectOption) {
    sequelizeOptions.dialectOptions = {
        ssl: {
            require: true,
            rejectUnauthorized: true,
            ca: fs.readFileSync(dbCAPath).toString(),
        }
    };
}

const sequelize = new Sequelize(
    config.db.database,
    config.db.username,
    secret.dbSecret,
    sequelizeOptions
);

module.exports = sequelize;
