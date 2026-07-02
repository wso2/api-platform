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

/**
 * Write-only mutation trail, mirroring platform-api's `audit` table. No FK on
 * performed_by (a dp_user_idp_references uuid) — same "hanging creator" pattern
 * as created_by/updated_by elsewhere, so a deleted user doesn't lose their history.
 */
const Audit = sequelize.define('dp_audit', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    action: {
        type: DataTypes.STRING(50),
        allowNull: false
    },
    resource_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    resource_type: {
        type: DataTypes.STRING(50)
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false
    },
    performed_by: {
        type: DataTypes.STRING(255)
    },
    performed_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_audit',
    returning: true,
    indexes: [
        {
            name: 'idx_audit_org_uuid',
            fields: ['org_uuid'],
        }
    ],
});

Audit.belongsTo(Organization, {
    foreignKey: 'org_uuid',
    onDelete: 'CASCADE',
});

Organization.hasMany(Audit, {
    foreignKey: 'org_uuid',
    onDelete: 'CASCADE',
});

module.exports = Audit;
