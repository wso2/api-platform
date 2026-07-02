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
const { DataTypes } = require('sequelize');
const sequelize = require('../db/sequelizeConfig');
const UserIdpReference = require('./userIdpReference');
const { Organization } = require('./organization');

/**
 * Which organizations a user has been seen in. Unlike created_by/updated_by
 * columns, this table has real foreign keys — it's a live membership record,
 * not a "hanging creator" reference — so both sides cascade on delete.
 */
const UserOrganizationMapping = sequelize.define('dp_user_organization_mapping', {
    user_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        primaryKey: true
    },
    org_uuid: {
        type: DataTypes.STRING(40),
        allowNull: false,
        primaryKey: true
    },
}, {
    timestamps: false,
    tableName: 'dp_user_organization_mappings',
    returning: true,
    indexes: [
        {
            name: 'idx_user_organization_mappings_org_uuid',
            fields: ['org_uuid'],
        },
    ],
});

UserOrganizationMapping.belongsTo(UserIdpReference, {
    foreignKey: 'user_uuid',
    onDelete: 'CASCADE',
});

UserIdpReference.hasMany(UserOrganizationMapping, {
    foreignKey: 'user_uuid',
    onDelete: 'CASCADE',
});

UserOrganizationMapping.belongsTo(Organization, {
    foreignKey: 'org_uuid',
    onDelete: 'CASCADE',
});

Organization.hasMany(UserOrganizationMapping, {
    foreignKey: 'org_uuid',
    onDelete: 'CASCADE',
});

module.exports = UserOrganizationMapping;
