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
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        allowNull: false,
        primaryKey: true
    },
    HANDLE: {
        field: 'handle',
        type: DataTypes.STRING,
        allowNull: false,
        unique: 'unique_org_plan_handle'
    },
    NAME: {
        field: 'name',
        type: DataTypes.STRING,
        allowNull: false
    },
    DESCRIPTION: {
        field: 'description',
        type: DataTypes.STRING(1023),
        allowNull: true
    },
    REQUEST_COUNT: {
        field: 'request_count',
        type: DataTypes.STRING,
        allowNull: true
    },
    REF_ID: {
        field: 'ref_id',
        type: DataTypes.STRING,
        allowNull: true
    },
    ORG_UUID: {
        field: 'org_uuid',
        type: DataTypes.STRING(40),
        allowNull: true,
        references: { model: 'dp_organization', key: 'uuid' }
    },
    CREATED_BY: {
        field: 'created_by',
        type: DataTypes.STRING,
        allowNull: false
    },
    CREATED_AT: {
        field: 'created_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
    UPDATED_BY: {
        field: 'updated_by',
        type: DataTypes.STRING,
        allowNull: false
    },
    UPDATED_AT: {
        field: 'updated_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_subscription_plan',
    returning: true,
    indexes: [
        { name: 'uq_subscription_plan_org_handle', unique: true, fields: ['ORG_UUID', 'HANDLE'] }
    ]
});

SubscriptionPlan.belongsTo(Organization, {
    foreignKey: {
        name: 'ORG_UUID',
        unique: 'unique_org_plan_handle'
    }
});

module.exports = SubscriptionPlan;
