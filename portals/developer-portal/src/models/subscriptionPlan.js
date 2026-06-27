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
const sequelize = require('../db/sequelizeConfig');
const { Organization } = require('./organization');

const SubscriptionPlan = sequelize.define('DP_SUBSCRIPTION_PLAN', {
    UUID: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        allowNull: false,
        primaryKey: true
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false,
        unique: 'unique_org_plan_name'
    },
    DISPLAY_NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    DESCRIPTION: {
        type: DataTypes.STRING,
        allowNull: true
    },
    REQUEST_COUNT: {
        type: DataTypes.STRING,
        allowNull: true
    },
    REF_ID: {
        type: DataTypes.STRING,
        allowNull: true
    }
}, {
    timestamps: false,
    tableName: 'DP_SUBSCRIPTION_PLAN',
    returning: true,
    indexes: [
        { name: 'IDX_SUB_PLAN_ORG_NAME', unique: true, fields: ['ORG_UUID', 'NAME'] }
    ]
});

SubscriptionPlan.belongsTo(Organization, {
    foreignKey: {
        name: 'ORG_UUID',
        unique: 'unique_org_plan_name'
    }
});

module.exports = SubscriptionPlan;
