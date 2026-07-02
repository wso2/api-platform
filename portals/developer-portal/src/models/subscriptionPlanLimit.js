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

const SubscriptionPlanLimit = sequelize.define('dp_subscription_plan_limit', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        allowNull: false,
        primaryKey: true
    },
    plan_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: { model: 'dp_subscription_plans', key: 'uuid' }
    },
    limit_type: {
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'REQUEST_COUNT'
    },
    time_unit: {
        type: DataTypes.STRING(20),
        allowNull: true
    },
    time_amount: {
        type: DataTypes.INTEGER,
        allowNull: false,
        defaultValue: 1
    },
    limit_count: {
        type: DataTypes.BIGINT,
        allowNull: false
    },
}, {
    timestamps: false,
    tableName: 'dp_subscription_plan_limits',
    indexes: [
        { name: 'idx_dp_subscription_plan_limits_plan', fields: ['plan_uuid'] },
        {
            name: 'uq_dp_subscription_plan_limits',
            unique: true,
            fields: ['plan_uuid', 'limit_type', 'time_amount', 'time_unit'],
            where: { time_unit: { [Sequelize.Op.ne]: null } }
        },
        {
            name: 'uq_dp_subscription_plan_limits_null_unit',
            unique: true,
            fields: ['plan_uuid', 'limit_type', 'time_amount'],
            where: { time_unit: null }
        }
    ]
});

module.exports = SubscriptionPlanLimit;
