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
const SubscriptionPlanLimit = require('./subscriptionPlanLimit');

const SubscriptionPlan = sequelize.define('dp_subscription_plan', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        allowNull: false,
        primaryKey: true
    },
    handle: {
        type: DataTypes.STRING,
        allowNull: false,
        unique: 'unique_org_plan_handle'
    },
    display_name: {
        type: DataTypes.STRING,
        allowNull: false
    },
    description: {
        type: DataTypes.STRING(1023),
        allowNull: true
    },
    ref_id: {
        type: DataTypes.STRING,
        allowNull: true
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: true,
        references: { model: 'dp_organizations', key: 'uuid' }
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
    updated_by: {
        type: DataTypes.STRING,
        allowNull: false
    },
    updated_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_subscription_plans',
    returning: true,
    indexes: [
        { name: 'uq_subscription_plan_org_handle', unique: true, fields: ['org_uuid', 'handle'] }
    ]
});

SubscriptionPlan.belongsTo(Organization, {
    foreignKey: {
        name: 'org_uuid',
        unique: 'unique_org_plan_handle'
    }
});

SubscriptionPlan.hasMany(SubscriptionPlanLimit, { foreignKey: 'plan_uuid', as: 'limits' });

module.exports = SubscriptionPlan;
