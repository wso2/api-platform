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
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    CREATED_BY: {
        type: DataTypes.STRING,
        allowNull: false
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    HANDLE: {
        type: DataTypes.STRING,
        allowNull: false
    },
    DESCRIPTION: {
        type: DataTypes.STRING,
        allowNull: true
    },
    CREATED_AT: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
    UPDATED_BY: {
        type: DataTypes.STRING,
        allowNull: false
    },
    UPDATED_AT: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'DP_APPLICATION',
    returning: true,
    indexes: [
        { name: 'IDX_APPLICATION_ORG_CREATED_BY', fields: ['ORG_UUID', 'CREATED_BY'] },
        { name: 'UQ_APPLICATION_ORG_HANDLE', unique: true, fields: ['ORG_UUID', 'HANDLE'] },
    ],
});

const ApplicationKeyMapping = sequelize.define('DP_APP_KEY_MAPPING', {

    UUID: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    APP_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    KM_UUID: {
        type: DataTypes.STRING(40),
        allowNull: true
    },
    AS_CLIENT_ID: {
        type: DataTypes.STRING,
        allowNull: true
    },
    TYPE: {
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PRODUCTION'
    },
    ADDITIONAL_PROPERTIES: {
        type: DataTypes.JSONB,
        allowNull: true
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
    UPDATED_BY: {
        type: DataTypes.STRING,
        allowNull: false
    },
    UPDATED_AT: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'DP_APP_KEY_MAPPING',
    returning: true,
    indexes: [
        { name: 'IDX_APP_KEY_MAPPING_APP_UUID', fields: ['APP_UUID'] },
        { name: 'IDX_APP_KEY_MAPPING_KM_UUID', fields: ['KM_UUID'] },
    ],
});

const SubscriptionMapping = sequelize.define('DP_SUBSCRIPTION', {

    UUID: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    CREATED_BY: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    API_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: {
            model: APIMetadata,
            key: 'UUID',
        },
    },
    PLAN_UUID: {
        type: DataTypes.STRING(40),
        allowNull: true,
        references: {
            model: SubscriptionPlan,
            key: 'UUID',
        },
    },
    ORG_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    TOKEN:   { type: DataTypes.STRING(512), allowNull: true, unique: true },
    STATUS:      { type: DataTypes.STRING(20), allowNull: false, defaultValue: 'ACTIVE' },
    CREATED_AT:  { type: DataTypes.DATE, allowNull: false, defaultValue: DataTypes.NOW },
    UPDATED_BY:  { type: DataTypes.STRING, allowNull: false },
    UPDATED_AT:  { type: DataTypes.DATE, allowNull: false, defaultValue: DataTypes.NOW },
}, {
    timestamps: false,
    tableName: 'DP_SUBSCRIPTION',
    returning: true,
    indexes: [
        { name: 'IDX_SUBSCRIPTION_ORG_CREATED_BY', fields: ['ORG_UUID', 'CREATED_BY'] },
        { name: 'IDX_SUBSCRIPTION_ORG_API_UUID', fields: ['ORG_UUID', 'API_UUID'] },
        { name: 'IDX_SUBSCRIPTION_PLAN_UUID', fields: ['PLAN_UUID'] },
        { name: 'IDX_SUBSCRIPTION_STATUS', fields: ['STATUS'] },
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

