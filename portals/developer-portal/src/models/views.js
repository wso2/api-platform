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
const sequelize = require('../db/sequelize');

const View = sequelize.define('DP_VIEWS', {
    VIEW_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        unique: true
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    DISPLAY_NAME: {
        type: DataTypes.STRING,
        allowNull: false
    }
}, {
    timestamps: false,
    tableName: 'DP_VIEWS',
    returning: true
},
    {
        indexes: [
            {
                unique: true,
                fields: ['NAME', 'DISPLAY_NAME', 'ORG_ID']
            }
        ]
    });

module.exports = View;
