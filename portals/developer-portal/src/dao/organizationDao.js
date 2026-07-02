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
const { Sequelize } = require('sequelize');
const viewDao = require('./viewDao');

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
        const [updatedRowsCount, updatedOrg] = await Organization.update(
            {
                display_name: orgData.displayName,
                business_owner: orgData.businessOwner,
                business_owner_contact: orgData.businessOwnerContact,
                business_owner_email: orgData.businessOwnerEmail,
                handle: devPortalId,
                idp_ref_id: orgData.idpRefId,
                ...(orgData.cpRefId !== undefined && { cp_ref_id: orgData.cpRefId }),
                configuration: orgData.configuration,
                updated_by: orgData.updatedBy,
                updated_at: new Date()
            },
            {
                where: { uuid: existing.uuid },
                returning: true,
                transaction: t,
            }
        );
        if (updatedRowsCount < 1) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        return [updatedRowsCount, updatedOrg];
    } catch (error) {
        if (error instanceof Sequelize.EmptyResultError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
};

const deleteOrg = async (orgId, t) => {
    try {
        const existing = await get(orgId, t);
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
