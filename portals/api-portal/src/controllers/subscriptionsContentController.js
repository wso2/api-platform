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
 
const { renderTemplateWithView, resolveActor, pageErrorStatus, artifactPageSection } = require('../utils/util');
const logger = require('../config/logger');
const constants = require('../utils/constants');
const orgDao = require('../dao/organizationDao');
const subDao = require('../dao/subscriptionDao');
const planPricingDao = require('../dao/planPricingDao');
const meterDao = require('../dao/meterDao');
const billingSubscriptionDao = require('../dao/billingSubscriptionDao');
const PlanPricing = require('../dto/planPricingDto');
const { accessStatus } = require('../services/billing/subscriptionBillingService');
const { pricingSummary } = require('../utils/pricingLabels');

// How each access status of a paid subscription reads in its status pill.
const ACCESS_LABELS = { ACTIVE: 'ACTIVE', PAST_DUE: 'PAST DUE', PAYMENT_PENDING: 'PAYMENT PENDING' };


const loadSubscriptions = async (req, res, next) => {

    let html;
    const { orgName, viewName } = req.params;

    try {
        const orgDetails = await orgDao.get(orgName);
        const orgId = orgDetails.uuid;

        if (!req.user) {
            return res.redirect(`${constants.ROUTE.BASE_PATH}/${orgName}${constants.ROUTE.VIEWS_PATH}${viewName}/login`);
        }
        let allSubscriptions = [];
        try {
            const createdBy = req.user && resolveActor(req);
            const localSubs = await subDao.list(orgId, { createdBy });
            const billingRows = await billingSubscriptionDao.listBySubscriptions(localSubs.map((s) => s.uuid));
            const planIds = [...new Set(localSubs.map((s) => s.plan_uuid).filter(Boolean))];
            const pricingByPlan = await planPricingDao.listByPlans(orgId, planIds);
            const metersByHandle = pricingByPlan.size > 0
                ? new Map((await meterDao.list(orgId)).map((m) => [m.handle, m])) : new Map();
            allSubscriptions = localSubs.map(sub => {
                const billing = billingRows.get(sub.uuid);
                const pricing = pricingByPlan.get(sub.plan_uuid);
                const access = billing ? accessStatus(billing.status) : null;
                return {
                    id: sub.uuid,
                    type: 'TOKEN_BASED',
                    apiName: sub.api_metadata?.name || '',
                    apiVersion: sub.api_metadata?.version || '',
                    apiHandle: sub.api_metadata?.handle || '#',
                    pageSection: artifactPageSection(sub.api_metadata?.type),
                    planName: sub.subscription_plan?.display_name || '',
                    status: sub.status,
                    subscriptionToken: sub.token,
                    createdAt: sub.created_at || null,
                    paid: Boolean(billing),
                    billingText: pricing ? pricingSummary(new PlanPricing(pricing), metersByHandle) : 'Free',
                    accessStatus: access,
                    statusLabel: access ? ACCESS_LABELS[access] : sub.status,
                    statusTone: access ? access.toLowerCase().replace('_', '-') : (sub.status === 'ACTIVE' ? 'active' : 'inactive'),
                };
            });
        } catch (err) {
            logger.warn('Failed to load subscriptions', {
                error: err.message, orgId
            });
        }

        const profile = {
            firstName: req.user.firstName,
            lastName: req.user.lastName,
            email: req.user.email,
            imageURL: req.user.picture || req.user.imageURL || constants.DEFAULT_PROFILE_IMAGE_URL,
            isAdmin: req.user.isAdmin && !req.foreignOrgSession,
        };

        const templateContent = {
            baseUrl: constants.ROUTE.BASE_PATH + '/' + orgName + constants.ROUTE.VIEWS_PATH + viewName,
            profile: profile,
            orgId: orgId,
            subscriptions: allSubscriptions,
        };

        html = await renderTemplateWithView('../pages/subscriptions/page.hbs', './src/defaultContent/layout/main.hbs', templateContent, true, orgId, viewName);
        res.send(html);
    } catch (error) {
        logger.error('Error loading subscriptions page', {
            error: error.message,
            stack: error.stack,
            orgName
        });
        error.status = pageErrorStatus(error);
        return next(error);
    }
};

module.exports = { loadSubscriptions };
