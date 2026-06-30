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

const KeyManager = sequelize.define('DP_KEY_MANAGER', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    name: {
        type: DataTypes.STRING,
        allowNull: false
    },
    type: {
        type: DataTypes.STRING(64),
        allowNull: false
    },
    enabled: {
        type: DataTypes.SMALLINT,
        allowNull: false,
        defaultValue: 1
    },
    token_endpoint: {
        type: DataTypes.STRING,
        allowNull: false
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
    tableName: 'dp_key_managers',
    returning: true,
    indexes: [
        {
            name: 'uq_key_manager_org_name',
            unique: true,
            fields: ['org_uuid', 'name']
        }
    ]
});

KeyManager.belongsTo(Organization, {
    foreignKey: 'org_uuid'
});
Organization.hasMany(KeyManager, {
    foreignKey: 'org_uuid',
    onDelete: 'CASCADE'
});

module.exports = { KeyManager };
