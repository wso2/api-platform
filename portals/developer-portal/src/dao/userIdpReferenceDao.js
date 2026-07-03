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
const UserIdpReference = require('../models/userIdpReference');
const { findOrCreateSafe } = require('./findOrCreateHelper');

const DELETED_USER = 'deleted_user';

/**
 * Find-or-create the idp reference row for this idp id, returning its uuid.
 * Falls back to a plain lookup on a unique-constraint race between concurrent
 * requests for the same idp id.
 */
const resolveUuid = async (idpId) => {
    const reference = await findOrCreateSafe(UserIdpReference, { idp_id: idpId }, { idp_id: idpId });
    return reference.uuid;
};

/**
 * Resolve a single created_by/updated_by-style uuid for display, returning
 * "deleted_user" when the reference no longer exists.
 */
const resolveDisplay = async (uuid) => {
    if (!uuid) return DELETED_USER;
    const reference = await UserIdpReference.findByPk(uuid);
    return reference ? reference.idp_id : DELETED_USER;
};

/**
 * Batch-resolve multiple uuids in one query, for list responses. Returns a
 * Map<uuid, idp_id | "deleted_user"> covering every uuid passed in.
 */
const resolveMany = async (uuids) => {
    const distinctUuids = [...new Set((uuids || []).filter(Boolean))];
    const result = new Map(distinctUuids.map((uuid) => [uuid, DELETED_USER]));
    if (distinctUuids.length === 0) return result;

    const references = await UserIdpReference.findAll({ where: { uuid: distinctUuids } });
    for (const reference of references) {
        result.set(reference.uuid, reference.idp_id);
    }
    return result;
};

/**
 * Resolved createdBy/updatedBy/createdAt/updatedAt for a single-resource GET
 * response. Batches both lookups into one query via resolveMany.
 */
const buildSingleAuditFields = async (model) => {
    const display = await resolveMany([model.created_by, model.updated_by]);
    return {
        createdBy: display.get(model.created_by) ?? DELETED_USER,
        updatedBy: display.get(model.updated_by) ?? DELETED_USER,
        createdAt: model.created_at,
        updatedAt: model.updated_at,
    };
};

/**
 * Resolved createdBy/createdAt/updatedAt (no updatedBy) for each model in a
 * list response, batching the created_by lookup into a single query.
 */
const buildListAuditFields = async (models) => {
    const display = await resolveMany(models.map((model) => model.created_by));
    return models.map((model) => ({
        createdBy: display.get(model.created_by) ?? DELETED_USER,
        createdAt: model.created_at,
        updatedAt: model.updated_at,
    }));
};

module.exports = {
    DELETED_USER,
    resolveUuid,
    resolveDisplay,
    resolveMany,
    buildSingleAuditFields,
    buildListAuditFields,
};
