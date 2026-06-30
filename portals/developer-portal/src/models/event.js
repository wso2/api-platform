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
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    type: {
        type: DataTypes.STRING(128),
        allowNull: false
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    aggregate_type: {
        type: DataTypes.STRING(64),
        allowNull: false
    },
    aggregate_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    payload: {
        type: DataTypes.JSONB,
        allowNull: false,
        defaultValue: {}
    },
    occurred_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: DataTypes.NOW
    },
    status: {
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PENDING'
    }
}, {
    timestamps: false,
    tableName: 'dp_event',
    returning: true,
    indexes: [
        { name: 'idx_event_status_occurred_at', fields: ['status', 'occurred_at'] },
        { name: 'idx_event_org_uuid', fields: ['org_uuid'] }
    ]
});

DPEvent.belongsTo(Organization, { foreignKey: 'org_uuid' });
Organization.hasMany(DPEvent, { foreignKey: 'org_uuid', onDelete: 'CASCADE' });

module.exports = DPEvent;
