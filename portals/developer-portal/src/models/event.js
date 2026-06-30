/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

// Outbox table — one row per domain event. Payload never contains plaintext key secrets.
const DPEvent = sequelize.define('DP_EVENT', {
    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    TYPE: {
        field: 'type',
        type: DataTypes.STRING(128),
        allowNull: false
    },
    ORG_UUID: {
        field: 'org_uuid',
        type: DataTypes.STRING(40),
        allowNull: false
    },
    AGGREGATE_TYPE: {
        field: 'aggregate_type',
        type: DataTypes.STRING(64),
        allowNull: false
    },
    AGGREGATE_UUID: {
        field: 'aggregate_uuid',
        type: DataTypes.STRING(40),
        allowNull: false
    },
    PAYLOAD: {
        field: 'payload',
        type: DataTypes.JSONB,
        allowNull: false,
        defaultValue: {}
    },
    OCCURRED_AT: {
        field: 'occurred_at',
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: DataTypes.NOW
    },
    STATUS: {
        field: 'status',
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PENDING'
    }
}, {
    timestamps: false,
    tableName: 'dp_event',
    returning: true,
    indexes: [
        { name: 'idx_event_status_occurred_at', fields: ['STATUS', 'OCCURRED_AT'] },
        { name: 'idx_event_org_uuid', fields: ['ORG_UUID'] }
    ]
});

DPEvent.belongsTo(Organization, { foreignKey: 'ORG_UUID' });
Organization.hasMany(DPEvent, { foreignKey: 'ORG_UUID', onDelete: 'CASCADE' });

module.exports = DPEvent;
