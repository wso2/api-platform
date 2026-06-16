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
const { Op, Sequelize } = require('sequelize');
const sequelize = require('../db/sequelizeConfig');
const DPEvent = require('../models/event');
const DPEventDelivery = require('../models/eventDelivery');

/**
 * Write an event row within the caller's transaction.
 * Returns the created event instance.
 */
async function create({ eventType, orgId, gatewayType, aggregateType, aggregateId, payload }, transaction) {
    return DPEvent.create(
        { EVENT_TYPE: eventType, ORG_ID: orgId, GATEWAY_TYPE: gatewayType || null,
          AGGREGATE_TYPE: aggregateType, AGGREGATE_ID: aggregateId, PAYLOAD: payload || {} },
        { transaction }
    );
}

/**
 * Write delivery rows for a set of subscribers, within the caller's transaction.
 * perSubscriberEncrypted: { [subscriberId]: { ...encryptedFields } }
 */
async function createDeliveries(eventId, subscribers, perSubscriberEncrypted, transaction) {
    const rows = subscribers.map(sub => ({
        EVENT_ID: eventId,
        SUBSCRIBER_ID: sub.id,
        TARGET_URL: sub.url,
        ENCRYPTED_FIELDS: (perSubscriberEncrypted && perSubscriberEncrypted[sub.id]) || null,
        STATUS: 'PENDING',
        ATTEMPT_COUNT: 0,
        NEXT_ATTEMPT_AT: new Date()
    }));
    return DPEventDelivery.bulkCreate(rows, { transaction });
}

/**
 * Claim a batch of PENDING events using SELECT FOR UPDATE SKIP LOCKED.
 * Returns events with their delivery rows.
 */
async function claimPending(batchSize) {
    const isPostgres = sequelize.getDialect() === 'postgres';
    const txOpts = isPostgres ? { isolationLevel: Sequelize.Transaction.ISOLATION_LEVELS.READ_COMMITTED } : {};
    return sequelize.transaction(txOpts, async (t) => {
        const findOpts = {
            where: { STATUS: 'PENDING' },
            order: [['OCCURRED_AT', 'ASC']],
            limit: batchSize,
            transaction: t,
        };
        if (isPostgres) {
            findOpts.lock = t.LOCK.UPDATE;
            findOpts.skipLocked = true;
        }
        const events = await DPEvent.findAll(findOpts);
        if (events.length === 0) return [];
        const ids = events.map(e => e.EVENT_ID);
        await DPEvent.update({ STATUS: 'DISPATCHED' }, { where: { EVENT_ID: { [Op.in]: ids } }, transaction: t });
        return events;
    });
}

/**
 * Claim a batch of due PENDING delivery rows using SELECT FOR UPDATE SKIP LOCKED.
 */
async function claimDueDeliveries(batchSize) {
    const isPostgres = sequelize.getDialect() === 'postgres';
    const txOpts = isPostgres ? { isolationLevel: Sequelize.Transaction.ISOLATION_LEVELS.READ_COMMITTED } : {};
    return sequelize.transaction(txOpts, async (t) => {
        const findOpts = {
            where: { STATUS: 'PENDING', NEXT_ATTEMPT_AT: { [Op.lte]: new Date() } },
            order: [['NEXT_ATTEMPT_AT', 'ASC']],
            limit: batchSize,
            transaction: t,
        };
        if (isPostgres) {
            findOpts.lock = t.LOCK.UPDATE;
            findOpts.skipLocked = true;
        }
        const rows = await DPEventDelivery.findAll(findOpts);
        if (rows.length === 0) return [];
        const ids = rows.map(r => r.DELIVERY_ID);
        await DPEventDelivery.update(
            { STATUS: 'IN_FLIGHT', LAST_ATTEMPT_AT: new Date() },
            { where: { DELIVERY_ID: { [Op.in]: ids } }, transaction: t }
        );
        return rows;
    });
}

/**
 * Mark a delivery as delivered.
 */
async function markDelivered(deliveryId, httpStatus) {
    await DPEventDelivery.update(
        { STATUS: 'DELIVERED', LAST_HTTP_STATUS: httpStatus, DELIVERED_AT: new Date() },
        { where: { DELIVERY_ID: deliveryId } }
    );
    await reconcile(await DPEventDelivery.findByPk(deliveryId));
}

/**
 * Schedule a retry or dead-letter a delivery that failed.
 */
async function markFailed(deliveryId, { httpStatus, error, attemptCount, nextAttemptAt, deadLetter }) {
    const update = {
        STATUS: deadLetter ? 'DEAD_LETTERED' : 'PENDING',
        ATTEMPT_COUNT: attemptCount,
        LAST_HTTP_STATUS: httpStatus || null,
        LAST_ERROR: error ? String(error).slice(0, 1000) : null,
        NEXT_ATTEMPT_AT: deadLetter ? new Date() : nextAttemptAt
    };
    await DPEventDelivery.update(update, { where: { DELIVERY_ID: deliveryId } });
    if (deadLetter) {
        await reconcile(await DPEventDelivery.findByPk(deliveryId));
    }
}

/**
 * If all deliveries for an event are terminal (DELIVERED or DEAD_LETTERED),
 * update the event status accordingly.
 */
async function reconcile(delivery) {
    if (!delivery) return;
    const all = await DPEventDelivery.findAll({ where: { EVENT_ID: delivery.EVENT_ID } });
    if (all.length === 0) return;
    const terminal = all.every(d => d.STATUS === 'DELIVERED' || d.STATUS === 'DEAD_LETTERED');
    if (!terminal) return;
    const allDelivered = all.every(d => d.STATUS === 'DELIVERED');
    await DPEvent.update(
        { STATUS: allDelivered ? 'ALL_DELIVERED' : 'FAILED' },
        { where: { EVENT_ID: delivery.EVENT_ID } }
    );
}

/**
 * Admin: list recent events with delivery counts.
 */
async function list({ orgId, status, limit = 50, offset = 0 }) {
    const where = {};
    if (orgId) where.ORG_ID = orgId;
    if (status) where.STATUS = status;
    return DPEvent.findAndCountAll({
        where,
        order: [['OCCURRED_AT', 'DESC']],
        limit,
        offset,
        include: [{ model: DPEventDelivery, attributes: ['DELIVERY_ID', 'SUBSCRIBER_ID', 'STATUS', 'ATTEMPT_COUNT', 'DELIVERED_AT'] }]
    });
}

/**
 * Admin: get a single event with all delivery details.
 */
async function get(eventId) {
    return DPEvent.findByPk(eventId, {
        include: [{ model: DPEventDelivery }]
    });
}

/**
 * Admin: reset a single delivery to PENDING so the worker retries immediately.
 */
async function retryDelivery(deliveryId, orgId) {
    const [count] = await DPEventDelivery.update(
        { STATUS: 'PENDING', NEXT_ATTEMPT_AT: new Date(), LAST_ERROR: null },
        {
            where: {
                DELIVERY_ID: deliveryId,
                STATUS: { [Op.in]: ['DEAD_LETTERED', 'FAILED', 'IN_FLIGHT'] },
                EVENT_ID: {
                    [Op.in]: sequelize.literal(
                        `(SELECT EVENT_ID FROM DP_EVENT WHERE ORG_ID = ${sequelize.escape(orgId)})`
                    )
                }
            }
        }
    );
    return count > 0;
}

module.exports = {
    create, createDeliveries,
    claimPending, claimDueDeliveries,
    markDelivered, markFailed,
    list, get, retryDelivery,
    reconcile
};
