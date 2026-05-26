/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const { renderTemplateFromAPI, renderTemplate } = require('../utils/util');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const adminDao = require('../dao/admin');
const apiDao = require('../dao/apiMetadata');
const platformSubDao = require('../dao/platformSubscription');
const APIDTO = require('../dto/apiDTO');
const apiMetadataService = require('../services/apiMetadataService');
const { shouldShowPlatformApiKeysNav } = require('../services/platformApiKeysNavService');


const loadSubscriptions = async (req, res) => {

    let html;
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await adminDao.getOrganization(orgName);
        const orgID = orgDetails.ORG_ID;
        const cpOrgID = orgDetails.ORGANIZATION_IDENTIFIER;
        req.cpOrgID = cpOrgID;

        if (!req.user) {
            return res.redirect(`/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        const userID = req.user.sub;
        const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;

        // 1. Load TOKEN-BASED subscriptions from local DB (all user subscriptions, no apiId filter)
        let tokenBasedSubscriptions = [];
        try {
            const localSubs = await platformSubDao.listPlatformSubscriptions(orgID);
            tokenBasedSubscriptions = localSubs.map(sub => ({
                id: sub.SUB_ID,
                type: 'TOKEN_BASED',
                apiName: sub.DP_API_METADATA?.API_NAME || '',
                apiVersion: sub.DP_API_METADATA?.API_VERSION || '',
                apiHandle: sub.DP_API_METADATA?.API_HANDLE || '#',
                planName: sub.DP_SUBSCRIPTION_POLICY?.POLICY_NAME || '',
                applicationName: null,
                applicationId: null,
                status: sub.STATUS,
                subscriptionToken: sub.SUB_TOKEN,
                createdAt: null,
            }));
        } catch (err) {
            logger.warn('Failed to load platform subscriptions', {
                error: err.message, orgID
            });
        }

        // 2. Load APP-BASED subscriptions
        const applications = await adminDao.getApplications(orgID, userID);
        const appBasedSubscriptions = [];
        for (const app of applications) {
            const subscribedAPIs = await adminDao.getSubscribedAPIs(orgID, app.APP_ID);
            for (const sub of subscribedAPIs) {
                const api = new APIDTO(sub);
                const subMapping = sub.dataValues.DP_APPLICATIONs[0].dataValues.DP_API_SUBSCRIPTION.dataValues;

                const paymentStatus = subMapping.PAYMENT_STATUS;
                if (paymentStatus && paymentStatus !== 'ACTIVE') {
                    continue;
                }

                const subPolicy = await apiDao.getSubscriptionPolicy(subMapping.POLICY_ID, orgID);
                appBasedSubscriptions.push({
                    id: subMapping.SUB_ID,
                    type: 'APP_BASED',
                    apiName: api.apiInfo.apiName,
                    apiVersion: api.apiInfo.apiVersion,
                    apiHandle: api.apiHandle,
                    apiRefId: api.apiReferenceID,
                    planName: subPolicy ? subPolicy.dataValues.POLICY_NAME : '',
                    applicationName: app.NAME,
                    appId: app.APP_ID,
                    // App-based subscriptions have no STATUS column in the local DB schema
                    // (DP_API_SUBSCRIPTION). Unlike token-based subscriptions whose status
                    // is managed by the control plane (ACTIVE/INACTIVE), app-based
                    // subscriptions are always considered active while they exist and are
                    // removed entirely when unsubscribed. This is an intentional limitation.
                    status: 'ACTIVE',
                    subscriptionToken: null,
                    createdAt: subMapping.createdAt || null,
                });
            }
        }

        // 3. Merge and sort (newest first)
        const allSubscriptions = [...tokenBasedSubscriptions, ...appBasedSubscriptions]
            .sort((a, b) => {
                const dateA = a.createdAt ? new Date(a.createdAt) : new Date(0);
                const dateB = b.createdAt ? new Date(b.createdAt) : new Date(0);
                return dateB - dateA;
            });

        const profile = {
            firstName: req.user.firstName,
            lastName: req.user.lastName,
            email: req.user.email,
            imageURL: req.user.picture || req.user.imageURL || '/images/default-profile.png',
            isAdmin: req.user.isAdmin,
        };

        const templateContent = {
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            profile: profile,
            devportalMode: devportalMode,
            orgID: orgID,
            subscriptions: allSubscriptions,
            isReadOnlyMode: config.readOnlyMode,
        };

        html = await renderTemplateFromAPI(templateContent, orgID, orgName, 'pages/subscriptions', viewName);
        res.send(html);
    } catch (error) {
        logger.error('Error loading subscriptions page', {
            error: error.message,
            stack: error.stack,
            orgName
        });
        const devportalMode = constants.DEVPORTAL_MODE.DEFAULT;
        const templateContent = {
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            devportalMode: devportalMode,
            errorMessage: constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE,
        };
        html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        res.status(500).send(html);
    }
};

const loadAPISubscriptions = async (req, res) => {

    let html;
    const { orgName, viewName, apiHandle } = req.params;

    try {
        const orgDetails = await adminDao.getOrganization(orgName);
        const orgID = orgDetails.ORG_ID;
        const cpOrgID = orgDetails.ORGANIZATION_IDENTIFIER;
        req.cpOrgID = cpOrgID;

        if (!req.user) {
            return res.redirect(`/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        const userID = req.user.sub;
        const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;

        const apiID = await apiDao.getAPIId(orgID, apiHandle);
        if (!apiID) {
            const templateContent = {
                baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
                devportalMode: devportalMode,
                errorMessage: constants.ERROR_MESSAGE.API_NOT_FOUND,
                profile: req.isAuthenticated() ? req.user : null,
            };
            html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
            return res.status(404).send(html);
        }
        let metaData = await apiMetadataService.getMetadataFromDB(orgID, apiID, viewName);
        if (metaData && typeof metaData === 'object') {
            metaData = JSON.parse(JSON.stringify(metaData));
            const images = metaData.apiInfo?.apiImageMetadata;
            if (images) {
                for (const key in images) {
                    images[key] = `${constants.ROUTE.DEVPORTAL_ASSETS_BASE_PATH}${orgID}${constants.ROUTE.API_FILE_PATH}${apiID}${constants.API_TEMPLATE_FILE_NAME}${images[key]}`;
                }
            }
        } else {
            metaData = null;
        }
        // 1. Load TOKEN-BASED subscriptions filtered by this API
        let tokenBasedSubscriptions = [];
        try {
            const localSubs = await platformSubDao.listPlatformSubscriptions(orgID, { apiId: apiID });
            tokenBasedSubscriptions = localSubs.map(sub => ({
                id: sub.SUB_ID,
                type: 'TOKEN_BASED',
                apiName: sub.DP_API_METADATA?.API_NAME || metaData?.apiInfo?.apiName || '',
                apiVersion: sub.DP_API_METADATA?.API_VERSION || metaData?.apiInfo?.apiVersion || '',
                apiHandle: sub.DP_API_METADATA?.API_HANDLE || apiHandle,
                apiRefId: sub.API_ID,
                planName: sub.DP_SUBSCRIPTION_POLICY?.POLICY_NAME || '',
                applicationName: null,
                applicationId: null,
                status: sub.STATUS,
                subscriptionToken: sub.SUB_TOKEN,
                createdAt: null,
            }));
        } catch (err) {
            logger.warn('Failed to load platform subscriptions for API', {
                error: err.message, orgID, apiHandle
            });
        }

        // 2. Load APP-BASED subscriptions filtered by this API
        const applications = await adminDao.getApplications(orgID, userID);
        const appBasedSubscriptions = [];
        for (const app of applications) {
            const subscribedAPIs = await adminDao.getSubscribedAPIs(orgID, app.APP_ID);
            for (const sub of subscribedAPIs) {
                const api = new APIDTO(sub);
                if (api.apiID !== apiID) {
                    continue;
                }
                const subMapping = sub.dataValues.DP_APPLICATIONs[0].dataValues.DP_API_SUBSCRIPTION.dataValues;

                // Hide paid subscriptions that haven't completed payment yet.
                // Free plans have null PAYMENT_STATUS and are always shown.
                const paymentStatus = subMapping.PAYMENT_STATUS;
                if (paymentStatus && paymentStatus !== 'ACTIVE') {
                    continue;
                }

                const subPolicy = await apiDao.getSubscriptionPolicy(subMapping.POLICY_ID, orgID);
                appBasedSubscriptions.push({
                    id: subMapping.SUB_ID,
                    type: 'APP_BASED',
                    apiName: api.apiInfo.apiName,
                    apiVersion: api.apiInfo.apiVersion,
                    apiHandle: api.apiHandle,
                    apiRefId: api.apiReferenceID,
                    planName: subPolicy ? subPolicy.dataValues.POLICY_NAME : '',
                    applicationName: app.NAME,
                    appId: app.APP_ID,
                    status: 'ACTIVE',
                    subscriptionToken: null,
                    createdAt: subMapping.createdAt || null,
                });
            }
        }

        // 3. Merge and sort (newest first)
        const allSubscriptions = [...tokenBasedSubscriptions, ...appBasedSubscriptions]
            .sort((a, b) => {
                const dateA = a.createdAt ? new Date(a.createdAt) : new Date(0);
                const dateB = b.createdAt ? new Date(b.createdAt) : new Date(0);
                return dateB - dateA;
            });

        const profile = {
            firstName: req.user.firstName,
            lastName: req.user.lastName,
            email: req.user.email,
            imageURL: req.user.picture || req.user.imageURL || '/images/default-profile.png',
            isAdmin: req.user.isAdmin,
        };

        let apiDefinitionForNav = null;
        if (metaData?.apiInfo?.apiType !== constants.API_TYPE.GRAPHQL && metaData?.apiInfo?.apiType !== constants.API_TYPE.MCP) {
            try {
                const apiFile = await apiDao.getAPIDoc(constants.DOC_TYPES.API_DEFINITION, orgID, apiID);
                apiDefinitionForNav = apiFile?.API_FILE?.toString(constants.CHARSET_UTF8) || null;
            } catch (definitionErr) {
                logger.debug('Could not load API definition for platform API keys nav check', {
                    orgID,
                    apiID,
                    error: definitionErr.message
                });
            }
        }

        const templateContent = {
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            profile: profile,
            devportalMode: devportalMode,
            orgID: orgID,
            subscriptions: allSubscriptions,
            apiMetadata: metaData,
            apiHandle: apiHandle,
            isReadOnlyMode: config.readOnlyMode,
            showPlatformApiKeysNav: await shouldShowPlatformApiKeysNav(req, metaData, null, apiDefinitionForNav),
        };

        html = await renderTemplateFromAPI(templateContent, orgID, orgName, 'pages/api-subscriptions', viewName);
        res.send(html);
    } catch (error) {
        logger.error('Error loading API subscriptions page', {
            error: error.message,
            stack: error.stack,
            orgName,
            apiHandle
        });
        const devportalMode = constants.DEVPORTAL_MODE.DEFAULT;
        const templateContent = {
            baseUrl: '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            devportalMode: devportalMode,
            errorMessage: constants.ERROR_MESSAGE.COMMON_ERROR_MESSAGE,
        };
        html = renderTemplate('../pages/error-page/page.hbs', "./src/defaultContent/" + 'layout/main.hbs', templateContent, true);
        res.status(500).send(html);
    }
};

module.exports = { loadSubscriptions, loadAPISubscriptions };
