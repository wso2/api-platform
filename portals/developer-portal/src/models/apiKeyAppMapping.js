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

const APIKeyAppMapping = sequelize.define('dp_api_key_app_mapping', {
    key_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        primaryKey: true,
        references: { model: APIKey, key: 'uuid' },
    },
    app_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: { model: Application, key: 'uuid' },
    },
    created_by: {
        type: DataTypes.STRING,
        allowNull: false
    },
    created_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_api_key_app_mappings',
    returning: true,
    indexes: [
        { name: 'idx_api_key_app_mappings_app_uuid', fields: ['app_uuid'] },
    ],
});

APIKeyAppMapping.belongsTo(APIKey, { foreignKey: 'key_uuid', onDelete: 'CASCADE' });
APIKey.hasOne(APIKeyAppMapping, { foreignKey: 'key_uuid', onDelete: 'CASCADE' });

APIKeyAppMapping.belongsTo(Application, { foreignKey: 'app_uuid', onDelete: 'CASCADE' });
Application.hasMany(APIKeyAppMapping, { foreignKey: 'app_uuid', onDelete: 'CASCADE' });

module.exports = APIKeyAppMapping;
