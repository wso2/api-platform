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
const Tags = require('../models/tag');
const { APITags } = require('../models/apiMetadata');
const { Sequelize } = require('sequelize');

/**
 * Tags are freeform — unlike labels, an unknown tag name is created on the fly
 * rather than rejected, so existing free-text tagging behavior is preserved.
 */
const getOrCreateIds = async (orgId, tagNames, createdBy, t) => {

    const IDList = [];
    try {
        for (const name of tagNames) {
            const trimmed = String(name).trim();
            if (!trimmed) continue;
            const [tag] = await Tags.findOrCreate({
                where: {
                    name: trimmed,
                    org_uuid: orgId
                },
                defaults: {
                    name: trimmed,
                    org_uuid: orgId,
                    created_by: createdBy,
                    updated_by: createdBy
                },
                transaction: t
            });
            IDList.push(tag.uuid);
        }
        return IDList;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createApiMapping = async (orgId, apiId, tagNames, createdBy, t) => {

    const IDList = await getOrCreateIds(orgId, tagNames || [], createdBy, t);
    try {
        const tagList = IDList.map(tagId => ({
            tag_uuid: tagId,
            api_uuid: apiId,
            created_by: createdBy,
        }));
        return await APITags.bulkCreate(tagList, { transaction: t, ignoreDuplicates: true });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

/**
 * Full replace — mirrors the previous TAGS column's overwrite-on-update semantics
 * (no incremental add/remove diffing like labels).
 */
const replaceApiMapping = async (orgId, apiId, tagNames, createdBy, t) => {

    try {
        await APITags.destroy({
            where: {
                api_uuid: apiId,
            },
            transaction: t
        });
        return await createApiMapping(orgId, apiId, tagNames || [], createdBy, t);
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

module.exports = {
    getOrCreateIds,
    createApiMapping,
    replaceApiMapping,
};
