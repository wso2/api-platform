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
const { APIMetadata, APILabels } = require('../models/apiMetadata');
const SubscriptionPolicy = require('../models/subscriptionPolicy');
const APIImageMetadata = require('../models/apiImage');
const Labels = require('../models/label');
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
            REFERENCE_ID: apiInfo.referenceID,
            STATUS: apiInfo.apiStatus,
            PROVIDER: apiInfo.provider,
            API_NAME: apiInfo.apiName,
            API_HANDLE: apiInfo.apiHandle ? apiInfo.apiHandle : `${apiInfo.apiName.toLowerCase().replace(/\s+/g, '')}-v${apiInfo.apiVersion}`,
            API_DESCRIPTION: apiInfo.apiDescription,
            API_VERSION: apiInfo.apiVersion,
            API_TYPE: apiInfo.apiType,
            VISIBILITY: apiInfo.visibility,
            VISIBLE_GROUPS: apiInfo.visibleGroups ? apiInfo.visibleGroups.join(' ') : null,
            AGENT_VISIBILITY: (apiMetadata.agentVisibility || apiInfo.agentVisibility || 'VISIBLE').toUpperCase(),
            TAGS: apiInfo.tags ? apiInfo.tags.join(' ') : null,
            TECHNICAL_OWNER: owners.technicalOwner,
            TECHNICAL_OWNER_EMAIL: owners.technicalOwnerEmail,
            BUSINESS_OWNER_EMAIL: owners.businessOwnerEmail,
            BUSINESS_OWNER: owners.businessOwner,
            SANDBOX_URL: apiMetadata.endPoints.sandboxURL,
            PRODUCTION_URL: apiMetadata.endPoints.productionURL,
            METADATA_SEARCH: apiMetadata,
            GATEWAY_TYPE: apiMetadata.apiInfo.gatewayType || null,
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
            REFERENCE_ID: apiInfo.referenceID,
            STATUS: apiInfo.apiStatus,
            PROVIDER: apiInfo.provider,
            API_NAME: apiInfo.apiName,
            API_HANDLE: apiInfo.apiHandle ? apiInfo.apiHandle : `${apiInfo.apiName.toLowerCase().replace(/\s+/g, '')}-v${apiInfo.apiVersion}`,
            API_DESCRIPTION: apiInfo.apiDescription,
            API_VERSION: apiInfo.apiVersion,
            API_TYPE: apiInfo.apiType,
            TAGS: apiInfo.tags ? apiInfo.tags.join(' ') : null,
            VISIBILITY: apiInfo.visibility,
            VISIBLE_GROUPS: apiInfo.visibleGroups ? apiInfo.visibleGroups.join(' ') : null,
            AGENT_VISIBILITY: (apiMetadata.agentVisibility || apiInfo.agentVisibility || 'VISIBLE').toUpperCase(),
            TECHNICAL_OWNER: owners.technicalOwner,
            TECHNICAL_OWNER_EMAIL: owners.technicalOwnerEmail,
            BUSINESS_OWNER_EMAIL: owners.businessOwnerEmail,
            BUSINESS_OWNER: owners.businessOwner,
            SANDBOX_URL: apiMetadata.endPoints.sandboxURL,
            PRODUCTION_URL: apiMetadata.endPoints.productionURL,
            METADATA_SEARCH: apiMetadata,
            GATEWAY_TYPE: apiMetadata.apiInfo.gatewayType || null,
        }, {
            where: {
                API_ID: apiID,
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
                API_ID: apiID,
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
                model: APIImageMetadata,
                where: {
                    API_ID: apiID
                },
                required: false
            }, {
                model: SubscriptionPolicy,
                through: { attributes: [] },
                required: false
            },
            {
                model: Labels,
                attributes: ["NAME"],
                through: { attributes: [] }
            }
            ],
            where: {
                ORG_ID: orgID,
                API_ID: apiID,
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
        if (condition.TAGS) {
            const tagsArray = condition.TAGS.split(",").map(tag => tag.trim());
            condition.TAGS = {
                [Op.or]: tagsArray.map(tag => ({
                    [Op.and]: {
                        [Sequelize.Op.or]: [
                            {
                                [Sequelize.Op.like]: `% ${tag} %`
                            },
                            {
                                [Sequelize.Op.like]: `% ${tag}`
                            },
                            {
                                [Sequelize.Op.like]: `${tag} %`
                            },
                            {
                                [Sequelize.Op.eq]: `${tag}`
                            }
                        ]
                    }
                }))
            };
        }
        const apiMetadataResponse = await APIMetadata.findAll({
            include: [{
                model: APIImageMetadata,
                required: false
            }, {
                model: SubscriptionPolicy,
                through: { attributes: [] },
                required: false
            }
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

const list = async (orgID, groups, viewName, t) => {

    const viewDao = require('./viewDao');
    const viewID = await viewDao.getId(orgID, viewName);
    let apiList = [];
    for (const group of groups) {
        try {
            const apiMetadataResponse = await APIMetadata.findAll({
                where: {
                    ORG_ID: orgID,
                    VISIBLE_GROUPS: {
                        [Op.like]: `%${group}%`
                    },
                    STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
                },
                include: [{
                    model: APIImageMetadata,
                    required: false
                }, {
                    model: SubscriptionPolicy,
                    through: { attributes: [] },
                    required: false
                },
                {
                    model: Labels,
                    attributes: ["NAME"],
                    required: true,
                    through: { attributes: [] },
                    where: {
                        LABEL_ID: {
                            [Op.in]: Sequelize.literal(`(SELECT "LABEL_ID" FROM "DP_VIEW_LABELS" WHERE "VIEW_ID" = '${viewID}')`)
                        }
                    }
                }
                ],
                transaction: t
            });
            if (apiMetadataResponse) {
                apiList.push(...apiMetadataResponse);
            }
        } catch (error) {
            {
                if (error instanceof Sequelize.UniqueConstraintError) {
                    throw error;
                }
                throw new Sequelize.DatabaseError(error);
            }
        }
    }
    // add all public apis
    try {
        const publicAPIS = await APIMetadata.findAll({
            where: {
                ORG_ID: orgID,
                STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
            },
            include: [{
                model: APIImageMetadata,
                required: false
            }, {
                model: SubscriptionPolicy,
                through: { attributes: [] },
                required: false
            },
            {
                model: Labels,
                attributes: ["NAME"],
                required: true,
                through: { attributes: [] },
                where: {
                    LABEL_ID: {
                        [Op.in]: Sequelize.literal(`(SELECT "LABEL_ID" FROM "DP_VIEW_LABELS" WHERE "VIEW_ID" = '${viewID}')`)
                    }
                }
            }
            ],
            transaction: t
        });
        apiList.push(...publicAPIS);
    } catch (error) {
        {
            if (error instanceof Sequelize.UniqueConstraintError) {
                throw error;
            }
            throw new Sequelize.DatabaseError(error);
        }
    }
    return apiList;
};

const listFromAllViews = async (orgID, groups, t) => {

    let apiList = [];
    for (const group of groups) {
        try {
            const apiMetadataResponse = await APIMetadata.findAll({
                where: {
                    ORG_ID: orgID,
                    VISIBLE_GROUPS: {
                        [Op.like]: `%${group}%`
                    },
                    STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
                },
                include: [{
                    model: APIImageMetadata,
                    required: false
                }, {
                    model: SubscriptionPolicy,
                    through: { attributes: [] },
                    required: false
                },
                {
                    model: Labels,
                    attributes: ["NAME"],
                    required: false,
                    through: { attributes: [] }
                }
                ],
                transaction: t
            });
            if (apiMetadataResponse) {
                apiList.push(...apiMetadataResponse);
            }
        } catch (error) {
            {
                if (error instanceof Sequelize.UniqueConstraintError) {
                    throw error;
                }
                throw new Sequelize.DatabaseError(error);
            }
        }
    }
    // add all public apis
    try {
        const publicAPIS = await APIMetadata.findAll({
            where: {
                ORG_ID: orgID,
                STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
            },
            include: [{
                model: APIImageMetadata,
                required: false
            }, {
                model: SubscriptionPolicy,
                through: { attributes: [] },
                required: false
            },
            {
                model: Labels,
                attributes: ["NAME"],
                required: true,
                through: { attributes: [] }
            }
            ],
            transaction: t
        });
        apiList.push(...publicAPIS);
    } catch (error) {
        {
            if (error instanceof Sequelize.UniqueConstraintError) {
                throw error;
            }
            throw new Sequelize.DatabaseError(error);
        }
    }
    return apiList;
};

const searchFallback = async (orgID, searchTerm, viewName, t) => {
    const viewDao = require('./viewDao');
    const pattern = `%${searchTerm}%`;
    const viewID = await viewDao.getId(orgID, viewName);
    return APIMetadata.findAll({
        where: {
            ORG_ID: orgID,
            STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] },
            [Op.or]: [
                Sequelize.where(
                    Sequelize.cast(Sequelize.col('DP_API_METADATA.METADATA_SEARCH'), 'TEXT'),
                    { [Op.like]: pattern }
                ),
                Sequelize.where(
                    Sequelize.col('DP_API_METADATA.TAGS'),
                    { [Op.like]: pattern }
                ),
            ],
        },
        include: [
            { model: APIImageMetadata, required: false },
            { model: SubscriptionPolicy, through: { attributes: [] }, required: false },
            {
                model: Labels,
                attributes: ['NAME'],
                required: true,
                through: { attributes: [] },
                where: {
                    LABEL_ID: {
                        [Op.in]: Sequelize.literal(`(SELECT "LABEL_ID" FROM "DP_VIEW_LABELS" WHERE "VIEW_ID" = '${viewID}')`)
                    }
                }
            },
        ],
        transaction: t,
    });
};

const search = async (orgID, groups, searchTerm, viewName, t) => {
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
            attributes: ['API_ID'],
            where: {
                API_HANDLE: apiHandle,
                ORG_ID: orgID
            }
        })
        return api?.API_ID;
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
            attributes: ['API_HANDLE'],
            where: {
                REFERENCE_ID: apiRefID,
                ORG_ID: orgID
            }
        })
        return api.API_HANDLE;
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
            attributes: ['API_ID'],
            where: {
                REFERENCE_ID: referenceId,
                ORG_ID: orgID
            },
            transaction: t
        });
        return api?.API_ID;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getSpecs = async (orgID, apiIDs) => {
    const APIContent = require('../models/apiContent');
    try {
        const apiSpecsResponse = await APIContent.findAll({
            attributes: [
                'API_ID',
                'FILE_NAME',
                'API_FILE'
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
                    attributes: ['API_NAME', 'API_VERSION', 'API_HANDLE'],
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
                apiSpec: spec.API_FILE ? spec.API_FILE.toString('utf8') : null
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
