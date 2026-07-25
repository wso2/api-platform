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
const constants = require('../utils/constants');
const logger = require('../config/logger');
const { bufferToUtf8 } = require('../utils/cryptoUtil');

const TABLE = 'dp_api_workflows';

/**
 * The Sequelize model exposed `agent_prompt` (a BLOB column) through a getter
 * that decoded it back to a UTF-8 string. Raw SQL rows bypass that getter, so
 * every row read from this table is passed through here before being handed
 * back to callers.
 */
function mapRow(row) {
    if (!row) return row;
    return { ...row, agent_prompt: bufferToUtf8(row.agent_prompt) };
}

const create = async (orgId, viewId, apiWorkflowData, createdBy, t) => {
    const exec = t || db;
    const uuid = crypto.randomUUID();
    const now = new Date();
    const status = apiWorkflowData.status || constants.API_WORKFLOW_STATUS.PUBLISHED;
    const agentVisibility = apiWorkflowData.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE;
    const contentType = apiWorkflowData.contentType || constants.API_WORKFLOW_CONTENT_TYPE.ARAZZO;
    const fileContent = apiWorkflowData.apiWorkflowDefinition != null ? Buffer.from(apiWorkflowData.apiWorkflowDefinition) : null;

    try {
        await exec.execute(
            `INSERT INTO ${TABLE}
                (uuid, org_uuid, view_uuid, display_name, handle, description, agent_prompt, status,
                 agent_visibility, file_content, content_type, created_by, updated_by, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            [uuid, orgId, viewId, apiWorkflowData.displayName, apiWorkflowData.handle, apiWorkflowData.description,
                Buffer.from(apiWorkflowData.agentPrompt), status, agentVisibility, fileContent, contentType,
                createdBy, createdBy, now, now]
        );
    } catch (error) {
        // Let the raw driver error propagate unchanged — callers classify it with
        // db.isDuplicateKeyError(error), which inspects driver-specific fields directly.
        if (!db.isDuplicateKeyError(error)) {
            logger.error('Error creating API Workflow', { error: error.message });
        }
        throw error;
    }

    return {
        uuid,
        org_uuid: orgId,
        view_uuid: viewId,
        display_name: apiWorkflowData.displayName,
        handle: apiWorkflowData.handle,
        description: apiWorkflowData.description,
        agent_prompt: apiWorkflowData.agentPrompt,
        status,
        agent_visibility: agentVisibility,
        file_content: fileContent,
        content_type: contentType,
        created_by: createdBy,
        updated_by: createdBy,
        created_at: now,
        updated_at: now,
    };
};

const update = async (orgId, viewId, apiWorkflowId, apiWorkflowData, updatedBy, t) => {
    const exec = t || db;
    const now = new Date();
    const setClauses = ['updated_at = ?', 'updated_by = ?'];
    const params = [now, updatedBy];
    if (apiWorkflowData.displayName !== undefined) { setClauses.push('display_name = ?'); params.push(apiWorkflowData.displayName); }
    if (apiWorkflowData.handle !== undefined) { setClauses.push('handle = ?'); params.push(apiWorkflowData.handle); }
    if (apiWorkflowData.description !== undefined) { setClauses.push('description = ?'); params.push(apiWorkflowData.description); }
    if (apiWorkflowData.agentPrompt !== undefined) { setClauses.push('agent_prompt = ?'); params.push(Buffer.from(apiWorkflowData.agentPrompt)); }
    if (apiWorkflowData.status !== undefined) { setClauses.push('status = ?'); params.push(apiWorkflowData.status); }
    if (apiWorkflowData.agentVisibility !== undefined) { setClauses.push('agent_visibility = ?'); params.push(apiWorkflowData.agentVisibility); }
    if (apiWorkflowData.apiWorkflowDefinition !== undefined) {
        setClauses.push('file_content = ?');
        params.push(apiWorkflowData.apiWorkflowDefinition != null ? Buffer.from(apiWorkflowData.apiWorkflowDefinition) : null);
    }
    if (apiWorkflowData.contentType !== undefined) { setClauses.push('content_type = ?'); params.push(apiWorkflowData.contentType); }
    params.push(apiWorkflowId, orgId, viewId);

    const { rowCount } = await exec.execute(
        `UPDATE ${TABLE} SET ${setClauses.join(', ')} WHERE uuid = ? AND org_uuid = ? AND view_uuid = ?`,
        params
    );
    if (rowCount === 0) {
        return [0, []];
    }
    const updated = await exec.queryOne(`SELECT * FROM ${TABLE} WHERE uuid = ?`, [apiWorkflowId]);
    return [rowCount, [mapRow(updated)]];
};

const deleteFlow = async (orgId, viewId, apiWorkflowId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${TABLE} WHERE uuid = ? AND org_uuid = ? AND view_uuid = ?`,
        [apiWorkflowId, orgId, viewId]
    );
    return rowCount;
};

const getByHandle = async (orgId, viewId, handle) => {
    const row = await db.queryOne(
        `SELECT * FROM ${TABLE} WHERE handle = ? AND org_uuid = ? AND view_uuid = ?`,
        [handle, orgId, viewId]
    );
    return mapRow(row);
};

const list = async (orgId, viewId) => {
    const rows = await db.query(
        `SELECT * FROM ${TABLE} WHERE org_uuid = ? AND view_uuid = ? ORDER BY created_at DESC`,
        [orgId, viewId]
    );
    return rows.map(mapRow);
};

const listPublished = async (orgId, viewId, { agentVisibility } = {}) => {
    const conditions = ['org_uuid = ?', 'view_uuid = ?', "status = 'PUBLISHED'"];
    const params = [orgId, viewId];
    if (agentVisibility) { conditions.push('agent_visibility = ?'); params.push(agentVisibility); }
    const rows = await db.query(
        `SELECT * FROM ${TABLE} WHERE ${conditions.join(' AND ')} ORDER BY created_at DESC`,
        params
    );
    return rows.map(mapRow);
};

const getPublishedByHandle = async (orgId, viewId, handle, { agentVisibility } = {}) => {
    const conditions = ['handle = ?', 'org_uuid = ?', 'view_uuid = ?', "status = 'PUBLISHED'"];
    const params = [handle, orgId, viewId];
    if (agentVisibility) { conditions.push('agent_visibility = ?'); params.push(agentVisibility); }
    const row = await db.queryOne(`SELECT * FROM ${TABLE} WHERE ${conditions.join(' AND ')}`, params);
    return mapRow(row);
};

module.exports = {
    create,
    update,
    delete: deleteFlow,
    getByHandle,
    list,
    listPublished,
    getPublishedByHandle
};
