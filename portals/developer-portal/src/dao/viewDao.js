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
const View = require('../models/view');
const ViewLabels = require('../models/viewLabel');
const Labels = require('../models/label');
const { OrgContent } = require('../models/organization');
const { Sequelize } = require('sequelize');
const constants = require('../utils/constants');
const { CustomError } = require('../utils/errors/customErrors');

const create = async (orgId, payload, createdBy, t) => {

    let displayName = payload.displayName ? payload.displayName : payload.handle;
    try {
        const viewResponse = await View.create({
            display_name: displayName,
            handle: payload.handle,
            org_uuid: orgId,
            created_by: createdBy,
            updated_by: createdBy
        }, { transaction: t });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const update = async (orgId, handle, displayName, updatedBy, t) => {

    try {
        let [record, created] = await View.findOrCreate({
            where: {
                handle: handle,
                org_uuid: orgId
            },
            defaults: {
                handle: handle,
                display_name: displayName ? displayName : handle,
                created_by: updatedBy,
                updated_by: updatedBy
            },
            transaction: t,
            returning: true
        });
        if (!created) {
            record = await record.update({
                handle: handle,
                ...(displayName && { display_name: displayName }),
                updated_by: updatedBy,
                updated_at: new Date()
            }, { transaction: t }); // Update if found
        }
        return record;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteView = async (orgId, handle, t) => {

    try {
        const view = await View.findOne({
            where: {
                handle: handle,
                org_uuid: orgId
            },
            transaction: t
        });
        if (!view) {
            return 0;
        }
        // Neither dependent's view_uuid FK actually cascades at the SQLite level —
        // dp_view_label_mappings (models/viewLabel.js) has no onDelete at all, and
        // dp_organization_assets (models/organization.js's OrgContent) only sets
        // onDelete: 'CASCADE' on the Sequelize association, which doesn't translate
        // into the generated SQLite constraint. Destroying a view with either still
        // attached fails with a FOREIGN KEY constraint error — detach both first,
        // same pattern organizationDao's whole-org delete already uses for OrgContent.
        await ViewLabels.destroy({ where: { view_uuid: view.dataValues.uuid }, transaction: t });
        await OrgContent.destroy({ where: { view_uuid: view.dataValues.uuid }, transaction: t });
        const viewResponse = await View.destroy({
            where: {
                handle: handle,
                org_uuid: orgId
            },
            transaction: t
        });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const get = async (orgId, handle) => {

    try {
        const viewResponse = await View.findOne({
            where: {
                handle: handle,
                org_uuid: orgId
            },
            include: {
                model: Labels,
                attributes: ["handle"],
                through: { attributes: [] }
            },
        });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getId = async (orgId, viewName, t) => {
    // `view` is an optional query param on /apis and /mcp-servers (apiViewQuery in the
    // OpenAPI spec) — a bare handle/display_name lookup with `undefined` throws at the
    // Sequelize layer ("WHERE parameter has invalid undefined value") rather than the
    // 404 below, so short-circuit before ever building that query.
    if (!viewName) return undefined;

    try {
        let viewResponse = await View.findOne({
            where: { handle: viewName, org_uuid: orgId },
            transaction: t
        });
        if (!viewResponse) {
            viewResponse = await View.findOne({
                where: { display_name: viewName, org_uuid: orgId },
                transaction: t
            });
        }
        if (!viewResponse) {
            throw new CustomError(404, constants.ERROR_CODE[404], "View not found")
        }
        return viewResponse.dataValues.uuid;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw error;
    }
}

const list = async (orgId) => {

    try {
        const viewResponse = await View.findAll({
            where: {
                org_uuid: orgId
            },
            include: {
                model: Labels,
                attributes: ["handle"],
                through: {
                    attributes: []
                }
            },
        });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const addLabels = async (orgId, viewId, labels, createdBy, t) => {

    const labelList = [];
    const IDList = await getLabelId(orgId, labels, t);
    try {
        IDList.forEach(label => {
            labelList.push({
                label_uuid: label,
                view_uuid: viewId,
                created_by: createdBy,
            });
        });
        const labelResponse = await ViewLabels.bulkCreate(labelList, { transaction: t });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const replaceLabels = async (orgId, viewId, labelNames, createdBy, t) => {
    try {
        await ViewLabels.destroy({ where: { view_uuid: viewId }, transaction: t });
        if (labelNames?.length) {
            await addLabels(orgId, viewId, labelNames, createdBy, t);
        }
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof CustomError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

// Internal helper used by addLabels, replaceLabels
async function getLabelId(orgId, labels, t) {
    const labelDao = require('./labelDao');
    return labelDao.getId(orgId, labels, t);
}

module.exports = {
    create,
    update,
    delete: deleteView,
    get,
    getId,
    list,
    addLabels,
    replaceLabels,
};
