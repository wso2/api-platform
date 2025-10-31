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
const config = require(process.cwd() + '/config');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const constants = require('../utils/constants');
const path = require('path');
const fs = require('fs');
const adminDao = require('../dao/admin');
const apiMetadata = require('../dao/apiMetadata');
const util = require('../utils/util');
const filePrefix = config.pathToContent;
const controlPlaneUrl = config.controlPlane.url;
const { ApplicationDTO } = require('../dto/application');
const APIDTO = require('../dto/apiDTO');
const adminService = require('../services/adminService');
const baseURLDev = config.baseUrl + constants.ROUTE.VIEWS_PATH;

const orgIDValue = async (orgName) => {
    const organization = await adminDao.getOrganization(orgName);
    return organization.ORG_ID;
}

const templateResponseValue = async (pageName) => {
    const completeTemplatePath = path.join(require.main.filename, '..', 'pages', pageName, 'page.hbs');
    return fs.readFileSync(completeTemplatePath, constants.CHARSET_UTF8);
}

// ***** Load Applications *****

const loadApplications = async (req, res) => {

    const viewName = req.params.viewName;
    const orgName = req.params.orgName;
    const orgDetails = await adminDao.getOrganization(orgName);
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;
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
                    return {
                        ...new ApplicationDTO(application),
                        subscriptionCount: subApis.length
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
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.API_TYPE.DEFAULT;
    try {
        const applicationId = req.params.applicationId;
        if (config.mode === constants.DEV_MODE) {
            metaData = await getMockApplication();
            kMmetaData = await getMockKeyManagers();
            templateContent = {
                applicationMetadata: metaData,
                keyManagersMetadata: kMmetaData,
                baseUrl: baseURLDev + viewName,
                features: {
                    sdkGeneration: config.features?.sdkGeneration?.enabled || false
                }
            }
            html = renderTemplate('../pages/application/page.hbs', filePrefix + 'layout/main.hbs', templateContent, true);
        } else {
            const orgName = req.params.orgName;
            const orgID = await orgIDValue(orgName);
            let groupList = [];
            if (req.query.groups) {
                groupList.push(req.query.groups.split(" "));
            }
            const subAPIs = await adminDao.getSubscribedAPIs(orgID, applicationId);
            const allAPIs = await apiMetadata.getAllAPIMetadata(orgID, groupList, viewName);

            const subscribedAPIIds = new Set(subAPIs.map(api => api.API_ID));
            const nonSubscribedAPIs = allAPIs
                .filter(api => !subscribedAPIIds.has(api.API_ID) && api.DP_SUBSCRIPTION_POLICies.length > 0)
                .map(api => new APIDTO(api));

            const userID = req[constants.USER_ID]
            const applicationList = await adminService.getApplicationKeyMap(orgID, applicationId, userID);
            metaData = applicationList;
            let applicationReference = "";
            let applicationKeyList;
            if (applicationList.appMap) {
                applicationReference = applicationList.appMap[0].appRefID;
                applicationKeyList = await getApplicationKeys(applicationList.appMap, req);
            }
            let otherAPICount = 0;
            let mcpAPICount = 0;
            let subList = [];
            if (subAPIs.length > 0) {

                subList = await Promise.all(subAPIs.map(async (sub) => {
                    const api = new APIDTO(sub);
                    let apiDTO = {};
                    apiDTO.apiInfo = {};
                    apiDTO.name = api.apiInfo.apiName;
                    apiDTO.apiID = api.apiID;
                    apiDTO.version = api.apiInfo.apiVersion;
                    apiDTO.apiType = api.apiInfo.apiType;
                    apiDTO.apiInfo.apiImageMetadata = api.apiInfo.apiImageMetadata;
                    apiDTO.image = api.apiInfo.apiImageMetadata["api-icon"];
                    apiDTO.subID = sub.dataValues.DP_APPLICATIONs[0].dataValues.DP_API_SUBSCRIPTION.dataValues.SUB_ID;
                    apiDTO.policyID = sub.dataValues.DP_APPLICATIONs[0].dataValues.DP_API_SUBSCRIPTION.dataValues.POLICY_ID;
                    apiDTO.refID = api.apiReferenceID;
                    apiDTO.apiHandle = api.apiHandle;
                    const apiDetails = await getAPIDetails(req, api.apiReferenceID);
                    const projectIdEntry = apiDetails?.additionalProperties?.find(item => item.name === 'projectId');
                    const projectId = projectIdEntry?.value;
                    if (apiDetails) {
                        apiDTO.security = apiDetails.securityScheme;
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
                    const apiKeys = await getAPIKeys(req, api.apiReferenceID, applicationReference);
                    apiDTO.apiKeys = apiKeys;
                    apiDTO.subscriptionPolicyDetails = api.subscriptionPolicies;
                    apiDTO.scopes = apiDetails.scopes.map(scope => scope.key);
                    return apiDTO
                }));
            }
            let isApiKey = false
            let apiKeyList = subList.filter(api => api.security !== null && api.security.includes('api_key'));
            if (apiKeyList.length > 0) {
                isApiKey = true;
            }
            util.appendAPIImageURL(subList, req, orgID);

            await Promise.all(nonSubscribedAPIs.map(async (api) => {
                api.subscriptionPolicyDetails = await util.appendSubscriptionPlanDetails(orgID, api.subscriptionPolicies);
            }));
            kMmetaData = await getAPIMKeyManagers(req);
            kMmetaData = kMmetaData.filter(keyManager => keyManager.enabled);

            //TODO: handle multiple KM scenarios
            //select only one KM for chroeo.

            if (Array.isArray(kMmetaData) && kMmetaData.length > 1) {
                kMmetaData = kMmetaData.filter(keyManager =>
                    keyManager.name.includes("_internal_key_manager_") ||
                    (!kMmetaData.some(km => km.name.includes("_internal_key_manager_")) && keyManager.name.includes("Resident Key Manager")) ||
                    (!kMmetaData.some(km => km.name.includes("_internal_key_manager_") || km.name.includes("Resident Key Manager")) && keyManager.name.includes("_appdev_sts_key_manager_") && keyManager.name.endsWith("_prod"))
                );
            }

            for (var keyManager of kMmetaData) {
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
            });

            let cpApplication = await getAPIMApplication(req, applicationReference);
            let subscriptionScopes = [];
            if (Array.isArray(cpApplication?.subscriptionScopes)) {
                for (const scope of cpApplication?.subscriptionScopes) {
                    subscriptionScopes.push(scope.key);
                }
            }
            //display only one key type (SANBOX).
            //TODO: handle multiple key types
            let profile = null;
            if (req.user) {
                profile = {
                    imageURL: req.user.imageURL,
                    firstName: req.user.firstName,
                    lastName: req.user.lastName,
                    email: req.user.email,
                }
            }

            templateContent = {
                orgID: orgID,
                applicationMetadata: {
                    ...metaData,
                    subscriptionCount: subList.length
                },
                keyManagersMetadata: kMmetaData,
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                subAPIs: subList,
                nonSubAPIs: nonSubscribedAPIs,
                productionKeys: productionKeys,
                isProduction: true,
                isApiKey: isApiKey,
                subscriptionScopes: subscriptionScopes,
                otherAPICount: otherAPICount,
                mcpAPICount: mcpAPICount,
                profile: req.isAuthenticated() ? profile : null,
                devportalMode: devportalMode,
                features: {
                    sdkGeneration: config.features?.sdkGeneration?.enabled || false
                },
                isReadOnlyMode: config.readOnlyMode
            }
            const templateResponse = await templateResponseValue('application');
            const layoutResponse = await loadLayoutFromAPI(orgID, viewName);
            if (layoutResponse === "") {
                html = renderTemplate('../pages/application/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            } else {
                html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
            }
        }
    } catch (error) {
        logger.error("Error occurred while loading application", {
            orgName: orgName,
            applicationId: applicationId,
            error: error.message,
            stack: error.stack
        });
        const templateContent = {
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            devportalMode: devportalMode,
            features: {
                sdkGeneration: config.features?.sdkGeneration?.enabled || false
            }
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

async function getAPIKeys(req, apiId, applicationId) {
    const responseData = await invokeApiRequest(req, 'GET', controlPlaneUrl + `/api-keys?apiId=${apiId}&keyType=PRODUCTION`, null, null);
    let apiKeys = {};
    for (const key of responseData) {
        if (key.application.id === applicationId) {
            apiKeys.key = key.keys;
            apiKeys.scopes = key.scopes;
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
    loadApplication
};
