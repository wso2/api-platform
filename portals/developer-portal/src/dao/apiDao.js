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
const { APIMetadata, APILabels, APITags } = require('../models/apiMetadata');
const SubscriptionPlan = require('../models/subscriptionPlan');
const APIContent = require('../models/apiContent');
const Labels = require('../models/label');
const Tags = require('../models/tag');
const { Sequelize } = require('sequelize');
const { Op } = require('sequelize');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const fs = require('fs');
const path = require('path');

const SEARCH_APIS_POSTGRES_SQL = fs.readFileSync(
    path.join(__dirname, '../../database/queries/search-apis.postgres.sql'),
    'utf8'
);

const create = async (orgID, apiMetadata, t) => {

    const apiInfo = apiMetadata.apiInfo;
    let owners = {};
    if (apiInfo.owners) {
        owners = apiInfo.owners;
    }
    try {
        const apiMetadataResponse = await APIMetadata.create({
            REF_ID: apiInfo.referenceID,
            STATUS: apiInfo.apiStatus,
            NAME: apiInfo.apiName,
            HANDLE: apiInfo.apiHandle ? apiInfo.apiHandle : `${apiInfo.apiName.toLowerCase().replace(/\s+/g, '')}-v${apiInfo.apiVersion}`,
            DESCRIPTION: apiInfo.apiDescription,
            VERSION: apiInfo.apiVersion,
            TYPE: apiInfo.apiType,
            AGENT_VISIBILITY: (apiMetadata.agentVisibility || apiInfo.agentVisibility || 'VISIBLE').toUpperCase(),
            TECHNICAL_OWNER: owners.technicalOwner,
            TECHNICAL_OWNER_EMAIL: owners.technicalOwnerEmail,
            BUSINESS_OWNER_EMAIL: owners.businessOwnerEmail,
            BUSINESS_OWNER: owners.businessOwner,
            SANDBOX_URL: apiMetadata.endPoints.sandboxURL,
            PRODUCTION_URL: apiMetadata.endPoints.productionURL,
            METADATA_SEARCH: apiMetadata,
            ORG_ID: orgID
        },
            { transaction: t }
        );
        return apiMetadataResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const update = async (orgID, apiID, apiMetadata, t) => {

    const apiInfo = apiMetadata.apiInfo;
    let owners = {};
    if (apiInfo.owners) {
        owners = apiInfo.owners;
    }
    try {
        const [updateCount] = await APIMetadata.update({
            REF_ID: apiInfo.referenceID,
            STATUS: apiInfo.apiStatus,
            NAME: apiInfo.apiName,
            HANDLE: apiInfo.apiHandle ? apiInfo.apiHandle : `${apiInfo.apiName.toLowerCase().replace(/\s+/g, '')}-v${apiInfo.apiVersion}`,
            DESCRIPTION: apiInfo.apiDescription,
            VERSION: apiInfo.apiVersion,
            TYPE: apiInfo.apiType,
            AGENT_VISIBILITY: (apiMetadata.agentVisibility || apiInfo.agentVisibility || 'VISIBLE').toUpperCase(),
            TECHNICAL_OWNER: owners.technicalOwner,
            TECHNICAL_OWNER_EMAIL: owners.technicalOwnerEmail,
            BUSINESS_OWNER_EMAIL: owners.businessOwnerEmail,
            BUSINESS_OWNER: owners.businessOwner,
            SANDBOX_URL: apiMetadata.endPoints.sandboxURL,
            PRODUCTION_URL: apiMetadata.endPoints.productionURL,
            METADATA_SEARCH: apiMetadata,
        }, {
            where: {
                ID: apiID,
                ORG_ID: orgID,
            },
            returning: false,
            transaction: t
        });
        if (!updateCount) {
            return [0, null];
        }
        const updatedInstance = await APIMetadata.findOne({
            where: { API_ID: apiID, ORG_ID: orgID },
            transaction: t,
        });
        return [updateCount, [updatedInstance]];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteApi = async (orgID, apiID, t) => {

    try {
        const apiMetadataResponse = await APIMetadata.destroy({
            where: {
                ID: apiID,
                ORG_ID: orgID
            },
            transaction: t
        });
        return apiMetadataResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const get = async (orgID, apiID, t) => {

    try {
        const apiMetadataResponse = await APIMetadata.findAll({
            include: [{
                model: APIContent,
                where: {
                    API_ID: apiID,
                    TYPE: constants.DOC_TYPES.IMAGES
                },
                required: false
            }, {
                model: SubscriptionPlan,
                through: { attributes: [] },
                required: false
            },
            {
                model: Labels,
                attributes: ["NAME"],
                through: { attributes: [] }
            },
            {
                model: Tags,
                attributes: ["NAME"],
                through: { attributes: [] },
                required: false
            }
            ],
            where: {
                ORG_ID: orgID,
                ID: apiID,
                STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
            },
            transaction: t
        });
        return apiMetadataResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getByCondition = async (condition, t) => {
    try {
        const tagsInclude = {
            model: Tags,
            attributes: ["NAME"],
            through: { attributes: [] },
            required: false
        };
        if (condition.TAGS) {
            const tagsArray = condition.TAGS.split(",").map(tag => tag.trim()).filter(Boolean);
            delete condition.TAGS;
            tagsInclude.required = true;
            tagsInclude.where = { NAME: { [Op.in]: tagsArray } };
        }
        const apiMetadataResponse = await APIMetadata.findAll({
            include: [{
                model: APIContent,
                where: { TYPE: constants.DOC_TYPES.IMAGES },
                required: false
            }, {
                model: SubscriptionPlan,
                through: { attributes: [] },
                required: false
            },
            tagsInclude
            ],
            where: condition,
            transaction: t
        });
        return apiMetadataResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const list = async (orgID, viewName, t) => {

    const viewDao = require('./viewDao');
    const viewID = await viewDao.getId(orgID, viewName);
    let apiList = [];
    try {
        const apiMetadataResponse = await APIMetadata.findAll({
            where: {
                ORG_ID: orgID,
                STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
            },
            include: [{
                model: APIContent,
                where: { TYPE: constants.DOC_TYPES.IMAGES },
                required: false
            }, {
                model: SubscriptionPlan,
                through: { attributes: [] },
                required: false
            },
            {
                model: Labels,
                attributes: ["NAME"],
                required: true,
                through: { attributes: [] },
                where: {
                    ID: {
                        [Op.in]: Sequelize.literal(`(SELECT "LABEL_ID" FROM "DP_VIEW_LABEL_MAPPING" WHERE "VIEW_ID" = '${viewID}')`)
                    }
                }
            },
            {
                model: Tags,
                attributes: ["NAME"],
                required: false,
                through: { attributes: [] }
            }
            ],
            transaction: t
        });
        apiList = apiMetadataResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
    return apiList;
};

const listFromAllViews = async (orgID, t) => {

    let apiList = [];
    try {
        const publicAPIS = await APIMetadata.findAll({
            where: {
                ORG_ID: orgID,
                STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
            },
            include: [{
                model: APIContent,
                where: { TYPE: constants.DOC_TYPES.IMAGES },
                required: false
            }, {
                model: SubscriptionPlan,
                through: { attributes: [] },
                required: false
            },
            {
                model: Labels,
                attributes: ["NAME"],
                required: true,
                through: { attributes: [] }
            },
            {
                model: Tags,
                attributes: ["NAME"],
                required: false,
                through: { attributes: [] }
            }
            ],
            transaction: t
        });
        apiList = publicAPIS;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
    return apiList;
};

const searchFallback = async (orgID, searchTerm, viewName, t) => {
    const viewDao = require('./viewDao');
    const pattern = `%${searchTerm}%`;
    const viewID = await viewDao.getId(orgID, viewName);

    const matchingTags = await Tags.findAll({
        attributes: ['ID'],
        where: { ORG_ID: orgID, NAME: { [Op.like]: pattern } },
        transaction: t,
    });
    const matchingTagIDs = matchingTags.map(tag => tag.ID);
    const matchingTagAPIs = matchingTagIDs.length
        ? await APITags.findAll({
            attributes: ['API_ID'],
            where: { TAG_ID: { [Op.in]: matchingTagIDs } },
            transaction: t,
        })
        : [];
    const taggedAPIIDs = [...new Set(matchingTagAPIs.map(row => row.API_ID))];

    return APIMetadata.findAll({
        where: {
            ORG_ID: orgID,
            STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] },
            [Op.or]: [
                Sequelize.where(
                    Sequelize.cast(Sequelize.col('DP_API_METADATA.METADATA_SEARCH'), 'TEXT'),
                    { [Op.like]: pattern }
                ),
                { ID: { [Op.in]: taggedAPIIDs } },
            ],
        },
        include: [
            { model: APIContent, where: { TYPE: constants.DOC_TYPES.IMAGES }, required: false },
            { model: SubscriptionPlan, through: { attributes: [] }, required: false },
            {
                model: Labels,
                attributes: ['NAME'],
                required: true,
                through: { attributes: [] },
                where: {
                    ID: {
                        [Op.in]: Sequelize.literal(`(SELECT "LABEL_ID" FROM "DP_VIEW_LABEL_MAPPING" WHERE "VIEW_ID" = '${viewID}')`)
                    }
                }
            },
            {
                model: Tags,
                attributes: ['NAME'],
                required: false,
                through: { attributes: [] }
            },
        ],
        transaction: t,
    });
};

const search = async (orgID, searchTerm, viewName, t) => {
    if (APIMetadata.sequelize.getDialect() !== 'postgres') {
        return searchFallback(orgID, searchTerm, viewName, t);
    }
    try {
        const results = await APIMetadata.sequelize.query(SEARCH_APIS_POSTGRES_SQL, {
            replacements: { searchTerm, orgID },
            type: Sequelize.QueryTypes.SELECT,
        });
        return results;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getId = async (orgID, apiHandle) => {

    try {
        const api = await APIMetadata.findOne({
            attributes: ['ID'],
            where: {
                HANDLE: apiHandle,
                ORG_ID: orgID
            }
        })
        return api?.ID;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getHandle = async (orgID, apiRefID) => {
    try {
        const api = await APIMetadata.findOne({
            attributes: ['HANDLE'],
            where: {
                REF_ID: apiRefID,
                ORG_ID: orgID
            }
        })
        return api.HANDLE;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getIdByRef = async (orgID, referenceId, t) => {
    try {
        const api = await APIMetadata.findOne({
            attributes: ['ID'],
            where: {
                REF_ID: referenceId,
                ORG_ID: orgID
            },
            transaction: t
        });
        return api?.ID;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getSpecs = async (orgID, apiIDs) => {
    try {
        const apiSpecsResponse = await APIContent.findAll({
            attributes: [
                'API_ID',
                'FILE_NAME',
                'FILE_CONTENT'
            ],
            where: {
                API_ID: {
                    [Op.in]: apiIDs
                },
                TYPE: constants.DOC_TYPES.API_DEFINITION
            },
            include: [
                {
                    model: APIMetadata,
                    required: true,
                    attributes: ['NAME', 'VERSION', 'HANDLE'],
                    where: {
                        ORG_ID: orgID
                    }
                }
            ]
        });

        return apiSpecsResponse.map(spec => {

            return {
                apiID: spec.API_ID,
                fileName: spec.FILE_NAME,
                apiSpec: spec.FILE_CONTENT ? spec.FILE_CONTENT.toString('utf8') : null
            };
        }).filter(spec => spec !== null);
    } catch (error) {
        logger.error('Error fetching API specifications', {
            error: error.message,
            stack: error.stack,
            operation: 'fetchAPISpecifications'
        });
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const existsByNameVersion = async (orgId, apiName, apiVersion) => {
    const row = await APIMetadata.findOne({
        attributes: ['API_ID'],
        where: { ORG_ID: orgId, API_NAME: apiName, API_VERSION: apiVersion },
    });
    return !!row;
};

module.exports = {
    create,
    update,
    delete: deleteApi,
    get,
    getByCondition,
    list,
    listFromAllViews,
    search,
    searchFallback,
    getId,
    getHandle,
    getIdByRef,
    getSpecs,
    existsByNameVersion,
};
