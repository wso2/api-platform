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
const { renderTemplateFromAPI } = require('../utils/util');
const { config } = require('../config/configLoader');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');
const subDao = require('../dao/subscriptionDao');


const loadSubscriptions = async (req, res, next) => {

    let html;
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (!req.user) {
            return res.redirect(`/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        const devportalMode = orgDetails.configuration?.devportalMode || constants.DEVPORTAL_MODE.DEFAULT;

        let allSubscriptions = [];
        try {
            const createdBy = req.user && req.user.sub;
            const localSubs = await subDao.list(orgId, { createdBy });
            allSubscriptions = localSubs.map(sub => ({
                id: sub.uuid,
                type: 'TOKEN_BASED',
                apiName: sub.dp_api_metadata?.name || '',
                apiVersion: sub.dp_api_metadata?.version || '',
                apiHandle: sub.dp_api_metadata?.handle || '#',
                planName: sub.dp_subscription_plan?.name || '',
                status: sub.status,
                subscriptionToken: sub.token,
                createdAt: sub.created_at || null,
            }));
        } catch (err) {
            logger.warn('Failed to load subscriptions', {
                error: err.message, orgId
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
            subscriptions: allSubscriptions,
            isReadOnlyMode: config.readOnlyMode,
        };

        html = await renderTemplateFromAPI(templateContent, orgId, orgName, 'pages/subscriptions', viewName);
        res.send(html);
    } catch (error) {
        logger.error('Error loading subscriptions page', {
            error: error.message,
            stack: error.stack,
            orgName
        });
        error.status = 500;
        return next(error);
    }
};

module.exports = { loadSubscriptions };
