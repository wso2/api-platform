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
const { getPortalId } = require('../utils/orgContext');

const EVENTS_TABLE = 'events';
const DELIVERIES_TABLE = 'event_deliveries';

/** Normalizes a events row: `payload` JSON column back to a JS object. */
function parseEventRow(row) {
    if (!row) return row;
    return { ...row, payload: parseJsonColumn(row.payload) };
}

/** Normalizes a event_deliveries row: `encrypted_fields` JSON column back to a JS object. */
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
    const portalId = getPortalId();
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
        `INSERT INTO ${EVENTS_TABLE} (uuid, type, org_uuid, portal_id, aggregate_type, aggregate_uuid, payload, occurred_at, status)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [row.uuid, row.type, row.org_uuid, portalId, row.aggregate_type, row.aggregate_uuid,
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
        await saveExec.execute(`UPDATE ${EVENTS_TABLE} SET status = ? WHERE uuid = ? AND portal_id = ?`, [row.status, row.uuid, portalId]);
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
    const portalId = getPortalId();
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
            `INSERT INTO ${DELIVERIES_TABLE} (uuid, portal_id, event_uuid, subscriber_id, target_url, encrypted_fields, status)
             VALUES (?, ?, ?, ?, ?, ?, ?)`,
            [
                row.uuid, portalId, row.event_uuid, row.subscriber_id, row.target_url,
                row.encrypted_fields !== null ? JSON.stringify(row.encrypted_fields) : null,
                row.status,
            ]
        );
    }
    return rows;
}

/**
 * Write FAILED delivery rows, within the caller's transaction, for subscribers the event
 * can't be delivered to at all (their secret can't be decrypted). The rows keep the event
 * from reading as delivered to everyone, and show the reason in its delivery details.
 */
async function recordUndeliverable(eventId, subscribers, reason, transaction) {
    const exec = transaction || db;
    const portalId = getPortalId();
    for (const sub of subscribers) {
        await exec.execute(
            `INSERT INTO ${DELIVERIES_TABLE} (uuid, portal_id, event_uuid, subscriber_id, target_url, status, last_error)
             VALUES (?, ?, ?, ?, ?, ?, ?)`,
            [crypto.randomUUID(), portalId, eventId, sub.id, sub.url, 'FAILED', String(reason).slice(0, 255)]
        );
    }
}

/**
 * Row-lock hints that make a claim SELECT skip rows another transaction is already
 * claiming. Postgres takes them as a trailing clause (`FOR UPDATE SKIP LOCKED`);
 * MSSQL takes them as a table hint right after the table name. SQLite has neither
 * and needs neither: its adapter runs every transaction on one serialized connection.
 */
function claimLockHints() {
    const dialect = db.getDialect();
    return {
        tableHint: dialect === 'mssql' ? ' WITH (UPDLOCK, READPAST, ROWLOCK)' : '',
        trailing: dialect === 'postgres' ? ' FOR UPDATE SKIP LOCKED' : '',
    };
}

/**
 * Flips each selected row from PENDING to `status` one at a time, guarded on it
 * still being PENDING, and returns only the rows this call actually moved. The lock
 * hints above already keep two claimers apart on Postgres and MSSQL; this guard is
 * what guarantees it regardless of dialect or isolation level, so a row can never be
 * handed to two claimers and delivered twice.
 */
async function claimRows(tx, table, rows, setClause, setParams) {
    const claimed = [];
    for (const row of rows) {
        const { rowCount } = await tx.execute(
            `UPDATE ${table} SET ${setClause} WHERE uuid = ? AND portal_id = ? AND status = ?`,
            [...setParams, row.uuid, getPortalId(), 'PENDING']
        );
        if (rowCount === 1) claimed.push(row);
    }
    return claimed;
}

/**
 * Claim a batch of this organization's PENDING events using SELECT FOR UPDATE SKIP
 * LOCKED (or its MSSQL equivalent). Returns events with their delivery rows.
 *
 * Scoped to `orgUuid` because the events table is shared: every portal instance
 * pointed at this database runs its own dispatcher, and an unscoped claim would let
 * one instance pick up another organization's events and dispatch them from its own
 * egress. The SKIP LOCKED claim keeps that from double-dispatching, so it wouldn't
 * look broken — it would just be the wrong instance doing the work, outside whatever
 * network policy that organization's deployment has.
 */
// `orgUuid` null/undefined claims across every organization of this portal_id —
// multi-tenancy mode, where this instance delivers for all of them (see dispatcher.js).
async function claimPending(batchSize, orgUuid) {
    const { tableHint, trailing } = claimLockHints();
    const orgFilter = orgUuid ? ' AND org_uuid = ?' : '';
    return db.withTransaction(async (tx) => {
        const { clause, params: pageParams } = db.paginationClause(batchSize, 0);
        const events = await tx.query(
            `SELECT * FROM ${EVENTS_TABLE}${tableHint} WHERE status = ? AND portal_id = ?${orgFilter} ORDER BY occurred_at ASC ${clause}${trailing}`,
            ['PENDING', getPortalId(), ...(orgUuid ? [orgUuid] : []), ...pageParams]
        );
        if (events.length === 0) return [];

        const claimed = await claimRows(tx, EVENTS_TABLE, events, 'status = ?', ['DISPATCHED']);
        return claimed.map(parseEventRow);
    });
}

/**
 * Claim a batch of this organization's PENDING delivery rows using SELECT FOR UPDATE
 * SKIP LOCKED.
 *
 * Scoped to `orgUuid` for the same reason as claimPending — and the stale-row
 * recovery sweep below is scoped too: unscoped, every instance would keep resetting
 * every *other* instance's genuinely in-flight deliveries the moment they passed the
 * five-minute mark, turning slow deliveries into spurious failures.
 *
 * event_deliveries has no org_uuid of its own, so both statements reach the
 * organization through events. Postgres needs `FOR UPDATE OF d` here: with a join
 * in play, a bare FOR UPDATE would also try to lock the events rows.
 */
// `orgUuid` null/undefined claims, and sweeps stale rows, across every organization of
// this portal_id — multi-tenancy mode, where this deployment is the only one using that
// portal_id. The sweep is safe without an org filter there: its five-minute threshold is far past any subscriber
// timeout, so a row that old is abandoned, not in flight on some replica.
async function claimDueDeliveries(batchSize, orgUuid) {
    const isPostgres = db.getDialect() === 'postgres';
    const { tableHint } = claimLockHints();
    return db.withTransaction(async (tx) => {
        // Recover stale IN_FLIGHT rows left by a crashed or stopped worker. Any delivery
        // that has been IN_FLIGHT for more than 5 minutes without a terminal update is
        // marked FAILED so it re-enters PENDING on the next dispatch cycle.
        const staleThreshold = new Date(Date.now() - 5 * 60 * 1000);
        const orgParams = orgUuid ? [orgUuid] : [];
        await tx.execute(
            `UPDATE ${DELIVERIES_TABLE} SET status = ?, last_error = ?
             WHERE status = ? AND last_attempt_at < ? AND portal_id = ?${orgUuid
                ? ` AND event_uuid IN (SELECT uuid FROM ${EVENTS_TABLE} WHERE org_uuid = ? AND portal_id = ?)` : ''}`,
            ['FAILED', 'Delivery abandoned: worker stopped mid-flight', 'IN_FLIGHT', staleThreshold, getPortalId(),
                ...(orgUuid ? [orgUuid, getPortalId()] : [])]
        );

        const lockClause = isPostgres ? ' FOR UPDATE OF d SKIP LOCKED' : '';
        const { clause, params: pageParams } = db.paginationClause(batchSize, 0);
        const rows = await tx.query(
            `SELECT d.* FROM ${DELIVERIES_TABLE} d${tableHint}
             JOIN ${EVENTS_TABLE} e ON e.uuid = d.event_uuid AND d.portal_id = e.portal_id
             WHERE d.status = ? AND e.portal_id = ?${orgUuid ? ' AND e.org_uuid = ?' : ''} ORDER BY e.occurred_at ASC ${clause}${lockClause}`,
            ['PENDING', getPortalId(), ...orgParams, ...pageParams]
        );
        if (rows.length === 0) return [];

        const claimed = await claimRows(tx, DELIVERIES_TABLE, rows,
            'status = ?, last_attempt_at = ?', ['IN_FLIGHT', new Date()]);
        return claimed.map(parseDeliveryRow);
    });
}

/**
 * Mark a delivery as delivered.
 */
async function markDelivered(deliveryId, httpStatus) {
    await db.execute(
        `UPDATE ${DELIVERIES_TABLE} SET status = ?, last_http_status = ?, delivered_at = ? WHERE uuid = ? AND portal_id = ?`,
        ['DELIVERED', httpStatus, new Date(), deliveryId, getPortalId()]
    );
    const delivery = await db.queryOne(`SELECT * FROM ${DELIVERIES_TABLE} WHERE uuid = ? AND portal_id = ?`, [deliveryId, getPortalId()]);
    await reconcile(parseDeliveryRow(delivery));
}

/**
 * Mark a delivery as failed. Single attempt — no retry scheduling.
 */
async function markFailed(deliveryId, { httpStatus, error }) {
    await db.execute(
        `UPDATE ${DELIVERIES_TABLE} SET status = ?, last_http_status = ?, last_error = ? WHERE uuid = ? AND portal_id = ?`,
        ['FAILED', httpStatus ?? null, error ? String(error).slice(0, 1000) : null, deliveryId, getPortalId()]
    );
    const delivery = await db.queryOne(`SELECT * FROM ${DELIVERIES_TABLE} WHERE uuid = ? AND portal_id = ?`, [deliveryId, getPortalId()]);
    await reconcile(parseDeliveryRow(delivery));
}

/**
 * If all deliveries for an event are terminal (DELIVERED or FAILED),
 * update the event status accordingly.
 */
async function reconcile(delivery) {
    if (!delivery) return;
    const all = await db.query(`SELECT * FROM ${DELIVERIES_TABLE} WHERE event_uuid = ? AND portal_id = ?`,
        [delivery.event_uuid, getPortalId()]);
    if (all.length === 0) return;
    const terminal = all.every((d) => d.status === 'DELIVERED' || d.status === 'FAILED');
    if (!terminal) return;
    const allDelivered = all.every((d) => d.status === 'DELIVERED');
    await db.execute(
        `UPDATE ${EVENTS_TABLE} SET status = ? WHERE uuid = ? AND portal_id = ?`,
        [allDelivered ? 'ALL_DELIVERED' : 'FAILED', delivery.event_uuid, getPortalId()]
    );
}

/**
 * Admin: list recent events with delivery counts.
 */
async function list({ orgId, status, limit = 50, offset = 0 }) {
    const conditions = ['portal_id = ?'];
    const params = [getPortalId()];
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
        `SELECT * FROM ${DELIVERIES_TABLE} WHERE event_uuid IN (${placeholders}) AND portal_id = ?`,
        [...ids, getPortalId()]
    );
    const deliveriesByEvent = groupBy(deliveries, 'event_uuid');

    const rows = events.map((e) => parseEventRow({
        ...e,
        event_deliveries: (deliveriesByEvent.get(e.uuid) || []).map(parseDeliveryRow),
    }));

    return { count, rows };
}

/**
 * Admin: get a single event with all delivery details.
 */
async function get(eventId) {
    const event = await db.queryOne(`SELECT * FROM ${EVENTS_TABLE} WHERE uuid = ? AND portal_id = ?`, [eventId, getPortalId()]);
    if (!event) return null;
    const deliveries = await db.query(`SELECT * FROM ${DELIVERIES_TABLE} WHERE event_uuid = ? AND portal_id = ?`,
        [eventId, getPortalId()]);
    return parseEventRow({ ...event, event_deliveries: deliveries.map(parseDeliveryRow) });
}

/**
 * List the most recent delivery attempts for a single webhook subscriber,
 * newest event first. Used by the webhook subscriber's "recent deliveries" log.
 */
async function listDeliveriesForSubscriber(orgId, subscriberId, limit = 20) {
    const { clause, params: pageParams } = db.paginationClause(limit, 0);
    const rows = await db.query(
        `SELECT d.*, e.type AS event_type, e.occurred_at AS event_occurred_at
         FROM ${DELIVERIES_TABLE} d
         INNER JOIN ${EVENTS_TABLE} e ON e.uuid = d.event_uuid AND d.portal_id = e.portal_id
         WHERE d.subscriber_id = ? AND e.org_uuid = ? AND e.portal_id = ?
         ORDER BY e.occurred_at DESC
         ${clause}`,
        [subscriberId, orgId, getPortalId(), ...pageParams]
    );
    return rows.map(({ event_type, event_occurred_at, ...delivery }) => parseDeliveryRow({
        ...delivery,
        event: { type: event_type, occurred_at: event_occurred_at },
    }));
}

module.exports = {
    create, createDeliveries, recordUndeliverable,
    claimPending, claimDueDeliveries,
    markDelivered, markFailed,
    list, get, listDeliveriesForSubscriber,
    reconcile
};
