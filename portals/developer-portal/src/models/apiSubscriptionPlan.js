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
const { DataTypes } = require('sequelize');
const sequelize = require('../db/sequelizeConfig');
const SubscriptionPlan = require('./subscriptionPlan');
const { APIMetadata } = require('./apiMetadata');

const APISubscriptionPlan = sequelize.define('DP_API_SUBSCRIPTION_PLAN_MAPPING', {
    API_ID: {
        type: DataTypes.UUID,
        allowNull: false,
        primaryKey: true
    },
    PLAN_ID: {
        type: DataTypes.UUID,
        allowNull: false,
        primaryKey: true
    },
}, {
    timestamps: false,
    tableName: 'DP_API_SUBSCRIPTION_PLAN_MAPPING',
    returning: true
});

APIMetadata.belongsToMany(SubscriptionPlan, {
    foreignKey: 'API_ID',
    otherKey: 'PLAN_ID',
    through: APISubscriptionPlan
});

SubscriptionPlan.belongsToMany(APIMetadata, {
    foreignKey: 'PLAN_ID',
    otherKey: 'API_ID',
    through: APISubscriptionPlan
});

module.exports = APISubscriptionPlan;
