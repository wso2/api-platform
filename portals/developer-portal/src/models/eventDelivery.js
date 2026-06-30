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
const DPEvent = require('./event');

// One delivery row per (event × subscriber). ENCRYPTED_FIELDS holds per-subscriber
// ciphertext ({ [fieldName]: envelope }) so plaintext is never in DP_EVENT.
const DPEventDelivery = sequelize.define('DP_EVENT_DELIVERY', {
    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    EVENT_UUID: {
        field: 'event_uuid',
        type: DataTypes.STRING(40),
        allowNull: false,
        references: { model: DPEvent, key: 'uuid' }
    },
    SUBSCRIBER_ID: {
        field: 'subscriber_id',
        type: DataTypes.STRING(128),
        allowNull: false
    },
    TARGET_URL: {
        field: 'target_url',
        type: DataTypes.STRING(1023),
        allowNull: false
    },
    ENCRYPTED_FIELDS: {
        field: 'encrypted_fields',
        type: DataTypes.JSON,
        allowNull: true,
        defaultValue: null
    },
    STATUS: {
        field: 'status',
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PENDING'
    },
    LAST_HTTP_STATUS: {
        field: 'last_http_status',
        type: DataTypes.INTEGER,
        allowNull: true
    },
    LAST_ERROR: {
        field: 'last_error',
        type: DataTypes.STRING,
        allowNull: true
    },
    LAST_ATTEMPT_AT: {
        field: 'last_attempt_at',
        type: DataTypes.DATE,
        allowNull: true
    },
    DELIVERED_AT: {
        field: 'delivered_at',
        type: DataTypes.DATE,
        allowNull: true
    }
}, {
    timestamps: false,
    tableName: 'dp_event_delivery',
    returning: true,
    indexes: [
        { name: 'idx_event_delivery_event_uuid', fields: ['EVENT_UUID'] },
        { name: 'uq_event_delivery_event_subscriber', unique: true, fields: ['EVENT_UUID', 'SUBSCRIBER_ID'] }
    ]
});

DPEventDelivery.belongsTo(DPEvent, { foreignKey: 'EVENT_UUID' });
DPEvent.hasMany(DPEventDelivery, { foreignKey: 'EVENT_UUID' });

module.exports = DPEventDelivery;
