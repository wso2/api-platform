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
const SubscriptionPolicy = require('./subscriptionPolicy');
const APISubscriptionPolicy = require('./apiSubscriptionPolicy');


const Application = sequelize.define('DP_APPLICATION', {

    APP_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4
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
        type: DataTypes.TEXT,
        allowNull: true,
        get() {
            const raw = this.getDataValue('ADDITIONAL_PROPERTIES');
            return raw ? JSON.parse(raw) : null;
        },
        set(val) {
            this.setDataValue('ADDITIONAL_PROPERTIES', val ? JSON.stringify(val) : null);
        }
    }
}, {
    timestamps: false,
    tableName: 'DP_APP_KEY_MAPPING',
    returning: true
},
);

const SubscriptionMapping = sequelize.define('DP_API_SUBSCRIPTION', {

    SUB_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    APP_ID: {
        type: DataTypes.UUID,
        allowNull: true,
        references: {
            model: Application,
            key: 'APP_ID',
        },
    },
    API_ID: {
        type: DataTypes.UUID,
        allowNull: false,
        references: {
            model: APIMetadata,
            key: 'API_ID',
        },
    },
    POLICY_ID: {
        type: DataTypes.UUID,
        allowNull: true,
        references: {
            model: SubscriptionPolicy,
            key: 'POLICY_ID',
        },
    },
    ORG_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    SUB_TOKEN: { type: DataTypes.STRING(512), allowNull: true, unique: true },
    STATUS:     { type: DataTypes.STRING(32), allowNull: false, defaultValue: 'ACTIVE' },
}, {
    timestamps: false,
    tableName: 'DP_API_SUBSCRIPTION',
    returning: true
},
);

SubscriptionMapping.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
})
Organization.hasMany(SubscriptionMapping, {
    foreignKey: 'ORG_ID'
})
Application.belongsToMany(APIMetadata, {
    through: SubscriptionMapping,
    foreignKey: "APP_ID",
    otherKey: "API_ID",
});
APIMetadata.belongsToMany(Application, {
    through: SubscriptionMapping,
    foreignKey: "API_ID",
    otherKey: "APP_ID",
});

APIMetadata.belongsToMany(SubscriptionPolicy, {
    through: APISubscriptionPolicy,
    foreignKey: "API_ID",
    otherKey: "POLICY_ID",
});

SubscriptionPolicy.belongsToMany(APIMetadata, {
    through: APISubscriptionPolicy,
    foreignKey: "POLICY_ID",
    otherKey: "API_ID",
});

SubscriptionMapping.belongsTo(APIMetadata, { foreignKey: 'API_ID', as: 'DP_API_METADATA' });
SubscriptionMapping.belongsTo(SubscriptionPolicy, { foreignKey: 'POLICY_ID', as: 'DP_SUBSCRIPTION_POLICY' });

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

module.exports = {
    Application,
    ApplicationKeyMapping,
    SubscriptionMapping
}

