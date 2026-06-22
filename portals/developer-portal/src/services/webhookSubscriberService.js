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
const { WebhookSubscriberDTO } = require('../dto/webhookSubscriberDto');
const constants = require('../utils/constants');
const logger = require('../config/logger');

function _validateRequiredFields(payload) {
    const missing = ['name', 'url', 'secret'].filter(f => !payload[f]);
    if (missing.length) {
        return `Missing required fields: ${missing.join(', ')}`;
    }
    return null;
}

const createWebhookSubscriber = async (req, res) => {
    try {
        const orgId = req.params.orgId;
        const payload = req.body;

        const validationError = _validateRequiredFields(payload);
        if (validationError) {
            return res.status(400).json({ error: validationError });
        }

        const record = await whDao.create(orgId, payload);
        const dto = new WebhookSubscriberDTO(record);
        return res.status(201).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            return res.status(409).json({
                error: `A webhook subscriber with name "${req.body?.name}" already exists in this organization.`
            });
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_CREATE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_CREATE_ERROR });
    }
};

const updateWebhookSubscriber = async (req, res) => {
    try {
        const { subscriberId } = req.params;
        const payload = req.body;

        const [, updatedRows] = await whDao.update(subscriberId, payload);
        const dto = new WebhookSubscriberDTO(updatedRows[0]);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND });
        }
        if (error instanceof Sequelize.UniqueConstraintError) {
            return res.status(409).json({
                error: 'A webhook subscriber with that name already exists in this organization.'
            });
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_UPDATE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_UPDATE_ERROR });
    }
};

const getWebhookSubscribers = async (req, res) => {
    try {
        const orgId = req.params.orgId;
        const records = await whDao.list(orgId);
        const dtos = records.map(r => new WebhookSubscriberDTO(r));
        return res.status(200).json(dtos);
    } catch (error) {
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR });
    }
};

const getWebhookSubscriber = async (req, res) => {
    try {
        const { subscriberId } = req.params;
        const record = await whDao.get(subscriberId);
        const dto = new WebhookSubscriberDTO(record);
        return res.status(200).json(dto);
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            return res.status(404).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_NOT_FOUND });
        }
        logger.error(constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR, { error });
        return res.status(500).json({ error: constants.ERROR_MESSAGE.WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR });
    }
};

const deleteWebhookSubscriber = async (req, res) => {
    try {
        const { subscriberId } = req.params;
        await whDao.delete(subscriberId);
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
    deleteWebhookSubscriber,
};
