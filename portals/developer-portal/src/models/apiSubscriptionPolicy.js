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
const sequelize = require('../db/sequelize');
const SubscriptionPolicy = require('./subscriptionPolicy');
const { APIMetadata } = require('./apiMetadata');

const APISubscriptionPolicy = sequelize.define('DP_API_SUBSCRIPTION_POLICY', {
    API_ID: {
        type: DataTypes.UUID,
        allowNull: false,
        primaryKey: true
    },
    POLICY_ID: {
        type: DataTypes.UUID,
        allowNull: false,
        primaryKey: true
    }
}, {
    timestamps: false,
    tableName: 'DP_API_SUBSCRIPTION_POLICY',
    returning: true
});

APIMetadata.belongsToMany(SubscriptionPolicy, {
    foreignKey: 'API_ID',
    otherKey: 'POLICY_ID',
    through: APISubscriptionPolicy
});

SubscriptionPolicy.belongsToMany(APIMetadata, {
    foreignKey: 'POLICY_ID',
    otherKey: 'API_ID',
    through: APISubscriptionPolicy
});

module.exports = APISubscriptionPolicy;
