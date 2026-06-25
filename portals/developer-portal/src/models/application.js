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

    APP_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_ID: {
        type: DataTypes.UUID,
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
    DESCRIPTION: {
        type: DataTypes.STRING,
        allowNull: true
    },
    TYPE: {
        type: DataTypes.STRING,
        allowNull: false
    }
}, {
    timestamps: false,
    tableName: 'DP_APPLICATION',
    returning: true,
    indexes: [
        { name: 'IDX_APPLICATION_ORG_CREATED_BY', fields: ['ORG_ID', 'CREATED_BY'] },
    ],
});

const ApplicationKeyMapping = sequelize.define('DP_APP_KEY_MAPPING', {

    MAPPING_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    APP_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    ORG_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    KM_ID: {
        type: DataTypes.UUID,
        allowNull: true
    },
    AS_CLIENT_ID: {
        type: DataTypes.STRING,
        allowNull: true
    },
    KEY_TYPE: {
        type: DataTypes.STRING,
        allowNull: false,
        defaultValue: 'PRODUCTION'
    },
    ADDITIONAL_PROPERTIES: {
        type: DataTypes.JSON,
        allowNull: true
    }
}, {
    timestamps: false,
    tableName: 'DP_APP_KEY_MAPPING',
    returning: true
});

const SubscriptionMapping = sequelize.define('DP_API_SUBSCRIPTION', {

    SUB_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    CREATED_BY: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    API_ID: {
        type: DataTypes.UUID,
        allowNull: false,
        references: {
            model: APIMetadata,
            key: 'API_ID',
        },
    },
    PLAN_ID: {
        type: DataTypes.UUID,
        allowNull: true,
        references: {
            model: SubscriptionPlan,
            key: 'PLAN_ID',
        },
    },
    ORG_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    SUB_TOKEN:   { type: DataTypes.STRING(512), allowNull: true, unique: true },
    STATUS:      { type: DataTypes.ENUM('ACTIVE', 'INACTIVE'), allowNull: false, defaultValue: 'ACTIVE' },
    CREATED_AT:  { type: DataTypes.DATE, allowNull: false, defaultValue: DataTypes.NOW },
}, {
    timestamps: false,
    tableName: 'DP_API_SUBSCRIPTION',
    returning: true,
    indexes: [
        { name: 'IDX_SUBSCRIPTION_ORG_CREATED_BY', fields: ['ORG_ID', 'CREATED_BY'] },
        { name: 'IDX_SUBSCRIPTION_ORG_API_ID', fields: ['ORG_ID', 'API_ID'] },
    ],
});

SubscriptionMapping.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
})
Organization.hasMany(SubscriptionMapping, {
    foreignKey: 'ORG_ID'
})
APIMetadata.belongsToMany(SubscriptionPlan, {
    through: APISubscriptionPlan,
    foreignKey: "API_ID",
    otherKey: "PLAN_ID",
});

SubscriptionPlan.belongsToMany(APIMetadata, {
    through: APISubscriptionPlan,
    foreignKey: "PLAN_ID",
    otherKey: "API_ID",
});

SubscriptionMapping.belongsTo(APIMetadata, { foreignKey: 'API_ID', as: 'DP_API_METADATA' });
SubscriptionMapping.belongsTo(SubscriptionPlan, { foreignKey: 'PLAN_ID', as: 'DP_SUBSCRIPTION_PLAN' });

Application.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
})
Organization.hasMany(Application, {
    foreignKey: 'ORG_ID'
})
ApplicationKeyMapping.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
})
Organization.hasMany(ApplicationKeyMapping, {
    foreignKey: 'ORG_ID'
})
ApplicationKeyMapping.belongsTo(Application, {
    foreignKey: 'APP_ID'
})
Application.hasMany(ApplicationKeyMapping, {
    foreignKey: 'APP_ID'
})
ApplicationKeyMapping.belongsTo(KeyManager, {
    foreignKey: 'KM_ID'
})
KeyManager.hasMany(ApplicationKeyMapping, {
    foreignKey: 'KM_ID'
})

module.exports = {
    Application,
    ApplicationKeyMapping,
    SubscriptionMapping
}

