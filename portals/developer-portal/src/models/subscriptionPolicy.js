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
const sequelize = require('../db/sequelize');
const { Organization } = require('./organization');

const SubscriptionPolicy = sequelize.define('DP_SUBSCRIPTION_POLICY', {
    POLICY_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        allowNull: false,
        primaryKey: true
    },
    POLICY_NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    DISPLAY_NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    BILLING_PLAN: {
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
    },
    PRICING_MODEL: {
        type: DataTypes.STRING,
        allowNull: true
    },
    CURRENCY: {
        type: DataTypes.STRING,
        allowNull: true
    },
    BILLING_PERIOD: {
        type: DataTypes.STRING,
        allowNull: true
    },
    FLAT_AMOUNT: {
        type: DataTypes.DECIMAL,
        allowNull: true
    },
    UNIT_AMOUNT: {
        type: DataTypes.DECIMAL,
        allowNull: true
    },
    PRICING_METADATA: {
        type: DataTypes.JSONB,
        allowNull: true
    }
}, {
    timestamps: false,
    tableName: 'DP_SUBSCRIPTION_POLICY',
    returning: true
});

SubscriptionPolicy.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
});

module.exports = SubscriptionPolicy;
