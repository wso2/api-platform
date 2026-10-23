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
const { groupBy } = require('../db/rows');
const { bufferToUtf8 } = require('../utils/cryptoUtil');
const { getPortalId } = require('../utils/orgContext');

const TABLE = 'meters';
const PRICES_TABLE = 'subscription_plan_pricing_usage';
const PLANS_TABLE = 'subscription_plans';

const COLUMNS = 'uuid, org_uuid, handle, display_name, description, type, type_key, filter_fields, ' +
    'created_by, created_at, updated_by, updated_at';

/** A JSON array stored as binary; unreadable JSON reads as empty. */
function parseArray(value) {
    try {
        const parsed = JSON.parse(bufferToUtf8(value) || '[]');
        return Array.isArray(parsed) ? parsed : [];
    } catch (_err) {
        return [];
    }
}

// The fields a meter's prices may filter on, with the event key each reads, stored as a JSON array
// of { field, eventKey }.
function filterFieldsParam(fields) {
    return db.binaryParam(Buffer.from(JSON.stringify(fields || []), 'utf8'));
}

function toMeter(row) {
    if (!row) return row;
    return { ...row, type_key: row.type_key || null, filter_fields: parseArray(row.filter_fields) };
}

const list = async (orgId, t) => {
    const exec = t || db;
    const rows = await exec.query(
        `SELECT ${COLUMNS} FROM ${TABLE} WHERE org_uuid = ? AND portal_id = ? ORDER BY handle`,
        [orgId, getPortalId()]
    );
    return rows.map(toMeter);
};

const getByHandle = async (orgId, handle, t) => {
    const exec = t || db;
    const row = await exec.queryOne(
        `SELECT ${COLUMNS} FROM ${TABLE} WHERE handle = ? AND org_uuid = ? AND portal_id = ?`,
        [handle, orgId, getPortalId()]
    );
    return toMeter(row) || null;
};

/** Meters of the organization with the given handles; handles that do not exist are absent. */
const listByHandles = async (orgId, handles, t) => {
    if (!handles || handles.length === 0) return [];
    const exec = t || db;
    const placeholders = handles.map(() => '?').join(', ');
    const rows = await exec.query(
        `SELECT ${COLUMNS} FROM ${TABLE}
         WHERE handle IN (${placeholders}) AND org_uuid = ? AND portal_id = ?`,
        [...handles, orgId, getPortalId()]
    );
    return rows.map(toMeter);
};

const create = async (orgId, meter, createdBy, t) => {
    const exec = t || db;
    const uuid = crypto.randomUUID();
    const now = new Date();
    await exec.execute(
        `INSERT INTO ${TABLE}
           (uuid, org_uuid, portal_id, handle, display_name, description, type, type_key, filter_fields,
            created_by, created_at, updated_by, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [uuid, orgId, getPortalId(), meter.handle, meter.displayName, meter.description ?? null,
            meter.type, meter.typeKey ?? null, filterFieldsParam(meter.filterFields), createdBy, now,
            createdBy, now]
    );
    return getByHandle(orgId, meter.handle, t);
};

/** Replaces the editable fields of a meter. The handle never changes. */
const update = async (orgId, handle, meter, updatedBy, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `UPDATE ${TABLE}
         SET display_name = ?, description = ?, type = ?, type_key = ?, filter_fields = ?, updated_by = ?,
             updated_at = ?
         WHERE handle = ? AND org_uuid = ? AND portal_id = ?`,
        [meter.displayName, meter.description ?? null, meter.type, meter.typeKey ?? null,
            filterFieldsParam(meter.filterFields), updatedBy, new Date(), handle, orgId, getPortalId()]
    );
    if (rowCount < 1) return null;
    return getByHandle(orgId, handle, t);
};

const deleteMeter = async (orgId, handle, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${TABLE} WHERE handle = ? AND org_uuid = ? AND portal_id = ?`,
        [handle, orgId, getPortalId()]
    );
    return rowCount;
};

/**
 * The handles of the plans with a price that charges for each meter, as a Map of meter uuid to
 * plan handles. A meter no plan uses is absent from the map.
 */
const listPlanHandlesByMeter = async (orgId, meterIds, t) => {
    if (!meterIds || meterIds.length === 0) return new Map();
    const exec = t || db;
    const placeholders = meterIds.map(() => '?').join(', ');
    const rows = await exec.query(
        `SELECT DISTINCT c.meter_uuid, p.handle FROM ${PRICES_TABLE} c
         JOIN ${PLANS_TABLE} p ON p.uuid = c.subscription_plan_uuid AND p.portal_id = c.portal_id
         WHERE c.meter_uuid IN (${placeholders}) AND c.org_uuid = ? AND c.portal_id = ?
         ORDER BY p.handle`,
        [...meterIds, orgId, getPortalId()]
    );
    const grouped = groupBy(rows, 'meter_uuid');
    const result = new Map();
    for (const [meterId, planRows] of grouped) {
        result.set(meterId, planRows.map((r) => r.handle));
    }
    return result;
};

module.exports = {
    list,
    getByHandle,
    listByHandles,
    create,
    update,
    delete: deleteMeter,
    listPlanHandlesByMeter,
};
