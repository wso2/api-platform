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
const { findOrCreateSafe } = require('./findOrCreateHelper');

const TAGS_TABLE = 'dp_tags';
const API_TAGS_TABLE = 'dp_api_tag_mappings';

// Built once at module load — buildUpsert only depends on the (fixed) dialect
// and column list, not on any per-call data.
const UPSERT_API_TAG_SQL = db.buildUpsert(
    API_TAGS_TABLE,
    ['uuid', 'tag_uuid', 'api_uuid', 'created_by'],
    ['tag_uuid', 'api_uuid'],
    [] // ignoreDuplicates semantics — leave the existing mapping row untouched on conflict
);

/**
 * Tags are freeform — unlike labels, an unknown tag name is created on the fly
 * rather than rejected, so existing free-text tagging behavior is preserved.
 */
const getOrCreateIds = async (orgId, tagNames, createdBy, t) => {
    const exec = t || db;
    const idList = [];
    for (const name of tagNames) {
        const trimmed = String(name).trim();
        if (!trimmed) continue;
        const tag = await findOrCreateSafe(
            TAGS_TABLE,
            { name: trimmed, org_uuid: orgId },
            {
                uuid: crypto.randomUUID(),
                name: trimmed,
                org_uuid: orgId,
                created_by: createdBy,
                updated_by: createdBy,
            },
            exec
        );
        idList.push(tag.uuid);
    }
    return idList;
};

const createApiMapping = async (orgId, apiId, tagNames, createdBy, t) => {
    const exec = t || db;
    const idList = await getOrCreateIds(orgId, tagNames || [], createdBy, t);
    for (const tagId of idList) {
        await exec.execute(UPSERT_API_TAG_SQL, [crypto.randomUUID(), tagId, apiId, createdBy]);
    }
    return idList;
};

/**
 * Full replace — mirrors the previous TAGS column's overwrite-on-update semantics
 * (no incremental add/remove diffing like labels).
 */
const replaceApiMapping = async (orgId, apiId, tagNames, createdBy, t) => {
    const exec = t || db;
    await exec.execute(`DELETE FROM ${API_TAGS_TABLE} WHERE api_uuid = ?`, [apiId]);
    return createApiMapping(orgId, apiId, tagNames || [], createdBy, t);
};

module.exports = {
    getOrCreateIds,
    createApiMapping,
    replaceApiMapping,
};
