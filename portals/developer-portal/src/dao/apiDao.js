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
const SubscriptionPlanLimit = require('../models/subscriptionPlanLimit');
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

    let owners = {};
    if (apiMetadata.owners) {
        owners = apiMetadata.owners;
    }
    try {
        const apiMetadataResponse = await APIMetadata.create({
            ref_id: apiMetadata.referenceId,
            status: apiMetadata.status,
            name: apiMetadata.name,
            handle: apiMetadata.handle ? apiMetadata.handle : `${apiMetadata.name.toLowerCase().replace(/\s+/g, '')}-v${apiMetadata.version}`,
            description: apiMetadata.description,
            version: apiMetadata.version,
            type: apiMetadata.type,
            agent_visibility: (apiMetadata.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE).toUpperCase(),
            technical_owner: owners.technicalOwner,
            technical_owner_email: owners.technicalOwnerEmail,
            business_owner_email: owners.businessOwnerEmail,
            business_owner: owners.businessOwner,
            sandbox_url: apiMetadata.endPoints.sandboxURL,
            production_url: apiMetadata.endPoints.productionURL,
            metadata_search: apiMetadata,
            org_uuid: orgId,
            created_by: createdBy,
            updated_by: createdBy
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

    let owners = {};
    if (apiMetadata.owners) {
        owners = apiMetadata.owners;
    }
    try {
        const [updateCount] = await APIMetadata.update({
            ref_id: apiMetadata.referenceId,
            status: apiMetadata.status,
            name: apiMetadata.name,
            handle: apiMetadata.handle ? apiMetadata.handle : `${apiMetadata.name.toLowerCase().replace(/\s+/g, '')}-v${apiMetadata.version}`,
            description: apiMetadata.description,
            version: apiMetadata.version,
            type: apiMetadata.type,
            agent_visibility: (apiMetadata.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE).toUpperCase(),
            technical_owner: owners.technicalOwner,
            technical_owner_email: owners.technicalOwnerEmail,
            business_owner_email: owners.businessOwnerEmail,
            business_owner: owners.businessOwner,
            sandbox_url: apiMetadata.endPoints.sandboxURL,
            production_url: apiMetadata.endPoints.productionURL,
            metadata_search: apiMetadata,
            updated_by: updatedBy,
            updated_at: new Date()
        }, {
            where: {
                uuid: apiId,
                org_uuid: orgId,
            },
            returning: false,
            transaction: t
        });
        if (!updateCount) {
            return [0, null];
        }
        const updatedInstance = await APIMetadata.findOne({
            where: { uuid: apiId, org_uuid: orgId },
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
                uuid: apiId,
                org_uuid: orgId
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
                    api_uuid: apiId,
                    type: constants.DOC_TYPES.IMAGES
                },
                required: false
            }, {
                model: SubscriptionPlan,
                through: { attributes: [] },
                required: false,
                include: [{ model: SubscriptionPlanLimit, as: 'limits' }]
            },
            {
                model: Labels,
                attributes: ["handle"],
                through: { attributes: [] }
            },
            {
                model: Tags,
                attributes: ["name"],
                through: { attributes: [] },
                required: false
            }
            ],
            where: {
                org_uuid: orgId,
                uuid: apiId,
                status: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
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
            attributes: ["name"],
            through: { attributes: [] },
            required: false
        };
        if (tags) {
            const tagsArray = tags.split(",").map(tag => tag.trim()).filter(Boolean);
            if (tagsArray.length > 0) {
                tagsInclude.required = true;
                tagsInclude.where = { name: { [Op.in]: tagsArray } };
            }
        }
        const apiMetadataResponse = await APIMetadata.findAll({
            include: [{
                model: APIContent,
                where: { type: constants.DOC_TYPES.IMAGES },
                required: false
            }, {
                model: SubscriptionPlan,
                through: { attributes: [] },
                required: false,
                include: [{ model: SubscriptionPlanLimit, as: 'limits' }]
            }, {
                model: Labels,
                attributes: ["handle"],
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

const list = async (orgId, viewName, t, typeFilter) => {

    const viewDao = require('./viewDao');
    const viewId = await viewDao.getId(orgId, viewName, t);
    let apiList = [];
    try {
        const apiMetadataResponse = await APIMetadata.findAll({
            where: {
                org_uuid: orgId,
                status: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] },
                ...(typeFilter?.include && { type: typeFilter.include }),
                ...(typeFilter?.exclude && { type: { [Op.ne]: typeFilter.exclude } })
            },
            include: [{
                model: APIContent,
                where: { type: constants.DOC_TYPES.IMAGES },
                required: false
            }, {
                model: SubscriptionPlan,
                through: { attributes: [] },
                required: false,
                include: [{ model: SubscriptionPlanLimit, as: 'limits' }]
            },
            {
                model: Labels,
                attributes: ["handle"],
                required: true,
                through: { attributes: [] },
                where: {
                    uuid: {
                        [Op.in]: Sequelize.literal(`(SELECT "label_uuid" FROM "dp_view_label_mappings" WHERE "view_uuid" = '${viewId}')`)
                    }
                }
            },
            {
                model: Tags,
                attributes: ["name"],
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
                org_uuid: orgId,
                status: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] }
            },
            include: [{
                model: APIContent,
                where: { type: constants.DOC_TYPES.IMAGES },
                required: false
            }, {
                model: SubscriptionPlan,
                through: { attributes: [] },
                required: false,
                include: [{ model: SubscriptionPlanLimit, as: 'limits' }]
            },
            {
                model: Labels,
                attributes: ["handle"],
                required: true,
                through: { attributes: [] }
            },
            {
                model: Tags,
                attributes: ["name"],
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

const searchFallback = async (orgId, searchTerm, viewName, t, typeFilter) => {
    const viewDao = require('./viewDao');
    const pattern = `%${searchTerm}%`;
    // `view` is optional (apiViewQuery in the OpenAPI spec) — search unscoped by view
    // when it's omitted, rather than forcing a view-label join that would otherwise
    // either crash (viewId undefined) or silently match nothing (a literal 'undefined').
    const viewId = await viewDao.getId(orgId, viewName, t);
    const labelsInclude = viewId
        ? {
            model: Labels,
            attributes: ['handle'],
            required: true,
            through: { attributes: [] },
            where: {
                uuid: {
                    [Op.in]: Sequelize.literal(`(SELECT "label_uuid" FROM "dp_view_label_mappings" WHERE "view_uuid" = '${viewId}')`)
                }
            }
        }
        : { model: Labels, attributes: ['handle'], required: false, through: { attributes: [] } };

    const matchingTags = await Tags.findAll({
        attributes: ['uuid'],
        where: { org_uuid: orgId, name: { [Op.like]: pattern } },
        transaction: t,
    });
    const matchingTagIDs = matchingTags.map(tag => tag.uuid);
    const matchingTagAPIs = matchingTagIDs.length
        ? await APITags.findAll({
            attributes: ['api_uuid'],
            where: { tag_uuid: { [Op.in]: matchingTagIDs } },
            transaction: t,
        })
        : [];
    const taggedAPIIDs = [...new Set(matchingTagAPIs.map(row => row.api_uuid))];

    return APIMetadata.findAll({
        where: {
            org_uuid: orgId,
            status: { [Op.in]: [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED] },
            ...(typeFilter?.include && { type: typeFilter.include }),
            ...(typeFilter?.exclude && { type: { [Op.ne]: typeFilter.exclude } }),
            [Op.or]: [
                Sequelize.where(
                    Sequelize.cast(Sequelize.col('dp_api_metadata.metadata_search'), 'TEXT'),
                    { [Op.like]: pattern }
                ),
                { uuid: { [Op.in]: taggedAPIIDs } },
            ],
        },
        include: [
            { model: APIContent, where: { type: constants.DOC_TYPES.IMAGES }, required: false },
            { model: SubscriptionPlan, through: { attributes: [] }, required: false, include: [{ model: SubscriptionPlanLimit, as: 'limits' }] },
            labelsInclude,
            {
                model: Tags,
                attributes: ['name'],
                required: false,
                through: { attributes: [] }
            },
        ],
        transaction: t,
    });
};

const search = async (orgId, searchTerm, viewName, t, typeFilter) => {
    if (APIMetadata.sequelize.getDialect() !== 'postgres') {
        return searchFallback(orgId, searchTerm, viewName, t, typeFilter);
    }
    try {
        const viewDao = require('./viewDao');
        const viewId = await viewDao.getId(orgId, viewName, t);
        const results = await APIMetadata.sequelize.query(SEARCH_APIS_POSTGRES_SQL, {
            replacements: {
                searchTerm, orgId,
                viewId: viewId || null,
                includeType: typeFilter?.include || null,
                excludeType: typeFilter?.exclude || null,
            },
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
            attributes: ['uuid'],
            where: {
                handle: apiHandle,
                org_uuid: orgId
            }
        })
        return api?.uuid;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

// Same as getId, but also constrains the match to a specific `type` (e.g. 'MCP') in a
// single query — used by resource families that only manage one API type.
const getIdByType = async (orgId, apiHandle, type) => {

    try {
        const api = await APIMetadata.findOne({
            attributes: ['uuid'],
            where: {
                handle: apiHandle,
                org_uuid: orgId,
                type
            }
        })
        return api?.uuid;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

// Inverse of getIdByType — matches any type EXCEPT the excluded one. Used by /apis/*
// once a type gets its own dedicated resource family (e.g. MCP via /mcp-servers), so
// /apis/* stops resolving handles that belong to that dedicated family.
const getIdExcludingType = async (orgId, apiHandle, excludedType) => {

    try {
        const api = await APIMetadata.findOne({
            attributes: ['uuid'],
            where: {
                handle: apiHandle,
                org_uuid: orgId,
                type: { [Op.ne]: excludedType }
            }
        })
        return api?.uuid;
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
            attributes: ['handle'],
            where: {
                ref_id: apiRefId,
                org_uuid: orgId
            }
        })
        return api.handle;
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
            attributes: ['uuid'],
            where: {
                ref_id: referenceId,
                org_uuid: orgId
            },
            transaction: t
        });
        return api?.uuid;
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
                'api_uuid',
                'file_name',
                'file_content'
            ],
            where: {
                api_uuid: {
                    [Op.in]: apiIds
                },
                type: constants.DOC_TYPES.API_DEFINITION
            },
            include: [
                {
                    model: APIMetadata,
                    required: true,
                    attributes: ['name', 'version', 'handle'],
                    where: {
                        org_uuid: orgId
                    }
                }
            ]
        });

        return apiSpecsResponse.map(spec => {

            return {
                apiId: spec.api_uuid,
                fileName: spec.file_name,
                apiSpec: spec.file_content ? spec.file_content.toString('utf8') : null
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
        attributes: ['uuid'],
        where: { org_uuid: orgId, name: apiName, version: apiVersion },
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
    getIdByType,
    getIdExcludingType,
    getHandle,
    getIdByRef,
    getSpecs,
    existsByNameVersion,
};
