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
const { renderTemplateFromAPI, resolveActor } = require('../utils/util');
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
        const orgId = orgDetails.uuid;

        if (!req.user) {
            return res.redirect(`/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;

        const apiId = await apiDao.getId(orgId, apiHandle);
        if (!apiId) {
            const err = new Error('API not found');
            err.status = 404;
            return next(err);
        }
        let metaData = await apiMetadataService.getMetadataFromDB(orgId, apiId, viewName);
        if (metaData && typeof metaData === 'object') {
            metaData = JSON.parse(JSON.stringify(metaData));
            const images = metaData.apiImageMetadata;
            if (images) {
                for (const key in images) {
                    images[key] = `${constants.DEVPORTAL_API.orgPath(orgId)}${constants.ROUTE.API_FILE_PATH}${apiId}${constants.API_TEMPLATE_FILE_NAME}${images[key]}`;
                }
            }
        } else {
            metaData = null;
        }

        let apiDefinitionForNav = null;
        if (metaData?.type !== constants.API_TYPE.GRAPHQL && metaData?.type !== constants.API_TYPE.MCP) {
            try {
                const apiFile = await apiFileDao.getDoc(constants.DOC_TYPES.API_DEFINITION, orgId, apiId);
                apiDefinitionForNav = apiFile?.file_content?.toString(constants.CHARSET_UTF8) || null;
            } catch (definitionErr) {
                logger.debug('Could not load API definition for API keys nav check', {
                    orgId,
                    apiId,
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
        const selectedAppHandle = typeof req.query.appId === 'string' ? req.query.appId.trim() : '';

        try {
            const selectedApp = selectedAppHandle
                ? await applicationDao.getId(orgId, resolveActor(req), selectedAppHandle)
                : undefined;
            const selectedAppId = selectedApp ? selectedApp.uuid : undefined;
            const keys = await apiKeyService.list(orgId, { apiId: apiId, appId: selectedAppId || undefined });
            apiKeys = (keys || []).map((k) => ({
                keyId: k.uuid,
                name: k.name,
                status: String(k.status || 'ACTIVE').toLowerCase(),
                expiresAt: k.expires_at,
                createdAt: k.created_at,
                revokedAt: k.revoked_at || undefined,
                apiId: k.dp_api_metadata?.handle || k.api_uuid,
                appId: k.dp_api_key_app_mapping?.dp_application?.handle || null,
                appDisplayName: k.dp_api_key_app_mapping?.dp_application?.display_name || null,
                maskedApiKey: '••••••••'
            }));
            apiKeysCount = apiKeys.length;
        } catch (dbError) {
            apiKeysLoadError = true;
            logger.warn('Failed to load API keys', {
                error: dbError.message,
                orgId,
                apiHandle
            });
        }

        try {
            const apps = await applicationDao.list(orgId, resolveActor(req));
            applications = (apps || []).map((a) => ({ appId: a.handle, displayName: a.display_name }));
        } catch (dbError) {
            logger.warn('Failed to load applications for API key association', {
                error: dbError.message,
                orgId
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
            orgId: orgId,
            apiKeys: apiKeys,
            apiKeysCount: apiKeysCount,
            apiKeysLoadError,
            applications,
            selectedAppId: selectedAppHandle,
            apiMetadata: metaData,
            apiHandle: apiHandle,
            isReadOnlyMode: config.readOnlyMode,
            showApiKeysNav,
            csrfToken: getSessionCsrfToken(req),
        };

        html = await renderTemplateFromAPI(templateContent, orgId, orgName, 'pages/api-keys', viewName);
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
