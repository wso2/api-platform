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
const { config } = require('../../config/configLoader');
const eventDao = require('../../dao/eventDao');
const { matchSubscribers } = require('./subscriberRegistry');
const { onPublished } = require('./eventPublisher');
const sequelize = require('../../db/sequelizeConfig');
const DPEvent = require('../../models/event');
const logger = require('../../config/logger');

let running = false;
let intervalHandle = null;

/**
 * Process one batch of PENDING (non-key) events: resolve subscribers, create
 * delivery rows, mark events as DISPATCHED.
 */
async function runBatch() {
    const delivery = config.webhooks && config.webhooks.delivery;
    const batchSize = (delivery && delivery.batchSize) || 50;
    const events = await eventDao.claimPending(batchSize);
    if (events.length === 0) return;

    for (const event of events) {
        try {
            const subscribers = await matchSubscribers(event.ORG_ID, event.EVENT_TYPE);
            if (subscribers.length === 0) {
                // No matching subscribers — mark as delivered immediately.
                await DPEvent.update({ STATUS: 'ALL_DELIVERED' }, { where: { ID: event.ID } });
                continue;
            }
            await eventDao.createDeliveries(event.ID, subscribers, null, null);
        } catch (err) {
            logger.error('Failed to create deliveries for event', {
                eventId: event.ID, error: err.message
            });
            try {
                await DPEvent.update({ STATUS: 'PENDING' }, { where: { ID: event.ID } });
                logger.info('Restored event eligibility after delivery creation failure', {
                    eventId: event.ID
                });
            } catch (restoreErr) {
                logger.error('Failed to restore event eligibility', {
                    eventId: event.ID, error: restoreErr.message
                });
            }
        }
    }
}

function start() {
    if (running) return;
    running = true;

    const delivery = config.webhooks && config.webhooks.delivery;
    const pollMs = (delivery && delivery.pollIntervalMs) || 2000;

    async function tick() {
        try {
            await runBatch();
        } catch (err) {
            logger.error('Batch error', { error: err.message || String(err) });
        }
    }

    intervalHandle = setInterval(tick, pollMs);
    // Also run immediately on event_published signals (no-op if nothing pending).
    onPublished(tick);

    logger.info('Dispatcher started', { pollIntervalMs: pollMs });
}

function stop() {
    running = false;
    if (intervalHandle) {
        clearInterval(intervalHandle);
        intervalHandle = null;
    }
}

module.exports = { start, stop };
