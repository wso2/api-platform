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

/**
 * A durable record per distinct IDP `sub` claim. Referenced by uuid from
 * created_by/updated_by-style columns elsewhere WITHOUT a foreign key, so
 * those columns can keep pointing at a uuid after the reference row here
 * is deleted (resolved to "deleted_user" at read time).
 */
const UserIdpReference = sequelize.define('dp_user_idp_reference', {
    uuid: {
        type: DataTypes.STRING(40),
        defaultValue: Sequelize.UUIDV4,
        primaryKey: true
    },
    idp_id: {
        type: DataTypes.STRING(255),
        allowNull: false,
        unique: true
    },
    created_at: {
        type: DataTypes.DATE,
        allowNull: false,
        defaultValue: Sequelize.NOW
    },
}, {
    timestamps: false,
    tableName: 'dp_user_idp_references',
    returning: true
});

module.exports = UserIdpReference;
