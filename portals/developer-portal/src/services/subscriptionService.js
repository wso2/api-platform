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
const apiDao = require('../dao/apiDao');
const subDao = require('../dao/subscriptionDao');
const sequelize = require('../db/sequelizeConfig');
const { publish: publishWebhookEvent } = require('./webhooks/eventPublisher');
const platformClient = require('./platformApiClient');
const { isPlatformApiPath } = platformClient;
const util = require('../utils/util');
const logger = require('../config/logger');

async function safePublish(eventType, payload, opts) {
    try {
        await publishWebhookEvent(eventType, payload, opts);
    } catch (err) {
        logger.warn('[subscriptionService] webhook publish failed (non-fatal)', {
            eventType, error: err.message,
        });
    }
}

function buildWebhookPayload(sub, apiMetadata, plan) {
    return {
        subscription_id: sub.SUB_ID,
        subscription_plan: {
            ref_id: plan ? (plan.REF_ID || null) : null,
            name: plan ? (plan.PLAN_NAME || plan.DISPLAY_NAME || null) : null,
        },
        api: {
            name: apiMetadata ? apiMetadata.API_NAME : null,
            version: apiMetadata ? apiMetadata.API_VERSION : null,
            ref_id: apiMetadata ? (apiMetadata.REFERENCE_ID || '') : '',
        },
    };
}

function formatSubscriptionResponse(sub) {
    const api = sub.DP_API_METADATA || {};
    const plan = sub.DP_SUBSCRIPTION_PLAN || {};
    return {
        subscriptionId: sub.SUB_ID,
        subscriptionToken: sub.SUB_TOKEN,
        status: sub.STATUS,
        gatewayType: api.GATEWAY_TYPE || null,
        apiId: sub.API_ID,
        subscriptionPlanName: plan.PLAN_NAME || null,
        createdBy: sub.CREATED_BY || null,
        createdAt: sub.CREATED_AT || null,
    };
}

const createSubscription = async (req, res) => {
    const orgID = req.params.orgId;
    const { apiId, subscriptionPlanId: reqPlanId } = req.body;
    const createdBy = req.user.sub;

    try {
        const apiMetadataResponse = await apiDao.get(orgID, apiId);
        if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'API not found',
            });
        }

        const apiMetadata = apiMetadataResponse[0];

        const plans = apiMetadata.DP_SUBSCRIPTION_PLANs || [];
        if (plans.length === 0) {
            return res.status(400).json({
                code: '400', message: 'Bad Request',
                description: 'This API does not support subscriptions',
            });
        }

        let planId = null;
        let matchedPlan = null;

        if (reqPlanId) {
            matchedPlan = plans.find(p => p.PLAN_ID === reqPlanId);
            if (!matchedPlan) {
                return res.status(400).json({
                    code: '400', message: 'Bad Request',
                    description: `Subscription plan not found for this API`,
                });
            }
            planId = matchedPlan.PLAN_ID;
        }

        let newSub;
        if (isPlatformApiPath(apiMetadata.GATEWAY_TYPE)) {
            const subscriptionPlanId = matchedPlan ? (matchedPlan.REF_ID || matchedPlan.PLAN_NAME) : null;
            const platformResp = await platformClient.createSubscription(req.user.accessToken, {
                apiId: apiMetadata.REFERENCE_ID,
                subscriberId: req.user.sub,
                subscriptionPlanId,
            });
            await sequelize.transaction(async (t) => {
                newSub = await subDao.create(orgID, apiId, planId, createdBy, t, {
                    subToken: platformResp.subscriptionToken,
                });
            });
        } else {
            await sequelize.transaction(async (t) => {
                newSub = await subDao.create(
                    orgID, apiId, planId, createdBy, t
                );
                await safePublish('subscription.created', buildWebhookPayload(newSub, apiMetadata, matchedPlan), {
                    transaction: t,
                    orgId: orgID,
                    gatewayType: apiMetadata.GATEWAY_TYPE,
                    aggregateType: 'subscription',
                    aggregateId: newSub.SUB_ID,
                    plaintextKey: newSub.SUB_TOKEN,
                });
            });
        }

        const created = await subDao.get(orgID, newSub.SUB_ID, createdBy);
        return res.status(201).json(formatSubscriptionResponse(created));
    } catch (error) {
        if (error.name === 'SequelizeUniqueConstraintError') {
            return res.status(409).json({
                code: '409', message: 'Conflict',
                description: 'A subscription for this API already exists',
            });
        }
        logger.error('Error creating subscription', {
            error: error.message, orgID, apiId,
        });
        util.handleError(res, error);
    }
};

const listSubscriptions = async (req, res) => {
    const orgID = req.params.orgId;
    const apiId = req.query.apiId;

    try {
        if (apiId) {
            const apiMetadataResponse = await apiDao.get(orgID, apiId);
            if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
                return res.status(404).json({
                    code: '404', message: 'Not Found', description: 'API not found',
                });
            }
        }

        const subs = await subDao.list(orgID, { apiId, createdBy: req.user.sub });
        return res.status(200).json({ count: subs.length, list: subs.map(formatSubscriptionResponse) });
    } catch (error) {
        logger.error('Error listing subscriptions', {
            error: error.message, orgID,
        });
        util.handleError(res, error);
    }
};

const getSubscription = async (req, res) => {
    const orgID = req.params.orgId;
    const subscriptionId = req.params.subId;

    try {
        const sub = await subDao.get(orgID, subscriptionId, req.user.sub);
        if (!sub) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }
        return res.status(200).json(formatSubscriptionResponse(sub));
    } catch (error) {
        logger.error('Error getting subscription', {
            error: error.message, subscriptionId,
        });
        util.handleError(res, error);
    }
};

const updateSubscription = async (req, res) => {
    const orgID = req.params.orgId;
    const subscriptionId = req.params.subId;
    const { status } = req.body;

    try {
        const existing = await subDao.get(orgID, subscriptionId, req.user.sub);
        if (!existing) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }

        const apiMetadata = existing.DP_API_METADATA;
        if (isPlatformApiPath(apiMetadata?.GATEWAY_TYPE)) {
            const platformSub = await platformClient.findSubscription(req.user.accessToken, {
                apiId: apiMetadata.REFERENCE_ID,
                subscriberId: req.user.sub,
            });
            if (platformSub) {
                await platformClient.updateSubscription(req.user.accessToken, {
                    platformSubId: platformSub.id,
                    subscriberId: req.user.sub,
                    status,
                });
            } else {
                logger.warn('[subscriptionService] platform-api subscription not found, updating local DB only', {
                    subscriptionId, apiRefId: apiMetadata.REFERENCE_ID,
                });
            }
            await subDao.updateStatus(orgID, subscriptionId, status, req.user.sub);
        } else {
            const updated = await subDao.updateStatus(
                orgID, subscriptionId, status, req.user.sub
            );
            if (!updated) {
                return res.status(404).json({
                    code: '404', message: 'Not Found', description: 'Subscription not found',
                });
            }
        }
        const sub = await subDao.get(orgID, subscriptionId, req.user.sub);
        return res.status(200).json(formatSubscriptionResponse(sub));
    } catch (error) {
        logger.error('Error updating subscription', {
            error: error.message, subscriptionId, status,
        });
        util.handleError(res, error);
    }
};

const deleteSubscription = async (req, res) => {
    const orgID = req.params.orgId;
    const subscriptionId = req.params.subId;

    try {
        const existing = await subDao.get(orgID, subscriptionId, req.user.sub);
        if (!existing) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }

        const apiMetadata = existing.DP_API_METADATA;
        const plan = existing.DP_SUBSCRIPTION_PLAN;

        if (isPlatformApiPath(apiMetadata?.GATEWAY_TYPE)) {
            const platformSub = await platformClient.findSubscription(req.user.accessToken, {
                apiId: apiMetadata.REFERENCE_ID,
                subscriberId: req.user.sub,
            });
            if (platformSub) {
                await platformClient.deleteSubscription(req.user.accessToken, {
                    platformSubId: platformSub.id,
                    subscriberId: req.user.sub,
                });
            }
            await sequelize.transaction(async (t) => {
                const deleted = await subDao.delete(orgID, subscriptionId, req.user.sub, t);
                if (!deleted) throw Object.assign(new Error('Not found'), { statusCode: 404 });
            });
        } else {
            await sequelize.transaction(async (t) => {
                const deleted = await subDao.delete(orgID, subscriptionId, req.user.sub, t);
                if (!deleted) throw Object.assign(new Error('Not found'), { statusCode: 404 });
                await safePublish('subscription.deleted', buildWebhookPayload(existing, apiMetadata, plan), {
                    transaction: t,
                    orgId: orgID,
                    gatewayType: apiMetadata ? apiMetadata.GATEWAY_TYPE : null,
                    aggregateType: 'subscription',
                    aggregateId: subscriptionId,
                });
            });
        }

        return res.status(200).json({ message: 'Subscription deleted successfully' });
    } catch (error) {
        if (error.statusCode === 404) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }
        logger.error('Error deleting subscription', {
            error: error.message, subscriptionId,
        });
        util.handleError(res, error);
    }
};

module.exports = {
    createSubscription,
    listSubscriptions,
    getSubscription,
    updateSubscription,
    deleteSubscription,
};
