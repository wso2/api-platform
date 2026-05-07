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
        subscription_id: sub.uuid,
        subscriber_id: sub.created_by,
        status: sub.status,
        subscription_plan: {
            ref_id: plan ? (plan.ref_id || null) : null,
            name: plan ? (plan.name || null) : null,
        },
        api: {
            name: apiMetadata ? apiMetadata.name : null,
            version: apiMetadata ? apiMetadata.version : null,
            ref_id: apiMetadata ? (apiMetadata.ref_id || '') : '',
        },
    };
}

function formatSubscriptionResponse(sub) {
    const plan = sub.dp_subscription_plan || {};
    return {
        subscriptionId: sub.uuid,
        subscriptionToken: sub.token,
        status: sub.status,
        apiId: sub.api_uuid,
        subscriptionPlanName: plan.name || null,
        createdBy: sub.created_by || null,
        createdAt: sub.created_at || null,
    };
}

const createSubscription = async (req, res) => {
    const orgId = req.orgId;
    const { apiId, subscriptionPlanId: reqPlanId } = req.body;
    const createdBy = req.user.sub;

    try {
        const apiMetadataResponse = await apiDao.get(orgId, apiId);
        if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'API not found',
            });
        }

        const apiMetadata = apiMetadataResponse[0];

        const plans = apiMetadata.dp_subscription_plans || [];
        if (plans.length === 0) {
            return res.status(400).json({
                code: '400', message: 'Bad Request',
                description: 'This API does not support subscriptions',
            });
        }

        const matchedPlan = plans.find(p => p.uuid === reqPlanId);
        if (!matchedPlan) {
            return res.status(400).json({
                code: '400', message: 'Bad Request',
                description: 'Subscription plan not found for this API',
            });
        }
        const planId = matchedPlan.uuid;

        let newSub;
        await sequelize.transaction(async (t) => {
            newSub = await subDao.create(
                orgId, apiId, planId, createdBy, t
            );
            await safePublish('subscription.created', buildWebhookPayload(newSub, apiMetadata, matchedPlan), {
                transaction: t,
                orgId: orgId,
                aggregateType: 'subscription',
                aggregateId: newSub.uuid,
                secretFields: { token: newSub.token },
            });
        });

        const created = await subDao.get(orgId, newSub.uuid, createdBy);
        logUserAction('SUBSCRIPTION_CREATED', req, { orgId: orgId, apiId, subscriptionId: newSub.uuid });
        return res.status(201).json(formatSubscriptionResponse(created));
    } catch (error) {
        if (error.name === 'SequelizeUniqueConstraintError') {
            return res.status(409).json({
                code: '409', message: 'Conflict',
                description: 'A subscription for this API already exists',
            });
        }
        logger.error('Error creating subscription', {
            error: error.message, orgId, apiId,
        });
        util.handleError(res, error);
    }
};

const listSubscriptions = async (req, res) => {
    const orgId = req.orgId;
    const apiId = req.query.apiId;

    try {
        if (apiId) {
            const apiMetadataResponse = await apiDao.get(orgId, apiId);
            if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
                return res.status(404).json({
                    code: '404', message: 'Not Found', description: 'API not found',
                });
            }
        }

        const subs = await subDao.list(orgId, { apiId, createdBy: req.user.sub });
        return res.status(200).json(util.toPaginatedList(subs.map(formatSubscriptionResponse), req));
    } catch (error) {
        logger.error('Error listing subscriptions', {
            error: error.message, orgId,
        });
        util.handleError(res, error);
    }
};

const getSubscription = async (req, res) => {
    const orgId = req.orgId;
    const subscriptionId = req.params.subId;

    try {
        const sub = await subDao.get(orgId, subscriptionId, req.user.sub);
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
    const orgId = req.orgId;
    const subscriptionId = req.params.subId;
    const { status } = req.body;
    if (!Object.values(constants.SUBSCRIPTION_STATUS).includes(status)) {
        return res.status(400).json({ code: '400', message: 'Bad Request', description: `Invalid status. Must be one of: ${Object.values(constants.SUBSCRIPTION_STATUS).join(', ')}.` });
    }

    try {
        const existing = await subDao.get(orgId, subscriptionId, req.user.sub);
        if (!existing) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }

        let sub;
        await sequelize.transaction(async (t) => {
            const updated = await subDao.updateStatus(orgId, subscriptionId, status, req.user.sub, t);
            if (!updated) {
                const err = new Error('Subscription not found');
                err.status = 404;
                throw err;
            }
            await publishWebhookEvent('subscription.updated',
                buildWebhookPayload({ ...existing.get({ plain: true }), status: status }, existing.dp_api_metadata, existing.dp_subscription_plan),
                { transaction: t, orgId: orgId, aggregateType: 'subscription', aggregateId: subscriptionId });
        });
        sub = await subDao.get(orgId, subscriptionId, req.user.sub);
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

const changePlan = async (req, res) => {
    const orgId = req.orgId;
    const subscriptionId = req.params.subId;
    const { apiId: reqApiId, planId: reqPlanId } = req.body;

    try {
        const existing = await subDao.get(orgId, subscriptionId, req.user.sub);
        if (!existing) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }

        const apiId = existing.api_uuid || (existing.dp_api_metadata ? existing.dp_api_metadata.uuid : null) || null;
        if (!apiId) {
            return res.status(400).json({
                code: '400', message: 'Bad Request', description: 'API not found for this subscription',
            });
        }
        if (reqApiId && reqApiId !== apiId) {
            return res.status(400).json({
                code: '400', message: 'Bad Request', description: 'apiId does not match this subscription',
            });
        }

        const apiMetadataResponse = await apiDao.get(orgId, apiId);
        if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'API not found',
            });
        }
        const apiMetadata = apiMetadataResponse[0];
        const plans = apiMetadata.dp_subscription_plans || [];
        const newPlan = plans.find(p => p.uuid === reqPlanId);
        if (!newPlan) {
            return res.status(400).json({
                code: '400', message: 'Bad Request', description: 'Subscription plan not found for this API',
            });
        }

        const previousPlan = existing.dp_subscription_plan;

        await sequelize.transaction(async (t) => {
            const updated = await subDao.updatePlan(orgId, subscriptionId, reqPlanId, req.user.sub, t);
            if (!updated) {
                const err = new Error('Subscription not found');
                err.status = 404;
                throw err;
            }
            const payload = {
                ...buildWebhookPayload(existing, apiMetadata, newPlan),
                previous_plan: {
                    ref_id: previousPlan ? (previousPlan.ref_id || null) : null,
                    name: previousPlan ? (previousPlan.name || null) : null,
                },
            };
            await safePublish('subscription.plan_changed', payload, {
                transaction: t, orgId, aggregateType: 'subscription', aggregateId: subscriptionId,
            });
        });

        logUserAction('SUBSCRIPTION_PLAN_CHANGED', req, { orgId, subscriptionId, planId: reqPlanId });
        const updated = await subDao.get(orgId, subscriptionId, req.user.sub);
        return res.status(200).json(formatSubscriptionResponse(updated));
    } catch (error) {
        if (error.status === 404) {
            return res.status(404).json({ code: '404', message: 'Not Found', description: 'Subscription not found' });
        }
        logger.error('Error changing subscription plan', { error: error.message, subscriptionId });
        util.handleError(res, error);
    }
};

const regenerateSubscriptionToken = async (req, res) => {
    const orgId = req.orgId;
    const subscriptionId = req.params.subId;

    try {
        const existing = await subDao.get(orgId, subscriptionId, req.user.sub);
        if (!existing) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }

        const apiMetadata = existing.dp_api_metadata;
        const plan = existing.dp_subscription_plan;
        let newToken;

        await sequelize.transaction(async (t) => {
            newToken = await subDao.regenerateToken(orgId, subscriptionId, req.user.sub, t);
            if (!newToken) {
                const err = new Error('Subscription not found');
                err.status = 404;
                throw err;
            }
            await safePublish('subscription.token_regenerated', buildWebhookPayload(existing, apiMetadata, plan), {
                transaction: t, orgId, aggregateType: 'subscription', aggregateId: subscriptionId,
                secretFields: { token: newToken },
            });
        });

        logUserAction('SUBSCRIPTION_TOKEN_REGENERATED', req, { orgId, subscriptionId });
        const updated = await subDao.get(orgId, subscriptionId, req.user.sub);
        return res.status(200).json(formatSubscriptionResponse(updated));
    } catch (error) {
        if (error.status === 404) {
            return res.status(404).json({ code: '404', message: 'Not Found', description: 'Subscription not found' });
        }
        logger.error('Error regenerating subscription token', { error: error.message, subscriptionId });
        util.handleError(res, error);
    }
};

const deleteSubscription = async (req, res) => {
    const orgId = req.orgId;
    const subscriptionId = req.params.subId;

    try {
        const existing = await subDao.get(orgId, subscriptionId, req.user.sub);
        if (!existing) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }

        const apiMetadata = existing.dp_api_metadata;
        const plan = existing.dp_subscription_plan;

        await sequelize.transaction(async (t) => {
            const deleted = await subDao.delete(orgId, subscriptionId, req.user.sub, t);
            if (!deleted) throw Object.assign(new Error('Not found'), { statusCode: 404 });
            await safePublish('subscription.deleted', buildWebhookPayload(existing, apiMetadata, plan), {
                transaction: t,
                orgId: orgId,
                aggregateType: 'subscription',
                aggregateId: subscriptionId,
            });
        });

        logUserAction('SUBSCRIPTION_DELETED', req, { orgId: orgId, subscriptionId });
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
    changePlan,
    regenerateSubscriptionToken,
};
