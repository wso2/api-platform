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
const orgDao = require('../dao/organizationDao');
const appDao = require('../dao/applicationDao');
const { ApplicationDTO } = require('../dto/applicationDto');
const sampleApiLoader = require('../utils/sampleApiLoader');
const kmDao = require('../dao/keyManagerDao');
const adminService = require('../services/adminService');
const apiKeyService = require('../services/apiKeyService');

const orgIDValue = async (orgName) => {
    const organization = await orgDao.get(orgName);
    return organization.uuid;
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
    const orgId = await orgIDValue(orgName);

    const userId = req[constants.USER_ID]
    const applicationList = await adminService.getApplicationKeyMap(orgId, applicationId, userId);

    let applicationReference = "";
    let applicationKeyList;
    if (Array.isArray(applicationList.appKeyMappings) && applicationList.appKeyMappings.length > 0) {
        applicationReference = applicationList.appKeyMappings[0].asClientId;
        try {
            const { ApplicationKeyMapping } = require('../models/application');
            const localMappings = await ApplicationKeyMapping.findAll({
                where: { app_uuid: applicationId }
            });
            const keyList = [];
            for (const mapping of localMappings) {
                if (mapping.as_client_id && mapping.km_uuid) {
                    try {
                        const km = await kmDao.get(mapping.km_uuid);
                        keyList.push({
                            keyManager: km.name,
                            consumerKey: mapping.as_client_id,
                            keyMappingId: mapping.uuid,
                            keyType: mapping.type || constants.KEY_TYPE.PRODUCTION,
                        });
                    } catch (mappingErr) {
                        logger.warn('Skipping key mapping due to error', {
                            mappingId: mapping.uuid, error: mappingErr.message
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
        const dbKeyManagers = await kmDao.listEnabled(orgId);
        for (const km of dbKeyManagers) {
            kMmetaData.push({
                id: km.uuid,
                name: km.name,
                type: km.type,
                enabled: true,
                tokenEndpoint: km.token_endpoint,
            });
        }
    } catch (kmError) {
        logger.warn('Failed to fetch key managers from DB', { error: kmError.message });
    }

    let productionKeys = [];
    let sandboxKeys = [];

    applicationKeyList?.list?.map(key => {
        let keyData = {
            keyManager: key.keyManager,
            consumerKey: key.consumerKey,
            keyMappingId: key.keyMappingId,
            keyType: key.keyType,
            appRefId: applicationReference
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
        orgId,
        applicationList,
        keyManagersMetadata: kMmetaData,
        productionKeys,
        sandboxKeys,
        subscriptionScopes,
        profile
    };
};

// ***** Load Applications *****

const loadApplications = async (req, res, next) => {

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
    const orgDetails = await orgDao.get(orgName);
    const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    let html, metaData, templateContent;
    try {
        const orgName = req.params.orgName;
        const orgId = await orgIDValue(orgName);
        const applications = await appDao.list(orgId, req.user.sub)
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
            orgId,
            applicationsMetadata: metaData,
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            profile: req.isAuthenticated() ? profile : null,
            devportalMode: devportalMode,
            isReadOnlyMode: config.readOnlyMode,
        }
        const templateResponse = await templateResponseValue('applications');
        const layoutResponse = await loadLayoutFromAPI(orgId, viewName);
        if (layoutResponse === "") {
            html = renderTemplate('../pages/applications/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        } else {
            html = await renderGivenTemplate(templateResponse, layoutResponse, templateContent);
        }
    } catch (error) {
        logger.error("Error occurred while loading Applications", {
            orgName: orgName,
            error: error.message,
            stack: error.stack
        });
        error.status = 500;
        return next(error);
    }
    res.send(html);
}

// ***** Load Application *****

const loadApplication = async (req, res, next) => {
    let html, templateContent, metaData, kMmetaData;
    const viewName = req.params.viewName;
    const orgName = req.params.orgName;
    const orgDetails = await orgDao.get(orgName);
    const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    try {
        const applicationId = req.params.applicationId;
        const data = await loadApplicationData(req, orgName, applicationId, viewName);
        metaData = data.applicationList;
        kMmetaData = data.keyManagersMetadata;
        const { associatedApiKeys, availableKeysByApi } = await loadApplicationApiKeysData(data.orgId, applicationId);

        templateContent = {
            orgId: data.orgId,
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
            isReadOnlyMode: config.readOnlyMode,
            associatedApiKeys,
            availableKeysByApi
        }
        const templateResponse = await templateResponseValue('application');
        const layoutResponse = await loadLayoutFromAPI(data.orgId, viewName);
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
        if (Number(error?.statusCode) === 401) {
            const err = Object.assign(new Error(constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE), { status: 401 });
            return next(err);
        } else {
            error.status = 500;
            return next(error);
        }
    }
    res.send(html);
}

const loadApplicationKeys = async (req, res, next) => {
    let html, templateContent, metaData, kMmetaData;
    const viewName = req.params.viewName;
    const orgName = req.params.orgName;
    const orgDetails = await orgDao.get(orgName);
    const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;
    try {
        const applicationId = req.params.applicationId;
        const data = await loadApplicationData(req, orgName, applicationId, viewName);
        metaData = data.applicationList;
        kMmetaData = data.keyManagersMetadata;

        templateContent = {
            orgId: data.orgId,
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
        const layoutResponse = await loadLayoutFromAPI(data.orgId, viewName);
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
        if (Number(error?.statusCode) === 401) {
            const err = Object.assign(new Error(constants.ERROR_MESSAGE.COMMON_AUTH_ERROR_MESSAGE), { status: 401 });
            return next(err);
        } else {
            error.status = 500;
            return next(error);
        }
    }
    res.send(html);
}

/**
 * Loads the keys currently associated with this app, plus a by-API grouping of keys
 * that could be associated instead (anything not already associated with this app).
 */
function formatApiDisplayName(apiMetadata, fallbackId) {
    if (!apiMetadata) return fallbackId;
    const namePart = [apiMetadata.name, apiMetadata.version].filter(Boolean).join(' ');
    return apiMetadata.handle ? `${namePart} (${apiMetadata.handle})` : namePart;
}

async function loadApplicationApiKeysData(orgId, applicationId) {
    let associatedApiKeys = [];
    let availableKeysByApi = [];
    try {
        const associated = await apiKeyService.list(orgId, { appId: applicationId });
        associatedApiKeys = associated.map((k) => ({
            keyId: k.uuid,
            name: k.name,
            status: String(k.status || 'ACTIVE').toLowerCase(),
            apiId: k.api_uuid,
            apiName: formatApiDisplayName(k.dp_api_metadata, k.api_uuid)
        }));

        // Capped — this just populates a UI picker, not a full export of the org's keys.
        const allKeys = await apiKeyService.list(orgId, { status: 'ACTIVE', limit: 200 });
        const byApi = new Map();
        allKeys.forEach((k) => {
            if (k.dp_api_key_app_mapping?.app_uuid === applicationId) return;
            const apiId = k.api_uuid;
            const apiName = formatApiDisplayName(k.dp_api_metadata, apiId);
            if (!byApi.has(apiId)) byApi.set(apiId, { apiId, apiName, keys: [] });
            byApi.get(apiId).keys.push({ keyId: k.uuid, name: k.name });
        });
        availableKeysByApi = Array.from(byApi.values());
    } catch (error) {
        logger.warn('Failed to load API keys for application API keys section', {
            orgId, applicationId, error: error.message
        });
    }
    return { associatedApiKeys, availableKeysByApi };
}

module.exports = {
    loadApplications,
    loadApplication,
    loadApplicationKeys
};
