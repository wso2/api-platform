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
'use strict';

const crypto = require('crypto');
const db = require('../db/driver');
const { groupBy, parseJsonColumn } = require('../db/rows');

const EVENTS_TABLE = 'dp_events';
const DELIVERIES_TABLE = 'dp_event_deliveries';

/** Normalizes a dp_events row: `payload` JSON column back to a JS object. */
function parseEventRow(row) {
    if (!row) return row;
    return { ...row, payload: parseJsonColumn(row.payload) };
}

/** Normalizes a dp_event_deliveries row: `encrypted_fields` JSON column back to a JS object. */
function parseDeliveryRow(row) {
    if (!row) return row;
    return { ...row, encrypted_fields: parseJsonColumn(row.encrypted_fields) };
}

/**
 * Write an event row within the caller's transaction.
 * Returns the created event row.
 */
async function create({ eventType, orgId, aggregateType, aggregateId, payload }, transaction) {
    const exec = transaction || db;
    const uuid = crypto.randomUUID();
    const row = {
        uuid,
        type: eventType,
        org_uuid: orgId,
        aggregate_type: aggregateType,
        aggregate_uuid: aggregateId,
        payload: payload || {},
        occurred_at: new Date(),
        status: 'PENDING',
    };

    await exec.execute(
        `INSERT INTO ${EVENTS_TABLE} (uuid, type, org_uuid, aggregate_type, aggregate_uuid, payload, occurred_at, status)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        [row.uuid, row.type, row.org_uuid, row.aggregate_type, row.aggregate_uuid,
            JSON.stringify(row.payload), row.occurred_at, row.status]
    );

    // eventPublisher.js (a currently-live, not-yet-migrated caller) mutates the
    // `status` field on the object returned here and then calls `.save({ transaction })`
    // on it, relying on the old Sequelize instance's `.save()` method. Attach a minimal
    // compatible shim — persisting just `status`, the only field that call site ever
    // mutates — so that call site keeps working until eventPublisher.js is migrated to
    // call an explicit DAO update instead.
    row.save = async (opts) => {
        const saveExec = (opts && opts.transaction) || exec;
        await saveExec.execute(`UPDATE ${EVENTS_TABLE} SET status = ? WHERE uuid = ?`, [row.status, row.uuid]);
        return row;
    };

    return row;
}

/**
 * Write delivery rows for a set of subscribers, within the caller's transaction.
 * perSubscriberEncrypted: { [subscriberId]: { [fieldName]: encryptedEnvelope } }
 * The per-subscriber map is stored as-is in encrypted_fields and merged into
 * the webhook payload's `data` by the delivery worker.
 */
async function createDeliveries(eventId, subscribers, perSubscriberEncrypted, transaction) {
    const exec = transaction || db;
    const rows = subscribers.map((sub) => ({
        uuid: crypto.randomUUID(),
        event_uuid: eventId,
        subscriber_id: sub.id,
        target_url: sub.url,
        encrypted_fields: (perSubscriberEncrypted && perSubscriberEncrypted[sub.id]) || null,
        status: 'PENDING',
    }));

    for (const row of rows) {
        await exec.execute(
            `INSERT INTO ${DELIVERIES_TABLE} (uuid, event_uuid, subscriber_id, target_url, encrypted_fields, status)
             VALUES (?, ?, ?, ?, ?, ?)`,
            [
                row.uuid, row.event_uuid, row.subscriber_id, row.target_url,
                row.encrypted_fields !== null ? JSON.stringify(row.encrypted_fields) : null,
                row.status,
            ]
        );
    }
    return rows;
}

/**
 * Claim a batch of PENDING events using SELECT FOR UPDATE SKIP LOCKED.
 * Returns events with their delivery rows.
 */
async function claimPending(batchSize) {
    const isPostgres = db.getDialect() === 'postgres';
    return db.withTransaction(async (tx) => {
        const lockClause = isPostgres ? ' FOR UPDATE SKIP LOCKED' : '';
        const events = await tx.query(
            `SELECT * FROM ${EVENTS_TABLE} WHERE status = ? ORDER BY occurred_at ASC LIMIT ?${lockClause}`,
            ['PENDING', batchSize]
        );
        if (events.length === 0) return [];

        const ids = events.map((e) => e.uuid);
        const placeholders = ids.map(() => '?').join(', ');
        await tx.execute(
            `UPDATE ${EVENTS_TABLE} SET status = ? WHERE uuid IN (${placeholders})`,
            ['DISPATCHED', ...ids]
        );
        return events.map(parseEventRow);
    });
}

/**
 * Claim a batch of PENDING delivery rows using SELECT FOR UPDATE SKIP LOCKED.
 */
async function claimDueDeliveries(batchSize) {
    const isPostgres = db.getDialect() === 'postgres';
    return db.withTransaction(async (tx) => {
        // Recover stale IN_FLIGHT rows left by a crashed or stopped worker. Any delivery
        // that has been IN_FLIGHT for more than 5 minutes without a terminal update is
        // marked FAILED so it re-enters PENDING on the next dispatch cycle.
        const staleThreshold = new Date(Date.now() - 5 * 60 * 1000);
        await tx.execute(
            `UPDATE ${DELIVERIES_TABLE} SET status = ?, last_error = ? WHERE status = ? AND last_attempt_at < ?`,
            ['FAILED', 'Delivery abandoned: worker stopped mid-flight', 'IN_FLIGHT', staleThreshold]
        );

        const lockClause = isPostgres ? ' FOR UPDATE SKIP LOCKED' : '';
        const rows = await tx.query(
            `SELECT * FROM ${DELIVERIES_TABLE} WHERE status = ? LIMIT ?${lockClause}`,
            ['PENDING', batchSize]
        );
        if (rows.length === 0) return [];

        const ids = rows.map((r) => r.uuid);
        const placeholders = ids.map(() => '?').join(', ');
        await tx.execute(
            `UPDATE ${DELIVERIES_TABLE} SET status = ?, last_attempt_at = ? WHERE uuid IN (${placeholders})`,
            ['IN_FLIGHT', new Date(), ...ids]
        );
        return rows.map(parseDeliveryRow);
    });
}

/**
 * Mark a delivery as delivered.
 */
async function markDelivered(deliveryId, httpStatus) {
    await db.execute(
        `UPDATE ${DELIVERIES_TABLE} SET status = ?, last_http_status = ?, delivered_at = ? WHERE uuid = ?`,
        ['DELIVERED', httpStatus, new Date(), deliveryId]
    );
    const delivery = await db.queryOne(`SELECT * FROM ${DELIVERIES_TABLE} WHERE uuid = ?`, [deliveryId]);
    await reconcile(parseDeliveryRow(delivery));
}

/**
 * Mark a delivery as failed. Single attempt — no retry scheduling.
 */
async function markFailed(deliveryId, { httpStatus, error }) {
    await db.execute(
        `UPDATE ${DELIVERIES_TABLE} SET status = ?, last_http_status = ?, last_error = ? WHERE uuid = ?`,
        ['FAILED', httpStatus ?? null, error ? String(error).slice(0, 1000) : null, deliveryId]
    );
    const delivery = await db.queryOne(`SELECT * FROM ${DELIVERIES_TABLE} WHERE uuid = ?`, [deliveryId]);
    await reconcile(parseDeliveryRow(delivery));
}

/**
 * If all deliveries for an event are terminal (DELIVERED or FAILED),
 * update the event status accordingly.
 */
async function reconcile(delivery) {
    if (!delivery) return;
    const all = await db.query(`SELECT * FROM ${DELIVERIES_TABLE} WHERE event_uuid = ?`, [delivery.event_uuid]);
    if (all.length === 0) return;
    const terminal = all.every((d) => d.status === 'DELIVERED' || d.status === 'FAILED');
    if (!terminal) return;
    const allDelivered = all.every((d) => d.status === 'DELIVERED');
    await db.execute(
        `UPDATE ${EVENTS_TABLE} SET status = ? WHERE uuid = ?`,
        [allDelivered ? 'ALL_DELIVERED' : 'FAILED', delivery.event_uuid]
    );
}

/**
 * Admin: list recent events with delivery counts.
 */
async function list({ orgId, status, limit = 50, offset = 0 }) {
    const conditions = [];
    const params = [];
    if (orgId) {
        conditions.push('org_uuid = ?');
        params.push(orgId);
    }
    if (status) {
        conditions.push('status = ?');
        params.push(status);
    }
    const whereClause = conditions.length ? `WHERE ${conditions.join(' AND ')}` : '';

    const countRow = await db.queryOne(`SELECT COUNT(*) AS count FROM ${EVENTS_TABLE} ${whereClause}`, params);
    const count = countRow ? Number(countRow.count) : 0;

    const { clause, params: pageParams } = db.paginationClause(limit, offset);
    const events = await db.query(
        `SELECT * FROM ${EVENTS_TABLE} ${whereClause} ORDER BY occurred_at DESC ${clause}`,
        [...params, ...pageParams]
    );

    if (events.length === 0) return { count, rows: [] };

    const ids = events.map((e) => e.uuid);
    const placeholders = ids.map(() => '?').join(', ');
    const deliveries = await db.query(
        `SELECT * FROM ${DELIVERIES_TABLE} WHERE event_uuid IN (${placeholders})`,
        ids
    );
    const deliveriesByEvent = groupBy(deliveries, 'event_uuid');

    const rows = events.map((e) => parseEventRow({
        ...e,
        dp_event_deliveries: (deliveriesByEvent.get(e.uuid) || []).map(parseDeliveryRow),
    }));

    return { count, rows };
}

/**
 * Admin: get a single event with all delivery details.
 */
async function get(eventId) {
    const event = await db.queryOne(`SELECT * FROM ${EVENTS_TABLE} WHERE uuid = ?`, [eventId]);
    if (!event) return null;
    const deliveries = await db.query(`SELECT * FROM ${DELIVERIES_TABLE} WHERE event_uuid = ?`, [eventId]);
    return parseEventRow({ ...event, dp_event_deliveries: deliveries.map(parseDeliveryRow) });
}

/**
 * List the most recent delivery attempts for a single webhook subscriber,
 * newest event first. Used by the webhook subscriber's "recent deliveries" log.
 */
async function listDeliveriesForSubscriber(orgId, subscriberId, limit = 20) {
    const rows = await db.query(
        `SELECT d.*, e.type AS event_type, e.occurred_at AS event_occurred_at
         FROM ${DELIVERIES_TABLE} d
         INNER JOIN ${EVENTS_TABLE} e ON e.uuid = d.event_uuid
         WHERE d.subscriber_id = ? AND e.org_uuid = ?
         ORDER BY e.occurred_at DESC
         LIMIT ?`,
        [subscriberId, orgId, limit]
    );
    return rows.map(({ event_type, event_occurred_at, ...delivery }) => parseDeliveryRow({
        ...delivery,
        dp_event: { type: event_type, occurred_at: event_occurred_at },
    }));
}

module.exports = {
    create, createDeliveries,
    claimPending, claimDueDeliveries,
    markDelivered, markFailed,
    list, get, listDeliveriesForSubscriber,
    reconcile
};
