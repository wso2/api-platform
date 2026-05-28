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
const { decrypt } = require('../utils/cryptoUtil');
const sequelize = require('../db/sequelize');

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

const updateOrganization = async (orgData, t) => {
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

const createIdentityProvider = async (orgId, idpData, t) => {
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
        }, { transaction: t });
        return idpResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateIdentityProvider = async (orgID, idpData, t) => {
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
                where: { ORG_ID: orgID },
                returning: true,
                transaction: t,
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


const getSubscribedAPIs = async (orgID, appID) => {
    try {
        const subscribedAPIs = await APIMetadata.findAll({
            where: { ORG_ID: orgID },
            include: [{
                model: Application,
                where: { APP_ID: appID },
                required: true,
                through: { attributes: ["SUB_ID", "POLICY_ID", "BILLING_CUSTOMER_ID", "BILLING_SUBSCRIPTION_ID", "PAYMENT_STATUS"] }
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
                KM_ID: mappingData.kmID,
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

/**
 * Update billing-related fields for a subscription (for monetization)
 */
const updateBillingFields = async (orgID, subID, fields, t) => {
    try {
        const updateData = {};
        if (fields.BILLING_CUSTOMER_ID !== undefined) updateData.BILLING_CUSTOMER_ID = fields.BILLING_CUSTOMER_ID;
        if (fields.BILLING_SUBSCRIPTION_ID !== undefined) updateData.BILLING_SUBSCRIPTION_ID = fields.BILLING_SUBSCRIPTION_ID;
        if (fields.PAYMENT_PROVIDER !== undefined) updateData.PAYMENT_PROVIDER = fields.PAYMENT_PROVIDER;
        if (fields.PAYMENT_STATUS !== undefined) updateData.PAYMENT_STATUS = fields.PAYMENT_STATUS;
        if (fields.CHECKOUT_SESSION_ID !== undefined) updateData.CHECKOUT_SESSION_ID = fields.CHECKOUT_SESSION_ID;
        if (Object.keys(updateData).length === 0) {
            throw new Sequelize.ValidationError('No billing fields provided for update');
        }
        const [updatedCount] = await SubscriptionMapping.update(updateData, {
            where: { ORG_ID: orgID, SUB_ID: subID },
            transaction: t,
        });
        return updatedCount;
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Find subscription by unique key (app, api, policy)
 */
const findSubscriptionByUniqueKey = async (orgID, appID, apiID, policyID, t) => {
    try {
        return await SubscriptionMapping.findOne({
            where: { ORG_ID: orgID, APP_ID: appID, API_ID: apiID, POLICY_ID: policyID },
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
            where: {
                ORG_ID: orgID,
                [Sequelize.Op.or]: [
                    { PAYMENT_STATUS: null },
                    { PAYMENT_STATUS: { [Sequelize.Op.notIn]: ['PENDING', 'PAYMENT_FAILED'] } },
                ],
            },
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
        const userApps = await Application.findAll({
            where: { ORG_ID: orgID, CREATED_BY: userID },
            attributes: ['APP_ID'],
        });
        const appIds = userApps.map((app) => app.APP_ID);
        if (appIds.length === 0) return [];
        return await SubscriptionMapping.findAll({
            where: {
                ORG_ID: orgID,
                APP_ID: { [Sequelize.Op.in]: appIds },
                [Sequelize.Op.or]: [
                    { PAYMENT_STATUS: null },
                    { PAYMENT_STATUS: { [Sequelize.Op.notIn]: ['PENDING', 'PAYMENT_FAILED'] } },
                ],
            },
        });
    } catch (error) {
        logger.error('listSubscriptionsByUser failed', { error, orgID, userID });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Get active subscriptions with full details for billing page
 */
const getSubscriptionsWithDetails = async (orgID) => {
    try {
        if (!orgID) throw new Error('orgID is required');
        const results = await sequelize.query(
            `
            SELECT 
                s."SUB_ID" as id,
                s."PAYMENT_STATUS" as status,
                s."BILLING_SUBSCRIPTION_ID",
                s."PAYMENT_PROVIDER",
                a."API_NAME" as "apiName",
                app."NAME" as "applicationName",
                p."POLICY_NAME" as "planName",
                p."BILLING_PERIOD" as "billingCycle"
            FROM "DP_API_SUBSCRIPTION" s
            LEFT JOIN "DP_API_METADATA" a ON s."API_ID" = a."API_ID" AND s."ORG_ID" = a."ORG_ID"
            LEFT JOIN "DP_APPLICATION" app ON s."APP_ID" = app."APP_ID" AND s."ORG_ID" = app."ORG_ID"
            LEFT JOIN "DP_SUBSCRIPTION_POLICY" p ON s."POLICY_ID" = p."POLICY_ID" AND s."ORG_ID" = p."ORG_ID"
            WHERE s."ORG_ID" = :orgID
            AND p."BILLING_PLAN" = 'COMMERCIAL'
            AND s."BILLING_CUSTOMER_ID" IS NOT NULL
            AND s."BILLING_SUBSCRIPTION_ID" IS NOT NULL
            `,
            { replacements: { orgID }, type: sequelize.QueryTypes.SELECT },
        );
        return results || [];
    } catch (error) {
        logger.error('getSubscriptionsWithDetails failed', { error, orgID });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Get subscription details (API, application, plan info) for a given userId and orgId
 * Only subscriptions for applications owned by the user are included
 */
const getUserSubscriptionsWithDetails = async (orgID, userID) => {
    try {
        if (!orgID || !userID) throw new Error('orgID and userID are required');
        const results = await sequelize.query(
            `
            SELECT 
                s."SUB_ID" as id,
                s."PAYMENT_STATUS",
                s."BILLING_SUBSCRIPTION_ID",
                s."BILLING_CUSTOMER_ID",
                s."PAYMENT_PROVIDER",
                a."API_NAME" as "apiName",
                app."NAME" as "applicationName",
                p."POLICY_NAME" as "planName",
                p."BILLING_PERIOD" as "billingCycle",
                p."FLAT_AMOUNT" as "flatAmount",
                p."UNIT_AMOUNT" as "unitAmount",
                p."PRICING_MODEL" as "pricingModel",
                p."PRICING_METADATA" as "pricingMetadata",
                asp."BILLING_METER_ID" as "meterId",
                (
                    SELECT akm."SUBSCRIPTION_REF_ID"
                    FROM "DP_APP_KEY_MAPPING" akm
                    WHERE akm."APP_ID" = s."APP_ID"
                      AND akm."API_REF_ID" = a."REFERENCE_ID"
                      AND akm."ORG_ID" = s."ORG_ID"
                    LIMIT 1
                ) as "apimSubscriptionId"
            FROM "DP_API_SUBSCRIPTION" s
            LEFT JOIN "DP_API_METADATA" a ON s."API_ID" = a."API_ID" AND s."ORG_ID" = a."ORG_ID"
            LEFT JOIN "DP_APPLICATION" app ON s."APP_ID" = app."APP_ID" AND s."ORG_ID" = app."ORG_ID"
            LEFT JOIN "DP_SUBSCRIPTION_POLICY" p ON s."POLICY_ID" = p."POLICY_ID" AND s."ORG_ID" = p."ORG_ID"
            LEFT JOIN "DP_API_SUBSCRIPTION_POLICY" asp ON s."API_ID" = asp."API_ID" AND p."POLICY_ID" = asp."POLICY_ID"
            WHERE s."ORG_ID" = :orgID
            AND app."CREATED_BY" = :userID
            AND p."BILLING_PLAN" = 'COMMERCIAL'
            AND s."BILLING_CUSTOMER_ID" IS NOT NULL
            AND s."BILLING_SUBSCRIPTION_ID" IS NOT NULL
            `,
            { replacements: { orgID, userID }, type: sequelize.QueryTypes.SELECT },
        );
        return results || [];
    } catch (error) {
        logger.error('getUserSubscriptionsWithDetails failed', { error, orgID, userID });
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Update subscription by Stripe billing subscription ID (for webhooks)
 */
const updateSubscriptionByBillingId = async (billingSubscriptionID, fields) => {
    try {
        if (!billingSubscriptionID) throw new Sequelize.ValidationError('billingSubscriptionID is required');
        const updateData = {};
        if (fields.PAYMENT_STATUS !== undefined) updateData.PAYMENT_STATUS = fields.PAYMENT_STATUS;
        if (fields.BILLING_CUSTOMER_ID !== undefined) updateData.BILLING_CUSTOMER_ID = fields.BILLING_CUSTOMER_ID;
        if (fields.PAYMENT_PROVIDER !== undefined) updateData.PAYMENT_PROVIDER = fields.PAYMENT_PROVIDER;
        if (Object.keys(updateData).length === 0) throw new Sequelize.ValidationError('No fields provided for update');
        const [updatedCount] = await SubscriptionMapping.update(updateData, {
            where: { BILLING_SUBSCRIPTION_ID: billingSubscriptionID },
        });
        return updatedCount;
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Activate a PENDING subscription by its checkout session ID (webhook fallback).
 * Only updates rows still in PENDING state to avoid overwriting later status changes.
 */
const activateSubscriptionByCheckoutSessionId = async (checkoutSessionId, fields) => {
    try {
        if (!checkoutSessionId) throw new Sequelize.ValidationError('checkoutSessionId is required');
        const updateData = { PAYMENT_STATUS: fields.PAYMENT_STATUS };
        if (fields.BILLING_SUBSCRIPTION_ID !== undefined) {
            updateData.BILLING_SUBSCRIPTION_ID = fields.BILLING_SUBSCRIPTION_ID;
        }
        const [updatedCount] = await SubscriptionMapping.update(updateData, {
            where: {
                CHECKOUT_SESSION_ID: checkoutSessionId,
                PAYMENT_STATUS: 'PENDING',
            },
        });
        return updatedCount;
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Delete subscription by Stripe billing subscription ID (for webhooks)
 */
const deleteSubscriptionByBillingId = async (billingSubscriptionID, t) => {
    try {
        if (!billingSubscriptionID) throw new Sequelize.ValidationError('billingSubscriptionID is required');
        const deletedRowsCount = await SubscriptionMapping.destroy({
            where: { BILLING_SUBSCRIPTION_ID: billingSubscriptionID },
            transaction: t,
        });
        if (deletedRowsCount < 1) throw new Sequelize.EmptyResultError('Subscription not found');
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Get API by ID (for monetization validation)
 */
const getAPIById = async (orgID, apiID, t) => {
    try {
        return await APIMetadata.findOne({
            where: { ORG_ID: orgID, API_ID: apiID },
            transaction: t,
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) return null;
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Get subscription policy by ID (for monetization validation)
 */
const getSubscriptionPolicyById = async (orgID, policyID, t) => {
    try {
        return await SubscriptionPolicy.findOne({
            where: { ORG_ID: orgID, POLICY_ID: policyID },
            transaction: t,
        });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) return null;
        throw new Sequelize.DatabaseError(error);
    }
};

/**
 * Get subscription with meter ID for usage tracking
 */
const getSubscriptionWithMeter = async (orgID, subID, t) => {
    try {
        const sequelize = require('../db/sequelize');
        const results = await sequelize.query(
            `
            SELECT
                s.*,
                asp."BILLING_METER_ID" as "meterId",
                (
                    SELECT akm."SUBSCRIPTION_REF_ID"
                    FROM "DP_APP_KEY_MAPPING" akm
                    JOIN "DP_API_METADATA" a ON a."API_ID" = s."API_ID"
                    WHERE akm."APP_ID" = s."APP_ID"
                      AND akm."API_REF_ID" = a."REFERENCE_ID"
                      AND akm."ORG_ID" = s."ORG_ID"
                    LIMIT 1
                ) as "apimSubscriptionId"
            FROM "DP_API_SUBSCRIPTION" s
            LEFT JOIN "DP_API_SUBSCRIPTION_POLICY" asp
                ON s."API_ID" = asp."API_ID" AND s."POLICY_ID" = asp."POLICY_ID"
            WHERE s."ORG_ID" = :orgID AND s."SUB_ID" = :subID
            LIMIT 1
            `,
            { replacements: { orgID, subID }, type: sequelize.QueryTypes.SELECT, transaction: t },
        );
        return results?.[0] || null;
    } catch (error) {
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
    upsertApplicationKeyMapping,
    updateApplicationKeyMapping,
    getApplicationAPIMapping,
    deleteAppMappings,
    updateBillingFields,
    findSubscriptionByUniqueKey,
    listSubscriptionsByOrg,
    listSubscriptionsByUser,
    getSubscriptionsWithDetails,
    getUserSubscriptionsWithDetails,
    updateSubscriptionByBillingId,
    activateSubscriptionByCheckoutSessionId,
    deleteSubscriptionByBillingId,
    getAPIById,
    getSubscriptionPolicyById,
    getSubscriptionWithMeter,
};
