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

const create = async (orgId, userId, appData) => {
    const createAppData = {
        display_name: appData.displayName,
        handle: appData.handle || appData.displayName,
        org_uuid: orgId,
        description: appData.description,
        created_by: userId,
        updated_by: userId
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
                display_name: appData.displayName,
                description: appData.description,
                updated_by: userId,
                updated_at: new Date()
            },
            {
                where: {
                    org_uuid: orgId,
                    uuid: appId,
                    created_by: userId
                }
            }
        );
        if (!updatedRowsCount) {
            return [updatedRowsCount, null];
        }
        const updatedApp = await Application.findOne({ where: { org_uuid: orgId, uuid: appId } });
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
                    org_uuid: orgId,
                    uuid: appId,
                    created_by: userId
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

const getId = async (orgId, userId, handle) => {
    try {
        return await Application.findOne(
            {
                attributes: ['uuid'],
                where: {
                    org_uuid: orgId,
                    created_by: userId,
                    handle: handle
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
                    org_uuid: orgId,
                    created_by: userId
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
                org_uuid: orgId,
                uuid: appId,
                created_by: userId
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
                    org_uuid: orgId,
                    uuid: appId
                },
                include: [
                    {
                        model: ApplicationKeyMapping,
                        where: {
                            app_uuid: appId
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
                app_uuid: mappingData.appId,
                km_uuid: mappingData.kmId ?? null,
                type: mappingData.type,
            },
            ...(t && { transaction: t }),
        });
        if (existing) {
            await existing.update({
                as_client_id: mappingData.asClientId,
                updated_by: mappingData.createdBy,
                updated_at: new Date()
            }, { transaction: t });
            return existing;
        }
        return await ApplicationKeyMapping.create({
            app_uuid: mappingData.appId,
            ...(mappingData.kmId && { km_uuid: mappingData.kmId }),
            as_client_id: mappingData.asClientId,
            type: mappingData.type,
            created_by: mappingData.createdBy,
            updated_by: mappingData.createdBy,
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
                app_uuid: appId
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
            attributes: ['uuid'],
            where: { uuid: mappingIds },
            include: [{ model: Application, where: { org_uuid: orgId }, attributes: [], required: true }],
            transaction: t,
        });
        const ownedIds = ownedMappings.map((m) => m.uuid);
        if (ownedIds.length === 0) return 0;
        return await ApplicationKeyMapping.destroy({
            where: { uuid: ownedIds },
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
            where: { app_uuid: appId }
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
            app_uuid: mappingData.appId,
            ...(mappingData.kmId && { km_uuid: mappingData.kmId }),
            ...(mappingData.asClientId && { as_client_id: mappingData.asClientId }),
            ...(mappingData.type && { type: mappingData.type }),
            created_by: mappingData.createdBy,
            updated_by: mappingData.createdBy,
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
