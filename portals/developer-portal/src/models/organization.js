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
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    name: {
        type: DataTypes.STRING,
        allowNull: false,
        unique: true
    },
    business_owner: {
        type: DataTypes.STRING,
        allowNull: true,
    },
    business_owner_contact: {
        type: DataTypes.STRING,
        allowNull: true
    },
    business_owner_email: {
        type: DataTypes.STRING,
        allowNull: true
    },
    handle: {
        type: DataTypes.STRING,
        allowNull: false,
        unique: true
    },
    idp_ref_id: {
        type: DataTypes.STRING,
        allowNull: false
    },
    cp_ref_id: {
        type: DataTypes.STRING,
        allowNull: true
    },
    configuration: {
        type: DataTypes.JSONB,
        allowNull: false
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
    tableName: 'dp_organizations',
    returning: true
});

const OrgContent = sequelize.define('DP_ORGANIZATION_ASSET', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    file_name: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    file_content: {
        type: DataTypes.BLOB,
        allowNull: false,
    },
    file_type: {
        type: DataTypes.STRING(20),
        allowNull: false,
    },
    file_path: {
        type: DataTypes.STRING,
        allowNull: false,
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        foreignKey: true,
    },
    view_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
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
    tableName: 'dp_organization_assets',
    indexes: [
        {
            name: 'uq_organization_asset_type_name_path_org_view',
            unique: true,
            fields: ['file_type', 'file_name', 'file_path', 'org_uuid', 'view_uuid']
        },
        {
            name: 'idx_organization_asset_org_uuid',
            fields: ['org_uuid']
        },
        {
            name: 'idx_organization_asset_view_uuid',
            fields: ['view_uuid']
        }
    ]
});

OrgContent.belongsTo(Organization, {
    foreignKey: 'org_uuid',
});

Organization.hasMany(OrgContent, {
    foreignKey: 'org_uuid',
    onDelete: 'CASCADE',
});

View.belongsTo(Organization, {
    foreignKey: 'org_uuid',
});

Organization.hasMany(View, {
    foreignKey: 'org_uuid',
    onDelete: 'CASCADE',
});

View.hasOne(OrgContent, {
    foreignKey: 'view_uuid',
    onDelete: 'CASCADE',
});

OrgContent.belongsTo(View, {
    foreignKey: 'view_uuid',
    onDelete: 'CASCADE'
});

// Export both models
module.exports = {
    Organization,
    OrgContent
};
