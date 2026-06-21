/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const Provider = require('../models/provider');
const { Sequelize } = require('sequelize');
const sequelize = require('../db/sequelizeConfig');

const create = async (orgID, provider, t) => {
    let providerDataList = [];
    for (const [key, value] of Object.entries(provider)) {
        if (key !== 'name') {
            const providerData = {
                ORG_ID: orgID,
                NAME: provider.name,
                PROPERTY: key,
                VALUE: value
            };
            providerDataList.push(providerData);
        }
    }
    try {
        const provider = await Provider.bulkCreate(providerDataList, { transaction: t });
        return provider;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const update = async (orgID, provider) => {
    try {
        let updatedProviders = [];
        for (const [key, value] of Object.entries(provider)) {
            if (key !== 'name') {
                const [updatedRowsCount, providerContent] = await Provider.update(
                    {
                        VALUE: value
                    },
                    {
                        where: {
                            ORG_ID: orgID,
                            PROPERTY: key,
                            NAME: provider.name
                        },
                        returning: true
                    }
                );
                updatedProviders.push(providerContent)
                if (updatedRowsCount < 1) {
                    throw new Sequelize.EmptyResultError('API Provider not found');
                }
            }
        }
        return updatedProviders;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteProperty = async (orgID, property, name) => {
    try {
        const deletedRowsCount = await Provider.destroy({
            where: {
                ORG_ID: orgID,
                PROPERTY: property,
                NAME: name
            }
        });
        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteProvider = async (orgID, name) => {
    try {
        const deletedRowsCount = await Provider.destroy({
            where: {
                ORG_ID: orgID,
                NAME: name
            }
        });
        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const list = async (orgID) => {
    try {
        const jsonObjectFn = sequelize.getDialect() === 'sqlite' ? 'json_group_object' : 'JSON_OBJECT_AGG';
        const providers = await Provider.findAll(
            {
                attributes: [
                    'NAME',
                    [
                        Sequelize.fn(
                            jsonObjectFn,
                            Sequelize.col('PROPERTY'),
                            Sequelize.col('VALUE')
                        ),
                        'properties'
                    ]
                ],
                where: { ORG_ID: orgID },
                group: ['NAME']
            }
        );
        if (providers.length === 0) {
            return [];
        }
        return providers;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const get = async (orgID, name) => {
    try {
        return await Provider.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    NAME: name
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

module.exports = {
    create,
    update,
    deleteProperty,
    delete: deleteProvider,
    list,
    get,
};
