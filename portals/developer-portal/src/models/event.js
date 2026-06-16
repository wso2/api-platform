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
const sequelize = require('../db/sequelize');

// Outbox table — one row per domain event. Payload never contains plaintext key secrets.
const DPEvent = sequelize.define('DP_EVENT', {
    EVENT_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    EVENT_TYPE: {
        type: DataTypes.STRING(128),
        allowNull: false
    },
    ORG_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    GATEWAY_TYPE: {
        type: DataTypes.STRING(128),
        allowNull: true
    },
    AGGREGATE_TYPE: {
        type: DataTypes.STRING(64),
        allowNull: false
    },
    AGGREGATE_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    PAYLOAD: {
        type: DataTypes.JSON,
        allowNull: false,
        defaultValue: {}
    },
    OCCURRED_AT: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: DataTypes.NOW
    },
    STATUS: {
        type: DataTypes.ENUM('PENDING', 'DISPATCHED', 'ALL_DELIVERED', 'FAILED'),
        allowNull: false,
        defaultValue: 'PENDING'
    }
}, {
    timestamps: false,
    tableName: 'DP_EVENT',
    returning: true,
    indexes: [
        { name: 'IDX_EVENT_STATUS_OCCURRED_AT', fields: ['STATUS', 'OCCURRED_AT'] }
    ]
});

module.exports = DPEvent;
