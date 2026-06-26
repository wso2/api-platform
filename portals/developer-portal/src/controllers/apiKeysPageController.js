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
 * "AS IS" BASIS, WITHOUT WARRANTIES OR ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
/* eslint-disable no-undef */
const { renderTemplateFromAPI } = require('../utils/util');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');
const apiDao = require('../dao/apiDao');
const apiFileDao = require('../dao/apiFileDao');
const applicationDao = require('../dao/applicationDao');
const apiMetadataService = require('../services/apiMetadataService');
const apiKeyService = require('../services/apiKeyService');
const { apiUsesApiKeySecurity } = require('../utils/apiDefinitionUtil');
const { getSessionCsrfToken } = require('../middlewares/csrfProtection');

const loadAPIApiKeys = async (req, res, next) => {
    let html;
    const { orgName, viewName, apiHandle } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgID = orgDetails.ID;

        if (!req.user) {
            return res.redirect(`/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        const devportalMode = orgDetails.CONFIGURATION?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;

        const apiID = await apiDao.getId(orgID, apiHandle);
        if (!apiID) {
            const err = new Error('API not found');
            err.status = 404;
            return next(err);
        }
        let metaData = await apiMetadataService.getMetadataFromDB(orgID, apiID, viewName);
        if (metaData && typeof metaData === 'object') {
            metaData = JSON.parse(JSON.stringify(metaData));
            const images = metaData.apiInfo?.apiImageMetadata;
            if (images) {
                for (const key in images) {
                    images[key] = `${constants.DEVPORTAL_API.orgPath(orgID)}${constants.ROUTE.API_FILE_PATH}${apiID}${constants.API_TEMPLATE_FILE_NAME}${images[key]}`;
                }
            }
        } else {
            metaData = null;
        }

        let apiDefinitionForNav = null;
        if (metaData?.apiInfo?.apiType !== constants.API_TYPE.GRAPHQL && metaData?.apiInfo?.apiType !== constants.API_TYPE.MCP) {
            try {
                const apiFile = await apiFileDao.getDoc(constants.DOC_TYPES.API_DEFINITION, orgID, apiID);
                apiDefinitionForNav = apiFile?.FILE_CONTENT?.toString(constants.CHARSET_UTF8) || null;
            } catch (definitionErr) {
                logger.debug('Could not load API definition for API keys nav check', {
                    orgID,
                    apiID,
                    error: definitionErr.message
                });
            }
        }

        const showApiKeysNav = apiUsesApiKeySecurity(metaData, apiDefinitionForNav);
        if (!showApiKeysNav) {
            const err = new Error('API Keys not available for this API');
            err.status = 404;
            return next(err);
        }

        let apiKeys = [];
        let apiKeysCount = 0;
        let apiKeysLoadError = false;
        let applications = [];
        const selectedAppId = typeof req.query.appId === 'string' ? req.query.appId.trim() : '';

        try {
            const keys = await apiKeyService.list(orgID, { apiId: apiID, appId: selectedAppId || undefined });
            apiKeys = (keys || []).map((k) => ({
                keyId: k.ID,
                name: k.NAME,
                status: String(k.STATUS || 'ACTIVE').toLowerCase(),
                expiresAt: k.EXPIRES_AT,
                createdAt: k.CREATED_AT,
                revokedAt: k.REVOKED_AT || undefined,
                apiId: k.API_ID,
                appId: k.APP_ID || null,
                appName: k.APP_NAME || null,
                maskedApiKey: '••••••••'
            }));
            apiKeysCount = apiKeys.length;
        } catch (dbError) {
            apiKeysLoadError = true;
            logger.warn('Failed to load API keys', {
                error: dbError.message,
                orgID,
                apiHandle
            });
        }

        try {
            const apps = await applicationDao.list(orgID, req.user.sub);
            applications = (apps || []).map((a) => ({ appId: a.ID, name: a.NAME }));
        } catch (dbError) {
            logger.warn('Failed to load applications for API key association', {
                error: dbError.message,
                orgID
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
            apiKeys: apiKeys,
            apiKeysCount: apiKeysCount,
            apiKeysLoadError,
            applications,
            selectedAppId,
            apiMetadata: metaData,
            apiHandle: apiHandle,
            isReadOnlyMode: config.readOnlyMode,
            showApiKeysNav,
            csrfToken: getSessionCsrfToken(req),
        };

        html = await renderTemplateFromAPI(templateContent, orgID, orgName, 'pages/api-keys', viewName);
        res.send(html);
    } catch (error) {
        logger.error('Error loading API keys page', {
            error: error.message,
            stack: error.stack,
            orgName,
            apiHandle
        });
        next(error);
    }
};

module.exports = { loadAPIApiKeys };
