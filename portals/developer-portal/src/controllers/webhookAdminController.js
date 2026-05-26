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
const eventDao = require('../dao/event');
const logger = require('../config/logger');

function formatDelivery(d) {
    return {
        deliveryId: d.DELIVERY_ID,
        subscriberId: d.SUBSCRIBER_ID,
        targetUrl: d.TARGET_URL || null,
        status: d.STATUS,
        attemptCount: d.ATTEMPT_COUNT,
        lastHttpStatus: d.LAST_HTTP_STATUS || null,
        lastError: d.LAST_ERROR || null,
        lastAttemptAt: d.LAST_ATTEMPT_AT || null,
        deliveredAt: d.DELIVERED_AT || null,
    };
}

function formatEvent(row) {
    const deliveries = (row.DP_EVENT_DELIVERIES || []).map(formatDelivery);
    return {
        eventId: row.EVENT_ID,
        eventType: row.EVENT_TYPE,
        orgId: row.ORG_ID,
        gatewayType: row.GATEWAY_TYPE || null,
        aggregateType: row.AGGREGATE_TYPE,
        aggregateId: row.AGGREGATE_ID,
        status: row.STATUS,
        occurredAt: row.OCCURRED_AT,
        deliveries,
    };
}

/**
 * GET /organizations/:orgId/events
 * Query params: status, limit, offset
 */
async function listEvents(req, res) {
    try {
        const { orgId } = req.params;
        const { status, limit = '50', offset = '0' } = req.query;
        const result = await eventDao.listEvents({
            orgId,
            status: status || undefined,
            limit: Math.min(parseInt(limit, 10) || 50, 200),
            offset: parseInt(offset, 10) || 0
        });
        res.json({ total: result.count, events: result.rows.map(formatEvent) });
    } catch (err) {
        logger.error('[webhookAdmin] listEvents error', { error: err.message });
        res.status(500).json({ message: err.message });
    }
}

/**
 * GET /organizations/:orgId/events/:eventId
 */
async function getEvent(req, res) {
    try {
        const event = await eventDao.getEvent(req.params.eventId);
        if (!event || event.ORG_ID !== req.params.orgId) {
            return res.status(404).json({ message: 'Event not found' });
        }
        res.json(formatEvent(event));
    } catch (err) {
        logger.error('[webhookAdmin] getEvent error', { error: err.message });
        res.status(500).json({ message: err.message });
    }
}

/**
 * POST /organizations/:orgId/admin/deliveries/:deliveryId/retry
 * Resets a DEAD_LETTERED / FAILED delivery to PENDING so the worker retries it.
 * Note: for apikey.* events the encrypted_key was already stored in the delivery row,
 * so replay works — only new generate/regenerate events expose a new plaintext key.
 */
async function retryDelivery(req, res) {
    try {
        const ok = await eventDao.retryDelivery(req.params.deliveryId, req.params.orgId);
        if (!ok) return res.status(404).json({ message: 'Delivery not found or not in a retryable state' });
        res.json({ message: 'Delivery queued for retry' });
    } catch (err) {
        logger.error('[webhookAdmin] retryDelivery error', { error: err.message });
        res.status(500).json({ message: err.message });
    }
}

module.exports = { listEvents, getEvent, retryDelivery };
