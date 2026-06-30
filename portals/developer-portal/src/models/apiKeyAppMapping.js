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
const { Sequelize, DataTypes } = require('sequelize');
const sequelize = require('../db/sequelizeConfig');
const APIKey = require('./apiKey');
const { Application } = require('./application');

const APIKeyAppMapping = sequelize.define('DP_API_KEY_APP_MAPPING', {
    KEY_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false,
        primaryKey: true,
        references: { model: APIKey, key: 'UUID' },
    },
    APP_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: { model: Application, key: 'UUID' },
    },
    CREATED_BY: {
        type: DataTypes.STRING,
        allowNull: false
    },
    CREATED_AT: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'DP_API_KEY_APP_MAPPING',
    returning: true,
    indexes: [
        { name: 'IDX_API_KEY_APP_MAPPING_APP_UUID', fields: ['APP_UUID'] },
    ],
});

APIKeyAppMapping.belongsTo(APIKey, { foreignKey: 'KEY_UUID', onDelete: 'CASCADE' });
APIKey.hasOne(APIKeyAppMapping, { foreignKey: 'KEY_UUID', onDelete: 'CASCADE' });

APIKeyAppMapping.belongsTo(Application, { foreignKey: 'APP_UUID', onDelete: 'CASCADE' });
Application.hasMany(APIKeyAppMapping, { foreignKey: 'APP_UUID', onDelete: 'CASCADE' });

module.exports = APIKeyAppMapping;
