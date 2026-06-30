/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const constants = require('../utils/constants');
const SubscriptionPlan = require('./subscriptionPlan');
const APISubscriptionPlan = require('./apiSubscriptionPlan');
const { KeyManager } = require('./keyManager');


const Application = sequelize.define('DP_APPLICATION', {

    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_UUID: {
        field: 'org_uuid',
        type: DataTypes.STRING(40),
        allowNull: false
    },
    CREATED_BY: {
        field: 'created_by',
        type: DataTypes.STRING,
        allowNull: false
    },
    NAME: {
        field: 'name',
        type: DataTypes.STRING,
        allowNull: false
    },
    HANDLE: {
        field: 'handle',
        type: DataTypes.STRING,
        allowNull: false
    },
    DESCRIPTION: {
        field: 'description',
        type: DataTypes.STRING(1023),
        allowNull: true
    },
    CREATED_AT: {
        field: 'created_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
    UPDATED_BY: {
        field: 'updated_by',
        type: DataTypes.STRING,
        allowNull: false
    },
    UPDATED_AT: {
        field: 'updated_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_application',
    returning: true,
    indexes: [
        { name: 'idx_application_org_created_by', fields: ['ORG_UUID', 'CREATED_BY'] },
        { name: 'uq_application_org_handle', unique: true, fields: ['ORG_UUID', 'HANDLE'] },
    ],
});

const ApplicationKeyMapping = sequelize.define('DP_APP_KEY_MAPPING', {

    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    APP_UUID: {
        field: 'app_uuid',
        type: DataTypes.STRING(40),
        allowNull: false
    },
    KM_UUID: {
        field: 'km_uuid',
        type: DataTypes.STRING(40),
        allowNull: false
    },
    AS_CLIENT_ID: {
        field: 'as_client_id',
        type: DataTypes.STRING,
        allowNull: true
    },
    TYPE: {
        field: 'type',
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PRODUCTION'
    },
    CREATED_BY: {
        field: 'created_by',
        type: DataTypes.STRING,
        allowNull: false
    },
    CREATED_AT: {
        field: 'created_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
    UPDATED_BY: {
        field: 'updated_by',
        type: DataTypes.STRING,
        allowNull: false
    },
    UPDATED_AT: {
        field: 'updated_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_app_key_mappings',
    returning: true,
    indexes: [
        { name: 'idx_app_key_mappings_app_uuid', fields: ['APP_UUID'] },
        { name: 'idx_app_key_mappings_km_uuid', fields: ['KM_UUID'] },
    ],
});

const SubscriptionMapping = sequelize.define('DP_SUBSCRIPTION', {

    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    CREATED_BY: {
        field: 'created_by',
        type: DataTypes.STRING,
        allowNull: false,
    },
    API_UUID: {
        field: 'api_uuid',
        type: DataTypes.STRING(40),
        allowNull: false,
        references: {
            model: APIMetadata,
            key: 'uuid',
        },
    },
    PLAN_UUID: {
        field: 'plan_uuid',
        type: DataTypes.STRING(40),
        allowNull: true,
        references: {
            model: SubscriptionPlan,
            key: 'uuid',
        },
    },
    ORG_UUID: {
        field: 'org_uuid',
        type: DataTypes.STRING(40),
        allowNull: false
    },
    TOKEN:   { field: 'token', type: DataTypes.STRING(512), allowNull: true, unique: true },
    STATUS:      { field: 'status', type: DataTypes.STRING(20), allowNull: false, defaultValue: 'ACTIVE' },
    CREATED_AT:  { field: 'created_at', type: DataTypes.DATE, allowNull: false, defaultValue: DataTypes.NOW },
    UPDATED_BY:  { field: 'updated_by', type: DataTypes.STRING, allowNull: false },
    UPDATED_AT:  { field: 'updated_at', type: DataTypes.DATE, allowNull: false, defaultValue: DataTypes.NOW },
}, {
    timestamps: false,
    tableName: 'dp_subscription',
    returning: true,
    indexes: [
        { name: 'idx_subscription_org_created_by', fields: ['ORG_UUID', 'CREATED_BY'] },
        { name: 'idx_subscription_org_api_uuid', fields: ['ORG_UUID', 'API_UUID'] },
        { name: 'idx_subscription_plan_uuid', fields: ['PLAN_UUID'] },
        { name: 'idx_subscription_status', fields: ['STATUS'] },
    ],
});

SubscriptionMapping.belongsTo(Organization, {
    foreignKey: 'ORG_UUID'
})
Organization.hasMany(SubscriptionMapping, {
    foreignKey: 'ORG_UUID'
})
APIMetadata.belongsToMany(SubscriptionPlan, {
    through: APISubscriptionPlan,
    foreignKey: "API_UUID",
    otherKey: "PLAN_UUID",
});

SubscriptionPlan.belongsToMany(APIMetadata, {
    through: APISubscriptionPlan,
    foreignKey: "PLAN_UUID",
    otherKey: "API_UUID",
});

SubscriptionMapping.belongsTo(APIMetadata, { foreignKey: 'API_UUID', as: 'DP_API_METADATA' });
SubscriptionMapping.belongsTo(SubscriptionPlan, { foreignKey: 'PLAN_UUID', as: 'DP_SUBSCRIPTION_PLAN' });

Application.belongsTo(Organization, {
    foreignKey: 'ORG_UUID'
})
Organization.hasMany(Application, {
    foreignKey: 'ORG_UUID'
})
ApplicationKeyMapping.belongsTo(Application, {
    foreignKey: 'APP_UUID'
})
Application.hasMany(ApplicationKeyMapping, {
    foreignKey: 'APP_UUID'
})
ApplicationKeyMapping.belongsTo(KeyManager, {
    foreignKey: 'KM_UUID'
})
KeyManager.hasMany(ApplicationKeyMapping, {
    foreignKey: 'KM_UUID'
})

module.exports = {
    Application,
    ApplicationKeyMapping,
    SubscriptionMapping
}
