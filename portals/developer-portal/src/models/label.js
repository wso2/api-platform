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


const Labels = sequelize.define('DP_LABEL', {

    UUID: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false,
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    DISPLAY_NAME: {
        type: DataTypes.STRING,
        allowNull: false
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
    },
}, {
    timestamps: false,
    tableName: 'DP_LABEL',
    returning: true,
    indexes: [
        {
            name: 'UQ_LABEL_NAME_ORG_UUID',
            unique: true,
            fields: ['NAME', 'ORG_UUID'],
        }
    ],
});

Labels.belongsTo(Organization, {
    foreignKey: 'ORG_UUID'
})

module.exports = Labels;

