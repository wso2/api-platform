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
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const db = require('../db/driver');
const { publish: publishWebhookEvent } = require('./webhooks/eventPublisher');
const util = require('../utils/util');
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');
const planPricingDao = require('../dao/planPricingDao');
const billingSubscriptionDao = require('../dao/billingSubscriptionDao');
const subscriptionBillingService = require('./billing/subscriptionBillingService');

// A paid plan starts with a payment, so it is subscribed to through checkout, never directly.
const PAID_PLAN_CODE = 'PAID_PLAN_REQUIRES_CHECKOUT';

/** Whether the plan (a subscription_plans row) is paid. */
async function isPaidPlan(orgId, plan) {
    return Boolean(plan && await planPricingDao.getByPlan(orgId, plan.uuid));
}

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

// The webhook subscriber_id must be the user's IdP subject (idp_id) — the same identity
// the REST API exposes as `createdBy` — not the portal-internal user UUID stored in
// created_by. Machine-credential callers carry no user identity (created_by === SYSTEM_ACTOR),
// so pass that through verbatim instead of running it through the reference lookup.
async function resolveSubscriberId(createdBy) {
    if (!createdBy || createdBy === constants.SYSTEM_ACTOR) return createdBy;
    return userIdpReferenceDao.resolveDisplay(createdBy);
}

async function buildWebhookPayload(sub, apiMetadata, plan) {
    return {
        subscription_id: sub.uuid,
        subscriber_id: await resolveSubscriberId(sub.created_by),
        status: sub.status,
        subscription_plan: {
            ref_id: plan ? (plan.ref_id || null) : null,
            name: plan ? (plan.display_name || null) : null,
        },
        api: {
            name: apiMetadata ? apiMetadata.name : null,
            version: apiMetadata ? apiMetadata.version : null,
            ref_id: apiMetadata ? (apiMetadata.ref_id || '') : '',
            type: apiMetadata ? (apiMetadata.type || null) : null,
        },
    };
}

/**
 * Creates a subscription inside the caller's transaction and publishes subscription.created.
 * `billing` ({ status, customer_id, subscription_id }) is set for a paid subscription, which is
 * created only after checkout, and travels with the event so the Platform API and gateways never
 * hold a paid subscription without its billing state.
 */
async function createSubscriptionRecord({ orgId, apiId, apiMetadata, plan, createdBy, billing }, t) {
    const newSub = await subDao.create(orgId, apiId, plan.uuid, createdBy, t);
    const payload = await buildWebhookPayload(newSub, apiMetadata, plan);
    if (billing) payload.billing = billing;
    await safePublish('subscription.created', payload, {
        transaction: t,
        orgId,
        aggregateType: 'subscription',
        aggregateId: newSub.uuid,
        secretFields: { token: newSub.token },
    });
    return newSub;
}

/**
 * Publishes subscription.billing_updated for a paid subscription (a subDao row with its
 * api_metadata and subscription_plan) inside the caller's transaction.
 */
async function publishBillingUpdated(orgId, sub, billing, t) {
    const payload = await buildWebhookPayload(sub, sub.api_metadata, sub.subscription_plan);
    payload.billing = billing;
    await safePublish('subscription.billing_updated', payload, {
        transaction: t, orgId, aggregateType: 'subscription', aggregateId: sub.uuid,
    });
}

/**
 * Removes a subscription and tells the Platform API, which stops its key at the gateways. With
 * `createdBy`, only that user's subscription is removed. Resolves to false when there is none.
 */
async function removeSubscription(orgId, sub, createdBy, t) {
    const deleted = await subDao.delete(orgId, sub.uuid, createdBy, t);
    if (!deleted) return false;
    await safePublish('subscription.deleted', await buildWebhookPayload(sub, sub.api_metadata, sub.subscription_plan), {
        transaction: t, orgId, aggregateType: 'subscription', aggregateId: sub.uuid,
    });
    return true;
}

/** "Nov 8, 2026": the day a subscription ends. */
function endDateText(endsAt) {
    return new Date(endsAt).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric', timeZone: 'UTC' });
}

/** The answer to unsubscribing from a plan that runs to the end of its paid period. */
function endingResponse(endsAt) {
    return {
        message: `Your subscription has been cancelled. You will retain access until ${endDateText(endsAt)}.`,
        endsAt: new Date(endsAt).toISOString(),
    };
}

function formatSubscriptionResponse(sub, audit) {
    const plan = sub.subscription_plan || {};
    const api = sub.api_metadata || {};
    return {
        subscriptionId: sub.uuid,
        subscriptionToken: sub.token,
        status: sub.status,
        artifactId: api.handle || sub.api_uuid,
        subscriptionPlanName: plan.display_name || null,
        ...audit,
    };
}

/**
 * Why a user who already has `subscriptionUuid` cannot subscribe to the API again: a paid plan that
 * runs to the end of its period still counts until then.
 */
async function existingSubscriptionMessage(subscriptionUuid) {
    const billing = await billingSubscriptionDao.getBySubscription(subscriptionUuid).catch(() => null);
    return billing && billing.ends_at
        ? `Your subscription to this API ends on ${endDateText(billing.ends_at)}. You can subscribe again after that.`
        : 'A subscription for this API already exists';
}

const createSubscription = async (req, res) => {
    const orgId = req.orgId;
    const { artifactId: apiHandle, subscriptionPlanId: reqPlanHandle } = req.body;
    const createdBy = util.resolveActor(req);
    let apiId;

    if (!apiHandle || typeof apiHandle !== 'string' || !apiHandle.trim()) {
        return util.sendError(res, 400, 'Bad Request', { errors: [{ message: 'artifactId is required' }] });
    }

    try {
        apiId = await apiDao.getId(orgId, apiHandle);
        if (!apiId) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'API not found' }] });
        }
        const apiMetadataResponse = await apiDao.get(orgId, apiId);
        if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'API not found' }] });
        }

        const apiMetadata = apiMetadataResponse[0];

        const plans = apiMetadata.subscription_plans || [];
        if (plans.length === 0) {
            return util.sendError(res, 400, 'Bad Request', { errors: [{ message: 'This API does not support subscriptions' }] });
        }

        const matchedPlan = plans.find(p => p.handle === reqPlanHandle);
        if (!matchedPlan) {
            return util.sendError(res, 400, 'Bad Request', { errors: [{ message: 'Subscription plan not found for this API' }] });
        }
        if (await isPaidPlan(orgId, matchedPlan)) {
            return util.sendError(res, 409, 'This is a paid plan. Subscribe to it through checkout.', { code: PAID_PLAN_CODE });
        }

        // One subscription per user per API.
        const existing = await subDao.findForUser(orgId, apiId, createdBy);
        if (existing) {
            return util.sendError(res, 409, 'Conflict', { errors: [{ message: await existingSubscriptionMessage(existing) }] });
        }

        // An open checkout for a paid plan of the API is ended first, so it can no longer be paid.
        // Loaded here: checkoutService loads this module, so loading it at the top would be circular.
        const checkoutService = require('./billing/checkoutService');
        try {
            await checkoutService.endOpenCheckout(orgId, createdBy, apiId);
        } catch (error) {
            return checkoutService.sendCheckoutError(res, error, 'Error while ending an open checkout');
        }

        let newSub;
        await db.withTransaction(async (t) => {
            newSub = await createSubscriptionRecord({ orgId, apiId, apiMetadata, plan: matchedPlan, createdBy }, t);
        });

        const created = await subDao.get(orgId, newSub.uuid, createdBy);
        logUserAction('SUBSCRIPTION_CREATED', req, { orgId: orgId, apiId, subscriptionId: newSub.uuid, resourceUuid: newSub.uuid, resourceType: 'subscription' });
        const audit = await userIdpReferenceDao.buildSingleAuditFields(created);
        return res.status(201).json(formatSubscriptionResponse(created, audit));
    } catch (error) {
        logger.error('Error creating subscription', {
            error: error.message, orgId, apiId,
        });
        util.handleError(res, error);
    }
};

const listSubscriptions = async (req, res) => {
    const orgId = req.orgId;
    const apiHandle = req.query.artifactId;
    let apiId;

    try {
        if (apiHandle) {
            apiId = await apiDao.getId(orgId, apiHandle);
            if (!apiId) {
                return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'API not found' }] });
            }
        }

        const subs = await subDao.list(orgId, { apiId, createdBy: util.resolveActor(req) });
        const auditList = await userIdpReferenceDao.buildListAuditFields(subs);
        return res.status(200).json(util.toPaginatedList(subs.map((sub, i) => formatSubscriptionResponse(sub, auditList[i])), req));
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
        const sub = await subDao.get(orgId, subscriptionId, util.resolveActor(req));
        if (!sub) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
        }
        const audit = await userIdpReferenceDao.buildSingleAuditFields(sub);
        return res.status(200).json(formatSubscriptionResponse(sub, audit));
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
        return util.sendError(res, 400, 'Bad Request', { errors: [{ message: `Invalid status. Must be one of: ${Object.values(constants.SUBSCRIPTION_STATUS).join(', ')}.` }] });
    }

    const actorId = util.resolveActor(req);
    try {
        const existing = await subDao.get(orgId, subscriptionId, actorId);
        if (!existing) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
        }
        // Stripe keeps charging a paid subscription regardless of its status here, and the gateway
        // reads an inactive paid subscription as unpaid; a paid one is stopped by unsubscribing.
        if (await billingSubscriptionDao.getBySubscription(subscriptionId)) {
            return util.sendError(res, 409, 'A paid subscription cannot be suspended. Unsubscribe to stop it.',
                { code: PAID_PLAN_CODE });
        }

        let sub;
        await db.withTransaction(async (t) => {
            const updated = await subDao.updateStatus(orgId, subscriptionId, status, actorId, t);
            if (!updated) {
                const err = new Error('Subscription not found');
                err.status = 404;
                throw err;
            }
            await publishWebhookEvent('subscription.updated',
                await buildWebhookPayload({ ...existing, status: status }, existing.api_metadata, existing.subscription_plan),
                { transaction: t, orgId: orgId, aggregateType: 'subscription', aggregateId: subscriptionId });
        });
        sub = await subDao.get(orgId, subscriptionId, actorId);
        const audit = await userIdpReferenceDao.buildSingleAuditFields(sub);
        return res.status(200).json(formatSubscriptionResponse(sub, audit));
    } catch (error) {
        if (error.status === 404) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
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
    const { artifactId: reqApiHandle, planId: reqPlanHandle } = req.body;
    const actorId = util.resolveActor(req);

    try {
        const existing = await subDao.get(orgId, subscriptionId, actorId);
        if (!existing) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
        }

        const apiId = existing.api_uuid || (existing.api_metadata ? existing.api_metadata.uuid : null) || null;
        if (!apiId) {
            return util.sendError(res, 400, 'Bad Request', { errors: [{ message: 'API not found for this subscription' }] });
        }
        const apiHandle = existing.api_metadata ? existing.api_metadata.handle : null;
        if (reqApiHandle && reqApiHandle !== apiHandle) {
            return util.sendError(res, 400, 'Bad Request', { errors: [{ message: 'artifactId does not match this subscription' }] });
        }

        const apiMetadataResponse = await apiDao.get(orgId, apiId);
        if (!apiMetadataResponse || apiMetadataResponse.length === 0) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'API not found' }] });
        }
        const apiMetadata = apiMetadataResponse[0];
        const plans = apiMetadata.subscription_plans || [];
        const newPlan = plans.find(p => p.handle === reqPlanHandle);
        if (!newPlan) {
            return util.sendError(res, 400, 'Bad Request', { errors: [{ message: 'Subscription plan not found for this API' }] });
        }
        const planId = newPlan.uuid;

        const previousPlan = existing.subscription_plan;
        // Moving into or out of a paid plan changes what is billed, which checkout and the
        // billing provider handle; a plan change here only moves between free plans.
        if (await isPaidPlan(orgId, newPlan) || await isPaidPlan(orgId, previousPlan)) {
            return util.sendError(res, 409, 'Paid plans cannot be changed here. Unsubscribe, then subscribe to the new '
                + 'plan.', { code: PAID_PLAN_CODE });
        }

        await db.withTransaction(async (t) => {
            const updated = await subDao.updatePlan(orgId, subscriptionId, planId, actorId, t);
            if (!updated) {
                const err = new Error('Subscription not found');
                err.status = 404;
                throw err;
            }
            const payload = {
                ...(await buildWebhookPayload(existing, apiMetadata, newPlan)),
                previous_plan: {
                    ref_id: previousPlan ? (previousPlan.ref_id || null) : null,
                    name: previousPlan ? (previousPlan.display_name || null) : null,
                },
            };
            await safePublish('subscription.plan_changed', payload, {
                transaction: t, orgId, aggregateType: 'subscription', aggregateId: subscriptionId,
            });
        });

        logUserAction('SUBSCRIPTION_PLAN_CHANGED', req, { orgId, subscriptionId, planId, resourceUuid: subscriptionId, resourceType: 'subscription' });
        const updated = await subDao.get(orgId, subscriptionId, actorId);
        const audit = await userIdpReferenceDao.buildSingleAuditFields(updated);
        return res.status(200).json(formatSubscriptionResponse(updated, audit));
    } catch (error) {
        if (error.status === 404) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
        }
        logger.error('Error changing subscription plan', { error: error.message, subscriptionId });
        util.handleError(res, error);
    }
};

const regenerateSubscriptionToken = async (req, res) => {
    const orgId = req.orgId;
    const subscriptionId = req.params.subId;
    const actorId = util.resolveActor(req);

    try {
        const existing = await subDao.get(orgId, subscriptionId, actorId);
        if (!existing) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
        }

        const apiMetadata = existing.api_metadata;
        const plan = existing.subscription_plan;
        let newToken;

        await db.withTransaction(async (t) => {
            newToken = await subDao.regenerateToken(orgId, subscriptionId, actorId, t);
            if (!newToken) {
                const err = new Error('Subscription not found');
                err.status = 404;
                throw err;
            }
            await safePublish('subscription.token_regenerated', await buildWebhookPayload(existing, apiMetadata, plan), {
                transaction: t, orgId, aggregateType: 'subscription', aggregateId: subscriptionId,
                secretFields: { token: newToken },
            });
        });

        logUserAction('SUBSCRIPTION_TOKEN_REGENERATED', req, { orgId, subscriptionId, resourceUuid: subscriptionId, resourceType: 'subscription' });
        const updated = await subDao.get(orgId, subscriptionId, actorId);
        const audit = await userIdpReferenceDao.buildSingleAuditFields(updated);
        return res.status(200).json(formatSubscriptionResponse(updated, audit));
    } catch (error) {
        if (error.status === 404) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
        }
        logger.error('Error regenerating subscription token', { error: error.message, subscriptionId });
        util.handleError(res, error);
    }
};

const deleteSubscription = async (req, res) => {
    const orgId = req.orgId;
    const subscriptionId = req.params.subId;
    const actorId = util.resolveActor(req);

    try {
        const existing = await subDao.get(orgId, subscriptionId, actorId);
        if (!existing) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
        }

        const apiMetadata = existing.api_metadata;
        const plan = existing.subscription_plan;

        // A paid subscription ends with the payment provider first; if the provider cannot be
        // reached it is kept, so it is never left billing unseen. A plan charged a flat fee runs to
        // the end of the period already paid for and is removed when the provider ends it, unless
        // the subscriber ends it immediately, with no refund for the rest of that period; a plan
        // charged by usage is removed now.
        const immediate = req.query.immediate === true || req.query.immediate === 'true';
        const billing = await billingSubscriptionDao.getBySubscription(subscriptionId);
        if (billing && billing.ends_at && !immediate) {
            return res.status(200).json(endingResponse(billing.ends_at));
        }
        if (billing) {
            let ending;
            try {
                ending = await subscriptionBillingService.endInProvider(billing, { immediate });
            } catch (err) {
                if (err instanceof subscriptionBillingService.SubscriptionBillingError) {
                    return util.sendError(res, err.status, err.message, { code: err.code });
                }
                throw err;
            }
            if (!ending.removeNow) {
                await billingSubscriptionDao.setEndsAt(subscriptionId, ending.endsAt);
                logUserAction('SUBSCRIPTION_END_SCHEDULED', req, { orgId, subscriptionId, resourceUuid: subscriptionId, resourceType: 'subscription' });
                return res.status(200).json(endingResponse(ending.endsAt));
            }
        }

        await db.withTransaction(async (t) => {
            const removed = await removeSubscription(orgId, { ...existing, api_metadata: apiMetadata, subscription_plan: plan },
                actorId, t);
            if (!removed) throw Object.assign(new Error('Not found'), { statusCode: 404 });
        });

        logUserAction('SUBSCRIPTION_DELETED', req, { orgId: orgId, subscriptionId, resourceUuid: subscriptionId, resourceType: 'subscription' });
        return res.status(200).json({ message: 'Subscription deleted successfully' });
    } catch (error) {
        if (error.statusCode === 404) {
            return util.sendError(res, 404, 'Not Found', { errors: [{ message: 'Subscription not found' }] });
        }
        logger.error('Error deleting subscription', {
            error: error.message, subscriptionId,
        });
        util.handleError(res, error);
    }
};

module.exports = {
    existingSubscriptionMessage,
    createSubscriptionRecord,
    publishBillingUpdated,
    removeSubscription,
    createSubscription,
    listSubscriptions,
    getSubscription,
    updateSubscription,
    deleteSubscription,
    changePlan,
    regenerateSubscriptionToken,
};
