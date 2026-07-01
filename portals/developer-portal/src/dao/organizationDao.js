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
        name: orgData.name,
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

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const get = async (param) => {
    try {
        const conditions = [
            { name: param },
            { handle: typeof param === 'string' ? param.toLowerCase() : param },
            { idp_ref_id: param },
        ];
        if (typeof param === 'string' && UUID_RE.test(param)) {
            conditions.push({ uuid: param });
        }
        const organization = await Organization.findOne({
            where: { [Sequelize.Op.or]: conditions }
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
        const organization = await Organization.findOne({
            where: {
                [Sequelize.Op.or]: [
                    { name: orgName },
                    { handle: typeof orgName === 'string' ? orgName.toLowerCase() : orgName },
                    { idp_ref_id: orgName }
                ]
            }
        });
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
    let devPortalId = "";
    if (orgData.handle) {
        devPortalId = orgData.handle.toLowerCase();
    }
    try {
        const [updatedRowsCount, updatedOrg] = await Organization.update(
            {
                name: orgData.name,
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
                where: { uuid: orgData.orgId },
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

const deleteOrg = async (orgId) => {
    try {
        const deletedRowsCount = await Organization.destroy({
            where: { uuid: orgId }
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

const createContent = async (orgData) => {
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
        });
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

const deleteAllContent = async (orgId, viewName) => {
    const viewId = await viewDao.getId(orgId, viewName);
    try {
        const deletedRowsCount = await OrgContent.destroy({
            where: {
                org_uuid: orgId,
                view_uuid: viewId
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

module.exports = {
    create,
    get,
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
