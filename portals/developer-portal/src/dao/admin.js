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
const { Organization, OrgContent } = require('../models/organization');
const { Sequelize } = require('sequelize');
const { Application, ApplicationKeyMapping, SubscriptionMapping } = require('../models/application');
const Provider = require('../models/provider');
const apiDao = require('./apiMetadata');
const { APIMetadata } = require('../models/apiMetadata');
const APIImageMetadata = require('../models/apiImages');
const SubscriptionPlan = require('../models/subscriptionPlan');
const logger = require('../config/logger');
const sequelize = require('../db/sequelize');

const createOrganization = async (orgData, t) => {
    let devPortalID = "";
    if (orgData.orgHandle) {
        devPortalID = orgData.orgHandle.toLowerCase();
    }
    const createOrgData = {
        ORG_NAME: orgData.orgName,
        BUSINESS_OWNER: orgData.businessOwner,
        BUSINESS_OWNER_CONTACT: orgData.businessOwnerContact,
        BUSINESS_OWNER_EMAIL: orgData.businessOwnerEmail,
        ORG_HANDLE: devPortalID,
        ORGANIZATION_IDENTIFIER: orgData.organizationIdentifier,
        ORG_CONFIG: orgData.orgConfig
    };
    try {
        const organization = await Organization.create(createOrgData, { transaction: t });
        return organization;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const getOrganization = async (param) => {
    try {
        const conditions = [
            { ORG_NAME: param },
            { ORG_HANDLE: typeof param === 'string' ? param.toLowerCase() : param },
            { ORGANIZATION_IDENTIFIER: param },
        ];
        if (typeof param === 'string' && UUID_RE.test(param)) {
            conditions.push({ ORG_ID: param });
        }
        const organization = await Organization.findOne({
            where: { [Sequelize.Op.or]: conditions }
        });
        if (!organization) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        return organization;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getOrgId = async (orgName) => {
    try {
        const organization = await Organization.findOne({
            where: {
                [Sequelize.Op.or]: [
                    { ORG_NAME: orgName },
                    { ORG_HANDLE: typeof orgName === 'string' ? orgName.toLowerCase() : orgName },
                    { ORGANIZATION_IDENTIFIER: orgName }
                ]
            }
        });
        if (!organization) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        return organization.ORG_ID;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getOrganizations = async () => {
    try {
        const organizations = await Organization.findAll();
        if (organizations.length === 0) {
            return [];
        }
        return organizations;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const updateOrganization = async (orgData, t) => {
    let devPortalID = "";
    if (orgData.orgHandle) {
        devPortalID = orgData.orgHandle.toLowerCase();
    }
    try {
        const [updatedRowsCount, updatedOrg] = await Organization.update(
            {
                ORG_NAME: orgData.orgName,
                BUSINESS_OWNER: orgData.businessOwner,
                BUSINESS_OWNER_CONTACT: orgData.businessOwnerContact,
                BUSINESS_OWNER_EMAIL: orgData.businessOwnerEmail,
                ORG_HANDLE: devPortalID,
                ORGANIZATION_IDENTIFIER: orgData.organizationIdentifier,
                ORG_CONFIG: orgData.orgConfiguration
            },
            {
                where: { ORG_ID: orgData.orgId },
                returning: true,
                transaction: t,
            }
        );
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        return [updatedRowsCount, updatedOrg];
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteOrganization = async (orgId) => {
    try {
        const deletedRowsCount = await Organization.destroy({
            where: { ORG_ID: orgId }
        });
        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createOrgContent = async (orgData) => {
    const viewID = await apiDao.getViewID(orgData.orgId, orgData.viewName);
    try {
        const orgContent = await OrgContent.create({
            FILE_TYPE: orgData.fileType,
            FILE_NAME: orgData.fileName,
            FILE_CONTENT: orgData.fileContent,
            FILE_PATH: orgData.filePath,
            ORG_ID: orgData.orgId,
            VIEW_ID: viewID
        });
        return orgContent;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateOrgContent = async (orgData) => {
    const viewID = await apiDao.getViewID(orgData.orgId, orgData.viewName);
    try {
        const [updatedRowsCount, updatedOrgContent] = await OrgContent.update({
            FILE_TYPE: orgData.fileType,
            FILE_NAME: orgData.fileName,
            FILE_CONTENT: orgData.fileContent,
            FILE_PATH: orgData.filePath,
        },
            {
                where: {
                    FILE_TYPE: orgData.fileType,
                    FILE_NAME: orgData.fileName,
                    FILE_PATH: orgData.filePath,
                    ORG_ID: orgData.orgId,
                    VIEW_ID: viewID
                },
                returning: true
            });
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('No new resources found');
        }
        return [updatedRowsCount, updatedOrgContent];
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}



const getOrgContent = async (orgData) => {
    try {
        const viewID = await apiDao.getViewID(orgData.orgId, orgData.viewName);
        if (orgData.fileName || orgData.filePath) {
            return await OrgContent.findOne(
                {
                    where: {
                        ORG_ID: orgData.orgId,
                        VIEW_ID: viewID,
                        FILE_TYPE: orgData.fileType,
                        ...(orgData.fileName && { FILE_NAME: orgData.fileName }),
                        ...(orgData.filePath && { FILE_PATH: orgData.filePath })
                    }
                });
        } else {
            return await OrgContent.findAll(
                {
                    where: {
                        ORG_ID: orgData.orgId,
                        VIEW_ID: viewID,
                        FILE_TYPE: orgData.fileType,
                    }
                });
        }
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteOrgContent = async (orgId, viewName, fileName) => {
    const viewId = await apiDao.getViewID(orgId, viewName);
    try {
        const deletedRowsCount = await OrgContent.destroy({
            where: {
                ORG_ID: orgId,
                VIEW_ID: viewId,
                FILE_NAME: fileName
            }
        });

        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization content not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteAllOrgContent = async (orgId, viewName) => {
    const viewId = await apiDao.getViewID(orgId, viewName);
    try {
        const deletedRowsCount = await OrgContent.destroy({
            where: {
                ORG_ID: orgId,
                VIEW_ID: viewId
            }
        });

        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization content not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const createProvider = async (orgID, provider, t) => {
    let providerDataList = [];
    for (const [key, value] of Object.entries(provider)) {
        if (key !== 'name') {
            const providerData = {
                ORG_ID: orgID,
                NAME: provider.name,
                PROPERTY: key,
                VALUE: value
            };
            providerDataList.push(providerData);
        }
    }
    try {
        const provider = await Provider.bulkCreate(providerDataList, { transaction: t });
        return provider;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateProvider = async (orgID, provider) => {
    try {
        let updatedProviders = [];
        for (const [key, value] of Object.entries(provider)) {
            if (key !== 'name') {
                const [updatedRowsCount, providerContent] = await Provider.update(
                    {
                        VALUE: value
                    },
                    {
                        where: {
                            ORG_ID: orgID,
                            PROPERTY: key,
                            NAME: provider.name
                        },
                        returning: true
                    }
                );
                updatedProviders.push(providerContent)
                if (updatedRowsCount < 1) {
                    throw new Sequelize.EmptyResultError('API Provider not found');
                }
            }
        }
        return updatedProviders;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteProviderProperty = async (orgID, property, name) => {
    try {
        const deletedRowsCount = await Provider.destroy({
            where: {
                ORG_ID: orgID,
                PROPERTY: property,
                NAME: name
            }
        });
        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteProvider = async (orgID, name) => {
    try {
        const deletedRowsCount = await Provider.destroy({
            where: {
                ORG_ID: orgID,
                NAME: name
            }
        });
        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getProviders = async (orgID) => {
    try {
        const jsonObjectFn = sequelize.getDialect() === 'sqlite' ? 'json_group_object' : 'JSON_OBJECT_AGG';
        const providers = await Provider.findAll(
            {
                attributes: [
                    'NAME',
                    [
                        Sequelize.fn(
                            jsonObjectFn,
                            Sequelize.col('PROPERTY'),
                            Sequelize.col('VALUE')
                        ),
                        'properties'
                    ]
                ],
                where: { ORG_ID: orgID },
                group: ['NAME']
            }
        );
        if (providers.length === 0) {
            return [];
        }
        return providers;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getProvider = async (orgID, name) => {
    try {
        return await Provider.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    NAME: name
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createApplication = async (orgID, userID, appData) => {
    const createAppData = {
        NAME: appData.name,
        ORG_ID: orgID,
        DESCRIPTION: appData.description,
        TYPE: appData.type,
        CREATED_BY: userID
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

const updateApplication = async (orgID, appID, userID, appData) => {
    try {
        const [updatedRowsCount, appContent] = await Application.update(
            {
                NAME: appData.name,
                DESCRIPTION: appData.description,
                TYPE: appData.type
            },
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID,
                    CREATED_BY: userID
                },
                returning: true
            }
        );
        return [updatedRowsCount, appContent];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getApplication = async (orgID, appID, userID) => {
    try {
        const application = await Application.findOne(
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID,
                    CREATED_BY: userID
                }
            });
        return application;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getApplicationID = async (orgID, userID, appName) => {
    try {
        return await Application.findOne(
            {
                attributes: ['APP_ID'],
                where: {
                    ORG_ID: orgID,
                    CREATED_BY: userID,
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

const getApplications = async (orgID, userID) => {
    try {
        return await Application.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    CREATED_BY: userID
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteApplication = async (orgID, appID, userID) => {
    try {
        const deletedRowsCount = await Application.destroy({
            where: {
                ORG_ID: orgID,
                APP_ID: appID,
                CREATED_BY: userID
            }
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

const createSubscription = async (orgID, subscription, t) => {
    try {
        const subMapping = await SubscriptionMapping.create({
            CREATED_BY: subscription.createdBy,
            API_ID: subscription.apiId,
            PLAN_ID: subscription.planId,
            ORG_ID: orgID,
        }, { transaction: t });
        return subMapping;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const updateSubscription = async (orgID, subscription, t) => {
    try {
        const subMapping = await SubscriptionMapping.update({
            PLAN_ID: subscription.planId
        }, {
            where: {
                ORG_ID: orgID,
                SUB_ID: subscription.subId
            },
            transaction: t,
        });
        return subMapping;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getSubscription = async (orgID, subID, t) => {
    try {
        return await SubscriptionMapping.findOne({
            where: { ORG_ID: orgID, SUB_ID: subID },
            transaction: t,
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getSubscriptions = async (orgID, apiID) => {
    try {
        return await SubscriptionMapping.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    API_ID: apiID,
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteSubscription = async (orgID, subID, t) => {

    try {
        const deletedRowsCount = await SubscriptionMapping.destroy({
            where: { ORG_ID: orgID, SUB_ID: subID },
            transaction: t,
        });
        if (deletedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('Subscription not found');
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}


const deleteAppMappingsByIds = async (orgID, mappingIds, t) => {
    if (!mappingIds || mappingIds.length === 0) return 0;
    try {
        return await ApplicationKeyMapping.destroy({
            where: { MAPPING_ID: mappingIds, ORG_ID: orgID },
            transaction: t,
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteAppMappings = async (orgID, appID, t) => {
    try {
        const deletedRowsCount = await ApplicationKeyMapping.destroy({
            where: {
                ORG_ID: orgID,
                APP_ID: appID
            }, transaction: t
        }, { transaction: t });
        if (deletedRowsCount < 1) {
            logger.info("No Application Key Mapping found", {
                orgID,
                appID,
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


const getKeyMapping = async (orgID, appID, t) => {
    try {
        const result = await Application.findOne(
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID
                },
                include: [
                    {
                        model: ApplicationKeyMapping,
                        where: {
                            APP_ID: appID
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



const getApplicationKeyMapping = async (orgID, appID) => {
    try {
        return await ApplicationKeyMapping.findAll({
            where: { ORG_ID: orgID, APP_ID: appID }
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}


const createApplicationKeyMapping = async (mappingData, t) => {
    try {
        const appKeyMapping = await ApplicationKeyMapping.create({
            ORG_ID: mappingData.orgID,
            APP_ID: mappingData.appID,
            ...(mappingData.kmID && { KM_ID: mappingData.kmID }),
            ...(mappingData.asClientID && { AS_CLIENT_ID: mappingData.asClientID }),
            ...(mappingData.keyType && { KEY_TYPE: mappingData.keyType }),
            ...(mappingData.additionalProperties && { ADDITIONAL_PROPERTIES: mappingData.additionalProperties }),
        }, { transaction: t });
        return appKeyMapping;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const upsertApplicationKeyMapping = async (mappingData, t) => {
    try {
        const existing = await ApplicationKeyMapping.findOne({
            where: {
                ORG_ID: mappingData.orgID,
                APP_ID: mappingData.appID,
                ...(mappingData.kmID && { KM_ID: mappingData.kmID }),
                KEY_TYPE: mappingData.keyType,
            },
            ...(t && { transaction: t }),
        });
        if (existing) {
            await existing.update({
                AS_CLIENT_ID: mappingData.asClientID,
                ADDITIONAL_PROPERTIES: mappingData.additionalProperties,
            }, { transaction: t });
            return existing;
        }
        return await ApplicationKeyMapping.create({
            ORG_ID: mappingData.orgID,
            APP_ID: mappingData.appID,
            ...(mappingData.kmID && { KM_ID: mappingData.kmID }),
            AS_CLIENT_ID: mappingData.asClientID,
            KEY_TYPE: mappingData.keyType,
            ADDITIONAL_PROPERTIES: mappingData.additionalProperties,
        }, { transaction: t });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};


/**
 * Find subscription by unique key (app, api, plan)
 */
const findSubscriptionByUniqueKey = async (orgID, apiID, planID, t) => {
    try {
        return await SubscriptionMapping.findOne({
            where: { ORG_ID: orgID, API_ID: apiID, PLAN_ID: planID },
            transaction: t,
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) return null;
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * List all subscriptions for an organization
 */
const listSubscriptionsByOrg = async (orgID) => {
    try {
        return await SubscriptionMapping.findAll({
            where: { ORG_ID: orgID },
        });
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * List subscriptions for a specific user in an organization
 */
const listSubscriptionsByUser = async (orgID, userID) => {
    try {
        return await SubscriptionMapping.findAll({
            where: { ORG_ID: orgID, CREATED_BY: userID },
        });
    } catch (error) {
        logger.error('listSubscriptionsByUser failed', { error, orgID, userID });
        throw new Sequelize.DatabaseError(error);
    }
};

module.exports = {
    createOrganization,
    getOrganization,
    updateOrganization,
    deleteOrganization,
    createOrgContent,
    updateOrgContent,
    getOrgContent,
    deleteOrgContent,
    deleteAllOrgContent,
    getOrgId,
    getOrganizations,
    createProvider,
    deleteProviderProperty,
    deleteProvider,
    updateProvider,
    getProviders,
    getProvider,
    createApplication,
    updateApplication,
    getApplication,
    getApplications,
    deleteApplication,
    createSubscription,
    updateSubscription,
    getSubscription,
    getSubscriptions,
    deleteSubscription,
    getApplicationID,
    getKeyMapping,
    getApplicationKeyMapping,
    createApplicationKeyMapping,
    upsertApplicationKeyMapping,
    deleteAppMappings,
    deleteAppMappingsByIds,
    findSubscriptionByUniqueKey,
    listSubscriptionsByOrg,
    listSubscriptionsByUser,
};
