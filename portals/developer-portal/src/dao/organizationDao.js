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
const { Organization, OrgContent } = require('../models/organization');
const { Sequelize, Op } = require('sequelize');
const viewDao = require('./viewDao');
const { Application, ApplicationKeyMapping, SubscriptionMapping } = require('../models/application');
const { KeyManager } = require('../models/keyManager');
const APIKey = require('../models/apiKey');
const DPEvent = require('../models/event');
const DPEventDelivery = require('../models/eventDelivery');
const { APIWorkflow } = require('../models/apiWorkflow');
const { WebhookSubscriber } = require('../models/webhookSubscriber');
const View = require('../models/view');
const Labels = require('../models/label');
const Tags = require('../models/tag');

const create = async (orgData, t) => {
    let devPortalId = "";
    if (orgData.handle) {
        devPortalId = orgData.handle.toLowerCase();
    }
    const createOrgData = {
        display_name: orgData.displayName,
        business_owner: orgData.businessOwner,
        business_owner_contact: orgData.businessOwnerContact,
        business_owner_email: orgData.businessOwnerEmail,
        handle: devPortalId,
        idp_ref_id: orgData.idpRefId,
        cp_ref_id: orgData.cpRefId,
        configuration: orgData.configuration,
        created_by: orgData.createdBy,
        updated_by: orgData.createdBy
    };
    try {
        const organization = await Organization.create(createOrgData, { transaction: t });
        return organization;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

// Matches by handle, then name, then idp_ref_id, in that priority order — deterministic
// even if one org's handle happens to equal another org's name or idp_ref_id, unlike a
// single Op.or query (which returns whichever row the DB orders first).
const findOrgByIdentifier = async (param, t) => {
    const opts = { ...(t && { transaction: t }) };
    const handle = typeof param === 'string' ? param.toLowerCase() : param;
    return (await Organization.findOne({ where: { handle }, ...opts })) ||
        (await Organization.findOne({ where: { display_name: param }, ...opts })) ||
        (await Organization.findOne({ where: { idp_ref_id: param }, ...opts }));
};

const get = async (param, t) => {
    try {
        const organization = await findOrgByIdentifier(param, t);
        if (!organization) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        return organization;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

// For internal callers that already hold a resolved org uuid (e.g. req.orgId set by
// auth middleware) — not for public REST lookups, which should use get()/handle instead.
const getByUuid = async (uuid, t) => {
    try {
        const organization = await Organization.findOne({
            where: { uuid },
            ...(t && { transaction: t }),
        });
        if (!organization) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        return organization;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const getId = async (orgName) => {
    try {
        const organization = await findOrgByIdentifier(orgName);
        if (!organization) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        return organization.uuid;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const list = async () => {
    try {
        const organizations = await Organization.findAll();
        if (organizations.length === 0) {
            return [];
        }
        return organizations;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const update = async (orgData, t) => {
    try {
        const existing = await get(orgData.orgId, t);
        const devPortalId = orgData.handle ? orgData.handle.toLowerCase() : existing.handle;
        const [updatedRowsCount] = await Organization.update(
            {
                display_name: orgData.displayName,
                business_owner: orgData.businessOwner,
                business_owner_contact: orgData.businessOwnerContact,
                business_owner_email: orgData.businessOwnerEmail,
                handle: devPortalId,
                idp_ref_id: orgData.idpRefId,
                ...(orgData.cpRefId !== undefined && { cp_ref_id: orgData.cpRefId }),
                ...(orgData.configuration !== undefined && { configuration: orgData.configuration }),
                updated_by: orgData.updatedBy,
                updated_at: new Date()
            },
            {
                where: { uuid: existing.uuid },
                transaction: t,
            }
        );
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        // `returning: true` only works on Postgres/MSSQL — Sequelize's sqlite
        // dialect doesn't support RETURNING on UPDATE, so re-fetch explicitly
        // instead (same pattern as applicationDao.update).
        const updatedOrg = await Organization.findOne({ where: { uuid: existing.uuid }, transaction: t });
        return [updatedRowsCount, [updatedOrg]];
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

// Tables whose org_uuid FK is ON DELETE NO ACTION (database/schema.postgres.sql)
// block Organization.destroy() unless their rows are removed first. Tables with
// ON DELETE CASCADE/SET NULL (dp_api_metadata, dp_subscription_plans, dp_audit,
// dp_user_organization_mappings, and the *_mappings join tables) are left to the
// database to handle and aren't touched here.
const deleteOrgDependents = async (orgUuid, t) => {
    const opts = { transaction: t };

    const events = await DPEvent.findAll({ attributes: ['uuid'], where: { org_uuid: orgUuid }, ...opts });
    if (events.length) {
        await DPEventDelivery.destroy({ where: { event_uuid: { [Op.in]: events.map(e => e.uuid) } }, ...opts });
    }
    await DPEvent.destroy({ where: { org_uuid: orgUuid }, ...opts });

    await APIKey.destroy({ where: { org_uuid: orgUuid }, ...opts });
    await SubscriptionMapping.destroy({ where: { org_uuid: orgUuid }, ...opts });

    const [apps, keyManagers] = await Promise.all([
        Application.findAll({ attributes: ['uuid'], where: { org_uuid: orgUuid }, ...opts }),
        KeyManager.findAll({ attributes: ['uuid'], where: { org_uuid: orgUuid }, ...opts }),
    ]);
    if (apps.length || keyManagers.length) {
        await ApplicationKeyMapping.destroy({
            where: {
                [Op.or]: [
                    ...(apps.length ? [{ app_uuid: { [Op.in]: apps.map(a => a.uuid) } }] : []),
                    ...(keyManagers.length ? [{ km_uuid: { [Op.in]: keyManagers.map(k => k.uuid) } }] : []),
                ],
            },
            ...opts,
        });
    }
    await Application.destroy({ where: { org_uuid: orgUuid }, ...opts });
    await KeyManager.destroy({ where: { org_uuid: orgUuid }, ...opts });

    await APIWorkflow.destroy({ where: { org_uuid: orgUuid }, ...opts });
    await OrgContent.destroy({ where: { org_uuid: orgUuid }, ...opts });
    // dp_view_label_mappings/dp_api_label_mappings cascade automatically from
    // dp_views/dp_labels ON DELETE CASCADE.
    await View.destroy({ where: { org_uuid: orgUuid }, ...opts });
    await Labels.destroy({ where: { org_uuid: orgUuid }, ...opts });
    await Tags.destroy({ where: { org_uuid: orgUuid }, ...opts });
    await WebhookSubscriber.destroy({ where: { org_uuid: orgUuid }, ...opts });
};

const deleteOrg = async (orgId, t) => {
    try {
        const existing = await get(orgId, t);
        await deleteOrgDependents(existing.uuid, t);
        const deletedRowsCount = await Organization.destroy({
            where: { uuid: existing.uuid },
            ...(t && { transaction: t }),
        });
        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createContent = async (orgData, t) => {
    const viewId = await viewDao.getId(orgData.orgId, orgData.viewName);
    try {
        const orgContent = await OrgContent.create({
            file_type: orgData.fileType,
            file_name: orgData.fileName,
            file_content: orgData.fileContent,
            file_path: orgData.filePath,
            org_uuid: orgData.orgId,
            view_uuid: viewId,
            created_by: orgData.createdBy,
            updated_by: orgData.createdBy
        }, { transaction: t });
        return orgContent;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const updateContent = async (orgData) => {
    const viewId = await viewDao.getId(orgData.orgId, orgData.viewName);
    try {
        const [updatedRowsCount, updatedOrgContent] = await OrgContent.update({
            file_type: orgData.fileType,
            file_name: orgData.fileName,
            file_content: orgData.fileContent,
            file_path: orgData.filePath,
            updated_by: orgData.updatedBy,
            updated_at: new Date()
        },
            {
                where: {
                    file_type: orgData.fileType,
                    file_name: orgData.fileName,
                    file_path: orgData.filePath,
                    org_uuid: orgData.orgId,
                    view_uuid: viewId
                },
                returning: true
            });
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('No new resources found');
        }
        return [updatedRowsCount, updatedOrgContent];
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getContent = async (orgData) => {
    try {
        const viewId = await viewDao.getId(orgData.orgId, orgData.viewName);
        if (orgData.fileName || orgData.filePath) {
            return await OrgContent.findOne(
                {
                    where: {
                        org_uuid: orgData.orgId,
                        view_uuid: viewId,
                        file_type: orgData.fileType,
                        ...(orgData.fileName && { file_name: orgData.fileName }),
                        ...(orgData.filePath && { file_path: orgData.filePath })
                    }
                });
        } else {
            return await OrgContent.findAll(
                {
                    where: {
                        org_uuid: orgData.orgId,
                        view_uuid: viewId,
                        file_type: orgData.fileType,
                    }
                });
        }
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteContent = async (orgId, viewName, fileName) => {
    const viewId = await viewDao.getId(orgId, viewName);
    try {
        const deletedRowsCount = await OrgContent.destroy({
            where: {
                org_uuid: orgId,
                view_uuid: viewId,
                file_name: fileName
            }
        });

        if (deletedRowsCount < 1) {
            throw Object.assign(new Sequelize.EmptyResultError('Organization content not found'));
        }
        return deletedRowsCount;
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteAllContent = async (orgId, viewName, t) => {
    const viewId = await viewDao.getId(orgId, viewName);
    try {
        return await OrgContent.destroy({
            where: {
                org_uuid: orgId,
                view_uuid: viewId
            },
            transaction: t
        });
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
};

module.exports = {
    create,
    get,
    getByUuid,
    getId,
    list,
    update,
    delete: deleteOrg,
    createContent,
    updateContent,
    getContent,
    deleteContent,
    deleteAllContent,
};
