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
const { bufferToUtf8 } = require('../utils/cryptoUtil');

const WebhookSubscriber = sequelize.define('DP_WEBHOOK_SUBSCRIBER', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    name: {
        type: DataTypes.STRING,
        allowNull: false
    },
    target_url: {
        type: DataTypes.STRING(1023),
        allowNull: false
    },
    secret_enc: {
        type: DataTypes.BLOB,
        allowNull: true,
        get() {
            return bufferToUtf8(this.getDataValue('secret_enc'));
        }
    },
    public_key: {
        type: DataTypes.BLOB,
        allowNull: true,
        get() {
            return bufferToUtf8(this.getDataValue('public_key'));
        }
    },
    event_patterns: {
        type: DataTypes.JSONB,
        allowNull: true,
        defaultValue: []
    },
    enabled: {
        type: DataTypes.SMALLINT,
        allowNull: false,
        defaultValue: 1
    },
    timeout_ms: {
        type: DataTypes.INTEGER,
        allowNull: false,
        defaultValue: 5000
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
    tableName: 'dp_webhook_subscriber',
    returning: true,
    indexes: [
        {
            name: 'uq_webhook_subscriber_org_name',
            unique: true,
            fields: ['org_uuid', 'name']
        }
    ]
});

WebhookSubscriber.belongsTo(Organization, {
    foreignKey: 'org_uuid'
});
Organization.hasMany(WebhookSubscriber, {
    foreignKey: 'org_uuid',
    onDelete: 'CASCADE'
});

module.exports = { WebhookSubscriber };
