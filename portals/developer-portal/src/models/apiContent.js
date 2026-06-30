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
const sequelize = require('../db/sequelizeConfig')

const APIContent = sequelize.define('DP_API_CONTENT', {
    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    API_UUID: {
        field: 'api_uuid',
        type: DataTypes.STRING(40),
        allowNull: false,
    },
    FILE_CONTENT: {
        field: 'file_content',
        type: DataTypes.BLOB,
        allowNull: false,
    },
    TYPE: {
        field: 'type',
        type: DataTypes.STRING(64),
        allowNull: false,
    },
    FILE_NAME: {
        field: 'file_name',
        type: DataTypes.STRING,
        allowNull: false,
    },
    LOOKUP_KEY: {
        field: 'lookup_key',
        type: DataTypes.STRING,
        allowNull: true,
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
    },
}, {
    timestamps: false,
    tableName: 'dp_api_content',
    returning: true,
    indexes: [
        {
            name: 'uq_api_content_api_type_file_name',
            unique: true,
            fields: ['API_UUID', 'TYPE', 'FILE_NAME']
        },
        {
            name: 'uq_api_content_api_type_lookup_key',
            unique: true,
            fields: ['API_UUID', 'TYPE', 'LOOKUP_KEY']
        }
    ]
});

module.exports = APIContent;
