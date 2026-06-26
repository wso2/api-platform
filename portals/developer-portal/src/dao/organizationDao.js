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
    let devPortalID = "";
    if (orgData.orgHandle) {
        devPortalID = orgData.orgHandle.toLowerCase();
    }
    const createOrgData = {
        NAME: orgData.orgName,
        BUSINESS_OWNER: orgData.businessOwner,
        BUSINESS_OWNER_CONTACT: orgData.businessOwnerContact,
        BUSINESS_OWNER_EMAIL: orgData.businessOwnerEmail,
        HANDLE: devPortalID,
        IDP_IDENTIFIER: orgData.organizationIdentifier,
        CONFIGURATION: orgData.orgConfig
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
            { NAME: param },
            { HANDLE: typeof param === 'string' ? param.toLowerCase() : param },
            { IDP_IDENTIFIER: param },
        ];
        if (typeof param === 'string' && UUID_RE.test(param)) {
            conditions.push({ ID: param });
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
                    { NAME: orgName },
                    { HANDLE: typeof orgName === 'string' ? orgName.toLowerCase() : orgName },
                    { IDP_IDENTIFIER: orgName }
                ]
            }
        });
        if (!organization) {
            throw new Sequelize.EmptyResultError('Organization not found');
        }
        return organization.ID;
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
    let devPortalID = "";
    if (orgData.orgHandle) {
        devPortalID = orgData.orgHandle.toLowerCase();
    }
    try {
        const [updatedRowsCount, updatedOrg] = await Organization.update(
            {
                NAME: orgData.orgName,
                BUSINESS_OWNER: orgData.businessOwner,
                BUSINESS_OWNER_CONTACT: orgData.businessOwnerContact,
                BUSINESS_OWNER_EMAIL: orgData.businessOwnerEmail,
                HANDLE: devPortalID,
                IDP_IDENTIFIER: orgData.organizationIdentifier,
                CONFIGURATION: orgData.orgConfiguration
            },
            {
                where: { ID: orgData.orgId },
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
            where: { ID: orgId }
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
    const viewID = await viewDao.getId(orgData.orgId, orgData.viewName);
    try {
        const orgContent = await OrgContent.create({
            FILE_TYPE: orgData.fileType,
            FILE_NAME: orgData.fileName,
            FILE_CONTENT: orgData.fileContent,
            FILE_PATH: orgData.filePath,
            ORG_ID: orgData.orgId,
            VIEW_ID: viewID
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
    const viewID = await viewDao.getId(orgData.orgId, orgData.viewName);
    try {
        const [updatedRowsCount, updatedOrgContent] = await OrgContent.update({
            FILE_TYPE: orgData.fileType,
            FILE_NAME: orgData.fileName,
            FILE_CONTENT: orgData.fileContent,
            FILE_PATH: orgData.filePath,
        },
            {
                where: {
                    FILE_TYPE: orgData.fileType,
                    FILE_NAME: orgData.fileName,
                    FILE_PATH: orgData.filePath,
                    ORG_ID: orgData.orgId,
                    VIEW_ID: viewID
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
        const viewID = await viewDao.getId(orgData.orgId, orgData.viewName);
        if (orgData.fileName || orgData.filePath) {
            return await OrgContent.findOne(
                {
                    where: {
                        ORG_ID: orgData.orgId,
                        VIEW_ID: viewID,
                        FILE_TYPE: orgData.fileType,
                        ...(orgData.fileName && { FILE_NAME: orgData.fileName }),
                        ...(orgData.filePath && { FILE_PATH: orgData.filePath })
                    }
                });
        } else {
            return await OrgContent.findAll(
                {
                    where: {
                        ORG_ID: orgData.orgId,
                        VIEW_ID: viewID,
                        FILE_TYPE: orgData.fileType,
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
                ORG_ID: orgId,
                VIEW_ID: viewId,
                FILE_NAME: fileName
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
                ORG_ID: orgId,
                VIEW_ID: viewId
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
