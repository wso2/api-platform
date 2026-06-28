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
const { bufferToUtf8 } = require('../utils/cryptoUtil');

const APIFlow = sequelize.define('DP_API_WORKFLOW', {
    UUID: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    VIEW_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    DESCRIPTION: {
        type: DataTypes.STRING,
        allowNull: false
    },
    HANDLE: {
        type: DataTypes.STRING,
        allowNull: false
    },
    AGENT_PROMPT: {
        type: DataTypes.BLOB,
        allowNull: false,
        get() {
            return bufferToUtf8(this.getDataValue('AGENT_PROMPT'));
        }
    },
    STATUS: {
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PUBLISHED'
    },
    FILE_CONTENT: {
        type: DataTypes.BLOB,
        allowNull: true
    },
    CONTENT_TYPE: {
        type: DataTypes.STRING,
        allowNull: true
    },
    AGENT_VISIBILITY: {
        type: DataTypes.STRING,
        allowNull: false,
        defaultValue: 'VISIBLE'
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
    }
}, {
    timestamps: false,
    tableName: 'DP_API_WORKFLOW',
    returning: true,
    indexes: [
        { name: 'UQ_API_WORKFLOW_ORG_VIEW_HANDLE', unique: true, fields: ['ORG_UUID', 'VIEW_UUID', 'HANDLE'] },
        { name: 'IDX_API_WORKFLOW_VIEW_UUID', fields: ['VIEW_UUID'] },
        { name: 'IDX_API_WORKFLOW_STATUS', fields: ['STATUS'] }
    ]
});

APIFlow.belongsTo(Organization, { foreignKey: 'ORG_UUID' });
Organization.hasMany(APIFlow, { foreignKey: 'ORG_UUID', onDelete: 'CASCADE' });

const View = require('./view');
APIFlow.belongsTo(View, { foreignKey: 'VIEW_UUID' });
View.hasMany(APIFlow, { foreignKey: 'VIEW_UUID', onDelete: 'CASCADE' });

module.exports = { APIFlow };
