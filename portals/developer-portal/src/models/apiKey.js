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
    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    API_UUID: {
        field: 'api_uuid',
        type: DataTypes.STRING(40),
        allowNull: false,
        references: { model: APIMetadata, key: 'uuid' }
    },
    SUBSCRIPTION_UUID: {
        field: 'subscription_uuid',
        type: DataTypes.STRING(40),
        allowNull: true,
        references: { model: SubscriptionMapping, key: 'uuid' }
    },
    ORG_UUID: {
        field: 'org_uuid',
        type: DataTypes.STRING(40),
        allowNull: false
    },
    NAME: {
        field: 'name',
        type: DataTypes.STRING(128),
        allowNull: false
    },
    STATUS: {
        field: 'status',
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'ACTIVE'
    },
    EXPIRES_AT: {
        field: 'expires_at',
        type: DataTypes.DATE,
        allowNull: true
    },
    CREATED_BY: {
        field: 'created_by',
        type: DataTypes.STRING,
        allowNull: false
    },
    UPDATED_BY: {
        field: 'updated_by',
        type: DataTypes.STRING,
        allowNull: false
    },
    REVOKED_AT: {
        field: 'revoked_at',
        type: DataTypes.DATE,
        allowNull: true
    },
    REVOKED_BY: {
        field: 'revoked_by',
        type: DataTypes.STRING(200),
        allowNull: true
    },
    CREATED_AT: {
        field: 'created_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
    UPDATED_AT: {
        field: 'updated_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    }
}, {
    timestamps: false,
    tableName: 'dp_api_key',
    returning: true,
    checks: [
        {
            name: 'chk_api_key_revoked',
            sql: `((revoked_at IS NULL AND status != 'REVOKED') OR (revoked_at IS NOT NULL AND status = 'REVOKED'))`
        }
    ],
    indexes: [
        { name: 'idx_api_key_org_api_uuid', fields: ['ORG_UUID', 'API_UUID'] },
        { name: 'idx_api_key_subscription_uuid', fields: ['SUBSCRIPTION_UUID'] },
        { name: 'idx_api_key_status', fields: ['STATUS'] },
    ],
});

APIKey.belongsTo(Organization, { foreignKey: 'ORG_UUID' });
Organization.hasMany(APIKey, { foreignKey: 'ORG_UUID' });
APIKey.belongsTo(APIMetadata, { foreignKey: 'API_UUID', as: 'DP_API_METADATA' });
APIKey.belongsTo(SubscriptionMapping, { foreignKey: 'SUBSCRIPTION_UUID', onDelete: 'SET NULL' });
SubscriptionMapping.hasMany(APIKey, { foreignKey: 'SUBSCRIPTION_UUID', onDelete: 'SET NULL' });

module.exports = APIKey;
