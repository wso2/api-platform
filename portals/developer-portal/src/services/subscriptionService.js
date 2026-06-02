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
const apiDao = require('../dao/apiMetadata');
const subDao = require('../dao/subscription');
const sequelize = require('../db/sequelize');
const { publish: publishWebhookEvent } = require('./webhooks/eventPublisher');
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

function buildWebhookPayload(sub, apiMetadata, policy) {
    return {
        subscription: {
            plan_ref_id: policy ? (policy.REF_ID || null) : null,
            plan_name: policy ? (policy.DISPLAY_NAME || policy.POLICY_NAME || null) : null,
            status: sub.STATUS,
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
    const policy = sub.DP_SUBSCRIPTION_POLICY || {};
    return {
        subscriptionId: sub.SUB_ID,
        subscriptionToken: sub.SUB_TOKEN,
        status: sub.STATUS,
        gatewayType: api.GATEWAY_TYPE || null,
        apiId: sub.API_ID,
        subscriptionPlanName: policy.POLICY_NAME || null,
        createdAt: sub.createdAt || null,
    };
}

const createSubscription = async (req, res) => {
    const orgID = req.params.orgId;
    const { apiId, subscriptionPlanName } = req.body;

    if (!apiId) {
        return res.status(400).json({
            code: '400', message: 'Bad Request', description: 'apiId is required',
        });
    }

    try {
        const apiMetadataResponse = await apiDao.getAPIMetadata(orgID, apiId);
        if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'API not found',
            });
        }

        const apiMetadata = apiMetadataResponse[0];

        const plans = apiMetadata.DP_SUBSCRIPTION_POLICies || [];
        if (plans.length === 0) {
            return res.status(400).json({
                code: '400', message: 'Bad Request',
                description: 'This API does not support subscriptions',
            });
        }

        let policyId = null;
        let matchedPlan = null;
        if (subscriptionPlanName) {
            matchedPlan = plans.find(
                p => p.POLICY_NAME === subscriptionPlanName || p.DISPLAY_NAME === subscriptionPlanName
            );
            if (!matchedPlan) {
                return res.status(400).json({
                    code: '400', message: 'Bad Request',
                    description: `Subscription plan '${subscriptionPlanName}' not found for this API`,
                });
            }
            policyId = matchedPlan.POLICY_ID;
        }

        let newSub;
        await sequelize.transaction(async (t) => {
            newSub = await subDao.createSubscription(
                orgID, apiId, policyId, t
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

        const created = await subDao.getSubscription(orgID, newSub.SUB_ID);
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
            const apiMetadataResponse = await apiDao.getAPIMetadata(orgID, apiId);
            if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
                return res.status(404).json({
                    code: '404', message: 'Not Found', description: 'API not found',
                });
            }
        }

        const subs = await subDao.listSubscriptions(orgID, { apiId });
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
    const subscriptionId = req.params.subscriptionId;

    try {
        const sub = await subDao.getSubscription(orgID, subscriptionId);
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
    const subscriptionId = req.params.subscriptionId;
    const { status } = req.body;

    if (!status || !['ACTIVE', 'INACTIVE'].includes(status)) {
        return res.status(400).json({
            code: '400', message: 'Bad Request',
            description: "status must be 'ACTIVE' or 'INACTIVE'",
        });
    }

    try {
        const updated = await subDao.updateSubscriptionStatus(
            orgID, subscriptionId, status
        );
        if (!updated) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }
        const sub = await subDao.getSubscription(orgID, subscriptionId);
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
    const subscriptionId = req.params.subscriptionId;

    try {
        const existing = await subDao.getSubscription(orgID, subscriptionId);
        if (!existing) {
            return res.status(404).json({
                code: '404', message: 'Not Found', description: 'Subscription not found',
            });
        }

        const apiMetadata = existing.DP_API_METADATA;
        const policy = existing.DP_SUBSCRIPTION_POLICY;

        await sequelize.transaction(async (t) => {
            const deleted = await subDao.deleteSubscription(orgID, subscriptionId, t);
            if (!deleted) throw Object.assign(new Error('Not found'), { statusCode: 404 });
            await safePublish('subscription.deleted', buildWebhookPayload(existing, apiMetadata, policy), {
                transaction: t,
                orgId: orgID,
                gatewayType: apiMetadata ? apiMetadata.GATEWAY_TYPE : null,
                aggregateType: 'subscription',
                aggregateId: subscriptionId,
            });
        });

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
