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
const getOrCreateIds = async (orgID, tagNames, t) => {

    const IDList = [];
    try {
        for (const name of tagNames) {
            const trimmed = String(name).trim();
            if (!trimmed) continue;
            const [tag] = await Tags.findOrCreate({
                where: {
                    NAME: trimmed,
                    ORG_UUID: orgID
                },
                defaults: {
                    NAME: trimmed,
                    ORG_UUID: orgID
                },
                transaction: t
            });
            IDList.push(tag.UUID);
        }
        return IDList;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createApiMapping = async (orgID, apiID, tagNames, t) => {

    const IDList = await getOrCreateIds(orgID, tagNames || [], t);
    try {
        const tagList = IDList.map(tagID => ({
            TAG_UUID: tagID,
            API_UUID: apiID,
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
const replaceApiMapping = async (orgID, apiID, tagNames, t) => {

    try {
        await APITags.destroy({
            where: {
                API_UUID: apiID,
            },
            transaction: t
        });
        return await createApiMapping(orgID, apiID, tagNames || [], t);
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
