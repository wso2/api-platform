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
const View = require('./view');
const Labels = require('./label');


const ViewLabels = sequelize.define('DP_VIEW_LABEL_MAPPING', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    view_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: {
            model: View,
            key: 'uuid',
        },
    },
    label_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: {
            model: Labels,
            key: 'uuid',
        }
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
}, {
    timestamps: false,
    tableName: 'dp_view_label_mappings',
    returning: true,
    indexes: [
        {
            name: 'uq_view_label_mappings_label_view',
            unique: true,
            fields: ['label_uuid', 'view_uuid']
        },
        {
            name: 'idx_view_label_mappings_view_uuid',
            fields: ['view_uuid']
        }
    ]
});

View.belongsToMany(Labels, {
    through: ViewLabels,
    foreignKey: "view_uuid",
    otherKey: "label_uuid",
});
Labels.belongsToMany(View, {
    through: ViewLabels,
    foreignKey: "label_uuid",
    otherKey: "view_uuid",
});

module.exports = ViewLabels;
