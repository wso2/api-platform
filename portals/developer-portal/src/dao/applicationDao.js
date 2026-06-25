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
const { Application, ApplicationKeyMapping, SubscriptionMapping } = require('../models/application');
const { Sequelize } = require('sequelize');
const logger = require('../config/logger');

const create = async (orgID, userID, appData) => {
    const createAppData = {
        NAME: appData.name,
        ORG_ID: orgID,
        DESCRIPTION: appData.description,
        TYPE: appData.type,
        CREATED_BY: userID
    };
    try {
        const application = await Application.create(createAppData);
        return application;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const update = async (orgID, appID, userID, appData) => {
    try {
        const [updatedRowsCount] = await Application.update(
            {
                NAME: appData.name,
                DESCRIPTION: appData.description,
                TYPE: appData.type
            },
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID,
                    CREATED_BY: userID
                }
            }
        );
        if (!updatedRowsCount) {
            return [updatedRowsCount, null];
        }
        const updatedApp = await Application.findOne({ where: { ORG_ID: orgID, APP_ID: appID } });
        return [updatedRowsCount, [updatedApp]];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const get = async (orgID, appID, userID) => {
    try {
        const application = await Application.findOne(
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID,
                    CREATED_BY: userID
                }
            });
        return application;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getId = async (orgID, userID, appName) => {
    try {
        return await Application.findOne(
            {
                attributes: ['APP_ID'],
                where: {
                    ORG_ID: orgID,
                    CREATED_BY: userID,
                    NAME: appName
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const list = async (orgID, userID) => {
    try {
        return await Application.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    CREATED_BY: userID
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteApp = async (orgID, appID, userID) => {
    try {
        const deletedRowsCount = await Application.destroy({
            where: {
                ORG_ID: orgID,
                APP_ID: appID,
                CREATED_BY: userID
            }
        });
        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Application not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getKeyMapping = async (orgID, appID, t) => {
    try {
        const result = await Application.findOne(
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID
                },
                include: [
                    {
                        model: ApplicationKeyMapping,
                        where: {
                            APP_ID: appID
                        }
                    }
                ],
                ...(t && { transaction: t })
            });
        return result;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const upsertKeyMapping = async (mappingData, t) => {
    try {
        const existing = await ApplicationKeyMapping.findOne({
            where: {
                ORG_ID: mappingData.orgID,
                APP_ID: mappingData.appID,
                ...(mappingData.kmID && { KM_ID: mappingData.kmID }),
                KEY_TYPE: mappingData.keyType,
            },
            ...(t && { transaction: t }),
        });
        if (existing) {
            await existing.update({
                AS_CLIENT_ID: mappingData.asClientID,
                ADDITIONAL_PROPERTIES: mappingData.additionalProperties,
            }, { transaction: t });
            return existing;
        }
        return await ApplicationKeyMapping.create({
            ORG_ID: mappingData.orgID,
            APP_ID: mappingData.appID,
            ...(mappingData.kmID && { KM_ID: mappingData.kmID }),
            AS_CLIENT_ID: mappingData.asClientID,
            KEY_TYPE: mappingData.keyType,
            ADDITIONAL_PROPERTIES: mappingData.additionalProperties,
        }, { transaction: t });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteMappings = async (orgID, appID, t) => {
    try {
        const deletedRowsCount = await ApplicationKeyMapping.destroy({
            where: {
                ORG_ID: orgID,
                APP_ID: appID
            }, transaction: t
        }, { transaction: t });
        if (deletedRowsCount < 1) {
            logger.info("No Application Key Mapping found", {
                orgID,
                appID,
                deletedRowsCount,
                operation: "deleteApplicationKeyMapping"
            });
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteMappingsByIds = async (orgID, mappingIds, t) => {
    if (!mappingIds || mappingIds.length === 0) return 0;
    try {
        return await ApplicationKeyMapping.destroy({
            where: { MAPPING_ID: mappingIds, ORG_ID: orgID },
            transaction: t,
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
};

const getKeyMappings = async (orgID, appID) => {
    try {
        return await ApplicationKeyMapping.findAll({
            where: { ORG_ID: orgID, APP_ID: appID }
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createKeyMapping = async (mappingData, t) => {
    try {
        const appKeyMapping = await ApplicationKeyMapping.create({
            ORG_ID: mappingData.orgID,
            APP_ID: mappingData.appID,
            ...(mappingData.kmID && { KM_ID: mappingData.kmID }),
            ...(mappingData.asClientID && { AS_CLIENT_ID: mappingData.asClientID }),
            ...(mappingData.keyType && { KEY_TYPE: mappingData.keyType }),
            ...(mappingData.additionalProperties && { ADDITIONAL_PROPERTIES: mappingData.additionalProperties }),
        }, { transaction: t });
        return appKeyMapping;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

module.exports = {
    create,
    update,
    get,
    getId,
    list,
    delete: deleteApp,
    getKeyMapping,
    upsertKeyMapping,
    deleteMappings,
    deleteMappingsByIds,
    getKeyMappings,
    createKeyMapping,
};
