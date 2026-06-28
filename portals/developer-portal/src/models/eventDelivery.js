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
    UUID: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    EVENT_UUID: {
        type: DataTypes.STRING(40),
        allowNull: false,
        references: { model: DPEvent, key: 'UUID' }
    },
    SUBSCRIBER_ID: {
        type: DataTypes.STRING(128),
        allowNull: false
    },
    TARGET_URL: {
        type: DataTypes.STRING(1023),
        allowNull: false
    },
    ENCRYPTED_FIELDS: {
        type: DataTypes.JSONB,
        allowNull: true,
        defaultValue: null
    },
    STATUS: {
        type: DataTypes.STRING(20),
        allowNull: false,
        defaultValue: 'PENDING'
    },
    LAST_HTTP_STATUS: {
        type: DataTypes.INTEGER,
        allowNull: true
    },
    LAST_ERROR: {
        type: DataTypes.STRING,
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
        { name: 'IDX_EVENT_DELIVERY_EVENT_UUID', fields: ['EVENT_UUID'] },
        { name: 'UQ_EVENT_DELIVERY_EVENT_SUBSCRIBER', unique: true, fields: ['EVENT_UUID', 'SUBSCRIBER_ID'] }
    ]
});

DPEventDelivery.belongsTo(DPEvent, { foreignKey: 'EVENT_UUID' });
DPEvent.hasMany(DPEventDelivery, { foreignKey: 'EVENT_UUID' });

module.exports = DPEventDelivery;
