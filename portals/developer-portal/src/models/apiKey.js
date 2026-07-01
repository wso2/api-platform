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

const APIKey = sequelize.define('dp_api_key', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    api_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: { model: APIMetadata, key: 'uuid' }
    },
    subscription_uuid: {
        type: DataTypes.STRING(40),
        allowNull: true,
        references: { model: SubscriptionMapping, key: 'uuid' }
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    name: {
        type: DataTypes.STRING(128),
        allowNull: false
    },
    status: {
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'ACTIVE'
    },
    expires_at: {
        type: DataTypes.DATE,
        allowNull: true
    },
    created_by: {
        type: DataTypes.STRING,
        allowNull: false
    },
    updated_by: {
        type: DataTypes.STRING,
        allowNull: false
    },
    revoked_at: {
        type: DataTypes.DATE,
        allowNull: true
    },
    revoked_by: {
        type: DataTypes.STRING(200),
        allowNull: true
    },
    created_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
    updated_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    }
}, {
    timestamps: false,
    tableName: 'dp_api_keys',
    returning: true,
    checks: [
        {
            name: 'chk_api_key_revoked',
            sql: `((revoked_at IS NULL AND status != 'REVOKED') OR (revoked_at IS NOT NULL AND status = 'REVOKED'))`
        }
    ],
    indexes: [
        { name: 'idx_api_key_org_api_uuid', fields: ['org_uuid', 'api_uuid'] },
        { name: 'idx_api_key_subscription_uuid', fields: ['subscription_uuid'] },
        { name: 'idx_api_key_status', fields: ['status'] },
    ],
});

APIKey.belongsTo(Organization, { foreignKey: 'org_uuid' });
Organization.hasMany(APIKey, { foreignKey: 'org_uuid' });
APIKey.belongsTo(APIMetadata, { foreignKey: 'api_uuid' });
APIKey.belongsTo(SubscriptionMapping, { foreignKey: 'subscription_uuid', onDelete: 'SET NULL' });
SubscriptionMapping.hasMany(APIKey, { foreignKey: 'subscription_uuid', onDelete: 'SET NULL' });

module.exports = APIKey;
