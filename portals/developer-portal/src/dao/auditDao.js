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

const INSERT_AUDIT_SQL = `
    INSERT INTO dp_audit (uuid, action, resource_uuid, resource_type, org_uuid, performed_by)
    VALUES (?, ?, ?, ?, ?, ?)
`;

/**
 * Records one audit trail entry. Write-only — no list/query function, mirroring
 * platform-api's audit table, which is inspected directly in the database.
 *
 * Called fire-and-forget (see auditLogger.js), so a failure here is caught and
 * logged by the caller and never blocks the request. Not passed a transaction —
 * for SQLite this autocommit write still serializes correctly behind any open
 * transaction (src/db/adapters/sqliteAdapter.js queues all connection access
 * behind a lock while a transaction is open), rather than contending with it.
 */
const record = async (action, resourceUuid, resourceType, orgUuid, performedBy) => {
    const uuid = crypto.randomUUID();
    await db.execute(INSERT_AUDIT_SQL, [uuid, action, resourceUuid, resourceType, orgUuid, performedBy]);
    return {
        uuid,
        action,
        resource_uuid: resourceUuid,
        resource_type: resourceType,
        org_uuid: orgUuid,
        performed_by: performedBy,
    };
};

module.exports = {
    record,
};
