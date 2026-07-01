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
async function create({ eventType, orgId, aggregateType, aggregateId, payload }, transaction) {
    return DPEvent.create(
        { type: eventType, org_uuid: orgId,
          aggregate_type: aggregateType, aggregate_uuid: aggregateId, payload: payload || {} },
        { transaction }
    );
}

/**
 * Write delivery rows for a set of subscribers, within the caller's transaction.
 * perSubscriberEncrypted: { [subscriberId]: { [fieldName]: encryptedEnvelope } }
 * The per-subscriber map is stored as-is in encrypted_fields and merged into
 * the webhook payload's `data` by the delivery worker.
 */
async function createDeliveries(eventId, subscribers, perSubscriberEncrypted, transaction) {
    const rows = subscribers.map(sub => ({
        event_uuid: eventId,
        subscriber_id: sub.id,
        target_url: sub.url,
        encrypted_fields: (perSubscriberEncrypted && perSubscriberEncrypted[sub.id]) || null,
        status: 'PENDING'
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
            where: { status: 'PENDING' },
            order: [['occurred_at', 'ASC']],
            limit: batchSize,
            transaction: t,
        };
        if (isPostgres) {
            findOpts.lock = t.LOCK.UPDATE;
            findOpts.skipLocked = true;
        }
        const events = await DPEvent.findAll(findOpts);
        if (events.length === 0) return [];
        const ids = events.map(e => e.uuid);
        await DPEvent.update({ status: 'DISPATCHED' }, { where: { uuid: { [Op.in]: ids } }, transaction: t });
        return events;
    });
}

/**
 * Claim a batch of PENDING delivery rows using SELECT FOR UPDATE SKIP LOCKED.
 */
async function claimDueDeliveries(batchSize) {
    const isPostgres = sequelize.getDialect() === 'postgres';
    const txOpts = isPostgres ? { isolationLevel: Sequelize.Transaction.ISOLATION_LEVELS.READ_COMMITTED } : {};
    return sequelize.transaction(txOpts, async (t) => {
        // Recover stale IN_FLIGHT rows left by a crashed or stopped worker. Any delivery
        // that has been IN_FLIGHT for more than 5 minutes without a terminal update is
        // marked FAILED so it re-enters PENDING on the next dispatch cycle.
        const staleThreshold = new Date(Date.now() - 5 * 60 * 1000);
        await DPEventDelivery.update(
            { status: 'FAILED', last_error: 'Delivery abandoned: worker stopped mid-flight' },
            { where: { status: 'IN_FLIGHT', last_attempt_at: { [Op.lt]: staleThreshold } }, transaction: t }
        );

        const findOpts = {
            where: { status: 'PENDING' },
            limit: batchSize,
            transaction: t,
        };
        if (isPostgres) {
            findOpts.lock = t.LOCK.UPDATE;
            findOpts.skipLocked = true;
        }
        const rows = await DPEventDelivery.findAll(findOpts);
        if (rows.length === 0) return [];
        const ids = rows.map(r => r.uuid);
        await DPEventDelivery.update(
            { status: 'IN_FLIGHT', last_attempt_at: new Date() },
            { where: { uuid: { [Op.in]: ids } }, transaction: t }
        );
        return rows;
    });
}

/**
 * Mark a delivery as delivered.
 */
async function markDelivered(deliveryId, httpStatus) {
    await DPEventDelivery.update(
        { status: 'DELIVERED', last_http_status: httpStatus, delivered_at: new Date() },
        { where: { uuid: deliveryId } }
    );
    await reconcile(await DPEventDelivery.findByPk(deliveryId));
}

/**
 * Mark a delivery as failed. Single attempt — no retry scheduling.
 */
async function markFailed(deliveryId, { httpStatus, error }) {
    await DPEventDelivery.update(
        {
            status: 'FAILED',
            last_http_status: httpStatus ?? null,
            last_error: error ? String(error).slice(0, 1000) : null
        },
        { where: { uuid: deliveryId } }
    );
    await reconcile(await DPEventDelivery.findByPk(deliveryId));
}

/**
 * If all deliveries for an event are terminal (DELIVERED or FAILED),
 * update the event status accordingly.
 */
async function reconcile(delivery) {
    if (!delivery) return;
    const all = await DPEventDelivery.findAll({ where: { event_uuid: delivery.event_uuid } });
    if (all.length === 0) return;
    const terminal = all.every(d => d.status === 'DELIVERED' || d.status === 'FAILED');
    if (!terminal) return;
    const allDelivered = all.every(d => d.status === 'DELIVERED');
    await DPEvent.update(
        { status: allDelivered ? 'ALL_DELIVERED' : 'FAILED' },
        { where: { uuid: delivery.event_uuid } }
    );
}

/**
 * Admin: list recent events with delivery counts.
 */
async function list({ orgId, status, limit = 50, offset = 0 }) {
    const where = {};
    if (orgId) where.org_uuid = orgId;
    if (status) where.status = status;
    return DPEvent.findAndCountAll({
        where,
        order: [['occurred_at', 'DESC']],
        limit,
        offset,
        include: [{ model: DPEventDelivery, attributes: ['uuid', 'subscriber_id', 'status', 'delivered_at'] }],
    });
}

/**
 * Admin: get a single event with all delivery details.
 */
async function get(eventId) {
    return DPEvent.findByPk(eventId, {
        include: [{ model: DPEventDelivery }],
    });
}

/**
 * List the most recent delivery attempts for a single webhook subscriber,
 * newest event first. Used by the webhook subscriber's "recent deliveries" log.
 */
async function listDeliveriesForSubscriber(orgId, subscriberId, limit = 20) {
    return DPEventDelivery.findAll({
        where: { subscriber_id: subscriberId },
        include: [{ model: DPEvent, where: { org_uuid: orgId }, attributes: ['type', 'occurred_at'] }],
        order: [[DPEvent, 'occurred_at', 'DESC']],
        limit
    });
}

module.exports = {
    create, createDeliveries,
    claimPending, claimDueDeliveries,
    markDelivered, markFailed,
    list, get, listDeliveriesForSubscriber,
    reconcile
};
