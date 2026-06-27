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
const { Sequelize, Op } = require('sequelize');
const constants = require('../utils/constants');
const { CustomError } = require('../utils/errors/customErrors');

const create = async (orgID, payload, t) => {

    let name = payload.name ? payload.name : payload.handle;
    try {
        const viewResponse = await View.create({
            NAME: name,
            HANDLE: payload.handle,
            ORG_ID: orgID
        }, { transaction: t });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const update = async (orgID, handle, name, t) => {

    try {
        let [record, created] = await View.findOrCreate({
            where: {
                HANDLE: handle,
                ORG_ID: orgID
            },
            defaults: {
                HANDLE: handle,
                NAME: name,
            },
            transaction: t,
            returning: true
        });
        if (!created) {
            record = await record.update({
                HANDLE: handle,
                NAME: name,
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

const deleteView = async (orgID, handle) => {

    try {
        const viewResponse = await View.destroy({
            where: {
                HANDLE: handle,
                ORG_ID: orgID
            }
        });
        return viewResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const get = async (orgID, handle) => {

    try {
        const viewResponse = await View.findOne({
            where: {
                HANDLE: handle,
                ORG_ID: orgID
            },
            include: {
                model: Labels,
                attributes: ["NAME"],
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

const getId = async (orgID, viewName) => {

    try {
        const viewResponse = await View.findOne({
            where: {
                [Op.or]: [
                    { NAME: viewName },
                    { HANDLE: viewName }
                ],
                ORG_ID: orgID
            }
        });
        if (!viewResponse) {
            throw new CustomError(404, constants.ERROR_CODE[404], "View not found")
        }
        return viewResponse.dataValues.ID;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw error;
    }
}

const list = async (orgID) => {

    try {
        const viewResponse = await View.findAll({
            where: {
                ORG_ID: orgID
            },
            include: {
                model: Labels,
                attributes: ["NAME"],
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

const addLabels = async (orgID, viewID, labels, t) => {

    const labelList = [];
    const IDList = await getLabelID(orgID, labels, t);
    try {
        IDList.forEach(label => {
            labelList.push({
                LABEL_ID: label,
                VIEW_ID: viewID,
                ORG_ID: orgID
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

const replaceLabels = async (orgID, viewID, labelNames, t) => {
    try {
        await ViewLabels.destroy({ where: { VIEW_ID: viewID, ORG_ID: orgID }, transaction: t });
        if (labelNames?.length) {
            await addLabels(orgID, viewID, labelNames, t);
        }
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof CustomError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteLabels = async (orgID, viewID, labels, t) => {

    const IDList = await getLabelID(orgID, labels);
    let deleteResponse;
    try {
        IDList.forEach(async label => {
            deleteResponse = await ViewLabels.destroy({
                where: {
                    LABEL_ID: label,
                    VIEW_ID: viewID,
                    ORG_ID: orgID
                }
            }, { transaction: t });
        });
        return deleteResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

// Internal helper used by addLabels, replaceLabels, deleteLabels
async function getLabelID(orgID, labels, t) {
    const labelDao = require('./labelDao');
    return labelDao.getId(orgID, labels, t);
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
    deleteLabels,
};
