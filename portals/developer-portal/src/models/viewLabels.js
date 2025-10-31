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
const { Organization } = require('./organization');
const View = require('./views');
const Labels = require('./labels');


const ViewLabels = sequelize.define('DP_VIEW_LABELS', {
    ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4
    },
    VIEW_ID: {
        type: DataTypes.UUID,
        references: {
            model: View,
            key: 'VIEW_ID',
        },
    },
    LABEL_ID: {
        type: DataTypes.UUID,
        references: {
            model: Labels,
            key: 'LABEL_ID',
        }
    }
}, {
    timestamps: false,
    tableName: 'DP_VIEW_LABELS',
    returning: true,
    unique: false
});

ViewLabels.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
});

View.belongsToMany(Labels, {
    through: ViewLabels,
    foreignKey: "VIEW_ID",
    otherKey: "LABEL_ID",
});
Labels.belongsToMany(View, {
    through: ViewLabels,
    foreignKey: "LABEL_ID",
    otherKey: "VIEW_ID",
});

module.exports = ViewLabels;

