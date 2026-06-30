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

    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    created_by: {
        type: DataTypes.STRING,
        allowNull: false
    },
    name: {
        type: DataTypes.STRING,
        allowNull: false
    },
    handle: {
        type: DataTypes.STRING,
        allowNull: false
    },
    description: {
        type: DataTypes.STRING(1023),
        allowNull: true
    },
    created_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
    updated_by: {
        type: DataTypes.STRING,
        allowNull: false
    },
    updated_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_applications',
    returning: true,
    indexes: [
        { name: 'idx_application_org_created_by', fields: ['org_uuid', 'created_by'] },
        { name: 'uq_application_org_handle', unique: true, fields: ['org_uuid', 'handle'] },
    ],
});

const ApplicationKeyMapping = sequelize.define('DP_APP_KEY_MAPPING', {

    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    app_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    km_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    as_client_id: {
        type: DataTypes.STRING,
        allowNull: true
    },
    type: {
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PRODUCTION'
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
    updated_by: {
        type: DataTypes.STRING,
        allowNull: false
    },
    updated_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_app_key_mappings',
    returning: true,
    indexes: [
        { name: 'idx_app_key_mappings_app_uuid', fields: ['app_uuid'] },
        { name: 'idx_app_key_mappings_km_uuid', fields: ['km_uuid'] },
    ],
});

const SubscriptionMapping = sequelize.define('DP_SUBSCRIPTION', {

    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    created_by: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    api_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: {
            model: APIMetadata,
            key: 'uuid',
        },
    },
    plan_uuid: {
        type: DataTypes.STRING(40),
        allowNull: true,
        references: {
            model: SubscriptionPlan,
            key: 'uuid',
        },
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    token:      { type: DataTypes.STRING(512), allowNull: true, unique: true },
    status:     { type: DataTypes.STRING(20), allowNull: false, defaultValue: 'ACTIVE' },
    created_at: { type: DataTypes.DATE, allowNull: false, defaultValue: DataTypes.NOW },
    updated_by: { type: DataTypes.STRING, allowNull: false },
    updated_at: { type: DataTypes.DATE, allowNull: false, defaultValue: DataTypes.NOW },
}, {
    timestamps: false,
    tableName: 'dp_subscriptions',
    returning: true,
    indexes: [
        { name: 'idx_subscription_org_created_by', fields: ['org_uuid', 'created_by'] },
        { name: 'idx_subscription_org_api_uuid', fields: ['org_uuid', 'api_uuid'] },
        { name: 'idx_subscription_plan_uuid', fields: ['plan_uuid'] },
        { name: 'idx_subscription_status', fields: ['status'] },
    ],
});

SubscriptionMapping.belongsTo(Organization, {
    foreignKey: 'org_uuid'
})
Organization.hasMany(SubscriptionMapping, {
    foreignKey: 'org_uuid'
})
APIMetadata.belongsToMany(SubscriptionPlan, {
    through: APISubscriptionPlan,
    foreignKey: "api_uuid",
    otherKey: "plan_uuid",
});

SubscriptionPlan.belongsToMany(APIMetadata, {
    through: APISubscriptionPlan,
    foreignKey: "plan_uuid",
    otherKey: "api_uuid",
});

SubscriptionMapping.belongsTo(APIMetadata, { foreignKey: 'api_uuid', as: 'DP_API_METADATA' });
SubscriptionMapping.belongsTo(SubscriptionPlan, { foreignKey: 'plan_uuid', as: 'DP_SUBSCRIPTION_PLAN' });

Application.belongsTo(Organization, {
    foreignKey: 'org_uuid'
})
Organization.hasMany(Application, {
    foreignKey: 'org_uuid'
})
ApplicationKeyMapping.belongsTo(Application, {
    foreignKey: 'app_uuid'
})
Application.hasMany(ApplicationKeyMapping, {
    foreignKey: 'app_uuid'
})
ApplicationKeyMapping.belongsTo(KeyManager, {
    foreignKey: 'km_uuid'
})
KeyManager.hasMany(ApplicationKeyMapping, {
    foreignKey: 'km_uuid'
})

module.exports = {
    Application,
    ApplicationKeyMapping,
    SubscriptionMapping
}
