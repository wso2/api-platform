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
const Labels = require('../models/label');
const { APILabels } = require('../models/apiMetadata');
const ViewLabels = require('../models/viewLabel');
const { Sequelize, Op } = require('sequelize');
const constants = require('../utils/constants');
const { CustomError } = require('../utils/errors/customErrors');

const createMany = async (orgID, labels, createdBy, t) => {

    const labelList = [];
    try {
        labels.forEach(label => {
            labelList.push({
                NAME: label.name,
                DISPLAY_NAME: label.displayName,
                ORG_UUID: orgID,
                CREATED_BY: createdBy,
                UPDATED_BY: createdBy
            });
        })
        const labelResponse = await Labels.bulkCreate(labelList, { transaction: t });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const createApiMapping = async (orgID, apiID, labels, createdBy, t) => {

    const labelList = [];
    const IDList = await getId(orgID, labels, t);
    try {
        IDList.forEach(label => {
            labelList.push({
                LABEL_UUID: label,
                API_UUID: apiID,
                CREATED_BY: createdBy,
            });
        });
        const labelResponse = await APILabels.bulkCreate(labelList, { transaction: t, ignoreDuplicates: true });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }

}

const update = async (orgID, label, updatedBy, t) => {

    try {
        let [record, created] = await Labels.findOrCreate({
            where: {
                NAME: label.name,
                ORG_UUID: orgID
            },
            defaults: {
                NAME: label.name,
                DISPLAY_NAME: label.displayName,
                CREATED_BY: updatedBy,
                UPDATED_BY: updatedBy
            },
            transaction: t,
            returning: true
        });
        if (!created) {
            record = await record.update({ DISPLAY_NAME: label.displayName, UPDATED_BY: updatedBy, UPDATED_AT: new Date() }, { transaction: t }); // Update if found
        }
        return record;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getId = async (orgID, labels, t) => {

    let IDList = [];
    try {
        for (const label of labels) {
            IDList.push(await getIdList(orgID, label, t));
        };
        return IDList;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof CustomError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getIdList = async (orgID, label, t) => {

    const labelResponse = await Labels.findOne({
        where: {
            NAME: label,
            ORG_UUID: orgID
        },
        transaction: t
    });
    if (!labelResponse) {
        throw new CustomError(404, constants.ERROR_CODE[404], "Label not found")
    }
    return labelResponse.dataValues.UUID;
}

const deleteLabel = async (orgID, labelNames) => {

    try {
        const labelResponse = await Labels.destroy({
            where: {
                NAME: labelNames,
                ORG_UUID: orgID
            }
        });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const list = async (orgID) => {

    try {
        const labelResponse = await Labels.findAll({
            where: {
                ORG_UUID: orgID
            }
        });
        return labelResponse;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const deleteApiMapping = async (orgID, apiID, labels, t) => {

    const IDList = await getId(orgID, labels, t);
    try {
        return await APILabels.destroy({
            where: {
                LABEL_UUID: { [Op.in]: IDList },
                API_UUID: apiID,
            },
            transaction: t
        });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const addToView = async (orgID, labelID, viewID, createdBy, t) => {
    try {
        const [record] = await ViewLabels.findOrCreate({
            where: { LABEL_UUID: labelID, VIEW_UUID: viewID },
            defaults: { CREATED_BY: createdBy },
            transaction: t,
        });
        return record;
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
}

module.exports = {
    createMany,
    createApiMapping,
    update,
    getId,
    getIdList,
    delete: deleteLabel,
    list,
    deleteApiMapping,
    addToView,
};
