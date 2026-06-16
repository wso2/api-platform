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
const constants = require('../utils/constants');

const KeyManager = sequelize.define('DP_KEY_MANAGER', {
    KM_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    TYPE: {
        type: DataTypes.ENUM,
        values: [
            constants.KEY_MANAGER_TYPES.ASGARDEO,
            constants.KEY_MANAGER_TYPES.WSO2IS,
            constants.KEY_MANAGER_TYPES.KEYCLOAK,
            constants.KEY_MANAGER_TYPES.GENERIC_OIDC
        ],
        allowNull: false
    },
    ENABLED: {
        type: DataTypes.BOOLEAN,
        allowNull: false,
        defaultValue: true
    },
    TOKEN_ENDPOINT: {
        type: DataTypes.STRING,
        allowNull: false
    },
    CLIENT_REG_ENDPOINT: {
        type: DataTypes.STRING,
        allowNull: false
    },
    ISSUER: {
        type: DataTypes.STRING,
        allowNull: true
    },
    JWKS_URL: {
        type: DataTypes.STRING,
        allowNull: true
    },
    ADMIN_CLIENT_ID_ENC: {
        type: DataTypes.TEXT,
        allowNull: false
    },
    ADMIN_CLIENT_SECRET_ENC: {
        type: DataTypes.TEXT,
        allowNull: false
    },
    SUPPORTED_GRANT_TYPES: {
        type: DataTypes.JSON,
        allowNull: true,
        defaultValue: ['client_credentials']
    },
    SUPPORTED_SCOPES: {
        type: DataTypes.JSON,
        allowNull: true,
        defaultValue: ['openid']
    },
    ADDITIONAL_PROPERTIES: {
        type: DataTypes.JSON,
        allowNull: true,
        defaultValue: {}
    }
}, {
    timestamps: false,
    tableName: 'DP_KEY_MANAGER',
    returning: true,
    indexes: [
        {
            name: 'UQ_KEY_MANAGER_ORG_NAME',
            unique: true,
            fields: ['ORG_ID', 'NAME']
        }
    ]
});

KeyManager.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
});
Organization.hasMany(KeyManager, {
    foreignKey: 'ORG_ID',
    onDelete: 'CASCADE'
});

module.exports = { KeyManager };
