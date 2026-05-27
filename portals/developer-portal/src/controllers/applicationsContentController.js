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
/* eslint-disable no-undef */
const { renderTemplate, renderGivenTemplate, loadLayoutFromAPI, invokeApiRequest } = require('../utils/util');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const constants = require('../utils/constants');
const path = require('path');
const fs = require('fs');
const adminDao = require('../dao/admin');
const apiMetadata = require('../dao/apiMetadata');
const apiKeyDao = require('../dao/apiKey');
const platformSubDao = require('../dao/platformSubscription');
const util = require('../utils/util');
const yaml = require('js-yaml');
const filePrefix = config.pathToContent;
const controlPlaneUrl = config.controlPlane.url;
const { ApplicationDTO } = require('../dto/application');
const APIDTO = require('../dto/apiDTO');
const adminService = require('../services/adminService');
const baseURLDev = config.baseUrl + constants.ROUTE.VIEWS_PATH;

const parseApiDefinition = (definition) => {
    if (!definition) {
        return null;
    }
    if (typeof definition !== 'string') {
        return definition;
    }
    try {
        return JSON.parse(definition);
    } catch (jsonError) {
        try {
            return yaml.load(definition);
        } catch (yamlError) {
            logger.warn('Failed to parse API definition', {
                error: yamlError.message || jsonError.message
            });
            return null;
        }
    }
};

const apiDefinitionHasApiKeySecurity = (apiDefinition) => {
    if (!apiDefinition || typeof apiDefinition !== 'object') {
        return false;
    }

    const securitySchemes = apiDefinition.components?.securitySchemes || apiDefinition.securityDefinitions;
    if (securitySchemes && typeof securitySchemes === 'object') {
        return Object.values(securitySchemes).some((scheme) => {
            if (!scheme || typeof scheme !== 'object') {
                return false;
            }
            const type = String(scheme.type || '').toLowerCase();
            return type === 'apikey' || type === 'httpapikey' || type === 'http_api_key';
        });
    }

    return false;
};

const loadApiDefinitionFromDb = async (orgID, apiID) => {
    try {
        const apiFile = await apiMetadata.getAPIDoc(constants.DOC_TYPES.API_DEFINITION, orgID, apiID);
        const rawDefinition = apiFile?.API_FILE?.toString(constants.CHARSET_UTF8);
        return parseApiDefinition(rawDefinition);
    } catch (error) {
        logger.warn('Failed to load API definition from DB', {
            orgID,
            apiID,
            error: error.message
        });
        return null;
    }
};

const buildPlatformApiKeyPayload = (keyRow) => {
    if (!keyRow) {
        return {};
    }
    return {
        key: [{ id: keyRow.KEY_ID }],
        name: keyRow.NAME,
        scopes: []
    };
};

const orgIDValue = async (orgName) => {
    const organization = await adminDao.getOrganization(orgName);
    return organization.ORG_ID;
}

const templateResponseValue = async (pageName) => {
    const completeTemplatePath = path.join(require.main.filename, '..', 'pages', pageName, 'page.hbs');
    return fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
}

const buildProfile = (req) => {
    if (!req?.user) {
        return null;
    }
    return {
        imageURL: req.user.imageURL,
        firstName: req.user.firstName,
        lastName: req.user.lastName,
        email: req.user.email,
        isAdmin: req.user.isAdmin,
    };
};

/**
 * Shared data loader for both application overview and manage-keys pages.
 * Keeps loadApplication / loadApplicationKeys lean and avoids drift between the two.
 */
const loadApplicationData = async (req, orgName, applicationId, viewName) => {
    const orgID = await orgIDValue(orgName);

    let groupList = [];
    if (req.query.groups) {
        groupList.push(req.query.groups.split(" "));
    }

    const subAPIs = await adminDao.getSubscribedAPIs(orgID, applicationId);

    const filteredSubAPIs = subAPIs.filter(sub => {
        const ps = sub.dataValues.DP_APPLICATIONs?.[0]?.dataValues?.DP_API_SUBSCRIPTION?.dataValues?.PAYMENT_STATUS;
        return !ps || ps === 'ACTIVE';
    });

    const allAPIs = await apiMetadata.getAllAPIMetadata(orgID, groupList, viewName);

    const subscribedAPIIds = new Set(filteredSubAPIs.map(api => api.API_ID));
    const nonSubscribedAPIs = allAPIs
        .filter(api => !subscribedAPIIds.has(api.API_ID) && api.DP_SUBSCRIPTION_POLICies.length > 0)
        .map(api => new APIDTO(api));

    const userID = req[constants.USER_ID]
    const applicationList = await adminService.getApplicationKeyMap(orgID, applicationId, userID);

    let applicationReference = "";
    let applicationKeyList;
    if (applicationList.appMap && config.controlPlane?.enabled !== false) {
        applicationReference = applicationList.appMap[0].appRefID;
        try {
            applicationKeyList = await getApplicationKeys(applicationList.appMap, req);
        } catch (keyError) {
            logger.warn('Failed to fetch application keys from CP', {
                appRefID: applicationReference, error: keyError.message
            });
        }
    }

    let otherAPICount = 0;
    let mcpAPICount = 0;
    let apiKeyEnabledAPICount = 0;
    const PLATFORM_GATEWAY_TYPE = 'wso2/api-platform';

    let subList = [];
    if (filteredSubAPIs.length > 0) {
        subList = await Promise.all(filteredSubAPIs.map(async (sub) => {
            const api = new APIDTO(sub);
            let apiDTO = {};
            apiDTO.apiInfo = {
                apiName: api.apiInfo.apiName,
                apiVersion: api.apiInfo.apiVersion,
                apiDescription: api.apiInfo.apiDescription,
                apiType: api.apiInfo.apiType,
                gatewayType: api.apiInfo.gatewayType || null,
                apiImageMetadata: api.apiInfo.apiImageMetadata
            };
            apiDTO.name = api.apiInfo.apiName;
            apiDTO.apiID = api.apiID;
            apiDTO.version = api.apiInfo.apiVersion;
            apiDTO.apiType = api.apiInfo.apiType;
            apiDTO.gatewayType = api.apiInfo.gatewayType || null;
            apiDTO.image = api.apiInfo.apiImageMetadata["api-icon"];
            apiDTO.subID = sub.dataValues.DP_APPLICATIONs[0].dataValues.DP_API_SUBSCRIPTION.dataValues.SUB_ID;
            apiDTO.policyID = sub.dataValues.DP_APPLICATIONs[0].dataValues.DP_API_SUBSCRIPTION.dataValues.POLICY_ID;
            apiDTO.refID = api.apiReferenceID;
            apiDTO.apiHandle = api.apiHandle;

            let apiDetails = null;
            if (config.controlPlane?.enabled !== false) {
                try {
                    apiDetails = await getAPIDetails(req, api.apiReferenceID);
                } catch (apiError) {
                    logger.warn('Failed to fetch API details from CP', {
                        apiReferenceID: api.apiReferenceID, error: apiError.message
                    });
                }
            }
            const projectIdEntry = apiDetails?.additionalProperties?.find(item => item.name === 'projectId');
            const projectId = projectIdEntry?.value;
            if (apiDetails) {
                apiDTO.security = apiDetails.securityScheme;
                if (apiDTO.security && apiDTO.security.includes('api_key') && apiDTO.gatewayType !== PLATFORM_GATEWAY_TYPE) {
                    apiKeyEnabledAPICount++;
                }
            }
            if (projectId) {
                apiDTO.projectId = projectId;
            }
            const subPolicy = await apiMetadata.getSubscriptionPolicy(apiDTO.policyID, orgID);
            if (subPolicy) {
                apiDTO.policyName = subPolicy.dataValues.POLICY_NAME;
            }
            if (constants.API_TYPE.MCP === api.apiInfo.apiType) {
                mcpAPICount++;
            } else {
                otherAPICount++;
            }
            let productionApiKeys = [];
            let sandboxApiKeys = [];
            if (config.controlPlane?.enabled !== false) {
                try {
                    productionApiKeys = await getAPIKeys(req, api.apiReferenceID, applicationReference, 'PRODUCTION');
                    sandboxApiKeys = await getAPIKeys(req, api.apiReferenceID, applicationReference, 'SANDBOX');
                } catch (keyError) {
                    logger.warn('Failed to fetch API keys from CP', {
                        apiReferenceID: api.apiReferenceID, error: keyError.message
                    });
                }
            }
            apiDTO.apiKeys = {
                production: productionApiKeys,
                sandbox: sandboxApiKeys
            };
            apiDTO.subscriptionPolicyDetails = await util.appendSubscriptionPlanDetails(orgID, api.subscriptionPolicies);
            apiDTO.scopes = (apiDetails?.scopes || []).map(scope => scope.key);
            return apiDTO
        }));
    }

    util.appendAPIImageURL(subList, req, orgID);

    const subAPIsForApplicationKeys = subList.filter(s => s.gatewayType !== PLATFORM_GATEWAY_TYPE);
    const isApiKey = subAPIsForApplicationKeys.some(
        api => api.security && api.security.includes('api_key')
    );

    await Promise.all(nonSubscribedAPIs.map(async (api) => {
        api.subscriptionPolicyDetails = await util.appendSubscriptionPlanDetails(orgID, api.subscriptionPolicies);
    }));

    let kMmetaData = [];
    if (config.controlPlane?.enabled !== false) {
        try {
            kMmetaData = await getAPIMKeyManagers(req);
        } catch (kmError) {
            logger.warn('Failed to fetch key managers from CP', { error: kmError.message });
        }
    }

    // Ensure kMmetaData is an array before filtering
    if (!Array.isArray(kMmetaData)) {
        kMmetaData = [];
    }

    kMmetaData = kMmetaData.filter(keyManager => keyManager.enabled);

    // TODO: Instead of using priority-based filtering, we should identify the key manager
    // configured for the production environment from the Bijira console configuration.
    // This temporary priority-based approach should be replaced with a proper configuration-based selection.
    if (Array.isArray(kMmetaData) && kMmetaData.length > 1) {
        kMmetaData = kMmetaData.filter(keyManager =>
            keyManager.name.includes("_internal_key_manager_") ||
            (!kMmetaData.some(km => km.name.includes("_internal_key_manager_")) && keyManager.name.includes("Resident Key Manager")) ||
            (!kMmetaData.some(km => km.name.includes("_internal_key_manager_") || km.name.includes("Resident Key Manager")) && keyManager.name.includes("_appdev_sts_key_manager_") && keyManager.name.endsWith("_prod"))
        );
    }

    for (const keyManager of kMmetaData) {
        if (keyManager.name === 'Resident Key Manager') {
            keyManager.tokenEndpoint = 'https://sts.choreo.dev/oauth2/token';
            keyManager.authorizeEndpoint = 'https://sts.choreo.dev/oauth2/authorize';
            keyManager.revokeEndpoint = 'https://sts.choreo.dev/oauth2/revoke';
        }
        keyManager.availableGrantTypes = await mapGrants(keyManager.availableGrantTypes);
        keyManager.applicationConfiguration = await mapDefaultValues(keyManager.applicationConfiguration);
    }

    let productionKeys = [];
    let sandboxKeys = [];

    applicationKeyList?.list?.map(key => {
        let client_name;
        if (key?.additionalProperties?.client_name) {
            client_name = key.additionalProperties.client_name;
        }
        let keyData = {
            keyManager: key.keyManager,
            consumerKey: key.consumerKey,
            consumerSecret: key.consumerSecret,
            keyMappingId: key.keyMappingId,
            keyType: key.keyType,
            supportedGrantTypes: key.supportedGrantTypes,
            additionalProperties: key.additionalProperties,
            clientName: client_name,
            callbackUrl: key.callbackUrl,
            appRefID: applicationReference
        };
        if (key.keyType === constants.KEY_TYPE.PRODUCTION) {
            productionKeys.push(keyData);
        } else {
            sandboxKeys.push(keyData);
        }
        return keyData;
    }) || [];

    kMmetaData.forEach(keyManager => {
        productionKeys.forEach(productionKey => {
            if (productionKey.keyManager === keyManager.name) {
                keyManager.productionKeys = productionKey;
            }
        });
        sandboxKeys.forEach(sandboxKey => {
            if (sandboxKey.keyManager === keyManager.name) {
                keyManager.sandboxKeys = sandboxKey;
            }
        });
        // Build applicationKeys per keyManager with single objects (not arrays)
        keyManager.applicationKeys = [
            {
                keys: keyManager.productionKeys || {},
                keyType: 'PRODUCTION'
            },
            {
                keys: keyManager.sandboxKeys || {},
                keyType: 'SANDBOX'
            }
        ];
    });

    let subscriptionScopes = [];
    if (applicationReference && config.controlPlane?.enabled !== false) {
        try {
            const cpApplication = await getAPIMApplication(req, applicationReference);
            if (cpApplication && Array.isArray(cpApplication.subscriptionScopes)) {
                for (const scope of cpApplication.subscriptionScopes) {
                    subscriptionScopes.push(scope.key);
                }
            }
        } catch (appError) {
            logger.warn('Failed to fetch application from CP', {
                applicationReference, error: appError.message
            });
        }
    }

    // Load TOKEN-BASED (platform) subscriptions from local DB
    let platformSubscriptions = [];
    if (req.user) {
        try {
            const localSubs = await platformSubDao.listPlatformSubscriptions(orgID);
            platformSubscriptions = await Promise.all(localSubs.map(async (sub) => {
                const apiRefId = sub.DP_API_METADATA?.REFERENCE_ID;
                const apiDefinition = await loadApiDefinitionFromDb(orgID, sub.API_ID);
                const security = apiDefinitionHasApiKeySecurity(apiDefinition) ? ['api_key'] : [];

                let productionApiKeys = {};
                let sandboxApiKeys = {};
                try {
                    const keys = await apiKeyDao.listKeys(orgID, {
                        apiId: sub.API_ID,
                        subscriptionId: sub.SUB_ID,
                        status: 'ACTIVE'
                    });
                    productionApiKeys = buildPlatformApiKeyPayload(keys?.[0]);
                } catch (keyError) {
                    logger.warn('Failed to fetch platform subscription API keys from DB', {
                        apiId: sub.API_ID, error: keyError.message
                    });
                }

                return {
                    id: sub.SUB_ID,
                    subID: sub.SUB_ID,
                    apiID: sub.API_ID,
                    name: sub.DP_API_METADATA?.API_NAME || '',
                    apiName: sub.DP_API_METADATA?.API_NAME || '',
                    version: sub.DP_API_METADATA?.API_VERSION || '',
                    apiVersion: sub.DP_API_METADATA?.API_VERSION || '',
                    apiHandle: sub.DP_API_METADATA?.API_HANDLE || '#',
                    apiType: sub.DP_API_METADATA?.API_TYPE || 'REST',
                    refID: apiRefId || sub.API_ID,
                    planName: sub.DP_SUBSCRIPTION_POLICY?.POLICY_NAME || '',
                    policyName: sub.DP_SUBSCRIPTION_POLICY?.POLICY_NAME || '',
                    status: sub.STATUS,
                    subscriptionToken: sub.SUB_TOKEN,
                    security,
                    apiKeys: {
                        production: productionApiKeys,
                        sandbox: sandboxApiKeys
                    },
                    scopes: [],
                    isPlatformSubscription: true,
                };
            }));
        } catch (err) {
            logger.warn('Failed to load platform subscriptions for application overview', {
                error: err.message
            });
        }
    }

    // Load platform APIs that don't require subscription (gatewayType=wso2/api-platform, no subscription plans)
    let noSubPlatformAPIs = [];
    try {
        const noSubApis = await apiMetadata.getAPIMetadataByCondition({
            ORG_ID: orgID,
            GATEWAY_TYPE: 'wso2/api-platform',
            STATUS: constants.API_STATUS.PUBLISHED
        });

        const filteredNoSubApis = (noSubApis || []).filter(api => {
            const policies = api.DP_SUBSCRIPTION_POLICies || [];
            return policies.length === 0;
        });

        if (filteredNoSubApis.length > 0) {
            noSubPlatformAPIs = await Promise.all(filteredNoSubApis.map(async (api) => {
                const apiDTO = new APIDTO(api);
                const refID = apiDTO.apiReferenceID;

                const apiDefinition = await loadApiDefinitionFromDb(orgID, apiDTO.apiID);
                const security = apiDefinitionHasApiKeySecurity(apiDefinition) ? ['api_key'] : [];

                let productionApiKeys = {};
                let sandboxApiKeys = {};
                try {
                    const keys = await apiKeyDao.listKeys(orgID, {
                        apiId: apiDTO.apiID,
                        subscriptionId: null,
                        status: 'ACTIVE'
                    });
                    productionApiKeys = buildPlatformApiKeyPayload(keys?.[0]);
                } catch (keyError) {
                    logger.warn('Failed to fetch no-sub platform API keys.', {
                        apiReferenceID: refID, error: keyError.message
                    });
                }

                return {
                    subID: apiDTO.apiID,
                    apiID: apiDTO.apiID,
                    name: apiDTO.apiInfo.apiName,
                    version: apiDTO.apiInfo.apiVersion,
                    apiHandle: apiDTO.apiHandle,
                    apiType: apiDTO.apiInfo.apiType || 'REST',
                    refID: refID,
                    policyName: '',
                    security,
                    apiKeys: {
                        production: productionApiKeys,
                        sandbox: sandboxApiKeys
                    },
                    scopes: [],
                    isNoSubPlatformAPI: true,
                };
            }));
        }
    } catch (err) {
        logger.warn('Failed to load no-subscription platform APIs', { error: err.message });
    }

    const profile = buildProfile(req);

    return {
        orgID,
        applicationList,
        keyManagersMetadata: kMmetaData,
        subAPIs: subList,
        subAPIsForApplicationKeys,
        platformSubscriptionsForApplicationKeys: [],
        noSubPlatformAPIsForApplicationKeys: [],
        nonSubAPIs: nonSubscribedAPIs,
        productionKeys,
        sandboxKeys,
        subscriptionScopes,
        otherAPICount,
        mcpAPICount,
        apiKeyEnabledAPICount,
        isApiKey,
        platformSubscriptions,
        noSubPlatformAPIs,
        profile
    };
};

// ***** Load Applications *****

const loadApplications = async (req, res) => {

    const viewName = req.params.viewName;
    const orgName = req.params.orgName;
    const orgDetails = await adminDao.getOrganization(orgName);
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    let html, metaData, templateContent;
    try {
        if (config.mode === constants.DEV_MODE) {
            metaData = await getMockApplications();
            templateContent = {
                applicationsMetadata: metaData,
                baseUrl: baseURLDev + viewName
            }
            html = renderTemplate('../pages/applications/page.hbs', filePrefix + 'layout/main.hbs', templateContent, true);
        } else {
            const orgName = req.params.orgName;
            const orgID = await orgIDValue(orgName);
            const applications = await adminDao.getApplications(orgID, req.user.sub)
            const metaData = await Promise.all(
                applications.map(async (application) => {
                    const subApis = await adminDao.getSubscriptions(orgID, application.APP_ID, '');
                    const activeCount = subApis.filter(s => {
                        const ps = s.PAYMENT_STATUS;
                        return !ps || ps === 'ACTIVE';
                    }).length;
                    return {
                        ...new ApplicationDTO(application),
                        subscriptionCount: activeCount
                    };
                })
            );
            let profile = null;
            if (req.user) {
                profile = {
                    imageURL: req.user.imageURL,
                    firstName: req.user.firstName,
                    lastName: req.user.lastName,
                    email: req.user.email,
                    isAdmin: req.user.isAdmin,
                }
            }

            templateContent = {
                applicationsMetadata: metaData,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                profile: req.isAuthenticated() ? profile : null,
                devportalMode: devportalMode,
                isReadOnlyMode: config.readOnlyMode,
            }
            const templateResponse = await templateResponseValue('applications');
            const layoutResponse = await loadLayoutFromAPI(orgID, viewName);
            if (layoutResponse === "") {
                html = renderTemplate('../pages/applications/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            } else {
                html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
            }
        }
    } catch (error) {
        const templateContent = {
            devportalMode: devportalMode,
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            errorMessage: constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE,
        }
        logger.error("Error occurred while loading Applications", {
            orgName: orgName,
            error: error.message,
            stack: error.stack
        });
        html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
    }
    res.send(html);
}

async function getMockApplications() {
    const mockApplicationsMetaDataPath = path.join(process.cwd(), filePrefix + '../mock/Applications', 'applications.json');
    const mockApplicationsMetaData = JSON.parse(fs.readFileSync(mockApplicationsMetaDataPath, 'utf-8'));
    return mockApplicationsMetaData.list;
}

// ***** Load Application *****

const loadApplication = async (req, res) => {
    let html, templateContent, metaData, kMmetaData;
    const viewName = req.params.viewName;
    const orgName = req.params.orgName;
    const orgDetails = await adminDao.getOrganization(orgName);
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    req.cpOrgID = orgDetails.ORGANIZATION_IDENTIFIER;
    try {
        const applicationId = req.params.applicationId;
        if (config.mode === constants.DEV_MODE) {
            metaData = await getMockApplication();
            kMmetaData = await getMockKeyManagers();
            templateContent = {
                applicationMetadata: metaData,
                keyManagersMetadata: kMmetaData,
                baseUrl: baseURLDev + viewName
            }
            html = renderTemplate('../pages/application/page.hbs', filePrefix + 'layout/main.hbs', templateContent, true);
        } else {
            const data = await loadApplicationData(req, orgName, applicationId, viewName);
            metaData = data.applicationList;
            kMmetaData = data.keyManagersMetadata;

            templateContent = {
                orgID: data.orgID,
                applicationMetadata: {
                    ...metaData,
                    subscriptionCount: data.subAPIs.length
                },
                keyManagersMetadata: kMmetaData,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                subAPIs: data.subAPIs,
                nonSubAPIs: data.nonSubAPIs,
                productionKeys: data.productionKeys,
                sandboxKeys: data.sandboxKeys,
                applicationKeys: [
                    {
                        keys: data.productionKeys,
                        keyType: constants.KEY_TYPE.PRODUCTION
                    },
                    {
                        keys: data.sandboxKeys,
                        keyType: constants.KEY_TYPE.SANDBOX
                    }
                ],
                isProduction: true,
                isApiKey: data.isApiKey,
                subscriptionScopes: data.subscriptionScopes,
                otherAPICount: data.otherAPICount,
                mcpAPICount: data.mcpAPICount,
                apiKeyEnabledAPICount: data.apiKeyEnabledAPICount,
                platformSubscriptions: data.platformSubscriptions,
                noSubPlatformAPIs: data.noSubPlatformAPIs,
                profile: req.isAuthenticated() ? data.profile : null,
                devportalMode: devportalMode,
                isReadOnlyMode: config.readOnlyMode
            }
            const templateResponse = await templateResponseValue('application');
            const layoutResponse = await loadLayoutFromAPI(data.orgID, viewName);
            if (layoutResponse === "") {
                html = renderTemplate('../pages/application/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            } else {
                html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
            }
        }
    } catch (error) {
        logger.error("Error occurred while loading application", {
            orgName: orgName,
            applicationId: req?.params?.applicationId,
            error: error.message,
            stack: error.stack
        });
        const templateContent = {
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            devportalMode: devportalMode,
            profile: req.isAuthenticated() ? req.user : null,
        }
        if (Number(error?.statusCode) === 401) {
            templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE;
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        } else {
            templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE;
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        }
    }
    res.send(html);
}

const loadApplicationKeys = async (req, res) => {
    let html, templateContent, metaData, kMmetaData;
    const viewName = req.params.viewName;
    const orgName = req.params.orgName;
    const orgDetails = await adminDao.getOrganization(orgName);
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    try {
        const applicationId = req.params.applicationId;
        if (config.mode === constants.DEV_MODE) {
            metaData = await getMockApplication();
            kMmetaData = await getMockKeyManagers();
            templateContent = {
                applicationMetadata: metaData,
                keyManagersMetadata: kMmetaData,
                baseUrl: baseURLDev + viewName,
                productionKeys: [],
                sandboxKeys: [],
                subAPIs: [],
                orgID: null,
                subscriptionScopes: [],
                isApiKey: false,
                subAPIsForApplicationKeys: [],
                platformSubscriptionsForApplicationKeys: [],
                noSubPlatformAPIsForApplicationKeys: [],
                isReadOnlyMode: config.readOnlyMode
            }
            html = renderTemplate('../pages/manage-keys/page.hbs', filePrefix + 'layout/main.hbs', templateContent, true);
        } else {
            const data = await loadApplicationData(req, orgName, applicationId, viewName);
            metaData = data.applicationList;
            kMmetaData = data.keyManagersMetadata;

            templateContent = {
                orgID: data.orgID,
                applicationMetadata: {
                    ...metaData,
                    subscriptionCount: data.subAPIs.length
                },
                keyManagersMetadata: kMmetaData,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                subAPIs: data.subAPIs,
                nonSubAPIs: data.nonSubAPIs,
                productionKeys: data.productionKeys,
                sandboxKeys: data.sandboxKeys,
                applicationKeys: [
                    {
                        keys: data.productionKeys,
                        keyType: constants.KEY_TYPE.PRODUCTION
                    },
                    {
                        keys: data.sandboxKeys,
                        keyType: constants.KEY_TYPE.SANDBOX
                    }
                ],
                isApiKey: data.isApiKey,
                subAPIsForApplicationKeys: data.subAPIsForApplicationKeys,
                platformSubscriptions: data.platformSubscriptions,
                platformSubscriptionsForApplicationKeys: data.platformSubscriptionsForApplicationKeys,
                noSubPlatformAPIs: data.noSubPlatformAPIs,
                noSubPlatformAPIsForApplicationKeys: data.noSubPlatformAPIsForApplicationKeys,
                subscriptionScopes: data.subscriptionScopes,
                otherAPICount: data.otherAPICount,
                mcpAPICount: data.mcpAPICount,
                apiKeyEnabledAPICount: data.apiKeyEnabledAPICount,
                profile: req.isAuthenticated() ? data.profile : null,
                devportalMode: devportalMode,
                isReadOnlyMode: config.readOnlyMode
            }
            const templateResponse = await templateResponseValue('manage-keys');
            const layoutResponse = await loadLayoutFromAPI(data.orgID, viewName);
            if (layoutResponse === "") {
                html = renderTemplate('../pages/manage-keys/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            } else {
                html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
            }
        }
    } catch (error) {
        logger.error("Error occurred while loading application keys", {
            orgName: orgName,
            applicationId: req?.params?.applicationId,
            error: error.message,
            stack: error.stack
        });
        const templateContent = {
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            devportalMode: devportalMode,
            profile: req.isAuthenticated() ? req.user : null,
        }
        if (Number(error?.statusCode) === 401) {
            templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE;
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        } else {
            templateContent.errorMessage = constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE;
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        }
    }
    res.send(html);
}

async function getApplicationKeys(applicationList, req) {

    //TODO: handle multiple CP applications
    for (const application of applicationList) {
        const appRef = application.appRefID;
        try {
            return await invokeApiRequest(req, 'GET', `${controlPlaneUrl}/applications/${appRef}/keys`, {}, {});
        } catch (error) {
            logger.error("Error occurred while generating application keys", {
                applicationRefId: appRef,
                error: error.message,
                stack: error.stack
            });
            return null;
        }
    }
}

async function getAllAPIs(req) {
    try {
        return await util.invokeApiRequest(req, 'GET', `${controlPlaneUrl}/apis`);
    } catch (error) {
        logger.error("Error occurred while loading APIs", {
            error: error.message,
            stack: error.stack
        });
        throw error;
    }
}

const getSubscribedApis = async (req, appId) => {
    try {
        return await util.invokeApiRequest(req, 'GET', `${controlPlaneUrl}/subscriptions?applicationId=${appId}`);
    } catch (error) {
        logger.error("Error occurred while loading subscriptions", {
            applicationId: appId,
            error: error.message,
            stack: error.stack
        });
        throw error;
    }
}
async function getMockApplication() {
    const mockApplicationMetaDataPath = path.join(process.cwd(), filePrefix + '../mock/Applications/DefaultApplication', 'DefaultApplication.json');
    const mockApplicationMetaData = JSON.parse(fs.readFileSync(mockApplicationMetaDataPath, 'utf-8'));
    return mockApplicationMetaData;
}

async function getMockKeyManagers() {
    const mockKeyManagersMetaDataPath = path.join(process.cwd(), filePrefix + '../mock/Applications/DefaultApplication', 'AllKeyManagers.json');
    const mockKeyManagersMetaData = JSON.parse(fs.readFileSync(mockKeyManagersMetaDataPath, 'utf-8'));
    return mockKeyManagersMetaData.list;
}

async function getAPIMApplication(req, applicationId) {
    const responseData = await invokeApiRequest(req, 'GET', controlPlaneUrl + '/applications/' + applicationId, null, null);
    return responseData;
}

async function getAPIMKeyManagers(req) {
    const responseData = await invokeApiRequest(req, 'GET', controlPlaneUrl + '/key-managers?devPortalAppEnv=prod', null, null);
    return responseData.list;
}

async function getAPIDetails(req, apiId) {
    const responseData = await invokeApiRequest(req, 'GET', controlPlaneUrl + `/apis/${apiId}`, null, null);
    return responseData;
}

async function getAPIKeys(req, apiId, applicationId, keyType) {
    const responseData = await invokeApiRequest(req, 'GET', controlPlaneUrl + `/api-keys?apiId=${apiId}&keyType=${keyType}`, null, null);
    let apiKeys = {};
    for (const key of responseData) {
        if (key.application.id === applicationId) {
            apiKeys.key = key.keys;
            apiKeys.scopes = key.scopes;
            apiKeys.name = key.alias;
        }
    }
    return apiKeys;
}

async function mapGrants(grantTypes) {

    let mappedGrantTypes = [];
    grantTypes.map(grantType => {
        if (grantType === 'password') {
            mappedGrantTypes.push({
                label: 'Password',
                name: grantType
            });
        } else if (grantType === 'client_credentials') {
            mappedGrantTypes.push(
                {
                    label: 'Client Credentials',
                    name: grantType
                }
            );
        } else if (grantType === 'refresh_token') {
            mappedGrantTypes.push(
                {
                    label: 'Refresh Token',
                    name: grantType
                }
            );
        } else if (grantType === 'authorization_code') {
            mappedGrantTypes.push(
                {
                    label: 'Authorization Code',
                    name: grantType
                }
            );
        } else if (grantType === 'implicit') {
            mappedGrantTypes.push(
                {
                    label: 'Implicit',
                    name: grantType
                }
            );
        }
    });
    return mappedGrantTypes;
}

async function mapDefaultValues(applicationConfiguration) {

    let appConfigs = [];
    let defaultConfigs = ["application_access_token_expiry_time", "user_access_token_expiry_time", "id_token_expiry_time"];
    applicationConfiguration.map(config => {
        if (defaultConfigs.includes(config.name) && config.default == 'N/A') {
            config.default = 900;
        } else if (config.name === 'refresh_token_expiry_time' && config.default == 'N/A') {
            config.default = 86400;
        }
        appConfigs.push(config);
    });
    return appConfigs;
}



module.exports = {
    loadApplications,
    loadApplication,
    loadApplicationKeys
};
