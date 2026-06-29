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

const create = async (orgId, apiMetadata, createdBy, t) => {

    const apiInfo = apiMetadata.apiInfo;
    let owners = {};
    if (apiInfo.owners) {
        owners = apiInfo.owners;
    }
    try {
        const apiMetadataResponse = await APIMetadata.create({
            REF_ID: apiInfo.referenceId,
            STATUS: apiInfo.status,
            NAME: apiInfo.name,
            HANDLE: apiInfo.handle ? apiInfo.handle : `${apiInfo.name.toLowerCase().replace(/\s+/g, '')}-v${apiInfo.version}`,
            DESCRIPTION: apiInfo.description,
            VERSION: apiInfo.version,
            TYPE: apiInfo.type,
            AGENT_VISIBILITY: (apiMetadata.agentVisibility || apiInfo.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE).toUpperCase(),
            TECHNICAL_OWNER: owners.technicalOwner,
            TECHNICAL_OWNER_EMAIL: owners.technicalOwnerEmail,
            BUSINESS_OWNER_EMAIL: owners.businessOwnerEmail,
            BUSINESS_OWNER: owners.businessOwner,
            SANDBOX_URL: apiMetadata.endPoints.sandboxURL,
            PRODUCTION_URL: apiMetadata.endPoints.productionURL,
            METADATA_SEARCH: apiMetadata,
            ORG_UUID: orgId,
            CREATED_BY: createdBy,
            UPDATED_BY: createdBy
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

const update = async (orgId, apiId, apiMetadata, updatedBy, t) => {

    const apiInfo = apiMetadata.apiInfo;
    let owners = {};
    if (apiInfo.owners) {
        owners = apiInfo.owners;
    }
    try {
        const [updateCount] = await APIMetadata.update({
            REF_ID: apiInfo.referenceId,
            STATUS: apiInfo.status,
            NAME: apiInfo.name,
            HANDLE: apiInfo.handle ? apiInfo.handle : `${apiInfo.name.toLowerCase().replace(/\s+/g, '')}-v${apiInfo.version}`,
            DESCRIPTION: apiInfo.description,
            VERSION: apiInfo.version,
            TYPE: apiInfo.type,
            AGENT_VISIBILITY: (apiMetadata.agentVisibility || apiInfo.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE).toUpperCase(),
            TECHNICAL_OWNER: owners.technicalOwner,
            TECHNICAL_OWNER_EMAIL: owners.technicalOwnerEmail,
            BUSINESS_OWNER_EMAIL: owners.businessOwnerEmail,
            BUSINESS_OWNER: owners.businessOwner,
            SANDBOX_URL: apiMetadata.endPoints.sandboxURL,
            PRODUCTION_URL: apiMetadata.endPoints.productionURL,
            METADATA_SEARCH: apiMetadata,
            UPDATED_BY: updatedBy,
            UPDATED_AT: new Date()
        }, {
            where: {
                UUID: apiId,
                ORG_UUID: orgId,
            },
            returning: false,
            transaction: t
        });
        if (!updateCount) {
            return [0, null];
        }
        const updatedInstance = await APIMetadata.findOne({
            where: { UUID: apiId, ORG_UUID: orgId },
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

const deleteApi = async (orgId, apiId, t) => {

    try {
        const apiMetadataResponse = await APIMetadata.destroy({
            where: {
                UUID: apiId,
                ORG_UUID: orgId
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

const get = async (orgId, apiId, t) => {

    try {
        const apiMetadataResponse = await APIMetadata.findAll({
            include: [{
                model: APIContent,
                where: {
                    API_UUID: apiId,
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
                ORG_UUID: orgId,
                UUID: apiId,
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

const getByCondition = async (condition, t, tags) => {
    try {
        const tagsInclude = {
            model: Tags,
            attributes: ["NAME"],
            through: { attributes: [] },
            required: false
        };
        if (tags) {
            const tagsArray = tags.split(",").map(tag => tag.trim()).filter(Boolean);
            if (tagsArray.length > 0) {
                tagsInclude.required = true;
                tagsInclude.where = { NAME: { [Op.in]: tagsArray } };
            }
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

const list = async (orgId, viewName, t) => {

    const viewDao = require('./viewDao');
    const viewId = await viewDao.getId(orgId, viewName, t);
    let apiList = [];
    try {
        const apiMetadataResponse = await APIMetadata.findAll({
            where: {
                ORG_UUID: orgId,
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
                    UUID: {
                        [Op.in]: Sequelize.literal(`(SELECT "LABEL_UUID" FROM "DP_VIEW_LABEL_MAPPING" WHERE "VIEW_UUID" = '${viewId}')`)
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

const listFromAllViews = async (orgId, t) => {

    let apiList = [];
    try {
        const publicAPIS = await APIMetadata.findAll({
            where: {
                ORG_UUID: orgId,
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

const searchFallback = async (orgId, searchTerm, viewName, t) => {
    const viewDao = require('./viewDao');
    const pattern = `%${searchTerm}%`;
    const viewId = await viewDao.getId(orgId, viewName, t);

    const matchingTags = await Tags.findAll({
        attributes: ['UUID'],
        where: { ORG_UUID: orgId, NAME: { [Op.like]: pattern } },
        transaction: t,
    });
    const matchingTagIDs = matchingTags.map(tag => tag.UUID);
    const matchingTagAPIs = matchingTagIDs.length
        ? await APITags.findAll({
            attributes: ['API_UUID'],
            where: { TAG_UUID: { [Op.in]: matchingTagIDs } },
            transaction: t,
        })
        : [];
    const taggedAPIIDs = [...new Set(matchingTagAPIs.map(row => row.API_UUID))];

    return APIMetadata.findAll({
        where: {
            ORG_UUID: orgId,
            STATUS: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] },
            [Op.or]: [
                Sequelize.where(
                    Sequelize.cast(Sequelize.col('DP_API_METADATA.METADATA_SEARCH'), 'TEXT'),
                    { [Op.like]: pattern }
                ),
                { UUID: { [Op.in]: taggedAPIIDs } },
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
                    UUID: {
                        [Op.in]: Sequelize.literal(`(SELECT "LABEL_UUID" FROM "DP_VIEW_LABEL_MAPPING" WHERE "VIEW_UUID" = '${viewId}')`)
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

const search = async (orgId, searchTerm, viewName, t) => {
    if (APIMetadata.sequelize.getDialect() !== 'postgres') {
        return searchFallback(orgId, searchTerm, viewName, t);
    }
    try {
        const viewDao = require('./viewDao');
        const viewId = await viewDao.getId(orgId, viewName, t);
        const results = await APIMetadata.sequelize.query(SEARCH_APIS_POSTGRES_SQL, {
            replacements: { searchTerm, orgId, viewId },
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

const getId = async (orgId, apiHandle) => {

    try {
        const api = await APIMetadata.findOne({
            attributes: ['UUID'],
            where: {
                HANDLE: apiHandle,
                ORG_UUID: orgId
            }
        })
        return api?.UUID;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getHandle = async (orgId, apiRefId) => {
    try {
        const api = await APIMetadata.findOne({
            attributes: ['HANDLE'],
            where: {
                REF_ID: apiRefId,
                ORG_UUID: orgId
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

const getIdByRef = async (orgId, referenceId, t) => {
    try {
        const api = await APIMetadata.findOne({
            attributes: ['UUID'],
            where: {
                REF_ID: referenceId,
                ORG_UUID: orgId
            },
            transaction: t
        });
        return api?.UUID;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getSpecs = async (orgId, apiIds) => {
    try {
        const apiSpecsResponse = await APIContent.findAll({
            attributes: [
                'API_UUID',
                'FILE_NAME',
                'FILE_CONTENT'
            ],
            where: {
                API_UUID: {
                    [Op.in]: apiIds
                },
                TYPE: constants.DOC_TYPES.API_DEFINITION
            },
            include: [
                {
                    model: APIMetadata,
                    required: true,
                    attributes: ['NAME', 'VERSION', 'HANDLE'],
                    where: {
                        ORG_UUID: orgId
                    }
                }
            ]
        });

        return apiSpecsResponse.map(spec => {

            return {
                apiId: spec.API_UUID,
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
        attributes: ['UUID'],
        where: { ORG_UUID: orgId, NAME: apiName, VERSION: apiVersion },
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
