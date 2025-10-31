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
const { IdentityProvider } = require('../models/identityProvider');
const { Application, ApplicationKeyMapping, SubscriptionMapping } = require('../models/application');
const Provider = require('../models/provider');
const apiDao = require('./apiMetadata');
const { APIMetadata } = require('../models/apiMetadata');
const APIImageMetadata = require('../models/apiImages');
const SubscriptionPolicy = require('../models/subscriptionPolicy');
const logger = require('../config/logger');

const createOrganization = async (orgData, t) => {
    let devPortalID = "";
    if (orgData.orgHandle) {
        devPortalID = orgData.orgHandle
    }
    const createOrgData = {
        ORG_NAME: orgData.orgName,
        BUSINESS_OWNER: orgData.businessOwner,
        BUSINESS_OWNER_CONTACT: orgData.businessOwnerContact,
        BUSINESS_OWNER_EMAIL: orgData.businessOwnerEmail,
        ORG_HANDLE: devPortalID,
        ROLE_CLAIM_NAME: orgData.roleClaimName,
        GROUPS_CLAIM_NAME: orgData.groupsClaimName,
        ORGANIZATION_CLAIM_NAME: orgData.organizationClaimName,
        ORGANIZATION_IDENTIFIER: orgData.organizationIdentifier,
        ADMIN_ROLE: orgData.adminRole,
        SUBSCRIBER_ROLE: orgData.subscriberRole,
        SUPER_ADMIN_ROLE: orgData.superAdminRole,
        ORG_CONFIG: orgData.orgConfig
    };
    // If orgId is provided, use it instead of auto-generating
    if (orgData.orgId) {
        createOrgData.ORG_ID = orgData.orgId;
    }
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

const getOrganization = async (param) => {
    try {
        const organization = await Organization.findOne({
            where: {
                [Sequelize.Op.or]: [
                    { ORG_NAME: param },
                    { ORG_HANDLE: param },
                    { ORG_ID: param },
                    { ORGANIZATION_IDENTIFIER: param }
                ]
            }
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
                    { ORG_HANDLE: orgName },
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

const updateOrganization = async (orgData) => {
    let devPortalID = "";
    if (orgData.orgHandle) {
        devPortalID = orgData.orgHandle
    }
    try {
        const [updatedRowsCount, updatedOrg] = await Organization.update(
            {
                ORG_NAME: orgData.orgName,
                BUSINESS_OWNER: orgData.businessOwner,
                BUSINESS_OWNER_CONTACT: orgData.businessOwnerContact,
                BUSINESS_OWNER_EMAIL: orgData.businessOwnerEmail,
                ORG_HANDLE: devPortalID,
                ROLE_CLAIM_NAME: orgData.roleClaimName,
                GROUPS_CLAIM_NAME: orgData.groupsClaimName,
                ORGANIZATION_CLAIM_NAME: orgData.organizationClaimName,
                ORGANIZATION_IDENTIFIER: orgData.organizationIdentifier,
                ADMIN_ROLE: orgData.adminRole,
                SUBSCRIBER_ROLE: orgData.subscriberRole,
                SUPER_ADMIN_ROLE: orgData.superAdminRole,
                ORG_CONFIG: orgData.orgConfiguration
            },
            {
                where: { ORG_ID: orgData.orgId },
                returning: true
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

const createIdentityProvider = async (orgId, idpData) => {
    try {
        const idpResponse = await IdentityProvider.create({
            ORG_ID: orgId,
            ISSUER: idpData.issuer,
            NAME: idpData.name,
            AUTHORIZATION_URL: idpData.authorizationURL,
            TOKEN_URL: idpData.tokenURL,
            ...(idpData.userInfoURL && { USER_INFOR_URL: idpData.userInfoURL }),
            CLIENT_ID: idpData.clientId,
            CALLBACK_URL: idpData.callbackURL,
            ...(idpData.signUpURL && { SIGNUP_URL: idpData.signUpURL }),
            LOGOUT_URL: idpData.logoutURL,
            LOGOUT_REDIRECT_URL: idpData.logoutRedirectURI,
            ...(idpData.scope && { SCOPE: idpData.scope }),
            ...(idpData.jwksURL && { JWKS_URL: idpData.jwksURL }),
            ...(idpData.certificate && { CERTIFICATE: idpData.certificate })
        });
        return idpResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateIdentityProvider = async (orgID, idpData) => {
    try {
        const [updatedRowsCount, idpContent] = await IdentityProvider.update(
            {
                ORG_ID: idpData.orgId,
                ISSUER: idpData.issuer,
                NAME: idpData.name,
                AUTHORIZATION_URL: idpData.authorizationURL,
                TOKEN_URL: idpData.tokenURL,
                ...(idpData.userInfoURL && { USER_INFOR_URL: idpData.userInfoURL }),
                CLIENT_ID: idpData.clientId,
                CALLBACK_URL: idpData.callbackURL,
                ...(idpData.signUpURL && { SIGNUP_URL: idpData.signUpURL }),
                LOGOUT_URL: idpData.logoutURL,
                LOGOUT_REDIRECT_URI: idpData.logoutRedirectURI,
                SCOPE: idpData.scope,
                ...(idpData.jwksURL && { JWKS_URL: idpData.jwksURL }),
                ...(idpData.certificate && { CERTIFICATE: idpData.certificate })
            },
            {
                where: {
                    ORG_ID: orgID
                },
                returning: true
            }
        );
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('IdentityProvider not found');
        }
        return [updatedRowsCount, idpContent];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getIdentityProvider = async (orgID) => {
    try {
        const idpResponse = await IdentityProvider.findAll({
            where: {
                ORG_ID: orgID,
            }
        });
        return idpResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteIdentityProvider = async (orgID) => {
    try {
        const idpResponse = await IdentityProvider.destroy({
            where: {
                ORG_ID: orgID
            }
        });
        return idpResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
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
        const providers = await Provider.findAll(
            {
                attributes: [
                    'NAME',
                    [
                        Sequelize.fn(
                            'JSON_OBJECT_AGG',
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
            APP_ID: subscription.applicationID,
            API_ID: subscription.apiId,
            POLICY_ID: subscription.policyId,
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
            POLICY_ID: subscription.policyId
        }, {
            where: {
                ORG_ID: orgID,
                APP_ID: subscription.applicationID,
                API_ID: subscription.apiId
            }
        }, { transaction: t });
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
        return await SubscriptionMapping.findOne(
            {
                where: {
                    ORG_ID: orgID,
                    SUB_ID: subID
                }, transaction: t
            }, { transaction: t });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getSubscriptions = async (orgID, appID, apiID) => {
    try {
        return await SubscriptionMapping.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    [Sequelize.Op.or]: [
                        { APP_ID: appID },
                        { API_ID: apiID }
                    ]
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getAppApiSubscription = async (orgID, appID, apiID) => {
    try {
        return await SubscriptionMapping.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID,
                    API_ID: apiID
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
            where: {
                ORG_ID: orgID,
                SUB_ID: subID
            }, transaction: t
        }, { transaction: t });
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

const deleteAppKeyMapping = async (orgID, appID, apiID, t) => {
    try {
        const deletedRowsCount = await ApplicationKeyMapping.destroy({
            where: {
                ORG_ID: orgID,
                APP_ID: appID,
                API_REF_ID: apiID
            }, transaction: t
        });
        if (deletedRowsCount < 1 && apiID !== null) {
            throw Object.assign(new Sequelize.EmptyResultError('Application Key Mapping not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

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

const getAPISubscriptionReference = async (orgID, appID, apiID, t) => {
    try {
        const subscriptionReference = await ApplicationKeyMapping.findAll(
            {
                attributes: ['SUBSCRIPTION_REF_ID'],
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID,
                    API_REF_ID: apiID
                }
            }, { transaction: t });
        return subscriptionReference;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createAppKeyMapping = async (appKeyMap, t) => {
    try {
        const appKeyMapping = await ApplicationKeyMapping.create(appKeyMap, { transaction: t });
        return appKeyMapping;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getKeyMapping = async (orgID, appID, t) => {
    try {
        return await Application.findOne(
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
                ]
            }, { transaction: t });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}


const getSubscribedAPIs = async (orgID, appID) => {
    try {
        const subscribedAPIs = await APIMetadata.findAll({
            where: { ORG_ID: orgID },
            include: [{
                model: Application,
                where: { APP_ID: appID },
                required: true,
                through: { attributes: ["SUB_ID", "POLICY_ID"] }
            },
            {
                model: APIImageMetadata,
                required: false
            }, {
                model: SubscriptionPolicy,
                through: { attributes: ['POLICY_ID'] },
                required: false
            }]
        });
        return subscribedAPIs;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getApplicationKeyMapping = async (orgID, appID, isSharedToken) => {
    try {
        return await ApplicationKeyMapping.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID,
                    SHARED_TOKEN: isSharedToken
                }
            });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getApplicationAPIMapping = async (orgID, appID, apiID, appRefID, isSharedToken) => {
    try {
        return await ApplicationKeyMapping.findAll(
            {
                where: {
                    ORG_ID: orgID,
                    APP_ID: appID,
                    API_REF_ID: apiID,
                    CP_APP_REF: appRefID,
                    SHARED_TOKEN: isSharedToken
                }
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
            CP_APP_REF: mappingData.cpAppRef,
            API_REF_ID: mappingData.apiRefID,
            SUBSCRIPTION_REF_ID: mappingData.subscriptionRefID,
            SHARED_TOKEN: mappingData.sharedToken,
            TOKEN_TYPE: mappingData.tokenType
        }, { transaction: t });
        return appKeyMapping;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateApplicationKeyMapping = async (apiID, mappingData, t) => {
    try {
        const [updatedRowsCount, appContent] = await ApplicationKeyMapping.update({
            API_REF_ID: mappingData.apiRefID,
            CP_APP_REF: mappingData.cpAppRef,
            SUBSCRIPTION_REF_ID: mappingData.subscriptionRefID,
            SHARED_TOKEN: mappingData.sharedToken,
            TOKEN_TYPE: mappingData.tokenType
        },
            {
                where: {
                    ORG_ID: mappingData.orgID,
                    APP_ID: mappingData.appID,
                    API_REF_ID: apiID
                },
                transaction: t
            });

        return [updatedRowsCount, appContent];
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

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
    createIdentityProvider,
    updateIdentityProvider,
    getIdentityProvider,
    deleteIdentityProvider,
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
    deleteAppKeyMapping,
    getAPISubscriptionReference,
    getApplicationID,
    createAppKeyMapping,
    getKeyMapping,
    getAppApiSubscription,
    getSubscribedAPIs,
    getApplicationKeyMapping,
    createApplicationKeyMapping,
    updateApplicationKeyMapping,
    getApplicationAPIMapping,
    deleteAppMappings
};
