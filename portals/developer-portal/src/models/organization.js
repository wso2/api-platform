/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const View = require('./view');

const Organization = sequelize.define('DP_ORGANIZATION', {
    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    NAME: {
        field: 'name',
        type: DataTypes.STRING,
        allowNull: false,
        unique: true
    },
    BUSINESS_OWNER: {
        field: 'business_owner',
        type: DataTypes.STRING,
        allowNull: true,
    },
    BUSINESS_OWNER_CONTACT: {
        field: 'business_owner_contact',
        type: DataTypes.STRING,
        allowNull: true
    },
    BUSINESS_OWNER_EMAIL: {
        field: 'business_owner_email',
        type: DataTypes.STRING,
        allowNull: true
    },
    HANDLE: {
        field: 'handle',
        type: DataTypes.STRING,
        allowNull: false,
        unique: true
    },
    IDP_REF_ID: {
        field: 'idp_ref_id',
        type: DataTypes.STRING,
        allowNull: false
    },
    CP_REF_ID: {
        field: 'cp_ref_id',
        type: DataTypes.STRING,
        allowNull: true
    },
    CONFIGURATION: {
        field: 'configuration',
        type: DataTypes.JSONB,
        allowNull: false
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
    tableName: 'dp_organization',
    returning: true
});

const OrgContent = sequelize.define('DP_ORGANIZATION_ASSET', {
    UUID: {
        field: 'uuid',
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    FILE_NAME: {
        field: 'file_name',
        type: DataTypes.STRING,
        allowNull: false,
    },
    FILE_CONTENT: {
        field: 'file_content',
        type: DataTypes.BLOB,
        allowNull: false,
    },
    FILE_TYPE: {
        field: 'file_type',
        type: DataTypes.STRING(20),
        allowNull: false,
    },
    FILE_PATH: {
        field: 'file_path',
        type: DataTypes.STRING,
        allowNull: false,
    },
    ORG_UUID: {
        field: 'org_uuid',
        type: DataTypes.STRING(40),
        allowNull: false,
        foreignKey: true,
    },
    VIEW_UUID: {
        field: 'view_uuid',
        type: DataTypes.STRING(40),
        allowNull: false,
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
    tableName: 'dp_organization_asset',
    indexes: [
        {
            name: 'uq_organization_asset_type_name_path_org_view',
            unique: true,
            fields: ['FILE_TYPE', 'FILE_NAME', 'FILE_PATH', 'ORG_UUID', 'VIEW_UUID']
        },
        {
            name: 'idx_organization_asset_org_uuid',
            fields: ['ORG_UUID']
        },
        {
            name: 'idx_organization_asset_view_uuid',
            fields: ['VIEW_UUID']
        }
    ]
});

OrgContent.belongsTo(Organization, {
    foreignKey: 'ORG_UUID',
});

Organization.hasMany(OrgContent, {
    foreignKey: 'ORG_UUID',
    onDelete: 'CASCADE',
});

View.belongsTo(Organization, {
    foreignKey: 'ORG_UUID',
});

Organization.hasMany(View, {
    foreignKey: 'ORG_UUID',
    onDelete: 'CASCADE',
});

View.hasOne(OrgContent, {
    foreignKey: 'VIEW_UUID',
    onDelete: 'CASCADE',
});

OrgContent.belongsTo(View, {
    foreignKey: 'VIEW_UUID',
    onDelete: 'CASCADE'
});

// Export both models
module.exports = {
    Organization,
    OrgContent
};
