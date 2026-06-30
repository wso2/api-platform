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
    VIEW_UUID: {
        field: 'view_uuid',
        type: DataTypes.STRING(40),
        allowNull: false
    },
    NAME: {
        field: 'name',
        type: DataTypes.STRING,
        allowNull: false
    },
    DESCRIPTION: {
        field: 'description',
        type: DataTypes.STRING(1023),
        allowNull: false
    },
    HANDLE: {
        field: 'handle',
        type: DataTypes.STRING,
        allowNull: false
    },
    AGENT_PROMPT: {
        field: 'agent_prompt',
        type: DataTypes.BLOB,
        allowNull: false,
        get() {
            return bufferToUtf8(this.getDataValue('AGENT_PROMPT'));
        }
    },
    STATUS: {
        field: 'status',
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PUBLISHED'
    },
    FILE_CONTENT: {
        field: 'file_content',
        type: DataTypes.BLOB,
        allowNull: true
    },
    CONTENT_TYPE: {
        field: 'content_type',
        type: DataTypes.STRING,
        allowNull: true
    },
    AGENT_VISIBILITY: {
        field: 'agent_visibility',
        type: DataTypes.STRING,
        allowNull: false,
        defaultValue: 'VISIBLE'
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
    }
}, {
    timestamps: false,
    tableName: 'dp_api_workflow',
    returning: true,
    indexes: [
        { name: 'uq_api_workflow_org_view_handle', unique: true, fields: ['ORG_UUID', 'VIEW_UUID', 'HANDLE'] },
        { name: 'idx_api_workflow_view_uuid', fields: ['VIEW_UUID'] },
        { name: 'idx_api_workflow_status', fields: ['STATUS'] }
    ]
});

APIFlow.belongsTo(Organization, { foreignKey: 'ORG_UUID' });
Organization.hasMany(APIFlow, { foreignKey: 'ORG_UUID', onDelete: 'CASCADE' });

const View = require('./view');
APIFlow.belongsTo(View, { foreignKey: 'VIEW_UUID' });
View.hasMany(APIFlow, { foreignKey: 'VIEW_UUID', onDelete: 'CASCADE' });

module.exports = { APIFlow };
