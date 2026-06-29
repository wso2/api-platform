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

const create = async (orgId, userId, appData) => {
    const createAppData = {
        NAME: appData.name,
        HANDLE: generateHandle(appData.name),
        ORG_UUID: orgId,
        DESCRIPTION: appData.description,
        CREATED_BY: userId,
        UPDATED_BY: userId
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

const update = async (orgId, appId, userId, appData) => {
    try {
        const [updatedRowsCount] = await Application.update(
            {
                NAME: appData.name,
                DESCRIPTION: appData.description,
                UPDATED_BY: userId,
                UPDATED_AT: new Date()
            },
            {
                where: {
                    ORG_UUID: orgId,
                    UUID: appId,
                    CREATED_BY: userId
                }
            }
        );
        if (!updatedRowsCount) {
            return [updatedRowsCount, null];
        }
        const updatedApp = await Application.findOne({ where: { ORG_UUID: orgId, UUID: appId } });
        return [updatedRowsCount, [updatedApp]];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const get = async (orgId, appId, userId, t) => {
    try {
        const application = await Application.findOne(
            {
                where: {
                    ORG_UUID: orgId,
                    UUID: appId,
                    CREATED_BY: userId
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

const getId = async (orgId, userId, appName) => {
    try {
        return await Application.findOne(
            {
                attributes: ['UUID'],
                where: {
                    ORG_UUID: orgId,
                    CREATED_BY: userId,
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

const list = async (orgId, userId) => {
    try {
        return await Application.findAll(
            {
                where: {
                    ORG_UUID: orgId,
                    CREATED_BY: userId
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteApp = async (orgId, appId, userId, t) => {
    try {
        const deletedRowsCount = await Application.destroy({
            where: {
                ORG_UUID: orgId,
                UUID: appId,
                CREATED_BY: userId
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

const getKeyMapping = async (orgId, appId, t) => {
    try {
        const result = await Application.findOne(
            {
                where: {
                    ORG_UUID: orgId,
                    UUID: appId
                },
                include: [
                    {
                        model: ApplicationKeyMapping,
                        where: {
                            APP_UUID: appId
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
                APP_UUID: mappingData.appId,
                KM_UUID: mappingData.kmId ?? null,
                TYPE: mappingData.type,
            },
            ...(t && { transaction: t }),
        });
        if (existing) {
            await existing.update({
                AS_CLIENT_ID: mappingData.asClientId,
                UPDATED_BY: mappingData.createdBy,
                UPDATED_AT: new Date()
            }, { transaction: t });
            return existing;
        }
        return await ApplicationKeyMapping.create({
            APP_UUID: mappingData.appId,
            ...(mappingData.kmId && { KM_UUID: mappingData.kmId }),
            AS_CLIENT_ID: mappingData.asClientId,
            TYPE: mappingData.type,
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

const deleteMappings = async (orgId, appId, t) => {
    try {
        const deletedRowsCount = await ApplicationKeyMapping.destroy({
            where: {
                APP_UUID: appId
            }, transaction: t
        }, { transaction: t });
        if (deletedRowsCount < 1) {
            logger.debug("No Application Key Mapping found", {
                orgId,
                appId,
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

const deleteMappingsByIds = async (orgId, mappingIds, t) => {
    if (!mappingIds || mappingIds.length === 0) return 0;
    try {
        const ownedMappings = await ApplicationKeyMapping.findAll({
            attributes: ['UUID'],
            where: { UUID: mappingIds },
            include: [{ model: Application, where: { ORG_UUID: orgId }, attributes: [], required: true }],
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

const getKeyMappings = async (orgId, appId) => {
    try {
        return await ApplicationKeyMapping.findAll({
            where: { APP_UUID: appId }
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
            APP_UUID: mappingData.appId,
            ...(mappingData.kmId && { KM_UUID: mappingData.kmId }),
            ...(mappingData.asClientId && { AS_CLIENT_ID: mappingData.asClientId }),
            ...(mappingData.type && { TYPE: mappingData.type }),
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
