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

const APISubscriptionPlan = sequelize.define('dp_api_subscription_plan_mapping', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    api_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    plan_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
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
    tableName: 'dp_api_subscription_plan_mappings',
    returning: true,
    indexes: [
        {
            name: 'uq_api_subscription_plan_mappings_plan_api',
            unique: true,
            fields: ['plan_uuid', 'api_uuid']
        },
        {
            name: 'idx_api_subscription_plan_mappings_api_uuid',
            fields: ['api_uuid']
        }
    ]
});

// APIMetadata<->SubscriptionPlan belongsToMany (through this model) is
// registered once in application.js, where both ends of the association
// already live alongside the rest of the subscription wiring.

module.exports = APISubscriptionPlan;
