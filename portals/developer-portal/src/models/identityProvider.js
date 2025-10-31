/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const sequelize = require('../db/sequelize')
const { Organization } = require('./organization')

const IdentityProvider = sequelize.define('DP_IDENTITY_PROVIDER', {
    ORG_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    ISSUER: {
        type: DataTypes.STRING,
        allowNull: false
    },
    AUTHORIZATION_URL: {
        type: DataTypes.STRING,
        allowNull: false
    },
    TOKEN_URL: {
        type: DataTypes.STRING,
        allowNull: false
    },
    USER_INFOR_URL: {
        type: DataTypes.STRING,
        allowNull: true
    },
    CLIENT_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    CALLBACK_URL: {
        type: DataTypes.STRING,
        allowNull: false
    },
    SCOPE: {
        type: DataTypes.STRING,
        allowNull: false
    },
    SIGNUP_URL: {
        type: DataTypes.STRING,
        allowNull: true
    },
    LOGOUT_URL: {
        type: DataTypes.STRING,
        allowNull: false
    },
    LOGOUT_REDIRECT_URL: {
        type: DataTypes.STRING,
        allowNull: false
    },
    JWKS_URL: {
        type: DataTypes.STRING,
        allowNull: true
    },
    CERTIFICATE: {
        type: DataTypes.STRING,
        allowNull: true
    }
},{
        timestamps: false,
        tableName: 'DP_IDENTITY_PROVIDER',
        returning: true
});

IdentityProvider.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
})
Organization.hasMany(IdentityProvider, {
    foreignKey: 'ORG_ID',
    onDelete: 'CASCADE',
});

module.exports = {
    IdentityProvider
};
