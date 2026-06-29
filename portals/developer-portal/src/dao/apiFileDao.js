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
const APIContent = require('../models/apiContent');
const { APIMetadata } = require('../models/apiMetadata');
const { Sequelize, Op } = require('sequelize');
const constants = require('../utils/constants');
const logger = require('../config/logger');

const store = async (apiFile, fileName, apiId, type, createdBy, t, key) => {

    try {
        const apiFileResponse = await APIContent.create({
            FILE_CONTENT: apiFile,
            FILE_NAME: fileName,
            API_UUID: apiId,
            TYPE: type,
            LOOKUP_KEY: key ?? null,
            CREATED_BY: createdBy,
            UPDATED_BY: createdBy
        }, { transaction: t }
        );
        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const storeMany = async (files, apiId, createdBy, t) => {

    let apiContent = []
    try {
        files.forEach(file => {
            apiContent.push({
                FILE_CONTENT: file.content,
                FILE_NAME: file.fileName,
                TYPE: file.type,
                API_UUID: apiId,
                LOOKUP_KEY: file.key ?? null,
                CREATED_BY: createdBy,
                UPDATED_BY: createdBy
            })
        });
        const apiContentResponse = await APIContent.bulkCreate(apiContent, { transaction: t });
        return apiContentResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const upsertMany = async (files, apiId, orgId, updatedBy, t) => {

    let filesToCreate = []
    try {
        for (const file of files) {
            // A keyed file (e.g. a named image slot) is identified by its LOOKUP_KEY, since its
            // FILE_NAME can change between uploads. Unkeyed files (docs, specs) are
            // identified by FILE_NAME as before.
            const apiFileResponse = file.key
                ? await getByKey(file.key, apiId, t)
                : await get(file.fileName, file.type, orgId, apiId, t);
            if (apiFileResponse == null || apiFileResponse == undefined) {
                filesToCreate.push({
                    FILE_CONTENT: file.content,
                    FILE_NAME: file.fileName,
                    API_UUID: apiId,
                    TYPE: file.type,
                    LOOKUP_KEY: file.key ?? null,
                    CREATED_BY: updatedBy,
                    UPDATED_BY: updatedBy
                })
            } else {
                const updateResponse = await APIContent.update(
                    {
                        FILE_CONTENT: file.content,
                        FILE_NAME: file.fileName,
                        LOOKUP_KEY: file.key ?? apiFileResponse.LOOKUP_KEY,
                        UPDATED_BY: updatedBy,
                        UPDATED_AT: new Date()
                    },
                    {
                        where: {
                            API_UUID: apiId,
                            FILE_NAME: apiFileResponse.FILE_NAME,
                            TYPE: apiFileResponse.TYPE,
                        },
                        include: [
                            {
                                model: APIMetadata,
                                where: {
                                    ORG_UUID: orgId
                                }
                            }
                        ],
                        transaction: t
                    }
                );
                if (!updateResponse) {
                    throw new Sequelize.DatabaseError('Error while updating API files');
                }
            }
        };
        if (filesToCreate.length > 0) {
            await APIContent.bulkCreate(filesToCreate, { transaction: t });
        }
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const get = async (fileName, type, orgId, apiId, t) => {

    try {
        const apiFileResponse = await APIContent.findOne({
            where: {
                FILE_NAME: fileName,
                API_UUID: apiId,
                TYPE: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_UUID: orgId
                    }
                }
            ],
            transaction: t
        });
        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getByType = async (type, orgId, apiId, t) => {
    try {
        const apiFileResponse = await APIContent.findOne({
            where: {
                API_UUID: apiId,
                TYPE: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_UUID: orgId
                    }
                }
            ],
            transaction: t
        });
        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

/**
 * Find a single content row by its LOOKUP_KEY (e.g. a named image slot like 'api-icon').
 */
const getByKey = async (key, apiId, t) => {
    try {
        return await APIContent.findOne({
            where: {
                API_UUID: apiId,
                TYPE: constants.DOC_TYPES.IMAGES,
                LOOKUP_KEY: key
            },
            transaction: t
        });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

/**
 * Delete a single content row by its LOOKUP_KEY (e.g. a named image slot like 'api-icon').
 */
const deleteByKey = async (key, apiId, t) => {
    try {
        return await APIContent.destroy({
            where: {
                API_UUID: apiId,
                TYPE: constants.DOC_TYPES.IMAGES,
                LOOKUP_KEY: key
            },
            transaction: t
        });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const upsert = async (apiFile, fileName, apiId, orgId, type, updatedBy, t, key) => {
    try {
        const apiFileResponse = await getByType(type, orgId, apiId, t);
        let fileUpdateResponse;
        if (apiFileResponse == null || apiFileResponse == undefined) {
            fileUpdateResponse = await APIContent.create({
                FILE_CONTENT: apiFile,
                FILE_NAME: fileName,
                API_UUID: apiId,
                TYPE: type,
                LOOKUP_KEY: key ?? null,
                CREATED_BY: updatedBy,
                UPDATED_BY: updatedBy
            }, { transaction: t });
        } else {
            fileUpdateResponse = await APIContent.update({
                FILE_CONTENT: apiFile,
                FILE_NAME: fileName,
                LOOKUP_KEY: key ?? apiFileResponse.LOOKUP_KEY,
                UPDATED_BY: updatedBy,
                UPDATED_AT: new Date()
            },
                {
                    where: {
                        API_UUID: apiId,
                        TYPE: type
                    },
                    include: [
                        {
                            model: APIMetadata,
                            where: {
                                ORG_UUID: orgId
                            }
                        }
                    ],
                    transaction: t
                }
            );
        }
        return fileUpdateResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const update = async (apiFile, fileName, apiId, orgId, type, updatedBy, t, key) => {

    try {
        const apiFileResponse = await get(fileName, type, orgId, apiId, t);
        let fileUpdateResponse;
        if (apiFileResponse == null || apiFileResponse == undefined) {
            fileUpdateResponse = await APIContent.create({
                FILE_CONTENT: apiFile,
                FILE_NAME: fileName,
                API_UUID: apiId,
                TYPE: type,
                LOOKUP_KEY: key ?? null,
                CREATED_BY: updatedBy,
                UPDATED_BY: updatedBy
            }, { transaction: t });
        } else {
            fileUpdateResponse = await APIContent.update({
                FILE_CONTENT: apiFile,
                FILE_NAME: fileName,
                LOOKUP_KEY: key ?? apiFileResponse.LOOKUP_KEY,
                UPDATED_BY: updatedBy,
                UPDATED_AT: new Date()
            },
                {
                    where: {
                        API_UUID: apiId,
                        FILE_NAME: fileName,
                        TYPE: type
                    },
                    include: [
                        {
                            model: APIMetadata,
                            where: {
                                ORG_UUID: orgId
                            }
                        }
                    ],
                    transaction: t
                }
            );
        }
        return fileUpdateResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteFile = async (fileName, type, orgId, apiId, t) => {

    try {
        const contentsToDelete = await APIContent.findAll({
            where: {
                FILE_NAME: fileName,
                API_UUID: apiId,
                TYPE: { [Op.like]: `%${type}%` }
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_UUID: orgId
                    }
                }
            ],
            transaction: t
        });
        let apiFileResponse;
        for (const content of contentsToDelete) {
            apiFileResponse = await APIContent.destroy({
                where: {
                    API_UUID: content.dataValues.API_UUID,
                    FILE_NAME: content.dataValues.FILE_NAME,
                    TYPE: content.dataValues.TYPE
                },
                transaction: t
            });
        }
        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteAll = async (type, orgId, apiId, t) => {

    try {
        const contentsToDelete = await APIContent.findAll({
            where: {
                API_UUID: apiId,
                TYPE: {
                    [Op.like]: `%${type}%`
                }
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_UUID: orgId
                    }
                }
            ],
            transaction: t
        });
        let apiFileResponse;
        for (const content of contentsToDelete) {
            apiFileResponse = await APIContent.destroy({
                where: {
                    API_UUID: content.dataValues.API_UUID,
                    FILE_NAME: content.dataValues.FILE_NAME,
                    TYPE: content.dataValues.TYPE
                },
                transaction: t
            });
        }
        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }

}

/**
 * Delete every content row of an exact TYPE for an API (e.g. clear all images
 * before re-storing a freshly uploaded set). Exact match on TYPE, scoped to
 * API_UUID, and participates in the caller's transaction.
 */
const deleteAllByType = async (type, apiId, t) => {
    try {
        return await APIContent.destroy({
            where: {
                API_UUID: apiId,
                TYPE: type
            },
            transaction: t
        });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getDoc = async (type, orgId, apiId, t) => {

    try {
        const apiFileResponse = await APIContent.findOne({
            where: {
                API_UUID: apiId,
                TYPE: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_UUID: orgId
                    }
                }
            ],
            transaction: t
        });
        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getDocByName = async (type, name, orgId, apiId, t) => {

    try {
        const apiFileResponse = await APIContent.findOne({
            where: {
                API_UUID: apiId,
                TYPE: type,
                FILE_NAME: name
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_UUID: orgId
                    }
                }
            ], transaction: t
        });
        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getDocTypes = async (orgId, apiId) => {
    const isPostgres = APIContent.sequelize.getDialect() === 'postgres';
    const fileNamesExpr = isPostgres
        ? [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_NAME")), "FILE_NAMES"]
        : [Sequelize.fn("GROUP_CONCAT", Sequelize.col("DP_API_CONTENT.FILE_NAME"), "|||"), "FILE_NAMES"];

    try {
        const rows = await APIContent.findAll({
            attributes: ["TYPE", fileNamesExpr],
            where: {
                API_UUID: apiId,
                TYPE: {
                    [Op.or]: [
                        { [Op.like]: "DOC_%" },
                        { [Op.like]: constants.DOC_TYPES.API_DEFINITION }
                    ]
                },
            },
            group: ["DP_API_CONTENT.TYPE"],
            include: [
                {
                    model: APIMetadata,
                    required: true,
                    attributes: [],
                    where: {
                        ORG_UUID: orgId
                    }
                }
            ]
        });

        if (!isPostgres) {
            for (const row of rows) {
                const raw = row.dataValues.FILE_NAMES;
                row.dataValues.FILE_NAMES = raw ? raw.split("|||") : [];
            }
        }

        return rows;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getDocs = async (orgId, apiId) => {
    const isPostgres = APIContent.sequelize.getDialect() === 'postgres';
    const include = [{
        model: APIMetadata,
        required: true,
        attributes: [],
        where: { ORG_UUID: orgId }
    }];
    const where = {
        API_UUID: apiId,
        [Op.or]: [
            { TYPE: { [Op.like]: "DOC_%" } },
            { FILE_NAME: { [Op.like]: "LINK_%" } }
        ]
    };

    try {
        if (isPostgres) {
            return await APIContent.findAll({
                attributes: [
                    "TYPE",
                    [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_NAME")), "FILE_NAMES"],
                    [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_CONTENT")), "API_FILES"]
                ],
                where,
                group: ["DP_API_CONTENT.TYPE"],
                include
            });
        }

        const rows = await APIContent.findAll({ attributes: ["TYPE", "FILE_NAME", "FILE_CONTENT"], where, include });
        const typeMap = new Map();
        for (const row of rows) {
            const { TYPE, FILE_NAME, FILE_CONTENT } = row.dataValues;
            if (!typeMap.has(TYPE)) typeMap.set(TYPE, { TYPE, FILE_NAMES: [], API_FILES: [] });
            typeMap.get(TYPE).FILE_NAMES.push(FILE_NAME);
            typeMap.get(TYPE).API_FILES.push(FILE_CONTENT);
        }
        return Array.from(typeMap.values()).map(g => ({ dataValues: g }));
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getDocLinks = async (orgId, apiId) => {
    const isPostgres = APIContent.sequelize.getDialect() === 'postgres';
    const include = [{
        model: APIMetadata,
        required: true,
        attributes: [],
        where: { ORG_UUID: orgId }
    }];
    const where = {
        API_UUID: apiId,
        FILE_NAME: { [Op.like]: "LINK_%" }
    };

    try {
        if (isPostgres) {
            return await APIContent.findAll({
                attributes: [
                    "TYPE",
                    [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_NAME")), "FILE_NAMES"],
                    [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_CONTENT")), "API_FILES"]
                ],
                where,
                group: ["DP_API_CONTENT.TYPE"],
                include
            });
        }

        const rows = await APIContent.findAll({ attributes: ["TYPE", "FILE_NAME", "FILE_CONTENT"], where, include });
        const typeMap = new Map();
        for (const row of rows) {
            const { TYPE, FILE_NAME, FILE_CONTENT } = row.dataValues;
            if (!typeMap.has(TYPE)) typeMap.set(TYPE, { TYPE, FILE_NAMES: [], API_FILES: [] });
            typeMap.get(TYPE).FILE_NAMES.push(FILE_NAME);
            typeMap.get(TYPE).API_FILES.push(FILE_CONTENT);
        }
        return Array.from(typeMap.values()).map(g => ({ dataValues: g }));
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const listDocNames = async (orgId, apiId) => {
    try {
        const rows = await APIContent.findAll({
            attributes: ['FILE_NAME'],
            where: {
                API_UUID: apiId,
                TYPE: { [Op.like]: `${constants.DOC_TYPES.DOC_ID}%` },
            },
            include: [{
                model: APIMetadata,
                required: true,
                attributes: [],
                where: { ORG_UUID: orgId }
            }]
        });
        return rows.map(r => r.dataValues.FILE_NAME);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteByFileName = async (fileName, orgId, apiId, t) => {
    try {
        const contentsToDelete = await APIContent.findAll({
            where: { FILE_NAME: fileName, API_UUID: apiId },
            include: [{ model: APIMetadata, required: true, attributes: [], where: { ORG_UUID: orgId } }],
            transaction: t
        });
        for (const content of contentsToDelete) {
            await APIContent.destroy({ where: { FILE_NAME: content.dataValues.FILE_NAME, API_UUID: apiId }, transaction: t });
        }
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
};

module.exports = {
    store,
    storeMany,
    upsertMany,
    get,
    getByType,
    getByKey,
    deleteByKey,
    upsert,
    update,
    delete: deleteFile,
    deleteAll,
    deleteAllByType,
    getDoc,
    getDocByName,
    getDocTypes,
    getDocs,
    getDocLinks,
    listDocNames,
    deleteByFileName,
};
