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
const APISubscriptionPolicy = require('../models/apiSubscriptionPolicy');
const SubscriptionPolicy = require('../models/subscriptionPolicy');
const APIContent = require('../models/apiContent');
const APIImageMetadata = require('../models/apiImages');
const Labels = require('../models/labels');
const View = require('../models/views');
const ViewLabels = require('../models/viewLabels');
const { Sequelize } = require('sequelize');
const { Op } = require('sequelize');
const constants = require('../utils/constants');
const { CustomError } = require('../utils/errors/customErrors');
const logger = require('../config/logger');

const createAPIMetadata = async (orgID, apiMetadata, t) => {

    const apiInfo = apiMetadata.apiInfo;
    let owners = {};
    if (apiInfo.owners) {
        owners = apiInfo.owners;
    }
    try {
        const createData = {
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
            TAGS: apiInfo.tags ? apiInfo.tags.join(' ') : null,
            TECHNICAL_OWNER: owners.technicalOwner,
            TECHNICAL_OWNER_EMAIL: owners.technicalOwnerEmail,
            BUSINESS_OWNER_EMAIL: owners.businessOwnerEmail,
            BUSINESS_OWNER: owners.businessOwner,
            SANDBOX_URL: apiMetadata.endPoints.sandboxURL,
            PRODUCTION_URL: apiMetadata.endPoints.productionURL,
            METADATA_SEARCH: apiMetadata,
            ORG_ID: orgID
        };
        // If apiId is provided, use it instead of auto-generating
        if (apiInfo.apiId) {
            createData.API_ID = apiInfo.apiId;
        }
        const apiMetadataResponse = await APIMetadata.create(createData,
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

const createAPILabelMapping = async (orgID, apiID, labels, t) => {

    const labelList = [];
    const IDList = await getLabelID(orgID, labels, t);
    try {
        IDList.forEach(label => {
            labelList.push({
                LABEL_ID: label,
                API_ID: apiID,
                ORG_ID: orgID
            });
        });
        const labelResponse = await APILabels.bulkCreate(labelList, { transaction: t });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }

}

const createLabels = async (orgID, labels, t) => {

    const labelList = [];
    try {
        labels.forEach(label => {
            labelList.push({
                NAME: label.name,
                DISPLAY_NAME: label.displayName,
                ORG_ID: orgID
            });
        })
        const labelResponse = await Labels.bulkCreate(labelList, { transaction: t });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateLabel = async (orgID, label) => {

    try {
        let [record, created] = await Labels.findOrCreate({
            where: {
                NAME: label.name,
                ORG_ID: orgID
            },
            defaults: {
                NAME: label.name,
                DISPLAY_NAME: label.displayName,
            },
            returning: true
        });
        if (!created) {
            record = await record.update(label); // Update if found
        }
        return record;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getLabelID = async (orgID, labels, t) => {

    let IDList = [];
    try {
        for (const label of labels) {
            IDList.push(await getLabelIDList(orgID, label, t));
        };
        return IDList;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof CustomError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getLabelIDList = async (orgID, label, t) => {

    const labelResponse = await Labels.findOne({
        where: {
            NAME: label,
            ORG_ID: orgID
        }
    }, { transaction: t });
    if (!labelResponse) {
        throw new CustomError(404, constants.ERROR_CODE[404], "Label not found")
    }
    return labelResponse.dataValues.LABEL_ID;
}

const deleteLabel = async (orgID, labelNames) => {

    try {
        const labelResponse = await Labels.destroy({
            where: {
                NAME: labelNames,
                ORG_ID: orgID
            }
        });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getLabels = async (orgID) => {

    try {
        const labelResponse = await Labels.findAll({
            where: {
                ORG_ID: orgID
            }
        });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const addView = async (orgID, payload, t) => {

    let displayName = payload.displayName ? payload.displayName : payload.name;
    try {
        const viewResponse = await View.create({
            DISPLAY_NAME: displayName,
            NAME: payload.name,
            ORG_ID: orgID
        }, { transaction: t });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateView = async (orgID, name, displayName, t) => {

    try {
        let [record, created] = await View.findOrCreate({
            where: {
                NAME: name,
                ORG_ID: orgID
            },
            defaults: {
                NAME: name,
                DISPLAY_NAME: displayName,
            },
            transaction: t,
            returning: true
        });
        if (!created) {
            record = await record.update({
                NAME: name,
                DISPLAY_NAME: displayName,
            }); // Update if found
        }
        return record;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteView = async (orgID, viewName) => {

    try {
        const viewResponse = await View.destroy({
            where: {
                NAME: viewName,
                ORG_ID: orgID
            }
        });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getView = async (orgID, viewName) => {

    try {
        const viewResponse = await View.findOne({
            where: {
                NAME: viewName,
                ORG_ID: orgID
            },
            include: {
                model: Labels,
                attributes: ["NAME"],
                through: { attributes: [] }
            },
        });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getViewID = async (orgID, viewName) => {

    try {
        const viewResponse = await View.findOne({
            where: {
                [Op.or]: [
                    { DISPLAY_NAME: viewName },
                    { NAME: viewName }
                ],
                ORG_ID: orgID
            }
        });
        if (!viewResponse) {
            throw new CustomError(404, constants.ERROR_CODE[404], "View not found")
        }
        return viewResponse.dataValues.VIEW_ID;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw error;
    }
}

const getAllViews = async (orgID) => {

    try {
        const viewResponse = await View.findAll({
            where: {
                ORG_ID: orgID
            },
            include: {
                model: Labels,
                attributes: ["NAME"],
                through: {
                    attributes: []
                }
            },
        });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const addViewLabels = async (orgID, viewID, labels, t) => {

    const labelList = [];
    const IDList = await getLabelID(orgID, labels, t);
    try {
        IDList.forEach(label => {
            labelList.push({
                LABEL_ID: label,
                VIEW_ID: viewID,
                ORG_ID: orgID
            });
        });
        const labelResponse = await ViewLabels.bulkCreate(labelList, { transaction: t });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const addLabel = async (orgID, labelID, viewID, t) => {

    try {
        const labelResponse = await ViewLabels.create({
            LABEL_ID: labelID,
            VIEW_ID: viewID,
            ORG_ID: orgID
        }, { transaction: t });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteViewLabels = async (orgID, viewID, labels, t) => {

    const IDList = await getLabelID(orgID, labels);
    let deleteResponse;
    try {
        IDList.forEach(async label => {
            deleteResponse = await ViewLabels.destroy({
                where: {
                    LABEL_ID: label,
                    VIEW_ID: viewID,
                    ORG_ID: orgID
                }
            }, { transaction: t });
        });
        return deleteResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}


const deleteAPILabels = async (orgID, apiID, labels, t) => {

    const IDList = await getLabelID(orgID, labels);
    let deleteResponse;
    try {
        IDList.forEach(async label => {
            deleteResponse = await APILabels.destroy({
                where: {
                    LABEL_ID: label,
                    API_ID: apiID,
                    ORG_ID: orgID
                }
            }, { transaction: t });
        });
        return deleteResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createAPISubscriptionPolicy = async (apiSubscriptionPolicies, apiID, t) => {

    let apiSubscriptionPolicyList = []
    try {
        apiSubscriptionPolicies.forEach(policy => {
            apiSubscriptionPolicyList.push({
                POLICY_ID: policy.policyID,
                API_ID: apiID
            })
        });
        const apiSubscriptionPolicyResponse = await APISubscriptionPolicy.bulkCreate(apiSubscriptionPolicyList, { transaction: t });
        return apiSubscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const putSubscriptionPolicy = async (orgID, policy, t) => {
    const currentSubscriptionPolicy = await getSubscriptionPolicyByName(orgID, policy.policyName, t);
    if (currentSubscriptionPolicy) {
        const updatedPolicy = await updateSubscriptionPolicy(orgID, currentSubscriptionPolicy.POLICY_ID, policy, t); 
        return {
            subscriptionPolicyResponse: updatedPolicy,
            statusCode: 200
        };
    } else {
        const createdPolicy = await createSubscriptionPolicy(orgID, policy, t);
        return {
            subscriptionPolicyResponse: createdPolicy,
            statusCode: 201
        };
    }
};

const createSubscriptionPolicy = async (orgID, policy, t) => {
    const requestCount = policy.requestCount === -1 ? "Unlimited" : policy.requestCount;
    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.create({
            POLICY_NAME: policy.policyName,
            DISPLAY_NAME: policy.displayName,
            BILLING_PLAN: policy.billingPlan,
            DESCRIPTION: policy.description,
            REQUEST_COUNT: requestCount,
            ORG_ID: orgID
        }, { transaction: t });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};
/**
 * Bulk create subscription policies
 * @param {} orgID 
 * @param {*} policies 
 * @param {*} t 
 * @returns 
 */
const bulkCreateSubscriptionPolicies = async (orgID, policies, t) => {
    let subscriptionPoliciesList = [];
    try {
        policies.forEach(policy => {
            const requestCount = policy.requestCount === -1 ? "Unlimited" : policy.requestCount;
            subscriptionPoliciesList.push({
                POLICY_NAME: policy.policyName,
                DISPLAY_NAME: policy.displayName,
                BILLING_PLAN: policy.billingPlan,
                DESCRIPTION: policy.description,
                REQUEST_COUNT: requestCount,
                ORG_ID: orgID
            });
        });
        return await SubscriptionPolicy.bulkCreate(subscriptionPoliciesList, { transaction: t });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateSubscriptionPolicy = async (orgID, policyID, policy, t) => {
    const requestCount = policy.requestCount === -1 ? "Unlimited" : policy.requestCount;
    try {
        const [affectedCount, updatedRows] = await SubscriptionPolicy.update({
            POLICY_NAME: policy.policyName,
            DISPLAY_NAME: policy.displayName,
            BILLING_PLAN: policy.billingPlan,
            DESCRIPTION: policy.description,
            REQUEST_COUNT: requestCount,
        }, {
            where: {
                POLICY_ID: policyID,
                ORG_ID: orgID
            },
            returning: true,
            transaction: t
        });
        return updatedRows[0];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteSubscriptionPolicy = async (orgID, policyName, t) => {

    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.destroy({
            where: {
                POLICY_NAME: policyName,
                ORG_ID: orgID
            },
            transaction: t 
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}


const getSubscriptionPolicyByName = async (orgID, policyName, t) => {

    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.findOne({
            where: {
                POLICY_NAME: policyName,
                ORG_ID: orgID
            },
            transaction: t 
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.ValidationError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};


const storeAPIImageMetadata = async (apiImages, apiID, t) => {

    let apiImagesList = [];
    try {
        for (var propertyKey in apiImages) {
            apiImagesList.push({
                IMAGE_TAG: propertyKey,
                IMAGE_NAME: apiImages[propertyKey],
                API_ID: apiID
            })
        }
        const apiImagesResponse = await APIImageMetadata.bulkCreate(apiImagesList, { transaction: t });
        return apiImagesResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const storeAPIFile = async (apiFile, fileName, apiID, type, t) => {

    try {
        const apiFileResponse = await APIContent.create({
            API_FILE: apiFile,
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

const storeAPIFiles = async (files, apiID, t) => {

    let apiContent = []
    try {
        files.forEach(file => {
            apiContent.push({
                API_FILE: file.content,
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

const updateOrCreateAPIFiles = async (files, apiID, orgID, t) => {

    let filesToCreate = []
    try {
        for (const file of files) {
            const apiFileResponse = await getAPIFile(file.fileName, file.type, orgID, apiID, t);
            if (apiFileResponse == null || apiFileResponse == undefined) {
                filesToCreate.push({
                    API_FILE: file.content,
                    FILE_NAME: file.fileName,
                    API_ID: apiID,
                    TYPE: file.type,
                })
            } else {
                const updateResponse = await APIContent.update(
                    {
                        API_FILE: file.content,
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

const getAPIFile = async (fileName, type, orgID, apiID, t) => {

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

const getAPIDoc = async (type, orgID, apiID, t) => {

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

const getAPIDocByName = async (type, name, orgID, apiID, t) => {

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

const getAPIDocTypes = async (orgID, apiID) => {

    try {
        const apiFileResponse = await APIContent.findAll({
            attributes: [
                "TYPE",
                [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_NAME")), "FILE_NAMES"]
            ],
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

        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getAPIDocs = async (orgID, apiID) => {
    try {
        const apiFileResponse = await APIContent.findAll({
            attributes: [
                "TYPE",
                [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_NAME")), "FILE_NAMES"],
                [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.API_FILE")), "API_FILES"]
            ],
            where: {
                API_ID: apiID,
                [Op.or]: [
                    { TYPE: { [Op.like]: "DOC_%" } },
                    { FILE_NAME: { [Op.like]: "LINK_%" } }
                ]
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

        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}


const getAPIDocLinks = async (orgID, apiID) => {

    try {
        const apiFileResponse = await APIContent.findAll({
            attributes: [
                "TYPE",
                [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.FILE_NAME")), "FILE_NAMES"],
                [Sequelize.fn("ARRAY_AGG", Sequelize.col("DP_API_CONTENT.API_FILE")), "API_FILES"]
            ],
            where: {
                API_ID: apiID,
                FILE_NAME: {
                    [Op.like]: "LINK_%"
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

        return apiFileResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getAPISpecs = async (orgID, apiIDs) => {
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

const getAPIMetadataByCondition = async (condition, t) => {
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

const getAPIMetadata = async (orgID, apiID, t) => {

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
                STATUS: constants.API_STATUS.PUBLISHED
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

const getAllAPIMetadata = async (orgID, groups, viewName, t) => {

    const viewID = await getViewID(orgID, viewName);
    let apiList = [];
    for (const group of groups) {
        try {
            const apiMetadataResponse = await APIMetadata.findAll({
                where: {
                    ORG_ID: orgID,
                    VISIBLE_GROUPS: {
                        [Op.like]: `%${group}%`
                    },
                    STATUS: constants.API_STATUS.PUBLISHED
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
                STATUS: constants.API_STATUS.PUBLISHED
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

const searchAPIMetadata = async (orgID, groups, searchTerm, t) => {
    try {
        const query = `
        SELECT 
            metadata.*,
            COALESCE(
                JSON_AGG("DP_API_IMAGEDATA") FILTER (WHERE "DP_API_IMAGEDATA"."API_ID" IS NOT NULL), 
                '[]'
            ) AS "DP_API_IMAGEDATA",
             COALESCE(
                JSON_AGG("DP_API_SUBSCRIPTION_POLICY") FILTER (WHERE "DP_API_SUBSCRIPTION_POLICY"."API_ID" IS NOT NULL), 
                '[]'
            ) AS "DP_API_SUBSCRIPTION_POLICY",
            COALESCE(
                ARRAY_AGG(DISTINCT "DP_LABELS"."NAME") FILTER (WHERE "DP_LABELS"."NAME" IS NOT NULL), 
                '{}'
            ) AS "DP_LABELs",
            ts_rank(
                to_tsvector('english', metadata."METADATA_SEARCH"::text),
                plainto_tsquery('english', COALESCE(:searchTerm, ''))
            ) AS "rank_metadata",
            STRING_AGG(
		        DISTINCT CASE 
		            WHEN content."API_FILE" IS NOT NULL 
		            AND to_tsvector('english', convert_from(content."API_FILE", 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
		            THEN content."TYPE"
		            ELSE 'METADATA'
		        END, ', '
		    ) AS "DATA_SOURCE" 
        FROM 
            "DP_API_METADATA" metadata
        LEFT JOIN  
            "DP_API_CONTENT" content 
            ON metadata."API_ID" = content."API_ID"
            AND (
                content."FILE_NAME" LIKE '%.hbs' 
                OR content."FILE_NAME" LIKE '%.md%' 
                OR content."FILE_NAME" LIKE '%.json%'
                OR content."FILE_NAME" LIKE '%.xml%'
                OR content."FILE_NAME" LIKE '%.graphql%'
            ) 
        LEFT OUTER JOIN 
            "DP_API_IMAGEDATA" 
            ON metadata."API_ID" = "DP_API_IMAGEDATA"."API_ID"
        LEFT OUTER JOIN 
            "DP_API_SUBSCRIPTION_POLICY" 
            ON metadata."API_ID" = "DP_API_SUBSCRIPTION_POLICY"."API_ID"
        LEFT OUTER JOIN 
            "DP_API_LABELS"  
            ON metadata."API_ID" = "DP_API_LABELS"."API_ID"
        LEFT OUTER JOIN 
            "DP_LABELS" 
            ON "DP_API_LABELS"."LABEL_ID" = "DP_LABELS"."LABEL_ID"
        WHERE 
            (
                to_tsvector('english', metadata."METADATA_SEARCH"::text) @@ plainto_tsquery('english', COALESCE(:searchTerm, ''))
                OR (
                    content."API_FILE" IS NOT NULL AND
                    to_tsvector('english', convert_from(content."API_FILE", 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
                )
            )
            AND metadata."ORG_ID" = :orgID
        GROUP BY 
            metadata."API_ID"
        ORDER BY
            rank_metadata DESC;
        `;
        const formattedGroups = `{${groups.map((g) => `"${g}"`).join(',')}}`;

        const results = await APIMetadata.sequelize.query(query, {
            replacements: { searchTerm, orgID, groups: formattedGroups },
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

const deleteAPIMetadata = async (orgID, apiID, t) => {

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

const updateAPIMetadata = async (orgID, apiID, apiMetadata, t) => {

    const apiInfo = apiMetadata.apiInfo;
    let owners = {};
    if (apiInfo.owners) {
        owners = apiInfo.owners;
    }
    try {
        let updateCount, apiMetadataResponse;
        [updateCount, apiMetadataResponse] = await APIMetadata.update({
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
            TECHNICAL_OWNER: owners.technicalOwner,
            TECHNICAL_OWNER_EMAIL: owners.technicalOwnerEmail,
            BUSINESS_OWNER_EMAIL: owners.businessOwnerEmail,
            BUSINESS_OWNER: owners.businessOwner,
            SANDBOX_URL: apiMetadata.endPoints.sandboxURL,
            PRODUCTION_URL: apiMetadata.endPoints.productionURL,
            METADATA_SEARCH: apiMetadata,
        }, {
            where: {
                API_ID: apiID,
                ORG_ID: orgID,
            },
            returning: true,
            transaction: t 
        });
        return [updateCount, apiMetadataResponse];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

async function updateAPISubscriptionPolicy(subscriptionPolicies, apiID, t) {

    let policiesToCreate = [];
    try {
        for (const policy of subscriptionPolicies) {
            policiesToCreate.push({
                POLICY_ID: policy.policyID,
                API_ID: apiID
            })
        }
        if (policiesToCreate.length > 0) {
            await APISubscriptionPolicy.destroy({
                where: { 
                    API_ID: apiID 
                },
                transaction: t
            });
            return await APISubscriptionPolicy.bulkCreate(policiesToCreate, { transaction: t });
        } else {
            return policiesToCreate;
        }
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getSubscriptionPolicy = async (policyID, orgID, t) => {
    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.findOne({
            where: {
                ORG_ID: orgID,
                POLICY_ID: policyID
            },
            transaction: t
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getSubscriptionPolicies = async (apiID, t) => {

    try {
        const subscriptionPolicyResponse = await SubscriptionPolicy.findAll({
            include: [
                {
                    model: APIMetadata,
                    where: { API_ID: apiID },
                    through: { attributes: [] }
                }
            ],
            transaction: t
        });
        return subscriptionPolicyResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateAPIImageMetadata = async (apiImages, orgID, apiID, t) => {

    let imageCreateList = [];
    try {
        for (const propertyKey in apiImages) {
            let apiImageResponse = await getImageMetadata(propertyKey, apiImages[propertyKey], orgID, apiID, t);
            if (apiImageResponse == null || apiImageResponse == undefined) {
                imageCreateList.push({
                    IMAGE_NAME: apiImages[propertyKey],
                    API_ID: apiID,
                    IMAGE_TAG: propertyKey
                })
            } else {
                const apiImageDataUpdate = await APIImageMetadata.update({
                    IMAGE_NAME: apiImages[propertyKey],
                    IMAGE_TAG: propertyKey
                }, {
                    where: {
                        [Op.or]: [
                            { IMAGE_TAG: apiImageResponse.IMAGE_TAG },
                            { IMAGE_NAME: apiImageResponse.IMAGE_NAME }
                        ],
                        API_ID: apiID
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
                if (!apiImageDataUpdate) {
                    throw new Sequelize.EmptyResultError("Error updating API Image Metadata");
                }
            }
            if (imageCreateList.length > 0) {
                await APIImageMetadata.bulkCreate(imageCreateList, { transaction: t });
            }
        }
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getImageMetadata = async (imageTag, imageName, orgID, apiID, t) => {

    try {
        const apiImageData = await APIImageMetadata.findOne({
            where: {
                [Op.or]: [
                    { IMAGE_TAG: imageTag },
                    { IMAGE_NAME: imageName }
                ],
                API_ID: apiID
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
        return apiImageData;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getImage = async (imageTag, apiID, t) => {
    try {
        const apiImageData = await APIImageMetadata.findOne({
            where: {
                IMAGE_TAG: imageTag,
                API_ID: apiID
            },
            transaction: t 
        });
        return apiImageData;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteImage = async (imageTag, apiID, t) => {
    try {
        const apiImageData = await APIImageMetadata.destroy({
            where: {
                IMAGE_TAG: imageTag,
                API_ID: apiID
            },
            transaction: t 
        });
        return apiImageData;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateAPIFile = async (apiFile, fileName, apiID, orgID, type, t) => {

    try {
        const apiFileResponse = await getAPIFile(fileName, type, orgID, apiID, t);
        let fileUpdateResponse;
        if (apiFileResponse == null || apiFileResponse == undefined) {
            fileUpdateResponse = await APIContent.create({
                API_FILE: apiFile,
                FILE_NAME: fileName,
                API_ID: apiID,
                TYPE: type
            }, { transaction: t });
        } else {
            fileUpdateResponse = await APIContent.update({
                API_FILE: apiFile,
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

const deleteAPIFile = async (fileName, type, orgID, apiID, t) => {

    try {
        const contentsToDelete = await APIContent.findAll({
            where: {
                FILE_NAME: fileName,
                API_ID: apiID,
                TYPE: { [Op.like]: `%${type}%`  }
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

const deleteAllAPIFiles = async (type, orgID, apiID, t) => {

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

const getAPIId = async (orgID, apiHandle) => {

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

const getAPIHandle = async (orgID, apiRefID) => {
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

module.exports = {
    createAPIMetadata,
    createAPISubscriptionPolicy,
    storeAPIFile,
    getAPIMetadata,
    getAllAPIMetadata,
    storeAPIImageMetadata,
    deleteAPIMetadata,
    updateAPIMetadata,
    updateAPISubscriptionPolicy,
    updateAPIImageMetadata,
    updateAPIFile,
    storeAPIFiles,
    updateOrCreateAPIFiles,
    getAPIFile,
    getAPIDoc,
    deleteAPIFile,
    deleteAllAPIFiles,
    getAPIDocs,
    getAPIDocLinks,
    getAPIDocByName,
    getAPIDocTypes,
    getAPIId,
    getAPIHandle,
    getAPISpecs,
    getAPIMetadataByCondition,
    searchAPIMetadata,
    putSubscriptionPolicy,
    createSubscriptionPolicy,
    bulkCreateSubscriptionPolicies,
    getSubscriptionPolicyByName,
    getSubscriptionPolicy,
    getSubscriptionPolicies,
    deleteSubscriptionPolicy,
    createLabels,
    getLabelID,
    deleteLabel,
    getLabels,
    addView,
    addViewLabels,
    deleteViewLabels,
    updateView,
    deleteView,
    getView,
    getAllViews,
    getViewID,
    createAPILabelMapping,
    deleteAPILabels,
    updateLabel,
    addLabel,
    getImage,
    deleteImage
};
