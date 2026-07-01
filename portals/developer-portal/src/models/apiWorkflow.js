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

const APIWorkflow = sequelize.define('dp_api_workflow', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    view_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    name: {
        type: DataTypes.STRING,
        allowNull: false
    },
    description: {
        type: DataTypes.STRING(1023),
        allowNull: false
    },
    handle: {
        type: DataTypes.STRING,
        allowNull: false
    },
    agent_prompt: {
        type: DataTypes.BLOB,
        allowNull: false,
        get() {
            return bufferToUtf8(this.getDataValue('agent_prompt'));
        }
    },
    status: {
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PUBLISHED'
    },
    file_content: {
        type: DataTypes.BLOB,
        allowNull: true
    },
    content_type: {
        type: DataTypes.STRING,
        allowNull: true
    },
    agent_visibility: {
        type: DataTypes.STRING,
        allowNull: false,
        defaultValue: 'VISIBLE'
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
    }
}, {
    timestamps: false,
    tableName: 'dp_api_workflows',
    returning: true,
    indexes: [
        { name: 'uq_api_workflow_org_view_handle', unique: true, fields: ['org_uuid', 'view_uuid', 'handle'] },
        { name: 'idx_api_workflow_view_uuid', fields: ['view_uuid'] },
        { name: 'idx_api_workflow_status', fields: ['status'] }
    ]
});

APIWorkflow.belongsTo(Organization, { foreignKey: 'org_uuid' });
Organization.hasMany(APIWorkflow, { foreignKey: 'org_uuid', onDelete: 'CASCADE' });

const View = require('./view');
APIWorkflow.belongsTo(View, { foreignKey: 'view_uuid' });
View.hasMany(APIWorkflow, { foreignKey: 'view_uuid', onDelete: 'CASCADE' });

module.exports = { APIWorkflow };
