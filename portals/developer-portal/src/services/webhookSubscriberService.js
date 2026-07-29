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
const db = require('../db/driver');
const { NotFoundError } = require('../utils/errors/customErrors');
const whDao = require('../dao/webhookSubscriberDao');
const eventDao = require('../dao/eventDao');
const { WebhookSubscriberDTO } = require('../dto/webhookSubscriberDto');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const constants = require('../utils/constants');
const util = require('../utils/util');
const { slugifyHandle, handleCandidate, uuidHandle } = require('../utils/handleSlug');
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');

// Generated-handle collision strategy: try the readable numeric ladder first
// (base, base-2, base-3), then fall back to a plain UUID. The UUID attempts are what
// make failure effectively unreachable, so a caller is never asked to rename a webhook
// just because the name is popular; the numeric ladder exists only because it reads
// better for the ordinary duplicate-name case. A name that slugifies to nothing skips
// the ladder and goes straight to a UUID. Both are bounded so a broken unique index
// can't spin indefinitely inside a request.
const NUMERIC_HANDLE_ATTEMPTS = 3;
const UUID_HANDLE_ATTEMPTS = 2;
const MAX_HANDLE_ATTEMPTS = NUMERIC_HANDLE_ATTEMPTS + UUID_HANDLE_ATTEMPTS;

function _validateRequiredFields(payload) {
    // `handle` is intentionally absent: it is either supplied by the caller as `id`
    // or derived from displayName below, so it is never a field the caller must send.
    const missing = ['displayName', 'targetUrl'].filter(f => !payload[f]);
    if (missing.length) {
        return `Missing required fields: ${missing.join(', ')}`;
    }
    return null;
}


/**
 * Build a specific conflict message for the handle unique constraint.
 * dp_webhook_subscribers has exactly one unique constraint — (org_uuid, handle) —
 * so any duplicate-key error from this table is always a handle collision. The raw
 * driver error (unlike the previous Sequelize.UniqueConstraintError) doesn't carry
 * a structured `fields` list, so there's nothing left to branch on here.
 */
function _uniqueConstraintMessage(payload) {
    return `A webhook subscriber with id "${payload?.id}" already exists in this organization.`;
}

const createWebhookSubscriber = async (req, res) => {
    try {
        const orgId = req.orgId;
        const payload = req.body;

        // `id` is optional. The settings UI never sends one — the handle is derived
        // from displayName here — but it stays accepted so an API client can still
        // pick a stable identifier of its own (and so existing callers keep working).
        const callerSuppliedId = !!(payload && payload.id);
        if (callerSuppliedId) {
            payload.handle = payload.id;
        }

        const validationError = _validateRequiredFields(payload);
        if (validationError) {
            return util.sendError(res, 400, validationError);
        }

        const base = callerSuppliedId
            ? payload.handle
            : slugifyHandle(payload.displayName); // may be '' — the loop uses a UUID then
        const userId = util.resolveActor(req);

        // A caller-supplied handle is taken at face value: a collision is their
        // conflict to resolve (409). A generated one is ours, so a collision just
        // means trying the next suffix. Retrying on the insert's duplicate-key error
        // rather than pre-checking availability keeps this correct under concurrent
        // creates, where a check-then-insert would race.
        const attempts = callerSuppliedId ? 1 : MAX_HANDLE_ATTEMPTS;
        for (let attempt = 0; attempt < attempts; attempt++) {
            payload.handle = (base && attempt < NUMERIC_HANDLE_ATTEMPTS)
                ? handleCandidate(base, attempt)
                : uuidHandle();
            try {
                const record = await whDao.create(orgId, payload, userId);
                logUserAction('WEBHOOK_SUBSCRIBER_CREATED', req, { orgId, subscriberId: record.uuid, resourceUuid: record.uuid, resourceType: 'webhook_subscriber' });
                const audit = await userIdpReferenceDao.buildSingleAuditFields(record);
                const dto = new WebhookSubscriberDTO(record, audit);
                return res.status(201).json(dto);
            } catch (error) {
                if (!db.isDuplicateKeyError(error)) {
                    throw error;
                }
                if (callerSuppliedId) {
                    return util.sendError(res, 409, _uniqueConstraintMessage(req.body));
                }
                // Generated handle collided — fall through and try the next suffix.
            }
        }

        // Effectively unreachable: reaching here means the numeric ladder collided AND
        // every random suffix collided too, which points at a broken unique index rather
        // than an unlucky caller — so this is logged as a server fault, not a 4xx.
        logger.error('Exhausted webhook subscriber handle generation attempts', { orgId, base, attempts });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_CREATE_ERROR);
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_CREATE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_CREATE_ERROR);
    }
};

const updateWebhookSubscriber = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { subscriberId } = req.params;
        const payload = req.body;
        if (payload && payload.id) {
            payload.handle = payload.id;
        }

        const userId = util.resolveActor(req);
        const [, updatedRows] = await whDao.update(orgId, subscriberId, payload, userId);
        logUserAction('WEBHOOK_SUBSCRIBER_UPDATED', req, { orgId, subscriberId, resourceUuid: updatedRows[0].uuid, resourceType: 'webhook_subscriber' });
        const audit = await userIdpReferenceDao.buildSingleAuditFields(updatedRows[0]);
        const dto = new WebhookSubscriberDTO(updatedRows[0], audit);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof NotFoundError) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND);
        }
        if (db.isDuplicateKeyError(error)) {
            return util.sendError(res, 409, _uniqueConstraintMessage(req.body));
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_UPDATE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_UPDATE_ERROR);
    }
};

const getWebhookSubscribers = async (req, res) => {
    try {
        const orgId = req.orgId;
        const records = await whDao.list(orgId);
        const auditList = await userIdpReferenceDao.buildListAuditFields(records);
        const dtos = records.map((r, i) => new WebhookSubscriberDTO(r, auditList[i]));
        return res.status(200).json(util.toPaginatedList(dtos, req));
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR);
    }
};

const getWebhookSubscriber = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { subscriberId } = req.params;
        const record = await whDao.get(orgId, subscriberId);
        const audit = await userIdpReferenceDao.buildSingleAuditFields(record);
        const dto = new WebhookSubscriberDTO(record, audit);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof NotFoundError) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND);
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR);
    }
};

function _formatDeliverySummary(delivery) {
    const event = delivery.dp_event;
    return {
        deliveryId: delivery.uuid,
        eventType: event ? event.type : null,
        occurredAt: event ? event.occurred_at : null,
        status: delivery.status,
        lastHttpStatus: delivery.last_http_status || null,
        lastError: delivery.last_error || null,
        lastAttemptAt: delivery.last_attempt_at || null,
        deliveredAt: delivery.delivered_at || null,
    };
}

const getWebhookSubscriberDeliveries = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { subscriberId } = req.params;
        const sub = await whDao.get(orgId, subscriberId);
        const deliveries = await eventDao.listDeliveriesForSubscriber(orgId, sub.uuid, 20);
        return res.status(200).json(util.toPaginatedList(deliveries.map(_formatDeliverySummary), req));
    } catch (error) {
        if (error instanceof NotFoundError) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND);
        }
        logger.error('Error fetching webhook subscriber deliveries', { error });
        return util.sendError(res, 500, 'Failed to fetch webhook subscriber deliveries');
    }
};

const deleteWebhookSubscriber = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { subscriberId } = req.params;
        const sub = await whDao.get(orgId, subscriberId);
        await whDao.delete(orgId, subscriberId);
        logUserAction('WEBHOOK_SUBSCRIBER_DELETED', req, { orgId, subscriberId, resourceUuid: sub.uuid, resourceType: 'webhook_subscriber' });
        return res.status(204).send();
    } catch (error) {
        if (error instanceof NotFoundError) {
            return util.sendError(res, 404, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND);
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_DELETE_ERROR, { error });
        return util.sendError(res, 500, constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_DELETE_ERROR);
    }
};

module.exports = {
    createWebhookSubscriber,
    updateWebhookSubscriber,
    getWebhookSubscribers,
    getWebhookSubscriber,
    getWebhookSubscriberDeliveries,
    deleteWebhookSubscriber,
};
