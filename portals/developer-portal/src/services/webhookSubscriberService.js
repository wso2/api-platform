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
const { Sequelize } = require('sequelize');
const whDao = require('../dao/webhookSubscriberDao');
const eventDao = require('../dao/eventDao');
const { WebhookSubscriberDTO } = require('../dto/webhookSubscriberDto');
const userIdpReferenceDao = require('../dao/userIdpReferenceDao');
const constants = require('../utils/constants');
const util = require('../utils/util');
const logger = require('../config/logger');

function _validateRequiredFields(payload) {
    const missing = ['name', 'targetUrl'].filter(f => !payload[f]);
    if (missing.length) {
        return `Missing required fields: ${missing.join(', ')}`;
    }
    return null;
}

/**
 * Build a specific conflict message for the name unique constraint.
 */
function _uniqueConstraintMessage(error, payload) {
    const fields = Array.isArray(error.fields)
        ? error.fields
        : error.fields ? Object.keys(error.fields) : (error.errors || []).map(e => e.path);
    if (fields.includes('name')) {
        return `A webhook subscriber with name "${payload?.name}" already exists in this organization.`;
    }
    return 'A webhook subscriber with that name already exists in this organization.';
}

const createWebhookSubscriber = async (req, res) => {
    try {
        const orgId = req.orgId;
        const payload = req.body;

        const validationError = _validateRequiredFields(payload);
        if (validationError) {
            return res.status(400).json({ error: validationError });
        }

        const userId = util.resolveActor(req);
        const record = await whDao.create(orgId, payload, userId);
        const dto = new WebhookSubscriberDTO(record);
        return res.status(201).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            return res.status(409).json({
                error: _uniqueConstraintMessage(error, req.body)
            });
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_CREATE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_CREATE_ERROR });
    }
};

const updateWebhookSubscriber = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { subscriberId } = req.params;
        const payload = req.body;

        const userId = util.resolveActor(req);
        const [, updatedRows] = await whDao.update(orgId, subscriberId, payload, userId);
        const dto = new WebhookSubscriberDTO(updatedRows[0]);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND });
        }
        if (error instanceof Sequelize.UniqueConstraintError) {
            return res.status(409).json({
                error: _uniqueConstraintMessage(error, req.body)
            });
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_UPDATE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_UPDATE_ERROR });
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
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR });
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
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND });
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR });
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
        await whDao.get(orgId, subscriberId);
        const deliveries = await eventDao.listDeliveriesForSubscriber(orgId, subscriberId, 20);
        return res.status(200).json({ list: deliveries.map(_formatDeliverySummary) });
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND });
        }
        logger.error('Error fetching webhook subscriber deliveries', { error });
        return res.status(500).json({ error: 'Failed to fetch webhook subscriber deliveries' });
    }
};

const deleteWebhookSubscriber = async (req, res) => {
    try {
        const orgId = req.orgId;
        const { subscriberId } = req.params;
        await whDao.delete(orgId, subscriberId);
        return res.status(204).send();
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND });
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_DELETE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_DELETE_ERROR });
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
