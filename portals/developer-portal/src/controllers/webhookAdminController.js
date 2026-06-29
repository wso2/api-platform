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
const eventDao = require('../dao/eventDao');
const logger = require('../config/logger');

function formatDelivery(d) {
    return {
        deliveryId: d.UUID,
        subscriberId: d.SUBSCRIBER_ID,
        targetUrl: d.TARGET_URL || null,
        status: d.STATUS,
        lastHttpStatus: d.LAST_HTTP_STATUS || null,
        lastError: d.LAST_ERROR || null,
        lastAttemptAt: d.LAST_ATTEMPT_AT || null,
        deliveredAt: d.DELIVERED_AT || null,
    };
}

function formatEvent(row) {
    const deliveries = (row.DP_EVENT_DELIVERIES || []).map(formatDelivery);
    return {
        eventId: row.UUID,
        eventType: row.TYPE,
        orgId: row.ORG_UUID,
        aggregateType: row.AGGREGATE_TYPE,
        aggregateId: row.AGGREGATE_UUID,
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
        const orgId = req.orgId;
        const { status, limit = '20', offset = '0' } = req.query;
        const parsedLimit = Math.max(1, Math.min(parseInt(limit, 10) || 20, 100));
        const parsedOffset = Math.max(0, parseInt(offset, 10) || 0);
        const result = await eventDao.list({
            orgId,
            status: status || undefined,
            limit: parsedLimit,
            offset: parsedOffset,
        });
        res.json({
            list: result.rows.map(formatEvent),
            pagination: { total: result.count, limit: parsedLimit, offset: parsedOffset },
        });
    } catch (err) {
        logger.error('Failed to list events', { error: err.message });
        res.status(500).json({ message: err.message });
    }
}

/**
 * GET /organizations/:orgId/events/:eventId
 */
async function getEvent(req, res) {
    try {
        const event = await eventDao.get(req.params.eventId);
        if (!event || event.ORG_UUID !== req.orgId) {
            return res.status(404).json({ message: 'Event not found' });
        }
        res.json(formatEvent(event));
    } catch (err) {
        logger.error('Failed to get event', { error: err.message });
        res.status(500).json({ message: err.message });
    }
}

module.exports = { listEvents, getEvent };
