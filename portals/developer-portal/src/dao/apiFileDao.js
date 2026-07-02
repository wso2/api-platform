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
            file_content: apiFile,
            file_name: fileName,
            api_uuid: apiId,
            type: type,
            lookup_key: key ?? null,
            created_by: createdBy,
            updated_by: createdBy
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
                file_content: file.content,
                file_name: file.fileName,
                type: file.type,
                api_uuid: apiId,
                lookup_key: file.key ?? null,
                created_by: createdBy,
                updated_by: createdBy
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
            // A keyed file (e.g. a named image slot) is identified by its lookup_key, since its
            // file_name can change between uploads. Unkeyed files (docs, specs) are
            // identified by file_name as before.
            const apiFileResponse = file.key
                ? await getByKey(file.key, apiId, t)
                : await get(file.fileName, file.type, orgId, apiId, t);
            if (apiFileResponse == null || apiFileResponse == undefined) {
                filesToCreate.push({
                    file_content: file.content,
                    file_name: file.fileName,
                    api_uuid: apiId,
                    type: file.type,
                    lookup_key: file.key ?? null,
                    created_by: updatedBy,
                    updated_by: updatedBy
                })
            } else {
                const updateResponse = await APIContent.update(
                    {
                        file_content: file.content,
                        file_name: file.fileName,
                        lookup_key: file.key ?? apiFileResponse.lookup_key,
                        updated_by: updatedBy,
                        updated_at: new Date()
                    },
                    {
                        where: {
                            api_uuid: apiId,
                            file_name: apiFileResponse.file_name,
                            type: apiFileResponse.type,
                        },
                        include: [
                            {
                                model: APIMetadata,
                                where: {
                                    org_uuid: orgId
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
                file_name: fileName,
                api_uuid: apiId,
                type: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        org_uuid: orgId
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
                api_uuid: apiId,
                type: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        org_uuid: orgId
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
 * Find a single content row by its lookup_key (e.g. a named image slot like 'api-icon').
 */
const getByKey = async (key, apiId, t) => {
    try {
        return await APIContent.findOne({
            where: {
                api_uuid: apiId,
                type: constants.DOC_TYPES.IMAGES,
                lookup_key: key
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
 * Delete a single content row by its lookup_key (e.g. a named image slot like 'api-icon').
 */
const deleteByKey = async (key, apiId, t) => {
    try {
        return await APIContent.destroy({
            where: {
                api_uuid: apiId,
                type: constants.DOC_TYPES.IMAGES,
                lookup_key: key
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
                file_content: apiFile,
                file_name: fileName,
                api_uuid: apiId,
                type: type,
                lookup_key: key ?? null,
                created_by: updatedBy,
                updated_by: updatedBy
            }, { transaction: t });
        } else {
            fileUpdateResponse = await APIContent.update({
                file_content: apiFile,
                file_name: fileName,
                lookup_key: key ?? apiFileResponse.lookup_key,
                updated_by: updatedBy,
                updated_at: new Date()
            },
                {
                    where: {
                        api_uuid: apiId,
                        type: type
                    },
                    include: [
                        {
                            model: APIMetadata,
                            where: {
                                org_uuid: orgId
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
                file_content: apiFile,
                file_name: fileName,
                api_uuid: apiId,
                type: type,
                lookup_key: key ?? null,
                created_by: updatedBy,
                updated_by: updatedBy
            }, { transaction: t });
        } else {
            fileUpdateResponse = await APIContent.update({
                file_content: apiFile,
                file_name: fileName,
                lookup_key: key ?? apiFileResponse.lookup_key,
                updated_by: updatedBy,
                updated_at: new Date()
            },
                {
                    where: {
                        api_uuid: apiId,
                        file_name: fileName,
                        type: type
                    },
                    include: [
                        {
                            model: APIMetadata,
                            where: {
                                org_uuid: orgId
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
                file_name: fileName,
                api_uuid: apiId,
                type: { [Op.like]: `%${type}%` }
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        org_uuid: orgId
                    }
                }
            ],
            transaction: t
        });
        let apiFileResponse;
        for (const content of contentsToDelete) {
            apiFileResponse = await APIContent.destroy({
                where: {
                    api_uuid: content.dataValues.api_uuid,
                    file_name: content.dataValues.file_name,
                    type: content.dataValues.type
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
                api_uuid: apiId,
                type: {
                    [Op.like]: `%${type}%`
                }
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        org_uuid: orgId
                    }
                }
            ],
            transaction: t
        });
        let apiFileResponse;
        for (const content of contentsToDelete) {
            apiFileResponse = await APIContent.destroy({
                where: {
                    api_uuid: content.dataValues.api_uuid,
                    file_name: content.dataValues.file_name,
                    type: content.dataValues.type
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
 * Delete every content row of an exact type for an API (e.g. clear all images
 * before re-storing a freshly uploaded set). Exact match on type, scoped to
 * api_uuid, and participates in the caller's transaction.
 */
const deleteAllByType = async (type, apiId, t) => {
    try {
        return await APIContent.destroy({
            where: {
                api_uuid: apiId,
                type: type
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
                api_uuid: apiId,
                type: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        org_uuid: orgId
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
                api_uuid: apiId,
                type: type,
                file_name: name
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        org_uuid: orgId
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
        ? [Sequelize.fn("ARRAY_AGG", Sequelize.col("dp_api_content.file_name")), "file_names"]
        : [Sequelize.fn("GROUP_CONCAT", Sequelize.col("dp_api_content.file_name"), "|||"), "file_names"];

    try {
        const rows = await APIContent.findAll({
            attributes: ["type", fileNamesExpr],
            where: {
                api_uuid: apiId,
                type: {
                    [Op.or]: [
                        { [Op.like]: "DOC_%" },
                        { [Op.like]: constants.DOC_TYPES.API_DEFINITION }
                    ]
                },
            },
            group: ["dp_api_content.type"],
            include: [
                {
                    model: APIMetadata,
                    required: true,
                    attributes: [],
                    where: {
                        org_uuid: orgId
                    }
                }
            ]
        });

        if (!isPostgres) {
            for (const row of rows) {
                const raw = row.dataValues.file_names;
                row.dataValues.file_names = raw ? raw.split("|||") : [];
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
        where: { org_uuid: orgId }
    }];
    const where = {
        api_uuid: apiId,
        [Op.or]: [
            { type: { [Op.like]: "DOC_%" } },
            { file_name: { [Op.like]: "LINK_%" } }
        ]
    };

    try {
        if (isPostgres) {
            return await APIContent.findAll({
                attributes: [
                    "type",
                    [Sequelize.fn("ARRAY_AGG", Sequelize.col("dp_api_content.file_name")), "file_names"],
                    [Sequelize.fn("ARRAY_AGG", Sequelize.col("dp_api_content.file_content")), "api_files"]
                ],
                where,
                group: ["dp_api_content.type"],
                include
            });
        }

        const rows = await APIContent.findAll({ attributes: ["type", "file_name", "file_content"], where, include });
        const typeMap = new Map();
        for (const row of rows) {
            const { type, file_name, file_content } = row.dataValues;
            if (!typeMap.has(type)) typeMap.set(type, { type, file_names: [], api_files: [] });
            typeMap.get(type).file_names.push(file_name);
            typeMap.get(type).api_files.push(file_content);
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
        where: { org_uuid: orgId }
    }];
    const where = {
        api_uuid: apiId,
        file_name: { [Op.like]: "LINK_%" }
    };

    try {
        if (isPostgres) {
            return await APIContent.findAll({
                attributes: [
                    "type",
                    [Sequelize.fn("ARRAY_AGG", Sequelize.col("dp_api_content.file_name")), "file_names"],
                    [Sequelize.fn("ARRAY_AGG", Sequelize.col("dp_api_content.file_content")), "api_files"]
                ],
                where,
                group: ["dp_api_content.type"],
                include
            });
        }

        const rows = await APIContent.findAll({ attributes: ["type", "file_name", "file_content"], where, include });
        const typeMap = new Map();
        for (const row of rows) {
            const { type, file_name, file_content } = row.dataValues;
            if (!typeMap.has(type)) typeMap.set(type, { type, file_names: [], api_files: [] });
            typeMap.get(type).file_names.push(file_name);
            typeMap.get(type).api_files.push(file_content);
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
            attributes: ['file_name'],
            where: {
                api_uuid: apiId,
                type: { [Op.like]: `${constants.DOC_TYPES.DOC_ID}%` },
            },
            include: [{
                model: APIMetadata,
                required: true,
                attributes: [],
                where: { org_uuid: orgId }
            }]
        });
        return rows.map(r => r.dataValues.file_name);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
};

const listDocNamesForApis = async (orgId, apiIds) => {
    try {
        const rows = await APIContent.findAll({
            attributes: ['file_name', 'api_uuid'],
            where: {
                api_uuid: { [Op.in]: apiIds },
                type: { [Op.like]: `${constants.DOC_TYPES.DOC_ID}%` },
            },
            include: [{
                model: APIMetadata,
                required: true,
                attributes: [],
                where: { org_uuid: orgId }
            }]
        });
        const docNamesByApiId = {};
        for (const apiId of apiIds) docNamesByApiId[apiId] = [];
        for (const row of rows) {
            docNamesByApiId[row.dataValues.api_uuid].push(row.dataValues.file_name);
        }
        return docNamesByApiId;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteByFileName = async (fileName, orgId, apiId, t) => {
    try {
        const contentsToDelete = await APIContent.findAll({
            where: { file_name: fileName, api_uuid: apiId },
            include: [{ model: APIMetadata, required: true, attributes: [], where: { org_uuid: orgId } }],
            transaction: t
        });
        for (const content of contentsToDelete) {
            await APIContent.destroy({ where: { file_name: content.dataValues.file_name, api_uuid: apiId }, transaction: t });
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
    listDocNamesForApis,
    deleteByFileName,
};
