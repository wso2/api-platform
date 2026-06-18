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
const { Organization } = require('./organization');
const { APIMetadata } = require('./apiMetadata');
const { SubscriptionMapping } = require('./application');

const APIKey = sequelize.define('DP_API_KEY', {
    KEY_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    API_ID: {
        type: DataTypes.UUID,
        allowNull: false,
        references: { model: APIMetadata, key: 'API_ID' }
    },
    SUBSCRIPTION_ID: {
        type: DataTypes.UUID,
        allowNull: true,
        references: { model: SubscriptionMapping, key: 'SUB_ID' }
    },
    ORG_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    NAME: {
        type: DataTypes.STRING(128),
        allowNull: false
    },
    STATUS: {
        type: DataTypes.ENUM('ACTIVE', 'REVOKED'),
        allowNull: false,
        defaultValue: 'ACTIVE'
    },
    EXPIRES_AT: {
        type: DataTypes.DATE,
        allowNull: true
    },
    CREATED_BY: {
        type: DataTypes.STRING,
        allowNull: false
    },
    REVOKED_AT: {
        type: DataTypes.DATE,
        allowNull: true
    }
}, {
    timestamps: true,
    createdAt: 'CREATED_AT',
    updatedAt: false,
    tableName: 'DP_API_KEY',
    returning: true
});

APIKey.belongsTo(Organization, { foreignKey: 'ORG_ID' });
Organization.hasMany(APIKey, { foreignKey: 'ORG_ID' });
APIKey.belongsTo(APIMetadata, { foreignKey: 'API_ID' });

module.exports = APIKey;
