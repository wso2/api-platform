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
 
const { renderTemplateWithView, resolveActor } = require('../utils/util');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');
const apiKeyService = require('../services/apiKeyService');

/**
 * Renders the org-wide "API Keys" page — every API key the signed-in user has
 * created, across all APIs and MCP servers. The list is loaded server-side (like the
 * global Subscriptions page) so all keys render without client-side pagination. Per-key
 * management (regenerate / revoke / associate) is handled inline by
 * /technical-scripts/api-keys-overview.js, which calls the existing per-API/-MCP REST
 * endpoints and reloads on success.
 */
const loadApiKeysOverview = async (req, res, next) => {
    let html;
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (!req.user) {
            return res.redirect(`/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;

        let apiKeys = [];
        let apiKeysLoadError = false;
        try {
            const keys = await apiKeyService.list(orgId, { createdBy: resolveActor(req) });
            apiKeys = (keys || []).map((k) => ({
                id: k.handle,
                displayName: k.display_name,
                status: String(k.status || 'ACTIVE'),
                expiresAt: k.expires_at,
                apiName: k.dp_api_metadata?.name || '',
                apiVersion: k.dp_api_metadata?.version || '',
                // apiHandle is the path segment the per-API / per-MCP endpoints expect.
                apiHandle: k.dp_api_metadata?.handle || k.api_uuid,
                apiType: k.dp_api_metadata?.type || '',
                appId: k.dp_api_key_app_mapping?.dp_application?.handle || '',
                appDisplayName: k.dp_api_key_app_mapping?.dp_application?.display_name || '',
            }));
        } catch (dbError) {
            apiKeysLoadError = true;
            logger.warn('Failed to load API keys overview', { error: dbError.message, orgId });
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
            apiKeysCount: apiKeys.length,
            apiKeysLoadError,
            isReadOnlyMode: config.server.readOnlyMode,
        };

        html = await renderTemplateWithView('../pages/api-keys-overview/page.hbs', './src/defaultContent/layout/main.hbs', templateContent, true, orgId, viewName);
        res.send(html);
    } catch (error) {
        logger.error('Error loading API keys overview page', {
            error: error.message,
            stack: error.stack,
            orgName,
        });
        error.status = 500;
        return next(error);
    }
};

module.exports = { loadApiKeysOverview };
