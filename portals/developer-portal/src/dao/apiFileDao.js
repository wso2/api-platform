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

const store = async (apiFile, fileName, apiID, type, t) => {

    try {
        const apiFileResponse = await APIContent.create({
            FILE_CONTENT: apiFile,
            FILE_NAME: fileName,
            API_ID: apiID,
            TYPE: type
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

const storeMany = async (files, apiID, t) => {

    let apiContent = []
    try {
        files.forEach(file => {
            apiContent.push({
                FILE_CONTENT: file.content,
                FILE_NAME: file.fileName,
                TYPE: file.type,
                API_ID: apiID
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

const upsertMany = async (files, apiID, orgID, t) => {

    let filesToCreate = []
    try {
        for (const file of files) {
            const apiFileResponse = await get(file.fileName, file.type, orgID, apiID, t);
            if (apiFileResponse == null || apiFileResponse == undefined) {
                filesToCreate.push({
                    FILE_CONTENT: file.content,
                    FILE_NAME: file.fileName,
                    API_ID: apiID,
                    TYPE: file.type,
                })
            } else {
                const updateResponse = await APIContent.update(
                    {
                        FILE_CONTENT: file.content,
                    },
                    {
                        where: {
                            API_ID: apiID,
                            FILE_NAME: apiFileResponse.FILE_NAME,
                            TYPE: file.type,
                        },
                        include: [
                            {
                                model: APIMetadata,
                                where: {
                                    ORG_ID: orgID
                                }
                            }
                        ]
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

const get = async (fileName, type, orgID, apiID, t) => {

    try {
        const apiFileResponse = await APIContent.findOne({
            where: {
                FILE_NAME: fileName,
                API_ID: apiID,
                TYPE: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_ID: orgID
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

const getByType = async (type, orgID, apiID, t) => {
    try {
        const apiFileResponse = await APIContent.findOne({
            where: {
                API_ID: apiID,
                TYPE: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_ID: orgID
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

const upsert = async (apiFile, fileName, apiID, orgID, type, t) => {
    try {
        const apiFileResponse = await getByType(type, orgID, apiID, t);
        let fileUpdateResponse;
        if (apiFileResponse == null || apiFileResponse == undefined) {
            fileUpdateResponse = await APIContent.create({
                FILE_CONTENT: apiFile,
                FILE_NAME: fileName,
                API_ID: apiID,
                TYPE: type
            }, { transaction: t });
        } else {
            fileUpdateResponse = await APIContent.update({
                FILE_CONTENT: apiFile,
                FILE_NAME: fileName
            },
                {
                    where: {
                        API_ID: apiID,
                        TYPE: type
                    },
                    include: [
                        {
                            model: APIMetadata,
                            where: {
                                ORG_ID: orgID
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

const update = async (apiFile, fileName, apiID, orgID, type, t) => {

    try {
        const apiFileResponse = await get(fileName, type, orgID, apiID, t);
        let fileUpdateResponse;
        if (apiFileResponse == null || apiFileResponse == undefined) {
            fileUpdateResponse = await APIContent.create({
                FILE_CONTENT: apiFile,
                FILE_NAME: fileName,
                API_ID: apiID,
                TYPE: type
            }, { transaction: t });
        } else {
            fileUpdateResponse = await APIContent.update({
                FILE_CONTENT: apiFile,
                FILE_NAME: fileName
            },
                {
                    where: {
                        API_ID: apiID,
                        FILE_NAME: fileName,
                        TYPE: type
                    },
                    include: [
                        {
                            model: APIMetadata,
                            where: {
                                ORG_ID: orgID
                            }
                        }
                    ]
                },
                { transaction: t }
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

const deleteFile = async (fileName, type, orgID, apiID, t) => {

    try {
        const contentsToDelete = await APIContent.findAll({
            where: {
                FILE_NAME: fileName,
                API_ID: apiID,
                TYPE: { [Op.like]: `%${type}%` }
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_ID: orgID
                    }
                }
            ],
            transaction: t
        });
        let apiFileResponse;
        for (const content of contentsToDelete) {
            apiFileResponse = await APIContent.destroy({
                where: {
                    FILE_NAME: content.dataValues.FILE_NAME

                }
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

const deleteAll = async (type, orgID, apiID, t) => {

    try {
        const contentsToDelete = await APIContent.findAll({
            where: {
                API_ID: apiID,
                TYPE: {
                    [Op.like]: `%${type}%`
                }
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_ID: orgID
                    }
                }
            ],
            transaction: t
        });
        let apiFileResponse;
        for (const content of contentsToDelete) {
            apiFileResponse = await APIContent.destroy({
                where: {
                    FILE_NAME: content.dataValues.FILE_NAME
                }
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

const getDoc = async (type, orgID, apiID, t) => {

    try {
        const apiFileResponse = await APIContent.findOne({
            where: {
                API_ID: apiID,
                TYPE: type
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_ID: orgID
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

const getDocByName = async (type, name, orgID, apiID, t) => {

    try {
        const apiFileResponse = await APIContent.findOne({
            where: {
                API_ID: apiID,
                TYPE: type,
                FILE_NAME: name
            },
            include: [
                {
                    model: APIMetadata,
                    where: {
                        ORG_ID: orgID
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

const getDocTypes = async (orgID, apiID) => {
    const isPostgres = APIContent.sequelize.getDialect() === 'postgres';
    const fileNamesExpr = isPostgres
        ? [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_NAME")), "FILE_NAMES"]
        : [Sequelize.fn("GROUP_CONCAT", Sequelize.col("DP_API_CONTENT.FILE_NAME"), "|||"), "FILE_NAMES"];

    try {
        const rows = await APIContent.findAll({
            attributes: ["TYPE", fileNamesExpr],
            where: {
                API_ID: apiID,
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
                        ORG_ID: orgID
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

const getDocs = async (orgID, apiID) => {
    const isPostgres = APIContent.sequelize.getDialect() === 'postgres';
    const include = [{
        model: APIMetadata,
        required: true,
        attributes: [],
        where: { ORG_ID: orgID }
    }];
    const where = {
        API_ID: apiID,
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

const getDocLinks = async (orgID, apiID) => {
    const isPostgres = APIContent.sequelize.getDialect() === 'postgres';
    const include = [{
        model: APIMetadata,
        required: true,
        attributes: [],
        where: { ORG_ID: orgID }
    }];
    const where = {
        API_ID: apiID,
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

const listDocNames = async (orgID, apiID) => {
    try {
        const rows = await APIContent.findAll({
            attributes: ['FILE_NAME'],
            where: {
                API_ID: apiID,
                TYPE: { [Op.like]: `${constants.DOC_TYPES.DOC_ID}%` },
            },
            include: [{
                model: APIMetadata,
                required: true,
                attributes: [],
                where: { ORG_ID: orgID }
            }]
        });
        return rows.map(r => r.dataValues.FILE_NAME);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteByFileName = async (fileName, orgID, apiID, t) => {
    try {
        const contentsToDelete = await APIContent.findAll({
            where: { FILE_NAME: fileName, API_ID: apiID },
            include: [{ model: APIMetadata, required: true, attributes: [], where: { ORG_ID: orgID } }],
            transaction: t
        });
        for (const content of contentsToDelete) {
            await APIContent.destroy({ where: { FILE_NAME: content.dataValues.FILE_NAME, API_ID: apiID }, transaction: t });
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
    upsert,
    update,
    delete: deleteFile,
    deleteAll,
    getDoc,
    getDocByName,
    getDocTypes,
    getDocs,
    getDocLinks,
    listDocNames,
    deleteByFileName,
};
