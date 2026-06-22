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

const WebhookSubscriber = sequelize.define('DP_WEBHOOK_SUBSCRIBER', {
    SUBSCRIBER_ID: {
        type: DataTypes.UUID,
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    ORG_ID: {
        type: DataTypes.UUID,
        allowNull: false
    },
    NAME: {
        type: DataTypes.STRING,
        allowNull: false
    },
    TARGET_URL: {
        type: DataTypes.TEXT,
        allowNull: false
    },
    SECRET_ENC: {
        type: DataTypes.TEXT,
        allowNull: false
    },
    PUBLIC_KEY: {
        type: DataTypes.TEXT,
        allowNull: true
    },
    GATEWAY_TYPE: {
        type: DataTypes.STRING,
        allowNull: true
    },
    EVENT_PATTERNS: {
        type: DataTypes.JSON,
        allowNull: true,
        defaultValue: []
    },
    ENABLED: {
        type: DataTypes.BOOLEAN,
        allowNull: false,
        defaultValue: true
    },
    TIMEOUT_MS: {
        type: DataTypes.INTEGER,
        allowNull: false,
        defaultValue: 5000
    }
}, {
    timestamps: false,
    tableName: 'DP_WEBHOOK_SUBSCRIBER',
    returning: true,
    indexes: [
        {
            name: 'UQ_WEBHOOK_SUBSCRIBER_ORG_NAME',
            unique: true,
            fields: ['ORG_ID', 'NAME']
        }
    ]
});

WebhookSubscriber.belongsTo(Organization, {
    foreignKey: 'ORG_ID'
});
Organization.hasMany(WebhookSubscriber, {
    foreignKey: 'ORG_ID',
    onDelete: 'CASCADE'
});

module.exports = { WebhookSubscriber };
