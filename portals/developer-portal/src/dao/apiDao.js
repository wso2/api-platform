/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
const constants = require('../utils/constants');
const logger = require('../config/logger');

const API_METADATA_TABLE = 'dp_api_metadata';
const CONTENT_TABLE = 'dp_api_contents';
const LABELS_TABLE = 'dp_labels';
const TAGS_TABLE = 'dp_tags';
const API_LABEL_MAPPINGS_TABLE = 'dp_api_label_mappings';
const API_TAG_MAPPINGS_TABLE = 'dp_api_tag_mappings';
const SUBSCRIPTION_PLANS_TABLE = 'dp_subscription_plans';
const SUBSCRIPTION_PLAN_LIMITS_TABLE = 'dp_subscription_plan_limits';
const API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE = 'dp_api_subscription_plan_mappings';
const VIEW_LABEL_MAPPINGS_TABLE = 'dp_view_label_mappings';

const PUBLISHED_STATUSES = [constants.API_STATUS.PUBLISHED, constants.API_STATUS.DEPRECATED];
const STATUS_PLACEHOLDERS = PUBLISHED_STATUSES.map(() => '?').join(', ');

/**
 * Full-text search query for dp_api_metadata (PostgreSQL only). Used by search()
 * below via db.bindNamedParams(). Named parameters:
 *   :searchTerm   — the user-supplied search string
 *   :orgId        — the organisation UUID to scope results to
 *   :viewId       — nullable; the view UUID to scope results to (API must have a label mapped
 *                   to this view). `view` is an optional query param — when omitted, results
 *                   are unscoped by view rather than matching nothing.
 *   :includeType  — nullable; when set, only rows with metadata.type = :includeType match
 *   :excludeType  — nullable; when set, rows with metadata.type = :excludeType are excluded
 *                   (keeps /apis and /mcp-servers list results type-scoped at the SQL level
 *                   rather than relying on callers to filter in application code)
 *
 * Other dialects use the LIKE-based fallback in searchFallback() below.
 *
 * Associations (contents/labels/tags/subscription plans + limits) are deliberately
 * NOT aggregated here — none of them feed the WHERE clause, and JSONB_AGG'ing
 * dp_api_subscription_plan_mappings would only yield mapping-table columns
 * (api_uuid, plan_uuid), not the actual plan data (handle, display_name, limits)
 * that APIDTO/APISubscriptionPlan need. search() below runs these rows through
 * the same attachAssociations() every other list method uses instead, per the
 * project's app-side-stitching convention (no JSON aggregation).
 */
const SEARCH_APIS_POSTGRES_SQL = `
    SELECT
        metadata.*,
        ts_rank(
            to_tsvector('english', metadata.metadata_search::text),
            plainto_tsquery('english', COALESCE(:searchTerm, ''))
        ) AS rank_metadata,
        STRING_AGG(
            DISTINCT CASE
                WHEN content.file_content IS NOT NULL
                AND to_tsvector('english', convert_from(content.file_content, 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
                THEN content.type
                ELSE 'METADATA'
            END, ', '
        ) AS "DATA_SOURCE"
    FROM
        dp_api_metadata metadata
    LEFT JOIN
        dp_api_contents content
        ON metadata.uuid = content.api_uuid
        AND (
            content.file_name LIKE '%.hbs'
            OR content.file_name LIKE '%.md%'
            OR content.file_name LIKE '%.json%'
            OR content.file_name LIKE '%.xml%'
            OR content.file_name LIKE '%.graphql%'
        )
    WHERE
        (
            to_tsvector('english', metadata.metadata_search::text) @@ plainto_tsquery('english', COALESCE(:searchTerm, ''))
            OR (
                content.file_content IS NOT NULL AND
                to_tsvector('english', convert_from(content.file_content, 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
            )
        )
        AND metadata.org_uuid = :orgId
        AND (:includeType::text IS NULL OR metadata.type = :includeType)
        AND (:excludeType::text IS NULL OR metadata.type != :excludeType)
        AND (
            :viewId::uuid IS NULL
            OR EXISTS (
                SELECT 1
                FROM dp_api_label_mappings alm
                JOIN dp_view_label_mappings vlm ON alm.label_uuid = vlm.label_uuid
                WHERE alm.api_uuid = metadata.uuid AND vlm.view_uuid = :viewId
            )
        )
    GROUP BY
        metadata.uuid
    ORDER BY
        rank_metadata DESC;
`;

/**
 * App-side "eager load" mirroring the previous Sequelize `include:` shape on
 * APIMetadata — attaches, on each row, the same property names/shapes the old
 * associations produced (see src/dto/apiDto.js and src/dto/subscriptionPlanDto.js):
 *   dp_api_contents       — content rows of type IMAGES
 *   dp_labels             — [{handle}]
 *   dp_tags               — [{name}]
 *   dp_subscription_plans — plan rows with `.limits` attached
 */
async function attachAssociations(apiRows, t) {
    const exec = t || db;
    if (apiRows.length === 0) return apiRows;
    const apiIds = apiRows.map((r) => r.uuid);
    const placeholders = apiIds.map(() => '?').join(', ');

    const contents = await exec.query(
        `SELECT * FROM ${CONTENT_TABLE} WHERE api_uuid IN (${placeholders}) AND type = ?`,
        [...apiIds, constants.DOC_TYPES.IMAGES]
    );
    const contentsByApi = groupBy(contents, 'api_uuid');

    const labelRows = await exec.query(
        `SELECT alm.api_uuid AS api_uuid, l.handle AS handle
         FROM ${API_LABEL_MAPPINGS_TABLE} alm JOIN ${LABELS_TABLE} l ON alm.label_uuid = l.uuid
         WHERE alm.api_uuid IN (${placeholders})`,
        apiIds
    );
    const labelsByApi = groupBy(labelRows, 'api_uuid');

    const tagRows = await exec.query(
        `SELECT atm.api_uuid AS api_uuid, tg.name AS name
         FROM ${API_TAG_MAPPINGS_TABLE} atm JOIN ${TAGS_TABLE} tg ON atm.tag_uuid = tg.uuid
         WHERE atm.api_uuid IN (${placeholders})`,
        apiIds
    );
    const tagsByApi = groupBy(tagRows, 'api_uuid');

    const planMappingRows = await exec.query(
        `SELECT m.api_uuid AS mapping_api_uuid, sp.*
         FROM ${API_SUBSCRIPTION_PLAN_MAPPINGS_TABLE} m JOIN ${SUBSCRIPTION_PLANS_TABLE} sp ON m.plan_uuid = sp.uuid
         WHERE m.api_uuid IN (${placeholders})`,
        apiIds
    );
    const planIds = [...new Set(planMappingRows.map((p) => p.uuid))];
    let limitsByPlan = new Map();
    if (planIds.length > 0) {
        const planPlaceholders = planIds.map(() => '?').join(', ');
        const limitRows = await exec.query(
            `SELECT * FROM ${SUBSCRIPTION_PLAN_LIMITS_TABLE} WHERE plan_uuid IN (${planPlaceholders})`,
            planIds
        );
        limitsByPlan = groupBy(limitRows, 'plan_uuid');
    }
    for (const plan of planMappingRows) {
        plan.limits = limitsByPlan.get(plan.uuid) || [];
    }
    const plansByApi = groupBy(planMappingRows, 'mapping_api_uuid');

    for (const api of apiRows) {
        api.dp_api_contents = contentsByApi.get(api.uuid) || [];
        api.dp_labels = labelsByApi.get(api.uuid) || [];
        api.dp_tags = tagsByApi.get(api.uuid) || [];
        api.dp_subscription_plans = (plansByApi.get(api.uuid) || []).map(({ mapping_api_uuid, ...rest }) => rest);
    }
    return apiRows;
}

const create = async (orgId, apiMetadata, createdBy, t) => {
    const exec = t || db;
    const owners = apiMetadata.owners || {};
    const uuid = crypto.randomUUID();
    const now = new Date();
    const handle = apiMetadata.handle || `${apiMetadata.name.toLowerCase().replace(/\s+/g, '')}-v${apiMetadata.version}`;
    const agentVisibility = (apiMetadata.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE).toUpperCase();

    await exec.execute(
        `INSERT INTO ${API_METADATA_TABLE}
            (uuid, ref_id, status, name, handle, description, version, type, agent_visibility,
             technical_owner, technical_owner_email, business_owner_email, business_owner,
             sandbox_url, production_url, metadata_search, org_uuid, created_by, updated_by, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        [
            uuid, apiMetadata.referenceId, apiMetadata.status, apiMetadata.name, handle, apiMetadata.description,
            apiMetadata.version, apiMetadata.type, agentVisibility, owners.technicalOwner, owners.technicalOwnerEmail,
            owners.businessOwnerEmail, owners.businessOwner, apiMetadata.endPoints.sandboxURL,
            apiMetadata.endPoints.productionURL, apiMetadata, orgId, createdBy, createdBy, now, now,
        ]
    );
    return {
        uuid, ref_id: apiMetadata.referenceId, status: apiMetadata.status, name: apiMetadata.name, handle,
        description: apiMetadata.description, version: apiMetadata.version, type: apiMetadata.type,
        agent_visibility: agentVisibility, technical_owner: owners.technicalOwner,
        technical_owner_email: owners.technicalOwnerEmail, business_owner_email: owners.businessOwnerEmail,
        business_owner: owners.businessOwner, sandbox_url: apiMetadata.endPoints.sandboxURL,
        production_url: apiMetadata.endPoints.productionURL, metadata_search: apiMetadata, org_uuid: orgId,
        created_by: createdBy, updated_by: createdBy, created_at: now, updated_at: now,
    };
};

const update = async (orgId, apiId, apiMetadata, updatedBy, t) => {
    const exec = t || db;
    const owners = apiMetadata.owners || {};
    const agentVisibility = (apiMetadata.agentVisibility || constants.AGENT_VISIBILITY.VISIBLE).toUpperCase();
    const updatedAt = new Date();

    const { rowCount } = await exec.execute(
        `UPDATE ${API_METADATA_TABLE}
         SET ref_id = ?, status = ?, name = ?, description = ?, version = ?, type = ?, agent_visibility = ?,
             technical_owner = ?, technical_owner_email = ?, business_owner_email = ?, business_owner = ?,
             sandbox_url = ?, production_url = ?, metadata_search = ?, updated_by = ?, updated_at = ?
         WHERE uuid = ? AND org_uuid = ?`,
        [
            apiMetadata.referenceId, apiMetadata.status, apiMetadata.name, apiMetadata.description,
            apiMetadata.version, apiMetadata.type, agentVisibility, owners.technicalOwner,
            owners.technicalOwnerEmail, owners.businessOwnerEmail, owners.businessOwner,
            apiMetadata.endPoints.sandboxURL, apiMetadata.endPoints.productionURL, apiMetadata,
            updatedBy, updatedAt, apiId, orgId,
        ]
    );
    if (!rowCount) {
        return [0, null];
    }
    const updatedInstance = await exec.queryOne(
        `SELECT * FROM ${API_METADATA_TABLE} WHERE uuid = ? AND org_uuid = ?`,
        [apiId, orgId]
    );
    return [rowCount, [updatedInstance]];
};

const deleteApi = async (orgId, apiId, t) => {
    const exec = t || db;
    const { rowCount } = await exec.execute(
        `DELETE FROM ${API_METADATA_TABLE} WHERE uuid = ? AND org_uuid = ?`,
        [apiId, orgId]
    );
    return rowCount;
};

const get = async (orgId, apiId, t) => {
    const exec = t || db;
    const rows = await exec.query(
        `SELECT * FROM ${API_METADATA_TABLE} WHERE org_uuid = ? AND uuid = ? AND status IN (${STATUS_PLACEHOLDERS})`,
        [orgId, apiId, ...PUBLISHED_STATUSES]
    );
    await attachAssociations(rows, t);
    return rows;
};

/**
 * Filtered lookup, replacing the previous arbitrary Sequelize `where` passthrough
 * (which relied on `Sequelize.Op.*` — no longer available). Callers now pass a
 * small structured filter instead of a raw ORM condition object:
 *   { orgId, uuid, typeFilter: { include, exclude } }
 * `tags`, when given, is the same comma-separated tag-name string `list()`/
 * `search()` accept; when present, only APIs having at least one matching tag
 * are returned (mirrors the previous `required: true` tags include).
 */
const getByCondition = async ({ orgId, uuid, typeFilter } = {}, t, tags) => {
    const exec = t || db;
    const conditions = [];
    const params = [];
    if (orgId !== undefined) { conditions.push('org_uuid = ?'); params.push(orgId); }
    if (uuid !== undefined) { conditions.push('uuid = ?'); params.push(uuid); }
    if (typeFilter?.include) { conditions.push('type = ?'); params.push(typeFilter.include); }
    if (typeFilter?.exclude) { conditions.push('type != ?'); params.push(typeFilter.exclude); }

    const tagsArray = tags ? tags.split(',').map((tag) => tag.trim()).filter(Boolean) : [];
    if (tagsArray.length > 0) {
        const tagPlaceholders = tagsArray.map(() => '?').join(', ');
        conditions.push(
            `EXISTS (SELECT 1 FROM ${API_TAG_MAPPINGS_TABLE} atm JOIN ${TAGS_TABLE} tg ON atm.tag_uuid = tg.uuid
                     WHERE atm.api_uuid = ${API_METADATA_TABLE}.uuid AND tg.name IN (${tagPlaceholders}))`
        );
        params.push(...tagsArray);
    }

    const whereSql = conditions.length ? `WHERE ${conditions.join(' AND ')}` : '';
    const rows = await exec.query(`SELECT * FROM ${API_METADATA_TABLE} ${whereSql}`, params);
    await attachAssociations(rows, t);
    return rows;
};

const list = async (orgId, viewName, t, typeFilter) => {
    const exec = t || db;
    const viewDao = require('./viewDao');
    const viewId = await viewDao.getId(orgId, viewName, t);

    const conditions = ['org_uuid = ?', `status IN (${STATUS_PLACEHOLDERS})`];
    const params = [orgId, ...PUBLISHED_STATUSES];
    if (typeFilter?.include) { conditions.push('type = ?'); params.push(typeFilter.include); }
    if (typeFilter?.exclude) { conditions.push('type != ?'); params.push(typeFilter.exclude); }
    // Required label-in-view filter — mirrors the previous `required: true` Labels include
    // scoped to this view's mapped labels; an API with no matching label is excluded entirely.
    conditions.push(
        `EXISTS (SELECT 1 FROM ${API_LABEL_MAPPINGS_TABLE} alm
                 WHERE alm.api_uuid = ${API_METADATA_TABLE}.uuid
                 AND alm.label_uuid IN (SELECT label_uuid FROM ${VIEW_LABEL_MAPPINGS_TABLE} WHERE view_uuid = ?))`
    );
    params.push(viewId);

    const rows = await exec.query(`SELECT * FROM ${API_METADATA_TABLE} WHERE ${conditions.join(' AND ')}`, params);
    await attachAssociations(rows, t);
    return rows;
};

const listFromAllViews = async (orgId, t, typeFilter) => {
    const exec = t || db;
    const conditions = ['org_uuid = ?', `status IN (${STATUS_PLACEHOLDERS})`];
    const params = [orgId, ...PUBLISHED_STATUSES];
    if (typeFilter?.include) { conditions.push('type = ?'); params.push(typeFilter.include); }
    if (typeFilter?.exclude) { conditions.push('type != ?'); params.push(typeFilter.exclude); }
    // Required label filter — mirrors the previous `required: true` Labels include with no
    // where clause: only APIs that have at least one label at all are included.
    conditions.push(`EXISTS (SELECT 1 FROM ${API_LABEL_MAPPINGS_TABLE} alm WHERE alm.api_uuid = ${API_METADATA_TABLE}.uuid)`);

    const rows = await exec.query(`SELECT * FROM ${API_METADATA_TABLE} WHERE ${conditions.join(' AND ')}`, params);
    await attachAssociations(rows, t);
    return rows;
};

const searchFallback = async (orgId, searchTerm, viewName, t, typeFilter) => {
    const exec = t || db;
    const viewDao = require('./viewDao');
    const pattern = `%${searchTerm}%`;
    // `view` is optional (apiViewQuery in the OpenAPI spec) — search unscoped by view
    // when it's omitted, rather than forcing a view-label join that would otherwise
    // either crash (viewId undefined) or silently match nothing (a literal 'undefined').
    const viewId = await viewDao.getId(orgId, viewName, t);

    const matchingTags = await exec.query(
        `SELECT uuid FROM ${TAGS_TABLE} WHERE org_uuid = ? AND name LIKE ?`,
        [orgId, pattern]
    );
    const matchingTagIds = matchingTags.map((tag) => tag.uuid);
    let taggedApiIds = [];
    if (matchingTagIds.length > 0) {
        const tagPlaceholders = matchingTagIds.map(() => '?').join(', ');
        const matchingTagApis = await exec.query(
            `SELECT api_uuid FROM ${API_TAG_MAPPINGS_TABLE} WHERE tag_uuid IN (${tagPlaceholders})`,
            matchingTagIds
        );
        taggedApiIds = [...new Set(matchingTagApis.map((row) => row.api_uuid))];
    }

    const conditions = ['org_uuid = ?', `status IN (${STATUS_PLACEHOLDERS})`];
    const params = [orgId, ...PUBLISHED_STATUSES];
    if (typeFilter?.include) { conditions.push('type = ?'); params.push(typeFilter.include); }
    if (typeFilter?.exclude) { conditions.push('type != ?'); params.push(typeFilter.exclude); }

    // metadata_search is JSONB on postgres (needs an explicit text cast for LIKE) but is
    // already TEXT/NVARCHAR on sqlite/mssql per the hand-written schema — no cast needed there.
    const metadataSearchExpr = db.getDialect() === 'postgres' ? 'CAST(metadata_search AS TEXT)' : 'metadata_search';
    const orParts = [`${metadataSearchExpr} LIKE ?`];
    const orParams = [pattern];
    if (taggedApiIds.length > 0) {
        orParts.push(`uuid IN (${taggedApiIds.map(() => '?').join(', ')})`);
        orParams.push(...taggedApiIds);
    }
    conditions.push(`(${orParts.join(' OR ')})`);
    params.push(...orParams);

    if (viewId) {
        conditions.push(
            `EXISTS (SELECT 1 FROM ${API_LABEL_MAPPINGS_TABLE} alm
                     WHERE alm.api_uuid = ${API_METADATA_TABLE}.uuid
                     AND alm.label_uuid IN (SELECT label_uuid FROM ${VIEW_LABEL_MAPPINGS_TABLE} WHERE view_uuid = ?))`
        );
        params.push(viewId);
    }

    const rows = await exec.query(`SELECT * FROM ${API_METADATA_TABLE} WHERE ${conditions.join(' AND ')}`, params);
    await attachAssociations(rows, t);
    return rows;
};

const search = async (orgId, searchTerm, viewName, t, typeFilter) => {
    if (db.getDialect() !== 'postgres') {
        return searchFallback(orgId, searchTerm, viewName, t, typeFilter);
    }
    const exec = t || db;
    const viewDao = require('./viewDao');
    const viewId = await viewDao.getId(orgId, viewName, t);
    const { sql, params } = db.bindNamedParams(SEARCH_APIS_POSTGRES_SQL, {
        searchTerm,
        orgId,
        viewId: viewId || null,
        includeType: typeFilter?.include || null,
        excludeType: typeFilter?.exclude || null,
    });
    const rows = await exec.query(sql, params);
    await attachAssociations(rows, t);
    return rows;
};

const getId = async (orgId, apiHandle) => {
    const api = await db.queryOne(
        `SELECT uuid FROM ${API_METADATA_TABLE} WHERE handle = ? AND org_uuid = ?`,
        [apiHandle, orgId]
    );
    return api?.uuid;
};

// Same as getId, but also constrains the match to a specific `type` (e.g. 'MCP') in a
// single query — used by resource families that only manage one API type.
const getIdByType = async (orgId, apiHandle, type) => {
    const api = await db.queryOne(
        `SELECT uuid FROM ${API_METADATA_TABLE} WHERE handle = ? AND org_uuid = ? AND type = ?`,
        [apiHandle, orgId, type]
    );
    return api?.uuid;
};

// Inverse of getIdByType — matches any type EXCEPT the excluded one. Used by /apis/*
// once a type gets its own dedicated resource family (e.g. MCP via /mcp-servers), so
// /apis/* stops resolving handles that belong to that dedicated family.
const getIdExcludingType = async (orgId, apiHandle, excludedType) => {
    const api = await db.queryOne(
        `SELECT uuid FROM ${API_METADATA_TABLE} WHERE handle = ? AND org_uuid = ? AND type != ?`,
        [apiHandle, orgId, excludedType]
    );
    return api?.uuid;
};

const getHandle = async (orgId, apiRefId) => {
    const api = await db.queryOne(
        `SELECT handle FROM ${API_METADATA_TABLE} WHERE ref_id = ? AND org_uuid = ?`,
        [apiRefId, orgId]
    );
    return api?.handle ?? null;
};

const getIdByRef = async (orgId, referenceId, t) => {
    const exec = t || db;
    const api = await exec.queryOne(
        `SELECT uuid FROM ${API_METADATA_TABLE} WHERE ref_id = ? AND org_uuid = ?`,
        [referenceId, orgId]
    );
    return api?.uuid;
};

const getSpecs = async (orgId, apiIds) => {
    if (!apiIds || apiIds.length === 0) return [];
    try {
        const placeholders = apiIds.map(() => '?').join(', ');
        const rows = await db.query(
            `SELECT c.api_uuid AS api_uuid, c.file_name AS file_name, c.file_content AS file_content
             FROM ${CONTENT_TABLE} c JOIN ${API_METADATA_TABLE} m ON c.api_uuid = m.uuid
             WHERE c.api_uuid IN (${placeholders}) AND c.type = ? AND m.org_uuid = ?`,
            [...apiIds, constants.DOC_TYPES.API_DEFINITION, orgId]
        );
        return rows.map((spec) => ({
            apiId: spec.api_uuid,
            fileName: spec.file_name,
            apiSpec: spec.file_content ? spec.file_content.toString('utf8') : null,
        }));
    } catch (error) {
        logger.error('Error fetching API specifications', {
            error: error.message,
            stack: error.stack,
            operation: 'fetchAPISpecifications',
        });
        throw error;
    }
};

const existsByNameVersion = async (orgId, apiName, apiVersion) => {
    const row = await db.queryOne(
        `SELECT uuid FROM ${API_METADATA_TABLE} WHERE org_uuid = ? AND name = ? AND version = ?`,
        [orgId, apiName, apiVersion]
    );
    return !!row;
};

module.exports = {
    create,
    update,
    delete: deleteApi,
    get,
    getByCondition,
    list,
    listFromAllViews,
    search,
    searchFallback,
    getId,
    getIdByType,
    getIdExcludingType,
    getHandle,
    getIdByRef,
    getSpecs,
    existsByNameVersion,
};
