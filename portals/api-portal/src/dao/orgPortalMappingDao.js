/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
'use strict';

const crypto = require('crypto');
const db = require('../db/driver');

const TABLE = 'org_portal_mapping';

/**
 * Creates a mapping between an org and a portal. Idempotent when the caller
 * wraps duplicate-key errors — the UNIQUE (org_uuid, portal_id) constraint
 * enforces within-org uniqueness at the DB level.
 */
const create = async (orgUuid, portalId, t) => {
    const exec = t || db;
    const uuid = crypto.randomUUID();
    await exec.execute(
        `INSERT INTO ${TABLE} (uuid, org_uuid, portal_id) VALUES (?, ?, ?)`,
        [uuid, orgUuid, portalId]
    );
};

/**
 * Returns the mapping row for the given portal_id, or null if none exists.
 * Used by the seeder to check whether this portal is already mapped to an org.
 */
const getByPortalId = async (portalId, t) => {
    const exec = t || db;
    return exec.queryOne(
        `SELECT * FROM ${TABLE} WHERE portal_id = ?`,
        [portalId]
    );
};

/**
 * Lists all portals mapped to the given org.
 */
const listByOrg = async (orgUuid, t) => {
    const exec = t || db;
    return exec.query(
        `SELECT * FROM ${TABLE} WHERE org_uuid = ?`,
        [orgUuid]
    );
};

module.exports = { create, getByPortalId, listByOrg };
