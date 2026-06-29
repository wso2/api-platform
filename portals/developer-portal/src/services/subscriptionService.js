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
const util = require('../utils/util');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');

// Logs context before rethrowing so the caller's transaction rolls back instead of
// silently committing the subscription change without its webhook event.
async function safePublish(eventType, payload, opts) {
    try {
        await publishWebhookEvent(eventType, payload, opts);
    } catch (err) {
        logger.error('Failed to publish webhook event', {
            eventType, error: err.message,
        });
        throw err;
    }
}

function buildWebhookPayload(sub, apiMetadata, plan) {
    return {
        subscription_id: sub.UUID,
        subscriber_id: sub.CREATED_BY,
        status: sub.STATUS,
        subscription_plan: {
            ref_id: plan ? (plan.REF_ID || null) : null,
            name: plan ? (plan.NAME || null) : null,
        },
        api: {
            name: apiMetadata ? apiMetadata.NAME : null,
            version: apiMetadata ? apiMetadata.VERSION : null,
            ref_id: apiMetadata ? (apiMetadata.REF_ID || '') : '',
        },
    };
}

function formatSubscriptionResponse(sub) {
    const plan = sub.DP_SUBSCRIPTION_PLAN || {};
    return {
        subscriptionId: sub.UUID,
        subscriptionToken: sub.TOKEN,
        status: sub.STATUS,
        apiId: sub.API_UUID,
        subscriptionPlanName: plan.NAME || null,
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
            matchedPlan = plans.find(p => p.UUID === reqPlanId);
            if (!matchedPlan) {
                return res.status(400).json({
                    code: '400', message: 'Bad Request',
                    description: `Subscription plan not found for this API`,
                });
            }
            planId = matchedPlan.UUID;
        }

        let newSub;
        await sequelize.transaction(async (t) => {
            newSub = await subDao.create(
                orgID, apiId, planId, createdBy, t
            );
            await safePublish('subscription.created', buildWebhookPayload(newSub, apiMetadata, matchedPlan), {
                transaction: t,
                orgId: orgID,
                aggregateType: 'subscription',
                aggregateId: newSub.UUID,
                secretFields: { token: newSub.TOKEN },
            });
        });

        const created = await subDao.get(orgID, newSub.UUID, createdBy);
        logUserAction('SUBSCRIPTION_CREATED', req, { orgId: orgID, apiId, subscriptionId: newSub.UUID });
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
        return res.status(200).json(util.toPaginatedList(subs.map(formatSubscriptionResponse), req));
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
    if (!Object.values(constants.SUBSCRIPTION_STATUS).includes(status)) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: `Invalid status. Must be one of: ${Object.values(constants.SUBSCRIPTION_STATUS).join(', ')}.` });
    }

    try {
        const existing = await subDao.get(orgID, subscriptionId, req.user.sub);
        if (!existing) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }

        let sub;
        await sequelize.transaction(async (t) => {
            const updated = await subDao.updateStatus(orgID, subscriptionId, status, req.user.sub, t);
            if (!updated) {
                const err = new Error('Subscription not found');
                err.status = 404;
                throw err;
            }
            await publishWebhookEvent('subscription.updated',
                buildWebhookPayload({ ...existing, STATUS: status }, existing.DP_API_METADATA, existing.DP_SUBSCRIPTION_PLAN),
                { transaction: t, orgId: orgID, aggregateType: 'subscription', aggregateId: subscriptionId });
        });
        sub = await subDao.get(orgID, subscriptionId, req.user.sub);
        return res.status(200).json(formatSubscriptionResponse(sub));
    } catch (error) {
        if (error.status === 404) {
            return res.status(404).json({ code: '404', message: 'Not Found', description: 'Subscription not found' });
        }
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

        await sequelize.transaction(async (t) => {
            const deleted = await subDao.delete(orgID, subscriptionId, req.user.sub, t);
            if (!deleted) throw Object.assign(new Error('Not found'), { statusCode: 404 });
            await safePublish('subscription.deleted', buildWebhookPayload(existing, apiMetadata, plan), {
                transaction: t,
                orgId: orgID,
                aggregateType: 'subscription',
                aggregateId: subscriptionId,
            });
        });

        logUserAction('SUBSCRIPTION_DELETED', req, { orgId: orgID, subscriptionId });
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
