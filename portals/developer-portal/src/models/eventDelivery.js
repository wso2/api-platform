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
// ciphertext (e.g. encrypted_key for apikey.* events) so plaintext is never in DP_EVENT.
const DPEventDelivery = sequelize.define('DP_EVENT_DELIVERY', {
    ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    EVENT_ID: {
        type: DataTypes.UUID,
        allowNull: false,
        references: { model: DPEvent, key: 'ID' }
    },
    SUBSCRIBER_ID: {
        type: DataTypes.STRING(128),
        allowNull: false
    },
    TARGET_URL: {
        type: DataTypes.TEXT,
        allowNull: false
    },
    ENCRYPTED_FIELDS: {
        type: DataTypes.JSON,
        allowNull: true,
        defaultValue: null
    },
    STATUS: {
        type: DataTypes.ENUM('PENDING', 'IN_FLIGHT', 'DELIVERED', 'FAILED', 'DEAD_LETTERED'),
        allowNull: false,
        defaultValue: 'PENDING'
    },
    ATTEMPT_COUNT: {
        type: DataTypes.INTEGER,
        allowNull: false,
        defaultValue: 0
    },
    NEXT_ATTEMPT_AT: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: DataTypes.NOW
    },
    LAST_HTTP_STATUS: {
        type: DataTypes.INTEGER,
        allowNull: true
    },
    LAST_ERROR: {
        type: DataTypes.TEXT,
        allowNull: true
    },
    LAST_ATTEMPT_AT: {
        type: DataTypes.DATE,
        allowNull: true
    },
    DELIVERED_AT: {
        type: DataTypes.DATE,
        allowNull: true
    }
}, {
    timestamps: false,
    tableName: 'DP_EVENT_DELIVERY',
    returning: true,
    indexes: [
        { name: 'IDX_EVENT_DELIVERY_STATUS_NEXT_ATTEMPT', fields: ['STATUS', 'NEXT_ATTEMPT_AT'] },
        { name: 'IDX_EVENT_DELIVERY_EVENT_ID', fields: ['EVENT_ID'] },
        { name: 'UQ_EVENT_DELIVERY_EVENT_SUBSCRIBER', unique: true, fields: ['EVENT_ID', 'SUBSCRIBER_ID'] }
    ]
});

DPEventDelivery.belongsTo(DPEvent, { foreignKey: 'EVENT_ID' });
DPEvent.hasMany(DPEventDelivery, { foreignKey: 'EVENT_ID' });

module.exports = DPEventDelivery;
