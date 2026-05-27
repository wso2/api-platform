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
const sequelize = require('../db/sequelize');
const { Organization } = require('./organization');

const APIFlow = sequelize.define('DP_API_FLOW', {
    API_FLOW_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    VIEW_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    DESCRIPTION: {
        type: DataTypes.TEXT,
        allowNull: false
    },
    HANDLE: {
        type: DataTypes.STRING,
        allowNull: false
    },
    AGENT_PROMPT: {
        type: DataTypes.TEXT,
        allowNull: false
    },
    STATUS: {
        type: DataTypes.ENUM,
        values: ['DRAFT', 'PUBLISHED'],
        allowNull: false,
        defaultValue: 'PUBLISHED'
    },
    FILE_CONTENT: {
        type: DataTypes.BLOB,
        allowNull: true
    },
    CONTENT_TYPE: {
        type: DataTypes.ENUM,
        values: ['ARAZZO', 'MD'],
        allowNull: true
    },
    VISIBILITY: {
        type: DataTypes.ENUM,
        values: ['PUBLIC', 'PRIVATE'],
        allowNull: false,
        defaultValue: 'PUBLIC'
    },
    AGENT_VISIBILITY: {
        type: DataTypes.ENUM,
        values: ['VISIBLE', 'HIDDEN'],
        allowNull: false,
        defaultValue: 'VISIBLE'
    },
    CREATED_AT: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
    UPDATED_AT: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    }
}, {
    timestamps: false,
    tableName: 'DP_API_FLOW',
    returning: true,
    indexes: [
        { unique: true, fields: ['ORG_ID', 'VIEW_ID', 'HANDLE'] }
    ]
});

APIFlow.belongsTo(Organization, { foreignKey: 'ORG_ID' });
Organization.hasMany(APIFlow, { foreignKey: 'ORG_ID', onDelete: 'CASCADE' });

const View = require('./views');
APIFlow.belongsTo(View, { foreignKey: 'VIEW_ID' });
View.hasMany(APIFlow, { foreignKey: 'VIEW_ID', onDelete: 'CASCADE' });

module.exports = { APIFlow };
