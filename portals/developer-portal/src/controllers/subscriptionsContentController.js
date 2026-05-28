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
const subDao = require('../dao/subscription');
const apiMetadataService = require('../services/apiMetadataService');
const { shouldShowApiKeysNav } = require('../services/apiKeysNavService');


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
        const devportalMode = orgDetails.ORG_CONFIG?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;

        let allSubscriptions = [];
        try {
            const localSubs = await subDao.listSubscriptions(orgID);
            allSubscriptions = localSubs.map(sub => ({
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
            logger.warn('Failed to load subscriptions', {
                error: err.message, orgID
            });
        }

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
        let allSubscriptions = [];
        try {
            const localSubs = await subDao.listSubscriptions(orgID, { apiId: apiID });
            allSubscriptions = localSubs.map(sub => ({
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
            logger.warn('Failed to load subscriptions for API', {
                error: err.message, orgID, apiHandle
            });
        }

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
                logger.debug('Could not load API definition for API keys nav check', {
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
            showApiKeysNav: await shouldShowApiKeysNav(req, metaData, null, apiDefinitionForNav),
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
