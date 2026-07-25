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
const logger = require('../config/logger');
const { logUserAction } = require('../middlewares/auditLogger');

function _validateRequiredFields(payload) {
    const missing = ['handle', 'targetUrl'].filter(f => !payload[f]);
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
        if (payload && payload.id) {
            payload.handle = payload.id;
        }
        payload.displayName = payload.displayName || payload.handle;

        const validationError = _validateRequiredFields(payload);
        if (validationError) {
            return util.sendError(res, 400, validationError);
        }

        const userId = util.resolveActor(req);
        const record = await whDao.create(orgId, payload, userId);
        logUserAction('WEBHOOK_SUBSCRIBER_CREATED', req, { orgId, subscriberId: record.uuid, resourceUuid: record.uuid, resourceType: 'webhook_subscriber' });
        const audit = await userIdpReferenceDao.buildSingleAuditFields(record);
        const dto = new WebhookSubscriberDTO(record, audit);
        return res.status(201).json(dto);
    } catch (error) {
        if (db.isDuplicateKeyError(error)) {
            return util.sendError(res, 409, _uniqueConstraintMessage(req.body));
        }
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
