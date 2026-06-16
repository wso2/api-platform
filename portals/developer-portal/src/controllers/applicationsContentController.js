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
const { renderTemplate, renderGivenTemplate, loadLayoutFromAPI } = require('../utils/util');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const path = require('path');
const fs = require('fs');
const adminDao = require('../dao/adminDao');
const { ApplicationDTO } = require('../dto/applicationDto');
const sampleApiLoader = require('../utils/sampleApiLoader');
const kmDao = require('../dao/keyManagerDao');
const adminService = require('../services/adminService');

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

    const userID = req[constants.USER_ID]
    const applicationList = await adminService.getApplicationKeyMap(orgID, applicationId, userID);

    let applicationReference = "";
    let applicationKeyList;
    if (Array.isArray(applicationList.appMap) && applicationList.appMap.length > 0) {
        applicationReference = applicationList.appMap[0].appRefID;
        try {
            const { ApplicationKeyMapping } = require('../models/application');
            const localMappings = await ApplicationKeyMapping.findAll({
                where: { APP_ID: applicationId, ORG_ID: orgID }
            });
            const keyList = [];
            for (const mapping of localMappings) {
                if (mapping.AS_CLIENT_ID && mapping.KM_ID) {
                    try {
                        const km = await kmDao.getKeyManager(mapping.KM_ID);
                        const storedProps = mapping.ADDITIONAL_PROPERTIES || {};
                        keyList.push({
                            keyManager: km.NAME,
                            consumerKey: mapping.AS_CLIENT_ID,
                            consumerSecret: '',
                            keyMappingId: mapping.MAPPING_ID,
                            keyType: mapping.KEY_TYPE || constants.KEY_TYPE.PRODUCTION,
                            supportedGrantTypes: storedProps.grant_types || km.SUPPORTED_GRANT_TYPES || ['client_credentials'],
                            additionalProperties: storedProps,
                            callbackUrl: storedProps.redirect_uris?.[0] || '',
                        });
                    } catch (mappingErr) {
                        logger.warn('Skipping key mapping due to error', {
                            mappingId: mapping.MAPPING_ID, error: mappingErr.message
                        });
                    }
                }
            }
            if (keyList.length) {
                applicationKeyList = { list: keyList };
            }
        } catch (keyError) {
            logger.warn('Failed to build application keys from local DB', {
                error: keyError.message, stack: keyError.stack
            });
        }
    }

    let kMmetaData = [];
    try {
        const dbKeyManagers = await kmDao.getEnabledKeyManagers(orgID);
        for (const km of dbKeyManagers) {
            const grantTypes = km.SUPPORTED_GRANT_TYPES || ['client_credentials'];
            kMmetaData.push({
                id: km.KM_ID,
                name: km.NAME,
                type: km.TYPE,
                enabled: true,
                tokenEndpoint: km.TOKEN_ENDPOINT,
                authorizeEndpoint: km.ADDITIONAL_PROPERTIES?.authorizeEndpoint || '',
                revokeEndpoint: km.ADDITIONAL_PROPERTIES?.revokeEndpoint || '',
                availableGrantTypes: await mapGrants(grantTypes),
                applicationConfiguration: await mapDefaultValues(
                    km.ADDITIONAL_PROPERTIES?.applicationConfiguration || []
                ),
            });
        }
    } catch (kmError) {
        logger.warn('Failed to fetch key managers from DB', { error: kmError.message });
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

    const profile = buildProfile(req);

    return {
        orgID,
        applicationList,
        keyManagersMetadata: kMmetaData,
        productionKeys,
        sandboxKeys,
        subscriptionScopes,
        profile
    };
};

// ***** Load Applications *****

const loadApplications = async (req, res) => {

    const viewName = req.params.viewName;

    if (config.designMode?.enabled) {
        const templateContent = {
            applicationsMetadata: sampleApiLoader.loadApplications(),
            baseUrl: config.baseUrl + constants.ROUTE.VIEWS_PATH + viewName,
            devMode: true,
        };
        const html = renderTemplate('../pages/applications/page.hbs', config.designMode.pathToLayout + 'layout/main.hbs', templateContent, true);
        return res.send(html);
    }

    const orgName = req.params.orgName;
    const orgDetails = await adminDao.getOrganization(orgName);
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    let html, metaData, templateContent;
    try {
        const orgName = req.params.orgName;
        const orgID = await orgIDValue(orgName);
        const applications = await adminDao.getApplications(orgID, req.user.sub)
        const metaData = applications.map(application => new ApplicationDTO(application));
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

// ***** Load Application *****

const loadApplication = async (req, res) => {
    let html, templateContent, metaData, kMmetaData;
    const viewName = req.params.viewName;
    const orgName = req.params.orgName;
    const orgDetails = await adminDao.getOrganization(orgName);
    const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    try {
        const applicationId = req.params.applicationId;
        const data = await loadApplicationData(req, orgName, applicationId, viewName);
        metaData = data.applicationList;
        kMmetaData = data.keyManagersMetadata;

        templateContent = {
            orgID: data.orgID,
            applicationMetadata: metaData,
            keyManagersMetadata: kMmetaData,
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
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
            subscriptionScopes: data.subscriptionScopes,
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
        const data = await loadApplicationData(req, orgName, applicationId, viewName);
        metaData = data.applicationList;
        kMmetaData = data.keyManagersMetadata;

        templateContent = {
            orgID: data.orgID,
            applicationMetadata: metaData,
            keyManagersMetadata: kMmetaData,
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
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
            subscriptionScopes: data.subscriptionScopes,
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
