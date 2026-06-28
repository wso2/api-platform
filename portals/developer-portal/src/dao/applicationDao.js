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

// HANDLE is an immutable, org-scoped slug; application names aren't unique,
// so a short random suffix keeps collisions practically impossible.
const generateHandle = (name) => {
    const slug = String(name || '').toLowerCase().trim()
        .replace(/[^\w\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .substring(0, 100);
    const suffix = Math.random().toString(36).slice(2, 8);
    return slug ? `${slug}-${suffix}` : `app-${suffix}`;
};

const create = async (orgID, userID, appData) => {
    const createAppData = {
        NAME: appData.name,
        HANDLE: generateHandle(appData.name),
        ORG_UUID: orgID,
        DESCRIPTION: appData.description,
        CREATED_BY: userID,
        UPDATED_BY: userID
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
                UPDATED_BY: userID,
                UPDATED_AT: new Date()
            },
            {
                where: {
                    ORG_UUID: orgID,
                    UUID: appID,
                    CREATED_BY: userID
                }
            }
        );
        if (!updatedRowsCount) {
            return [updatedRowsCount, null];
        }
        const updatedApp = await Application.findOne({ where: { ORG_UUID: orgID, UUID: appID } });
        return [updatedRowsCount, [updatedApp]];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const get = async (orgID, appID, userID, t) => {
    try {
        const application = await Application.findOne(
            {
                where: {
                    ORG_UUID: orgID,
                    UUID: appID,
                    CREATED_BY: userID
                },
                ...(t && { transaction: t })
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
                attributes: ['UUID'],
                where: {
                    ORG_UUID: orgID,
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
                    ORG_UUID: orgID,
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

const deleteApp = async (orgID, appID, userID, t) => {
    try {
        const deletedRowsCount = await Application.destroy({
            where: {
                ORG_UUID: orgID,
                UUID: appID,
                CREATED_BY: userID
            },
            ...(t && { transaction: t })
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
                    ORG_UUID: orgID,
                    UUID: appID
                },
                include: [
                    {
                        model: ApplicationKeyMapping,
                        where: {
                            APP_UUID: appID
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
                APP_UUID: mappingData.appID,
                KM_UUID: mappingData.kmID ?? null,
                TYPE: mappingData.keyType,
            },
            ...(t && { transaction: t }),
        });
        if (existing) {
            await existing.update({
                AS_CLIENT_ID: mappingData.asClientID,
                ADDITIONAL_PROPERTIES: mappingData.additionalProperties,
                UPDATED_BY: mappingData.createdBy,
                UPDATED_AT: new Date()
            }, { transaction: t });
            return existing;
        }
        return await ApplicationKeyMapping.create({
            APP_UUID: mappingData.appID,
            ...(mappingData.kmID && { KM_UUID: mappingData.kmID }),
            AS_CLIENT_ID: mappingData.asClientID,
            TYPE: mappingData.keyType,
            ADDITIONAL_PROPERTIES: mappingData.additionalProperties,
            CREATED_BY: mappingData.createdBy,
            UPDATED_BY: mappingData.createdBy,
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
                APP_UUID: appID
            }, transaction: t
        }, { transaction: t });
        if (deletedRowsCount < 1) {
            logger.debug("No Application Key Mapping found", {
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
        const ownedMappings = await ApplicationKeyMapping.findAll({
            attributes: ['UUID'],
            where: { UUID: mappingIds },
            include: [{ model: Application, where: { ORG_UUID: orgID }, attributes: [], required: true }],
            transaction: t,
        });
        const ownedIds = ownedMappings.map((m) => m.UUID);
        if (ownedIds.length === 0) return 0;
        return await ApplicationKeyMapping.destroy({
            where: { UUID: ownedIds },
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
            where: { APP_UUID: appID }
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
            APP_UUID: mappingData.appID,
            ...(mappingData.kmID && { KM_UUID: mappingData.kmID }),
            ...(mappingData.asClientID && { AS_CLIENT_ID: mappingData.asClientID }),
            ...(mappingData.keyType && { TYPE: mappingData.keyType }),
            ...(mappingData.additionalProperties && { ADDITIONAL_PROPERTIES: mappingData.additionalProperties }),
            CREATED_BY: mappingData.createdBy,
            UPDATED_BY: mappingData.createdBy,
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
