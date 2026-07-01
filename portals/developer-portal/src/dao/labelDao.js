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

const create = async (orgId, label, createdBy, t) => {
    try {
        return await Labels.create({
            name: label.name,
            display_name: label.displayName,
            org_uuid: orgId,
            created_by: createdBy,
            updated_by: createdBy
        }, { transaction: t, returning: true });
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const findById = async (orgId, labelId, t) => {
    const record = await Labels.findOne({
        where: { uuid: labelId, org_uuid: orgId },
        transaction: t
    });
    if (!record) {
        throw new CustomError(404, constants.ERROR_CODE[404], 'Label not found');
    }
    return record;
}

const updateById = async (orgId, labelId, label, updatedBy, t) => {
    const record = await findById(orgId, labelId, t);
    return record.update(
        { display_name: label.displayName, updated_by: updatedBy, updated_at: new Date() },
        { transaction: t }
    );
}

const deleteById = async (orgId, labelId) => {
    try {
        const count = await Labels.destroy({ where: { uuid: labelId, org_uuid: orgId } });
        if (count === 0) {
            throw new CustomError(404, constants.ERROR_CODE[404], 'Label not found');
        }
        return count;
    } catch (error) {
        if (error instanceof CustomError) throw error;
        throw new Sequelize.DatabaseError(error);
    }
}

const createMany = async (orgId, labels, createdBy, t) => {

    const labelList = [];
    try {
        labels.forEach(label => {
            labelList.push({
                name: label.name,
                display_name: label.displayName,
                org_uuid: orgId,
                created_by: createdBy,
                updated_by: createdBy
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

const createApiMapping = async (orgId, apiId, labels, createdBy, t) => {

    const labelList = [];
    const IDList = await getId(orgId, labels, t);
    try {
        IDList.forEach(label => {
            labelList.push({
                label_uuid: label,
                api_uuid: apiId,
                created_by: createdBy,
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

const update = async (orgId, label, updatedBy, t) => {

    try {
        let [record, created] = await Labels.findOrCreate({
            where: {
                name: label.name,
                org_uuid: orgId
            },
            defaults: {
                name: label.name,
                display_name: label.displayName,
                created_by: updatedBy,
                updated_by: updatedBy
            },
            transaction: t,
            returning: true
        });
        if (!created) {
            record = await record.update({ display_name: label.displayName, updated_by: updatedBy, updated_at: new Date() }, { transaction: t }); // Update if found
        }
        return record;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getId = async (orgId, labels, t) => {

    let IDList = [];
    try {
        for (const label of labels) {
            IDList.push(await getIdList(orgId, label, t));
        };
        return IDList;
    } catch (error) {
        if (error instanceof Sequelize.UniqueConstraintError || error instanceof CustomError) {
            throw error;
        }
        throw new Sequelize.DatabaseError(error);
    }
}

const getIdList = async (orgId, label, t) => {

    const labelResponse = await Labels.findOne({
        where: {
            name: label,
            org_uuid: orgId
        },
        transaction: t
    });
    if (!labelResponse) {
        throw new CustomError(404, constants.ERROR_CODE[404], "Label not found")
    }
    return labelResponse.dataValues.uuid;
}

const list = async (orgId) => {

    try {
        const labelResponse = await Labels.findAll({
            where: {
                org_uuid: orgId
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

const deleteApiMapping = async (orgId, apiId, labels, t) => {

    const IDList = await getId(orgId, labels, t);
    try {
        return await APILabels.destroy({
            where: {
                label_uuid: { [Op.in]: IDList },
                api_uuid: apiId,
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

const addToView = async (orgId, labelId, viewId, createdBy, t) => {
    try {
        const [record] = await ViewLabels.findOrCreate({
            where: { label_uuid: labelId, view_uuid: viewId },
            defaults: { created_by: createdBy },
            transaction: t,
        });
        return record;
    } catch (error) {
        throw new Sequelize.DatabaseError(error);
    }
}

module.exports = {
    create,
    createMany,
    createApiMapping,
    update,
    updateById,
    findById,
    getId,
    getIdList,
    deleteById,
    list,
    deleteApiMapping,
    addToView,
};
